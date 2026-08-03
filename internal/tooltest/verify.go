// Package tooltest —— 工具集成验证系统
package tooltest

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tools"
)

// ToolStatus —— 工具验证状态
type ToolStatus struct {
	Name        string        `json:"name"`
	Level       int           `json:"level"`
	Available   bool          `json:"available"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	Tested      bool          `json:"tested"`
	InstallType string        `json:"install_type"`           // 三态安装途径: binary|pip|none(恒输出)
	Installable string        `json:"installable,omitempty"`  // 可自动安装的二进制名(nuclei/ffuf)
	PipHint     string        `json:"pip_hint,omitempty"`     // 需手动 pip 的命令
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
	// 三态安装途径恒输出(前端按钮与批量安装的唯一驱动)。
	status.InstallType = tools.InstallType(name)
	if !result.Success {
		status.Error = result.Stderr
		if status.Error == "" {
			status.Error = result.Stdout
		}
		// 标注修复途径: 可自动下载的二进制 / 需手动 pip 的 Python 依赖
		status.Installable = tools.InstallableBinary(name)
		status.PipHint = tools.InstallPipHint(name)
	}

	results = append(results, status)
	}

	return results
}

// verifyTool —— 验证单个工具(非攻击性: 只探测依赖二进制是否就位 / RPC 端口可达)。
// 修原版直接以攻击参数真实执行工具(对 127.0.0.1 发 SQLi/凭证喷洒等):
// "验证可用性"不该真的发起攻击 —— 改为检查依赖, 语义等价且零风险。
func verifyTool(tool *tools.Tool) tools.ToolResult {
	// Metasploit 工具: 仅 TCP 可达性探测(不调用任何 exploit)。
	if strings.HasPrefix(tool.Name, "msf_") {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:55553", 2*time.Second)
		if err != nil {
			return tools.ToolResult{Success: false, Stderr: "msfrpcd 未运行 (127.0.0.1:55553)", RC: -1}
		}
		_ = conn.Close()
		return tools.ToolResult{Success: true, Stdout: "msfrpcd reachable"}
	}

	// K8s 工具: 检查 ServiceAccount 凭证文件(pod 内才存在)。
	if tool.Name == "k8s_sa_enum" {
		if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err != nil {
			return tools.ToolResult{Success: false, Stderr: "非 Pod 环境: 无 ServiceAccount token", RC: -1}
		}
		return tools.ToolResult{Success: true, Stdout: "ServiceAccount token present"}
	}

	// 其余工具: 检查其依赖的命令是否存在于 PATH(不执行工具本身)。
	binary := tools.ToolBinary(tool.Name)
	if binary == "" {
		return tools.ToolResult{Success: true, Stdout: "no external dependency"}
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: fmt.Sprintf("%s 未安装 (command not found)", binary), RC: -1}
	}
	return tools.ToolResult{Success: true, Stdout: path}
}

// getSafeTestArgs —— 为每个工具提供安全的测试参数（导出供 CLI 使用）
func GetSafeTestArgs(toolName string) map[string]any {
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
