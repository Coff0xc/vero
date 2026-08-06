# VERO 问题清单

> 最后更新: 2026-04-03
> 分支: `fix/critical-bugs`

---

## 🔴 P0 严重 (已修复)

### P0-1: 攻击图边严重缺失
- **文件**: `internal/core/loop.go:277-331`
- **问题**: finding 类型未建立 service→exposes→finding 边，攻击图拓扑断裂
- **修复**: 添加边生成逻辑
- **状态**: ✅ 已提交 (`8c979f5`)

### P0-2: exploit_cve 返回假成功数据
- **文件**: `internal/scenarios/exploit_library.go:186-210`
- **问题**: 硬编码返回"获得shell访问"假阳性
- **修复**: 返回失败
- **状态**: ✅ 已提交 (`8c979f5`)

### P0-3: searchsploit 返回模拟数据
- **文件**: `internal/scenarios/exploit_library.go:68-91`
- **问题**: mockSearchsploitOutput 返回固定假数据
- **修复**: 删除 mock 函数
- **状态**: ✅ 已提交 (`0eb9cf0`)

### P0-4: poc_manager 返回模拟数据
- **文件**: `internal/scenarios/exploit_library.go:263-301`
- **问题**: pocList/pocDownload/pocExecute 返回假数据
- **修复**: 返回失败
- **状态**: ✅ 已提交 (`0eb9cf0`)

---

## 🟠 P1 高优先级 (已修复)

### P1-1: claim 验证缺少 LLM schema 字段
- **文件**: `internal/llm/claude.go`, `internal/core/action.go`, `internal/core/loop.go`
- **问题**: verifies 字段未定义，claim-verify 模式不完整
- **修复**: 添加 verifies 到 actSchema 和 Action 结构体
- **状态**: ✅ 已提交

### P1-2: UpsertNode 证据不去重
- **文件**: `internal/core/graph.go:68-87`
- **问题**: 相同 Tool+Excerpt 的证据重复添加
- **修复**: 基于 Tool+Excerpt 指纹去重
- **状态**: ✅ 已提交

### P1-3: 前端白屏 — 数组字段 null 导致 TypeError
- **文件**: `web/src/store.ts` (applyEvent)
- **问题**: 后端 `route`/`path` 事件的 `services`/`activated`/`nodes` 字段为 `null`，前端直接调用 `.join()` 或迭代时崩溃
- **影响**: React ErrorBoundary 拦截错误，整个 UI 冻结无事件流出，表现为「新建战役后画面卡住」
- **修复**: 所有数组字段访问加 `?? []` 兜底
  ```ts
  // Before (broken):
  patch.kpi = { ...s.kpi, services: e.data.services }
  // After (fixed):
  patch.kpi = { ...s.kpi, services: e.data.services ?? [] }
  ```
- **关联**: `web/src/lib/sse.ts` — ingest 调用加 try/catch 防御，es.onerror 日志
- **状态**: ✅ 已修复 (提交 e7c6032)

### P1-4: 停止按钮无效 — 战役无法中止
- **文件**: `internal/server/server.go` (handleCancel), `web/src/lib/actions.ts`, `web/src/store.ts`
- **问题**: 后端 handleCancel 仅取消 context，未广播任何事件；前端 cancelCampaign 仅发请求不更新 UI，导致停止后画面仍卡死在"运行中"
- **修复**:
  - 后端: handleCancel 调用 cancel 后立即广播 `campaign_stopped` 事件
  - 前端: cancelCampaign 先乐观更新 store 状态 (status→idle, 清空 hitl) 再请求后端
  - 前端: types.ts/store.ts 新增 `campaign_stopped` 事件处理
- **状态**: ✅ 已修复 (提交 7c2c53e)

### P1-5: 战役结束后 UI 不自动恢复 — 必须手动点停止
- **文件**: `web/src/store.ts`, `web/src/components/ChatView.tsx`
- **问题**: 战役结束 (done 事件) 后 status 设为 'done' 而非 'idle'，UI 仍显示"停止"按钮；输入框残留目标文本；点击"新建"也设 status 为 'running'，导致每次都要手动点停止
- **修复**:
  - store.ts: 'done' 事件处理改为 status='idle', stage='idle', 清空 hitl
  - store.ts: reset('') 空目标时 status='idle' (新建会话)，有目标时 status='running' (启动战役)
  - ChatView.tsx: 输入框初始为空字符串 (不再预填 localhost:3000)
  - ChatView.tsx: start 成功后清空输入框
  - ChatView.tsx: useEffect 监听 status 从 'running'→'idle' 时自动清空输入框
- **状态**: ✅ 已修复 (待提交)

---

## 🟡 P2 中优先级 (待修复)

### P2-1: ffuf Windows 路径问题
- **文件**: `internal/scenarios/ffuf.go`
- **问题**: 硬编码 Linux 路径 `/usr/share/wordlists/...`，`-se` 参数无效
- **预计**: 1 小时

### P2-2: handleStart 不校验 target
- **文件**: `internal/server/server.go`
- **问题**: 接受任意 target 参数，无校验
- **预计**: 30 分钟

### P2-3: ParseNmapXML Excerpt 错误
- **文件**: `internal/scenarios/port_scan.go`
- **问题**: Excerpt 使用端口号而非原始输出
- **预计**: 30 分钟

### P2-4: AI 扫描纯程序化而非 Agent 驱动
- **文件**: `internal/scenarios/` 下多个扫描器
- **问题**: 扫描器硬编码执行命令，无 LLM 动态决策
- **建议**: 将扫描器重构为 LLM 可调用的工具

---

## 🔵 P3 低优先级 (可延后)

### P3-1: Reflexion 未接入主循环
- **文件**: `internal/agent/`
- **问题**: ReflexionEnhanced 未通过接口暴露
- **建议**: 定义 ReflexionProvider 接口

### P3-2: UI 美化
- **文件**: `web/src/`
- **问题**: 界面粗糙，缺少动效和交互反馈
- **建议**: 分批次优化

---

## 验证结果

### SSE 事件流测试 (2026-04-03)
使用 Go 客户端订阅 `/events` 并启动战役，15 秒内接收到 16 条事件：
```
engine → phase(init) → plan → step(port_scan) → tool(port_scan) 
→ phase(recon) → graph(host) → edge → graph(service:445) 
→ edge → graph(service:3306) → graph(开放端口与服务列表) 
→ step(http_probe) → tool(http_probe) → plan → step(http_probe)
```
**结论**: 后端事件流正常，前端白屏问题由 P1-3 修复。

### 停止按钮测试 (2026-04-03)
使用 Go 客户端验证 `campaign_stopped` 事件：
```
1. 订阅 /events
2. POST /start → 200 OK {"ok":true}
3. 等待 3 秒
4. POST /cancel → 200 OK {"ok":true}
5. 立即收到: data: {"data":{"reason":"操作员已停止"},"kind":"campaign_stopped"}
6. 随后收到: done → phase(done) → route → summary → done
```
**结论**: 停止按钮立即生效，双重保障（乐观更新 + SSE 事件）。

### 编译测试
- ✅ `go build` 通过
- ✅ `npm run build` 通过 (vite 421KB JS, 49KB CSS)
- ✅ 无类型错误

### 端口
- 默认端口 `8000` (无需修改)
- 启动: `./vero.exe` (默认) 或 `./vero.exe -port 8000`

---

## Git 状态

```
fix/ai-engine-and-security-patches (5 commits)
├── 8c979f5 fix(critical): 修复攻击图边缺失和exploit假阳性问题
├── 0eb9cf0 fix(tools): 移除searchsploit和poc_manager假数据输出
├── e7c6032 fix(frontend): SSE ingest容错 + issues清单
├── 7c2c53e fix(cancel): 停止按钮立即响应 — 后端广播campaign_stopped + 前端乐观更新
└── 7274627 fix(ui): 战役结束后自动恢复待命状态 — 无需手动点停止
```
