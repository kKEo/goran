package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/wire"
)

type taskInput struct {
	Name           string            `json:"name"`
	Kind           string            `json:"kind"`
	Params         json.RawMessage   `json:"params"`
	Secrets        map[string]string `json:"secrets"`
	Labels         []string          `json:"labels"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	MaxAttempts    int               `json:"max_attempts"`
}

// validateParams checks the kind-specific shape before a task is queued, so
// an agent never claims a task it cannot possibly run.
func validateParams(kind string, raw json.RawMessage) error {
	var p map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("params must be a JSON object: %w", err)
		}
	}
	if p == nil {
		p = map[string]interface{}{}
	}
	switch kind {
	case wire.KindShell:
		if script, _ := p["script"].(string); strings.TrimSpace(script) == "" {
			return errors.New(`shell task needs a non-empty "script"`)
		}
	case wire.KindTerraform:
		if err := validateSource(p["source"]); err != nil {
			return err
		}
	case wire.KindAnsible:
		if err := validateSource(p["source"]); err != nil {
			return err
		}
		if playbook, _ := p["playbook"].(string); playbook == "" {
			return errors.New(`ansible task needs "playbook"`)
		}
	default:
		return fmt.Errorf("unknown task kind %q (use shell, terraform or ansible)", kind)
	}
	return nil
}

func validateSource(v interface{}) error {
	src, ok := v.(map[string]interface{})
	if !ok {
		return errors.New(`"source" must be an object with "git" (URL) or "dir" (path on the agent)`)
	}
	git, _ := src["git"].(string)
	dir, _ := src["dir"].(string)
	if git == "" && dir == "" {
		return errors.New(`"source" needs "git" (URL) or "dir" (path on the agent)`)
	}
	return nil
}

func (s *Server) createTask(c *gin.Context) {
	ws := middleware.CurrentWorkspace(c)
	u := middleware.CurrentUser(c)
	var in taskInput
	if !bind(c, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 128 {
		fail(c, http.StatusBadRequest, "name is required (max 128 characters)")
		return
	}
	if err := validateParams(in.Kind, in.Params); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	for envName, secretName := range in.Secrets {
		if !envRe.MatchString(envName) {
			fail(c, http.StatusBadRequest, fmt.Sprintf("secrets: %q is not a valid environment variable name", envName))
			return
		}
		var n int64
		s.db.Model(&model.Secret{}).Where("workspace_id = ? AND name = ?", ws.ID, secretName).Count(&n)
		if n == 0 {
			fail(c, http.StatusBadRequest, fmt.Sprintf("secret %q does not exist in this workspace", secretName))
			return
		}
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = int(s.cfg.DefaultTimeout.Seconds())
	}
	if in.MaxAttempts <= 0 {
		in.MaxAttempts = s.cfg.MaxAttempts
	}
	params := model.JSON(in.Params)
	if len(params) == 0 || string(params) == "null" {
		params = model.JSON("{}")
	}
	t := model.Task{
		WorkspaceID:    ws.ID,
		Name:           in.Name,
		Kind:           in.Kind,
		Params:         params,
		Secrets:        model.StringMap(in.Secrets),
		Labels:         model.NormalizeLabels(in.Labels),
		TimeoutSeconds: in.TimeoutSeconds,
		Status:         model.StatusNew,
		MaxAttempts:    in.MaxAttempts,
		CreatedByID:    u.ID,
	}
	if t.Secrets == nil {
		t.Secrets = model.StringMap{}
	}
	if err := s.db.Create(&t).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (s *Server) listTasks(c *gin.Context) {
	ws := middleware.CurrentWorkspace(c)
	if _, err := s.RequeueExpired(s.cfg.Now()); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	q := s.db.Where("workspace_id = ?", ws.ID)
	if st := c.Query("status"); st != "" {
		q = q.Where("status IN ?", strings.Split(st, ","))
	}
	var tasks []model.Task
	if err := q.Order("id DESC").Limit(200).Find(&tasks).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	c.JSON(http.StatusOK, tasks)
}

// loadTask fetches :id inside the current workspace.
func (s *Server) loadTask(c *gin.Context) (*model.Task, bool) {
	id, ok := paramID(c, "id")
	if !ok {
		return nil, false
	}
	ws := middleware.CurrentWorkspace(c)
	var t model.Task
	if err := s.db.Where("id = ? AND workspace_id = ?", id, ws.ID).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, "task not found")
		} else {
			fail(c, http.StatusInternalServerError, err.Error())
		}
		return nil, false
	}
	return &t, true
}

func (s *Server) getTask(c *gin.Context) {
	if _, err := s.RequeueExpired(s.cfg.Now()); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	t, ok := s.loadTask(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, t)
}

// getTaskLog returns the concatenated output as text/plain.
func (s *Server) getTaskLog(c *gin.Context) {
	t, ok := s.loadTask(c)
	if !ok {
		return
	}
	var chunks []model.TaskLog
	if err := s.db.Where("task_id = ?", t.ID).Order("id").Find(&chunks).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	var b strings.Builder
	for _, ch := range chunks {
		b.WriteString(ch.Chunk)
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(b.String()))
}

// transition applies a guarded status change and returns the fresh task.
func (s *Server) transition(c *gin.Context, t *model.Task, from model.TaskStatus, updates map[string]interface{}) {
	if t.Status != from {
		fail(c, http.StatusConflict, fmt.Sprintf("task is %s, expected %s", t.Status, from))
		return
	}
	res := s.db.Model(&model.Task{}).Where("id = ? AND status = ?", t.ID, from).Updates(updates)
	if res.Error != nil {
		fail(c, http.StatusInternalServerError, res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		fail(c, http.StatusConflict, "task changed state concurrently, reload and retry")
		return
	}
	s.db.First(t, t.ID)
	c.JSON(http.StatusOK, t)
}

// approveTask releases a terraform plan for apply. The lease starts now: the
// agent that planned must pick the apply up before it expires, otherwise the
// task is requeued and planned again elsewhere.
func (s *Server) approveTask(c *gin.Context) {
	t, ok := s.loadTask(c)
	if !ok {
		return
	}
	now := s.cfg.Now()
	s.transition(c, t, model.StatusAwaitingApproval, map[string]interface{}{
		"status":           model.StatusApproved,
		"approved_by_id":   middleware.CurrentUser(c).ID,
		"lease_expires_at": now.Add(s.cfg.Lease),
	})
}

func (s *Server) rejectTask(c *gin.Context) {
	t, ok := s.loadTask(c)
	if !ok {
		return
	}
	now := s.cfg.Now()
	s.transition(c, t, model.StatusAwaitingApproval, map[string]interface{}{
		"status":           model.StatusRejected,
		"approved_by_id":   middleware.CurrentUser(c).ID,
		"lease_expires_at": nil,
		"finished_at":      now,
	})
}

// cancelTask withdraws a task that no agent is working on.
func (s *Server) cancelTask(c *gin.Context) {
	t, ok := s.loadTask(c)
	if !ok {
		return
	}
	if t.Status != model.StatusNew && t.Status != model.StatusAwaitingApproval {
		fail(c, http.StatusConflict, fmt.Sprintf("only new or awaiting_approval tasks can be canceled (task is %s)", t.Status))
		return
	}
	now := s.cfg.Now()
	s.transition(c, t, t.Status, map[string]interface{}{
		"status":           model.StatusCanceled,
		"lease_expires_at": nil,
		"finished_at":      now,
	})
}
