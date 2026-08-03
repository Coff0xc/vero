package llm

// 智能渗透的 LLM 端实现(共享 prompt 构造与响应解析):
//   - Observer(LLM-as-parser): 工具输出未被固定 parser 结构化时, 让模型提取观察。
//     证据约束不变: excerpt 必须逐字存在于输出 —— 解析层强制校验。
//   - BattleReflector(战役级反思): 每 reflectEvery 步让模型做战略总结。
//
// 纯函数模块, 与具体 API(claude/deepseek)解耦, 便于离线单测。

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

// observeSystem —— 观察解析器的系统指令: 强调证据逐字约束(防模型编造)。
const observeSystem = "你是渗透测试观察解析器。从工具原始输出中提取结构化观察。规则:\n" +
	"1. 只提取输出中有明确证据的发现, 不要臆测输出里不存在的东西;\n" +
	"2. excerpt 字段必须逐字复制自原始输出(不能改写/拼凑);\n" +
	"3. 输出必须是 JSON 数组, 每项: {\"kind\":\"host|service|endpoint|finding|cred\",\"key\":\"唯一标识\",\"label\":\"简短描述\",\"excerpt\":\"逐字片段\",\"severity\":\"critical|high|medium|low|info\"};\n" +
	"4. 无发现输出 []。"

// observePrompt —— 喂给模型的观察提取任务。
func observePrompt(tool string, args map[string]any, stdout string) string {
	return fmt.Sprintf("工具: %s\n参数: %v\n\n原始输出:\n```\n%s\n```\n\n提取观察(JSON 数组):",
		tool, args, tools.Clip(stdout, 4000))
}

// parseObserveResponse —— 解析模型返回的观察 JSON; 逐字校验 excerpt 存在于原始输出。
// 校验失败的条目丢弃(防模型编造证据)。返回空切片表示无有效观察。
func parseObserveResponse(raw, original string) []tools.Observation {
	raw = strings.TrimSpace(raw)
	// 容错: 模型可能包 ```json``` 代码块或前后加说明文字。
	if i := strings.Index(raw, "["); i >= 0 {
		if j := strings.LastIndex(raw, "]"); j > i {
			raw = raw[i : j+1]
		}
	}
	var items []struct {
		Kind     string `json:"kind"`
		Key      string `json:"key"`
		Label    string `json:"label"`
		Excerpt  string `json:"excerpt"`
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	var out []tools.Observation
	for _, it := range items {
		if it.Kind == "" || it.Key == "" || it.Excerpt == "" {
			continue
		}
		if !strings.Contains(original, it.Excerpt) {
			continue // 证据不在原文 -> 丢弃(幻觉防护)
		}
		out = append(out, tools.Observation{
			Kind:     it.Kind,
			Key:      it.Key,
			Label:    it.Label,
			Excerpt:  it.Excerpt,
			Severity: it.Severity,
		})
	}
	return out
}

// reflectSystem —— 战役级反思的系统指令: 证伪总结 + 策略调整。
const reflectSystem = "你是自主渗透测试的战略指挥官。回顾战役进展并给出战略反思。规则:\n" +
	"1. 明确指出哪些假设已被证伪、哪些方法无效(避免重复踩坑);\n" +
	"2. 指出当前攻击面里最值得深挖的目标与理由;\n" +
	"3. 给下一步最可能成功的 1-2 条路径建议;\n" +
	"4. 输出 3-6 句中文, 供下轮决策参考。"

// reflectPrompt —— 反思任务的上下文(攻击图 + 执行摘要)。
func reflectPrompt(goal string, g *core.AttackGraph, history []core.HistoryItem) string {
	var hb strings.Builder
	if len(history) == 0 {
		hb.WriteString("(尚无动作)")
	}
	for i, h := range history {
		fmt.Fprintf(&hb, "[%d] %s(%s) → %s\n", i+1, h.Action.Tool, briefArgs(h.Action.Args), outcomeZh(h))
	}
	return fmt.Sprintf("目标: %s\n\n当前攻击图:\n%s\n\n已执行动作摘要:\n%s\n\n战略反思:",
		goal, g.Snapshot(), hb.String())
}
