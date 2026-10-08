// Package wire holds the JSON types exchanged between the Goran server and its agents.
// Both binaries import it so the protocol is defined exactly once.
package wire

import (
	"encoding/json"
	"time"
)

// Task kinds understood by agents.
const (
	KindShell     = "shell"
	KindTerraform = "terraform"
	KindAnsible   = "ansible"
)

// Phases the server assigns when it hands a task to an agent.
const (
	PhaseRun   = "run"   // shell and ansible: a single execution
	PhasePlan  = "plan"  // terraform: init + plan, then wait for approval unless auto_approve
	PhaseApply = "apply" // terraform: apply the plan saved during PhasePlan
)

// Result statuses an agent may report back.
const (
	ResultDone             = "done"
	ResultError            = "error"
	ResultAwaitingApproval = "awaiting_approval"
)

// AgentTask is the payload returned by GET /agent/next. It is the only place
// where decrypted secret values travel; they are handed to the agent that holds
// the lease and never appear in any user-facing response.
type AgentTask struct {
	ID             uint              `json:"id"`
	Name           string            `json:"name"`
	Kind           string            `json:"kind"`
	Phase          string            `json:"phase"`
	Params         json.RawMessage   `json:"params"`
	Env            map[string]string `json:"env,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	LeaseSeconds   int               `json:"lease_seconds"`
	Attempt        int               `json:"attempt"`
	MaxAttempts    int               `json:"max_attempts"`
}

// RegisterRequest is sent once by a new agent to POST /agent/register.
type RegisterRequest struct {
	Token  string   `json:"token"`
	Name   string   `json:"name"`
	Labels []string `json:"labels"`
}

// RegisterResponse carries the agent's permanent key. It is shown exactly once.
type RegisterResponse struct {
	AgentID     uint   `json:"agent_id"`
	WorkspaceID uint   `json:"workspace_id"`
	Name        string `json:"name"`
	Key         string `json:"key"`
}

// LogRequest appends a chunk of combined stdout/stderr to the task log.
type LogRequest struct {
	Chunk string `json:"chunk"`
}

// LeaseResponse is returned by heartbeat and log calls.
type LeaseResponse struct {
	Status         string    `json:"status"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

// ResultRequest finishes (or parks) a task.
type ResultRequest struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ErrorResponse is the body of every non-2xx reply.
type ErrorResponse struct {
	Error string `json:"error"`
}
