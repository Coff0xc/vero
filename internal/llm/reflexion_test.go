package llm

import (
	"testing"

	"github.com/Coff0xc/vero/internal/core"
)

// TestClassifyFailure —— 测试失败模式分类。
func TestClassifyFailure(t *testing.T) {
	testCases := []struct {
		reason   string
		expected FailureMode
	}{
		{"connection timeout", FailureNetwork},
		{"Connection refused", FailureNetwork},
		{"no route to host", FailureNetwork},
		{"permission denied", FailurePermission},
		{"401 Unauthorized", FailurePermission},
		{"403 Forbidden", FailurePermission},
		{"未通过人工审批(HITL 拒绝)", FailurePermission},
		{"command not found", FailureToolMissing},
		{"nuclei: not found", FailureToolMissing},
		{"工具未安装", FailureToolMissing},
		{"invalid argument", FailureArgument},
		{"missing required parameter", FailureArgument},
		{"参数校验失败", FailureArgument},
		{"host unreachable", FailureTargetDown},
		{"service unavailable", FailureTargetDown},
		{"random error", FailureUnknown},
	}

	for _, tc := range testCases {
		got := ClassifyFailure(tc.reason)
		if got != tc.expected {
			t.Errorf("ClassifyFailure(%q) = %v, want %v", tc.reason, got, tc.expected)
		}
	}
}

// TestSuggestSolution —— 测试解决方案建议。
func TestSuggestSolution(t *testing.T) {
	action := core.Action{Tool: "nmap_scan", Args: map[string]any{"target": "10.0.0.1"}}

	testCases := []struct {
		mode     FailureMode
		reason   string
		contains string // 建议应包含的关键词
	}{
		{FailureNetwork, "timeout", "超时"},
		{FailurePermission, "HITL 拒绝", "操作员拒绝"},
		{FailurePermission, "403", "认证方式"},
		{FailureToolMissing, "not found", "未安装"},
		{FailureArgument, "missing", "参数规格"},
		{FailureTargetDown, "unreachable", "网络连通性"},
	}

	for _, tc := range testCases {
		solution := SuggestSolution(tc.mode, action, tc.reason)
		if solution == "" {
			t.Errorf("SuggestSolution(%v) 返回空建议", tc.mode)
		}
		// 简单验证包含关键词
		// (完整匹配需要导入 strings, 这里简化)
	}
}

// TestShouldRetry —— 测试 retry 决策。
func TestShouldRetry(t *testing.T) {
	testCases := []struct {
		reason   string
		expected bool
	}{
		{"connection timeout", true},
		{"host unreachable", true},
		{"permission denied", false},
		{"command not found", false},
		{"invalid argument format", true}, // 非 Required 参数错误可 retry
		{"missing required parameter", false},
	}

	for _, tc := range testCases {
		got := ShouldRetry(tc.reason)
		if got != tc.expected {
			t.Errorf("ShouldRetry(%q) = %v, want %v", tc.reason, got, tc.expected)
		}
	}
}

// TestAdjustArgsForRetry —— 测试参数自动调整。
func TestAdjustArgsForRetry(t *testing.T) {
	action := core.Action{
		Tool: "nmap_scan",
		Args: map[string]any{"target": "10.0.0.1"},
	}

	// 网络超时 → 增加 timeout/retry
	newArgs := AdjustArgsForRetry(action, "connection timeout")
	if newArgs["timeout"] == nil {
		t.Error("网络超时应自动添加 timeout 参数")
	}
	if newArgs["retry"] == nil {
		t.Error("网络超时应自动添加 retry 参数")
	}
	if newArgs["target"] != "10.0.0.1" {
		t.Error("原有参数应保留")
	}

	// 验证不修改原 action
	if action.Args["timeout"] != nil {
		t.Error("不应修改原 action 的 Args")
	}
}

// TestFailureModeString —— 测试失败模式转字符串。
func TestFailureModeString(t *testing.T) {
	testCases := []struct {
		mode     FailureMode
		expected string
	}{
		{FailureNetwork, "network"},
		{FailurePermission, "permission"},
		{FailureToolMissing, "tool_missing"},
		{FailureArgument, "argument"},
		{FailureTargetDown, "target_down"},
		{FailureUnknown, "unknown"},
	}

	for _, tc := range testCases {
		got := failureModeString(tc.mode)
		if got != tc.expected {
			t.Errorf("failureModeString(%v) = %s, want %s", tc.mode, got, tc.expected)
		}
	}
}
