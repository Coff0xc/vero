# Bug 修复进度总结

## 已完成修复 (2批，共5个问题)

### 第一批 - C1 + T1 (已推送)
✅ **C1 - 攻击图边严重缺失**
- 为 finding 类型建立 service→exposes→finding 边
- 修复位置: `internal/core/loop.go:277-331`
- 效果: 攻击图拓扑完整，FindPath 可找到路径

✅ **T1 - exploit_cve 返回假成功数据**
- 返回失败而非假的"获得shell访问"
- 修复位置: `internal/scenarios/exploit_library.go:186-210`
- 效果: 消除 critical 假阳性

### 第二批 - T2 + T3 (已推送)
✅ **T2 - searchsploit 返回模拟数据**
- 删除 mockSearchsploitOutput 函数 (30行)
- 未安装时返回失败+提示
- 效果: 无假 exploit 节点

✅ **T3 - poc_manager 返回模拟数据**
- 修改 pocList/pocDownload/pocExecute 返回失败
- 删除假数据生成代码 (29行)
- 效果: 无假 action 节点

⚠️ **L1 - Reflexion 未接入主循环 (跳过)**
- 原因: ReflexionEnhanced 未通过接口暴露到核心循环
- 需要重构 LLM 接口才能完成
- 预计时间: 1-2小时

---

## 修复统计

| 批次 | 问题ID | 优先级 | 状态 | 代码变化 |
|------|--------|--------|------|----------|
| 1 | C1 | P0 | ✅ 完成 | +41行 (边创建逻辑) |
| 1 | T1 | P0 | ✅ 完成 | -5行 (删除假数据) |
| 2 | T2 | P0 | ✅ 完成 | -30行 (删除mock函数) |
| 2 | T3 | P0 | ✅ 完成 | -29行 (删除假输出) |
| 2 | L1 | P0 | ⚠️ 跳过 | 需要接口重构 |

**总计**: 4/5 完成 (80%)

**代码变化**:
- 新增: 41 行 (攻击图边创建)
- 删除: 64 行 (假数据生成)
- 净减少: 23 行

---

## 下一批修复计划 (P1 高优先级)

### 1. C3 - claim 验证缺少 LLM schema 字段
**问题**: actSchema 缺少 `verifies` 字段，LLM 无法声明验证关系

**修复方案**:
```go
// internal/llm/claude.go: actSchema
{
    "name": "verifies",
    "description": "此动作验证的 claim ID (可选, 仅用于验证假设)",
    "type": "string"
}
```

**预计时间**: 45 分钟

---

### 2. C5 - UpsertNode 证据不去重
**问题**: 同一证据多次添加到 Evidence 数组

**修复方案**:
```go
// internal/core/graph.go: UpsertNode
func (g *AttackGraph) UpsertNode(n *Node) {
    existing := g.Nodes[n.ID]
    if existing != nil {
        // 去重: 检查 Tool + Excerpt 指纹
        for _, newEv := range n.Evidence {
            isDup := false
            for _, oldEv := range existing.Evidence {
                if oldEv.Tool == newEv.Tool && oldEv.Excerpt == newEv.Excerpt {
                    isDup = true
                    break
                }
            }
            if !isDup {
                existing.Evidence = append(existing.Evidence, newEv)
            }
        }
    }
}
```

**预计时间**: 20 分钟

---

### 3. T4-T6 - ffuf Windows 路径问题
**问题**: 
- T4: `ffuf -w /usr/share/wordlists/...` 硬编码 Linux 路径
- T5: `-se` 参数在新版本 ffuf 中无效
- T6: Windows 下路径分隔符错误

**修复方案**:
```go
// internal/scenarios/fuzzing.go
func ffufFuzz(args map[string]any) tools.ToolResult {
    wordlist := tools.ArgStr(args, "wordlist", "")
    if wordlist == "" {
        // 跨平台默认字典路径
        if runtime.GOOS == "windows" {
            wordlist = "C:\\wordlists\\common.txt"
        } else {
            wordlist = "/usr/share/wordlists/dirb/common.txt"
        }
    }
    
    // 移除无效的 -se 参数
    cmdArgs := []string{
        "ffuf", "-u", url, "-w", wordlist,
        "-mc", "200,204,301,302,307,401,403",
    }
}
```

**预计时间**: 1 小时

---

### 4. D23 - handleStart 不校验 target
**问题**: 接受任意 target 参数，可能攻击错误目标

**修复方案**:
```go
// internal/server/api.go: handleStart
func handleStart(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Target string `json:"target"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // 校验 target 格式
    if req.Target == "" {
        http.Error(w, "target 参数为空", 400)
        return
    }
    
    // 校验 URL 格式
    if !strings.HasPrefix(req.Target, "http://") && 
       !strings.HasPrefix(req.Target, "https://") {
        http.Error(w, "target 必须以 http:// 或 https:// 开头", 400)
        return
    }
}
```

**预计时间**: 30 分钟

---

### 5. D14 - ParseNmapXML Excerpt 错误
**问题**: Excerpt 使用端口号而非原始输出，证据回查失败

**修复方案**:
```go
// internal/tools/nmap.go: ParseNmapXML
obs = append(obs, Observation{
    Kind:    "service",
    Key:     fmt.Sprintf("%s:%s", host, port),
    Label:   fmt.Sprintf("%s:%s (%s)", host, port, service),
    Excerpt: fmt.Sprintf("PORT %s/tcp open %s", port, service), // 修复: 使用nmap原始格式
})
```

**预计时间**: 30 分钟

---

## P1 批次总时间: 3 小时

完成后将修复：
- 4 个 P0 问题 (已完成)
- 5 个 P1 问题 (下一批)
- **总计 9/10 高优先级问题**

---

## 测试策略

### 单元测试
- C3: 测试 LLM 生成包含 verifies 字段的动作
- C5: 测试 UpsertNode 多次调用不重复证据
- T4-T6: 测试 Windows 环境下 ffuf 路径处理

### 集成测试
- 运行完整战役，检查攻击图边完整性
- 触发工具失败，验证错误消息正确
- 检查报告无假阳性节点

### 回归测试
- 重新运行之前通过的测试用例
- 确保修复未引入新问题

---

## Git 分支状态

```bash
fix/critical-bugs (当前分支)
├── 8c979f5 fix(critical): 修复攻击图边缺失和exploit假阳性问题
└── [新提交] fix(tools): 移除searchsploit和poc_manager假数据输出
```

**下一步**:
1. 继续在 `fix/critical-bugs` 分支修复 P1 问题
2. 所有 P0+P1 完成后创建 PR
3. 合并到 main 分支

---

## 需要用户决策

**选项 A**: 继续修复 P1 问题 (C3+C5+T4-T6+D23+D14, 预计 3 小时)

**选项 B**: 先合并当前修复，单独 PR 处理 P1

**选项 C**: 暂停修复，先测试已完成的 C1+T1+T2+T3

**推荐**: 选项 A (一次性完成所有高优先级修复)
