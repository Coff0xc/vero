package scenarios

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- ffuf 目录爆破集成 ----------

// ffufDirBrute —— 使用 ffuf 进行目录/文件枚举, 发现隐藏路径。
// 输出 JSON 格式便于解析, 过滤常见状态码(200/301/302/401/403)。
func ffufDirBrute(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	if target == "" {
		return tools.ToolResult{Success: false, Stderr: "ffuf: 缺 target", RC: -1}
	}

	// 确保 target 末尾有 /
	if !strings.HasSuffix(target, "/") {
		target += "/"
	}

	// 字典路径: 优先用 args 指定, 回退常见位置
	wordlist := tools.ArgStr(args, "wordlist", "")
	if wordlist == "" {
		// 常见字典路径(按优先级)
		candidates := []string{
			"/usr/share/wordlists/dirb/common.txt",      // Kali Linux
			"/usr/share/seclists/Discovery/Web-Content/common.txt", // SecLists
			"wordlist.txt", // 当前目录
		}
		// 简化: 直接用第一个候选(实际应检查文件存在)
		wordlist = candidates[0]
	}

	// ffuf 参数:
	// -u: 目标 URL, FUZZ 占位符
	// -w: 字典文件
	// -mc: 匹配状态码(200/204/301/302/307/401/403 表明路径存在)
	// -o: 输出文件(/dev/stdout 输出到标准输出)
	// -of: 输出格式(json)
	// -t: 线程数(默认 40, 避免过载)
	// -timeout: 请求超时(秒)
	// -se: 静默模式, 不打印 banner
	return tools.Sh([]string{
		"ffuf",
		"-u", target + "FUZZ",
		"-w", wordlist,
		"-mc", "200,204,301,302,307,401,403",
		"-o", "/dev/stdout",
		"-of", "json",
		"-t", "40",
		"-timeout", "10",
		"-se", // 静默, 避免 banner 干扰 JSON 解析
	}, 600*time.Second)
}

// ffufResult —— ffuf JSON 输出结构(仅映射需要的字段)。
type ffufResult struct {
	Results []ffufEntry `json:"results"`
}

type ffufEntry struct {
	Input      map[string]string `json:"input"`       // {"FUZZ": "admin"}
	Position   int               `json:"position"`    // 字典中的位置
	StatusCode int               `json:"status"`      // HTTP 状态码
	Length     int               `json:"length"`      // 响应大小
	Words      int               `json:"words"`       // 响应单词数
	Lines      int               `json:"lines"`       // 响应行数
	URL        string            `json:"url"`         // 完整 URL
}

// ParseFFUF —— 解析 ffuf JSON 输出, 提取发现的路径。
// 每个发现的路径作为 finding, Excerpt 记录状态码+大小(用于证据回查)。
func ParseFFUF(stdout string, args map[string]any) []tools.Observation {
	var result ffufResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil
	}

	target := tools.ArgStr(args, "target", "?")
	var obs []tools.Observation

	for _, entry := range result.Results {
		path := entry.Input["FUZZ"]
		if path == "" {
			continue
		}

		// 严重级判断
		severity := "info"
		if entry.StatusCode == 200 || entry.StatusCode == 204 {
			severity = "medium" // 可访问路径
		}
		if entry.StatusCode == 401 || entry.StatusCode == 403 {
			severity = "low" // 需认证/禁止访问, 但路径存在
		}

		// 敏感路径关键词(高危)
		sensitiveKeywords := []string{
			"admin", "backup", "config", "login", "upload", "shell",
			".git", ".env", ".sql", "phpinfo", "test", "debug",
		}
		for _, kw := range sensitiveKeywords {
			if strings.Contains(strings.ToLower(path), kw) {
				severity = "high"
				break
			}
		}

		label := fmt.Sprintf("[%s] Path found: /%s (HTTP %d, %d bytes)",
			severity, path, entry.StatusCode, entry.Length)

		// Excerpt: 状态码+大小(可在 JSON 原文中找到)
		excerpt := fmt.Sprintf(`"status":%d,"length":%d`, entry.StatusCode, entry.Length)

		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     target + ":path:" + path,
			Label:   label,
			Excerpt: excerpt,
		})
	}

	return obs
}

// ffufVhostEnum —— 虚拟主机枚举(通过 Host 头爆破子域名)。
// 用于发现同一 IP 上的多个虚拟主机。
func ffufVhostEnum(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	domain := tools.ArgStr(args, "domain", "")
	if target == "" || domain == "" {
		return tools.ToolResult{Success: false, Stderr: "ffuf vhost: 缺 target 或 domain", RC: -1}
	}

	wordlist := tools.ArgStr(args, "wordlist", "/usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt")

	// -u: 目标 IP/URL
	// -w: 子域名字典
	// -H: 设置 Host 头为 FUZZ.domain
	// -mc: 匹配状态码
	// -fs: 过滤响应大小(排除默认页面, 需根据目标调整)
	return tools.Sh([]string{
		"ffuf",
		"-u", target,
		"-w", wordlist,
		"-H", "Host: FUZZ." + domain,
		"-mc", "200,204,301,302,307,401,403",
		"-o", "/dev/stdout",
		"-of", "json",
		"-t", "40",
		"-timeout", "10",
		"-se",
	}, 600*time.Second)
}

// ParseFFUFVhost —— 解析虚拟主机枚举结果。
func ParseFFUFVhost(stdout string, args map[string]any) []tools.Observation {
	var result ffufResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil
	}

	domain := tools.ArgStr(args, "domain", "?")
	var obs []tools.Observation

	for _, entry := range result.Results {
		subdomain := entry.Input["FUZZ"]
		if subdomain == "" {
			continue
		}

		fqdn := subdomain + "." + domain
		label := fmt.Sprintf("[info] Virtual host found: %s (HTTP %d)", fqdn, entry.StatusCode)
		excerpt := fmt.Sprintf(`"status":%d`, entry.StatusCode)

		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     fqdn + ":vhost",
			Label:   label,
			Excerpt: excerpt,
		})
	}

	return obs
}
