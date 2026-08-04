package core

// Action —— LLM 提议的下一步动作(对应 Python Action)。
type Action struct {
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args"`
	Rationale string         `json:"rationale"`
	Claim     string         `json:"claim,omitempty"`    // 声称达成 -> 默认 hypothesis, 独立验证才 confirm
	Produces  string         `json:"produces,omitempty"` // 规划步预期产出 type -> 成功即建该节点(前进一格)
	Verifies  string         `json:"verifies,omitempty"` // 修复 C3: 此动作验证的 claim ID
}

// Event —— 内核向外(CLI/Web)广播的一条事件(对应 Python Event)。
// Kind: step/tool/graph/hitl/hitl_request/route/summary/done。
type Event struct {
	Kind string         `json:"kind"`
	Data map[string]any `json:"data"`
}
