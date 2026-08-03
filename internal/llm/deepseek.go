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
	apiKey string
	model  string
	temp   float64
	reg    *tools.Registry
	client *http.Client
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
			{"role": "user", "content": buildReActPrompt(goal, g, history, d.reg.Specs())},
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
			return nil
		}
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := d.client.Do(req)
		if err != nil {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		// 检查状态码: 401/403(密钥问题)与 5xx/429(服务端问题)不再静默当作"没动作"。
		// 密钥类错误重试无意义, 直接放弃并告警; 服务端错误退避重试。
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			_ = resp.Body.Close()
			fmt.Fprintf(os.Stderr, "[deepseek] API 拒绝访问 (HTTP %d): 检查 DEEPSEEK_API_KEY\n", resp.StatusCode)
			return nil
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			fmt.Fprintf(os.Stderr, "[deepseek] API 返回 HTTP %d, 第 %d 次重试\n", resp.StatusCode, attempt+1)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		derr := json.NewDecoder(resp.Body).Decode(&out)
		_ = resp.Body.Close()
		if derr == nil {
			ok = true
			break
		}
	}
	if !ok || len(out.Choices) == 0 || len(out.Choices[0].Message.ToolCalls) == 0 {
		return nil
	}

	var d2 struct {
		Rationale string `json:"rationale"`
		Plan      []struct {
			Tool      string         `json:"tool"`
			Args      map[string]any `json:"args"`
			Rationale string         `json:"rationale"`
			Claim     string         `json:"claim"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(out.Choices[0].Message.ToolCalls[0].Function.Arguments), &d2); err != nil {
		return nil
	}
	p := &core.Plan{Rationale: d2.Rationale}
	for _, a := range d2.Plan {
		if a.Args == nil {
			a.Args = map[string]any{}
		}
		p.Actions = append(p.Actions, core.Action{Tool: a.Tool, Args: a.Args, Rationale: a.Rationale, Claim: a.Claim})
	}
	return p
}
