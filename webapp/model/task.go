package model

import "time"

// TaskStatus is the task state machine:
//
//	new -> pending -> running -> done | error | awaiting_approval
//	awaiting_approval -> approved -> pending -> running -> done | error
//	awaiting_approval -> rejected
//	new | awaiting_approval -> canceled
//
// A task in pending, running or approved holds a lease. When the lease expires
// the task goes back to new (or to error once max_attempts is reached).
type TaskStatus string

const (
	StatusNew              TaskStatus = "new"
	StatusPending          TaskStatus = "pending"
	StatusRunning          TaskStatus = "running"
	StatusAwaitingApproval TaskStatus = "awaiting_approval"
	StatusApproved         TaskStatus = "approved"
	StatusDone             TaskStatus = "done"
	StatusError            TaskStatus = "error"
	StatusRejected         TaskStatus = "rejected"
	StatusCanceled         TaskStatus = "canceled"
)

// Terminal reports whether no further transitions are possible.
func (s TaskStatus) Terminal() bool {
	switch s {
	case StatusDone, StatusError, StatusRejected, StatusCanceled:
		return true
	}
	return false
}

// Leased lists the statuses that carry a lease and are subject to requeueing.
var Leased = []TaskStatus{StatusPending, StatusRunning, StatusApproved}

// Task is a unit of work an agent executes. Params is kind-specific JSON;
// Secrets maps environment variable names to workspace secret names.
type Task struct {
	Base
	WorkspaceID    uint       `gorm:"index" json:"workspace_id"`
	Name           string     `gorm:"size:128" json:"name"`
	Kind           string     `gorm:"size:32" json:"kind"`
	Params         JSON       `gorm:"type:text" json:"params"`
	Secrets        StringMap  `gorm:"type:text" json:"secrets"`
	Labels         Labels     `gorm:"type:text" json:"labels"`
	TimeoutSeconds int        `json:"timeout_seconds"`
	Status         TaskStatus `gorm:"size:32;index" json:"status"`
	Phase          string     `gorm:"size:16" json:"phase"`
	AgentID        *uint      `gorm:"index" json:"agent_id"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at"`
	Attempts       int        `json:"attempts"`
	MaxAttempts    int        `json:"max_attempts"`
	ExitCode       *int       `json:"exit_code"`
	Error          string     `json:"error"`
	CreatedByID    uint       `json:"created_by_id"`
	ApprovedByID   *uint      `json:"approved_by_id"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}

// TaskLog is one appended chunk of a task's combined output.
type TaskLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	TaskID    uint      `gorm:"index" json:"task_id"`
	Chunk     string    `json:"chunk"`
	CreatedAt time.Time `json:"created_at"`
}

// All lists every table for AutoMigrate.
func All() []interface{} {
	return []interface{}{
		&User{}, &Workspace{}, &Membership{}, &ApiToken{},
		&RegistrationToken{}, &Agent{}, &Secret{}, &Task{}, &TaskLog{},
	}
}
