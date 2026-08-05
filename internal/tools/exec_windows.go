//go:build windows

package tools

import (
	"os/exec"
	"strconv"
)

// setProcAttr —— Windows: 无操作。os/exec 内部用 Job Object 管理进程,
// 超时终止时进程树会被一并回收; 显式 taskkill 兜底见 killProcessGroup。
func setProcAttr(cmd *exec.Cmd) {}

// killProcessGroup —— Windows: taskkill /T 终止进程树(含孙进程)。
func killProcessGroup(pid int) {
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}
