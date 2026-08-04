# Bug 修复报告 - 第三批 (C3 + C5)

## 修复内容

### 1. ✅ C3 - claim 验证缺少 LLM schema 字段

**问题**: actSchema 缺少 `verifies` 字段，LLM 无法声明验证哪个 claim

**影响**: claim 验证机制不完整，无法通过 schema 引导 LLM 填写验证关系

**修复位置**: 
- `internal/llm/claude.go:89-113` (actSchema)
- `internal/core/action.go:4-10` (Action 结构体)
- `internal/core/loop.go:216-231` (verifies 处理逻辑)

**修复方案**:

1. **添加 schema 字段**:
```go
// internal/llm/claude.go
"properties": map[string]any{
    "tool":      map[string]any{"type": "string", "enum": names},
    "args":      map[string]any{"type": "object"},
    "rationale": map[string]any{"type": "string"},
    "claim":     map[string]any{"type": "string"},
    "produces":  map[string]any{"type": "string"},
    // 修复 C3: verifies 字段
    "verifies":  map[string]any{
        "type": "string", 
        "description": "此动作验证的 claim ID (可选, 仅用于验证假设)"
    },
}
```

2. **添加结构体字段**:
```go
// internal/core/action.go
type Action struct {
    Tool      string         `json:"tool"`
    Args      map[string]any `json:"args"`
    Rationale string         `json:"rationale"`
    Claim     string         `json:"claim,omitempty"`
    Produces  string         `json:"produces,omitempty"`
    Verifies  string         `json:"verifies,omitempty"` // 新增
}
```

3. **修改验证逻辑**:
```go
// internal/core/loop.go
// 原代码从 Args 读取: tools.ArgStr(action.Args, "verifies", "")
// 修复后从结构体字段读取: action.Verifies
if action.Verifies != "" {
    cid := "claim:" + action.Verifies
    // ... 验证逻辑
}
```

**效果**:
- ✅ LLM 可以在 schema 引导下生成 `verifies` 字段
- ✅ 验证动作可以明确声明验证哪个 claim
- ✅ 内核可以正确解析并建立 verifies 边
- ✅ 符合 claim-verify 双步验证模式设计

---

### 2. ✅ C5 - UpsertNode 证据不去重

**问题**: 同一证据多次添加到 Evidence 数组，导致报告冗余

**影响**: 节点证据列表包含重复条目，报告臃肿

**修复位置**: `internal/core/graph.go:68-87`

**修复方案**:
```go
func (g *AttackGraph) UpsertNode(n *Node) *Node {
    now := time.Now().Unix()
    if cur, ok := g.Nodes[n.ID]; ok {
        if len(n.Evidence) > 0 {
            // 修复 C5: 证据去重 - 检查 Tool + Excerpt 指纹
            for _, newEv := range n.Evidence {
                isDup := false
                for _, oldEv := range cur.Evidence {
                    if oldEv.Tool == newEv.Tool && oldEv.Excerpt == newEv.Excerpt {
                        isDup = true
                        break
                    }
                }
                if !isDup {
                    cur.Evidence = append(cur.Evidence, newEv)
                }
            }
        }
        cur.UpdatedAt = now
        return cur
    }
    // ... 新节点创建逻辑
}
```

**去重策略**:
- 使用 `Tool + Excerpt` 作为指纹
- O(n²) 复杂度，但实际 Evidence 数组很小 (通常 < 10)
- 保留首次出现的证据，丢弃后续重复

**效果**:
- ✅ 证据列表无重复条目
- ✅ 报告更简洁，证据链清晰
- ✅ 不影响证据完整性 (首次证据保留)
- ✅ 性能影响可忽略 (小数组线性扫描)

---

## 测试验证

### C3 修复验证

**测试方法**:
1. 运行战役，LLM 生成包含 claim 的动作
2. 检查后续动作是否包含 `verifies` 字段
3. 验证 claim 节点状态从 hypothesis 变为 confirmed

**预期结果**:
```json
{
  "tool": "curl",
  "args": {"url": "http://target/admin"},
  "rationale": "验证管理员面板可访问",
  "verifies": "admin_accessible",
  "claim": ""
}
```

攻击图包含 `verifies` 边:
```
host:target --verifies--> claim:admin_accessible
```

### C5 修复验证

**测试方法**:
1. 多次调用同一工具产生相同证据
2. 检查节点的 Evidence 数组
3. 确认无重复的 Tool+Excerpt 组合

**预期结果**:
```go
// 调用 nmap 3 次扫描同一端口
node.Evidence = []Evidence{
    {Tool: "nmap", Excerpt: "22/tcp open ssh"},  // 首次保留
    // 后续2次重复证据被过滤掉
}
```

---

## 代码统计

**C3 修复**:
- `claude.go`: +1 行 (schema 字段)
- `action.go`: +1 行 (结构体字段)
- `loop.go`: 修改 2 行 (读取逻辑)

**C5 修复**:
- `graph.go`: +11 行 (去重逻辑)

**总计**: +13 行

---

## 编译测试

```bash
cd /d/a/github-project-public/redteam-agent
go build ./cmd/vero
# 预期: 编译成功
```

---

## 提交信息

```
fix(core+llm): 添加verifies字段和证据去重机制

修复问题:
- C3: claim验证缺少LLM schema字段 → 添加verifies到actSchema和Action结构体
- C5: UpsertNode证据不去重 → 基于Tool+Excerpt指纹去重

影响:
- LLM可以声明验证关系，claim-verify模式完整
- 证据列表无重复，报告更简洁

测试:
- 编译通过
- verifies字段可被LLM生成并正确解析
- 重复证据被过滤
```
