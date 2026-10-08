// Package client is the typed HTTP client agents use to talk to the server.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kkEo/g-mk8s/wire"
)

// ErrLeaseLost means the server no longer considers this agent the owner of the
// task: the lease expired and the task was requeued, or it was finished elsewhere.
var ErrLeaseLost = errors.New("lease lost: task is no longer assigned to this agent")

// ErrUnauthorized means the agent key or registration token was rejected.
var ErrUnauthorized = errors.New("unauthorized: credential rejected by the server")

// APIError is any other non-2xx answer.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("server answered %d: %s", e.Status, e.Message) }

// Client talks to one server with one agent key.
type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

// New builds a client with a 30 second request timeout.
func New(baseURL, key string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Key:     key,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, in, out interface{}) (int, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return 0, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	req.Header.Set("User-Agent", "goran-agent")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	switch {
	case resp.StatusCode == http.StatusNoContent:
		return resp.StatusCode, nil
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if out != nil && len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return resp.StatusCode, fmt.Errorf("decode response: %w", err)
			}
		}
		return resp.StatusCode, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return resp.StatusCode, ErrUnauthorized
	case resp.StatusCode == http.StatusConflict:
		return resp.StatusCode, ErrLeaseLost
	}
	var e wire.ErrorResponse
	_ = json.Unmarshal(data, &e)
	if e.Error == "" {
		e.Error = strings.TrimSpace(string(data))
	}
	return resp.StatusCode, &APIError{Status: resp.StatusCode, Message: e.Error}
}

// Register exchanges a one-time registration token for an agent key.
func (c *Client) Register(ctx context.Context, req wire.RegisterRequest) (*wire.RegisterResponse, error) {
	var out wire.RegisterResponse
	if _, err := c.do(ctx, http.MethodPost, "/agent/register", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Next claims a task. It returns nil, nil when the queue is empty.
func (c *Client) Next(ctx context.Context) (*wire.AgentTask, error) {
	var t wire.AgentTask
	status, err := c.do(ctx, http.MethodGet, "/agent/next", nil, &t)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent {
		return nil, nil
	}
	return &t, nil
}

// Heartbeat extends the lease.
func (c *Client) Heartbeat(ctx context.Context, taskID uint) (*wire.LeaseResponse, error) {
	var out wire.LeaseResponse
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/agent/tasks/%d/heartbeat", taskID), struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AppendLog streams a chunk of output and extends the lease.
func (c *Client) AppendLog(ctx context.Context, taskID uint, chunk string) (*wire.LeaseResponse, error) {
	var out wire.LeaseResponse
	if _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/agent/tasks/%d/logs", taskID), wire.LogRequest{Chunk: chunk}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Result finishes the task (done, error) or parks it (awaiting_approval).
func (c *Client) Result(ctx context.Context, taskID uint, res wire.ResultRequest) error {
	_, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/agent/tasks/%d/result", taskID), res, nil)
	return err
}
