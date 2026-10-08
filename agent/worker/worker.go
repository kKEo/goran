// Package worker is the agent's main loop: claim a task, run it, stream its
// log, keep the lease alive and report the result.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kkEo/g-mk8s/agent/client"
	"github.com/kkEo/g-mk8s/agent/logsink"
	"github.com/kkEo/g-mk8s/agent/runner"
	"github.com/kkEo/g-mk8s/wire"
)

// Worker runs tasks one at a time.
type Worker struct {
	Client  *client.Client
	WorkDir string
	Poll    time.Duration
	Logf    func(format string, args ...interface{})
}

func (w *Worker) logf(format string, args ...interface{}) {
	if w.Logf != nil {
		w.Logf(format, args...)
	}
}

// Run polls until ctx is done. After a task it polls again immediately.
func (w *Worker) Run(ctx context.Context) error {
	if w.Poll <= 0 {
		w.Poll = 2 * time.Second
	}
	if abs, err := filepath.Abs(w.WorkDir); err == nil {
		w.WorkDir = abs
	}
	w.logf("polling %s every %s, workdir %s", w.Client.BaseURL, w.Poll, w.WorkDir)
	for {
		handled, err := w.RunOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			w.logf("poll: %v", err)
		}
		if handled {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(w.Poll):
		}
	}
}

// RunOnce claims and executes at most one task.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	task, err := w.Client.Next(ctx)
	if err != nil {
		return false, err
	}
	if task == nil {
		return false, nil
	}
	w.execute(ctx, task)
	return true, nil
}

func serverCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 20*time.Second)
}

func (w *Worker) execute(parent context.Context, task *wire.AgentTask) {
	w.logf("task %d %q (%s/%s, attempt %d/%d): starting", task.ID, task.Name, task.Kind, task.Phase, task.Attempt, task.MaxAttempts)
	timeout := time.Duration(task.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Hour
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var leaseLost atomic.Bool
	loseLease := func() {
		leaseLost.Store(true)
		cancel()
	}

	sink := logsink.New(func(chunk string) error {
		sctx, done := serverCtx()
		defer done()
		_, err := w.Client.AppendLog(sctx, task.ID, chunk)
		if errors.Is(err, client.ErrLeaseLost) {
			loseLease()
		}
		return err
	}, time.Second, 8*1024)
	sink.Start()

	lease := time.Duration(task.LeaseSeconds) * time.Second
	if lease <= 0 {
		lease = time.Minute
	}
	hbStop := make(chan struct{})
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		t := time.NewTicker(lease / 3)
		defer t.Stop()
		for {
			select {
			case <-hbStop:
				return
			case <-t.C:
				sctx, done := serverCtx()
				_, err := w.Client.Heartbeat(sctx, task.ID)
				done()
				if errors.Is(err, client.ErrLeaseLost) {
					w.logf("task %d: lease lost, stopping", task.ID)
					loseLease()
					return
				}
				if err != nil {
					w.logf("task %d: heartbeat: %v", task.ID, err)
				}
			}
		}
	}()

	// Child processes run inside the source checkout, not in the agent's
	// directory, so every path handed to them must be absolute even when
	// WorkDir is relative (the default "work").
	taskDir := filepath.Join(w.WorkDir, strconv.FormatUint(uint64(task.ID), 10))
	if abs, err := filepath.Abs(taskDir); err == nil {
		taskDir = abs
	}
	var outcome runner.Outcome
	err := os.MkdirAll(taskDir, 0o755)
	if err == nil {
		var r runner.Runner
		if r, err = runner.For(task.Kind); err == nil {
			rc := &runner.Context{Task: task, Dir: taskDir, Out: sink, Env: buildEnv(task)}
			outcome, err = r.Run(ctx, rc)
		}
	}

	close(hbStop)
	<-hbDone
	if ferr := sink.Close(); ferr != nil {
		w.logf("task %d: could not ship the last log chunk: %v", task.ID, ferr)
	}
	if leaseLost.Load() {
		w.logf("task %d: lease lost; result discarded", task.ID)
		return
	}

	res := wire.ResultRequest{}
	switch {
	case parent.Err() != nil:
		res.Status = wire.ResultError
		res.Error = "agent shut down while the task was running"
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		res.Status = wire.ResultError
		res.Error = fmt.Sprintf("timed out after %s", timeout)
	case err != nil:
		res.Status = wire.ResultError
		res.Error = err.Error()
	case outcome.ExitCode != 0:
		code := outcome.ExitCode
		res.Status = wire.ResultError
		res.Error = fmt.Sprintf("exit status %d", code)
		res.ExitCode = &code
	case outcome.AwaitApproval:
		res.Status = wire.ResultAwaitingApproval
	default:
		zero := 0
		res.Status = wire.ResultDone
		res.ExitCode = &zero
	}
	w.report(task.ID, res)
	w.logf("task %d: %s %s", task.ID, res.Status, res.Error)

	if res.Status != wire.ResultAwaitingApproval && !keepWorkdir(task) {
		_ = os.RemoveAll(taskDir)
	}
}

// report retries a few times so a blip does not turn a finished task into a
// lease-expiry retry.
func (w *Worker) report(id uint, res wire.ResultRequest) {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		sctx, done := serverCtx()
		err = w.Client.Result(sctx, id, res)
		done()
		if err == nil || errors.Is(err, client.ErrLeaseLost) || errors.Is(err, client.ErrUnauthorized) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		w.logf("task %d: could not report result: %v", id, err)
	}
}

// buildEnv is the agent's environment plus task metadata plus the task's
// secrets, which win over everything else.
func buildEnv(task *wire.AgentTask) []string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	env["GORAN_TASK_ID"] = strconv.FormatUint(uint64(task.ID), 10)
	env["GORAN_TASK_NAME"] = task.Name
	env["GORAN_TASK_KIND"] = task.Kind
	env["GORAN_TASK_PHASE"] = task.Phase
	for k, v := range task.Env {
		env[k] = v
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func keepWorkdir(task *wire.AgentTask) bool {
	var p struct {
		Keep bool `json:"keep_workdir"`
	}
	_ = json.Unmarshal(task.Params, &p)
	return p.Keep
}
