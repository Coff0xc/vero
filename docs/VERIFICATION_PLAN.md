# 当前成果验证计划

## 🎯 验证目标

验证 10 个已完成项目的实际可用性，确保：
1. 工具能被 LLM 正确调用
2. Parser 能正确提取结果
3. 反射学习能持久化教训
4. 前端依赖面板能正常工作

---

## 📋 验证清单

### 阶段 1: 快速冒烟测试 (15 分钟)

#### 1.1 系统编译验证
- [ ] 后端编译通过 (`go build ./cmd/vero`)
- [ ] 前端构建通过 (`npm run build`)
- [ ] 测试套件通过 (反射学习/代码审计/云渗透/K8s)

#### 1.2 服务启动验证
- [ ] 后端服务启动 (`./vero`)
- [ ] 前端开发服务启动 (`npm run dev`)
- [ ] 依赖检测 API 响应 (`curl http://localhost:8080/api/dependencies`)

#### 1.3 前端依赖面板验证
- [ ] 设置页面加载
- [ ] 工具依赖 Tab 切换
- [ ] 依赖列表显示（已安装/缺失）
- [ ] 刷新按钮工作

---

### 阶段 2: 核心功能验证 (30 分钟)

#### 2.1 代码审计能力验证
```bash
# 创建测试代码
mkdir -p /tmp/test-vero
cat > /tmp/test-vero/test.py <<'EOF'
password = "admin123"  # B105: 硬编码密码
import os
os.system(user_input)  # B602: 命令注入
EOF

# 启动战役（观察日志）
curl -X POST http://localhost:8080/start \
  -H "Content-Type: application/json" \
  -d '{"target": "/tmp/test-vero"}'

# 验证点：
# - [ ] LLM 识别到代码仓库
# - [ ] 调用 bandit_scan 或 semgrep_scan
# - [ ] Parser 提取到 "password" 相关 finding
```

#### 2.2 反射学习验证
```bash
# 触发失败（工具不存在）
curl -X POST http://localhost:8080/start \
  -H "Content-Type: application/json" \
  -d '{"target": "invalid://test"}'

# 检查 lessons 表
sqlite3 vero.db "SELECT tool, mode, reason FROM lessons ORDER BY created_at DESC LIMIT 3"

# 验证点：
# - [ ] lessons 表有记录
# - [ ] mode 字段正确分类（tool_missing/network/...）
# - [ ] solution 有建议内容
```

#### 2.3 工具依赖检测验证
```bash
# 检查依赖 API
curl -s http://localhost:8080/api/dependencies | jq .

# 验证点：
# - [ ] dependencies 数组有 16 个元素
# - [ ] 核心工具 (nuclei/nmap) installed 状态正确
# - [ ] 缺失工具有 install_hint
# - [ ] missing_count 数字准确
```

---

### 阶段 3: 集成验证 (45 分钟)

#### 3.1 真实靶场验证 - 代码审计
```bash
# 使用真实 Python 项目
git clone https://github.com/OWASP/NodeGoat /tmp/nodegoat
cd /tmp/nodegoat

# 启动 Vero 扫描
curl -X POST http://localhost:8080/start \
  -H "Content-Type: application/json" \
  -d '{"target": "/tmp/nodegoat"}'

# 观察 SSE 事件流
curl -N http://localhost:8080/events

# 验证点：
# - [ ] dependency_check 检测到 CVE (express/lodash 旧版本)
# - [ ] semgrep_scan 检测到 SQL 注入/XSS
# - [ ] findings 数量 > 5
```

#### 3.2 工具链完整性验证
```bash
# 检查所有工具是否注册
curl -s http://localhost:8080/api/tools 2>/dev/null || \
  echo "需要实现 /api/tools 端点"

# 手动验证核心工具
tools=(
  "semgrep_scan"
  "bandit_scan"
  "aws_s3_enum"
  "aws_iam_privesc"
  "k8s_enum_pods"
  "k8s_rbac_check"
  "docker_escape_exploit"
)

for tool in "${tools[@]}"; do
  # 检查工具是否在代码中注册
  grep -r "Name.*$tool" internal/scenarios/
done
```

#### 3.3 前端完整流程验证
```bash
# 1. 打开浏览器: http://localhost:5173
# 2. 检查设置面板 → 工具依赖
# 3. 点击刷新按钮
# 4. 启动一次战役
# 5. 观察攻击图是否构建
# 6. 检查 Evidence Drawer 是否有证据

# 验证点：
# - [ ] 依赖面板显示正常
# - [ ] 战役能启动
# - [ ] 攻击图有节点
# - [ ] Evidence 有内容
# - [ ] Findings 表有数据
```

---

### 阶段 4: 问题诊断 (如发现问题)

#### 常见问题检查
1. **LLM 不调用新工具**
   ```bash
   # 检查 RegisterDefaults
   grep -A 20 "RegisterDefaults" internal/scenarios/scenarios.go
   
   # 检查工具是否有 Args 规格
   grep -A 5 "Args.*ArgSpec" internal/scenarios/code_audit.go
   ```

2. **Parser 不提取观察**
   ```bash
   # 检查 Parser 是否注册
   grep "Parse:.*Parse" internal/scenarios/code_audit.go
   
   # 检查输出格式
   semgrep --config auto --json /tmp/test-vero/
   ```

3. **反射学习不生效**
   ```bash
   # 检查 lessons 表结构
   sqlite3 vero.db ".schema lessons"
   
   # 检查 OnFailure 调用
   grep -r "OnFailure" internal/core/loop.go
   ```

4. **前端依赖面板空白**
   ```bash
   # 检查 API 响应
   curl -v http://localhost:8080/api/dependencies
   
   # 检查前端 console
   # 浏览器 F12 → Console → 查看错误
   ```

---

## 📊 成功标准

### 最低标准 (必须通过)
- ✅ 系统编译无错误
- ✅ 服务正常启动
- ✅ 依赖检测 API 返回数据
- ✅ 前端依赖面板显示

### 良好标准 (期望通过)
- ✅ 至少 1 个新工具被 LLM 调用
- ✅ Parser 提取到至少 1 个观察
- ✅ lessons 表有记录
- ✅ 前端流程完整

### 优秀标准 (理想目标)
- ✅ 3+ 个新工具被调用
- ✅ 10+ 个观察被提取
- ✅ 反射学习自动 retry
- ✅ 攻击图清晰可读

---

## 🐛 问题记录模板

发现问题时记录：

```markdown
### 问题 X: [简短描述]
- **现象**: [具体表现]
- **重现步骤**: [如何触发]
- **预期行为**: [应该怎样]
- **实际行为**: [实际怎样]
- **相关日志**: [错误信息]
- **优先级**: 高/中/低
```

---

## ⏱️ 时间分配

- 阶段 1 (冒烟): 15 分钟
- 阶段 2 (核心): 30 分钟
- 阶段 3 (集成): 45 分钟
- 阶段 4 (诊断): 30-60 分钟 (按需)

**总计**: 1.5-2.5 小时

---

## 📝 验证报告模板

完成后填写：

```markdown
# 验证报告 - YYYY-MM-DD

## 通过项目
- [x] 项目 1: 描述
- [x] 项目 2: 描述

## 失败项目
- [ ] 项目 X: 原因

## 发现问题
1. 问题描述 (优先级: 高)
2. 问题描述 (优先级: 中)

## 下一步
- 修复问题 X
- 继续剩余 6 个项目
```

---

**准备就绪！开始验证？**
