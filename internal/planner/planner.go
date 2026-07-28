// Package planner —— 图上路径规划 + 跨场景串链 + 动态重规划(对应 Python planner.py)。
//
// 前向 BFS: 从攻击图当前 confirmed 状态出发, 按攻击规则搜到目标 type, 返回下一步 action。
// 规则跨场景(web -> web_shell -> 内网 cred -> AD 横向), 路径穿越场景边界 = 跨场景串链。
// 动态重规划: 某步被拒/失败 -> OnReject 把该规则加 avoid, 下次 BFS 换备选路径; 无路则停(不空转)。
//
// PlannerLLM 实现 core.LLM + core.Rejecter, 是不需 API key 的确定性决策器。
package planner

import (
	"sort"
	"strings"

	"github.com/Coff0xc/vero/internal/core"
)

// Rule —— 一条攻击规则: 前提(needs 类型的 confirmed 节点存在) -> 产出(produces 类型)。
type Rule struct {
	Name     string
	Needs    []string // 前提: 图里需存在这些 type 的 confirmed 节点
	Produces string
	Make     func(g *core.AttackGraph) core.Action
}

// haveTypes —— 图里已 confirmed 的节点 type 集合。
func haveTypes(g *core.AttackGraph) map[string]bool {
	h := map[string]bool{}
	for _, n := range g.Nodes {
		if n.State == core.StateConfirmed {
			h[n.Type] = true
		}
	}
	return h
}

// Plan —— 返回 (action, ruleName); 无路可走返回 (nil, "")。avoid = 要跳过的规则名集合。
func Plan(g *core.AttackGraph, goalType string, rules []Rule, avoid map[string]bool) (*core.Action, string) {
	have := haveTypes(g)
	if have[goalType] {
		return nil, ""
	}
	active := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if !avoid[r.Name] {
			active = append(active, r)
		}
	}

	// BFS: 状态 = 已有 type 集合; 找到第一条通往 goalType 的路径, 返回其首步规则。
	type item struct {
		have  map[string]bool
		first *Rule
	}
	frontier := []item{{have: have, first: nil}}
	seen := map[string]bool{setKey(have): true}
	for len(frontier) > 0 {
		cur := frontier[0]
		frontier = frontier[1:]
		for i := range active {
			r := active[i]
			if subset(r.Needs, cur.have) && !cur.have[r.Produces] {
				first := cur.first
				if first == nil {
					first = &active[i] // 根层: 当前规则即通路首步
				}
				if r.Produces == goalType {
					a := first.Make(g)
					return &a, first.Name // 通路首步
				}
				nxt := cloneSet(cur.have)
				nxt[r.Produces] = true
				if k := setKey(nxt); !seen[k] {
					seen[k] = true
					frontier = append(frontier, item{have: nxt, first: first})
				}
			}
		}
	}

	// 无完整通路: 贪心执行任一可行且能扩展图的规则。
	for i := range active {
		r := active[i]
		if subset(r.Needs, have) && !have[r.Produces] {
			a := r.Make(g)
			return &a, r.Name
		}
	}
	return nil, ""
}

func setKey(m map[string]bool) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ",")
}

func subset(needs []string, have map[string]bool) bool {
	for _, n := range needs {
		if !have[n] {
			return false
		}
	}
	return true
}

func cloneSet(m map[string]bool) map[string]bool {
	c := make(map[string]bool, len(m))
	for k := range m {
		c[k] = true
	}
	return c
}

// hostOf —— 图里第一个 host 的 label(按插入序稳定); 无则回退默认靶标。
func hostOf(g *core.AttackGraph) string {
	for _, id := range g.Order {
		if n := g.Nodes[id]; n.Type == "host" {
			return n.Label
		}
	}
	return "10.0.0.5"
}

// MakeRules —— 根据工具注册表动态生成规则集(优先真实工具, 回退到仿真)。
func MakeRules(hasNmap bool) []Rule {
	// 侦察工具: 优先 nmap_scan(服务版本+漏洞检测), 回退 fake_scan
	scanTool := "fake_scan"
	scanRationale := "侦察: 端口+服务枚举(仿真)"
	if hasNmap {
		scanTool = "nmap_scan"
		scanRationale = "侦察: nmap 完整扫描(服务版本+脚本+漏洞)"
	}

	return []Rule{
		{Name: "recon", Needs: nil, Produces: "service",
			Make: func(g *core.AttackGraph) core.Action {
				target := "10.0.0.5"
				if h := hostOf(g); h != "10.0.0.5" {
					target = h
				}
				return core.Action{Tool: scanTool, Args: map[string]any{"target": target}, Rationale: scanRationale}
			}},
		{Name: "web_exploit", Needs: []string{"service"}, Produces: "web_shell",
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: "web_vuln_scan", Args: map[string]any{"target": hostOf(g)}, Rationale: "web 打点", Produces: "web_shell"}
			}},
		{Name: "loot_creds", Needs: []string{"web_shell"}, Produces: "cred",
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: "fake_dump", Args: map[string]any{"target": hostOf(g)}, Rationale: "从 web shell 捞内网凭证", Produces: "cred"}
			}},
		{Name: "lateral_smb", Needs: []string{"cred"}, Produces: "foothold",
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: "psexec_smb", Args: map[string]any{"target": hostOf(g)}, Rationale: "SMB 横向移动", Produces: "foothold"}
			}},
		{Name: "lateral_wmi", Needs: []string{"cred"}, Produces: "foothold", // 备选路径: SMB 不通时换 WMI
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: "wmiexec", Args: map[string]any{"target": hostOf(g)}, Rationale: "WMI 横向移动(备选)", Produces: "foothold"}
			}},
	}
}

// RULES —— 默认规则集(向后兼容, 使用仿真扫描)。
var RULES = MakeRules(false)

// PlannerLLM —— 确定性规划器当决策器: 图上路径搜索决定每步(不需 API key)。
type PlannerLLM struct {
	GoalType string
	Rules    []Rule
	avoid    map[string]bool
	lastRule string
}

func NewPlanner(goalType string) *PlannerLLM {
	return &PlannerLLM{GoalType: goalType, Rules: RULES, avoid: map[string]bool{}}
}

func (p *PlannerLLM) Propose(_ string, g *core.AttackGraph, _ []core.HistoryItem) *core.Action {
	action, rule := Plan(g, p.GoalType, p.Rules, p.avoid)
	p.lastRule = rule
	return action
}

// OnReject —— 上一步不通(被拒/失败) -> 避开该规则, 下次 BFS 自动换备选路径。
func (p *PlannerLLM) OnReject() {
	if p.lastRule != "" {
		p.avoid[p.lastRule] = true
	}
}
