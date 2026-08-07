package llm

import (
	"testing"

	"github.com/Coff0xc/vero/internal/core"
)

func TestMockLLMProposePlan(t *testing.T) {
	script := []core.Action{
		{Tool: "port_scan", Args: map[string]any{"target": "example.com"}, Rationale: "端口扫描"},
		{Tool: "http_probe", Args: map[string]any{"target": "example.com"}, Rationale: "HTTP 指纹"},
	}
	mock := NewMock(script)

	// 第一次调用 ProposePlan 应该返回所有动作
	plan := mock.ProposePlan("test", nil, nil)
	if plan == nil {
		t.Fatal("ProposePlan 返回 nil, 应该返回所有动作")
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("期望 2 个动作, 实际 %d 个", len(plan.Actions))
	}
	if plan.Rationale == "" {
		t.Fatal("计划 Rationale 不应为空")
	}

	// 第二次调用应该返回 nil (脚本已执行完)
	plan2 := mock.ProposePlan("test", nil, nil)
	if plan2 != nil {
		t.Fatal("第二次 ProposePlan 应该返回 nil, 表示脚本完成")
	}
}

func TestMockLLMProposeSingle(t *testing.T) {
	script := []core.Action{
		{Tool: "port_scan", Args: map[string]any{"target": "example.com"}, Rationale: "端口扫描"},
	}
	mock := NewMock(script)

	// 由于实现了 Planner 接口, Propose 也应该返回一个动作
	a := mock.Propose("test", nil, nil)
	if a == nil {
		t.Fatal("Propose 不应返回 nil")
	}
	if a.Tool != "port_scan" {
		t.Fatalf("期望 port_scan, 实际 %s", a.Tool)
	}
}

func TestMockLLMOnReject(t *testing.T) {
	mock := NewMock(nil)

	// 初始没有错误
	if mock.LastError() != "" {
		t.Fatal("初始 LastError 应该为空")
	}

	// 调用 OnReject 后应该有错误消息
	mock.OnReject()
	if mock.LastError() == "" {
		t.Fatal("OnReject 后 LastError 不应为空")
	}
}

func TestMockLLMOnFailure(t *testing.T) {
	mock := NewMock(nil)

	// 引擎级错误不应设置错误消息
	mock.OnFailure(core.Action{Tool: "engine"}, "unknown tool: engine")
	if mock.LastError() != "" {
		t.Fatal("引擎级错误不应设置 LastError")
	}

	// 其他未知工具应设置错误消息
	mock.OnFailure(core.Action{Tool: "unknown_tool"}, "unknown tool: unknown_tool")
	if mock.LastError() == "" {
		t.Fatal("未知工具错误应设置 LastError")
	}
}

func TestMockLLMShouldRetry(t *testing.T) {
	mock := NewMock(nil)

	// timeout 应该重试
	if !mock.ShouldRetry("connection timeout") {
		t.Fatal("timeout 应该重试")
	}

	// 其他错误不应重试
	if mock.ShouldRetry("unknown tool") {
		t.Fatal("unknown tool 不应重试")
	}
}

func TestMockLLMEmptyScript(t *testing.T) {
	mock := NewMock(nil)

	// 空脚本应该立即返回 nil
	plan := mock.ProposePlan("test", nil, nil)
	if plan != nil {
		t.Fatal("空脚本 ProposePlan 应该返回 nil")
	}

	a := mock.Propose("test", nil, nil)
	if a != nil {
		t.Fatal("空脚本 Propose 应该返回 nil")
	}
}
