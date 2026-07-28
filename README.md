# Vero

<div align="center">

**Evidence-Driven Autonomous Red Team Agent**

[English](#english) | [中文](#chinese)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org)
[![GitHub Stars](https://img.shields.io/github/stars/Coff0xc/vero?style=social)](https://github.com/Coff0xc/vero)

</div>

---

<a name="english"></a>

## 🎯 What is Vero?

Vero is an **evidence-driven AI penetration testing agent** that solves the fundamental trust problem in AI security tools: **every finding must have proof**.

Unlike traditional AI red team tools that generate unverifiable claims, Vero enforces evidence constraints at the **type system level** — no evidence = no confirmation. This architectural choice eliminates hallucinations by design.

### The Problem with Traditional AI Security Tools

- **Hallucination Rate**: 20-30% of findings are fabricated
- **No Verification**: Claims without tool output
- **False Positives**: Wastes analyst time
- **Legal Risk**: Unverifiable reports in production

### How Vero is Different

```go
type Finding struct {
    Title    string
    Severity string
    Evidence Evidence  // REQUIRED, not optional
}

func (g *AttackGraph) Confirm(id string, ev Evidence) error {
    if ev == (Evidence{}) {
        return fmt.Errorf("evidence required")
    }
    // Evidence must exist verbatim in tool output
    if !VerifyEvidence(ev) {
        return fmt.Errorf("evidence not found in tool output")
    }
    // ...
}
```

**No evidence → No confirmation → No hallucination**

---

## 🚀 Quick Start

### Prerequisites

- Go 1.26+
- (Optional) Security tools: nmap, nuclei, curl, metasploit, nxc, etc.

### Installation

```bash
# Clone repository
git clone https://github.com/Coff0xc/vero.git
cd vero

# Build
go build -o vero ./cmd/vero

# Run self-check (offline, no API key needed)
./vero -selfcheck

# Start Web UI
./vero
# Open http://localhost:8000
```

### With Claude API

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export VERO_MODEL=claude-opus-4-8
./vero
```

---

## ✨ Core Features

### 1. Evidence-Driven Architecture

Every node in the attack graph must have:
- **Tool output**: Raw stdout/stderr
- **Excerpt**: Verbatim snippet from output
- **Parser**: Structured observations with source tracking

```go
type Node struct {
    ID       string
    Type     string      // host, service, finding, cred
    Label    string
    State    string      // hypothesis → confirmed → refuted
    Evidence []Evidence  // Must be verifiable
}
```

### 2. Human-in-the-Loop Safety

Actions are classified by danger level (L0-L4):

| Level | Type | HITL | Examples |
|-------|------|------|----------|
| **L0** | Reconnaissance | Auto | whois, passive fingerprinting |
| **L1** | Scanning | Auto | nmap, httpx, nuclei |
| **L2** | Credential Ops | Auto | kerberoast, dump |
| **L3** | Exploitation | **Required** | exploit, psexec, lateral movement |
| **L4** | Destructive | **Required** | backdoor, wiper, data destruction |

High-risk operations pause for operator approval.

### 3. Attack Graph

All discoveries are stored as a directed graph with evidence chains:

```
host:10.0.0.5 → service:10.0.0.5:80 → finding:sqli → cred:admin → foothold
```

You can trace **exactly how** each finding was discovered.

### 4. 26 Built-in Tools

**Network**: nmap, masscan, rustscan  
**Web**: nuclei, ffuf, sqlmap, nikto  
**Exploit**: metasploit (RPC), searchsploit  
**Cloud**: aws-cli, az-cli, gcloud, s3  
**Container**: docker, kubectl, trivy  
**AD**: bloodhound, crackmapexec (nxc), mimikatz, kerberoast  
**Post-Exploit**: lsass, sam, secretsdump  

### 5. 7 Scenario Packs

Pre-configured tool combinations for common engagements:

- **Web Application**: http_probe → nuclei → ffuf → exploit_sqli
- **Active Directory**: smb_enum → ldap_enum → kerberoast → wmi_exec
- **Cloud (AWS/Azure/GCP)**: IMDS extraction → IAM enum → S3 enumeration
- **Container/K8s**: Docker escape → ServiceAccount extraction
- **Post-Exploitation**: Credential dumping → Lateral movement
- **External Recon**: Passive fingerprinting → OSINT
- **Internal Network**: Port scanning → Service enumeration

### 6. Workflow Templates

5 predefined penetration testing workflows:

```
web-recon: HTTP fingerprint → vuln scan → dir brute
web-full: Recon → Scan → Exploit → Post-exploit
ad-recon: SMB enum → LDAP query → Credential attack
cloud-recon: IMDS extraction → Storage enumeration
container-escape: Docker escape → K8s SA extraction
```

### 7. Enhanced Reporting

- **Timeline View**: Phase-based event tracking (recon → exploit → post-exploit)
- **Attack Path Graph**: Node-edge visualization of exploitation chain
- **Risk Scoring**: 0-10 weighted score with CVSS integration
- **Export Formats**: JSON, Markdown, HTML

### 8. Web UI + CLI

**Web Interface** (http://localhost:8000):
- Real-time SSE event stream
- Interactive attack graph visualization
- Tool management dashboard
- Workflow execution interface
- Report viewer with timeline/graph

**CLI**:
- Color-coded output (success/error/warning)
- Progress bars for long operations
- ASCII art banner
- Formatted table output

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────┐
│          Web UI (React + SSE)           │
│   Campaign Console | Tools | Workflows  │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│           API Server (Go)               │
│  /start /approve /tools /workflows      │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         Core Loop (Evidence)            │
│  LLM → Action → HITL → Execute → Parse  │
│         → Verify → Update Graph         │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         Tools (32 built-in)             │
│   Network | Web | Exploit | Cloud | AD  │
└─────────────────────────────────────────┘
```

**Core Loop**:
1. LLM proposes action
2. Check danger level (L0-L4)
3. HITL gate if L3+
4. Execute tool
5. Parse output → Observations
6. Verify evidence (verbatim check)
7. Update attack graph
8. Repeat

---

## 📚 Documentation

- **[User Manual](USER_MANUAL.md)** - Complete guide (60 pages)
- **[Deployment Guide](DEPLOYMENT.md)** - Docker/K8s setup (50 pages)
- **[Project Summary](PROJECT_SUMMARY.md)** - Technical architecture
- **[Benchmark](benchmark/)** - Trustworthiness evaluation framework

---

## 🧪 Tool Verification

Check which tools are available on your system:

```bash
./vero -tooltest
```

Example output:
```
═══ Tool Verification ═══

Total: 26 tools
✓ Available: 8 (30.8%)
✗ Unavailable: 18 (69.2%)

By Level:
  L0-Recon: 2/5 available
  L1-Scan: 3/10 available
  L2-Cred: 1/7 available
  L3-Exploit: 2/4 available

Details:
  http_probe          [L1-Scan]   ✓ Available (45ms)
  nuclei              [L1-Scan]   ✓ Available (1203ms)
  nmap                [L1-Scan]   ✗ Unavailable (command not found)
  ...
```

---

## 🔒 Security & Ethics

**Vero is for authorized penetration testing only.**

- ⚠️ **Authorization required** before use
- ✅ **HITL gates** prevent accidental damage
- ✅ **Audit logs** for compliance (audit.jsonl)
- ✅ **Evidence chain** for legal defensibility
- ✅ **Rollback support** (rollback.jsonl)

**DO NOT use for unauthorized testing. That's illegal.**

---

## 🛠️ Development

```bash
# Run tests
make test

# Development mode (hot reload)
make dev-server  # Backend on :8000
make dev-web     # Frontend on :5173

# Build production binary
make build

# Clean build artifacts
make clean
```

---

## 🤝 Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new features
4. Submit a pull request

---

## 📜 License

MIT License - See [LICENSE](LICENSE)

---

## 🙏 Acknowledgments

- **Anthropic Claude** - LLM decision engine
- **Nuclei** - Vulnerability scanner
- **NetExec (nxc)** - AD enumeration
- **Metasploit** - Exploit framework

---

## 📊 Project Stats

- **26 tools** integrated
- **7 scenario packs** available
- **5 workflow templates** predefined
- **3 web components** (ToolManager, WorkflowManager, ReportViewer)
- **110+ pages** of documentation

---

<a name="chinese"></a>

## 🎯 什么是 Vero？

Vero 是一个**证据驱动的 AI 自主渗透测试智能体**，解决了 AI 安全工具的根本信任问题：**每个发现都必须有证据**。

与传统 AI 红队工具生成无法验证的声称不同，Vero 在**类型系统层面**强制执行证据约束——没有证据 = 无法确认。这种架构设计从根本上消除了幻觉。

### 传统 AI 安全工具的问题

- **幻觉率**: 20-30% 的发现是虚构的
- **无法验证**: 没有工具输出的声称
- **误报**: 浪费分析师时间
- **法律风险**: 生产环境中无法验证的报告

### Vero 的不同之处

```go
type Finding struct {
    Title    string
    Severity string
    Evidence Evidence  // 必需，不是可选
}

func (g *AttackGraph) Confirm(id string, ev Evidence) error {
    if ev == (Evidence{}) {
        return fmt.Errorf("证据必需")
    }
    // 证据必须逐字存在于工具输出中
    if !VerifyEvidence(ev) {
        return fmt.Errorf("工具输出中未找到证据")
    }
    // ...
}
```

**没有证据 → 无法确认 → 零幻觉**

---

## 🚀 快速开始

### 前置要求

- Go 1.26+
- (可选) 安全工具: nmap, nuclei, curl, metasploit, nxc 等

### 安装

```bash
# 克隆仓库
git clone https://github.com/Coff0xc/vero.git
cd vero

# 构建
go build -o vero ./cmd/vero

# 运行自检（离线，无需 API 密钥）
./vero -selfcheck

# 启动 Web UI
./vero
# 打开 http://localhost:8000
```

### 使用 Claude API

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export VERO_MODEL=claude-opus-4-8
./vero
```

---

## ✨ 核心功能

### 1. 证据驱动架构

攻击图中的每个节点都必须有：
- **工具输出**: 原始 stdout/stderr
- **摘录**: 输出中的逐字片段
- **解析器**: 带源跟踪的结构化观测

### 2. 人机协同安全

动作按危险等级分类 (L0-L4)：

| 等级 | 类型 | 人工审批 | 示例 |
|------|------|---------|------|
| **L0** | 侦察 | 自动 | whois, 被动指纹 |
| **L1** | 扫描 | 自动 | nmap, httpx, nuclei |
| **L2** | 凭证操作 | 自动 | kerberoast, dump |
| **L3** | 利用 | **必需** | exploit, psexec, 横向移动 |
| **L4** | 破坏性 | **必需** | 后门, 擦除器, 数据销毁 |

高风险操作会暂停等待操作员批准。

### 3. 攻击图

所有发现都存储为带证据链的有向图：

```
host:10.0.0.5 → service:10.0.0.5:80 → finding:sqli → cred:admin → foothold
```

可以追溯**每个发现是如何**被发现的。

### 4. 26 个内置工具

**网络**: nmap, masscan, rustscan  
**Web**: nuclei, ffuf, sqlmap, nikto  
**利用**: metasploit (RPC), searchsploit  
**云**: aws-cli, az-cli, gcloud, s3  
**容器**: docker, kubectl, trivy  
**AD**: bloodhound, crackmapexec (nxc), mimikatz, kerberoast  
**后渗透**: lsass, sam, secretsdump  

### 5. 7 个场景包

常见渗透测试的预配置工具组合：

- **Web 应用**: http_probe → nuclei → ffuf → exploit_sqli
- **Active Directory**: smb_enum → ldap_enum → kerberoast → wmi_exec
- **云环境 (AWS/Azure/GCP)**: IMDS 提取 → IAM 枚举 → S3 枚举
- **容器/K8s**: Docker 逃逸 → ServiceAccount 提取
- **后渗透**: 凭证转储 → 横向移动
- **外部侦察**: 被动指纹 → OSINT
- **内部网络**: 端口扫描 → 服务枚举

### 6. 工作流模板

5 个预定义渗透测试工作流：

```
web-recon: HTTP 指纹 → 漏洞扫描 → 目录爆破
web-full: 侦察 → 扫描 → 利用 → 后渗透
ad-recon: SMB 枚举 → LDAP 查询 → 凭证攻击
cloud-recon: IMDS 提取 → 存储枚举
container-escape: Docker 逃逸 → K8s SA 提取
```

### 7. 增强报告

- **时间线视图**: 基于阶段的事件追踪（侦察 → 利用 → 后渗透）
- **攻击路径图**: 利用链的节点-边可视化
- **风险评分**: 0-10 加权评分，集成 CVSS
- **导出格式**: JSON, Markdown, HTML

### 8. Web UI + CLI

**Web 界面** (http://localhost:8000):
- 实时 SSE 事件流
- 交互式攻击图可视化
- 工具管理仪表板
- 工作流执行界面
- 带时间线/图形的报告查看器

**CLI**:
- 彩色编码输出（成功/错误/警告）
- 长操作的进度条
- ASCII 艺术横幅
- 格式化表格输出

---

## 🏗️ 架构

```
┌─────────────────────────────────────────┐
│          Web UI (React + SSE)           │
│   战役控制台 | 工具 | 工作流              │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│           API 服务器 (Go)               │
│  /start /approve /tools /workflows      │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         核心循环（证据）                 │
│  LLM → 动作 → HITL → 执行 → 解析        │
│         → 验证 → 更新图                  │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         工具 (32 个内置)                 │
│   网络 | Web | 利用 | 云 | AD            │
└─────────────────────────────────────────┘
```

---

## 📚 文档

- **[用户手册](USER_MANUAL.md)** - 完整指南（60 页）
- **[部署指南](DEPLOYMENT.md)** - Docker/K8s 配置（50 页）
- **[项目摘要](PROJECT_SUMMARY.md)** - 技术架构
- **[基准测试](benchmark/)** - 可信度评估框架

---

## 🧪 工具验证

检查系统上哪些工具可用：

```bash
./vero -tooltest
```

示例输出：
```
═══ 工具验证 ═══

总计: 26 个工具
✓ 可用: 8 (30.8%)
✗ 不可用: 18 (69.2%)

按等级:
  L0-侦察: 2/5 可用
  L1-扫描: 3/10 可用
  L2-凭证: 1/7 可用
  L3-利用: 2/4 可用

详情:
  http_probe          [L1-扫描]   ✓ 可用 (45ms)
  nuclei              [L1-扫描]   ✓ 可用 (1203ms)
  nmap                [L1-扫描]   ✗ 不可用 (未找到命令)
  ...
```

---

## 🔒 安全与道德

**Vero 仅用于授权渗透测试。**

- ⚠️ 使用前**需要授权**
- ✅ **HITL 门控**防止意外损坏
- ✅ **审计日志**用于合规 (audit.jsonl)
- ✅ **证据链**用于法律辩护
- ✅ **回滚支持** (rollback.jsonl)

**请勿用于未授权测试。这是违法的。**

---

## 🛠️ 开发

```bash
# 运行测试
make test

# 开发模式（热重载）
make dev-server  # 后端在 :8000
make dev-web     # 前端在 :5173

# 构建生产二进制文件
make build

# 清理构建产物
make clean
```

---

## 🤝 贡献

欢迎贡献！请：

1. Fork 仓库
2. 创建特性分支
3. 为新功能添加测试
4. 提交 pull request

---

## 📜 许可证

MIT License - 见 [LICENSE](LICENSE)

---

## 🙏 致谢

- **Anthropic Claude** - LLM 决策引擎
- **Nuclei** - 漏洞扫描器
- **NetExec (nxc)** - AD 枚举
- **Metasploit** - 利用框架

---

## 📊 项目统计

- **26 个工具**已集成
- **7 个场景包**可用
- **5 个工作流模板**预定义
- **3 个 Web 组件** (ToolManager, WorkflowManager, ReportViewer)
- **110+ 页**文档

---

## ⚠️ 功能状态

### ✅ 已实现

- 证据驱动架构
- 26 个工具集成
- 7 个场景包
- 5 个工作流模板
- Web UI (3 个 Tab: 战役/工具/工作流)
- 增强报告生成（时间线 + 攻击路径图）
- CLI 优化（彩色输出 + 进度条）
- HITL 安全门控
- 攻击图可视化
- 工具验证系统

### ❌ 未实现（社区可贡献）

- ~~云端 API 后端（用户认证 + 战役存储）~~
- ~~定价页面和支付流程~~
- ~~持续监控调度器（Enterprise 功能）~~
- ~~多项目管理 + RBAC~~

**当前版本**为**开源自托管版本**，专注于核心渗透测试能力。企业级功能（云端、定价、RBAC）不在当前范围内。

---

**作者**: coff0xc  
**GitHub**: https://github.com/Coff0xc/vero  
**许可**: MIT
