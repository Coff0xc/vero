package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

const deepSeekURL = "https://api.deepseek.com/chat/completions"

// DeepSeekModel —— 默认模型(OpenAI 兼容 function calling); 可用 REDCELL_MODEL 覆盖。
const DeepSeekModel = "deepseek-chat"

// DeepSeekLLM —— DeepSeek 决策器: OpenAI 兼容 function calling 强制结构化输出 action。
// 复用 ReAct 上下文(buildReActPrompt) 与 systemPrompt。key 走环境变量 DEEPSEEK_API_KEY。
type DeepSeekLLM struct {
	apiKey string
	model  string
	reg    *tools.Registry
	client *http.Client
}

func NewDeepSeek(reg *tools.Registry) *DeepSeekLLM {
	model := os.Getenv("REDCELL_MODEL")
	if model == "" {
		model = DeepSeekModel
	}
	return &DeepSeekLLM{
		apiKey: os.Getenv("DEEPSEEK_API_KEY"),
		model:  model,
		reg:    reg,
		client: &http.Client{Timeout: 90 * time.Second},
	}
}

func (d *DeepSeekLLM) Propose(goal string, g *core.AttackGraph, history []core.HistoryItem) *core.Action {
	names := d.reg.Names()
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
				"description": "选择下一个要执行的 action",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool":      map[string]any{"type": "string", "enum": names},
						"args":      map[string]any{"type": "object", "description": "工具参数, 通常含 target"},
						"rationale": map[string]any{"type": "string"},
						"claim":     map[string]any{"type": "string"},
					},
					"required": []string{"tool", "args", "rationale"},
				},
			},
		}},
		"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "act"}},
		"max_tokens":  1024,
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
		Tool      string         `json:"tool"`
		Args      map[string]any `json:"args"`
		Rationale string         `json:"rationale"`
		Claim     string         `json:"claim"`
	}
	if err := json.Unmarshal([]byte(out.Choices[0].Message.ToolCalls[0].Function.Arguments), &d2); err != nil {
		return nil
	}
	if d2.Args == nil {
		d2.Args = map[string]any{}
	}
	return &core.Action{Tool: d2.Tool, Args: d2.Args, Rationale: d2.Rationale, Claim: d2.Claim}
}
