package llm

// OpenAILLM —— 统一的 OpenAI 兼容决策器(多提供商: DeepSeek/Kimi/通义/Ollama/OpenRouter/OpenAI 等)。
//
// 与 DeepSeekLLM 的区别: baseURL/apiKey/model 来自 config.Provider(用户可在设置面板
// 填入任意 OpenAI 兼容端点 + key, 自动拉取模型列表), 而非写死 deepseek。
// 能力: function calling 多步规划(act schema) + Observe(LLM-as-parser) +
// Reflect(战役反思) + Chat(对话问答) + 深度思考(reasoning_content 透传)。
//
// 深度思考: 当 provider.Reasoning 时, 请求用 reasoning 模型族(如 deepseek-reasoner),
// 响应的 reasoning_content 存入 lastThinking, 内核广播 thinking 事件供前端展示思考过程。
// 联网: 部分聚合商支持 web_search 参数, 透传(provider.WebSearch 时附加)。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/config"
	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

// OpenAILLM —— 通用 OpenAI 兼容决策器。
type OpenAILLM struct {
	prov   *config.Provider
	temp   float64
	reg    *tools.Registry
	client *http.Client
	lastErr string

	lessons        []lesson
	lastReflection string
	lastThinking   string // 深度思考: 最近一次决策的 reasoning_content
}

// NewOpenAI —— 用配置的提供商构造决策器(reg 提供工具 allowlist)。
func NewOpenAI(p *config.Provider, reg *tools.Registry, temp float64) *OpenAILLM {
	return &OpenAILLM{
		prov:   p,
		temp:   temp,
		reg:    reg,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

// LastError —— 实现 core.ErrorReporter。
func (o *OpenAILLM) LastError() string { return o.lastErr }

// LastThinking —— 最近一次决策的思维链(深度思考, 供内核广播 thinking 事件)。
func (o *OpenAILLM) LastThinking() string { return o.lastThinking }

// ThinkingReporter —— 可选能力: 决策器最近一次决策的思维链(深度思考展示)。
type ThinkingReporter interface {
	LastThinking() string
}

func (o *OpenAILLM) Propose(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Action {
	p := o.proposePlan(goal, g, history)
	if p == nil || len(p.Actions) == 0 {
		return nil
	}
	return &p.Actions[0]
}

func (o *OpenAILLM) ProposePlan(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Plan {
	return o.proposePlan(goal, g, history)
}

// chatEndpoint —— 该提供商的 chat/completions 端点(baseURL 容错: 已含 /v1 或补 /v1)。
func (o *OpenAILLM) chatEndpoint() string {
	base := strings.TrimRight(o.prov.BaseURL, "/")
	if strings.HasSuffix(base, "/v1") || strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

// postChat —— 一次 chat/completions 请求(带重试/错误归一化)。
// 返回完整响应体(map)与原始 content; reasoning 模型时同时填 lastThinking。
func (o *OpenAILLM) postChat(body map[string]any) (map[string]any, error) {
	raw, _ := json.Marshal(body)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, o.chatEndpoint(), bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if o.prov.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+o.prov.APIKey)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := o.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("API 拒绝访问 (HTTP %d), 请检查 key", resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var eb struct{ Error struct{ Message string `json:"message"` } `json:"error"` }
			_ = json.NewDecoder(resp.Body).Decode(&eb)
			_ = resp.Body.Close()
			msg := eb.Error.Message
			if msg == "" {
				msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			lastErr = fmt.Errorf("API 返回错误: %s", msg)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		var out map[string]any
		derr := json.NewDecoder(resp.Body).Decode(&out)
		_ = resp.Body.Close()
		if derr != nil {
			lastErr = derr
			continue
		}
		return out, nil
	}
	return nil, lastErr
}

// extractContentAndThinking —— 从响应提取 content 与 reasoning_content(深度思考)。
func extractContentAndThinking(out map[string]any) (string, string, error) {
	choices, ok := out["choices"].([]any)
	if !ok || len(choices) == 0 {
		return "", "", fmt.Errorf("空响应(检查模型名/端点)")
	}
	msg, ok := choices[0].(map[string]any)["message"].(map[string]any)
	if !ok {
		return "", "", fmt.Errorf("响应缺 message")
	}
	content, _ := msg["content"].(string)
	thinking, _ := msg["reasoning_content"].(string)
	return content, thinking, nil
}

func (o *OpenAILLM) proposePlan(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Plan {
	props, required := actSchema(o.reg.Names())
	user := buildReActPromptWithLessons(goal, g, history, o.reg.Specs(), o.lessons)
	if o.lastReflection != "" {
		user += "\n\n战役反思(上一轮总结, 参考其方向):\n" + o.lastReflection
	}
	body := map[string]any{
		"model":       o.prov.Model,
		"messages":    []map[string]any{{"role": "system", "content": systemPrompt}, {"role": "user", "content": user}},
		"tools":       []map[string]any{{"type": "function", "function": map[string]any{"name": "act", "description": "选择下一段要执行的动作计划", "parameters": map[string]any{"type": "object", "properties": props, "required": required}}}},
		"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "act"}},
		"max_tokens":  2048,
		"temperature": o.temp,
	}
	if o.prov.Reasoning { // 深度思考: 某些提供商用 effort 字段/或模型名即推理族
		body["reasoning_effort"] = "medium"
		// thinking 模式不支持强制 function 调用(DeepSeek: "Thinking mode does not
		// support this tool_choice") —— 放开为 auto, 模型自行决定; 未走 tool_calls
		// 时下方有 content JSON 解析兜底。
		body["tool_choice"] = "auto"
		// 推理 token 与计划 JSON 共享 max_tokens: 2048 易被思维链占满导致 arguments
		// 截断(unexpected end of JSON input), 推理模型放宽。
		body["max_tokens"] = 8192
	}
	out, err := o.postChat(body)
	if err != nil {
		o.lastErr = "模型请求失败: " + err.Error()
		fmt.Fprintf(os.Stderr, "[openai:%s] %s\n", o.prov.ID, o.lastErr) // headless 排障: 后端 stderr 可见
		return nil
	}
	content, thinking, err := extractContentAndThinking(out)
	if err != nil {
		o.lastErr = err.Error()
		return nil
	}
	o.lastThinking = thinking // 深度思考思维链(前端展示)

	// function calling 输出在 tool_calls[0].function.arguments。
	tc, _ := out["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["tool_calls"].([]any)
	var argsJSON string
	if len(tc) > 0 {
		fn, _ := tc[0].(map[string]any)["function"].(map[string]any)
		argsJSON, _ = fn["arguments"].(string)
	}
	if argsJSON == "" {
		// 模型没走 function calling 而直接给文本: 尝试按 JSON 解析 content。
		argsJSON = content
	}
	var d struct {
		Rationale string `json:"rationale"`
		Plan      []struct {
			Tool      string         `json:"tool"`
			Args      map[string]any `json:"args"`
			Rationale string         `json:"rationale"`
			Claim     string         `json:"claim"`
			Produces  string         `json:"produces"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &d); err != nil {
		o.lastErr = "返回的动作计划无法解析: " + err.Error()
		fmt.Fprintf(os.Stderr, "[openai:%s] %s | 原始输出: %.200s\n", o.prov.ID, o.lastErr, argsJSON)
		return nil
	}
	p := &core.Plan{Rationale: d.Rationale}
	for _, a := range d.Plan {
		if a.Args == nil {
			a.Args = map[string]any{}
		}
		p.Actions = append(p.Actions, core.Action{Tool: a.Tool, Args: a.Args, Rationale: a.Rationale, Claim: a.Claim, Produces: a.Produces})
	}
	return p
}

// chatText —— 无工具纯文本对话(Observe/Reflect/Chat 用)。
func (o *OpenAILLM) chatText(system, user string) (string, error) {
	body := map[string]any{
		"model":    o.prov.Model,
		"messages": []map[string]any{{"role": "system", "content": system}, {"role": "user", "content": user}},
		"max_tokens": 1024,
		"temperature": o.temp,
	}
	if o.prov.Reasoning {
		body["reasoning_effort"] = "medium"
	}
	if o.prov.WebSearch { // 联网: 透传提供商支持的 web_search 标记(不支持时模型忽略/报错回退)
		body["web_search"] = true
	}
	out, err := o.postChat(body)
	if err != nil {
		return "", err
	}
	content, thinking, err := extractContentAndThinking(out)
	if err != nil {
		return "", err
	}
	if thinking != "" {
		o.lastThinking = thinking
	}
	return content, nil
}

// OnFailure —— 实现 core.Reflector(失败教训记忆)。
func (o *OpenAILLM) OnFailure(action core.Action, reason string) {
	o.lessons = recordLesson(o.lessons, action, reason)
}

// Observe —— 实现 core.Observer(LLM-as-parser)。
func (o *OpenAILLM) Observe(tool string, args map[string]any, stdout string) []tools.Observation {
	raw, err := o.chatText(observeSystem, observePrompt(tool, args, stdout))
	if err != nil {
		return nil
	}
	return parseObserveResponse(raw, stdout)
}

// Reflect —— 实现 core.BattleReflector(战役级反思)。
func (o *OpenAILLM) Reflect(goal string, g *core.AttackGraph, history []core.HistoryItem) string {
	txt, err := o.chatText(reflectSystem, reflectPrompt(goal, g, history))
	if err != nil || strings.TrimSpace(txt) == "" {
		return ""
	}
	o.lastReflection = tools.Clip(strings.TrimSpace(txt), 600)
	return o.lastReflection
}

// Chat —— 实现 core.Chatter(对话式问答)。
func (o *OpenAILLM) Chat(context, question string, history [][2]string) (string, error) {
	return o.chatText(chatSystem, ChatPrompt(context, question, history))
}

// ---- 提供商连接测试(设置面板用) ----

// ListModels —— GET {base}/models, 返回可用模型 id 列表(自动获取模型下拉)。
func ListModels(p *config.Provider) ([]string, error) {
	base := strings.TrimRight(p.BaseURL, "/")
	var ep string
	if strings.HasSuffix(base, "/v1") || strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") {
		ep = base + "/models"
	} else {
		ep = base + "/v1/models"
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ep, nil)
	if err != nil {
		return nil, err
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d (检查 base_url / key)", resp.StatusCode)
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("响应解析失败(该端点可能不兼容 OpenAI /models): %v", err)
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("该端点未返回任何模型")
	}
	return ids, nil
}

// CheckEnv —— 环境变量兜底(旧 DEEPSEEK_API_KEY 迁移用)。
var _ = os.Getenv
