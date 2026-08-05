// Package planner —— 图上路径规划 + 跨场景串链 + 动态重规划(对应 Python planner.py)。
//
// 前向 BFS: 从攻击图当前 confirmed 状态出发, 按攻击规则搜到目标 type, 返回下一步 action。
// 规则跨场景(web -> web_shell -> 内网 cred -> AD 横向), 路径穿越场景边界 = 跨场景串链。
// 动态重规划: 某步被拒/失败 -> OnReject 把该规则加 avoid / OnFailure 按工具精确避让,
// 下次 BFS 换备选路径; 无路则停(不空转)。
//
// 真实优先: 规划规则只引用真实工具(nmap_scan/web_vuln_scan/secretsdump 等); 无真实工具时
// 明确停机并暴露原因(实现 core.ErrorReporter), 绝不回退到离线仿真“假装打”。
// 仿真规则集(fake_scan/fake_dump)仅 selfcheck 与测试显式使用(NewSimPlanner)。
//
// PlannerLLM 实现 core.LLM + core.Rejecter + core.Reflector + core.ErrorReporter,
// 是不需 API key 的确定性决策器。
package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Coff0xc/vero/internal/core"
)

// Rule —— 一条攻击规则: 前提(needs 类型的 confirmed 节点存在) -> 产出(produces 类型)。
type Rule struct {
	Name     string
	Needs    []string // 前提: 图里需存在这些 type 的 confirmed 节点
	Produces string
	Tool     string // 规则主用工具名(供 OnFailure 按工具精确避让; 空 = 不参与工具过滤)
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

// Plan —— 返回 (action, ruleName); 无路可走返回 (nil, "")。
// avoid = 要跳过的规则名集合(OnReject 兼容); blocked = 工具名->失败原因(OnFailure 精确反思)。
func Plan(g *core.AttackGraph, goalType string, rules []Rule, avoid map[string]bool, blocked map[string]string) (*core.Action, string) {
	have := haveTypes(g)
	if have[goalType] {
		return nil, ""
	}
	active := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if avoid[r.Name] {
			continue
		}
		if r.Tool != "" && blocked[r.Tool] != "" {
			continue
		}
		active = append(active, r)
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

// hostOf —— 图里第一个 host 的 label(按插入序稳定); 无则返回空串。
// D22 修复: 不再回退硬编码 10.0.0.5 —— 图里没有 host 节点时对错误 IP 发起扫描,
// 改为返回空, 由参数校验(缺 target)明确拒绝该规则, 安全停机而非盲扫。
func hostOf(g *core.AttackGraph) string {
	for _, id := range g.Order {
		if n := g.Nodes[id]; n.Type == "host" {
			return n.Label
		}
	}
	return ""
}

// MakeRules —— 生成规划规则集(只引用真实工具)。
// hasNmap: 本机是否安装 nmap(真实侦察工具)。allowSim: 是否允许离线仿真(selfcheck/演示专用)。
// 规则: !hasNmap && !allowSim 时返回错误 —— 明确停机, 禁止“假装打”(fake_scan 只进 selfcheck)。
func MakeRules(hasNmap, allowSim bool) ([]Rule, error) {
	// 侦察工具: 真实 nmap_scan; 无 nmap 且不允许仿真 -> 直接报不可用。
	scanTool, scanRationale := "nmap_scan", "侦察: nmap 完整扫描(服务版本+脚本+漏洞)"
	if !hasNmap {
		if !allowSim {
			return nil, fmt.Errorf("无真实侦察工具可用: 本机未安装 nmap, 无法执行端口/服务侦察。请安装 nmap 或使用 Web 作战室的工具安装功能")
		}
		scanTool, scanRationale = "fake_scan", "侦察: 端口+服务枚举(仿真, selfcheck 专用)"
	}

	// 凭证提取工具: 真实用 secretsdump(后渗透包); 仿真仅 selfcheck 用 fake_dump。
	lootTool, lootRationale := "secretsdump", "从 web shell 捞内网凭证(secretsdump)"
	if !hasNmap && allowSim {
		lootTool, lootRationale = "fake_dump", "从 web shell 捞内网凭证(仿真, selfcheck 专用)"
	}

	return []Rule{
		{Name: "recon", Needs: nil, Produces: "service", Tool: scanTool,
			Make: func(g *core.AttackGraph) core.Action {
				target := hostOf(g)
				if target == "" {
					// 无 host 节点: 不留空转, 目标缺失由参数校验显式拒绝(安全停机)。
					return core.Action{Tool: scanTool, Args: map[string]any{"target": ""}, Rationale: scanRationale}
				}
				return core.Action{Tool: scanTool, Args: map[string]any{"target": target}, Rationale: scanRationale}
			}},
		{Name: "web_exploit", Needs: []string{"service"}, Produces: "web_shell", Tool: "web_vuln_scan",
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: "web_vuln_scan", Args: map[string]any{"target": hostOf(g)}, Rationale: "web 打点", Produces: "web_shell"}
			}},
		{Name: "loot_creds", Needs: []string{"web_shell"}, Produces: "cred", Tool: lootTool,
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: lootTool, Args: map[string]any{"target": hostOf(g)}, Rationale: lootRationale, Produces: "cred"}
			}},
		{Name: "lateral_smb", Needs: []string{"cred"}, Produces: "foothold", Tool: "nxc_smb_spray", // 真实横向: ADPackEnhanced 注册的 nxc SMB 密码喷洒(修: 原 psexec_smb 未注册, 生产路径必被 allowlist 拒)
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: "nxc_smb_spray", Args: map[string]any{"target": hostOf(g)}, Rationale: "SMB 横向移动(nxc 密码喷洒)", Produces: "foothold"}
			}},
		{Name: "lateral_wmi", Needs: []string{"cred"}, Produces: "foothold", Tool: "nxc_wmi_exec", // 备选路径: SMB 不通时换 WMI(ADPackEnhanced 注册; L3 触发 HITL)
			Make: func(g *core.AttackGraph) core.Action {
				return core.Action{Tool: "nxc_wmi_exec", Args: map[string]any{"target": hostOf(g)}, Rationale: "WMI 横向移动(备选)", Produces: "foothold"}
			}},
	}, nil
}

// simRules —— 仿真规则集(仅 selfcheck/测试显式使用; 生产默认路径禁止)。
func simRules() []Rule {
	rs, err := MakeRules(false, true)
	if err != nil {
		panic("simRules: " + err.Error()) // 仿真分支恒可用, 不可能触发
	}
	return rs
}

// RULES —— 仿真规则集(仅 selfcheck/测试用; 生产请用 NewPlanner 走真实规则)。
var RULES = simRules()

// PlannerLLM —— 确定性规划器当决策器: 图上路径搜索决定每步(不需 API key)。
// 真实优先: NewPlanner 只用真实工具规则, 无可用工具时 Propose 返回 nil 并暴露原因(LastError),
// 内核据此广播 error 事件 —— 明确“停”, 不再回退仿真假装打。
// 实现 core.LLM + core.Rejecter + core.Reflector + core.ErrorReporter。
type PlannerLLM struct {
	GoalType  string
	Rules     []Rule
	avoid     map[string]bool   // 规则名避让(OnReject 兼容)
	blocked   map[string]string // 工具名 -> 失败原因(OnFailure 精确反思)
	lastRule  string
	lastError string
}

// NewPlanner —— 真实模式: 只允许真实工具。本机无 nmap 时规则集为空, Propose 停机并暴露原因。
// hasNmap 探测由调用方给出(默认认为未装; 装了 nmap 的 CLI/Web 路径显式传 true)。
func NewPlanner(goalType string) *PlannerLLM {
	return NewPlannerWith(goalType, false, false)
}

// NewPlannerNmap —— 真实模式, 且明确声明本机已装 nmap(侦察规则用 nmap_scan)。
func NewPlannerNmap(goalType string) *PlannerLLM {
	return NewPlannerWith(goalType, true, false)
}

// NewSimPlanner —— 仿真模式(selfcheck/演示专用): 显式允许 fake_scan/fake_dump。
// 调用方必须同时注册对应的仿真工具(fake_scan/fake_dump), 否则内核仍会拒绝执行。
func NewSimPlanner(goalType string) *PlannerLLM {
	return NewPlannerWith(goalType, false, true)
}

// NewPlannerWith —— 底层构造。
func NewPlannerWith(goalType string, hasNmap, allowSim bool) *PlannerLLM {
	rules, err := MakeRules(hasNmap, allowSim)
	p := &PlannerLLM{GoalType: goalType, avoid: map[string]bool{}, blocked: map[string]string{}}
	if err != nil {
		p.lastError = err.Error()
		return p
	}
	p.Rules = rules
	return p
}

// LastError —— 实现 core.ErrorReporter: 返回停机/无路可走的原因(内核广播 error 事件用)。
func (p *PlannerLLM) LastError() string { return p.lastError }

func (p *PlannerLLM) Propose(_ string, g *core.AttackGraph, _ []core.HistoryItem) *core.Action {
	if len(p.Rules) == 0 { // 真实工具缺失 -> 明确停机, 不假装打
		if p.lastError == "" {
			p.lastError = "无可用规划规则(真实工具缺失)"
		}
		return nil
	}
	action, rule := Plan(g, p.GoalType, p.Rules, p.avoid, p.blocked)
	p.lastRule = rule
	// 只在“目标未达成”时才能报无路可走(修: 原实现 blocked 分支在目标达成守卫之前,
	// 重规划成功收官后下轮 Propose 仍报“已尝试工具均失败”, 内核会广播误导性 error 事件)。
	if action == nil && !haveTypes(g)[p.GoalType] && p.lastError == "" {
		if len(p.blocked) > 0 { // 无路可走且已有工具失败记录 -> 把原因暴露给前端
			p.lastError = "无可用路径: 已尝试工具均失败(" + blockedSummary(p.blocked) + ")"
		} else {
			p.lastError = "无可用路径: 前提工具/条件不满足"
		}
	}
	return action
}

// OnReject —— 上一步不通(被拒/失败) -> 避开该规则, 下次 BFS 自动换备选路径(兼容旧行为)。
func (p *PlannerLLM) OnReject() {
	if p.lastRule != "" {
		p.avoid[p.lastRule] = true
	}
}

// OnFailure —— 实现 core.Reflector: 精确记录“哪个工具 + 什么原因”失败,
// 下次规划避开该工具(比 OnReject 只避规则名更细; 可跨规则生效, 如 psexec_smb 失败不影响 wmiexec)。
func (p *PlannerLLM) OnFailure(action core.Action, reason string) {
	if action.Tool == "" {
		return
	}
	p.blocked[action.Tool] = reason
	// 规则级避让由 OnReject 负责(内核失败路径会同时调 OnReject 与 OnFailure,
	// 这里再写 avoid 是幂等冗余——修: 删除, 避免与 OnReject 隐式耦合)。
}

// blockedSummary —— 失败工具摘要(供 LastError 可读展示)。
func blockedSummary(blocked map[string]string) string {
	names := make([]string, 0, len(blocked))
	for t := range blocked {
		names = append(names, t)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
