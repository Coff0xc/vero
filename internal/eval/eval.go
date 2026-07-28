// Package eval —— 离线 eval + 核心 KPI(对应 Python eval.py)。
// 用 MockLLM 跑一个战役, 对照 oracle(已知真值)算 KPI。误报率 = 命门。
package eval

import (
	"math"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/llm"
	"github.com/Coff0xc/vero/internal/tools"
)

// KPI —— 一次战役的评测结果。
type KPI struct {
	Confirmed          int     `json:"confirmed"`
	TruePositive       int     `json:"true_positive"`
	FalsePositive      int     `json:"false_positive"`
	FalsePositiveRate  float64 `json:"false_positive_rate"`
	EvidenceViolations int     `json:"evidence_violations"`
	SuccessRate        float64 `json:"success_rate"`
}

// Evaluate —— 跑 MockLLM 脚本战役, 对照 oracle(真实存在的 confirmed 节点 id 集合)算 KPI。
func Evaluate(script []core.Action, reg *tools.Registry, oracle map[string]bool) KPI {
	g, trace := core.RunAgent("eval", llm.NewMock(script), reg, core.AutoApprove, core.DiscardEmit, 30)

	confirmed := map[string]bool{}
	for _, n := range g.Nodes {
		if n.State == core.StateConfirmed {
			confirmed[n.ID] = true
		}
	}
	tp, fp := 0, 0
	for id := range confirmed {
		if oracle[id] {
			tp++
		} else {
			fp++
		}
	}
	violations := core.VerifyEvidence(g, trace)
	return KPI{
		Confirmed:          len(confirmed),
		TruePositive:       tp,
		FalsePositive:      fp,
		FalsePositiveRate:  round3(float64(fp) / float64(max(len(confirmed), 1))),
		EvidenceViolations: len(violations),
		SuccessRate:        round3(float64(tp) / float64(max(len(oracle), 1))),
	}
}

func round3(f float64) float64 { return math.Round(f*1000) / 1000 }
