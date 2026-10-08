package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/wire"
)

const maxLogChunk = 64 * 1024

// RequeueExpired puts tasks whose lease ran out back on the queue, or fails
// them once they have used up their attempts. It is safe to call often.
func (s *Server) RequeueExpired(now time.Time) (int, error) {
	var expired []model.Task
	err := s.db.Where("status IN ? AND lease_expires_at IS NOT NULL AND lease_expires_at < ?", model.Leased, now).
		Find(&expired).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range expired {
		var updates map[string]interface{}
		if t.Attempts >= t.MaxAttempts {
			updates = map[string]interface{}{
				"status":           model.StatusError,
				"error":            fmt.Sprintf("lease expired after %d attempt(s); the agent stopped reporting", t.Attempts),
				"lease_expires_at": nil,
				"finished_at":      now,
			}
		} else {
			updates = map[string]interface{}{
				"status":           model.StatusNew,
				"agent_id":         nil,
				"phase":            "",
				"lease_expires_at": nil,
			}
		}
		res := s.db.Model(&model.Task{}).
			Where("id = ? AND status = ? AND lease_expires_at = ?", t.ID, t.Status, t.LeaseExpiresAt).
			Updates(updates)
		if res.Error != nil {
			return n, res.Error
		}
		n += int(res.RowsAffected)
	}
	return n, nil
}

// nextTask hands the agent one task: first an approved plan it already holds,
// then the oldest new task whose labels it satisfies. Claiming is a guarded
// UPDATE so two agents never get the same task.
func (s *Server) nextTask(c *gin.Context) {
	ag := middleware.CurrentAgent(c)
	now := s.cfg.Now()
	if _, err := s.RequeueExpired(now); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	lease := now.Add(s.cfg.Lease)

	var approved model.Task
	err := s.db.Where("workspace_id = ? AND agent_id = ? AND status = ?", ag.WorkspaceID, ag.ID, model.StatusApproved).
		Order("id").First(&approved).Error
	if err == nil {
		res := s.db.Model(&model.Task{}).Where("id = ? AND status = ?", approved.ID, model.StatusApproved).
			Updates(map[string]interface{}{"status": model.StatusPending, "phase": wire.PhaseApply, "lease_expires_at": lease})
		if res.Error == nil && res.RowsAffected == 1 {
			s.respondTask(c, approved.ID, ag)
			return
		}
	}

	var candidates []model.Task
	if err := s.db.Where("workspace_id = ? AND status = ?", ag.WorkspaceID, model.StatusNew).
		Order("id").Limit(50).Find(&candidates).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	for _, t := range candidates {
		if !ag.Labels.ContainsAll(t.Labels) {
			continue
		}
		phase := wire.PhaseRun
		if t.Kind == wire.KindTerraform {
			phase = wire.PhasePlan
		}
		updates := map[string]interface{}{
			"status":           model.StatusPending,
			"phase":            phase,
			"agent_id":         ag.ID,
			"lease_expires_at": lease,
			"attempts":         gorm.Expr("attempts + 1"),
		}
		if t.StartedAt == nil {
			updates["started_at"] = now
		}
		res := s.db.Model(&model.Task{}).Where("id = ? AND status = ?", t.ID, model.StatusNew).Updates(updates)
		if res.Error != nil {
			fail(c, http.StatusInternalServerError, res.Error.Error())
			return
		}
		if res.RowsAffected == 1 {
			s.respondTask(c, t.ID, ag)
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// respondTask serialises a freshly claimed task, decrypting its secrets. If a
// secret is missing the task fails immediately instead of bouncing between agents.
func (s *Server) respondTask(c *gin.Context, id uint, ag *model.Agent) {
	var t model.Task
	if err := s.db.First(&t, id).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	env, err := s.resolveSecrets(t.WorkspaceID, t.Secrets)
	if err != nil {
		now := s.cfg.Now()
		s.db.Model(&t).Updates(map[string]interface{}{
			"status": model.StatusError, "error": err.Error(), "lease_expires_at": nil, "finished_at": now,
		})
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, wire.AgentTask{
		ID:             t.ID,
		Name:           t.Name,
		Kind:           t.Kind,
		Phase:          t.Phase,
		Params:         json.RawMessage(t.Params),
		Env:            env,
		TimeoutSeconds: t.TimeoutSeconds,
		LeaseSeconds:   int(s.cfg.Lease.Seconds()),
		Attempt:        t.Attempts,
		MaxAttempts:    t.MaxAttempts,
	})
}

// ownedTask loads :id if this agent currently holds it. Anything else is a
// 409 so the agent knows its lease is gone and stops working on the task.
func (s *Server) ownedTask(c *gin.Context) (*model.Task, bool) {
	id, ok := paramID(c, "id")
	if !ok {
		return nil, false
	}
	ag := middleware.CurrentAgent(c)
	var t model.Task
	err := s.db.Where("id = ? AND agent_id = ? AND status IN ?", id, ag.ID,
		[]model.TaskStatus{model.StatusPending, model.StatusRunning}).First(&t).Error
	if err != nil {
		fail(c, http.StatusConflict, "task is not assigned to this agent (lease lost or task finished)")
		return nil, false
	}
	return &t, true
}

func (s *Server) extendLease(c *gin.Context, t *model.Task) {
	now := s.cfg.Now()
	lease := now.Add(s.cfg.Lease)
	updates := map[string]interface{}{"lease_expires_at": lease}
	if t.Status == model.StatusPending {
		updates["status"] = model.StatusRunning
	}
	if err := s.db.Model(t).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, wire.LeaseResponse{Status: string(model.StatusRunning), LeaseExpiresAt: lease})
}

func (s *Server) heartbeat(c *gin.Context) {
	t, ok := s.ownedTask(c)
	if !ok {
		return
	}
	s.extendLease(c, t)
}

func (s *Server) appendLog(c *gin.Context) {
	t, ok := s.ownedTask(c)
	if !ok {
		return
	}
	var in wire.LogRequest
	if !bind(c, &in) {
		return
	}
	if len(in.Chunk) > maxLogChunk {
		fail(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("chunk exceeds %d bytes", maxLogChunk))
		return
	}
	if in.Chunk != "" {
		if err := s.db.Create(&model.TaskLog{TaskID: t.ID, Chunk: in.Chunk, CreatedAt: s.cfg.Now()}).Error; err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.extendLease(c, t)
}

func (s *Server) reportResult(c *gin.Context) {
	t, ok := s.ownedTask(c)
	if !ok {
		return
	}
	var in wire.ResultRequest
	if !bind(c, &in) {
		return
	}
	var status model.TaskStatus
	switch in.Status {
	case wire.ResultDone:
		status = model.StatusDone
	case wire.ResultError:
		status = model.StatusError
	case wire.ResultAwaitingApproval:
		status = model.StatusAwaitingApproval
	default:
		fail(c, http.StatusBadRequest, "status must be done, error or awaiting_approval")
		return
	}
	now := s.cfg.Now()
	updates := map[string]interface{}{
		"status":           status,
		"lease_expires_at": nil,
		"error":            in.Error,
		"exit_code":        in.ExitCode,
	}
	if status.Terminal() {
		updates["finished_at"] = now
	}
	res := s.db.Model(&model.Task{}).Where("id = ? AND agent_id = ? AND status IN ?", t.ID, t.AgentID,
		[]model.TaskStatus{model.StatusPending, model.StatusRunning}).Updates(updates)
	if res.Error != nil {
		fail(c, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		fail(c, http.StatusConflict, "task is not assigned to this agent (lease lost or task finished)")
		return
	}
	s.db.First(t, t.ID)
	c.JSON(http.StatusOK, t)
}
