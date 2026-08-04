package tools

import (
	"strings"
	"testing"
)

// TestValidateArgs —— 参数规格校验: 缺必填拒、带必填过、未声明规格不拦(向后兼容)。
func TestValidateArgs(t *testing.T) {
	tool := &Tool{
		Name: "demo",
		Args: []ArgSpec{
			{Name: "target", Desc: "目标主机/IP", Required: true},
			{Name: "ports", Desc: "端口范围, 可选"},
		},
	}

	if msg := ValidateArgs(tool, map[string]any{}); !strings.Contains(msg, "target") {
		t.Errorf("缺必填参数应报参数名, 实际 %q", msg)
	}
	if msg := ValidateArgs(tool, map[string]any{"target": "  "}); msg == "" {
		// 注意: ArgStr 不做 trim, 纯空白当前视为有值 —— 记录行为边界
		t.Log("空白字符串视为有值(ArgStr 语义)")
	}
	if msg := ValidateArgs(tool, map[string]any{"target": "10.0.0.5"}); msg != "" {
		t.Errorf("带必填参数应通过, 实际 %q", msg)
	}
	if msg := ValidateArgs(tool, map[string]any{"target": "10.0.0.5", "ports": "1-1000"}); msg != "" {
		t.Errorf("可选参数存在也应通过, 实际 %q", msg)
	}

	// 类型不符(如 int)按缺失处理
	if msg := ValidateArgs(tool, map[string]any{"target": 12345}); msg == "" {
		t.Error("target 传非字符串应视为缺失")
	}

	// 未声明 Args 的工具不校验
	plain := &Tool{Name: "plain"}
	if msg := ValidateArgs(plain, map[string]any{}); msg != "" {
		t.Errorf("无规格工具不应被拦, 实际 %q", msg)
	}
}

// TestSpecsIncludeArgs —— Registry.Specs 应携带参数规格(供 prompt 渲染)。
func TestSpecsIncludeArgs(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&Tool{
		Name: "demo", Level: LevelScan, Desc: "演示",
		Args: []ArgSpec{{Name: "target", Desc: "目标", Required: true}},
	})
	specs := reg.Specs()
	if len(specs) != 1 || len(specs[0].Args) != 1 || specs[0].Args[0].Name != "target" {
		t.Fatalf("Specs 未携带参数规格: %+v", specs)
	}
}
