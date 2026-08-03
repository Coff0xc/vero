package llm

import (
	"strings"

	"github.com/Coff0xc/vero/internal/core"
)

// lesson —— 一条结构化反思教训(Reflector.OnFailure 收集, Reflexion/RedAgent 模式)。
// 与 history 推导的教训块(lessonsBlock)互补: OnFailure 由内核在
// 未知工具 / HITL 拒绝 / 执行失败三个分支主动回传, 携带精确失败原因
// (如 "unknown tool: X"、"未通过人工审批"、stderr 首行) ——
// 这些原因在 HistoryItem 里不存(被拒项无 Result), 只有主动回调拿得到。
type lesson struct {
	tool   string
	args   map[string]any // 浅拷贝, 防与历史项共享 map 被外部改写
	reason string
}

// maxLessons —— 教训记忆上限: 超限丢最旧(防 prompt 无限膨胀)。
const maxLessons = 8

// recordLesson —— 记一条教训: 同工具只保留最新(去重, 防重复失败刷屏);
// 超上限丢最旧。返回新的教训切片(纯函数, 调用方赋回结构体字段)。
func recordLesson(ls []lesson, a core.Action, reason string) []lesson {
	if reason == "" {
		reason = "未知原因"
	}
	args := map[string]any{}
	for k, v := range a.Args {
		args[k] = v
	}
	l := lesson{tool: a.Tool, args: args, reason: oneline(reason, 200)}
	// 同工具去重: 移除旧条目, 追加最新(保最近顺序)。
	for i, cur := range ls {
		if cur.tool == l.tool {
			ls = append(ls[:i], ls[i+1:]...)
			break
		}
	}
	ls = append(ls, l)
	if len(ls) > maxLessons {
		ls = ls[len(ls)-maxLessons:] // 丢最旧, 保留最近 maxLessons 条
	}
	return ls
}

// lessonsText —— 把教训列表渲染成提示词块, 追加在"已执行动作与观察"之后。
// 无教训返回空串(不输出该块, 与 lessonsBlock 空串语义一致)。
// 措辞刻意不用"必须避免": 教训只增不减(成功路径无回调清除), 一次瞬时失败不应永久毒化工具选择 ——
// 该工具后续成功的证据在 ReAct 轨迹里可见, 模型可自行推翻教训。
func lessonsText(ls []lesson) string {
	if len(ls) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("失败教训(反思记忆, 供参考; 原因可能已解除, 若轨迹显示该工具后续成功可忽略):\n")
	for _, l := range ls {
		b.WriteString("  - " + l.tool + "(" + briefArgs(l.args) + ") 失败原因: " + l.reason + "\n")
	}
	return b.String()
}
