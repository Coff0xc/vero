package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

	lastReflection string // BattleReflector: 战役级战略反思(每 N 步), 下轮注入 prompt
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
	// 战役级反思注入: 有则追加在 ReAct 轨迹之后, 让下轮决策基于证伪收敛。
	user := buildReActPromptWithLessons(goal, g, history, d.reg.Specs(), d.lessons)
	if d.lastReflection != "" {
		user += "\n\n战役反思(上一轮总结, 参考其方向):\n" + d.lastReflection
	}
	body := map[string]any{
		"model": d.model,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": user},
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
			Verifies  string         `json:"verifies"`
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
		p.Actions = append(p.Actions, core.Action{Tool: a.Tool, Args: a.Args, Rationale: a.Rationale, Claim: a.Claim, Produces: a.Produces, Verifies: a.Verifies})
	}
	return p
}

// chatText —— 无工具纯文本对话(Observe/Reflect 用): 复用 proposePlan 的
// 重试/错误处理模式; 返回 choices[0].message.content。
func (d *DeepSeekLLM) chatText(system, user string) (string, error) {
	if d.apiKey == "" {
		return "", fmt.Errorf("deepseek: 未配置 API key")
	}
	body := map[string]any{
		"model": d.model,
		"messages": []map[string]any{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"max_tokens":  1024,
		"temperature": d.temp,
	}
	raw, _ := json.Marshal(body)
	// D21 修复: out 循环外声明 + 每次重试前重置 —— 解码失败时残留上次响应字段,
	// 后续循环用旧值判断 len(Choices) 返回脏数据。
	type chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	out := chatResp{}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ { // D30: 与 proposePlan 重试次数对齐(3 次)
		out = chatResp{} // 重置: 本次响应的字段不继承上次
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, deepSeekURL, bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("deepseek: HTTP %d", resp.StatusCode)
			time.Sleep(time.Second)
			continue
		}
		derr := json.NewDecoder(resp.Body).Decode(&out)
		_ = resp.Body.Close()
		if derr != nil {
			lastErr = derr
			continue
		}
		if len(out.Choices) > 0 {
			return out.Choices[0].Message.Content, nil
		}
		lastErr = fmt.Errorf("deepseek: 空响应")
	}
	return "", lastErr
}

// Observe —— 实现 core.Observer(LLM-as-parser): 从工具原始输出提取结构化观察。
// 证据约束: parseObserveResponse 强制 excerpt 逐字存在于 stdout, 模型编造即丢弃。
func (d *DeepSeekLLM) Observe(tool string, args map[string]any, stdout string) []tools.Observation {
	if d.apiKey == "" {
		return nil
	}
	raw, err := d.chatText(observeSystem, observePrompt(tool, args, stdout))
	if err != nil {
		return nil // 静默降级: 观察失败不阻断战役
	}
	return parseObserveResponse(raw, stdout)
}

// Reflect —— 实现 core.BattleReflector(战役级反思): 战略总结缓存到 lastReflection,
// 下轮 proposePlan 注入 prompt; 返回文本供内核广播 reflect 事件。
func (d *DeepSeekLLM) Reflect(goal string, g *core.AttackGraph, history []core.HistoryItem) string {
	if d.apiKey == "" {
		return ""
	}
	txt, err := d.chatText(reflectSystem, reflectPrompt(goal, g, history))
	if err != nil || strings.TrimSpace(txt) == "" {
		return ""
	}
	d.lastReflection = tools.Clip(strings.TrimSpace(txt), 600)
	return d.lastReflection
}

// ShouldRetry —— 实现 core.Retrier: 接入 ReflexionEnhanced 的可恢复失败判断。
func (d *DeepSeekLLM) ShouldRetry(reason string) bool {
	return ShouldRetry(reason)
}

// AdjustArgsForRetry —— 实现 core.Retrier: 接入 ReflexionEnhanced 的参数自动调整。
func (d *DeepSeekLLM) AdjustArgsForRetry(action core.Action, reason string) map[string]any {
	return AdjustArgsForRetry(action, reason)
}

// Chat —— 对话式问答(对话智能): 基于战役上下文 + 多轮历史回答用户问题。
// history 为 [role, content] 对(role: user|assistant), 支持多轮对话。
func (d *DeepSeekLLM) Chat(context, question string, history [][2]string) (string, error) {
	return d.chatText(chatSystem, ChatPrompt(context, question, history))
}
