# Vero 环境配置与运行问题清单

> 生成日期: 2026-08-04
> 项目路径: Z:\Coff0xc-Repos\vero
> 环境: Windows 11 | Go 1.26.5 | Node v24.15.0 | npm 11.12.1

---

## 环境信息

| 项 | 值 |
|---|---|
| 操作系统 | Windows 11 |
| Go 版本 | go1.26.5 windows/amd64 |
| Node.js | v24.15.0 |
| npm | 11.12.1 |
| 二进制产物 | vero.exe (37,399,040 字节) |
| 前端构建 | internal/webui/dist (已 embed) |
| 依赖数 | Go 模块正常 / npm 266 包 |
| Web 服务地址 | http://127.0.0.1:8000 |

---

## 真实问题清单

| # | 问题 | 详情 | 影响 | 解决建议 |
|---|------|------|------|----------|
| 1 | **工具覆盖率极低** | 44 个已注册工具中仅 **1 个可用 (2.3%)**，Windows 环境缺少 nmap / nuclei / ffuf / metasploit / nxc 等关键安全工具 | 严重 | 在 Kali Linux 上运行，或用 WSL2 安装安全工具链 |
| 2 | **无 API Key 时功能受限** | 未设置 `ANTHROPIC_API_KEY` 或 `DEEPSEEK_API_KEY` 时，LLM 决策降级为确定性规划器 | 中 | 申请 API Key 启用真实 AI 决策能力 |
| 3 | **前端需预构建** | 修改前端代码后需重新 `npm run build`，否则 Go embed 的资源不会更新 | 中 | 开发时用 `cd web && npm run dev` 启用热重载调试前端 |
| 4 | **靶场依赖 Docker** | benchmark 靶场（CVE-2021-44228、dvwa、juice-shop）依赖 Docker Compose | 中 | 需安装 Docker Desktop 才能运行基准测试 |

---

## 运行命令速查

```powershell
# 启动 Web UI（主要方式）
.\vero.exe

# 指定端口
.\vero.exe -port 9000

# 离线自检
.\vero.exe -selfcheck

# 工具验证
.\vero.exe -tooltest

# 真实端口扫描（Go 原生，无需 nmap）
.\vero.exe -scan 192.168.1.100

# 真实 HTTP 指纹侦察（被动）
.\vero.exe -probe https://example.com

# 真实 nmap 完整扫描（需安装 nmap）
.\vero.exe -nmap 192.168.1.100

# 真实 Web 侦察（需 nuclei）
.\vero.exe -webscan https://example.com

# LLM 自主侦察（需 API Key + 真实工具）
$env:ANTHROPIC_API_KEY = "sk-ant-..."
.\vero.exe -agent 192.168.1.100
```

---

## 构建命令速查

```powershell
# 后端依赖
go mod download

# 前端依赖 + 构建
cd web
npm install
npm run build
cd ..

# 构建后端二进制（前端已 embed）
go build -o vero.exe ./cmd/vero

# 一键构建（Makefile）
make build

# 运行测试
make test

# 开发模式（前后端分离）
make dev-server   # 后端 :8000
make dev-web       # 前端 :5173
```

---

## 实战测试发现的 Bug（目标: file.nciyuan.net）

> 测试日期: 2026-08-04
> 测试模式: probe / scan / webscan / agent(DeepSeek)
> 测试结果: 4 服务 · 35 finding · 证据回查 0 违规

| # | Bug | 详情 | 严重度 | 代码位置 |
|---|-----|------|--------|----------|
| 1 | **agent 模式攻击图边严重缺失** | 攻击图有 50+ 节点但 EDGES 只有 3 条（host→service），所有 finding / endpoint / claim 节点均为孤立节点，无关联边 | 高 | core/graph.go (AddEdge 逻辑) |
| 2 | **probe 模式 EDGES 为空** | `-probe` 发现 4 个 finding 但 EDGES 完全为空，节点间无任何关联 | 高 | core/graph.go |
| 3 | **claim 节点永久停留 hypothesis** | agent 模式产生大量 `claim` 类型节点状态始终为 hypothesis，从未被 confirmed 或 refuted，形成"悬空假设" | 高 | core/loop.go (反思/确认逻辑) |
| 4 | **nuclei 自动安装失败** | API `install-all` 返回 `open Z:\...\nuclei.exe: A device which does not exist was specified`，BinDir() 目录创建时机有缺陷 | 高 | tools/install.go (InstallBinary → extract) |
| 5 | **install-all 谎报成功** | ffuf 安装返回 `ok:true` + 路径，但 tools/bin 目录为空，文件实际不存在（目录未预先创建导致解压写文件失败） | 高 | tools/install.go (extract → os.Create 失败静默) |
| 6 | **渗透报告证据块重复 8 次** | 报告第 15 项 "HTTP Missing Security Headers" 的证据代码块重复输出 8 次（160-201 行） | 中 | report/generator.go |
| 7 | **报告证据截断错误** | probe_endpoint 的证据显示截断的时间戳片段（如 `85835138.2947`），Excerpt 选取逻辑截取了原始输出的中间部分而非有效片段 | 中 | report/generator.go / tools/parse.go |
| 8 | **nuclei finding 的 Excerpt 无信息价值** | nuclei 扫描结果的证据 excerpt 只显示 URL（`http://file.nciyuan.net`）而非漏洞模板名称/描述/匹配内容 | 中 | tools/parse.go (ParseNuclei) |
| 9 | **nxc pip 包名错误** | 代码用 `nxc` 作为 PyPI 包名，但 PyPI 上实际包名是 `netexec`（`pip install nxc` 必然失败：No matching distribution found） | 中 | tools/install.go:82 (`pipPackage` map) |
| 10 | **CLI tooltest 与 Web API verify 逻辑不一致** | CLI `-tooltest` 直接执行工具（`tool.Run`），Web API `/verify` 只检查 `exec.LookPath`；同一工具两处结果不同（ffuf/curl 已安装但 CLI 报不可用） | 中 | cmd/vero/tooltest.go:41 vs internal/tooltest/verify.go:90 |

---

## 测试发现的安全问题（目标: file.nciyuan.net）

| # | 发现 | 证据 | 严重度 |
|---|------|------|--------|
| S1 | **explorer 端点未授权可访问** | `/index.php?explorer/index` 返回 200 + JSON（含 CSRF_TOKEN error），说明端点存在但需 CSRF token | MEDIUM |
| S2 | **upload 端点未授权可访问** | `/index.php?explorer/upload` 返回 200 + JSON，上传端点存在 | MEDIUM |

### 技术栈指纹
- **反向代理**: Caddy
- **应用框架**: ASP.NET MVC 5.2 (.NET 4.0.30319)
- **应用系统**: kodbox 1.68（可道云网盘）
- **开放端口**: 80 (http) / 443 (https) / 8080 (http-proxy)

---

## 深度代码审计发现的 Bug（架构级）

> 审计范围: core/loop.go · core/graph.go · tools/*.go · report/*.go · server/*.go · llm/*.go · planner/*.go · scenarios/*.go
> 审计方法: 源码逐文件审查 + 实战测试交叉验证

### 核心引擎层（攻击图 / 主循环 / 证据约束）

| # | Bug | 详情 | 严重度 | 代码位置 |
|---|-----|------|--------|----------|
| D1 | **攻击图只建 `runs` 边，finding/endpoint/claim 节点全部孤立** | `applyObservations` 只为 `Kind=="service"` 的观察建 `host→runs→service` 边（[loop.go:282-293](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L282-L293)），finding 类观察直接 UpsertNode+Confirm 但不建任何边。导致攻击图有节点无拓扑，FindPath 永远找不到 finding/foothold 路径 | 高 | core/loop.go:282 |
| D2 | **probe 模式不建任何边** | `probe` 命令直接调用工具并 applyObservations，但 probe_endpoint 工具产出 Kind="finding"，不触发建边逻辑（只 service 才建边），所以 probe 结果图 EDGES 永远为空 | 高 | core/loop.go:282 |
| D3 | **claim 验证只检查 `verifies` 参数，LLM 从不填** | claim 确认逻辑在 [loop.go:218](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L218) 依赖 `tools.ArgStr(action.Args, "verifies", "")`，但 DeepSeek/Claude 的 function calling schema 没有这个参数字段，LLM 永远不会填它。所有 claim 节点永久停留 hypothesis | 高 | core/loop.go:218 + llm/inject.go (actSchema) |
| D4 | **`produces` 边只连前一阶段，跨级跳跃不建边** | `prevStageNode` 按 `attackChainStages` 数组的相邻关系找前置节点（[loop.go:351-385](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L351-L385)），如 produces="cred" 只找 "web_shell" 类型节点。但实战中 cred 可能直接从 service 产出（跳过 web_shell），此时不建边 | 中 | core/loop.go:351 |
| D5 | **停滞检测的 sig 用 `fmt.Sprint(action.Args)` 不稳定** | [loop.go:257](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L257) `sig := action.Tool + "|" + fmt.Sprint(action.Args)`，map 的 `fmt.Sprint` 输出顺序不稳定（Go map 遍历无序），同一动作可能产生不同 sig，导致停滞检测失效 | 中 | core/loop.go:257 |
| D6 | **UpsertNode 合并证据不去重** | [graph.go:71](file:///Z:/Coff0xc-Repos/vero/internal/core/graph.go#L71) `cur.Evidence = append(cur.Evidence, n.Evidence...)` 直接追加，同一工具对同一节点多次调用会累积重复证据条目。实战中表现为报告里同一证据块重复 8 次 | 中 | core/graph.go:71 |
| D7 | **VerifyEvidence 用 `strings.Contains` 做全文匹配** | [graph.go:211](file:///Z:/Coff0xc-Repos/vero/internal/core/graph.go#L211) `!strings.Contains(blob, ev.Excerpt)`，Excerpt 如被截断到时间戳中间片段（如 `85835138.2947`），可能恰好匹配通过或恰好匹配失败。匹配粒度太粗 | 中 | core/graph.go:211 |
| D8 | **Phase 状态机不广播最终 exploit→done** | [loop.go:122-127](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L122-L127) 循环结束后才设 `phase="done"`，但 `advancePhase` 只推进到 `exploit` 就不再变化，中间没有 `exploit→done` 的显式过渡事件 | 低 | core/loop.go:122 |

### 工具层（安装 / 验证 / 执行 / Parser）

| # | Bug | 详情 | 严重度 | 代码位置 |
|---|-----|------|--------|----------|
| D9 | **ffuf 输出到 `/dev/stdout` 在 Windows 不存在** | [ffuf.go:55](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L55) `"-o", "/dev/stdout"`，Windows 没有 `/dev/stdout`，ffuf 会创建名为 `/dev/stdout` 的文件而非写到标准输出 | 高 | scenarios/ffuf.go:55 |
| D10 | **ffuf 字典路径硬编码 Linux 路径** | [ffuf.go:31-37](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L31-L37) 字典候选列表全是 Linux 路径（`/usr/share/wordlists/...`），Windows 上必然找不到，直接用第一个候选不检查存在性 | 高 | scenarios/ffuf.go:31 |
| D11 | **exploit_cve 全部是模拟输出** | [exploit_library.go:186-222](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L186-L222) `exploitCVE` 函数不执行任何真实命令，所有输出都是 `fmt.Sprintf` 拼接的假结果（"✅ 利用成功！获得 shell 访问"），ParseExploitCVE 据此标记为 critical — 这是确定性假阳性 | 高 | scenarios/exploit_library.go:186 |
| D12 | **searchsploit 未安装时返回模拟数据** | [exploit_library.go:83-86](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L83-L86) `os.Stat("/usr/bin/searchsploit")` 失败时调用 `mockSearchsploitOutput` 返回硬编码的 Struts/Log4j 漏洞数据，会被 parser 当作真实发现加入攻击图 | 高 | scenarios/exploit_library.go:83 |
| D13 | **poc_manager 全部是模拟输出** | [exploit_library.go:275-314](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L275-L314) `pocList`/`pocDownload`/`pocExecute` 全部返回 `fmt.Sprintf` 拼接的假字符串，不执行任何真实操作 | 高 | scenarios/exploit_library.go:275 |
| D14 | **ParseNmapXML 的 Excerpt 构造为属性片段** | [nmap.go:158-161](file:///Z:/Coff0xc-Repos/vero/internal/tools/nmap.go#L158-L161) Excerpt 设为 `portid="80" protocol="tcp"` 或 `name="http"`，这是 XML 属性片段而非原始输出行。VerifyEvidence 在 trace（纯文本 stdout）里找不到 XML 属性格式的片段 | 中 | tools/nmap.go:158 |
| D15 | **PortScan 的 normalizeHost 对 IPv6 地址处理错误** | [scan.go:141-146](file:///Z:/Coff0xc-Repos/vero/internal/tools/scan.go#L141-L146) `LastIndex(t, ":")` 剥离端口，但 IPv6 地址如 `[::1]:80` 会错误地在第一个 `:` 处截断 | 中 | tools/scan.go:141 |
| D16 | **Sh 函数不设进程组，无法 kill 超时子进程** | [exec.go:17-22](file:///Z:/Coff0xc-Repos/vero/internal/tools/exec.go#L17-L22) `exec.CommandContext` 超时后只取消 context 但不 kill 子进程，nmap/nuclei 等长时间运行的子进程可能变成孤儿进程 | 中 | tools/exec.go:17 |
| D17 | **ArgStr 不做 TrimSpace** | [tool.go:137-144](file:///Z:/Coff0xc-Repos/vero/internal/tools/tool.go#L137-L144) 纯空白字符串 `"  "` 会被视为有效值通过 ValidateArgs 校验（测试 [argspec_test.go:22](file:///Z:/Coff0xc-Repos/vero/internal/tools/argspec_test.go#L22) 已记录此行为），可能导致工具拿到空白参数执行 | 中 | tools/tool.go:137 |

### 报告生成层

| # | Bug | 详情 | 严重度 | 代码位置 |
|---|-----|------|--------|----------|
| D18 | **报告严重度从 label 解析而非 Node.Severity** | [report.go:96](file:///Z:/Coff0xc-Repos/vero/internal/report/report.go#L96) `sevOf(f.Label)` 从 `[critical] xxx` 前缀解析，但 parser 填的 Severity 在 Node 结构里。新报告 generator.go 用 `sevOfNode` 正确读了，但旧 Markdown 报告仍用 `sevOf(f.Label)` — 两条路径严重度不一致 | 高 | report/report.go:96 vs report/generator.go:30 |
| D19 | **buildServices 从 ID 提取端口用 `Split(":")` 不可靠** | [generator.go:98-101](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L98-L101) `parts := strings.Split(n.ID, ":")` 取 `parts[2]`，但 IPv6 节点 ID 如 `host:[::1]:80` 会被冒号切错 | 中 | report/generator.go:98 |
| D20 | **calculateCVSS 覆盖 severity 参数** | [generator.go:163](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L163) `severity = "critical"` 直接覆盖传入的 severity 参数（Go 参数是值传递不影响调用方），但返回的 CVSSScore.Severity 与 finding 原始 severity 不一致 | 中 | report/generator.go:163 |
| D21 | **recommendations 只硬编码 3 种漏洞类型** | [report.go:111-136](file:///Z:/Coff0xc-Repos/vero/internal/report/report.go#L111-L136) 只匹配 SQLi/Swagger/Security Headers，其他所有 finding（如暴露端口、版本泄露、目录遍历）都落入 `default: continue`，不生成修复建议 | 中 | report/report.go:126 |
| D22 | **generateDescription/generateRemediation 全是硬编码字符串** | [generator.go:193-217](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L193-L217) 只有 3 种漏洞类型的描述和修复建议，其余一律返回"检测到潜在安全风险，建议进一步人工验证和修复" | 中 | report/generator.go:193 |

### 服务器 / API / SSE 层

| # | Bug | 详情 | 严重度 | 代码位置 |
|---|-----|------|--------|----------|
| D23 | **handleStart 不校验 target 格式** | [server.go:151-154](file:///Z:/Coff0xc-Repos/vero/internal/server/server.go#L151-L154) 直接 `json.Decode(&body)` 不校验 target 是否为空或合法，空 target 被默认替换为 `http://localhost:3000`（[campaign.go:51](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L51)），用户可能不知道在打 localhost | 中 | server/server.go:151 |
| D24 | **SSE Broker 常规事件丢弃无计数** | [sse.go:49](file:///Z:/Coff0xc-Repos/vero/internal/server/sse.go#L49) `default:` 分支静默丢弃，只有 critical 事件打日志。慢消费者会导致大量 tool/graph 事件丢失，前端看到不完整的攻击图但不报错 | 中 | server/sse.go:49 |
| D25 | **handleChat 不校验 history 格式** | [campaign.go:253-257](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L253-L257) `body.History` 类型为 `[][2]string`，但不校验 role 值是否为 `user`/`assistant`，恶意输入可注入 system role 消息 | 中 | server/campaign.go:253 |
| D26 | **WebGate.Pending 只返回 key 不含工具详情** | [hitl.go:94-101](file:///Z:/Coff0xc-Repos/vero/internal/server/hitl.go#L94-L101) SSE 重连后前端拿到的 pending 列表只有 `{"key":"hitl-1"}`，没有 tool/args/level 信息，无法渲染审批卡片 | 中 | server/hitl.go:94 |
| D27 | **SQLite MaxOpenConns(1) 导致查询串行化** | [store.go:55](file:///Z:/Coff0xc-Repos/vero/internal/store/store.go#L55) `db.SetMaxOpenConns(1)` 所有 DB 操作串行执行。战役中 SaveEvent 与前端 ListCampaigns 互相阻塞，前端加载战役列表可能卡顿 | 中 | store/store.go:55 |
| D28 | **campaign.go 的 scriptLLM 包含 exploit_sqli 但无注册** | [campaign.go:28](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L28) 脚本模式固定序列包含 `exploit_sqli`，但该工具在 scenarios 包中注册，如果场景包注册顺序变化可能找不到工具，内核 allowlist 拒绝后脚本模式空转 | 中 | server/campaign.go:28 |

### LLM 集成 / 规划器层

| # | Bug | 详情 | 严重度 | 代码位置 |
|---|-----|------|--------|----------|
| D29 | **DeepSeek 重试 3 次但不重置 response body** | [deepseek.go:118-154](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L118-L154) 循环内每次重试都重新构造 request，但 `out` 变量在循环外声明，前次 decode 的残留数据可能影响后续判断（Go json.Decode 不清零未映射字段） | 中 | llm/deepseek.go:118 |
| D30 | **DeepSeek chatText 只重试 2 次且 timeout 90s** | [deepseek.go:210](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L210) chatText 用于 Observe/Reflect，重试次数比 proposePlan 少（2 vs 3），且总超时可能超过单步 budget | 低 | llm/deepseek.go:210 |
| D31 | **PlannerLLM 的 `hostOf` 默认返回 `10.0.0.5`** | [planner.go:137](file:///Z:/Coff0xc-Repos/vero/internal/planner/planner.go#L137) 图里没有 host 节点时硬编码返回 `10.0.0.5`，如果用户目标是其他 IP，规划器会对错误目标发起扫描 | 中 | planner/planner.go:137 |
| D32 | **ReflexionEnhanced 完全未接入主循环** | [reflexion.go](file:///Z:/Coff0xc-Repos/vero/internal/llm/reflexion.go) 整个文件定义了持久化反思学习器（SQLite lessons 表、失败分类、retry 建议等），但没有任何代码调用 `NewReflexionEnhanced` 或 `RecordFailure`。这套系统写了但没接线 | 高 | llm/reflexion.go (全文件) |
| D33 | **ShouldRetry / AdjustArgsForRetry 未被调用** | [reflexion.go:216-257](file:///Z:/Coff0xc-Repos/vero/internal/llm/reflexion.go#L216-L257) 自动重试逻辑写了但未接入主循环 `runAction`，工具失败后直接返回 false 不重试 | 高 | llm/reflexion.go:216 |
| D34 | **targetInjector 的 ProposePlan 回退路径不透传能力** | [inject.go:55-59](file:///Z:/Coff0xc-Repos/vero/internal/llm/inject.go#L55-L59) 内层不支持 Planner 时回退到 `t.Propose`，但返回的 `*Plan` 里的 Action 没有经过 `injectTarget`（只调了 `t.Propose` 内部的 injectTarget，Plan 级的 inject 在 49 行） | 低 | llm/inject.go:55 |

### 场景包层

| # | Bug | 详情 | 严重度 | 代码位置 |
|---|-----|------|--------|----------|
| D35 | **ffufDirBrute 的 `-se` 参数不存在** | [ffuf.go:58](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/ffuf.go#L58) ffuf 没有 `-se` 参数（正确是 `-s` 静默 + 不输出 banner），ffuf 会报错 `flag provided but not defined: -se` | 中 | scenarios/ffuf.go:58 |
| D36 | **ParseSearchsploit 不设 Observation.Key** | [exploit_library.go:161-165](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L161-L165) Observation 的 Key 字段为空，UpsertNode 时 `nid := o.Kind + ":" + o.Key` 会生成 `exploit:` — 所有 exploit 观察合并为一个节点 | 中 | scenarios/exploit_library.go:161 |
| D37 | **mockSearchsploitOutput 的类型断言不安全** | [exploit_library.go:119-121](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L119-L121) `r["Title"].(string)` 无 ok 检查，如果 map 值为 nil 会 panic | 中 | scenarios/exploit_library.go:119 |

---

## 架构级设计缺陷（主题层/理念层）

> 以下不是代码 bug，而是工具设计理念层面的结构性问题，影响整体可用性

| # | 缺陷 | 详情 | 影响 | 涉及范围 |
|---|------|------|------|----------|
| A1 | **"证据驱动"理念与实际实现脱节** | 设计文档声称"无证据不确认"，但实际只有 `service` 类型观察建边，`finding` 类型直接 Confirm 不建边。攻击图有"节点"但无"图"拓扑，FindPath（核心攻击链推理）几乎永远找不到完整路径 | 攻击链可视化无效、报告攻击路径为空 | core/loop.go + core/graph.go |
| A2 | **"LLM 自主决策"依赖 LLM 自觉填隐藏字段** | `verifies`/`produces`/`claim` 参数依赖 LLM 在 function calling 输出中自觉填写，但 schema 里没有这些字段定义。LLM 永远不会填它们，导致 claim 验证和攻击链推进机制完全失效 | claim 永久 hypothesis、攻击链断裂 | llm/inject.go (actSchema) + core/loop.go |
| A3 | **"抗幻觉"只防 LLM 编造，不防工具模拟** | exploit_cve/poc_manager/searchsploit 在工具未安装时返回硬编码假数据（含"✅ 利用成功"），parser 把这些当作真实 critical finding 加入攻击图。证据回查能通过（因为 excerpt 确实在"工具输出"里逐字存在），但这是确定性假阳性 | 假漏洞进入报告、误导演 LLM 后续决策 | scenarios/exploit_library.go |
| A4 | **"跨平台"与"安全工具链"矛盾** | Go 原生工具（PortScan/HTTPProbe）确实跨平台，但场景包大量调用 Linux 专属工具（ffuf 的 `/dev/stdout`、searchsploit 的 `/usr/bin/` 路径、nuclei 的 Linux 路径），Windows 用户跑起来全部失败 | Windows 环境 44 个工具仅 1-3 个可用 | scenarios/*.go + tools/install.go |
| A5 | **"多步规划"中断后无回退机制** | Plan 模式下某步失败即中断后续步骤（[loop.go:191](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L191) `return false`），但已执行步骤的副作用（如已建立的节点/边）不回滚。下一轮 ProposePlan 看到的攻击图状态是"半成品"，可能导致规划器误判 | 规划器在部分失败后可能走错误路径 | core/loop.go:191 |
| A6 | **"HITL"理念与 YOLO 模式冲突无审计区分** | YOLO 模式跳过全部审批（[campaign.go:83](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L83) `approve = core.AutoApprove`），但审计日志不记录 YOLO 状态。事后审查审计日志无法区分"自动批准"和"人工批准" | 审计不可信、合规风险 | server/campaign.go:83 + audit/audit.go |

---

## 综合修复清单 (2026-08-06) — PR #2

> GitHub Issue: [#14 综合问题清单与修复方案](https://github.com/Coff0xc/vero/issues/14)
> GitHub PR: [#2 fix: AI引擎选择/安全加固/前端事件修复](https://github.com/Coff0xc/vero/pull/2)
> 修改文件: 14 个, +450/-45 行

### 🔴 Critical

| # | 问题 | 修复 | 状态 |
|---|------|------|------|
| P1 | **AI 未参与处理**: 旧 DeepSeekKey 未迁移到 providers 系统, 引擎回退脚本模式 | `config.go`: Load() 自动迁移旧 key; `campaign.go`: 分离判断+诊断日志 | ✅ |
| P2 | **API 无认证**: 任意来源可触发战役/审批/访问配置 | `server.go`: authGuard 中间件, Bearer token 认证, 回环免认证 | ✅ |

### 🟠 High

| # | 问题 | 修复 | 状态 |
|---|------|------|------|
| P3 | **引擎回退误显示为"engine 失败"**: tool(success:false) → warning 事件 | `campaign.go`: emit warning; `types.ts`/`store.ts`/`ChatView.tsx`/`index.css`: 前端支持 | ✅ |
| P4 | **MockLLM 接口不完整**: 缺少 Planner/ErrorReporter/Rejecter/Reflector 等 | `mock.go`: 实现全部接口; `mock_test.go`: 单元测试 | ✅ |
| P5 | **DeepSeek 401/403 静默空转**: 密钥错误重试无意义 | `deepseek.go`: 401/403 直接放弃并告警 | ✅ |

### 🟡 Medium

| # | 问题 | 修复 | 状态 |
|---|------|------|------|
| P6 | **Claude 模型名错误**: claude-3-opus 已过时 | `claude.go`: 更新为 claude-sonnet-4-20250514 | ✅ |
| P7 | **VERO_HOST 环境变量缺失**: Docker/K8s 无法对外暴露 | `main.go`: 读取 VERO_HOST; `docker-compose.yml`: 声明 | ✅ |
| P8 | **UTF-8 截断**: code_audit.go 字节截断导致乱码 | `code_audit.go`: tools.Clip 替代字节截断 | ✅ |

### 🟢 Low

| # | 问题 | 修复 | 状态 |
|---|------|------|------|
| P9 | **Docker 配置环境变量未声明** | `docker-compose.yml`: 补全 VERO_HOST/DEEPSEEK_API_KEY/VERO_AUTH_TOKEN | ✅ |
| P10 | **chatText 重试次数不一致** (2 vs 3) | `deepseek.go`: 统一为 3 次, 修复残留字段问题 | ✅ |

### 💭 Enhancement (未修复)

| # | 问题 | 说明 |
|---|------|------|
| P11 | **规则引擎+AI外壳而非真正的AI Agent** | 硬编码攻击序列、字符串匹配证据、无跨会话记忆, 需长期架构改进 |

### 测试验证

```powershell
# 单元测试
go test ./internal/...

# 编译
go build -o vero.exe ./cmd/vero

# 运行
.\vero.exe
```
