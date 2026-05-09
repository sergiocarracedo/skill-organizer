//go:build !windows

package overlap

import (
	"os/exec"
	"syscall"
)

func configureInterruptHandling(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func interruptProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		return syscall.Kill(-pgid, syscall.SIGINT)
	}
	return cmd.Process.Signal(syscall.SIGINT)
}
