package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
)

func (s *Server) listWorkspaces(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var wss []model.Workspace
	q := s.db.Order("workspaces.id")
	if !u.Admin {
		q = q.Joins("JOIN memberships ON memberships.workspace_id = workspaces.id").
			Where("memberships.user_id = ?", u.ID)
	}
	if err := q.Find(&wss).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, wss)
}

type workspaceInput struct {
	Name string `json:"name"`
}

// createWorkspace makes the caller its first admin.
func (s *Server) createWorkspace(c *gin.Context) {
	var in workspaceInput
	if !bind(c, &in) {
		return
	}
	if !validName(in.Name) {
		fail(c, http.StatusBadRequest, "name must be 1-64 letters, digits, '_', '.' or '-' and not purely numeric")
		return
	}
	u := middleware.CurrentUser(c)
	ws := model.Workspace{Name: in.Name}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.Workspace
		if err := tx.Where("name = ?", in.Name).First(&existing).Error; err == nil {
			return errConflict
		}
		if err := tx.Create(&ws).Error; err != nil {
			return err
		}
		return tx.Create(&model.Membership{WorkspaceID: ws.ID, UserID: u.ID, Role: model.RoleAdmin}).Error
	})
	if errors.Is(err, errConflict) {
		fail(c, http.StatusConflict, "workspace already exists")
		return
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, ws)
}

var errConflict = errors.New("conflict")

func (s *Server) getWorkspace(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"workspace":  middleware.CurrentWorkspace(c),
		"membership": middleware.CurrentMembership(c),
	})
}

type memberView struct {
	UserID uint   `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

func (s *Server) listMembers(c *gin.Context) {
	ws := middleware.CurrentWorkspace(c)
	var out []memberView
	s.db.Table("memberships").
		Select("memberships.user_id, users.name, users.email, memberships.role").
		Joins("JOIN users ON users.id = memberships.user_id").
		Where("memberships.workspace_id = ?", ws.ID).
		Order("memberships.id").Scan(&out)
	if out == nil {
		out = []memberView{}
	}
	c.JSON(http.StatusOK, out)
}

type memberInput struct {
	User string `json:"user"`
	Role string `json:"role"`
}

func (s *Server) addMember(c *gin.Context) {
	var in memberInput
	if !bind(c, &in) {
		return
	}
	if in.Role == "" {
		in.Role = model.RoleMember
	}
	if in.Role != model.RoleAdmin && in.Role != model.RoleMember {
		fail(c, http.StatusBadRequest, "role must be admin or member")
		return
	}
	var u model.User
	if err := s.db.Where("name = ?", in.User).First(&u).Error; err != nil {
		fail(c, http.StatusNotFound, "user not found")
		return
	}
	ws := middleware.CurrentWorkspace(c)
	var m model.Membership
	err := s.db.Where("workspace_id = ? AND user_id = ?", ws.ID, u.ID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		m = model.Membership{WorkspaceID: ws.ID, UserID: u.ID, Role: in.Role}
		err = s.db.Create(&m).Error
	} else if err == nil {
		err = s.db.Model(&m).Update("role", in.Role).Error
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, m)
}
