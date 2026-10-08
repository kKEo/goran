package process

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInterleavesOutputAndReturnsExitCode(t *testing.T) {
	var out bytes.Buffer
	code, err := Run(context.Background(), Spec{
		Command: "/bin/sh",
		Args:    []string{"-c", "echo one; echo two 1>&2; echo three; exit 3"},
		Output:  &out,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, code)
	assert.Equal(t, "one\ntwo\nthree\n", out.String())
}

func TestRunPassesEnvAndDir(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	code, err := Run(context.Background(), Spec{
		Command: "/bin/sh",
		Args:    []string{"-c", "echo $GREETING; pwd"},
		Dir:     dir,
		Env:     []string{"GREETING=hello", "PATH=/usr/bin:/bin"},
		Output:  &out,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "hello\n")
}

func TestRunKillsOnTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	code, err := Run(ctx, Spec{Command: "/bin/sh", Args: []string{"-c", "sleep 30; echo late"}})
	require.NoError(t, err)
	assert.Equal(t, -1, code)
	assert.Less(t, time.Since(start), 10*time.Second)
	assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

func TestRunMissingBinary(t *testing.T) {
	code, err := Run(context.Background(), Spec{Command: "/definitely/not/here"})
	assert.Error(t, err)
	assert.Equal(t, -1, code)
}
