// Command benchmark-evaluator —— Vero Benchmark 评估器 + 战役 runner。
//
// 定位: benchmark 的可执行核心。由于本程序位于 github.com/Coff0xc/vero 模块内,
// 直接 import internal/core 复用真实算法(FindPath / VerifyEvidence), 与产品内核零算法漂移
// (若外部项目要复用, 同构算法定义见 README.md「指标定义」节)。
//
// 两种模式:
//
//	-mode run      跑一场真实战役(真实工具 + 脚本/LLM 决策) -> 导出攻击图快照 JSON -> 自动评估
//	-mode evaluate 读已有快照 JSON + ground truth -> 计算指标 -> 写结果 JSON
//
// 用法示例:
//
//	go run ./benchmark/evaluator -mode run -scenario juice-shop -target http://localhost:3000 \
//	    -ground-truth benchmark/scenarios/juice-shop/ground_truth.json -out benchmark/results/juice-shop/result.json
//	go run ./benchmark/evaluator -mode evaluate -snapshot x.json -ground-truth y.json -out z.json
//
// 引擎选择(仅 -mode run): -engine auto|script|llm, 或环境变量 VERO_ENGINE。
//
//	auto   有 DEEPSEEK_API_KEY/ANTHROPIC_API_KEY 用真实 LLM 决策, 否则真实工具脚本模式(默认)
//	script 固定脚本(port_scan -> http_probe -> web_vuln_scan -> exploit_sqli), 无需 key
//	llm    强制真实 LLM 决策(需 API key)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/llm"
	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- 数据结构 ----------

// Snapshot —— 一场战役的完整输出: 攻击图 + 工具输出 trace(证据逐字回查的真相源)。
type Snapshot struct {
	Scenario  string            `json:"scenario"`
	Target    string            `json:"target"`
	Engine    string            `json:"engine"`
	Timestamp string            `json:"timestamp"`
	Graph     *core.AttackGraph `json:"graph"`
	Trace     []string          `json:"trace"`
}

// GroundTruth —— 场景标准答案(与 scenarios/*/ground_truth.json 对齐)。
type GroundTruth struct {
	ScenarioID       string            `json:"scenario_id"`
	Target           string            `json:"target"`
	Severity         string            `json:"severity"`
	ExpectedFindings []ExpectedFinding `json:"expected_findings"`
	DecoyFindings    []DecoyFinding    `json:"decoy_findings"`
	AttackChain      *AttackChain      `json:"attack_chain,omitempty"`
}

// ExpectedFinding —— 一条期望发现: 命中条件 = 攻击图 finding 节点的 label 或任一条
// evidence.excerpt 小写包含任一 evidence_keyword。
type ExpectedFinding struct {
	ID               string   `json:"id"`
	Type             string   `json:"type"`
	Title            string   `json:"title"`
	EvidenceKeywords []string `json:"evidence_keywords"`
}

// DecoyFinding —— 一个不应被报告的发现(命中即判定为高危误报)。
type DecoyFinding struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	WhyFalse string `json:"why_false"`
}

// AttackChain —— 攻击链目标: 从 start_type 的 confirmed 节点沿 confirmed 边到 goal_type。
type AttackChain struct {
	StartType string `json:"start_type"`
	GoalType  string `json:"goal_type"`
}

// Metrics —— 核心指标(对齐 internal/eval.KPI + benchmark 信任度指标)。
type Metrics struct {
	Confirmed          int     `json:"confirmed"`
	Hypothesis         int     `json:"hypothesis"`
	EvidenceViolations int     `json:"evidence_violations"`
	HallucinationRate  float64 `json:"hallucination_rate"`  // 证据违规 / confirmed
	EvidenceCoverage   float64 `json:"evidence_coverage"`   // 有逐字证据的 confirmed / confirmed
	TruePositive       int     `json:"true_positive"`       // 命中 expected 的唯一 finding 节点数
	FalsePositive      int     `json:"false_positive"`      // 未命中 expected 的 finding 节点数
	FalsePositiveRate  float64 `json:"false_positive_rate"` // 误报 / finding 节点总数
	Recall             float64 `json:"recall"`              // 命中 expected / expected 总数
	Precision          float64 `json:"precision"`           // 命中 expected / finding 节点总数
	DecoyHits          int     `json:"decoy_hits"`          // 命中 decoy 的节点数(高危误报)
	AttackChainSuccess bool    `json:"attack_chain_success"`
}

// Details —— 明细(供人工复核)。
type Details struct {
	AttackChain            []string `json:"attack_chain,omitempty"`
	MatchedExpected        []string `json:"matched_expected"`
	UnmatchedFindings      []string `json:"unmatched_findings"`
	DecoyMatched           []string `json:"decoy_matched,omitempty"`
	EvidenceViolationNodes []string `json:"evidence_violation_nodes"`
}

// Result —— 一场评估的完整结果。
type Result struct {
	Scenario         string  `json:"scenario"`
	Target           string  `json:"target"`
	Engine           string  `json:"engine"`
	Timestamp        string  `json:"timestamp"`
	TimeTakenSeconds float64 `json:"time_taken_seconds"`
	Metrics          Metrics `json:"metrics"`
	Details          Details `json:"details"`
	Verdict          string  `json:"verdict"`
}

// ---------- 评估核心 ----------

// evaluate —— 快照对照 ground truth 计算全部指标。纯函数, 无 IO, 可单测。
func evaluate(snap *Snapshot, gt *GroundTruth) *Result {
	start := time.Now()
	g := snap.Graph
	res := &Result{
		Scenario:  snap.Scenario,
		Target:    snap.Target,
		Engine:    snap.Engine,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// 1. 证据逐字回查(产品内核真算法): confirmed 节点的每条 excerpt 必须逐字出现在工具输出 trace 里。
	// 幻觉率按“违规节点数 / confirmed 节点数”计(修: 原实现按违规摘录条数, 单节点多条违规时率>1.0 语义失真;
	// 且 viol 条目格式为 "nodeID: 证据...", nodeID 自身含冒号, 无法可靠切分, 故按节点重算违规数)。
	viol := core.VerifyEvidence(g, snap.Trace)
	blob := strings.Join(snap.Trace, "\n")
	violNodeIDs := map[string]bool{}
	for _, id := range g.Order {
		n := g.Nodes[id]
		if n.State != core.StateConfirmed {
			continue
		}
		for _, ev := range n.Evidence {
			if ev.Excerpt != "" && !strings.Contains(blob, ev.Excerpt) {
				violNodeIDs[id] = true
			}
		}
	}
	res.Metrics.EvidenceViolations = len(violNodeIDs)
	res.Details.EvidenceViolationNodes = []string{}
	for _, v := range viol {
		res.Details.EvidenceViolationNodes = append(res.Details.EvidenceViolationNodes, v)
	}

	// 2. 节点统计 + 证据覆盖率。
	confirmed, hypo, withEvidence := 0, 0, 0
	var findings []string // confirmed 的 finding 节点 id(保插入序, 匹配结果稳定可复现)
	for _, id := range g.Order {
		n := g.Nodes[id]
		switch n.State {
		case core.StateConfirmed:
			confirmed++
			hasEv := false
			for _, ev := range n.Evidence {
				if strings.TrimSpace(ev.Excerpt) != "" {
					hasEv = true
					break
				}
			}
			if hasEv {
				withEvidence++
			}
			if n.Type == "finding" {
				findings = append(findings, id)
			}
		case core.StateHypothesis:
			hypo++
		}
	}
	res.Metrics.Confirmed = confirmed
	res.Metrics.Hypothesis = hypo
	if confirmed > 0 {
		res.Metrics.EvidenceCoverage = round3(float64(withEvidence) / float64(confirmed))
		res.Metrics.HallucinationRate = round3(float64(len(violNodeIDs)) / float64(confirmed))
	}

	// 3. finding 匹配: 每个 expected 至多命中一次(按节点插入序); 未命中 expected 的节点查 decoy。
	used := map[string]bool{} // expected id -> 已命中
	tp, fp, decoyHits := 0, 0, 0
	res.Details.MatchedExpected = []string{}
	res.Details.UnmatchedFindings = []string{}
	res.Details.DecoyMatched = []string{}
	for _, id := range findings {
		n := g.Nodes[id]
		if m := matchExpected(n, gt, used); m != "" {
			tp++
			res.Details.MatchedExpected = append(res.Details.MatchedExpected, id+" -> "+m)
			continue
		}
		if d := matchDecoy(n, gt); d != "" {
			decoyHits++
			res.Details.DecoyMatched = append(res.Details.DecoyMatched, id+" -> decoy:"+d)
		}
		fp++
		res.Details.UnmatchedFindings = append(res.Details.UnmatchedFindings, id+" ("+n.Label+")")
	}
	res.Metrics.TruePositive = tp
	res.Metrics.FalsePositive = fp
	res.Metrics.DecoyHits = decoyHits
	totalFindings := len(findings)
	if totalFindings > 0 {
		res.Metrics.FalsePositiveRate = round3(float64(fp) / float64(totalFindings))
		res.Metrics.Precision = round3(float64(tp) / float64(totalFindings))
	}
	if nExp := len(gt.ExpectedFindings); nExp > 0 {
		res.Metrics.Recall = round3(float64(tp) / float64(nExp))
	} else {
		res.Metrics.Recall = 1.0 // 无期望发现 = 无漏报
	}

	// 4. 攻击链: 从首个 confirmed start_type 节点沿 confirmed 边 BFS 到 goal_type(内核真算法)。
	chain, ok := findAttackChain(g, gt)
	res.Metrics.AttackChainSuccess = ok
	res.Details.AttackChain = chain

	res.TimeTakenSeconds = round3(time.Since(start).Seconds())
	res.Verdict = verdict(res.Metrics)
	return res
}

// matchExpected —— 节点是否命中某个未使用的 expected finding; 返回 expected id, 未命中返回 ""。
func matchExpected(n *core.Node, gt *GroundTruth, used map[string]bool) string {
	for i := range gt.ExpectedFindings {
		e := &gt.ExpectedFindings[i]
		if used[e.ID] {
			continue
		}
		if nodeMatchesKeywords(n, e.EvidenceKeywords) {
			used[e.ID] = true
			return e.ID
		}
	}
	return ""
}

// matchDecoy —— 节点是否命中任一 decoy 关键词(未命中 expected 时调用)。
func matchDecoy(n *core.Node, gt *GroundTruth) string {
	for i := range gt.DecoyFindings {
		d := &gt.DecoyFindings[i]
		if nodeMatchesKeywords(n, []string{d.Title}) {
			return d.ID
		}
	}
	return ""
}

// nodeMatchesKeywords —— 节点 label 或任一条 evidence.excerpt 小写包含任一关键词。
func nodeMatchesKeywords(n *core.Node, keywords []string) bool {
	hay := strings.ToLower(n.Label)
	for _, ev := range n.Evidence {
		hay += "\n" + strings.ToLower(ev.Excerpt)
	}
	for _, kw := range keywords {
		if kw != "" && strings.Contains(hay, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// findAttackChain —— 沿 confirmed 边 BFS 从 start_type 到 goal_type; 返回 (节点 id 链, 是否达成)。
func findAttackChain(g *core.AttackGraph, gt *GroundTruth) ([]string, bool) {
	startType, goalType := "service", "foothold"
	if gt.AttackChain != nil {
		if gt.AttackChain.StartType != "" {
			startType = gt.AttackChain.StartType
		}
		if gt.AttackChain.GoalType != "" {
			goalType = gt.AttackChain.GoalType
		}
	}
	for _, id := range g.Order {
		n := g.Nodes[id]
		if n.Type == startType && n.State == core.StateConfirmed {
			if p := g.FindPath(id, goalType); len(p) > 0 {
				return p, true
			}
		}
	}
	return nil, false
}

// verdict —— 简单分级(供人读; 不决定进程退出码, benchmark 是测量不是门禁)。
func verdict(m Metrics) string {
	switch {
	case m.EvidenceViolations > 0:
		return "存疑: 存在证据违规(幻觉), 需人工复核"
	case m.AttackChainSuccess:
		return "达成: 攻击链贯通且证据完整"
	case m.TruePositive > 0:
		return "部分达成: 有确认发现但攻击链未贯通"
	default:
		return "未达成: 无期望发现(可能工具链覆盖不足)"
	}
}

func round3(f float64) float64 {
	return float64(int(f*1000+0.5)) / 1000
}

// ---------- 战役 runner ----------

// runCampaign —— 真实工具战役: 端口扫描 + web 场景包(真实 curl/nuclei/exploit)。
// engine: script(固定脚本) | llm(真实 LLM 决策, 需 key) | auto(有 key 用 LLM, 否则脚本)。
func runCampaign(target, engine string) (*core.AttackGraph, []string, string, error) {
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{Name: "port_scan", Level: tools.LevelScan,
		Desc: "TCP connect 端口扫描, 发现开放端口/服务(target 用 host, 自动去 scheme/端口)", Run: tools.PortScan, Parse: tools.ParseNmap})
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg) // http_probe(curl) / web_vuln_scan(nuclei) / exploit_sqli 真实执行

	var chosen core.LLM
	engineName := engine
	switch engine {
	case "llm":
		chosen = newLLM(reg, target)
	case "script":
		chosen = scriptLLM(target)
	default: // auto
		if hasLLMKey() {
			chosen = newLLM(reg, target)
			engineName = "llm"
		} else {
			chosen = scriptLLM(target)
			engineName = "script"
		}
	}

	goal := "对目标 " + target + " 做红队侦察与漏洞验证: 端口扫描→HTTP指纹→漏扫→发现可利用点尝试利用(如 SQLi)。用真实证据坐实; 充分则停止。"
	g, trace := core.RunAgent(goal, chosen, reg, core.AutoApprove, core.DiscardEmit, 10)
	return g, trace, engineName, nil
}

// scriptLLM —— 真实工具脚本决策(无需 API key; exploit_sqli 为 L3, benchmark 靶场授权下 AutoApprove)。
func scriptLLM(target string) core.LLM {
	return llm.NewMock([]core.Action{
		{Tool: "port_scan", Args: map[string]any{"target": target}, Rationale: "端口扫描"},
		{Tool: "http_probe", Args: map[string]any{"target": target}, Rationale: "HTTP 指纹"},
		{Tool: "web_vuln_scan", Args: map[string]any{"target": target}, Rationale: "nuclei 漏洞扫描"},
		{Tool: "exploit_sqli", Args: map[string]any{"target": target}, Rationale: "SQLi 登录绕过尝试", Produces: "web_shell"}, // Produces: 成功即建 web_shell 节点+service 边, 使攻击链指标真实可达
	})
}

func newLLM(reg *tools.Registry, target string) core.LLM {
	// target 来自 -target flag(修: 原实现取 flag.Arg(0), 但 run 模式 target 只经 flag 传入,
	// positional arg 恒空 -> WithTarget 注入空 target, LLM 战役零产出)。
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		return llm.WithTarget(llm.NewDeepSeek(reg, "", 0.2), target)
	}
	return llm.WithTarget(llm.NewClaude(reg, 0.2), target)
}

func hasLLMKey() bool {
	return os.Getenv("DEEPSEEK_API_KEY") != "" || os.Getenv("ANTHROPIC_API_KEY") != ""
}

// ---------- IO ----------

func loadJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func saveJSON(path string, v any) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// ---------- 入口 ----------

func main() {
	mode := flag.String("mode", "evaluate", "evaluate(读快照算指标) | run(跑战役并评估)")
	target := flag.String("target", "", "战役目标 URL (mode=run)")
	scenario := flag.String("scenario", "", "场景 id, 如 juice-shop (mode=run, 写入快照/结果元数据)")
	snapshotPath := flag.String("snapshot", "", "快照 JSON 输入路径 (mode=evaluate)")
	groundTruthPath := flag.String("ground-truth", "", "ground truth JSON 路径")
	outPath := flag.String("out", "", "结果 JSON 输出路径(缺省打印到 stdout)")
	engine := flag.String("engine", "", "auto|script|llm; 缺省读 VERO_ENGINE, 再缺省 auto")
	flag.Parse()

	if *engine == "" {
		*engine = os.Getenv("VERO_ENGINE")
	}
	if *engine == "" {
		*engine = "auto"
	}

	switch *mode {
	case "run":
		if *target == "" {
			fatal("mode=run 需要 -target")
		}
		g, trace, eng, err := runCampaign(*target, *engine)
		if err != nil {
			fatal("战役失败: " + err.Error())
		}
		snap := &Snapshot{
			Scenario:  *scenario,
			Target:    *target,
			Engine:    eng,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Graph:     g,
			Trace:     trace,
		}
		// 路径约定: 带 ground truth 时 -out 为结果路径, 快照自动写同目录 snapshot.json;
		// 不带 ground truth 时 -out 即快照路径(只导快照不评估)。
		snapPath, resPath := *outPath, *outPath
		if *groundTruthPath != "" {
			if resPath == "" {
				resPath = "result.json"
			}
			snapPath = filepath.Join(filepath.Dir(resPath), "snapshot.json")
		} else {
			if snapPath == "" {
				snapPath = "snapshot.json"
			}
			resPath = ""
		}
		if err := saveJSON(snapPath, snap); err != nil {
			fatal("快照写入失败: " + err.Error())
		}
		fmt.Printf("快照已导出: %s (confirmed=%d trace=%d 条)\n", snapPath, len(g.Nodes), len(trace))
		if resPath == "" {
			return // 只导快照, 不评估
		}
		*outPath = resPath
		*snapshotPath = snapPath // 后续评估段按此路径读回快照(修: 原实现读完空路径直接崩)
	case "evaluate":
		if *snapshotPath == "" || *groundTruthPath == "" {
			fatal("mode=evaluate 需要 -snapshot 与 -ground-truth")
		}
	}
	if *groundTruthPath == "" {
		fatal("需要 -ground-truth")
	}

	var snap Snapshot
	if err := loadJSON(*snapshotPath, &snap); err != nil {
		fatal("快照读取失败: " + err.Error())
	}
	if snap.Graph == nil {
		fatal("快照缺少 graph 字段")
	}
	var gt GroundTruth
	if err := loadJSON(*groundTruthPath, &gt); err != nil {
		fatal("ground truth 读取失败: " + err.Error())
	}
	if snap.Scenario == "" {
		snap.Scenario = gt.ScenarioID
	}
	if snap.Target == "" {
		snap.Target = gt.Target
	}

	res := evaluate(&snap, &gt)
	if *outPath == "" {
		out, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(out))
		return
	}
	if err := saveJSON(*outPath, res); err != nil {
		fatal("结果写入失败: " + err.Error())
	}
	fmt.Printf("评估完成: %s\n  verdict=%s  recall=%.3f  fp_rate=%.3f  幻觉率=%.3f  攻击链=%v\n",
		*outPath, res.Verdict, res.Metrics.Recall, res.Metrics.FalsePositiveRate,
		res.Metrics.HallucinationRate, res.Metrics.AttackChainSuccess)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "错误:", msg)
	os.Exit(1)
}
