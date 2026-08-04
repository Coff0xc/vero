package scenarios

import (
	"fmt"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// WindowsToolsPack —— Windows 专用工具包
func WindowsToolsPack() Pack {
	return Pack{
		Name: "Windows工具集",
		Tools: []*tools.Tool{
			// PowerShell 侦察
			{
				Name:     "powershell_enum",
				Level:    tools.LevelRecon,
				Desc:     "PowerShell 系统信息枚举 (用户/组/进程/服务)",
				Platform: "windows",
				Run:      powershellEnum,
				Parse:    ParsePowerShellEnum,
			},
			// Windows Defender 检测
			{
				Name:     "defender_check",
				Level:    tools.LevelRecon,
				Desc:     "检测 Windows Defender 状态和排除路径",
				Platform: "windows",
				Run:      defenderCheck,
			},
			// AMSI 绕过检测
			{
				Name:     "amsi_bypass",
				Level:    tools.LevelExploit,
				Desc:     "尝试 AMSI (Anti-Malware Scan Interface) 绕过",
				Platform: "windows",
				Run:      amsiBypass,
			},
			// Windows 凭证提取
			{
				Name:     "mimikatz",
				Level:    tools.LevelCred,
				Desc:     "Mimikatz 凭证提取 (需管理员权限)",
				Platform: "windows",
				Run:      mimikatz,
				Parse:    ParseMimikatz,
				Args: []tools.ArgSpec{
					{Name: "command", Desc: "Mimikatz 命令 (sekurlsa::logonpasswords/lsadump::sam)", Required: true},
				},
			},
			// 注册表持久化检测
			{
				Name:     "registry_persist",
				Level:    tools.LevelRecon,
				Desc:     "检测注册表持久化位置 (Run/RunOnce/Services)",
				Platform: "windows",
				Run:      registryPersist,
			},
			// WMI 侦察
			{
				Name:     "wmi_query",
				Level:    tools.LevelRecon,
				Desc:     "WMI 查询 (Win32_Process/Win32_Service/Win32_LoggedOnUser)",
				Platform: "windows",
				Run:      wmiQuery,
				Args: []tools.ArgSpec{
					{Name: "query", Desc: "WMI 查询语句", Required: true},
				},
			},
			// Seatbelt 信息收集
			{
				Name:     "seatbelt",
				Level:    tools.LevelRecon,
				Desc:     "Seatbelt 全面安全审计 (凭证/浏览器/进程/网络)",
				Platform: "windows",
				Run:      seatbelt,
			},
			// SharpHound 域侦察
			{
				Name:     "sharphound",
				Level:    tools.LevelRecon,
				Desc:     "SharpHound 域环境收集 (用户/组/计算机/信任关系)",
				Platform: "windows",
				Run:      sharphound,
			},
			// Windows 端口转发
			{
				Name:     "netsh_portproxy",
				Level:    tools.LevelExploit,
				Desc:     "netsh 端口转发设置 (横向移动)",
				Platform: "windows",
				Run:      netshPortproxy,
				Args: []tools.ArgSpec{
					{Name: "local_port", Desc: "本地端口", Required: true},
					{Name: "remote_host", Desc: "远程主机", Required: true},
					{Name: "remote_port", Desc: "远程端口", Required: true},
				},
			},
			// UAC 绕过检测
			{
				Name:     "uac_check",
				Level:    tools.LevelRecon,
				Desc:     "检测 UAC 配置和已知绕过向量",
				Platform: "windows",
				Run:      uacCheck,
			},
		},
		Fingerprint: func(services map[string]bool) bool {
			// Windows 工具总是可用 (在 Windows 平台上)
			return true
		},
	}
}

// ========== Windows 工具实现 ==========

func powershellEnum(args map[string]any) tools.ToolResult {
	script := "Write-Host '=== System Info ==='; Get-ComputerInfo | Select-Object CsName, OsName, OsVersion; Write-Host '=== Current User ==='; whoami /all; Write-Host '=== Local Users ==='; Get-LocalUser | Select-Object Name, Enabled; Write-Host '=== Running Processes ==='; Get-Process | Select-Object -First 20 Name, Id, CPU"
	return tools.Sh([]string{"powershell", "-NoProfile", "-Command", script}, 30*time.Second)
}

func ParsePowerShellEnum(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	// 简化: 提取关键信息作为 finding
	if contains(out, "Administrator") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "windows:user:admin",
			Label:   "[info] Administrator account detected",
			Excerpt: "Administrator",
		})
	}
	return obs
}

func defenderCheck(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"powershell", "-Command", "Get-MpComputerStatus | Select-Object RealTimeProtectionEnabled, AntivirusEnabled"}, 15*time.Second)
}

func amsiBypass(args map[string]any) tools.ToolResult {
	// 检测 AMSI 是否可绕过 (不实际执行绕过代码)
	return tools.ToolResult{Success: false, Stderr: "AMSI绕过仅用于授权测试，请手动执行"}
}

func mimikatz(args map[string]any) tools.ToolResult {
	cmd := tools.ArgStr(args, "command", "sekurlsa::logonpasswords")
	return tools.ToolResult{
		Success: false,
		Stderr:  fmt.Sprintf("Mimikatz工具需要单独安装，请执行: mimikatz.exe \"%s\" exit", cmd),
	}
}

func ParseMimikatz(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	// 提取凭证信息
	if contains(out, "Username") && contains(out, "Password") {
		obs = append(obs, tools.Observation{
			Kind:     "cred",
			Key:      "windows:cred:mimikatz",
			Label:    "[high] Credentials extracted via Mimikatz",
			Severity: "high",
		})
	}
	return obs
}

func registryPersist(args map[string]any) tools.ToolResult {
	script := "Write-Host '=== Run Keys ==='; Get-ItemProperty 'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run' -ErrorAction SilentlyContinue; Get-ItemProperty 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run' -ErrorAction SilentlyContinue"
	return tools.Sh([]string{"powershell", "-Command", script}, 15*time.Second)
}

func wmiQuery(args map[string]any) tools.ToolResult {
	query := tools.ArgStr(args, "query", "SELECT * FROM Win32_Process")
	return tools.Sh([]string{"wmic", "path", query, "get"}, 30*time.Second)
}

func seatbelt(args map[string]any) tools.ToolResult {
	return tools.ToolResult{
		Success: false,
		Stderr:  "Seatbelt需要单独编译，请从 https://github.com/GhostPack/Seatbelt 获取",
	}
}

func sharphound(args map[string]any) tools.ToolResult {
	return tools.ToolResult{
		Success: false,
		Stderr:  "SharpHound需要单独下载，请从 https://github.com/BloodHoundAD/SharpHound 获取",
	}
}

func netshPortproxy(args map[string]any) tools.ToolResult {
	localPort := tools.ArgStr(args, "local_port", "")
	remoteHost := tools.ArgStr(args, "remote_host", "")
	remotePort := tools.ArgStr(args, "remote_port", "")

	cmd := fmt.Sprintf("netsh interface portproxy add v4tov4 listenport=%s connectaddress=%s connectport=%s",
		localPort, remoteHost, remotePort)

	return tools.ToolResult{
		Success: false,
		Stderr:  fmt.Sprintf("端口转发需要管理员权限，手动执行: %s", cmd),
	}
}

func uacCheck(args map[string]any) tools.ToolResult {
	script := "Get-ItemProperty 'HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Policies\\System' | Select-Object EnableLUA, ConsentPromptBehaviorAdmin"
	return tools.Sh([]string{"powershell", "-Command", script}, 10*time.Second)
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
