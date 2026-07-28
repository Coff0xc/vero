package tools

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

// Sh 执行外部命令(对应 Python _sh)。超时/失败都归一化为 ToolResult, 绝不 panic。
// 真实工具适配器(nmap/httpx/nuclei)都走它; 是否放行真实执行由上层的 live 开关决定,
// 默认 dry-run/仿真, 真实高危动作还要过 HITL —— 延续授权红队的谨慎边界。
func Sh(argv []string, timeout time.Duration) ToolResult {
	if len(argv) == 0 {
		return ToolResult{Success: false, Stderr: "empty command", RC: -1}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	rc := 0
	if err != nil {
		rc = -1
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		}
	}
	return ToolResult{Success: err == nil, Stdout: out.String(), Stderr: errb.String(), RC: rc}
}
