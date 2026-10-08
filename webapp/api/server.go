// Package api wires the HTTP handlers of the Goran control plane.
package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/webapp/ui"
	"github.com/kkEo/g-mk8s/webapp/util"
)

// Config is everything the server needs. Zero values get sensible defaults.
type Config struct {
	DB *gorm.DB
	// Box encrypts secrets at rest. When nil the secrets endpoints answer 503.
	Box *util.Box
	// Lease is how long an agent may stay silent before its task is requeued.
	Lease time.Duration
	// MaxAttempts caps how many times a task is handed out before it errors.
	MaxAttempts int
	// DefaultTimeout bounds a task's execution when the task does not say.
	DefaultTimeout time.Duration
	// Now is injectable for tests.
	Now func() time.Time
}

func (c *Config) defaults() {
	if c.Lease == 0 {
		c.Lease = 60 * time.Second
	}
	if c.MaxAttempts == 0 {
		c.MaxAttempts = 3
	}
	if c.DefaultTimeout == 0 {
		c.DefaultTimeout = time.Hour
	}
	if c.Now == nil {
		c.Now = time.Now
	}
}

// Server owns the router and the background reaper.
type Server struct {
	cfg    Config
	db     *gorm.DB
	router *gin.Engine
}

// New builds the server. It has no side effects beyond route registration.
func New(cfg Config) *Server {
	cfg.defaults()
	s := &Server{cfg: cfg, db: cfg.DB}
	s.router = s.buildRouter()
	return s
}

// Router exposes the gin engine (tests and main use it).
func (s *Server) Router() *gin.Engine { return s.router }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) buildRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	auth := &middleware.Auth{DB: s.db, Now: s.cfg.Now}

	r.GET("/", ui.Index)
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.POST("/agent/register", s.registerAgent)

	api := r.Group("/api", auth.User())
	api.GET("/me", s.me)
	api.GET("/users", auth.RequireAdmin(), s.listUsers)
	api.POST("/users", auth.RequireAdmin(), s.createUser)
	api.POST("/users/:name/tokens", auth.RequireAdmin(), s.createTokenForUser)
	api.GET("/tokens", s.listTokens)
	api.POST("/tokens", s.createToken)
	api.DELETE("/tokens/:id", s.deleteToken)
	api.GET("/workspaces", s.listWorkspaces)
	api.POST("/workspaces", s.createWorkspace)

	ws := api.Group("/workspaces/:ws", auth.Membership())
	ws.GET("", s.getWorkspace)
	ws.GET("/members", s.listMembers)
	ws.POST("/members", auth.RequireRole(model.RoleAdmin), s.addMember)
	ws.GET("/secrets", s.listSecrets)
	ws.PUT("/secrets/:name", auth.RequireRole(model.RoleAdmin), s.putSecret)
	ws.DELETE("/secrets/:name", auth.RequireRole(model.RoleAdmin), s.deleteSecret)
	ws.POST("/registration-tokens", auth.RequireRole(model.RoleAdmin), s.createRegistrationToken)
	ws.GET("/agents", s.listAgents)
	ws.DELETE("/agents/:id", auth.RequireRole(model.RoleAdmin), s.deleteAgent)
	ws.GET("/tasks", s.listTasks)
	ws.POST("/tasks", s.createTask)
	ws.GET("/tasks/:id", s.getTask)
	ws.GET("/tasks/:id/log", s.getTaskLog)
	ws.POST("/tasks/:id/approve", auth.RequireRole(model.RoleAdmin), s.approveTask)
	ws.POST("/tasks/:id/reject", auth.RequireRole(model.RoleAdmin), s.rejectTask)
	ws.POST("/tasks/:id/cancel", s.cancelTask)

	ag := r.Group("/agent", auth.Agent())
	ag.GET("/next", s.nextTask)
	ag.POST("/tasks/:id/heartbeat", s.heartbeat)
	ag.POST("/tasks/:id/logs", s.appendLog)
	ag.POST("/tasks/:id/result", s.reportResult)
	return r
}

// StartReaper requeues tasks with expired leases every interval until ctx ends,
// so the UI reflects lost agents even when no agent is polling.
func (s *Server) StartReaper(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := s.RequeueExpired(s.cfg.Now()); err != nil {
					log.Printf("reaper: %v", err)
				} else if n > 0 {
					log.Printf("reaper: requeued %d task(s) with expired leases", n)
				}
			}
		}
	}()
}
