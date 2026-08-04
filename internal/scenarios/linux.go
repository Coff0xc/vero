package scenarios

import (
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// LinuxToolsPack —— Linux 专用工具包
func LinuxToolsPack() Pack {
	return Pack{
		Name: "Linux工具集",
		Tools: []*tools.Tool{
			// LinPEAS 提权检测
			{
				Name:     "linpeas",
				Level:    tools.LevelRecon,
				Desc:     "LinPEAS Linux 提权路径枚举",
				Platform: "linux",
				Run:      linpeas,
				Parse:    ParseLinPEAS,
			},
			// Pspy 进程监控
			{
				Name:     "pspy",
				Level:    tools.LevelRecon,
				Desc:     "pspy 无需 root 权限的进程监控 (发现定时任务)",
				Platform: "linux",
				Run:      pspy,
			},
			// Linux 内核漏洞检测
			{
				Name:     "linux_exploit_suggester",
				Level:    tools.LevelRecon,
				Desc:     "Linux 内核漏洞利用建议器",
				Platform: "linux",
				Run:      linuxExploitSuggester,
				Parse:    ParseExploitSuggester,
			},
			// SUID 二进制文件查找
			{
				Name:     "find_suid",
				Level:    tools.LevelRecon,
				Desc:     "查找 SUID/SGID 二进制文件 (提权向量)",
				Platform: "linux",
				Run:      findSuid,
				Parse:    ParseSuid,
			},
			// 敏感文件搜索
			{
				Name:     "search_sensitive",
				Level:    tools.LevelRecon,
				Desc:     "搜索敏感文件 (.ssh/keys/.env/config/password)",
				Platform: "linux",
				Run:      searchSensitive,
				Parse:    ParseSensitiveFiles,
			},
			// Sudo 提权检测
			{
				Name:     "sudo_check",
				Level:    tools.LevelRecon,
				Desc:     "检查 sudo 配置和 GTFOBins 提权向量",
				Platform: "linux",
				Run:      sudoCheck,
			},
			// Cron 任务枚举
			{
				Name:     "cron_enum",
				Level:    tools.LevelRecon,
				Desc:     "枚举系统和用户 cron 任务 (劫持向量)",
				Platform: "linux",
				Run:      cronEnum,
				Parse:    ParseCron,
			},
			// 容器逃逸检测
			{
				Name:     "container_escape_check",
				Level:    tools.LevelRecon,
				Desc:     "检测容器环境和逃逸向量 (Docker/LXC)",
				Platform: "linux",
				Run:      containerEscapeCheck,
			},
			// 网络嗅探
			{
				Name:     "tcpdump_capture",
				Level:    tools.LevelScan,
				Desc:     "tcpdump 网络流量捕获 (需 root)",
				Platform: "linux",
				Run:      tcpdumpCapture,
				Args: []tools.ArgSpec{
					{Name: "interface", Desc: "网络接口 (eth0/wlan0)", Required: true},
					{Name: "duration", Desc: "捕获时长(秒), 默认30"},
				},
			},
			// SSH 密钥收集
			{
				Name:     "ssh_key_harvest",
				Level:    tools.LevelCred,
				Desc:     "收集系统中所有 SSH 私钥",
				Platform: "linux",
				Run:      sshKeyHarvest,
				Parse:    ParseSSHKeys,
			},
			// 历史命令分析
			{
				Name:     "history_analysis",
				Level:    tools.LevelRecon,
				Desc:     "分析 bash/zsh 历史命令 (密码/密钥/凭证)",
				Platform: "linux",
				Run:      historyAnalysis,
				Parse:    ParseHistory,
			},
			// 可写路径检测
			{
				Name:     "writable_paths",
				Level:    tools.LevelRecon,
				Desc:     "查找当前用户可写的系统路径 (配置/脚本劫持)",
				Platform: "linux",
				Run:      writablePaths,
			},
		},
		Fingerprint: func(services map[string]bool) bool {
			// Linux 工具总是可用 (在 Linux 平台上)
			return true
		},
	}
}

// ========== Linux 工具实现 ==========

func linpeas(args map[string]any) tools.ToolResult {
	return tools.ToolResult{
		Success: false,
		Stderr:  "LinPEAS需要下载: curl -L https://github.com/carlospolop/PEASS-ng/releases/latest/download/linpeas.sh | sh",
	}
}

func ParseLinPEAS(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	// 解析LinPEAS输出中的高危发现
	if strings.Contains(out, "99% PE") || strings.Contains(out, "95% PE") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "linux:privesc:high",
			Label:    "[high] High probability privilege escalation path found",
			Excerpt:  "99% PE",
			Severity: "high",
		})
	}
	return obs
}

func pspy(args map[string]any) tools.ToolResult {
	return tools.ToolResult{
		Success: false,
		Stderr:  "pspy需要下载: wget https://github.com/DominicBreuker/pspy/releases/download/v1.2.1/pspy64",
	}
}

func linuxExploitSuggester(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"bash", "-c", "uname -a; cat /etc/os-release"}, 10*time.Second)
}

func ParseExploitSuggester(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	// 提取内核版本用于漏洞匹配
	if strings.Contains(out, "Linux") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "linux:kernel:version",
			Label:   "[info] Kernel version detected",
			Excerpt: extractKernelVersion(out),
		})
	}
	return obs
}

func findSuid(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"find", "/", "-perm", "-4000", "-type", "f", "2>/dev/null"}, 60*time.Second)
}

func ParseSuid(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	// 危险的SUID二进制
	dangerousBinaries := []string{"nmap", "vim", "find", "bash", "more", "less", "nano", "cp"}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for _, dangerous := range dangerousBinaries {
			if strings.Contains(line, dangerous) {
				obs = append(obs, tools.Observation{
					Kind:     "finding",
					Key:      fmt.Sprintf("linux:suid:%s", dangerous),
					Label:    fmt.Sprintf("[high] Dangerous SUID binary: %s", line),
					Excerpt:  line,
					Severity: "high",
				})
				break
			}
		}
	}
	return obs
}

func searchSensitive(args map[string]any) tools.ToolResult {
	script := `
	find /home /root /var/www -type f \( -name "*.key" -o -name "*.pem" -o -name ".env" -o -name "id_rsa" -o -name "*.conf" \) 2>/dev/null | head -50
	`
	return tools.Sh([]string{"bash", "-c", script}, 30*time.Second)
}

func ParseSensitiveFiles(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	sensitivePatterns := map[string]string{
		"id_rsa":    "SSH private key",
		".pem":      "Certificate/Key file",
		".env":      "Environment config",
		"password":  "Password file",
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for pattern, desc := range sensitivePatterns {
			if strings.Contains(line, pattern) {
				obs = append(obs, tools.Observation{
					Kind:     "finding",
					Key:      fmt.Sprintf("linux:sensitive:%s", line),
					Label:    fmt.Sprintf("[medium] %s found: %s", desc, line),
					Excerpt:  line,
					Severity: "medium",
				})
				break
			}
		}
	}
	return obs
}

func sudoCheck(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"sudo", "-l"}, 10*time.Second)
}

func cronEnum(args map[string]any) tools.ToolResult {
	script := `
	echo "=== System Crontabs ==="
	cat /etc/crontab 2>/dev/null
	ls -la /etc/cron.* 2>/dev/null
	echo "=== User Crontabs ==="
	crontab -l 2>/dev/null
	`
	return tools.Sh([]string{"bash", "-c", script}, 15*time.Second)
}

func ParseCron(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	// 查找可写的cron脚本
	if strings.Contains(out, "rwx") || strings.Contains(out, "rw-") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "linux:cron:writable",
			Label:    "[medium] Writable cron script detected",
			Severity: "medium",
		})
	}
	return obs
}

func containerEscapeCheck(args map[string]any) tools.ToolResult {
	script := `
	echo "=== Container Check ==="
	cat /proc/1/cgroup 2>/dev/null | grep -i docker
	cat /.dockerenv 2>/dev/null
	echo "=== Capabilities ==="
	capsh --print 2>/dev/null
	`
	return tools.Sh([]string{"bash", "-c", script}, 10*time.Second)
}

func tcpdumpCapture(args map[string]any) tools.ToolResult {
	iface := tools.ArgStr(args, "interface", "eth0")
	_ = tools.ArgStr(args, "duration", "30") // duration用于提示，不实际使用

	return tools.ToolResult{
		Success: false,
		Stderr:  fmt.Sprintf("tcpdump需要root权限，手动执行: sudo tcpdump -i %s -c 100 -w /tmp/capture.pcap", iface),
	}
}

func sshKeyHarvest(args map[string]any) tools.ToolResult {
	script := `
	find /home /root -name "id_rsa" -o -name "id_ed25519" -o -name "id_ecdsa" 2>/dev/null
	`
	return tools.Sh([]string{"bash", "-c", script}, 30*time.Second)
}

func ParseSSHKeys(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && (strings.Contains(line, "id_rsa") || strings.Contains(line, "id_ed25519")) {
			obs = append(obs, tools.Observation{
				Kind:     "cred",
				Key:      fmt.Sprintf("linux:sshkey:%s", line),
				Label:    fmt.Sprintf("[high] SSH private key: %s", line),
				Excerpt:  line,
				Severity: "high",
			})
		}
	}
	return obs
}

func historyAnalysis(args map[string]any) tools.ToolResult {
	script := `
	cat ~/.bash_history ~/.zsh_history 2>/dev/null | grep -iE "password|passwd|secret|key|token" | head -20
	`
	return tools.Sh([]string{"bash", "-c", script}, 10*time.Second)
}

func ParseHistory(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	if strings.Contains(out, "password") || strings.Contains(out, "secret") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "linux:history:sensitive",
			Label:    "[medium] Sensitive data in command history",
			Excerpt:  "password",
			Severity: "medium",
		})
	}
	return obs
}

func writablePaths(args map[string]any) tools.ToolResult {
	script := `
	find /etc /usr/local/bin /usr/bin -writable -type f 2>/dev/null | head -30
	`
	return tools.Sh([]string{"bash", "-c", script}, 30*time.Second)
}

func extractKernelVersion(out string) string {
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Linux") && strings.Contains(line, "version") {
			return line
		}
	}
	return "unknown"
}
