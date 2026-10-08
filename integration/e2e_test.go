// Package integration runs the real agent worker against the real server in
// one process: bootstrap, queue a task, let the worker claim and execute it,
// and read the result back through the user API.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkEo/g-mk8s/agent/client"
	"github.com/kkEo/g-mk8s/agent/worker"
	"github.com/kkEo/g-mk8s/webapp/api"
	"github.com/kkEo/g-mk8s/webapp/db"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/webapp/util"
	"github.com/kkEo/g-mk8s/wire"
)

type stack struct {
	t      *testing.T
	url    string
	token  string
	ws     string
	worker *worker.Worker
}

func newStack(t *testing.T) *stack {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Open(db.Options{Path: ":memory:"})
	require.NoError(t, err)
	key, _ := util.NewKey()
	box, _ := util.NewBox(key)
	srv := api.New(api.Config{DB: database, Box: box, Lease: 5 * time.Second})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	boot, err := api.Bootstrap(database, "admin", "", "acme")
	require.NoError(t, err)
	s := &stack{t: t, url: ts.URL, token: boot.Token, ws: "acme"}

	reg := s.call("POST", "/api/workspaces/acme/registration-tokens", nil)["token"].(string)
	resp, err := client.New(ts.URL, "").Register(context.Background(), wire.RegisterRequest{Token: reg, Name: "e2e"})
	require.NoError(t, err)
	s.worker = &worker.Worker{Client: client.New(ts.URL, resp.Key), WorkDir: t.TempDir(), Poll: 50 * time.Millisecond, Logf: t.Logf}
	return s
}

func (s *stack) call(method, path string, body interface{}) map[string]interface{} {
	s.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, s.url+path, r)
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(s.t, err)
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	require.Less(s.t, resp.StatusCode, 300, "%s %s: %s", method, path, data)
	var m map[string]interface{}
	if len(data) > 0 && data[0] == '{' {
		require.NoError(s.t, json.Unmarshal(data, &m))
	}
	return m
}

func (s *stack) text(path string) string {
	req, _ := http.NewRequest("GET", s.url+path, nil)
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(s.t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func (s *stack) task(id float64) model.Task {
	raw := s.text(fmt.Sprintf("/api/workspaces/acme/tasks/%.0f", id))
	var tk model.Task
	require.NoError(s.t, json.Unmarshal([]byte(raw), &tk), raw)
	return tk
}

func TestShellTaskEndToEnd(t *testing.T) {
	s := newStack(t)
	s.call("PUT", "/api/workspaces/acme/secrets/api-token", map[string]string{"value": "s3cr3t-value"})
	created := s.call("POST", "/api/workspaces/acme/tasks", map[string]interface{}{
		"name": "hello", "kind": "shell",
		"params":  map[string]string{"script": "echo \"token is $API_TOKEN\"; echo to-stderr 1>&2; echo task=$GORAN_TASK_ID"},
		"secrets": map[string]string{"API_TOKEN": "api-token"},
	})
	id := created["id"].(float64)

	handled, err := s.worker.RunOnce(context.Background())
	require.NoError(t, err)
	assert.True(t, handled)

	tk := s.task(id)
	assert.Equal(t, model.StatusDone, tk.Status)
	require.NotNil(t, tk.ExitCode)
	assert.Equal(t, 0, *tk.ExitCode)
	log := s.text(fmt.Sprintf("/api/workspaces/acme/tasks/%.0f/log", id))
	assert.Contains(t, log, "token is s3cr3t-value")
	assert.Contains(t, log, "to-stderr")
	assert.Contains(t, log, fmt.Sprintf("task=%.0f", id))

	handled, err = s.worker.RunOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, handled, "queue is empty afterwards")
}

func TestFailingAndTimingOutTasks(t *testing.T) {
	s := newStack(t)
	failing := s.call("POST", "/api/workspaces/acme/tasks", map[string]interface{}{
		"name": "boom", "kind": "shell", "params": map[string]string{"script": "echo bad; exit 7"},
	})["id"].(float64)
	slow := s.call("POST", "/api/workspaces/acme/tasks", map[string]interface{}{
		"name": "slow", "kind": "shell", "params": map[string]string{"script": "sleep 30"}, "timeout_seconds": 1,
	})["id"].(float64)

	for i := 0; i < 2; i++ {
		handled, err := s.worker.RunOnce(context.Background())
		require.NoError(t, err)
		require.True(t, handled)
	}
	f := s.task(failing)
	assert.Equal(t, model.StatusError, f.Status)
	assert.Equal(t, "exit status 7", f.Error)
	require.NotNil(t, f.ExitCode)
	assert.Equal(t, 7, *f.ExitCode)

	sl := s.task(slow)
	assert.Equal(t, model.StatusError, sl.Status)
	assert.Contains(t, sl.Error, "timed out after 1s")
}

func TestTerraformPlanApproveApplyEndToEnd(t *testing.T) {
	s := newStack(t)
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls.log")
	fake := "#!/bin/sh\necho \"terraform $*\"\necho \"$1\" >> " + calls + "\n" +
		"for a in \"$@\"; do case $a in -out=*) : > \"${a#-out=}\";; esac; done\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "terraform"), []byte(fake), 0o755))
	src := t.TempDir()

	id := s.call("POST", "/api/workspaces/acme/tasks", map[string]interface{}{
		"name": "vpc", "kind": "terraform",
		"params": map[string]interface{}{"source": map[string]string{"dir": src}, "binary": filepath.Join(bin, "terraform")},
	})["id"].(float64)

	handled, err := s.worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, handled)
	tk := s.task(id)
	assert.Equal(t, model.StatusAwaitingApproval, tk.Status)
	assert.Equal(t, "init\nplan\n", readFile(t, calls))

	handled, err = s.worker.RunOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, handled, "nothing to run while waiting for approval")

	s.call("POST", fmt.Sprintf("/api/workspaces/acme/tasks/%.0f/approve", id), nil)
	handled, err = s.worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, handled)
	tk = s.task(id)
	assert.Equal(t, model.StatusDone, tk.Status)
	assert.Equal(t, "init\nplan\napply\n", readFile(t, calls))
	log := s.text(fmt.Sprintf("/api/workspaces/acme/tasks/%.0f/log", id))
	assert.True(t, strings.Contains(log, "waiting for approval") && strings.Contains(log, "applying approved plan"), log)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

// The default work directory is relative ("work"). Child processes run inside
// the source checkout, so every path the agent hands them must be absolute or
// git clones and plan files land in the wrong place.
func TestRelativeWorkDirWithGitSource(t *testing.T) {
	s := newStack(t)
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { _ = os.Chdir(old) })
	s.worker.WorkDir = "work"

	repo := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repo, "main.tf"), []byte("# nothing to see\n"), 0o644))
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "."},
		{"-c", "user.email=e2e@example.com", "-c", "user.name=e2e", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	bin := t.TempDir()
	fake := "#!/bin/sh\nfor a in \"$@\"; do case $a in -out=*) : > \"${a#-out=}\";; esac; done\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "terraform"), []byte(fake), 0o755))

	id := s.call("POST", "/api/workspaces/acme/tasks", map[string]interface{}{
		"name": "clone", "kind": "terraform",
		"params": map[string]interface{}{
			"source": map[string]string{"git": repo}, "binary": filepath.Join(bin, "terraform"), "auto_approve": true,
		},
	})["id"].(float64)
	handled, err := s.worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, handled)
	tk := s.task(id)
	assert.Equal(t, model.StatusDone, tk.Status, "task error: %s", tk.Error)
	_, err = os.Stat("work")
	assert.NoError(t, err, "the work directory is created relative to the agent's directory")
}
