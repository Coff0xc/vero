package llm

import (
	"testing"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

// TestClaudeImplementsRetrier —— U3 回归: ClaudeLLM 必须实现 core.Retrier,
// 否则 loop.go 的 `llm.(Retrier)` 类型断言对 Claude 后端恒 false, 工具静默失败永不重试。
// (此前仅 DeepSeekLLM 实现, Claude 后端 U3 修复完全失效。)
func TestClaudeImplementsRetrier(t *testing.T) {
	var c core.LLM = NewClaude(tools.NewRegistry(), "test-key", 0.2)
	rt, ok := c.(core.Retrier)
	if !ok {
		t.Fatal("ClaudeLLM 必须实现 core.Retrier (ShouldRetry + AdjustArgsForRetry)")
	}
	// "无输出"(curl -sI 静默失败)应判为可重试(U3: 归类 FailureTargetDown)。
	if !rt.ShouldRetry("工具失败(无输出)") {
		t.Error("Claude.ShouldRetry(\"工具失败(无输出)\") 应为 true (U3 静默失败重试)")
	}
	// 明确不可重试的失败(工具未安装)应为 false。
	if rt.ShouldRetry("command not found") {
		t.Error("Claude.ShouldRetry(\"command not found\") 应为 false")
	}
}

// TestClassifyNoOutputRetryable —— U3 回归: "无输出" reason 必须归类为可重试。
func TestClassifyNoOutputRetryable(t *testing.T) {
	for _, reason := range []string{"工具失败(无输出)", "tool failed (no output)", "empty output", ""} {
		if ClassifyFailure(reason) != FailureTargetDown {
			t.Errorf("ClassifyFailure(%q) 应为 FailureTargetDown", reason)
		}
		if !ShouldRetry(reason) {
			t.Errorf("ShouldRetry(%q) 应为 true", reason)
		}
	}
}
