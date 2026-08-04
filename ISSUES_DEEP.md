# Vero 深度审计问题清单

> 审计时间: 2026-08-04  
> 审计方法: 全代码库逐文件审查 + 实战测试交叉验证  
> 每条问题均附代码位置和证据

---

## 一、工具层问题

### T1. exploit_cve 返回硬编码假数据，parser 标记为 critical

**证据**: [exploit_library.go:186-222](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L186-L222)

```go
output += "✅ 利用成功！获得 shell 访问\n"
return tools.ToolResult{Success: true, Stdout: output}
```

`exploitCVE` 函数不执行任何真实命令。无论目标是否存在漏洞，只要 CVE 在 `exploitMap` 里（3 个 CVE），就返回 `"✅ 利用成功！获得 shell 访问"`。`ParseExploitCVE` 检测到 `"利用成功"` 字样后标记为 `critical`。这是确定性假阳性。

### T2. searchsploit 未安装时返回模拟漏洞数据

**证据**: [exploit_library.go:83-86](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L83-L86)

```go
if _, err := os.Stat("/usr/bin/searchsploit"); err != nil {
    return mockSearchsploitOutput(query, filter)
}
```

Windows 上 `/usr/bin/searchsploit` 永远不存在，函数返回硬编码的 Struts2 和 Log4j 漏洞数据。`ParseSearchsploit` 把这些当作真实发现，标记 `severity: "high"`。

### T3. poc_manager 全部返回模拟输出

**证据**: [exploit_library.go:275-314](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L275-L314)

`pocList`、`pocDownload`、`pocExecute` 三个函数全部返回 `fmt.Sprintf` 拼接的假字符串。`pocExecute` 返回 `"✅ PoC 执行完成"` 但未执行任何操作。`ParsePoCManager` 检测到 `"执行完成"` 后生成 observation。

### T4. ffuf 输出到 `/dev/stdout`，Windows 不存在

**证据**: [ffuf.go:55](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L55)

```go
"-o", "/dev/stdout",
```

Windows 没有 `/dev/stdout` 路径。ffuf 会尝试创建名为 `/dev/stdout` 的文件，导致输出写入失败或写入错误位置。`ParseFFUF` 拿到的 stdout 为空，返回 nil，工具静默失败。

### T5. ffuf 字典路径硬编码 Linux 路径

**证据**: [ffuf.go:31-37](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L31-L37)

```go
candidates := []string{
    "/usr/share/wordlists/dirb/common.txt",
    "/usr/share/seclists/Discovery/Web-Content/common.txt",
    "wordlist.txt",
}
wordlist = candidates[0] // 直接用第一个，不检查存在性
```

代码注释写 `"简化: 直接用第一个候选(实际应检查文件存在)"`。Windows 上 `/usr/share/wordlists/dirb/common.txt` 不存在，ffuf 会因字典文件找不到而报错。`ffufVhostEnum` 也有同样问题（[ffuf.go:141](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L141)）。

### T6. ffuf `-se` 参数不存在

**证据**: [ffuf.go:58](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L58)

```go
"-se", // 静默, 避免 banner 干扰 JSON 解析
```

ffuf 没有 `-se` 参数。正确的是 `-s`（静默）或 `-noninteractive`。ffuf 会报 `flag provided but not defined: -se` 并退出。`ffufVhostEnum` 同样使用 `-se`（[ffuf.go:158](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L158)）。

### T7. ParseSearchsploit 不设 Observation.Key

**证据**: [exploit_library.go:161-165](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L161-L165)

```go
obs = append(obs, tools.Observation{
    Kind:     "exploit",
    Label:    fmt.Sprintf("%s (EDB-%s, %s)", title, edbID, cve),
    Severity: "high",
})
```

`Key` 字段为空。在 `applyObservations`（[loop.go:278](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L278)）中 `nid := o.Kind + ":" + o.Key` 会生成 `exploit:` — 所有 exploit 观察合并为同一个节点，不同漏洞的标题和 CVE 互相覆盖。

### T8. mockSearchsploitOutput 类型断言不安全

**证据**: [exploit_library.go:118-121](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L118-L121)

```go
title := strings.ToLower(r["Title"].(string))
cve := strings.ToLower(r["CVE"].(string))
```

如果 map 值为 nil 或非 string 类型，`.(string)` 会 panic。虽然当前硬编码的数据不会触发，但如果后续有人修改数据源（如从 JSON 反序列化），会直接崩溃。

### T9. nmap XML parser 的 Excerpt 是属性片段而非原始行

**证据**: [nmap.go:158-161](file:///Z:/Coff0xc-Repos/vero/internal/tools/nmap.go#L158-L161)

```go
Excerpt: fmt.Sprintf(`portid="%s" protocol="%s"`, port.Port, port.Protocol),
```

Excerpt 设为 `portid="80" protocol="tcp"`。但 `VerifyEvidence`（[graph.go:211](file:///Z:/Coff0xc-Repos/vero/internal/core/graph.go#L211)）在 trace（纯文本 stdout）里做 `strings.Contains` 查找。nmap 的 XML 输出格式为 `<port portid="80" protocol="tcp">`，属性片段 `portid="80" protocol="tcp"` 能在 XML 里匹配到，但如果 nmap 输出格式变化（如属性顺序不同），匹配会失败。

### T10. Sh 函数超时不 kill 子进程

**证据**: [exec.go:17-22](file:///Z:/Coff0xc-Repos/vero/internal/tools/exec.go#L17-L22)

```go
func Sh(args []string, timeout time.Duration) ToolResult {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, args[0], args[1:]...)
    // ...
}
```

`exec.CommandContext` 超时后取消 context，但 Go 的实现只发送 SIGKILL 给主进程。如果工具启动了子进程（如 nmap 调用 nse 脚本、nuclei 调用外部模板），子进程可能变成孤儿进程继续运行。

### T11. ArgStr 不做 TrimSpace，空白字符串通过校验

**证据**: [tool.go:137-144](file:///Z:/Coff0xc-Repos/vero/internal/tools/tool.go#L137-L144)

```go
func ArgStr(args map[string]any, key, dflt string) string {
    if v, ok := args[key].(string); ok && v != "" {
        return v
    }
    return dflt
}
```

`v != ""` 对纯空白字符串 `"   "` 返回 true。`ValidateArgs` 调用 `ArgStr` 检查必填参数，空白字符串通过校验，工具拿到空白参数执行。argspec_test.go:22 记录了此行为但未修复。

### T12. PortScan 的 normalizeHost 对 IPv6 地址处理错误

**证据**: [scan.go:141-146](file:///Z:/Coff0xc-Repos/vero/internal/tools/scan.go#L141-L146)

```go
func normalizeHost(t string) string {
    // ...
    if i := strings.LastIndex(t, ":"); i > 0 {
        t = t[:i]
    }
}
```

`LastIndex(t, ":")` 对 IPv6 地址如 `[::1]:80` 会在最后一个 `:` 处截断，得到 `[::1]` 而非 `[::1]`。但对 `[::1]`（无端口），`LastIndex` 找到中间的 `:`，截断后变成 `[::`，地址损坏。

### T13. tooltest verifyTool 对无依赖工具返回 "no external dependency" 但标记为可用

**证据**: [verify.go:87-89](file:///Z:/Coff0xc-Repos/vero/internal/tooltest/verify.go#L87-L89)

```go
binary := tools.ToolBinary(tool.Name)
if binary == "" {
    return tools.ToolResult{Success: true, Stdout: "no external dependency"}
}
```

`exploit_cve`、`searchsploit_query`、`poc_manager` 的 `ToolBinary` 返回空字符串（不在 switch 里），所以 verifyTool 报告它们"可用"。但这些工具实际返回的是模拟数据（T1/T2/T3），`-tooltest` 的结果误导用户认为工具真实可用。

### T14. CoreDependencies 中 secretsdump 检测的二进制名与 ToolBinary 不一致

**证据**: 
- [deps.go:67](file:///Z:/Coff0xc-Repos/vero/internal/tools/deps.go#L67): `Binary: "secretsdump.py"`
- [install.go:128](file:///Z:/Coff0xc-Repos/vero/internal/tools/install.go#L128): `ToolBinary("secretsdump")` 返回 `"python3"`

`CoreDependencies` 检查 `secretsdump.py` 是否在 PATH，但 `ToolBinary` 返回 `python3`。tooltest 的 `verifyTool` 用 `ToolBinary` 检查 `python3` 是否在 PATH（是），报告工具可用。但实际运行时 `searchsploitQuery` 调用的是 `secretsdump.py` 而非 `python3`。两条验证路径检查不同的二进制，结果不一致。

### T15. pipPackage 中 nxc 映射到 "nxc" 而非 "netexec"

**证据**: [install.go:82](file:///Z:/Coff0xc-Repos/vero/internal/tools/install.go#L82)

```go
"nxc": "nxc", // 注释说 "NetExec 改名后 PyPI 包名是 nxc"
```

但 `CoreDependencies` 的 InstallHint 写的是 `pipx install git+https://github.com/Pennyw0rth/NetExec`（[deps.go:65](file:///Z:/Coff0xc-Repos/vero/internal/tools/deps.go#L65)）。两条路径给出的安装命令不同。实际 PyPI 上 `nxc` 包是否存在需验证（NetExec 项目推荐用 pipx 从 git 安装）。

---

## 二、核心逻辑层问题

### C1. applyObservations 只为 service 类型建边，finding/endpoint 全部孤立

**证据**: [loop.go:282-293](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L282-L293)

```go
if o.Kind == "service" {
    host := strings.SplitN(o.Key, ":", 2)[0]
    hid := "host:" + host
    g.UpsertNode(&Node{ID: hid, Type: "host", Label: host})
    if !g.HasEdge(hid, "runs", nid) {
        g.Edges = append(g.Edges, &Edge{Src: hid, Rel: "runs", Dst: nid, ...})
    }
}
```

只有 `Kind=="service"` 的观察会建 `host→runs→service` 边。`finding` 类型的观察直接 `UpsertNode + Confirm` 但不建任何边。攻击图里 finding 节点全部孤立，`FindPath` 的 BFS 永远找不到通向 finding 的路径。

### C2. probe 模式产出的 finding 类型不建边

**证据**: [loop.go:282](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L282) + [scenarios.go:76](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/scenarios.go#L76)

`ParseHTTP` 产出 `Kind: "finding"` 的观察（技术栈指纹）。`applyObservations` 只对 `service` 建边，finding 不建边。所以 probe 模式的结果图 EDGES 永远为空，前端攻击图看不到任何连接关系。

### C3. claim 验证依赖 `verifies` 参数，但 LLM schema 没有此字段

**证据**: 
- [loop.go:218](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L218): `if v := tools.ArgStr(action.Args, "verifies", ""); v != "" {`
- [deepseek.go:89-101](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L89-L101): function calling schema 里 `properties` 由 `actSchema(d.reg.Names())` 生成

`actSchema` 只定义了 `rationale`、`plan`（含 tool/args/rationale/claim/produces）字段。`verifies` 不在 schema 里，DeepSeek 的 function calling 永远不会输出它。所有 claim 节点永久停留 `hypothesis` 状态，无法通过验证升级为 `confirmed`。

### C4. 停滞检测的 sig 用 `fmt.Sprint(action.Args)` 不稳定

**证据**: [loop.go:257](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L257)

```go
sig := action.Tool + "|" + fmt.Sprint(action.Args)
```

`action.Args` 是 `map[string]any`。Go 的 `fmt.Sprint` 对 map 的输出顺序不稳定（Go map 遍历无序）。同一组参数在不同运行中可能产生不同的 sig，导致停滞检测失效——本应检测到的重复动作被误判为不同动作。

### C5. UpsertNode 合并证据不去重

**证据**: [graph.go:71](file:///Z:/Coff0xc-Repos/vero/internal/core/graph.go#L71)

```go
cur.Evidence = append(cur.Evidence, n.Evidence...)
```

同一工具对同一节点多次调用（如多轮 agent 对同一端点重复探测），证据条目会无限累积。实战中表现为报告里同一证据块重复 8 次，节点 evidence 数组膨胀。

### C6. VerifyEvidence 用 `strings.Contains` 做全文匹配

**证据**: [graph.go:211](file:///Z:/Coff0xc-Repos/vero/internal/core/graph.go#L211)

```go
if ev.Excerpt != "" && !strings.Contains(blob, ev.Excerpt) {
    bad = append(bad, ...)
}
```

`blob` 是所有工具 stdout 的拼接。如果 Excerpt 被截断到某个恰好出现在无关工具输出里的片段（如时间戳 `85835138.2947`），会产生假阳性匹配通过。反之如果 Excerpt 包含特殊字符被转义，会假阴性匹配失败。

### C7. advancePhase 没有 exploit→done 的显式过渡

**证据**: [loop.go:122-127](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L122-L127) + [loop.go:402-418](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L402-L418)

```go
// 循环结束后
if phase != "done" {
    phase = "done"
    emit(Event{Kind: "phase", Data: map[string]any{"phase": "done"}})
}
```

`advancePhase` 只推进到 `exploit` 就不再变化（`if *phase == "exploit" { return }`）。`done` 状态只在循环退出后设置。如果循环因为预算耗尽或停滞退出，前端会先看到 `exploit` 阶段然后突然跳到 `done`，中间没有过渡事件。

### C8. produces 边只连相邻阶段，跨级跳跃不建边

**证据**: [loop.go:351-385](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L351-L385)

```go
var attackChainStages = []string{"service", "web_shell", "cred", "foothold", "shell"}

func prevStageNode(g *AttackGraph, stageType, target string) string {
    // 找 stageType 在数组中的位置，取前一个
    prev := attackChainStages[i-1]
    // 在图里找 prev 类型的 confirmed 节点
}
```

如果工具的 `Produces` 是 `cred` 但图里还没有 `web_shell` 节点（如直接从 service 产出 cred），`prevStageNode` 找不到前置节点，不建边。攻击链在这里断裂。

---

## 三、报告生成层问题

### R1. 报告严重度从 label 前缀解析而非 Node.Severity

**证据**: [report.go:96](file:///Z:/Coff0xc-Repos/vero/internal/report/report.go#L96)

```go
func sevOf(label string) string {
    // 从 "[critical] xxx" 前缀解析
}
```

但 parser 填的 Severity 在 `Node.Severity` 字段里（结构化）。`generator.go:30` 的 `sevOfNode` 正确读了 `Node.Severity`，但旧版 Markdown 报告的 `sevOf(f.Label)` 从 label 前缀解析。如果 parser 改了 label 格式（如不再加 `[severity]` 前缀），Markdown 报告的严重度会全部丢失。

### R2. buildServices 从节点 ID 提取端口用 `Split(":")` 不可靠

**证据**: [generator.go:98-101](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L98-L101)

```go
parts := strings.Split(n.ID, ":")
// 取 parts[2] 作为端口
```

IPv6 节点 ID 如 `service:host:[::1]:80` 会被冒号切错，`parts[2]` 是 `[` 而非 `80`。

### R3. recommendations 只匹配 3 种漏洞类型

**证据**: [report.go:111-136](file:///Z:/Coff0xc-Repos/vero/internal/report/report.go#L111-L136)

```go
switch {
case strings.Contains(strings.ToLower(f.Label), "sql"):
    // SQLi 修复建议
case strings.Contains(strings.ToLower(f.Label), "swagger"):
    // Swagger 修复建议
case strings.Contains(strings.ToLower(f.Label), "security header"):
    // 安全头修复建议
default:
    continue // 其他全部跳过，不生成修复建议
}
```

暴露端口、版本泄露、目录遍历、敏感端点等所有其他 finding 都不生成修复建议。报告中这些漏洞只有"发现"没有"建议"。

### R4. generateDescription/generateRemediation 只有 3 种漏洞的硬编码描述

**证据**: [generator.go:193-217](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L193-L217)

```go
func generateDescription(finding Finding) string {
    switch {
    case strings.Contains(strings.ToLower(finding.Label), "sql"):
        return "SQL 注入..."
    case strings.Contains(strings.ToLower(finding.Label), "xss"):
        return "跨站脚本..."
    case strings.Contains(strings.ToLower(finding.Label), "rce"):
        return "远程代码执行..."
    default:
        return "检测到潜在安全风险，建议进一步人工验证和修复"
    }
}
```

除 SQLi/XSS/RCE 外的所有 finding 描述都是"检测到潜在安全风险，建议进一步人工验证和修复"——没有实际信息价值。

### R5. calculateCVSS 覆盖 severity 参数

**证据**: [generator.go:163](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L163)

```go
severity = "critical" // 直接覆盖
```

`calculateCVSS` 函数在 CVSS 分数计算后，用 `"critical"` 覆盖 `severity` 变量。返回的 `CVSSScore.Severity` 与 finding 原始 severity 不一致。例如一个 medium 漏洞的 CVSSScore.Severity 可能被标为 critical。

---

## 四、服务器/API/SSE 层问题

### S1. handleStart 不校验 target 格式

**证据**: [server.go:151-154](file:///Z:/Coff0xc-Repos/vero/internal/server/server.go#L151-L154) + [campaign.go:51](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L51)

```go
// server.go
json.Decode(&body) // 不校验 target 是否为空

// campaign.go
if target == "" {
    target = "http://localhost:3000" // 默认替换
}
```

用户提交空 target 会被默认替换为 `http://localhost:3000`。用户不知道在打 localhost，可能对本机服务发起意外攻击。

### S2. SSE Broker 常规事件丢弃无计数

**证据**: [sse.go:49](file:///Z:/Coff0xc-Repos/vero/internal/server/sse.go#L49)

```go
default:
    // 静默丢弃，只有 critical 事件打日志
```

慢消费者（如前端卡顿）会导致大量 tool/graph 事件被丢弃。前端看到不完整的攻击图但不报错，用户无法感知数据丢失。

### S3. handleChat 不校验 history 的 role 值

**证据**: [campaign.go:253-257](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L253-L257)

```go
body.History // 类型为 [][2]string，不校验 role 值
```

恶意输入可注入 `system` role 消息，操纵 LLM 的系统提示词。

### S4. WebGate.Pending 只返回 key 不含工具详情

**证据**: [hitl.go:94-101](file:///Z:/Coff0xc-Repos/vero/internal/server/hitl.go#L94-L101)

```go
type pendingItem struct {
    Key string `json:"key"`
}
```

SSE 重连后前端拿到的 pending 列表只有 `{"key":"hitl-1"}`，没有 tool/args/level 信息。前端无法渲染审批卡片，用户不知道要审批什么。

### S5. SQLite MaxOpenConns(1) 导致查询串行化

**证据**: [store.go:55](file:///Z:/Coff0xc-Repos/vero/internal/store/store.go#L55)

```go
db.SetMaxOpenConns(1)
```

所有 DB 操作串行执行。战役中 `SaveEvent` 与前端 `ListCampaigns` 互相阻塞，前端加载战役列表可能卡顿。长事务（如批量保存事件）会阻塞所有其他查询。

### S6. campaign.go 的 scriptLLM 包含 exploit_sqli 但该工具仅限 web 场景

**证据**: [campaign.go:28](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L28)

```go
var scriptLLM = []string{
    "http_probe", "web_vuln_scan", "extract_endpoints",
    "probe_endpoint", "exploit_sqli",
}
```

脚本模式固定序列包含 `exploit_sqli`。如果目标的 Web 服务不是 juice-shop（`exploit_sqli` 硬编码打 `/rest/user/login`），工具会失败但返回 `Success: false`。脚本模式下某步失败即中断后续步骤（[loop.go:191](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L191)），整个脚本模式空转。

---

## 五、LLM/规划器层问题

### L1. ReflexionEnhanced 完全未接入主循环

**证据**: [reflexion.go](file:///Z:/Coff0xc-Repos/vero/internal/llm/reflexion.go) 全文件

`ReflexionEnhanced` 定义了 SQLite 持久化（lessons 表）、失败分类（`ClassifyFailure`）、自动重试（`ShouldRetry`/`AdjustArgsForRetry`）、Few-shot 注入（`FormatLessonsForPrompt`）。

搜索整个代码库，`NewReflexionEnhanced` 没有任何调用方。`RecordFailure`、`QueryLessons`、`ShouldRetry`、`AdjustArgsForRetry` 全部没有被主循环调用。这套系统是死代码。

### L2. ShouldRetry / AdjustArgsForRetry 未被调用

**证据**: [reflexion.go:216-257](file:///Z:/Coff0xc-Repos/vero/internal/llm/reflexion.go#L216-L257)

主循环 `runAction` 在工具失败后只调用 `OnFailure`（[loop.go:186-188](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L186-L188)），不调用 `ShouldRetry` 判断是否可重试，也不调用 `AdjustArgsForRetry` 调整参数重试。工具失败后直接 `return false`，不重试。

### L3. PlannerLLM 的 hostOf 默认返回 `10.0.0.5`

**证据**: [planner.go:137](file:///Z:/Coff0xc-Repos/vero/internal/planner/planner.go#L137)

```go
func hostOf(g *core.AttackGraph) string {
    for _, id := range g.Order {
        if n := g.Nodes[id]; n.Type == "host" {
            return n.Label
        }
    }
    return "10.0.0.5"
}
```

如果攻击图里还没有 host 节点（初始状态），规划器对 `10.0.0.5` 发起扫描。如果用户目标是其他 IP，规划器在第一轮就会对错误目标发起攻击。

### L4. DeepSeek 重试 3 次但不重置 response body

**证据**: [deepseek.go:118-154](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L118-L154)

```go
var out struct { ... }
for attempt := 0; attempt < 3; attempt++ {
    // ... 构造 request ...
    derr := json.NewDecoder(resp.Body).Decode(&out)
    if derr == nil { ok = true; break }
}
```

`out` 变量在循环外声明。第一次 decode 部分成功后字段有残留值，第二次 decode 如果返回更少字段，残留值不会被清除（Go json.Unmarshal 不清零未映射字段）。可能导致解析结果混乱。

### L5. DeepSeek 的 actSchema 未包含 verifies 字段

**证据**: [deepseek.go:77](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L77) + [inject.go](file:///Z:/Coff0xc-Repos/vero/internal/llm/inject.go)

`actSchema` 生成的 function calling schema 只有 `rationale` 和 `plan`（每个 plan item 有 `tool`/`args`/`rationale`/`claim`/`produces`）。`verifies` 字段不在 schema 里。

主循环在 [loop.go:218](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L218) 依赖 `tools.ArgStr(action.Args, "verifies", "")` 来验证 claim。LLM 永远不会在 args 里填 `verifies`，所以 claim 验证机制完全失效。

### L6. DeepSeek chatText 重试次数比 proposePlan 少

**证据**: [deepseek.go:210](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L210)

```go
for attempt := 0; attempt < 2; attempt++ { // chatText 只重试 2 次
```

`proposePlan` 重试 3 次（[deepseek.go:118](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L118)），`chatText` 只重试 2 次。`chatText` 用于 `Observe`（LLM-as-parser）和 `Reflect`（战役反思），网络抖动时这些功能比决策更容易失败。

---

## 六、场景包层问题

### P1. exploit_cve/searchsploit_query/poc_manager 未注册到场景包

**证据**: [scenarios.go:301-313](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/scenarios.go#L301-L313)

```go
func RegisterDefaults(m *Manager, reg *tools.Registry) {
    m.Register(reg, WebPack())
    m.Register(reg, ADPack())
    // ... 其他包 ...
}
```

`ExploitLibraryPack()` 返回 `[]tools.Tool`（不是 `Pack`），但 `RegisterDefaults` 没有调用它。这 3 个工具（`searchsploit_query`/`exploit_cve`/`poc_manager`）没有被注册到 Registry。`-tooltest` 不会测试它们，agent 模式也不会调用它们。但 `ExploitPack()` 注册了 Metasploit 相关工具。

### P2. exploit_sqli 的端点路径硬编码为 `/rest/user/login`

**证据**: [scenarios.go:153](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/scenarios.go#L153)

```go
"-X", "POST", target + "/rest/user/login", "-H", "Content-Type: application/json",
```

`exploit_sqli` 硬编码打 `/rest/user/login`（juice-shop 的端点）。对其他目标（如 file.nciyuan.net 的 kodbox）这个端点不存在，工具会失败。`Produces: "web_shell"` 意味着如果成功会建 web_shell 节点，但对非 juice-shop 目标永远不会成功。

### P3. ParseProbe 的敏感词匹配过于宽泛

**证据**: [recon_agent.go:219](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/recon_agent.go#L219)

```go
for _, kw := range []string{"error", "exception", "stacktrace", "token", "secret", "api_key", "password", "admin", "debug"} {
    if strings.Contains(strings.ToLower(out), kw) {
        // 生成 [medium] finding
    }
}
```

`error`、`admin`、`debug` 等词在正常页面中极其常见（如 "admin" 出现在导航栏、"error" 出现在 JavaScript 代码中）。这会产生大量假阳性 finding，刷屏攻击图。

### P4. extract_endpoints 的正则不处理相对路径的 base 拼接

**证据**: [recon_agent.go:86-91](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/recon_agent.go#L86-L91)

```go
if !strings.HasPrefix(p, "/") {
    if base != nil {
        p = strings.TrimSuffix(base.Path, "/") + "/" + p
    }
}
```

如果 `base.Path` 是 `/app/index.php`，`TrimSuffix("/app/index.php", "/")` 不会去掉任何东西（因为没有尾部 `/`），拼接结果变成 `/app/index.php/relative-path`，路径错误。应该用 `path.Dir(base.Path)` 提取目录。

---

## 问题统计

| 层 | 高 | 中 | 合计 |
|----|----|----|------|
| 工具层 (T) | 6 | 9 | 15 |
| 核心逻辑层 (C) | 3 | 5 | 8 |
| 报告层 (R) | 1 | 4 | 5 |
| 服务器/API 层 (S) | 0 | 6 | 6 |
| LLM/规划器层 (L) | 3 | 3 | 6 |
| 场景包层 (P) | 1 | 3 | 4 |
| **合计** | **14** | **30** | **44** |
