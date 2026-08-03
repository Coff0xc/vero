package main

import (
	"encoding/json"
	"testing"

	"github.com/Coff0xc/vero/internal/core"
)

// mustNode —— 构造一个 confirmed 节点(评估器只读字段; State/Evidence 直填即可)。
func mustNode(g *core.AttackGraph, id, typ, label string, ev ...string) {
	n := &core.Node{ID: id, Type: typ, Label: label, State: core.StateConfirmed}
	for _, e := range ev {
		n.Evidence = append(n.Evidence, core.Evidence{Tool: "port_scan", Excerpt: e})
	}
	g.UpsertNode(n)
}

// edge —— 构造一条 confirmed 边。
func edge(g *core.AttackGraph, src, rel, dst string) {
	g.Edges = append(g.Edges, &core.Edge{Src: src, Rel: rel, Dst: dst, State: core.StateConfirmed})
}

// juiceGT —— 与 scenarios/juice-shop/ground_truth.json 同构的小样例。
func juiceGT() *GroundTruth {
	return &GroundTruth{
		ScenarioID: "juice-shop",
		Target:     "http://localhost:3000",
		ExpectedFindings: []ExpectedFinding{
			{ID: "tech-fingerprint-express", Title: "Express 指纹", EvidenceKeywords: []string{"express"}},
			{ID: "sqli-login-bypass", Title: "SQLi 登录绕过", EvidenceKeywords: []string{"authentication", "token"}},
		},
		DecoyFindings: []DecoyFinding{
			{ID: "decoy-struts2", Title: "Apache Struts2 RCE"},
			{ID: "decoy-heartbleed", Title: "OpenSSL Heartbleed"},
		},
		AttackChain: &AttackChain{StartType: "service", GoalType: "foothold"},
	}
}

// TestEvaluateBaseline —— 正常战役: 指纹 + SQLi 均命中, 无证据违规, 无攻击链(foothold 缺失)。
func TestEvaluateBaseline(t *testing.T) {
	g := core.NewAttackGraph()
	mustNode(g, "host:localhost", "host", "localhost", "Host is up")
	mustNode(g, "service:localhost:3000", "service", "http on localhost:3000", "3000/tcp open http")
	mustNode(g, "finding:localhost:express", "finding", "X-Powered-By: Express", "X-Powered-By: Express")
	mustNode(g, "finding:localhost:sqli", "finding", "SQLi 登录绕过成功(获得 admin token)", `"authentication":{"token":"abc"}`)
	edge(g, "host:localhost", "runs", "service:localhost:3000")

	snap := &Snapshot{
		Scenario: "juice-shop",
		Target:   "http://localhost:3000",
		Graph:    g,
		Trace:    []string{"Host is up\n3000/tcp open http\nX-Powered-By: Express", `{"authentication":{"token":"abc"}}`},
	}
	res := evaluate(snap, juiceGT())

	if res.Metrics.Confirmed != 4 {
		t.Errorf("confirmed = %d, want 4", res.Metrics.Confirmed)
	}
	if res.Metrics.TruePositive != 2 {
		t.Errorf("tp = %d, want 2", res.Metrics.TruePositive)
	}
	if res.Metrics.FalsePositive != 0 {
		t.Errorf("fp = %d, want 0", res.Metrics.FalsePositive)
	}
	if res.Metrics.Recall != 1.0 {
		t.Errorf("recall = %v, want 1.0", res.Metrics.Recall)
	}
	if res.Metrics.Precision != 1.0 {
		t.Errorf("precision = %v, want 1.0", res.Metrics.Precision)
	}
	if res.Metrics.EvidenceViolations != 0 {
		t.Errorf("violations = %d, want 0", res.Metrics.EvidenceViolations)
	}
	if res.Metrics.HallucinationRate != 0 {
		t.Errorf("hallucination = %v, want 0", res.Metrics.HallucinationRate)
	}
	if res.Metrics.EvidenceCoverage != 1.0 {
		t.Errorf("coverage = %v, want 1.0", res.Metrics.EvidenceCoverage)
	}
	if res.Metrics.AttackChainSuccess {
		t.Error("attack_chain_success = true, want false (无 foothold 节点)")
	}
	if len(res.Details.MatchedExpected) != 2 {
		t.Errorf("matched_expected = %v, want 2 条", res.Details.MatchedExpected)
	}
}

// TestEvaluateHallucination —— 证据违规: confirmed 节点 excerpt 不在 trace -> 计入幻觉率。
func TestEvaluateHallucination(t *testing.T) {
	g := core.NewAttackGraph()
	mustNode(g, "service:h:80", "service", "http on h:80", "80/tcp open http")
	// 声称有证据但 excerpt 在 trace 里根本不存在(幻觉证据)
	mustNode(g, "finding:h:rce", "finding", "RCE 命中", "这个摘要在工具输出里不存在")

	snap := &Snapshot{Graph: g, Trace: []string{"80/tcp open http"}}
	res := evaluate(snap, juiceGT())

	if res.Metrics.EvidenceViolations != 1 {
		t.Errorf("violations = %d, want 1", res.Metrics.EvidenceViolations)
	}
	if res.Metrics.HallucinationRate != 0.5 {
		t.Errorf("hallucination = %v, want 0.5 (1/2 confirmed)", res.Metrics.HallucinationRate)
	}
	if len(res.Details.EvidenceViolationNodes) != 1 {
		t.Errorf("violation nodes = %v, want 1 条", res.Details.EvidenceViolationNodes)
	}
}

// TestEvaluateDecoy —— 误报: finding 未命中 expected 但命中 decoy(无关漏洞) -> decoyHits + fp 计入。
func TestEvaluateDecoy(t *testing.T) {
	g := core.NewAttackGraph()
	mustNode(g, "service:h:80", "service", "http on h:80", "80/tcp open http")
	mustNode(g, "finding:h:struts", "finding", "Apache Struts2 RCE (CVE-2017-5638)", "struts2")

	snap := &Snapshot{Graph: g, Trace: []string{"80/tcp open http", "struts2"}}
	res := evaluate(snap, juiceGT())

	if res.Metrics.DecoyHits != 1 {
		t.Errorf("decoy_hits = %d, want 1", res.Metrics.DecoyHits)
	}
	if res.Metrics.FalsePositive != 1 {
		t.Errorf("fp = %d, want 1", res.Metrics.FalsePositive)
	}
	if res.Metrics.FalsePositiveRate != 1.0 {
		t.Errorf("fp_rate = %v, want 1.0", res.Metrics.FalsePositiveRate)
	}
	if res.Metrics.Recall != 0.0 {
		t.Errorf("recall = %v, want 0.0", res.Metrics.Recall)
	}
	if len(res.Details.DecoyMatched) != 1 {
		t.Errorf("decoy_matched = %v, want 1 条", res.Details.DecoyMatched)
	}
}

// TestFindAttackChain —— 攻击链: confirmed 边 service -produces-> foothold 应 BFS 贯通。
func TestFindAttackChain(t *testing.T) {
	g := core.NewAttackGraph()
	mustNode(g, "host:t", "host", "t", "Host is up")
	mustNode(g, "service:t:80", "service", "http on t:80", "80/tcp open http")
	mustNode(g, "web_shell:t", "web_shell", "webshell on t", "shell written")
	mustNode(g, "foothold:t", "foothold", "foothold on t", "session established")
	edge(g, "host:t", "runs", "service:t:80")
	edge(g, "service:t:80", "produces", "web_shell:t")
	edge(g, "web_shell:t", "produces", "foothold:t")

	snap := &Snapshot{Graph: g, Trace: []string{"Host is up", "80/tcp open http", "shell written", "session established"}}
	res := evaluate(snap, juiceGT())

	if !res.Metrics.AttackChainSuccess {
		t.Fatal("attack_chain_success = false, want true")
	}
	want := []string{"service:t:80", "web_shell:t", "foothold:t"}
	if len(res.Details.AttackChain) != len(want) {
		t.Fatalf("chain = %v, want %v", res.Details.AttackChain, want)
	}
	for i := range want {
		if res.Details.AttackChain[i] != want[i] {
			t.Errorf("chain[%d] = %s, want %s", i, res.Details.AttackChain[i], want[i])
		}
	}
}

// TestSnapshotRoundTrip —— 快照 JSON 序列化/反序列化往返一致(评估器读文件路径的关键路径)。
func TestSnapshotRoundTrip(t *testing.T) {
	g := core.NewAttackGraph()
	mustNode(g, "host:localhost", "host", "localhost", "Host is up")
	mustNode(g, "finding:localhost:sqli", "finding", "SQLi 登录绕过成功(获得 admin token)", `"token":"x"`)
	snap := &Snapshot{Scenario: "juice-shop", Target: "http://localhost:3000", Graph: g, Trace: []string{"Host is up", `"token":"x"`}}

	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var back Snapshot
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Graph == nil || back.Graph.Nodes["host:localhost"] == nil {
		t.Fatal("round-trip 丢失节点")
	}
	if back.Graph.Nodes["host:localhost"].State != core.StateConfirmed {
		t.Errorf("round-trip 丢失状态: %v", back.Graph.Nodes["host:localhost"].State)
	}
	if len(back.Trace) != 2 {
		t.Errorf("round-trip 丢失 trace: %v", back.Trace)
	}
}
