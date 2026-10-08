package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Source says where a terraform or ansible task's files come from: a git
// repository cloned into the task directory, or a directory already present
// on the agent host.
type Source struct {
	Git  string `json:"git"`
	Ref  string `json:"ref"`
	Path string `json:"path"`
	Dir  string `json:"dir"`
}

// Prepare materialises the source and returns the directory to run in. A
// clone made during the plan phase is reused by the apply phase.
func (s Source) Prepare(ctx context.Context, rc *Context) (string, error) {
	if s.Dir != "" {
		dir := filepath.Join(s.Dir, s.Path)
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			return "", fmt.Errorf("source dir %q is not a directory on this agent", dir)
		}
		rc.Printf("using local source %s", dir)
		return dir, nil
	}
	if s.Git == "" {
		return "", errors.New("source needs git or dir")
	}
	dest := filepath.Join(rc.Dir, "src")
	if _, err := os.Stat(dest); err == nil {
		rc.Printf("reusing checkout in %s", dest)
	} else {
		args := []string{"clone", "--depth", "1"}
		if s.Ref != "" {
			args = append(args, "--branch", s.Ref)
		}
		args = append(args, s.Git, dest)
		code, err := rc.run(ctx, rc.Dir, "git", args...)
		if err != nil {
			return "", fmt.Errorf("git clone: %w", err)
		}
		if code != 0 {
			return "", fmt.Errorf("git clone exited with status %d", code)
		}
	}
	return filepath.Join(dest, s.Path), nil
}
