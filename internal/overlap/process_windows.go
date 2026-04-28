//go:build windows

package overlap

import "os/exec"

func configureInterruptHandling(cmd *exec.Cmd) {}

func interruptProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
