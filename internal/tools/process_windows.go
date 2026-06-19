//go:build windows

package tools

import "os/exec"

func SetProcAttr(cmd *exec.Cmd) {}

func KillProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
