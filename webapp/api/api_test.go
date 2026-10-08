package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkEo/g-mk8s/webapp/db"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/webapp/util"
	"github.com/kkEo/g-mk8s/wire"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

type env struct {
	t     *testing.T
	srv   *Server
	clock *fakeClock
	admin string
	ws    string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := db.Open(db.Options{Path: ":memory:"})
	require.NoError(t, err)
	key, err := util.NewKey()
	require.NoError(t, err)
	box, err := util.NewBox(key)
	require.NoError(t, err)
	clock := &fakeClock{t: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}
	srv := New(Config{DB: database, Box: box, Lease: 60 * time.Second, MaxAttempts: 2, Now: clock.Now})
	res, err := Bootstrap(database, "admin", "admin@example.com", "acme")
	require.NoError(t, err)
	return &env{t: t, srv: srv, clock: clock, admin: res.Token, ws: "acme"}
}

func (e *env) do(method, path, token string, body interface{}) *httptest.ResponseRecorder {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		switch b := body.(type) {
		case string:
			r = strings.NewReader(b)
		default:
			j, err := json.Marshal(b)
			require.NoError(e.t, err)
			r = bytes.NewReader(j)
		}
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.srv.ServeHTTP(w, req)
	return w
}

func (e *env) obj(w *httptest.ResponseRecorder) map[string]interface{} {
	e.t.Helper()
	var m map[string]interface{}
	require.NoError(e.t, json.Unmarshal(w.Body.Bytes(), &m), w.Body.String())
	return m
}

func (e *env) wsPath(suffix string) string { return "/api/workspaces/" + e.ws + suffix }

func (e *env) registerAgent(name string, labels ...string) string {
	e.t.Helper()
	w := e.do("POST", e.wsPath("/registration-tokens"), e.admin, map[string]int{"ttl_minutes": 30})
	require.Equal(e.t, 201, w.Code, w.Body.String())
	reg := e.obj(w)["token"].(string)
	w = e.do("POST", "/agent/register", "", wire.RegisterRequest{Token: reg, Name: name, Labels: labels})
	require.Equal(e.t, 201, w.Code, w.Body.String())
	key := e.obj(w)["key"].(string)
	require.True(e.t, strings.HasPrefix(key, "ga_"))
	return key
}

func (e *env) createTask(token string, body map[string]interface{}) uint {
	e.t.Helper()
	w := e.do("POST", e.wsPath("/tasks"), token, body)
	require.Equal(e.t, 201, w.Code, w.Body.String())
	return uint(e.obj(w)["id"].(float64))
}

func (e *env) next(agentKey string) (*wire.AgentTask, int) {
	e.t.Helper()
	w := e.do("GET", "/agent/next", agentKey, nil)
	if w.Code != 200 {
		return nil, w.Code
	}
	var t wire.AgentTask
	require.NoError(e.t, json.Unmarshal(w.Body.Bytes(), &t))
	return &t, w.Code
}

func (e *env) task(id uint) model.Task {
	e.t.Helper()
	w := e.do("GET", e.wsPath(fmt.Sprintf("/tasks/%d", id)), e.admin, nil)
	require.Equal(e.t, 200, w.Code, w.Body.String())
	var t model.Task
	require.NoError(e.t, json.Unmarshal(w.Body.Bytes(), &t))
	return t
}

func (e *env) userToken(name string) string {
	e.t.Helper()
	w := e.do("POST", "/api/users", e.admin, map[string]interface{}{"name": name, "email": name + "@example.com"})
	require.Equal(e.t, 201, w.Code, w.Body.String())
	w = e.do("POST", "/api/users/"+name+"/tokens", e.admin, map[string]string{"name": "cli"})
	require.Equal(e.t, 201, w.Code, w.Body.String())
	return e.obj(w)["token"].(string)
}

func shellTask(name, script string) map[string]interface{} {
	return map[string]interface{}{"name": name, "kind": "shell", "params": map[string]string{"script": script}}
}

func TestAuthRequired(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 401, e.do("GET", "/api/me", "", nil).Code)
	assert.Equal(t, 401, e.do("GET", "/api/me", "gu_nope", nil).Code)
	assert.Equal(t, 401, e.do("GET", "/agent/next", e.admin, nil).Code, "a user token is not an agent key")
	w := e.do("GET", "/api/me", e.admin, nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"admin":true`)
}

func TestTokensAreHashedAndNeverListed(t *testing.T) {
	e := newEnv(t)
	w := e.do("POST", "/api/tokens", e.admin, map[string]string{"name": "laptop"})
	require.Equal(t, 201, w.Code)
	raw := e.obj(w)["token"].(string)
	assert.True(t, strings.HasPrefix(raw, "gu_"))

	var stored model.ApiToken
	require.NoError(t, e.srv.db.Where("name = ?", "laptop").First(&stored).Error)
	assert.NotEqual(t, raw, stored.Hash)
	assert.Equal(t, util.Hash(raw), stored.Hash)

	w = e.do("GET", "/api/tokens", e.admin, nil)
	assert.Equal(t, 200, w.Code)
	assert.NotContains(t, w.Body.String(), raw)
	assert.Contains(t, w.Body.String(), `"prefix":"`+raw[:10]+`"`)

	assert.Equal(t, 200, e.do("GET", "/api/me", raw, nil).Code, "new token works")
	w = e.do("DELETE", fmt.Sprintf("/api/tokens/%d", stored.ID), raw, nil)
	assert.Equal(t, 204, w.Code)
	assert.Equal(t, 401, e.do("GET", "/api/me", raw, nil).Code, "deleted token is rejected")
}

func TestTenancyAndRoles(t *testing.T) {
	e := newEnv(t)
	bob := e.userToken("bob")
	assert.Equal(t, 403, e.do("POST", "/api/users", bob, map[string]string{"name": "eve"}).Code)
	assert.Equal(t, 403, e.do("POST", "/api/users/admin/tokens", bob, nil).Code)

	w := e.do("GET", "/api/workspaces", bob, nil)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "[]", w.Body.String())
	assert.Equal(t, 404, e.do("GET", e.wsPath("/tasks"), bob, nil).Code, "non-member sees no workspace")

	w = e.do("POST", e.wsPath("/members"), e.admin, map[string]string{"user": "bob", "role": "member"})
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, 200, e.do("GET", e.wsPath("/tasks"), bob, nil).Code)
	id := e.createTask(bob, shellTask("bob-task", "true"))
	assert.Equal(t, 403, e.do("PUT", e.wsPath("/secrets/x"), bob, map[string]string{"value": "v"}).Code, "members cannot write secrets")
	assert.Equal(t, 403, e.do("POST", e.wsPath("/registration-tokens"), bob, nil).Code)
	assert.Equal(t, 403, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/approve", id)), bob, nil).Code)

	w = e.do("POST", "/api/workspaces", bob, map[string]string{"name": "bobs"})
	require.Equal(t, 201, w.Code)
	assert.Equal(t, 200, e.do("GET", "/api/workspaces/bobs", bob, nil).Code, "creator is admin of the new workspace")
	assert.Equal(t, 200, e.do("GET", "/api/workspaces/bobs", e.admin, nil).Code, "global admin sees every workspace")
	assert.Equal(t, 409, e.do("POST", "/api/workspaces", e.admin, map[string]string{"name": "bobs"}).Code)
	assert.Equal(t, 400, e.do("POST", "/api/workspaces", e.admin, map[string]string{"name": "123"}).Code, "numeric names are reserved for ids")
}

func TestCreateTaskValidation(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		name string
		body map[string]interface{}
		want string
	}{
		{"no name", map[string]interface{}{"kind": "shell", "params": map[string]string{"script": "true"}}, "name is required"},
		{"empty script", map[string]interface{}{"name": "x", "kind": "shell", "params": map[string]string{"script": " "}}, "needs a non-empty"},
		{"unknown kind", map[string]interface{}{"name": "x", "kind": "docker"}, "unknown task kind"},
		{"terraform without source", map[string]interface{}{"name": "x", "kind": "terraform", "params": map[string]string{}}, "must be an object with"},
		{"ansible without playbook", map[string]interface{}{"name": "x", "kind": "ansible", "params": map[string]interface{}{"source": map[string]string{"dir": "/tmp"}}}, "ansible task needs"},
		{"missing secret", map[string]interface{}{"name": "x", "kind": "shell", "params": map[string]string{"script": "true"}, "secrets": map[string]string{"TOKEN": "nope"}}, "does not exist in this workspace"},
		{"bad env name", map[string]interface{}{"name": "x", "kind": "shell", "params": map[string]string{"script": "true"}, "secrets": map[string]string{"bad-name": "nope"}}, "not a valid environment variable"},
	}
	for _, tc := range cases {
		w := e.do("POST", e.wsPath("/tasks"), e.admin, tc.body)
		assert.Equal(t, 400, w.Code, tc.name)
		assert.Contains(t, w.Body.String(), tc.want, tc.name)
	}
	w := e.do("POST", e.wsPath("/tasks"), e.admin, `{"name": "x", "kind": "shell", "params": {"script": "echo hi"}, "labels": ["eu", "eu", " prod "]}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	got := e.obj(w)
	assert.Equal(t, "new", got["status"])
	assert.Equal(t, []interface{}{"eu", "prod"}, got["labels"])
	assert.EqualValues(t, 3600, got["timeout_seconds"])
	assert.EqualValues(t, 2, got["max_attempts"])
}

func TestAgentRegistration(t *testing.T) {
	e := newEnv(t)
	w := e.do("POST", e.wsPath("/registration-tokens"), e.admin, nil)
	require.Equal(t, 201, w.Code, w.Body.String())
	reg := e.obj(w)["token"].(string)
	assert.True(t, strings.HasPrefix(reg, "gr_"))

	w = e.do("POST", "/agent/register", "", wire.RegisterRequest{Token: reg, Name: "runner-1", Labels: []string{"eu"}})
	require.Equal(t, 201, w.Code, w.Body.String())
	key := e.obj(w)["key"].(string)
	assert.Equal(t, 401, e.do("POST", "/agent/register", "", wire.RegisterRequest{Token: reg, Name: "runner-2"}).Code, "single use")
	assert.Equal(t, 401, e.do("POST", "/agent/register", "", wire.RegisterRequest{Token: "gr_bogus", Name: "runner-2"}).Code)

	w = e.do("POST", e.wsPath("/registration-tokens"), e.admin, map[string]int{"ttl_minutes": 5})
	expired := e.obj(w)["token"].(string)
	e.clock.Advance(6 * time.Minute)
	assert.Equal(t, 401, e.do("POST", "/agent/register", "", wire.RegisterRequest{Token: expired, Name: "late"}).Code, "expired")

	assert.Equal(t, 204, e.do("GET", "/agent/next", key, nil).Code)
	assert.Equal(t, 401, e.do("GET", "/agent/next", "ga_bogus", nil).Code)

	w = e.do("GET", e.wsPath("/agents"), e.admin, nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"runner-1"`)
	assert.NotContains(t, w.Body.String(), key)

	var ag model.Agent
	require.NoError(t, e.srv.db.First(&ag).Error)
	assert.Equal(t, 204, e.do("DELETE", e.wsPath(fmt.Sprintf("/agents/%d", ag.ID)), e.admin, nil).Code)
	assert.Equal(t, 401, e.do("GET", "/agent/next", key, nil).Code, "revoked key")
}

func TestClaimLeaseLogsAndResult(t *testing.T) {
	e := newEnv(t)
	require.Equal(t, 201, e.do("PUT", e.wsPath("/secrets/linode"), e.admin, map[string]string{"value": "tok123"}).Code)
	id := e.createTask(e.admin, map[string]interface{}{
		"name": "deploy", "kind": "shell",
		"params":  map[string]string{"script": "echo $LINODE_TOKEN"},
		"secrets": map[string]string{"LINODE_TOKEN": "linode"},
	})
	a := e.registerAgent("a")
	b := e.registerAgent("b")

	task, code := e.next(a)
	require.Equal(t, 200, code)
	assert.Equal(t, id, task.ID)
	assert.Equal(t, wire.PhaseRun, task.Phase)
	assert.Equal(t, "tok123", task.Env["LINODE_TOKEN"], "secrets are decrypted only for the claiming agent")
	assert.Equal(t, 60, task.LeaseSeconds)
	assert.Equal(t, 1, task.Attempt)

	_, code = e.next(b)
	assert.Equal(t, 204, code, "a claimed task is not handed out twice")

	stored := e.task(id)
	assert.Equal(t, model.StatusPending, stored.Status)
	require.NotNil(t, stored.AgentID)
	require.NotNil(t, stored.LeaseExpiresAt)
	assert.Equal(t, e.clock.Now().Add(60*time.Second), stored.LeaseExpiresAt.UTC())
	body := e.do("GET", e.wsPath(fmt.Sprintf("/tasks/%d", id)), e.admin, nil).Body.String()
	assert.NotContains(t, body, "tok123", "the user API never shows secret values")

	assert.Equal(t, 409, e.do("POST", fmt.Sprintf("/agent/tasks/%d/heartbeat", id), b, nil).Code, "not the owner")
	w := e.do("POST", fmt.Sprintf("/agent/tasks/%d/heartbeat", id), a, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, model.StatusRunning, e.task(id).Status)

	e.clock.Advance(30 * time.Second)
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/logs", id), a, wire.LogRequest{Chunk: "line 1\n"}).Code)
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/logs", id), a, wire.LogRequest{Chunk: "line 2\n"}).Code)
	assert.Equal(t, e.clock.Now().Add(60*time.Second), e.task(id).LeaseExpiresAt.UTC(), "logs extend the lease")
	assert.Equal(t, 413, e.do("POST", fmt.Sprintf("/agent/tasks/%d/logs", id), a, wire.LogRequest{Chunk: strings.Repeat("x", maxLogChunk+1)}).Code)

	w = e.do("GET", e.wsPath(fmt.Sprintf("/tasks/%d/log", id)), e.admin, nil)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "line 1\nline 2\n", w.Body.String())

	zero := 0
	w = e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "done", ExitCode: &zero})
	require.Equal(t, 200, w.Code, w.Body.String())
	done := e.task(id)
	assert.Equal(t, model.StatusDone, done.Status)
	assert.Nil(t, done.LeaseExpiresAt)
	assert.NotNil(t, done.FinishedAt)
	require.NotNil(t, done.ExitCode)
	assert.Equal(t, 0, *done.ExitCode)
	assert.Equal(t, 409, e.do("POST", fmt.Sprintf("/agent/tasks/%d/heartbeat", id), a, nil).Code, "finished tasks reject further writes")
	assert.Equal(t, 409, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "done"}).Code, "finished tasks reject a second result")
}

func TestLabelsRouting(t *testing.T) {
	e := newEnv(t)
	id := e.createTask(e.admin, map[string]interface{}{"name": "eu-only", "kind": "shell", "params": map[string]string{"script": "true"}, "labels": []string{"eu"}})
	plain := e.registerAgent("plain")
	eu := e.registerAgent("eu", "prod", "eu")
	_, code := e.next(plain)
	assert.Equal(t, 204, code)
	task, code := e.next(eu)
	require.Equal(t, 200, code)
	assert.Equal(t, id, task.ID)
}

func TestLeaseExpiryRequeuesThenFails(t *testing.T) {
	e := newEnv(t)
	id := e.createTask(e.admin, shellTask("flaky", "true"))
	a := e.registerAgent("a")
	b := e.registerAgent("b")

	_, code := e.next(a)
	require.Equal(t, 200, code)
	e.clock.Advance(30 * time.Second)
	_, code = e.next(b)
	assert.Equal(t, 204, code, "lease still valid")

	e.clock.Advance(31 * time.Second)
	task, code := e.next(b)
	require.Equal(t, 200, code, "expired lease hands the task to another agent")
	assert.Equal(t, id, task.ID)
	assert.Equal(t, 2, task.Attempt)
	assert.Equal(t, 409, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "done"}).Code, "the old owner can no longer report")

	e.clock.Advance(61 * time.Second)
	failed := e.task(id)
	assert.Equal(t, model.StatusError, failed.Status)
	assert.Contains(t, failed.Error, "lease expired after 2 attempt(s)")
	_, code = e.next(a)
	assert.Equal(t, 204, code)
}

func terraformTask(name string) map[string]interface{} {
	return map[string]interface{}{"name": name, "kind": "terraform", "params": map[string]interface{}{"source": map[string]string{"dir": "/srv/tf"}}}
}

func TestApprovalFlow(t *testing.T) {
	e := newEnv(t)
	bob := e.userToken("bob")
	require.Equal(t, 200, e.do("POST", e.wsPath("/members"), e.admin, map[string]string{"user": "bob", "role": "member"}).Code)
	id := e.createTask(bob, terraformTask("vpc"))
	a := e.registerAgent("a")

	task, code := e.next(a)
	require.Equal(t, 200, code)
	assert.Equal(t, wire.PhasePlan, task.Phase)
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "awaiting_approval"}).Code)
	parked := e.task(id)
	assert.Equal(t, model.StatusAwaitingApproval, parked.Status)
	assert.Nil(t, parked.LeaseExpiresAt, "no lease while a human decides")
	_, code = e.next(a)
	assert.Equal(t, 204, code, "nothing to do until approved")

	assert.Equal(t, 403, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/approve", id)), bob, nil).Code, "members cannot approve")
	assert.Equal(t, 200, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/cancel", id)), bob, nil).Code, "members may cancel while awaiting approval")
	assert.Equal(t, model.StatusCanceled, e.task(id).Status)
}

func TestApprovalFlowApplyOnSameAgent(t *testing.T) {
	e := newEnv(t)
	id := e.createTask(e.admin, terraformTask("vpc"))
	a := e.registerAgent("a")
	b := e.registerAgent("b")
	_, code := e.next(a)
	require.Equal(t, 200, code)
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "awaiting_approval"}).Code)

	w := e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/approve", id)), e.admin, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, model.StatusApproved, e.task(id).Status)
	assert.Equal(t, 409, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/approve", id)), e.admin, nil).Code, "approve twice")

	_, code = e.next(b)
	assert.Equal(t, 204, code, "the plan lives on agent a, so b must not get the apply")
	task, code := e.next(a)
	require.Equal(t, 200, code)
	assert.Equal(t, id, task.ID)
	assert.Equal(t, wire.PhaseApply, task.Phase)
	assert.Equal(t, model.StatusPending, e.task(id).Status)

	zero := 0
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "done", ExitCode: &zero}).Code)
	assert.Equal(t, model.StatusDone, e.task(id).Status)
}

func TestRejectAndCancel(t *testing.T) {
	e := newEnv(t)
	a := e.registerAgent("a")
	id := e.createTask(e.admin, terraformTask("vpc"))
	_, code := e.next(a)
	require.Equal(t, 200, code)
	assert.Equal(t, 409, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/cancel", id)), e.admin, nil).Code, "running tasks cannot be canceled")
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "awaiting_approval"}).Code)
	w := e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/reject", id)), e.admin, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, model.StatusRejected, e.task(id).Status)
	_, code = e.next(a)
	assert.Equal(t, 204, code)

	id2 := e.createTask(e.admin, shellTask("later", "true"))
	assert.Equal(t, 200, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/cancel", id2)), e.admin, nil).Code)
	assert.Equal(t, model.StatusCanceled, e.task(id2).Status)
	_, code = e.next(a)
	assert.Equal(t, 204, code)

	id3 := e.createTask(e.admin, terraformTask("vpc3"))
	_, code = e.next(a)
	require.Equal(t, 200, code)
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id3), a, wire.ResultRequest{Status: "awaiting_approval"}).Code)
	assert.Equal(t, 200, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/cancel", id3)), e.admin, nil).Code, "cancel while awaiting approval")
	assert.Equal(t, model.StatusCanceled, e.task(id3).Status)
}

func TestApprovedPickupLeaseExpiryReplans(t *testing.T) {
	e := newEnv(t)
	a := e.registerAgent("a")
	b := e.registerAgent("b")
	id := e.createTask(e.admin, terraformTask("vpc"))
	_, code := e.next(a)
	require.Equal(t, 200, code)
	require.Equal(t, 200, e.do("POST", fmt.Sprintf("/agent/tasks/%d/result", id), a, wire.ResultRequest{Status: "awaiting_approval"}).Code)
	require.Equal(t, 200, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/approve", id)), e.admin, nil).Code)

	e.clock.Advance(61 * time.Second)
	task, code := e.next(b)
	require.Equal(t, 200, code, "agent a vanished: the task is planned again on b")
	assert.Equal(t, id, task.ID)
	assert.Equal(t, wire.PhasePlan, task.Phase)
	assert.Equal(t, 2, task.Attempt)
}

func TestSecretsAreEncryptedAndHidden(t *testing.T) {
	e := newEnv(t)
	w := e.do("PUT", e.wsPath("/secrets/linode"), e.admin, map[string]string{"value": "plain-value"})
	require.Equal(t, 201, w.Code, w.Body.String())
	assert.Equal(t, 200, e.do("PUT", e.wsPath("/secrets/linode"), e.admin, map[string]string{"value": "rotated"}).Code)
	assert.Equal(t, 400, e.do("PUT", e.wsPath("/secrets/linode"), e.admin, map[string]string{"value": ""}).Code)

	var sec model.Secret
	require.NoError(t, e.srv.db.Where("name = ?", "linode").First(&sec).Error)
	assert.NotContains(t, string(sec.Ciphertext), "rotated")
	assert.NotContains(t, string(sec.Ciphertext), "plain-value")

	w = e.do("GET", e.wsPath("/secrets"), e.admin, nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"linode"`)
	assert.NotContains(t, w.Body.String(), "rotated")

	id := e.createTask(e.admin, map[string]interface{}{"name": "x", "kind": "shell", "params": map[string]string{"script": "true"}, "secrets": map[string]string{"TOKEN": "linode"}})
	assert.Equal(t, 204, e.do("DELETE", e.wsPath("/secrets/linode"), e.admin, nil).Code)
	assert.Equal(t, 404, e.do("DELETE", e.wsPath("/secrets/linode"), e.admin, nil).Code)

	a := e.registerAgent("a")
	_, code := e.next(a)
	assert.Equal(t, 204, code, "a task whose secret vanished is failed, not handed out")
	failed := e.task(id)
	assert.Equal(t, model.StatusError, failed.Status)
	assert.Contains(t, failed.Error, `secret "linode" not found`)
}

func TestWorkspaceIsolation(t *testing.T) {
	e := newEnv(t)
	require.Equal(t, 201, e.do("POST", "/api/workspaces", e.admin, map[string]string{"name": "beta"}).Code)
	e.createTask(e.admin, shellTask("acme-task", "true"))

	w := e.do("POST", "/api/workspaces/beta/registration-tokens", e.admin, nil)
	require.Equal(t, 201, w.Code)
	w = e.do("POST", "/agent/register", "", wire.RegisterRequest{Token: e.obj(w)["token"].(string), Name: "beta-agent"})
	require.Equal(t, 201, w.Code)
	betaKey := e.obj(w)["key"].(string)
	_, code := e.next(betaKey)
	assert.Equal(t, 204, code, "agents only see their own workspace's tasks")

	bob := e.userToken("bob")
	require.Equal(t, 200, e.do("POST", e.wsPath("/members"), e.admin, map[string]string{"user": "bob"}).Code)
	assert.Equal(t, 404, e.do("GET", "/api/workspaces/beta/tasks", bob, nil).Code)
	assert.Equal(t, 200, e.do("GET", "/api/workspaces/acme/tasks", bob, nil).Code)
}

func TestListTasksFilterAndOrder(t *testing.T) {
	e := newEnv(t)
	e.createTask(e.admin, shellTask("first", "true"))
	second := e.createTask(e.admin, shellTask("second", "true"))
	require.Equal(t, 200, e.do("POST", e.wsPath(fmt.Sprintf("/tasks/%d/cancel", second)), e.admin, nil).Code)

	w := e.do("GET", e.wsPath("/tasks"), e.admin, nil)
	var all []model.Task
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &all))
	require.Len(t, all, 2)
	assert.Equal(t, "second", all[0].Name, "newest first")

	w = e.do("GET", e.wsPath("/tasks?status=new"), e.admin, nil)
	var fresh []model.Task
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fresh))
	require.Len(t, fresh, 1)
	assert.Equal(t, "first", fresh[0].Name)
}

func TestHealthAndConsole(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, http.StatusOK, e.do("GET", "/healthz", "", nil).Code)
	w := e.do("GET", "/", "", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}
