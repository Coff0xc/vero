package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"sync/atomic"
	"time"
)

// Sh 执行外部命令(对应 Python _sh)。超时/失败都归一化为 ToolResult, 绝不 panic。
// 真实工具适配器(nmap/httpx/nuclei)都走它; 是否放行真实执行由上层的 live 开关决定,
// 默认 dry-run/仿真, 真实高危动作还要过 HITL —— 延续授权红队的谨慎边界。
// D9 修复: 超时杀整个进程组(见 exec_unix.go / exec_windows.go), 防止子进程变孤儿。
func Sh(argv []string, timeout time.Duration) ToolResult {
	if len(argv) == 0 {
		return ToolResult{Success: false, Stderr: "empty command", RC: -1}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	setProcAttr(cmd) // Unix: Setpgid; Windows: 无操作(Job Object 由 os/exec 内部保障)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb

	// 手动超时: 触发时杀掉整个进程组(命令自身派生的子进程一并终止)。
	// 不用 exec.CommandContext —— 它只杀直接子进程, 孙进程会成孤儿。
	var timedOut atomic.Bool
	timer := time.AfterFunc(timeout, func() {
		timedOut.Store(true)
		if cmd.Process != nil {
			killProcessGroup(cmd.Process.Pid)
		}
	})
	err := cmd.Run()
	timer.Stop()

	rc := 0
	if err != nil {
		rc = -1
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		}
	}
	if timedOut.Load() {
		return ToolResult{Success: false, Stdout: out.String(), Stderr: errb.String() + fmt.Sprintf("\n[vero] 命令超时(%s), 已终止进程组", timeout), RC: -1}
	}
	return ToolResult{Success: err == nil, Stdout: out.String(), Stderr: errb.String(), RC: rc}
}
