package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Shell runs params.script through params.shell (default /bin/sh -c).
type Shell struct{}

type shellParams struct {
	Script string `json:"script"`
	Shell  string `json:"shell"`
	Cwd    string `json:"cwd"`
}

func (Shell) Run(ctx context.Context, rc *Context) (Outcome, error) {
	var p shellParams
	if err := json.Unmarshal(rc.Task.Params, &p); err != nil {
		return Outcome{}, fmt.Errorf("shell params: %w", err)
	}
	if strings.TrimSpace(p.Script) == "" {
		return Outcome{}, errors.New("shell task has an empty script")
	}
	if p.Shell == "" {
		p.Shell = "/bin/sh"
	}
	dir := rc.Dir
	if p.Cwd != "" {
		dir = p.Cwd
	}
	rc.Printf("$ %s", strings.TrimSpace(p.Script))
	code, err := rc.runQuiet(ctx, dir, p.Shell, "-c", p.Script)
	return Outcome{ExitCode: code}, err
}
