//go:build windows

package process

import "os/exec"

// configure keeps the default behaviour (kill the process) on Windows.
func configure(cmd *exec.Cmd) {}
