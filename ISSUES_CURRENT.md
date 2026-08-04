# Vero 问题清单 (当前状态)

> 更新日期: 2026-08-04
> 分支: fix/critical-bugs (基于 main)
> 测试状态: go test ./... 全部通过
> 审计方法: 全代码库逐文件审查 + 实战测试(file.nciyuan.net)交叉验证

---

## 修复摘要

| 批次 | 修复项 | 涉及文件 |
|------|--------|----------|
| 批次 1 (commit 8c979f5) | C1 finding 节点建边 / exploit 假阳性 | loop.go / exploit_library.go |
| 批次 1 (commit 0eb9cf0) | T1/T2/T3 移除 searchsploit/poc_manager 假数据 | exploit_library.go |
| 批次 1 (commit fec7b70) | C3 verifies 字段 / C5 证据去重 | deepseek.go / graph.go |
| 批次 1 (commit 16c7b16) | T4/T5/T6 ffuf Windows 兼容 + 字典路径 + -se 参数 | ffuf.go |
| 批次 2 (本次) | C3 兼容 Args 提取 verifies / C4 稳定 sig / C8 跨级建边 / L1+L2 Retrier 接入 / T7 ParseSearchsploit Key / ParseParallelScan 大小写 / k8s_node_exploit 双注册兼容 | loop.go / llm.go / deepseek.go / exploit_library.go / orchestration.go / k8s_enhanced.go |

---

## 已修复问题 (本批)

| # | 问题 | 修复方式 | 代码位置 |
|---|------|----------|----------|
| C3 | claim 验证依赖 `verifies` 字段但 LLM schema 没有此字段 | 1) DeepSeek 响应解析添加 Verifies 字段; 2) loop.go 兼容从 Args 中提取 verifies | [deepseek.go](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go) / [loop.go](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go) |
| C4 | 停滞检测 sig 用 `fmt.Sprint(map)` 不稳定 | 新增 `stableArgsSig` 函数: 按 key 排序后 JSON 序列化 | [loop.go](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go) |
| C8 | `produces` 跨级跳跃不建边 (如 service 直接产出 cred) | `prevStageNode` 改为向前递归搜索所有前置阶段 confirmed 节点 | [loop.go](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go) |
| L1 | ReflexionEnhanced 完全未接入主循环 (死代码) | 定义 `Retrier` 接口, loop.go 在工具失败时调用 `ShouldRetry` + `AdjustArgsForRetry` | [llm.go](file:///Z:/Coff0xc-Repos/vero/internal/core/llm.go) / [loop.go](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go) |
| L2 | `ShouldRetry` / `AdjustArgsForRetry` 未被调用 | DeepSeekLLM 实现 Retrier 接口, 主循环接入 | [deepseek.go](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go) |
| L5 | DeepSeek actSchema 未包含 verifies 字段 | proposePlan 解析结构体添加 `Verifies string` 字段 | [deepseek.go](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go) |
| T7 | ParseSearchsploit 不设 Observation.Key, 所有 exploit 共享节点 ID | 设置 `Key: edbID` (EDB-ID) 作为唯一标识 | [exploit_library.go](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go) |
| NEW-1 | ParseParallelScan 大小写不匹配 (`Open ports` vs `open`) | `Contains` 和 `Count` 均改为 `strings.ToLower` 后匹配 | [orchestration.go](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/orchestration.go) |
| NEW-2 | `k8s_node_exploit` 在 ContainerPack 和 K8sPackEnhanced 双重注册, 增强版覆盖原版导致 deep_test 失败 | `ParseK8sNodeExploitEnhanced` 兼容原版输出格式 (Can access host / Kubelet API) | [k8s_enhanced.go](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/k8s_enhanced.go) |
| TEST-1 | e2e_test.go 中 `contains` 函数与 windows.go 重复定义导致编译失败 | 删除 e2e_test.go 中的 `contains`, 改用 `strings.Contains` | [e2e_test.go](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/e2e_test.go) |
| TEST-2 | exploit_library_test.go 期望假数据成功, 与修复后行为冲突 | 测试预期改为: 工具未安装/不支持时返回失败 | [exploit_library_test.go](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library_test.go) |

---

## 仍然存在的问题

### 高严重度

| # | 问题 | 详情 | 代码位置 |
|---|------|------|----------|
| T8 | **mockSearchsploitOutput 类型断言不安全** | `r["Title"].(string)` 无 ok 检查, map 值为 nil 时 panic | [exploit_library.go:118-121](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/exploit_library.go#L118-L121) |
| D14 | **nuclei 自动安装失败** | API install-all 返回 `A device which does not exist was specified`, BinDir() 目录创建时机缺陷 | tools/install.go |
| D15 | **install-all 谎报成功** | ffuf 安装返回 ok:true 但文件实际不存在 (目录未预先创建) | tools/install.go |
| S6 | **SQLite MaxOpenConns(1) 导致查询串行化** | 战役 SaveEvent 与前端 ListCampaigns 互相阻塞 | [store.go:55](file:///Z:/Coff0xc-Repos/vero/internal/store/store.go#L55) |

### 中严重度

| # | 问题 | 详情 | 代码位置 |
|---|------|------|----------|
| D6 | **VerifyEvidence 用 `strings.Contains` 做全文匹配** | Excerpt 截断到时间戳片段会产生假阳性/假阴性 | [graph.go:211](file:///Z:/Coff0xc-Repos/vero/internal/core/graph.go#L211) |
| D7 | **ParseNmapXML Excerpt 构造为属性片段** | Excerpt 设为 `portid="80" protocol="tcp"`, 在纯文本 trace 中可能匹配失败 | [nmap.go:158-161](file:///Z:/Coff0xc-Repos/vero/internal/tools/nmap.go#L158-L161) |
| D8 | **PortScan normalizeHost 对 IPv6 处理错误** | `LastIndex(t, ":")` 对 `[::1]:80` 截断位置错误 | [scan.go:141-146](file:///Z:/Coff0xc-Repos/vero/internal/tools/scan.go#L141-L146) |
| D9 | **Sh 函数不设进程组, 超时不 kill 子进程** | nmap/nuclei 长时间运行可能变孤儿进程 | [exec.go:17-22](file:///Z:/Coff0xc-Repos/vero/internal/tools/exec.go#L17-L22) |
| D10 | **ArgStr 不做 TrimSpace** | 纯空白字符串 `"  "` 通过校验 | [tool.go:137-144](file:///Z:/Coff0xc-Repos/vero/internal/tools/tool.go#L137-L144) |
| D11 | **报告严重度从 label 解析而非 Node.Severity** | 旧版 Markdown 报告 `sevOf(f.Label)` 与新版 `sevOfNode` 不一致 | [report.go:96](file:///Z:/Coff0xc-Repos/vero/internal/report/report.go#L96) vs [generator.go:30](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L30) |
| D12 | **buildServices 从 ID 提取端口用 `Split(":")` 不可靠** | IPv6 节点 ID 会被冒号切错 | [generator.go:98-101](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L98-L101) |
| D13 | **calculateCVSS 覆盖 severity 参数** | 返回的 CVSSScore.Severity 与 finding 原始 severity 不一致 | [generator.go:163](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L163) |
| D14 | **recommendations/generateDescription 只硬编码 3 种漏洞** | 其他 finding 不生成修复建议, 描述全是 "检测到潜在安全风险" | [report.go:111-136](file:///Z:/Coff0xc-Repos/vero/internal/report/report.go#L111-L136) / [generator.go:193-217](file:///Z:/Coff0xc-Repos/vero/internal/report/generator.go#L193-L217) |
| D15 | **handleStart 不校验 target 格式** | 空 target 被默认替换为 `http://localhost:3000` | [server.go:151-154](file:///Z:/Coff0xc-Repos/vero/internal/server/server.go#L151-L154) |
| D16 | **SSE Broker 常规事件丢弃无计数** | 慢消费者导致事件丢失, 前端不报错 | [sse.go:49](file:///Z:/Coff0xc-Repos/vero/internal/server/sse.go#L49) |
| D17 | **handleChat 不校验 history 的 role 值** | 恶意输入可注入 system role 消息 | [campaign.go:253-257](file:///Z:/Coff0xc-Repos/vero/internal/server/campaign.go#L253-L257) |
| D18 | **WebGate.Pending 只返回 key 不含工具详情** | 前端无法渲染审批卡片 | [hitl.go:94-101](file:///Z:/Coff0xc-Repos/vero/internal/server/hitl.go#L94-L101) |
| D19 | **CLI tooltest 与 Web API verify 逻辑不一致** | CLI 直接执行 `tool.Run`, Web API 只检查 `exec.LookPath` | cmd/vero/tooltest.go:41 vs internal/tooltest/verify.go:90 |
| D20 | **nxc pip 包名错误** | 代码用 `nxc`, 实际 PyPI 包名是 `netexec` | [install.go:82](file:///Z:/Coff0xc-Repos/vero/internal/tools/install.go#L82) |
| D21 | **DeepSeek 重试 3 次但不重置 response body** | `out` 变量循环外声明, 残留字段影响后续判断 | [deepseek.go:118-154](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L118-L154) |
| D22 | **PlannerLLM 的 hostOf 默认返回 `10.0.0.5`** | 图里没有 host 节点时对错误 IP 发起扫描 | [planner.go:137](file:///Z:/Coff0xc-Repos/vero/internal/planner/planner.go#L137) |
| D23 | **exploit_sqli 端点硬编码为 `/rest/user/login`** | 仅适用 juice-shop, 其他目标必失败 | [scenarios.go:153](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/scenarios.go#L153) |
| D24 | **ParseProbe 敏感词匹配过于宽泛** | `error`/`admin`/`debug` 等词在正常页面中常见, 假阳性刷屏 | [recon_agent.go:219](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/recon_agent.go#L219) |
| D25 | **extract_endpoints 相对路径 base 拼接错误** | 应使用 `path.Dir(base.Path)` 而非 `TrimSuffix` | [recon_agent.go:86-91](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/recon_agent.go#L86-L91) |
| D26 | **ExploitLibraryPack 未注册到 RegisterDefaults** | 3 个工具不会出现在 tooltest 和 agent 调用中 | [scenarios.go:301-313](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/scenarios.go#L301-L313) |
| D27 | **secretsdump 二进制名检测不一致** | deps.go 检测 `secretsdump.py`, ToolBinary 返回 `python3` | [deps.go:67](file:///Z:/Coff0xc-Repos/vero/internal/tools/deps.go#L67) vs [install.go:128](file:///Z:/Coff0xc-Repos/vero/internal/tools/install.go#L128) |
| D28 | **k8s_node_exploit 同名双注册 (根因未除)** | 本批仅做 parser 兼容, 根因是 ContainerPack 和 K8sPackEnhanced 都注册了同名工具 | [container.go:287](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/container.go#L287) + [k8s_enhanced.go:437](file:///Z:/Coff0xc-Repos/vero/internal/scenarios/k8s_enhanced.go#L437) |

### 低严重度

| # | 问题 | 详情 | 代码位置 |
|---|------|------|----------|
| D29 | **advancePhase 没有 exploit→done 显式过渡** | 循环退出后才设 done, 中间无过渡事件 | [loop.go:122-127](file:///Z:/Coff0xc-Repos/vero/internal/core/loop.go#L122-L127) |
| D30 | **DeepSeek chatText 只重试 2 次** | proposePlan 重试 3 次, chatText 少一次 | [deepseek.go:210](file:///Z:/Coff0xc-Repos/vero/internal/llm/deepseek.go#L210) |
| D31 | **targetInjector ProposePlan 回退路径不透传能力** | 回退到 `t.Propose` 时未做 Plan 级 inject | [inject.go:55-59](file:///Z:/Coff0xc-Repos/vero/internal/llm/inject.go#L55-L59) |

---

## 架构级设计缺陷 (主题层)

| # | 缺陷 | 状态 |
|---|------|------|
| A1 | "证据驱动"理念与实际实现脱节 — finding 节点建边已修复, 但 endpoint/claim 节点仍部分孤立 | 部分修复 |
| A2 | "LLM 自主决策"依赖 LLM 自觉填隐藏字段 — verifies 已修复, 但 produces/claim 仍依赖 LLM 自觉 | 部分修复 |
| A3 | "抗幻觉"只防 LLM 编造, 不防工具模拟 — exploit_cve/searchsploit/poc_manager 假数据已移除 | 已修复 |
| A4 | "跨平台"与"安全工具链"矛盾 — ffuf 已修 Windows 兼容, 但 nxc/metasploit 仍需 Linux | 部分修复 |
| A5 | "多步规划"中断后无回退机制 — 某步失败即中断, 副作用不回滚 | 未修复 |
| A6 | "HITL"理念与 YOLO 模式冲突无审计区分 — YOLO 跳过审批但审计日志不记录 | 未修复 |

---

## 测试覆盖

| 测试 | 状态 |
|------|------|
| `go build ./...` | ✅ 通过 |
| `go test ./...` | ✅ 全部通过 (含 deep_test / k8s_enhanced_test / orchestration_test / exploit_library_test) |
| TestContainerToolsDeepDive | ✅ 通过 (k8s_node_exploit 双注册兼容修复) |
| TestParseParallelScan | ✅ 通过 (大小写不敏感修复) |
| TestK8sPackEnhanced | ✅ 通过 |
| TestExploitLibraryPack | ✅ 通过 (测试预期改为失败) |

---

## 问题统计

| 状态 | 高 | 中 | 低 | 合计 |
|------|----|----|----|------|
| 已修复 (本批 + 批次 1) | 8 | 5 | 1 | 14 |
| 仍然存在 | 4 | 25 | 3 | 32 |
| **合计** | **12** | **30** | **4** | **46** |
