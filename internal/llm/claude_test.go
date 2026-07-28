package llm

import (
	"strings"
	"testing"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

// ReAct 的核心是"喂给模型的上下文": 完整轨迹 + 每步真实观察 + 成败标记。
// 不联网也能验证这份上下文构建正确 —— 这是 ClaudeLLM 决策质量的地基。
func TestBuildReActPromptCarriesObservations(t *testing.T) {
	g := core.NewAttackGraph()
	g.UpsertNode(&core.Node{ID: "host:10.0.0.5", Type: "host", Label: "10.0.0.5"})
	ok := tools.ToolResult{Success: true, Stdout: "Host 10.0.0.5 is up\n80/tcp open http"}
	fail := tools.ToolResult{Success: false, Stderr: "SMB signing required"}
	history := []core.HistoryItem{
		{Outcome: "done", Action: core.Action{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}}, Result: &ok},
		{Outcome: "done", Action: core.Action{Tool: "psexec_smb", Args: map[string]any{"target": "10.0.0.5"}}, Result: &fail},
	}
	p := buildReActPrompt("拿下 foothold", g, history, []tools.ToolSpec{{Name: "fake_scan"}, {Name: "wmiexec"}})

	if !strings.Contains(p, "80/tcp open http") {
		t.Fatal("轨迹应携带成功动作的真实观察(模型才能看到发生了什么)")
	}
	if !strings.Contains(p, "SMB signing required") {
		t.Fatal("轨迹应携带失败动作的观察, 供模型换路")
	}
	if !strings.Contains(p, "失败") {
		t.Fatal("失败动作应标注失败, 引导模型反思换路而非重试")
	}
	if !strings.Contains(p, "fake_scan") || !strings.Contains(p, "psexec_smb") {
		t.Fatal("轨迹应含已执行动作序列")
	}
}

func TestBuildReActPromptEmptyHistory(t *testing.T) {
	g := core.NewAttackGraph()
	p := buildReActPrompt("目标", g, nil, []tools.ToolSpec{{Name: "fake_scan"}})
	if !strings.Contains(p, "尚未执行") {
		t.Fatal("空轨迹应提示尚未执行任何动作")
	}
	if !strings.Contains(p, "fake_scan") {
		t.Fatal("应列出可用工具")
	}
}
