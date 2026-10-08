// Package process runs one child command with stdout and stderr interleaved
// into a single writer, bounded by a context.
package process

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"time"
)

// Spec describes the command to run.
type Spec struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	// Output receives stdout and stderr in arrival order. nil discards.
	Output io.Writer
}

// lockedWriter serialises the two pipe copiers onto one writer so lines never
// interleave mid-write.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// Run starts the command and waits for it. It returns the exit code (−1 when
// the process was killed or never started). When ctx is done the whole
// process group is killed so helpers such as terraform providers die too.
func Run(ctx context.Context, spec Spec) (int, error) {
	out := spec.Output
	if out == nil {
		out = io.Discard
	}
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	w := &lockedWriter{w: out}
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.WaitDelay = 5 * time.Second
	configure(cmd)
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}
