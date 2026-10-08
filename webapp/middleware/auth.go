// Package middleware authenticates users (API tokens) and agents (agent keys)
// and resolves workspace membership for /api/workspaces/:ws routes.
package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/webapp/util"
)

const (
	ctxUser       = "goran.user"
	ctxWorkspace  = "goran.workspace"
	ctxMembership = "goran.membership"
	ctxAgent      = "goran.agent"
)

// Auth builds the middlewares.
type Auth struct {
	DB  *gorm.DB
	Now func() time.Time
}

// Fail writes a JSON error and aborts the request.
func Fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

// BearerToken extracts the credential from the Authorization header.
// Both "Bearer <token>" and a bare token are accepted.
func BearerToken(c *gin.Context) string {
	h := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return h
}

// User requires a valid user API token.
func (a *Auth) User() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := BearerToken(c)
		if tok == "" {
			Fail(c, http.StatusUnauthorized, "missing Authorization: Bearer <token>")
			return
		}
		var t model.ApiToken
		if err := a.DB.Where("hash = ?", util.Hash(tok)).First(&t).Error; err != nil {
			Fail(c, http.StatusUnauthorized, "invalid token")
			return
		}
		var u model.User
		if err := a.DB.First(&u, t.UserID).Error; err != nil {
			Fail(c, http.StatusUnauthorized, "invalid token")
			return
		}
		now := a.Now()
		a.DB.Model(&t).UpdateColumn("last_used_at", now)
		c.Set(ctxUser, &u)
		c.Next()
	}
}

// RequireAdmin allows only global admins.
func (a *Auth) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := CurrentUser(c); u == nil || !u.Admin {
			Fail(c, http.StatusForbidden, "admin only")
			return
		}
		c.Next()
	}
}

// Membership resolves :ws (name or numeric id) and checks the caller belongs
// to it. Global admins are treated as workspace admins everywhere. Unknown and
// foreign workspaces both answer 404 so their existence is not leaked.
func (a *Auth) Membership() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			Fail(c, http.StatusUnauthorized, "unauthorized")
			return
		}
		ref := c.Param("ws")
		var ws model.Workspace
		q := a.DB
		if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
			q = q.Where("id = ?", id)
		} else {
			q = q.Where("name = ?", ref)
		}
		if err := q.First(&ws).Error; err != nil {
			Fail(c, http.StatusNotFound, "workspace not found")
			return
		}
		var m model.Membership
		err := a.DB.Where("workspace_id = ? AND user_id = ?", ws.ID, u.ID).First(&m).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if !u.Admin {
				Fail(c, http.StatusNotFound, "workspace not found")
				return
			}
			m = model.Membership{WorkspaceID: ws.ID, UserID: u.ID, Role: model.RoleAdmin}
		} else if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.Set(ctxWorkspace, &ws)
		c.Set(ctxMembership, &m)
		c.Next()
	}
}

// RequireRole allows workspace admins, or members when role is "member".
func (a *Auth) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		m := CurrentMembership(c)
		if m == nil || (role == model.RoleAdmin && m.Role != model.RoleAdmin) {
			Fail(c, http.StatusForbidden, "workspace admin only")
			return
		}
		c.Next()
	}
}

// Agent requires a valid agent key and records when the agent was last seen.
func (a *Auth) Agent() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := BearerToken(c)
		if key == "" {
			Fail(c, http.StatusUnauthorized, "missing Authorization: Bearer <agent key>")
			return
		}
		var ag model.Agent
		if err := a.DB.Where("key_hash = ?", util.Hash(key)).First(&ag).Error; err != nil {
			Fail(c, http.StatusUnauthorized, "invalid agent key")
			return
		}
		now := a.Now()
		a.DB.Model(&ag).UpdateColumn("last_seen_at", now)
		c.Set(ctxAgent, &ag)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) *model.User {
	v, _ := c.Get(ctxUser)
	u, _ := v.(*model.User)
	return u
}

func CurrentWorkspace(c *gin.Context) *model.Workspace {
	v, _ := c.Get(ctxWorkspace)
	w, _ := v.(*model.Workspace)
	return w
}

func CurrentMembership(c *gin.Context) *model.Membership {
	v, _ := c.Get(ctxMembership)
	m, _ := v.(*model.Membership)
	return m
}

func CurrentAgent(c *gin.Context) *model.Agent {
	v, _ := c.Get(ctxAgent)
	a, _ := v.(*model.Agent)
	return a
}
