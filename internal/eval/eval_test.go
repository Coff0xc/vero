package eval

import (
	"testing"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/tools"
)

func evalReg() *tools.Registry {
	r := tools.NewRegistry()
	tools.RegisterBuiltins(r)
	return r
}

// 诚实战役: 发现的都在 oracle 里 -> 0 误报。
func TestHonestCampaign(t *testing.T) {
	honest := Evaluate(
		[]core.Action{{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}, Rationale: "scan"}},
		evalReg(),
		map[string]bool{"host:10.0.0.5": true, "service:10.0.0.5:22": true})
	if honest.FalsePositiveRate != 0.0 {
		t.Fatalf("诚实战役误报率应 0: %+v", honest)
	}
	if honest.EvidenceViolations != 0 {
		t.Fatal("不应有证据违规")
	}
	if honest.SuccessRate != 1.0 {
		t.Fatalf("全部命中 oracle: %+v", honest)
	}
}

// 有误报的战役: oracle 说 service 不该存在 -> 误报率 > 0。
func TestNoisyCampaign(t *testing.T) {
	noisy := Evaluate(
		[]core.Action{{Tool: "fake_scan", Args: map[string]any{"target": "10.0.0.5"}, Rationale: "scan"}},
		evalReg(),
		map[string]bool{"host:10.0.0.5": true})
	if noisy.FalsePositive != 1 {
		t.Fatalf("应抓到 1 个误报: %+v", noisy)
	}
	if noisy.FalsePositiveRate <= 0 {
		t.Fatal("误报率应 > 0")
	}
}
