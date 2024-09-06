//go:build !windows
// +build !windows

package terraform

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

var procAttrs = &unix.SysProcAttr{Setpgid: true}

func interrupt(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(syscall.SIGINT)
}
