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
type Observation struct {
	Kind    string // host/service/cred/finding
	Key     string // 唯一标识, 如 "10.0.0.5:22"
	Label   string
	Excerpt string // 逐字来源片段
}

// RunFunc 执行工具; ParseFunc 把 stdout 结构化。
type RunFunc func(args map[string]any) ToolResult
type ParseFunc func(stdout string, args map[string]any) []Observation

// Tool —— 一个可执行工具(对应 Python Tool)。Parse 可为 nil(无结构化产出)。
type Tool struct {
	Name  string
	Level int
	Desc  string // 给 LLM 的能力描述(能做什么/边界), 让 LLM 不再盲选工具名
	Run   RunFunc
	Parse ParseFunc
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
}

// Specs —— 已注册工具的规格列表(排序稳定), 供 LLM 知道每个工具能做什么、边界在哪。
func (r *Registry) Specs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(r.tools))
	for _, n := range r.Names() {
		t := r.tools[n]
		specs = append(specs, ToolSpec{Name: t.Name, Level: t.Level, Desc: t.Desc})
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
