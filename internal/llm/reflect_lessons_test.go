package llm

import (
	"strings"
	"testing"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

// Reflector 主动反思(OnFailure): 内核回传的动作+精确原因应进入教训记忆,
// 并在后续决策 prompt 中可见 —— 这是 Reflexion 模式的核心闭环。

func TestOnFailureAccumulatesLessons(t *testing.T) {
	c := NewClaude(tools.NewRegistry(), "", 0.2)
	if _, ok := any(c).(core.Reflector); !ok {
		t.Fatal("ClaudeLLM 应实现 core.Reflector")
	}
	d := NewDeepSeek(tools.NewRegistry(), "", 0.2)
	if _, ok := any(d).(core.Reflector); !ok {
		t.Fatal("DeepSeekLLM 应实现 core.Reflector")
	}

	c.OnFailure(core.Action{Tool: "nmap_scan", Args: map[string]any{"target": "10.0.0.5"}}, "unknown tool: nmap_scan")
	c.OnFailure(core.Action{Tool: "psexec_smb", Args: map[string]any{"target": "10.0.0.5"}}, "SMB signing required")
	if len(c.lessons) != 2 {
		t.Fatalf("应累积 2 条教训, 实际 %d", len(c.lessons))
	}
	d.OnFailure(core.Action{Tool: "nmap_scan", Args: map[string]any{"target": "10.0.0.5"}}, "unknown tool: nmap_scan")
	if len(d.lessons) != 1 {
		t.Fatalf("DeepSeek 应累积 1 条教训, 实际 %d", len(d.lessons))
	}
}

// 教训进入 prompt: buildReActPromptWithLessons 应在"已执行动作与观察"之后
// 追加"失败教训(反思记忆)"块, 且携带工具/参数摘要/原因。
func TestLessonsInjectedIntoPrompt(t *testing.T) {
	g := core.NewAttackGraph()
	lessons := []lesson{
		{tool: "psexec_smb", args: map[string]any{"target": "10.0.0.5"}, reason: "SMB signing required"},
		{tool: "wmiexec", args: map[string]any{"target": "10.0.0.5"}, reason: "未通过人工审批(HITL 拒绝)"},
	}
	p := buildReActPromptWithLessons("拿下 foothold", g, nil, []tools.ToolSpec{{Name: "nmap_scan"}}, lessons)

	if !strings.Contains(p, "失败教训(反思记忆") {
		t.Fatal("提示词应注入失败教训块")
	}
	if !strings.Contains(p, "psexec_smb(target=10.0.0.5) 失败原因: SMB signing required") {
		t.Fatal("教训块应含工具/参数摘要/精确原因")
	}
	if !strings.Contains(p, "wmiexec(target=10.0.0.5) 失败原因: 未通过人工审批(HITL 拒绝)") {
		t.Fatal("教训块应含 HITL 拒绝原因(history 推导拿不到的信息)")
	}
	// 位置: 教训块在轨迹之后、当前攻击图之前; 且不影响原"上轮教训"块(history 推导)。
	ti, li, ci := strings.Index(p, "已执行动作"), strings.Index(p, "失败教训(反思记忆"), strings.Index(p, "当前攻击图")
	if ti < 0 || li < 0 || ci < 0 || !(ti < li && li < ci) {
		t.Fatal("教训块应位于'已执行动作'之后、'当前攻击图'之前")
	}
}

// 无教训时不输出反思块(空串), 与既有 prompt 行为完全一致。
func TestLessonsEmptyBlockOmitted(t *testing.T) {
	g := core.NewAttackGraph()
	p := buildReActPromptWithLessons("目标", g, nil, []tools.ToolSpec{{Name: "nmap_scan"}}, nil)
	if strings.Contains(p, "失败教训(反思记忆") {
		t.Fatal("无教训时不应输出反思块")
	}
	if !strings.Contains(p, "尚未执行") {
		t.Fatal("空轨迹提示应保留")
	}
}

// 同工具去重: 同一工具多次失败只保留最新原因, 防 prompt 膨胀。
func TestLessonDedupByTool(t *testing.T) {
	c := NewClaude(tools.NewRegistry(), "", 0.2)
	c.OnFailure(core.Action{Tool: "nmap_scan", Args: map[string]any{"target": "10.0.0.5"}}, "timeout")
	c.OnFailure(core.Action{Tool: "nmap_scan", Args: map[string]any{"target": "10.0.0.6"}}, "permission denied")
	if len(c.lessons) != 1 {
		t.Fatalf("同工具应只保留 1 条最新教训, 实际 %d", len(c.lessons))
	}
	if c.lessons[0].reason != "permission denied" {
		t.Fatalf("应保留最新原因, 实际 %q", c.lessons[0].reason)
	}
	// 不同工具互不影响。
	c.OnFailure(core.Action{Tool: "psexec_smb", Args: map[string]any{"target": "10.0.0.5"}}, "SMB signing required")
	if len(c.lessons) != 2 {
		t.Fatalf("不同工具应各留 1 条, 实际 %d", len(c.lessons))
	}
}

// 上限: 超过 maxLessons 丢最旧(只保留最近 8 条)。
func TestLessonCapDropsOldest(t *testing.T) {
	var ls []lesson
	for i := 0; i < maxLessons+3; i++ {
		ls = recordLesson(ls, core.Action{Tool: "t" + string(rune('a'+i%26)), Args: map[string]any{"n": i}}, "r")
	}
	if len(ls) != maxLessons {
		t.Fatalf("教训应封顶 %d 条, 实际 %d", maxLessons, len(ls))
	}
	// 丢最旧: 前 3 条(索引 0..2)应被淘汰, 最新 3 条应保留。
	kept := map[string]bool{}
	for _, l := range ls {
		kept[l.tool] = true
	}
	for _, dropped := range []string{"ta", "tb", "tc"} {
		if kept[dropped] {
			t.Fatalf("最旧教训 %s 应被丢弃", dropped)
		}
	}
	// 最新一条(i=maxLessons+2 → 'k')必须在。
	if !kept["tk"] {
		t.Fatal("最新教训应保留")
	}
}

// args 浅拷贝: 记录后外部改原始 map 不影响已存教训(防共享 map 别名污染)。
func TestLessonArgsCopied(t *testing.T) {
	args := map[string]any{"target": "10.0.0.5"}
	var ls []lesson
	ls = recordLesson(ls, core.Action{Tool: "nmap_scan", Args: args}, "timeout")
	args["target"] = "10.0.0.6" // 外部改写
	if ls[0].args["target"] != "10.0.0.5" {
		t.Fatal("教训应持有 args 拷贝, 不受外部改写影响")
	}
}
