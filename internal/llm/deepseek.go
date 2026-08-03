package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

const deepSeekURL = "https://api.deepseek.com/chat/completions"

// DeepSeekModel —— 默认模型(OpenAI 兼容 function calling); 可用 VERO_MODEL 覆盖(兼容 REDCELL_MODEL)。
const DeepSeekModel = "deepseek-chat"

// DeepSeekLLM —— DeepSeek 决策器: OpenAI 兼容 function calling 强制结构化输出 action。
// 复用 ReAct 上下文(buildReActPrompt) 与 systemPrompt。key 走环境变量 DEEPSEEK_API_KEY。
type DeepSeekLLM struct {
	apiKey  string
	model   string
	temp    float64
	reg     *tools.Registry
	client  *http.Client
	lastErr string   // 最近一次决策失败原因(API 错误/无效模型等), 供内核暴露给前端
	lessons []lesson // 结构化反思教训(Reflector.OnFailure 收集, 注入后续决策 prompt)
}

// NewDeepSeek —— apiKey 为空则从 DEEPSEEK_API_KEY 环境变量读。temp 为思考强度(0~1)。
func NewDeepSeek(reg *tools.Registry, apiKey string, temp float64) *DeepSeekLLM {
	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	model := modelFromEnv()
	if model == "" {
		model = DeepSeekModel
	}
	return &DeepSeekLLM{
		apiKey: apiKey,
		model:  model,
		temp:   temp,
		reg:    reg,
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

// LastError —— 实现 core.ErrorReporter: 返回最近一次决策失败原因(供内核向前端暴露)。
func (d *DeepSeekLLM) LastError() string { return d.lastErr }

// OnFailure —— 实现 core.Reflector: 动作失败/被拒时回传动作与精确原因,
// 记入反思记忆(与 ClaudeLLM 同构); 教训在下一轮 proposePlan 注入 prompt。
func (d *DeepSeekLLM) OnFailure(action core.Action, reason string) {
	d.lessons = recordLesson(d.lessons, action, reason)
}

func (d *DeepSeekLLM) Propose(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Action {
	p := d.proposePlan(goal, g, history)
	if p == nil || len(p.Actions) == 0 {
		return nil
	}
	return &p.Actions[0]
}

// ProposePlan —— 多步规划: 一次请求输出整段有序计划(核心增强, #44)。
func (d *DeepSeekLLM) ProposePlan(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Plan {
	return d.proposePlan(goal, g, history)
}

func (d *DeepSeekLLM) proposePlan(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Plan {
	props, required := actSchema(d.reg.Names())
	body := map[string]any{
		"model": d.model,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": buildReActPromptWithLessons(goal, g, history, d.reg.Specs(), d.lessons)},
		},
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name":        "act",
				"description": "选择下一段要执行的动作计划",
				"parameters": map[string]any{
					"type":       "object",
					"properties": props,
					"required":   required,
				},
			},
		}},
		"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "act"}},
		"max_tokens":  2048,
		"temperature": d.temp,
	}
	raw, _ := json.Marshal(body)
	var out struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	ok := false
	for attempt := 0; attempt < 3; attempt++ { // 抗网络抖动: 失败重试, 不因单次波动中断战役
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, deepSeekURL, bytes.NewReader(raw))
		if err != nil {
			d.lastErr = "DeepSeek API 请求构造失败: " + err.Error()
			return nil
		}
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := d.client.Do(req)
		if err != nil {
			d.lastErr = "DeepSeek API 请求失败: " + err.Error()
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		// 检查状态码: 401/403(密钥问题)与 5xx/429(服务端问题)不再静默当作"没动作"。
		// 密钥类错误重试无意义, 直接放弃并告警; 服务端错误退避重试。
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			_ = resp.Body.Close()
			d.lastErr = fmt.Sprintf("DeepSeek API 拒绝访问 (HTTP %d), 请检查 API key", resp.StatusCode)
			fmt.Fprintf(os.Stderr, "[deepseek] %s\n", d.lastErr)
			return nil
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			d.lastErr = fmt.Sprintf("DeepSeek API 返回 HTTP %d (可能模型名无效, 应使用 deepseek-chat / deepseek-reasoner)", resp.StatusCode)
			fmt.Fprintf(os.Stderr, "[deepseek] HTTP %d, 第 %d 次重试\n", resp.StatusCode, attempt+1)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		derr := json.NewDecoder(resp.Body).Decode(&out)
		_ = resp.Body.Close()
		if derr == nil {
			ok = true
			break
		}
		d.lastErr = "DeepSeek API 响应解析失败: " + derr.Error()
	}
	if !ok || len(out.Choices) == 0 || len(out.Choices[0].Message.ToolCalls) == 0 {
		if d.lastErr == "" {
			d.lastErr = "DeepSeek 未返回有效动作计划(检查模型名/密钥/网络)"
		}
		return nil
	}

	var d2 struct {
		Rationale string `json:"rationale"`
		Plan      []struct {
			Tool      string         `json:"tool"`
			Args      map[string]any `json:"args"`
			Rationale string         `json:"rationale"`
			Claim     string         `json:"claim"`
			Produces  string         `json:"produces"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(out.Choices[0].Message.ToolCalls[0].Function.Arguments), &d2); err != nil {
		d.lastErr = "DeepSeek 返回的动作计划无法解析: " + err.Error()
		return nil
	}
	p := &core.Plan{Rationale: d2.Rationale}
	for _, a := range d2.Plan {
		if a.Args == nil {
			a.Args = map[string]any{}
		}
		p.Actions = append(p.Actions, core.Action{Tool: a.Tool, Args: a.Args, Rationale: a.Rationale, Claim: a.Claim, Produces: a.Produces})
	}
	return p
}
