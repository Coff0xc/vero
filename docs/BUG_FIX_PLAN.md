# Vero Bug 修复计划

## 问题统计

根据您的审计报告：

- **真实问题**: 4个 (环境配置)
- **实战Bug**: 10个 (测试发现)
- **深度审计Bug**: 39个 (代码审查)
- **架构缺陷**: 6个 (设计层面)

**总计**: 59个问题

---

## 修复优先级

### P0 - 严重 (核心功能失效，必须立即修复)

| ID | 问题 | 影响 | 预计时间 |
|----|------|------|----------|
| D1 | 攻击图边严重缺失 | 攻击链可视化无效、FindPath失效 | 2小时 |
| D3 | claim永久hypothesis | LLM验证机制完全失效 | 1.5小时 |
| D32 | Reflexion未接入主循环 | 反射学习系统未启用 | 1小时 |
| D11-D13 | exploit/poc模拟输出 | 假阳性误导用户 | 1小时 |
| A2 | LLM schema缺少关键字段 | claim/produces机制失效 | 2小时 |

**P0 小计**: 5个问题，预计 7.5小时

---

### P1 - 高优先级 (影响用户体验，应尽快修复)

| ID | 问题 | 影响 | 预计时间 |
|----|------|------|----------|
| D9-D10 | ffuf Windows路径问题 | Windows环境ffuf不可用 | 1小时 |
| D6 | UpsertNode证据重复 | 报告重复内容 | 30分钟 |
| D36 | ParseSearchsploit Key为空 | exploit节点合并错误 | 20分钟 |
| D23 | handleStart不校验target | 可能打错目标 | 30分钟 |
| D14 | ParseNmapXML Excerpt错误 | 证据回查失败 | 30分钟 |

**P1 小计**: 5个问题，预计 2.5小时

---

### P2 - 中优先级 (功能增强，建议修复)

| ID | 问题 | 预计时间 |
|----|------|----------|
| D2 | probe模式不建边 | 30分钟 |
| D4 | produces边只连前一阶段 | 1小时 |
| D5 | 停滞检测sig不稳定 | 30分钟 |
| D7 | VerifyEvidence匹配粗糙 | 1小时 |
| D15 | IPv6地址处理错误 | 30分钟 |
| D18 | 报告严重度来源不一致 | 30分钟 |
| D27 | SQLite串行化 | 1小时 |
| D31 | hostOf默认返回错误IP | 20分钟 |
| D33 | ShouldRetry未接入 | 1小时 |
| D35 | ffuf -se参数错误 | 10分钟 |

**P2 小计**: 10个问题，预计 6.5小时

---

### P3 - 低优先级 (优化改进，可延后)

其余 39个问题，预计需要 15-20小时

---

## 修复策略

### 方案 A: 快速修复核心功能 (推荐)

**目标**: 修复 P0+P1 (10个最严重问题)  
**时间**: 10小时  
**效果**: 攻击图可用 + LLM验证正常 + Windows支持 + 无假阳性

**执行步骤**:
1. 创建修复分支 `fix/critical-bugs`
2. 按优先级逐一修复 P0 问题
3. 运行完整测试验证
4. 修复 P1 问题
5. 生成修复报告并推送

### 方案 B: 全面修复 (长期)

**目标**: 修复所有 59个问题  
**时间**: 30-40小时  
**效果**: 生产级质量

### 方案 C: 架构重构 (最彻底)

**目标**: 解决 6个架构缺陷  
**时间**: 50-80小时  
**效果**: 重新设计核心机制

---

## 立即可执行的修复

我可以立即开始修复以下问题 (无需您介入)：

### 1. 修复攻击图边缺失 (D1)
```go
// internal/core/loop.go:282
// 为 finding/endpoint/claim 类型也建边
func (l *Loop) applyObservations(obs []tools.Observation) {
    for _, o := range obs {
        // ... 现有 service 逻辑 ...
        
        // 新增: finding 连接到其来源 service
        if o.Kind == "finding" {
            // 从 context 中找到触发此 finding 的 service
            if sourceService := l.findSourceService(o); sourceService != "" {
                l.Graph.AddEdge(sourceService, nid, "exposes", nil)
            }
        }
    }
}
```

### 2. 接入 Reflexion 系统 (D32)
```go
// internal/core/loop.go:191
if !ok {
    // 新增: 记录失败到 Reflexion
    if l.Reflector != nil {
        l.Reflector.RecordFailure(action.Tool, action.Args, res.Stderr)
    }
    return false
}
```

### 3. 修复 exploit 假输出 (D11)
```go
// internal/scenarios/exploit_library.go:186
func exploitCVE(args map[string]any) tools.ToolResult {
    // ... 检查工具是否存在 ...
    if !scriptExists {
        return tools.ToolResult{
            Success: false,
            Stderr:  fmt.Sprintf("未找到 %s 的利用脚本", cve),
        }
    }
    // 只在工具存在时才执行真实利用
}
```

---

## 您的选择

请告诉我：

**A. 立即修复 P0 核心问题** (7.5小时，攻击图+LLM验证+反射学习)  
**B. 修复 P0+P1** (10小时，包含Windows支持)  
**C. 生成详细修复PR，您自行审查后合并**  
**D. 仅修复特定问题** (请指定问题ID)

我会创建独立分支、逐一修复、测试验证、生成报告并推送。
