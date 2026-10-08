package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/webapp/util"
	"github.com/kkEo/g-mk8s/wire"
)

type registrationTokenInput struct {
	TTLMinutes int `json:"ttl_minutes"`
}

// createRegistrationToken mints a one-time token an operator pastes into
// `goran-agent register`. Default lifetime is one hour.
func (s *Server) createRegistrationToken(c *gin.Context) {
	var in registrationTokenInput
	if c.Request.ContentLength > 0 && !bind(c, &in) {
		return
	}
	if in.TTLMinutes <= 0 {
		in.TTLMinutes = 60
	}
	raw, err := util.NewToken("gr_")
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ws := middleware.CurrentWorkspace(c)
	rt := model.RegistrationToken{
		WorkspaceID: ws.ID,
		Prefix:      util.Prefix(raw),
		Hash:        util.Hash(raw),
		ExpiresAt:   s.cfg.Now().Add(time.Duration(in.TTLMinutes) * time.Minute),
	}
	if err := s.db.Create(&rt).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token":      raw,
		"expires_at": rt.ExpiresAt,
		"workspace":  ws.Name,
		"note":       "single use; run: goran-agent register --server <url> --token " + raw + " --name <agent-name>",
	})
}

// registerAgent is unauthenticated: the registration token is the credential.
func (s *Server) registerAgent(c *gin.Context) {
	var in wire.RegisterRequest
	if !bind(c, &in) {
		return
	}
	if in.Token == "" || !validName(in.Name) {
		fail(c, http.StatusBadRequest, "token and a valid name are required")
		return
	}
	now := s.cfg.Now()
	var rt model.RegistrationToken
	if err := s.db.Where("hash = ?", util.Hash(in.Token)).First(&rt).Error; err != nil {
		fail(c, http.StatusUnauthorized, "invalid registration token")
		return
	}
	if rt.UsedAt != nil || now.After(rt.ExpiresAt) {
		fail(c, http.StatusUnauthorized, "registration token already used or expired")
		return
	}
	key, err := util.NewToken("ga_")
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ag := model.Agent{
		WorkspaceID: rt.WorkspaceID,
		Name:        in.Name,
		Labels:      model.NormalizeLabels(in.Labels),
		KeyPrefix:   util.Prefix(key),
		KeyHash:     util.Hash(key),
		LastSeenAt:  &now,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Consume the token atomically so two concurrent registrations cannot share it.
		res := tx.Model(&model.RegistrationToken{}).
			Where("id = ? AND used_at IS NULL", rt.ID).
			Update("used_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errConflict
		}
		if err := tx.Create(&ag).Error; err != nil {
			return err
		}
		return tx.Model(&model.RegistrationToken{}).Where("id = ?", rt.ID).Update("agent_id", ag.ID).Error
	})
	if err == errConflict {
		fail(c, http.StatusUnauthorized, "registration token already used")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, wire.RegisterResponse{AgentID: ag.ID, WorkspaceID: ag.WorkspaceID, Name: ag.Name, Key: key})
}

func (s *Server) listAgents(c *gin.Context) {
	ws := middleware.CurrentWorkspace(c)
	var agents []model.Agent
	s.db.Where("workspace_id = ?", ws.ID).Order("id").Find(&agents)
	if agents == nil {
		agents = []model.Agent{}
	}
	c.JSON(http.StatusOK, agents)
}

// deleteAgent revokes the key. Tasks it holds are requeued when their lease expires.
func (s *Server) deleteAgent(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	ws := middleware.CurrentWorkspace(c)
	res := s.db.Where("id = ? AND workspace_id = ?", id, ws.ID).Delete(&model.Agent{})
	if res.RowsAffected == 0 {
		fail(c, http.StatusNotFound, "agent not found")
		return
	}
	c.Status(http.StatusNoContent)
}
