package scenarios

// 交互式侦察工具链 —— 黑盒自主渗透的"感知层"。
//
// 与被动扫描(nmap/nuclei)的区别: 这些工具让 LLM 主动抓取目标页面、提取端点/表单/JS、
// 逐个探测 —— 攻击面由 agent 自己发现并建成图, 而非盲扫端口+模板库。
// 产出 type=endpoint 的节点, 攻击图快照里 LLM 能看到"目标长什么样", 从而生成有针对性的假设。
//
// 证据约束不变: 每条 observation 的 Excerpt 逐字取自工具输出, 内核回查。

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// fetchPage —— 抓取目标页面 HTML body(非 HEAD; -L 跟随跳转)。
func fetchPage(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"curl", "-sL", "--noproxy", "*", "--max-time", "20",
		normalizeURL(tools.ArgStr(args, "target", ""))}, 60*time.Second)
}

// fetchHeaders —— 抓取响应头(与 http_probe 互补: 这里返回完整头文本供 LLM 观察)。
func fetchHeaders(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"curl", "-sI", "--noproxy", "*", "--max-time", "20",
		normalizeURL(tools.ArgStr(args, "target", ""))}, 60*time.Second)
}

// reHrefSrc —— 提取 HTML 里的 href/src/action/formaction 属性值。
var reHrefSrc = regexp.MustCompile(`(?i)(?:href|src|action|formaction)\s*=\s*["']([^"'#]+)["']`)

// reFormInput —— 提取表单 input 的 name(参数候选)。
var reFormInput = regexp.MustCompile(`(?i)<input[^>]*\bname\s*=\s*["']([^"']+)["']`)

// reScriptSrc —— 提取 <script src=...>(JS 端点, 可能内嵌 API 路径)。
var reScriptSrc = regexp.MustCompile(`(?i)<script[^>]*\bsrc\s*=\s*["']([^"']+)["']`)

// staticSuffix —— 静态资源后缀, 不作为端点(JS 保留, 需进一步分析)。
var staticSuffix = []string{".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot", ".pdf", ".map"}

// endpointSensitive —— 敏感路径特征(命中即 finding)。
var endpointSensitive = []string{"/admin", "/api", "/config", "/backup", "/debug", "/console", "/actuator", "/swagger", "/graphql", "/.env", "/server-status", "/rest/", "/login", "/upload", "/download", "/export", "/import"}

// extractEndpoints —— 抓取目标页面并提取端点/表单/JS(工具自包含: 内部 curl + 正则)。
// 产出 type=endpoint 节点(攻击面地图) + 敏感端点 finding。
func extractEndpoints(args map[string]any) tools.ToolResult {
	target := normalizeURL(tools.ArgStr(args, "target", ""))
	if target == "" {
		return tools.ToolResult{Success: false, Stderr: "extract_endpoints: 缺 target", RC: -1}
	}
	html := fetchPage(args).Stdout
	if strings.TrimSpace(html) == "" {
		return tools.ToolResult{Success: false, Stdout: html, Stderr: "extract_endpoints: 页面为空(抓取失败?)", RC: -1}
	}
	base, _ := url.Parse(target)
	seen := map[string]bool{}
	var lines []string
	for _, m := range reHrefSrc.FindAllStringSubmatch(html, -1) {
		p := strings.TrimSpace(m[1])
		if p == "" || strings.HasPrefix(p, "javascript:") || strings.HasPrefix(p, "mailto:") || strings.HasPrefix(p, "tel:") || strings.HasPrefix(p, "data:") {
			continue
		}
		// 去掉 fragment/query 后的 path(参数留给参数分析)
		if u, err := url.Parse(p); err == nil {
			p = u.Path
			if p == "" {
				p = "/"
			}
		}
		// 跳过纯静态资源(JS 保留)
		skip := false
		for _, s := range staticSuffix {
			if strings.HasSuffix(strings.ToLower(p), s) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			// 相对路径 -> 相对当前目录解析(D25 修复: 用 path.Dir(base.Path) 而非 TrimSuffix)。
			// 例: base=/user/view/manifest, 相对 edit → /user/view/edit(而非 /user/view/manifest/edit)。
			if base != nil {
				p = path.Join(path.Dir(base.Path), p)
			}
		}
		if p != "/" && !seen[p] {
			seen[p] = true
			lines = append(lines, p)
		}
	}
	// 表单参数候选(输入 name)
	var params []string
	for _, m := range reFormInput.FindAllStringSubmatch(html, -1) {
		params = append(params, m[1])
	}
	// JS 端点(前端路由可能内嵌 API)
	var jsFiles []string
	for _, m := range reScriptSrc.FindAllStringSubmatch(html, -1) {
		jsFiles = append(jsFiles, m[1])
	}
	sort.Strings(lines)
	sort.Strings(params)
	sort.Strings(jsFiles)

	var b strings.Builder
	for _, l := range lines {
		b.WriteString("endpoint: " + l + "\n")
	}
	for _, p := range params {
		b.WriteString("param: " + p + "\n")
	}
	for _, j := range jsFiles {
		b.WriteString("js: " + j + "\n")
	}
	out := b.String()
	if out == "" {
		return tools.ToolResult{Success: true, Stdout: html, Stderr: "extract_endpoints: 未提取到端点(页面可能无链接)", RC: 0}
	}
	return tools.ToolResult{Success: true, Stdout: out, RC: 0}
}

// ParseEndpoints —— 解析 extract_endpoints 输出: 端点/参数/JS 各行 -> observation。
// 敏感路径提升为 finding(带 severity), 其余为 endpoint(攻击面地图节点)。
func ParseEndpoints(out string, args map[string]any) []tools.Observation {
	t := tools.ArgStr(args, "target", "?")
	var obs []tools.Observation
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "endpoint: "):
			p := strings.TrimPrefix(line, "endpoint: ")
			// 双保险: 静态资源不进攻击面(防御脏输入/LLM 拼接)。
			skip := false
			for _, s := range staticSuffix {
				if strings.HasSuffix(strings.ToLower(p), s) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			severe := ""
			for _, s := range endpointSensitive {
				if strings.Contains(strings.ToLower(p), s) {
					severe = "medium"
					break
				}
			}
			kind, key, label := "endpoint", p, "endpoint "+p
			if severe != "" {
				kind = "finding"
				key = "endpoint:" + p
				label = fmt.Sprintf("[%s] 敏感端点: %s", severe, p)
			}
			obs = append(obs, tools.Observation{Kind: kind, Key: key, Label: label, Excerpt: line, Severity: severe})
		case strings.HasPrefix(line, "param: "):
			p := strings.TrimPrefix(line, "param: ")
			obs = append(obs, tools.Observation{Kind: "endpoint", Key: t + ":param:" + p, Label: "form param " + p, Excerpt: line})
		case strings.HasPrefix(line, "js: "):
			j := strings.TrimPrefix(line, "js: ")
			obs = append(obs, tools.Observation{Kind: "endpoint", Key: t + ":js:" + j, Label: "js " + j, Excerpt: line})
		}
	}
	return obs
}

// probeEndpoint —— 对发现的端点发 GET 探测: 记录状态码/响应长度/敏感响应词。
// 产出 finding: 泄露敏感信息的端点(如 200 + error/token/debug 关键字)值得深挖。
func probeEndpoint(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	path := tools.ArgStr(args, "path", "/")
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base := normalizeURL(target)
	u := base
	if !strings.HasSuffix(base, "/") {
		u = base + path
	} else {
		u = strings.TrimSuffix(base, "/") + path
	}
	return tools.Sh([]string{"curl", "-s", "--noproxy", "*", "--max-time", "15",
		"-o", "-", "-w", "\n---HTTP %{http_code} len=%{size_download} ct=%{content_type}---\n", u}, 60*time.Second)
}

// ParseProbe —— 解析探测响应: 状态/长度 + 敏感词命中 -> observation。
func ParseProbe(out string, args map[string]any) []tools.Observation {
	if !strings.Contains(out, "---HTTP ") {
		return nil
	}
	t := tools.ArgStr(args, "target", "?")
	path := tools.ArgStr(args, "path", "/")
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "---HTTP ") {
			line = strings.Trim(l, "- ")
			break
		}
	}
	if line == "" {
		return nil
	}
	obs := []tools.Observation{{Kind: "endpoint", Key: t + path, Label: "probe " + line, Excerpt: line}}
	// 404 错误页(如 Express "Cannot GET")是正常行为, 不产 sensitive finding —— 防误报刷屏。
	if strings.Contains(line, "HTTP 404") {
		return obs
	}
	// 敏感响应词: 命中即值得深挖的 finding(证据 = 响应行)。
	// D24 修复: 宽泛单词(error/admin/debug 在正常页面太常见)改为强信号模式 ——
	// 具体标识符(access_token/api_key)或组合词(stack trace/debug mode), 大幅降误报。
	sensitivePatterns := []struct{ kw, label string }{
		{"stacktrace", "堆栈跟踪"},
		{"stack trace", "堆栈跟踪"},
		{"access_token", "访问令牌"},
		{"api_key", "API 密钥"},
		{"apikey", "API 密钥"},
		{"secret_key", "密钥"},
		{"client_secret", "客户端密钥"},
		{"debug=true", "调试开关"},
		{"debug mode", "调试模式"},
		{"admin panel", "管理后台"},
		{"admin login", "管理后台"},
		{"password=", "明文密码"},
		{"pwd=", "明文密码"},
	}
	low := strings.ToLower(out)
	for _, p := range sensitivePatterns {
		if strings.Contains(low, p.kw) {
			ex := onelineExcerpt(out, p.kw, 160)
			obs = append(obs, tools.Observation{
				Kind: "finding", Key: t + path + ":sensitive:" + p.kw,
				Label:    fmt.Sprintf("[medium] 端点 %s 响应含敏感信号 %q(%s)", path, p.kw, p.label),
				Excerpt:  ex,
				Severity: "medium",
			})
			break // 每端点最多一条敏感 finding, 防刷屏
		}
	}
	return obs
}

// onelineExcerpt —— 从输出中取含关键字的片段(逐字存在于输出, 供证据回查)。
func onelineExcerpt(out, kw string, max int) string {
	low := strings.ToLower(out)
	i := strings.Index(low, kw)
	if i < 0 {
		return tools.Clip(out, max)
	}
	start := i - 40
	if start < 0 {
		start = 0
	}
	end := i + len(kw) + 80
	if end > len(out) { // 修: 切片越界 panic(短输出时 end 超 len)
		end = len(out)
	}
	return tools.Clip(out[start:end], max)
}

// ReconPack —— 交互式侦察场景包: 黑盒感知层(web 指纹激活)。
func ReconPack() Pack {
	return Pack{
		Name: "recon",
		Tools: []*tools.Tool{
			{Name: "fetch_page", Level: tools.LevelScan, Desc: "抓取目标页面 HTML body(curl -sL), 供分析页面结构/表单/链接", Run: fetchPage,
				Args: []tools.ArgSpec{{Name: "target", Desc: "目标 URL(带 scheme), 如 http://host:port/login", Required: true}}},
			{Name: "fetch_headers", Level: tools.LevelScan, Desc: "抓取目标响应头文本(curl -sI), 供分析技术栈/安全头/泄露头", Run: fetchHeaders,
				Args: []tools.ArgSpec{{Name: "target", Desc: "目标 URL(带 scheme)", Required: true}}},
			{Name: "extract_endpoints", Level: tools.LevelScan, Desc: "抓取页面并提取端点/表单参数/JS 文件, 建立攻击面地图(target 用 URL)", Run: extractEndpoints, Parse: ParseEndpoints,
				Args: []tools.ArgSpec{{Name: "target", Desc: "目标 URL(带 scheme)", Required: true}}},
			{Name: "probe_endpoint", Level: tools.LevelScan, Desc: "对目标端点发 GET 探测, 记录状态码/长度/敏感词", Run: probeEndpoint, Parse: ParseProbe,
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "目标 URL(带 scheme)", Required: true},
					{Name: "path", Desc: "探测路径, 如 /admin, 默认 /"},}},
		},
		Fingerprint: func(s map[string]bool) bool {
			return s["http"] || s["https"] || s["ssl/http"] || s["http-proxy"]
		},
	}
}
