package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- Metasploit RPC 集成 ----------

// MSFClient —— Metasploit RPC 客户端, 通过 msfrpcd 远程调用 Metasploit。
type MSFClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewMSFClient —— 创建 MSF RPC 客户端。
// 需要先启动 msfrpcd: msfrpcd -P <password> -S -a 127.0.0.1
func NewMSFClient(baseURL, password string) (*MSFClient, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// 认证获取 token
	token, err := msfAuth(baseURL, password, client)
	if err != nil {
		return nil, fmt.Errorf("MSF auth failed: %w", err)
	}

	return &MSFClient{
		baseURL: baseURL,
		token:   token,
		client:  client,
	}, nil
}

// msfAuth —— 向 msfrpcd 认证获取 token。
func msfAuth(baseURL, password string, client *http.Client) (string, error) {
	payload := map[string]any{
		"method":  "auth.login",
		"params":  []any{"msf", password},
	}

	resp, err := msfCall(baseURL, "", payload, client)
	if err != nil {
		return "", err
	}

	// 响应格式: {"result": "success", "token": "..."}
	if result, ok := resp["result"].(string); ok && result == "success" {
		if token, ok := resp["token"].(string); ok {
			return token, nil
		}
	}

	return "", fmt.Errorf("auth failed: %v", resp)
}

// msfCall —— 通用 MSF RPC 调用。
func msfCall(baseURL, token string, payload map[string]any, client *http.Client) (map[string]any, error) {
	// 添加 token 到 params (如果有)
	if token != "" {
		if params, ok := payload["params"].([]any); ok {
			payload["params"] = append([]any{token}, params...)
		} else {
			payload["params"] = []any{token}
		}
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", baseURL+"/api/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// SearchExploit —— 搜索可用的 exploit 模块 (按 CVE 或关键词)。
func (m *MSFClient) SearchExploit(query string) ([]string, error) {
	payload := map[string]any{
		"method": "module.search",
		"params": []any{query},
	}

	resp, err := msfCall(m.baseURL, m.token, payload, m.client)
	if err != nil {
		return nil, err
	}

	// 响应格式: {"modules": ["exploit/linux/http/...", ...]}
	if modules, ok := resp["modules"].([]any); ok {
		var exploits []string
		for _, mod := range modules {
			if name, ok := mod.(string); ok && strings.HasPrefix(name, "exploit/") {
				exploits = append(exploits, name)
			}
		}
		return exploits, nil
	}

	return nil, nil
}

// ExecuteExploit —— 执行指定 exploit 模块。
func (m *MSFClient) ExecuteExploit(module string, opts map[string]any) (string, error) {
	// 1. 获取模块信息 (验证参数)
	payload := map[string]any{
		"method": "module.options",
		"params": []any{"exploit", module},
	}

	_, err := msfCall(m.baseURL, m.token, payload, m.client)
	if err != nil {
		return "", fmt.Errorf("module not found: %w", err)
	}

	// 2. 执行 exploit
	execPayload := map[string]any{
		"method": "module.execute",
		"params": []any{"exploit", module, opts},
	}

	resp, err := msfCall(m.baseURL, m.token, execPayload, m.client)
	if err != nil {
		return "", err
	}

	// 响应格式: {"job_id": "1", "uuid": "..."}
	if jobID, ok := resp["job_id"].(string); ok {
		return jobID, nil
	}

	return "", fmt.Errorf("execute failed: %v", resp)
}

// GetSessions —— 获取当前活跃 session 列表。
func (m *MSFClient) GetSessions() ([]map[string]any, error) {
	payload := map[string]any{
		"method": "session.list",
		"params": []any{},
	}

	resp, err := msfCall(m.baseURL, m.token, payload, m.client)
	if err != nil {
		return nil, err
	}

	// 响应格式: {"1": {"type": "shell", "tunnel_peer": "...", ...}, ...}
	sessions := []map[string]any{}
	for key, val := range resp {
		if key == "error" || key == "error_message" {
			continue
		}
		if sess, ok := val.(map[string]any); ok {
			sess["id"] = key
			sessions = append(sessions, sess)
		}
	}

	return sessions, nil
}

// msfPassword —— 读取 MSF RPC 密码: 环境变量 VERO_MSF_PASSWORD 优先(REDCELL_MSF_PASSWORD 兼容),
// 无则用显式参数。修原版硬编码默认 "msf" 弱口令: 未配置时明确报错, 拒绝静默用弱口令。
func msfPassword(args map[string]any) string {
	if p := os.Getenv("VERO_MSF_PASSWORD"); p != "" {
		return p
	}
	if p := os.Getenv("REDCELL_MSF_PASSWORD"); p != "" {
		return p
	}
	return tools.ArgStr(args, "msf_password", "")
}

// ---------- 工具适配器 ----------

// msfSearchExploit —— 工具: 搜索 Metasploit exploit (按 CVE/关键词)。
func msfSearchExploit(args map[string]any) tools.ToolResult {
	query := tools.ArgStr(args, "query", "")
	if query == "" {
		return tools.ToolResult{Success: false, Stderr: "msf_search: 缺 query", RC: -1}
	}

	baseURL := tools.ArgStr(args, "msf_url", "http://127.0.0.1:55553")
	password := msfPassword(args)
	if password == "" {
		return tools.ToolResult{Success: false, Stderr: "msf_search: 未配置密码, 请设置环境变量 VERO_MSF_PASSWORD", RC: -1}
	}

	client, err := NewMSFClient(baseURL, password)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}

	exploits, err := client.SearchExploit(query)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}

	if len(exploits) == 0 {
		return tools.ToolResult{Success: true, Stdout: fmt.Sprintf("No exploits found for: %s", query)}
	}

	stdout := fmt.Sprintf("Found %d exploits for '%s':\n", len(exploits), query)
	for _, exp := range exploits {
		stdout += "  " + exp + "\n"
	}

	return tools.ToolResult{Success: true, Stdout: stdout}
}

// msfExecuteExploit —— 工具: 执行 Metasploit exploit。
func msfExecuteExploit(args map[string]any) tools.ToolResult {
	module := tools.ArgStr(args, "module", "")
	rhost := tools.ArgStr(args, "rhost", "")

	if module == "" || rhost == "" {
		return tools.ToolResult{Success: false, Stderr: "msf_execute: 缺 module 或 rhost", RC: -1}
	}

	baseURL := tools.ArgStr(args, "msf_url", "http://127.0.0.1:55553")
	password := msfPassword(args)
	if password == "" {
		return tools.ToolResult{Success: false, Stderr: "msf_execute: 未配置密码, 请设置环境变量 VERO_MSF_PASSWORD", RC: -1}
	}

	client, err := NewMSFClient(baseURL, password)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}

	// 构建 exploit 选项
	opts := map[string]any{
		"RHOST": rhost,
		"LHOST": tools.ArgStr(args, "lhost", "127.0.0.1"),
		"LPORT": tools.ArgStr(args, "lport", "4444"),
	}

	// 添加额外参数
	if rport := tools.ArgStr(args, "rport", ""); rport != "" {
		opts["RPORT"] = rport
	}

	jobID, err := client.ExecuteExploit(module, opts)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}

	stdout := fmt.Sprintf("Exploit %s launched (job_id: %s)\nTarget: %s\nWaiting for session...", module, jobID, rhost)
	return tools.ToolResult{Success: true, Stdout: stdout}
}

// msfGetSessions —— 工具: 列出当前 Metasploit sessions。
func msfGetSessions(args map[string]any) tools.ToolResult {
	baseURL := tools.ArgStr(args, "msf_url", "http://127.0.0.1:55553")
	password := msfPassword(args)
	if password == "" {
		return tools.ToolResult{Success: false, Stderr: "msf_get_sessions: 未配置密码, 请设置环境变量 VERO_MSF_PASSWORD", RC: -1}
	}

	client, err := NewMSFClient(baseURL, password)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}

	sessions, err := client.GetSessions()
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}

	if len(sessions) == 0 {
		return tools.ToolResult{Success: true, Stdout: "No active sessions"}
	}

	stdout := fmt.Sprintf("Active sessions (%d):\n", len(sessions))
	for _, sess := range sessions {
		stdout += fmt.Sprintf("  [%v] %v @ %v\n", sess["id"], sess["type"], sess["tunnel_peer"])
	}

	return tools.ToolResult{Success: true, Stdout: stdout}
}

// ---------- Parser ----------

// ParseMSFSearch —— 解析 msf_search 输出, 提取可用 exploit。
func ParseMSFSearch(stdout string, args map[string]any) []tools.Observation {
	query := tools.ArgStr(args, "query", "?")
	var obs []tools.Observation

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "exploit/") {
			obs = append(obs, tools.Observation{
				Kind:    "finding",
				Key:     query + ":exploit:" + line,
				Label:   fmt.Sprintf("[info] Available exploit: %s", line),
				Excerpt: line,
			})
		}
	}

	return obs
}

// ParseMSFExecute —— 解析 msf_execute 输出, 提取 shell/session。
func ParseMSFExecute(stdout string, args map[string]any) []tools.Observation {
	target := tools.ArgStr(args, "rhost", "?")

	// 如果输出含 "job_id", 说明 exploit 已启动
	if strings.Contains(stdout, "job_id") {
		return []tools.Observation{{
			Kind:    "finding",
			Key:     target + ":exploit_launched",
			Label:   "[high] Exploit launched successfully",
			Excerpt: "job_id:",
		}}
	}

	return nil
}

// ParseMSFSessions —— 解析 msf_get_sessions 输出, 提取 shell。
func ParseMSFSessions(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 匹配: [1] shell @ 192.168.1.100:4444
		if strings.HasPrefix(line, "[") && strings.Contains(line, "@") {
			obs = append(obs, tools.Observation{
				Kind:    "shell",
				Key:     "shell:" + line,
				Label:   fmt.Sprintf("[critical] Active shell: %s", line),
				Excerpt: line,
			})
		}
	}

	return obs
}

// ExploitPack —— 漏洞利用场景包: Metasploit 自动化。
func ExploitPack() Pack {
	return Pack{
		Name: "exploit",
		Tools: []*tools.Tool{
			{Name: "msf_search", Level: tools.LevelRecon,
				Desc: "Metasploit exploit 搜索, 按 CVE/关键词查找可用模块, 需 msfrpcd 运行(query 参数)",
				Run: msfSearchExploit, Parse: ParseMSFSearch},
			{Name: "msf_execute", Level: tools.LevelExploit,
				Desc: "Metasploit exploit 执行, 需 module/rhost/lhost 参数, 返回 job_id",
				Run: msfExecuteExploit, Parse: ParseMSFExecute},
			{Name: "msf_get_sessions", Level: tools.LevelRecon,
				Desc: "Metasploit session 列表, 查看当前活跃 shell/meterpreter",
				Run: msfGetSessions, Parse: ParseMSFSessions},
		},
		Fingerprint: func(s map[string]bool) bool {
			// 当发现漏洞 finding 时激活
			return s["finding"] || s["vulnerability"]
		},
	}
}
