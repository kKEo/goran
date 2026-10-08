// Package runner turns a claimed task into child processes. One Runner per
// task kind; all of them write their combined output to Context.Out.
package runner

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/kkEo/g-mk8s/agent/process"
	"github.com/kkEo/g-mk8s/wire"
)

// Context is what a runner gets for one task.
type Context struct {
	Task *wire.AgentTask
	// Dir is the per-task working directory. It survives between the plan and
	// apply phases of a terraform task.
	Dir string
	// Out receives everything the task prints.
	Out io.Writer
	// Env is the complete environment for child processes (secrets included).
	Env []string
}

// Outcome is what a runner reports when it did not fail outright.
type Outcome struct {
	ExitCode      int
	AwaitApproval bool
}

// Runner executes one kind of task.
type Runner interface {
	Run(ctx context.Context, rc *Context) (Outcome, error)
}

// For picks the runner for a kind.
func For(kind string) (Runner, error) {
	switch kind {
	case wire.KindShell:
		return &Shell{}, nil
	case wire.KindTerraform:
		return &Terraform{}, nil
	case wire.KindAnsible:
		return &Ansible{}, nil
	}
	return nil, fmt.Errorf("unsupported task kind %q", kind)
}

// Printf writes an agent-side note into the task log.
func (rc *Context) Printf(format string, args ...interface{}) {
	fmt.Fprintf(rc.Out, "[goran] "+format+"\n", args...)
}

// run executes a command in dir and echoes the command line (never secret
// values: those travel through the environment) into the log.
func (rc *Context) run(ctx context.Context, dir, name string, args ...string) (int, error) {
	rc.Printf("$ %s %s", name, strings.Join(args, " "))
	return process.Run(ctx, process.Spec{Command: name, Args: args, Dir: dir, Env: rc.Env, Output: rc.Out})
}

// runQuiet is run without echoing the command line.
func (rc *Context) runQuiet(ctx context.Context, dir, name string, args ...string) (int, error) {
	return process.Run(ctx, process.Spec{Command: name, Args: args, Dir: dir, Env: rc.Env, Output: rc.Out})
}

// withEnv returns a copy of rc with extra environment entries appended.
func (rc *Context) withEnv(extra ...string) *Context {
	cp := *rc
	cp.Env = append(append([]string(nil), rc.Env...), extra...)
	return &cp
}
