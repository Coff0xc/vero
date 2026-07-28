// Package tooltest —— 工具集成验证系统
package tooltest

import (
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tools"
)

// ToolStatus —— 工具验证状态
type ToolStatus struct {
	Name      string        `json:"name"`
	Level     int           `json:"level"`
	Available bool          `json:"available"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Tested    bool          `json:"tested"`
}

// VerifyAll —— 验证所有已注册工具
func VerifyAll(reg *tools.Registry) []ToolStatus {
	results := []ToolStatus{}

	for _, name := range reg.Names() {
		tool, _ := reg.Get(name)
		status := ToolStatus{
			Name:   name,
			Level:  tool.Level,
			Tested: true,
		}

		start := time.Now()
		result := verifyTool(tool)
		status.Duration = time.Since(start)
		status.Available = result.Success
		if !result.Success {
			status.Error = result.Stderr
			if status.Error == "" {
				status.Error = result.Stdout
			}
		}

		results = append(results, status)
	}

	return results
}

// verifyTool —— 验证单个工具（使用安全参数）
func verifyTool(tool *tools.Tool) tools.ToolResult {
	// 根据工具名称使用安全的测试参数
	args := getSafeTestArgs(tool.Name)

	// 设置超时保护
	done := make(chan tools.ToolResult, 1)
	go func() {
		done <- tool.Run(args)
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(10 * time.Second):
		return tools.ToolResult{
			Success: false,
			Stderr:  "timeout after 10s",
			RC:      -1,
		}
	}
}

// getSafeTestArgs —— 为每个工具提供安全的测试参数
func getSafeTestArgs(toolName string) map[string]any {
	switch {
	case strings.Contains(toolName, "http") || strings.Contains(toolName, "web"):
		return map[string]any{"target": "http://127.0.0.1:1"}
	case strings.Contains(toolName, "nmap"):
		return map[string]any{"target": "127.0.0.1", "ports": "1"}
	case strings.Contains(toolName, "smb") || strings.Contains(toolName, "ldap"):
		return map[string]any{"target": "127.0.0.1"}
	case strings.Contains(toolName, "cloud") || strings.Contains(toolName, "aws") || strings.Contains(toolName, "azure") || strings.Contains(toolName, "gcp"):
		return map[string]any{"action": "test"}
	case strings.Contains(toolName, "container") || strings.Contains(toolName, "docker") || strings.Contains(toolName, "k8s"):
		return map[string]any{"action": "test"}
	default:
		return map[string]any{"target": "127.0.0.1"}
	}
}

// Summary —— 生成验证摘要报告
func Summary(results []ToolStatus) string {
	total := len(results)
	available := 0
	byLevel := map[int]int{0: 0, 1: 0, 2: 0, 3: 0, 4: 0}
	availableByLevel := map[int]int{0: 0, 1: 0, 2: 0, 3: 0, 4: 0}

	for _, r := range results {
		if r.Available {
			available++
			availableByLevel[r.Level]++
		}
		byLevel[r.Level]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("工具验证报告\n"))
	sb.WriteString(fmt.Sprintf("=============\n\n"))
	sb.WriteString(fmt.Sprintf("总工具数: %d\n", total))
	sb.WriteString(fmt.Sprintf("可用: %d (%.1f%%)\n", available, float64(available)/float64(total)*100))
	sb.WriteString(fmt.Sprintf("不可用: %d (%.1f%%)\n\n", total-available, float64(total-available)/float64(total)*100))

	sb.WriteString("按级别统计:\n")
	levels := map[int]string{
		0: "L0-侦察",
		1: "L1-扫描",
		2: "L2-凭证",
		3: "L3-利用",
		4: "L4-破坏",
	}
	for i := 0; i <= 4; i++ {
		if byLevel[i] > 0 {
			sb.WriteString(fmt.Sprintf("  %s: %d/%d 可用\n", levels[i], availableByLevel[i], byLevel[i]))
		}
	}

	sb.WriteString("\n不可用工具:\n")
	for _, r := range results {
		if !r.Available {
			errMsg := r.Error
			if len(errMsg) > 60 {
				errMsg = errMsg[:60] + "..."
			}
			sb.WriteString(fmt.Sprintf("  ✗ %s: %s\n", r.Name, errMsg))
		}
	}

	return sb.String()
}

// CountAllTools —— 统计所有场景包中的工具数量
func CountAllTools() int {
	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)
	return len(reg.Names())
}
