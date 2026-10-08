package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
)

func (s *Server) requireBox(c *gin.Context) bool {
	if s.cfg.Box == nil {
		fail(c, http.StatusServiceUnavailable, "secrets are disabled: set GORAN_MASTER_KEY on the server")
		return false
	}
	return true
}

type secretView struct {
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"`
}

// listSecrets returns names only. Values never leave the server through /api.
func (s *Server) listSecrets(c *gin.Context) {
	ws := middleware.CurrentWorkspace(c)
	var secrets []model.Secret
	s.db.Where("workspace_id = ?", ws.ID).Order("name").Find(&secrets)
	out := make([]secretView, 0, len(secrets))
	for _, sec := range secrets {
		out = append(out, secretView{Name: sec.Name, UpdatedAt: sec.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")})
	}
	c.JSON(http.StatusOK, out)
}

type secretInput struct {
	Value string `json:"value"`
}

func (s *Server) putSecret(c *gin.Context) {
	if !s.requireBox(c) {
		return
	}
	name := c.Param("name")
	if !validName(name) {
		fail(c, http.StatusBadRequest, "secret name must be 1-64 letters, digits, '_', '.' or '-'")
		return
	}
	var in secretInput
	if !bind(c, &in) {
		return
	}
	if in.Value == "" {
		fail(c, http.StatusBadRequest, "value must not be empty")
		return
	}
	ct, err := s.cfg.Box.Seal([]byte(in.Value))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ws := middleware.CurrentWorkspace(c)
	var sec model.Secret
	err = s.db.Where("workspace_id = ? AND name = ?", ws.ID, name).First(&sec).Error
	status := http.StatusOK
	if errors.Is(err, gorm.ErrRecordNotFound) {
		sec = model.Secret{WorkspaceID: ws.ID, Name: name, Ciphertext: ct}
		err = s.db.Create(&sec).Error
		status = http.StatusCreated
	} else if err == nil {
		err = s.db.Model(&sec).Update("ciphertext", ct).Error
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(status, gin.H{"name": sec.Name, "updated_at": sec.UpdatedAt})
}

func (s *Server) deleteSecret(c *gin.Context) {
	ws := middleware.CurrentWorkspace(c)
	res := s.db.Where("workspace_id = ? AND name = ?", ws.ID, c.Param("name")).Delete(&model.Secret{})
	if res.RowsAffected == 0 {
		fail(c, http.StatusNotFound, "secret not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// resolveSecrets decrypts the secrets a task references into env values.
// It is only ever called on the agent path, when a task has just been claimed.
func (s *Server) resolveSecrets(workspaceID uint, refs model.StringMap) (map[string]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	if s.cfg.Box == nil {
		return nil, errors.New("secrets are disabled on the server (GORAN_MASTER_KEY not set)")
	}
	env := make(map[string]string, len(refs))
	for envName, secretName := range refs {
		var sec model.Secret
		if err := s.db.Where("workspace_id = ? AND name = ?", workspaceID, secretName).First(&sec).Error; err != nil {
			return nil, fmt.Errorf("secret %q not found", secretName)
		}
		pt, err := s.cfg.Box.Open(sec.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("secret %q cannot be decrypted (master key changed?)", secretName)
		}
		env[envName] = string(pt)
	}
	return env, nil
}
