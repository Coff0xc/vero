// Package tools —— 工具层: allowlist + 杀伤力分级(L0-L4) + 输出 parser。
//
// 设计要点(承接 Python agent.py 的工具层, 工程化到 Go):
//   - Registry 显式实例(非全局单例), 可测试、无隐藏状态; 场景包往传入的 Registry 注册。
//   - 每个工具带 Level(杀伤力), 内核据此决定是否走 HITL。
//   - Parse 把工具原始 stdout 结构化成 []Observation, 每条记住逐字来源(供证据回查)。
//
// 本包零内部依赖, 是依赖分层的最底层(tools -> core -> ...)。
package tools

import "sort"

// 杀伤力分级: 数字越大越危险, 内核用它做 HITL 门控与审计。
const (
	LevelRecon    = 0 // 被动侦察 (whois/被动指纹)
	LevelScan     = 1 // 主动扫描 (nmap/httpx)
	LevelCred     = 2 // 凭证操作 (dump/kerberoast)
	LevelExploit  = 3 // 利用/提权 (exploit/psexec)
	LevelDestruct = 4 // 破坏/持久化 (backdoor/wipe)
)

// ToolResult —— 工具执行结果(对应 Python ToolResult)。
type ToolResult struct {
	Success bool   `json:"success"`
	Stdout  string `json:"stdout"`
	Stderr  string `json:"stderr,omitempty"`
	RC      int    `json:"rc"`
}

// Observation —— parser 从 stdout 提取的一条结构化观测。
// Excerpt 是逐字来源片段, 是"证据逐字回查"的锚点: confirmed 节点的证据必须能在工具输出里逐字找到。
// Severity/Technique/Tactic 由 parser 结构化填充(不从 label 解析), 进图时写入 Node。
type Observation struct {
	Kind      string // host/service/cred/finding
	Key       string // 唯一标识, 如 "10.0.0.5:22"
	Label     string
	Excerpt   string // 逐字来源片段
	Severity  string // critical/high/medium/low/info(与 Node.Severity 对齐, 报告直接读它)
	Technique string // MITRE ATT&CK technique ID(如 T1190); 未映射留空
	Tactic    string // MITRE ATT&CK tactic(如 initial-access); 未映射留空
}

// ArgSpec —— 工具参数规格(抄 PentAGI/Dark-Moon 的参数级工具描述):
// 进 prompt 让 LLM 按规格填参(不再瞎猜字段名), 内核执行前校验必填项,
// 校验失败的精确原因经 Reflector 回喂决策器, 下轮自纠。
type ArgSpec struct {
	Name     string // 参数名(args map 的 key)
	Desc     string // 一句话说明(含格式/取值示例)
	Required bool   // 缺失/空串时内核拒绝执行并回喂原因
}

// RunFunc 执行工具; ParseFunc 把 stdout 结构化。
type RunFunc func(args map[string]any) ToolResult
type ParseFunc func(stdout string, args map[string]any) []Observation

// Tool —— 一个可执行工具(对应 Python Tool)。Parse 可为 nil(无结构化产出)。
// Produces —— 工具成功后的默认攻击链产出类型(service/web_shell/cred/foothold/shell):
// 内核在动作未显式标注 produces 时用它建节点+边, 使攻击链不依赖 LLM 自觉填字段。
// Args —— 参数规格(可选): 声明后进 prompt + 执行前校验必填。
type Tool struct {
	Name     string
	Level    int
	Desc     string // 给 LLM 的能力描述(能做什么/边界), 让 LLM 不再盲选工具名
	Run      RunFunc
	Parse    ParseFunc
	Produces string    // 可选: 成功即推进的攻击链产出类型
	Args     []ArgSpec // 可选: 参数规格(进 prompt + 执行前校验)
}

// ValidateArgs —— 按 Args 规格校验动作参数: 返回空串表示通过,
// 否则返回精确原因(缺哪个必填参数/该参数的说明), 供内核回喂决策器自纠。
// 未声明 Args 的工具不校验(向后兼容)。
func ValidateArgs(t *Tool, args map[string]any) string {
	for _, spec := range t.Args {
		if !spec.Required {
			continue
		}
		if ArgStr(args, spec.Name, "") == "" {
			return "参数校验失败: 工具 " + t.Name + " 缺必填参数 " + spec.Name + "(" + spec.Desc + ")"
		}
	}
	return ""
}

// Registry —— 工具注册表(显式实例, 取代 Python 的全局 REGISTRY dict)。
type Registry struct {
	tools map[string]*Tool
}

func NewRegistry() *Registry { return &Registry{tools: map[string]*Tool{}} }

// Register 并入一个工具(同名覆盖, 与场景包注册语义一致)。
func (r *Registry) Register(t *Tool) { r.tools[t.Name] = t }

// Get 按名取工具。
func (r *Registry) Get(name string) (*Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Has 判断是否已注册。
func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// Names 返回已注册工具名(排序, 供喂给 LLM 的 allowlist 稳定可复现)。
func (r *Registry) Names() []string {
	ns := make([]string, 0, len(r.tools))
	for n := range r.tools {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// ToolSpec —— 喂给 LLM 的工具规格(名/级别/描述)。
type ToolSpec struct {
	Name  string
	Level int
	Desc  string
	Args  []ArgSpec
}

// Specs —— 已注册工具的规格列表(排序稳定), 供 LLM 知道每个工具能做什么、边界在哪、参数怎么填。
func (r *Registry) Specs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(r.tools))
	for _, n := range r.Names() {
		t := r.tools[n]
		specs = append(specs, ToolSpec{Name: t.Name, Level: t.Level, Desc: t.Desc, Args: t.Args})
	}
	return specs
}

// ArgStr 从 args 里安全取字符串(缺失/类型不符则返回 def)。
// Action.Args 与工具 args 都是 map[string]any, 统一用它取值。
func ArgStr(args map[string]any, key, def string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

// Clip 安全截断到 n 字符(按 rune, 不切坏多字节); 不足 n 时原样返回, 绝不 panic。
// 修 Python 遗留的裸切片 stdout[:500] 等 —— 输出短于 n 直接越界崩溃。
func Clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
