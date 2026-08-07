# Vero 问题清单 (当前状态)

> 更新日期: 2026-08-05
> 分支: main (批次 3 修复已全部合并)
> 测试状态: go test ./... 全部通过 + 真实 agent 测试 (file.nciyuan.net)
> 审计方法: 全代码库逐文件审查 + 实战测试交叉验证
>
> **批次 3 (2026-08-05): 代码层问题全部清零** — U1-U5 修复 + 高/中/低全部 29 项修复,
> 每项独立 commit 推送 main。剩余仅架构级 A5/A6 (需设计决策) + 真实环境复测。

---

## 真实测试反馈 (file.nciyuan.net + DeepSeek)

测试模式: tooltest / probe / scan / agent(LLM 决策)
结果: 30+ 节点 · 9 finding · 证据回查 0 违规 · 8 轮工具调用

### ✅ 修复生效部分

| # | 验证项 | 真实结果 |
|---|--------|----------|
| 1 | 工具不再返回假数据 (T1-T3) | `exploit_sqli`/`ffuf_dir_brute` 返回 `success=false`, 不再伪造"✅ 利用成功" |
| 2 | 攻击图节点正确创建 | 30+ 节点: 1 host + 5 service + 9 finding + 15 endpoint + 4 claim |
| 3 | 证据回查 0 违规 | 抗幻觉机制工作正常, 所有证据可溯回 curl/nuclei 真实输出 |
| 4 | LLM 决策正常 | DeepSeek 驱动 8 轮工具调用 (port_scan→http_probe→fetch_page→web_vuln_scan→extract_endpoints→exploit_sqli) |
| 5 | scan 模式边建边正常 | 4 条 `host→runs→service` 边正确建立 |
| 6 | C5 证据去重 | 证据块未重复 (修复前会重复 8 次) |
| 7 | 编译和单元测试 | `go build ./...` + `go test ./...` 全绿 |

### ❌ 修复未生效部分 (真实测试发现)

> **批次 3 状态 (2026-08-05): U1-U5 均已修复, 待 file.nciyuan.net 真实复测确认。**
> 修复内容见下方"批次 3 修复"表。

#### U1. claim 节点全部停留 hypothesis 状态 (C3 修复失效)

**现象**: 攻击图 4 个 claim 全部 hypothesis, 无一被 confirmed:
```
claim:确定 file.nciyuan.net 开放的端口与服务 (finding,hypothesis)
claim:获取 HTTP 服务响应头指纹 (finding,hypothesis)
claim:获取首页内容与攻击面 (finding,hypothesis)
claim:发现 web 漏洞与敏感端点 (finding,hypothesis)
```

**根因**: schema 里有 `verifies` 字段, 但 LLM 根本不填它。LLM 把 claim 当作"计划描述"而非"待验证假设", 没有机制告诉 LLM "这个 action 验证了之前的哪个 claim"。这是设计层面的结构性问题, 不是简单的 schema 字段缺失。

**影响**: 攻击图无法区分"假设"和"已验证事实", FindPath 找不到完整攻击链。

#### U2. finding/endpoint 节点仍然孤立 (C1 修复失效)

**现象**: EDGES 只有 4 条 `host→runs→service` 边, 所有 finding/endpoint 节点无关联边:
```
finding:http://file.nciyuan.net:tech:Via: 0.0 Caddy  ← 孤立
endpoint:http://file.nciyuan.net/index.php?user/view/manifest  ← 孤立
```

**根因**: C1 修复的 finding 建边逻辑期望 Key 格式为 `host:port`, 但实际 Key 是 `http://file.nciyuan.net:tech:Via`。`strings.Split(o.Key, ":")` 切错位置:
- 期望: `["file.nciyuan.net", "80", ...]`
- 实际: `["http", "//file.nciyuan.net", "tech", "Via"]`

同时 probe 模式下没有 service 节点, finding 找不到可关联的 service。

**影响**: 攻击图有节点无拓扑, FindPath 永远找不到通向 finding 的路径。

#### U3. Reflexion 重试未触发 (L1/L2 修复失效)

**现象**: `http_probe` 对 8080 端口连续失败 4 次, 但没看到 "自动重试中(调整参数)..." 日志:
```
· [L1] http_probe → ... 先做 HTTP 指纹探测确认其技术栈
  ⤷ http_probe success=false
· [L1] http_probe → ... 先做 HTTP 指纹探测确认其技术栈与是否独立应用
  ⤷ http_probe success=false
· [L1] http_probe → ... 先做 HTTP HEAD 探测获取响应头
  ⤷ http_probe success=false
· [L1] http_probe → ... 先做 HTTP 指纹探测以判断其应用类型
  ⤷ http_probe success=false
```

**根因**: `curl -sI` 静默模式失败时 stdout/stderr 都为空, `resultReason` 返回 `"工具失败(无输出)"`, `ClassifyFailure` 判断为 `FailureUnknown`, `ShouldRetry` 返回 false。重试逻辑只处理网络超时/参数错误, 无法处理静默失败。

**影响**: 工具失败后不重试, LLM 反复尝试相同动作, 浪费决策轮次。

#### U4. 停滞检测未触发 (C4 修复部分失效)

**现象**: 同一个 `http_probe` 对 8080 端口连续失败 4 次, 循环没有停止。

**根因**: `stableArgsSig` 让相同 args 产生相同 sig, 但 LLM 每次传的 args 可能略有不同 (target 格式 `http://file.nciyuan.net:8080` vs `file.nciyuan.net:8080`), sig 不同。同时停滞检测要求 `sig 相同 AND 节点数不变`, 但前序成功工具增加了节点数, 条件不满足。

**影响**: LLM 在失败动作上空转, 浪费 budget。

#### U5. probe 模式 EDGES 仍然为空

**现象**:
```
EDGES:
  (空)
```

**根因**: probe 只产出 finding 类型, 没有 service 节点。finding 建边逻辑找不到匹配的 service 节点 (U2 的另一个表现)。

**影响**: probe 模式的攻击图完全无拓扑, 前端可视化无意义。

---

## 修复摘要

| 批次 | 修复项 | 涉及文件 |
|------|--------|----------|
| 批次 1 (commit 8c979f5) | C1 finding 节点建边 / exploit 假阳性 | loop.go / exploit_library.go |
| 批次 1 (commit 0eb9cf0) | T1/T2/T3 移除 searchsploit/poc_manager 假数据 | exploit_library.go |
| 批次 1 (commit fec7b70) | C3 verifies 字段 / C5 证据去重 | deepseek.go / graph.go |
| 批次 1 (commit 16c7b16) | T4/T5/T6 ffuf Windows 兼容 + 字典路径 + -se 参数 | ffuf.go |
| 批次 2 (commit 8e74eb2) | C3 兼容 Args 提取 verifies / C4 稳定 sig / C8 跨级建边 / L1+L2 Retrier 接入 / T7 ParseSearchsploit Key / ParseParallelScan 大小写 / k8s_node_exploit 双注册兼容 | loop.go / llm.go / deepseek.go / exploit_library.go / orchestration.go / k8s_enhanced.go |

### 批次 2 修复的真实测试验证

| # | 修复项 | 单元测试 | 真实测试 |
|---|--------|----------|----------|
| C3 | claim 验证兼容 Args 提取 verifies | ✅ | ❌ LLM 不填 verifies 字段 |
| C4 | 停滞检测稳定 sig | ✅ | ⚠️ LLM 每次 args 不同, sig 不同 |
| C8 | produces 跨级建边 | ✅ | ⚠️ 没有产生 produces 的工具调用 |
| L1+L2 | Retrier 接入 | ✅ | ❌ ShouldRetry 无法处理静默失败 |
| T7 | ParseSearchsploit Key | ✅ | ✅ |
| NEW-1 | ParseParallelScan 大小写 | ✅ | ✅ |
| NEW-2 | k8s_node_exploit 双注册兼容 | ✅ | ✅ |

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

## 真实测试新发现的问题 (需后续修复)

| # | 问题 | 详情 | 根因 | 严重度 |
|---|------|------|------|--------|
| U1 | **claim 验证机制完全失效** | 4 个 claim 全部停留 hypothesis, 无一 confirmed | LLM 不填 verifies 字段, 设计层面问题 | 高 |
| U2 | **finding/endpoint 节点孤立** | EDGES 只有 4 条 host→service 边, finding/endpoint 无关联 | Key 格式 `http://host:tech:X` 与建边逻辑 `host:port` 不匹配 | 高 |
| U3 | **Reflexion 重试未触发** | http_probe 连续失败 4 次未触发重试 | curl -sI 静默失败时 stdout/stderr 为空, ClassifyFailure 判断为 Unknown | 高 |
| U4 | **停滞检测未触发** | 同一动作连续失败 4 次未停止 | LLM 每次 args 略有不同 (target 格式变化), sig 不同 | 中 |
| U5 | **probe 模式 EDGES 为空** | probe 产出的 finding 全部孤立 | probe 没有 service 节点, finding 找不到可关联的 service | 高 |

---

## 批次 3 修复 (2026-08-05, 已合并 main)

### U1-U5 真实测试失效项

| # | 修复内容 | 涉及文件 | 验证 |
|---|----------|----------|------|
| U1 | claim 验证: prompt 明确 verifies 用法 + actSchema 描述修正(原误写 claim ID) + Claude 补 Verifies 透传 + 自动关联兜底(动作成功且 hypothesis claim 含目标 host → confirm, 证据=真实输出) | claude.go / deepseek.go / loop.go | 单测✅ |
| U2 | finding 建边: keyHostPort URL 感知解析(替代 Split(":")), service 前缀+端口后缀匹配 | loop.go | 单测✅ |
| U3 | 静默失败(无输出)归类 FailureTargetDown, 触发延迟重试 | reflexion.go | 单测✅ |
| U4 | 停滞检测: stableArgsSig target 归一化 + 连续 3 次失败(不论 args)兜底 | loop.go | 单测✅ |
| U5 | probe 模式: finding 找不到 service 时回退建 host→exposes→finding 边, 确保 host 节点存在 | loop.go | 单测✅ |

> **批次 4 静态复核修正 (2026-08-06)**: 逐文件 + 可执行探针复核发现批次 3 的 U1/U3/U4
> 存在"单测过但根因未除"的缺陷, 已在批次 4 修复(见下)。U2/U5 复核确认真实生效。

### 批次 4 修复 (2026-08-06, 静态复核后补修)

| # | 批次 3 遗留缺陷 | 根因 | 批次 4 修复 | 回归测试 |
|---|----------------|------|-------------|----------|
| U3-Claude | 静默失败重试仅对 DeepSeek 生效, Claude 后端完全失效 | 只有 DeepSeekLLM 实现 core.Retrier, ClaudeLLM 无 ShouldRetry/AdjustArgsForRetry, `llm.(Retrier)` 断言恒 false | ClaudeLLM 补实现 Retrier(委托 reflexion.ShouldRetry/AdjustArgsForRetry) [claude.go] | TestClaudeImplementsRetrier / TestClassifyNoOutputRetryable |
| U4-死代码 | "连续 3 次失败兜底"是死代码, 连续失败仍跑满预算 | 失败路径在 `return false`(loop.go 失败分支)提前返回, `failStreak>=3` 检测位于成功路径之后永不可达; 且成功路径 failStreak 恒为 0 | 兜底前移到失败分支 + 新增 `*stop` 信号让外层循环真正终止(此前停滞检测只中断当前 plan, 外层仍 Propose 空转) [loop.go] | TestU4ConsecFailStall |
| U1-过严 | 自然语言 claim(label 不含 host 字面量)永不触发兜底, 根因(全 hypothesis)在这类 claim 上仍在 | 兜底靠 `strings.Contains(label, host)` 关联 | 创建 claim 时记录 target host(claimHost map), 关联优先用记录值, 回退文本匹配 [loop.go] | TestU1NaturalLanguageClaimConfirmedByLaterAction |
| U1-过松 | 任意成功工具 confirm 所有含 host 的 claim, 语义无关证据绑定(可绕过抗幻觉) | 兜底不排除本动作自己的 claim, 且不区分动作语义 | 排除本动作自身 claim(self-assertion ≠ verification), 仅【之前】动作创建的 claim 可被后续独立动作确认 [loop.go] | TestU1SelfAssertionStaysHypothesis |

### 高严重度 (批次 3.1)

| # | 修复内容 |
|---|----------|
| T8 | 删除 mockSearchsploitOutput 死代码(类型断言 panic) |
| D14 | extract 跳过 zip/tar 目录条目(Windows "device does not exist") |
| D15 | InstallBinary 安装后校验产物非空(不再谎报成功) |
| S6 | SQLite WAL + busy_timeout + MaxOpenConns(4)(查询不再串行化) |

### 中/低严重度 (批次 3.2, D6-D31)

| 域 | 修复项 |
|----|--------|
| 证据链 | D6 按 trace 条目匹配+截断前缀兼容 · D7 Excerpt 用原文行(属性顺序 bug) |
| 网络 | D8 IPv6 归一化 · D9 超时杀进程组(Unix pgid / Win taskkill) |
| 参数 | D10 ArgStr TrimSpace · D15 validateTarget 统一校验 |
| 报告 | D11-D13 严重度统一结构化字段 · D14 共享漏洞知识映射(12 类) |
| 服务端 | D16 SSE 丢弃计数+告警 · D17 history role 过滤 · D18 Pending 带审批详情 |
| CLI/工具 | D19 CLI 改用 VerifyAll · D20 nxc→netexec · D21 重试重置响应体 · D22 hostOf 不再盲扫 · D23 endpoint 可配 · D24 敏感词强信号模式 · D25 path.Dir 解析 · D26 ExploitLibraryPack 注册 · D27 secretsdump.py 检测一致 · D28 双注册根因(ContainerPack 移除) |
| 低 | D29 攻击链贯通即 done · D30 chatText 重试对齐 3 次 · D31 回退注入回归测试(审计确认已修) |

### 新增回归测试

TestKeyHostPort / TestNormHost / TestNormTarget / TestStableArgsSigTargetNormalized / TestValidateTarget / TestSanitizeHistory / TestNormalizeHostIPv6 / TestExtractSkipsDirEntries / TestParseProbeNoFalsePositive / TestInjectorPlanFallbackInjectsTarget / TestInjectorPlanPassthroughInjectsTarget

---

## 仍然存在的问题 (代码层)

> **批次 3 后: 无。全部 32 项(高 4 + 中 25 + 低 3)已修复并通过 go test ./...。**
> 历史条目保留如下供追溯(状态均已标注)。

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
| A1 | "证据驱动"理念与实际实现脱节 — finding 节点建边已修复但 Key 格式不匹配, 真实测试仍孤立 | 批次 3 已修 (U2/U5), 待真实复测 |
| A2 | "LLM 自主决策"依赖 LLM 自觉填隐藏字段 — verifies 字段添加了但 LLM 不填, 真实测试全部 hypothesis | 批次 3 已修 (U1: prompt+兜底), 待真实复测 |
| A3 | "抗幻觉"只防 LLM 编造, 不防工具模拟 — exploit_cve/searchsploit/poc_manager 假数据已移除 | 已修复 |
| A4 | "跨平台"与"安全工具链"矛盾 — ffuf 已修 Windows 兼容, 但 nxc/metasploit 仍需 Linux | 部分修复 |
| A5 | "多步规划"中断后无回退机制 — 某步失败即中断, 副作用不回滚 | 未修复 (需设计决策) |
| A6 | "HITL"理念与 YOLO 模式冲突无审计区分 — YOLO 跳过审批但审计日志不记录 | 未修复 (需设计决策) |

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
| 真实 tooltest (file.nciyuan.net) | ✅ 2/44 工具可用 (4.5%) — 与修复前一致 |
| 真实 probe (file.nciyuan.net) | ⚠️ 4 finding + 0 edges (U5: probe 无 service 节点) |
| 真实 scan (file.nciyuan.net) | ✅ 4 端口 + 4 edges 正常 |
| 真实 agent (file.nciyuan.net) | ⚠️ 30+ 节点 + 9 finding, 但 4 claim 全 hypothesis + finding 孤立 + 重试未触发 |

---

## 问题统计

| 状态 | 高 | 中 | 低 | 合计 |
|------|----|----|----|------|
| 批次 3 前已修复 | 8 | 5 | 1 | 14 |
| 批次 3 修复 (T8/D14/D15/S6 + D6-D31 + U1-U5) | 4 | 25 | 3 | 32 |
| 真实测试失效项 (U1-U5, 已修待复测) | 5 | 0 | 0 | 5 |
| **仍然存在** | **0** | **0** | **0** | **0** |
| 架构级未决 (A5/A6) | — | — | — | 2 |
