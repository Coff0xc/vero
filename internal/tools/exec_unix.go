//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

// setProcAttr —— Unix: 新建进程组, 使 killProcessGroup 能整组终止。
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup —— Unix: kill 负 PID = 整个进程组(含孙进程)。
func killProcessGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
