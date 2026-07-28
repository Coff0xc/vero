# Vero 用户手册

**版本**: 1.0.0  
**更新日期**: 2026-07-28

---

## 📖 目录

1. [快速开始](#快速开始)
2. [核心概念](#核心概念)
3. [工具列表](#工具列表)
4. [命令行使用](#命令行使用)
5. [场景包系统](#场景包系统)
6. [安全注意事项](#安全注意事项)
7. [故障排除](#故障排除)
8. [API 参考](#api-参考)

---

## 🚀 快速开始

### 安装

```bash
# 下载预编译二进制
wget https://github.com/your-org/redcell/releases/latest/redcell.exe

# 或从源码构建
git clone https://github.com/your-org/redcell
cd redcell
go build -o redcell.exe ./cmd/redcell
```

### 第一次运行

```bash
# 自检模式 (无需外部依赖)
.\redcell.exe -selfcheck

# 启动 Web 作战室
.\redcell.exe
# 访问 http://127.0.0.1:8000
```

### 环境配置

```bash
# 可选: LLM 决策引擎
export ANTHROPIC_API_KEY="sk-ant-..."
export DEEPSEEK_API_KEY="sk-..."

# 可选: Metasploit RPC
msfrpcd -P password -U msf -a 127.0.0.1 -p 55553
```

---

## 🧠 核心概念

### 1. 攻击图 (Attack Graph)

Vero 使用**证据驱动的攻击图**追踪渗透路径：

```
[扫描] port_scan(10.0.0.5)
  ↓ 发现: service:10.0.0.5:80 (http)
[指纹] http_probe(10.0.0.5:80)
  ↓ 发现: tech:nginx, tech:php
[漏扫] web_vuln_scan(10.0.0.5)
  ↓ 发现: finding:sqli_login
[利用] exploit_sqli(10.0.0.5)
  ↓ 获得: cred:admin_token
```

### 2. 工具 Level 分级 (HITL 门控)

| Level | 类型 | 行为 | 示例工具 |
|-------|------|------|----------|
| **1 - LevelScan** | 扫描/枚举 | 自动执行 | port_scan, nmap_scan, http_probe |
| **2 - LevelCred** | 凭证提取 | 自动执行 | secretsdump, lsass_dump, k8s_sa_enum |
| **3 - LevelExploit** | 漏洞利用 | **需人工审批** | exploit_sqli, msf_execute, k8s_node_exploit |

### 3. 场景包 (Scenario Packs)

场景包根据**服务指纹**自动激活工具集：

```go
// Web 场景包
http/https 服务 → 激活 http_probe, web_vuln_scan, ffuf_dir_brute, exploit_sqli

// AD 场景包
SMB/LDAP/Kerberos → 激活 nxc_smb_spray, nxc_ldap_enum, kerberoast

// 云场景包
总是激活 → aws_imds_enum, azure_imds_enum, gcp_imds_enum, s3_bucket_enum

// 容器场景包
检测到 /.dockerenv → 激活 docker_escape_check, k8s_sa_enum, k8s_node_exploit
```

### 4. 反幻觉机制

所有 Parser 必须遵循：
- **证据可溯**: 每个 Observation 有 `Excerpt` 字段指向原始输出
- **逐字匹配**: 使用 `strings.Contains()` 而非推测
- **去重机制**: 避免重复提取同一事实

---

## 🔧 工具列表

### Web 渗透 (5 个工具)

| 工具名 | Level | 功能 | 用法 |
|--------|-------|------|------|
| `http_probe` | 1 | HTTP 指纹识别 | `target: http://example.com` |
| `web_vuln_scan` | 1 | Nuclei 漏洞扫描 | `target: http://example.com` |
| `ffuf_dir_brute` | 1 | 目录爆破 | `target: http://example.com, wordlist: /path/to/list` |
| `ffuf_vhost_enum` | 1 | 虚拟主机枚举 | `domain: example.com, wordlist: /path/to/list` |
| `exploit_sqli` | **3** | SQLi 登录绕过 | `target: http://example.com` |

### AD/内网渗透 (8 个工具)

| 工具名 | Level | 功能 | 用法 |
|--------|-------|------|------|
| `smb_enum` | 1 | SMB 枚举 | `target: 10.0.0.5` |
| `kerberoast` | 2 | Kerberoasting | `target: 10.0.0.5, user: alice, pass: P@ss` |
| `nxc_smb_spray` | 1 | SMB 密码喷洒 | `target: 10.0.0.0/24, user: admin, pass: P@ss` |
| `nxc_ldap_enum` | 1 | LDAP 用户枚举 | `target: dc01.corp.local, user: alice, pass: P@ss` |
| `nxc_ldap_computers` | 1 | LDAP 主机枚举 | `target: dc01.corp.local, user: alice, pass: P@ss` |
| `nxc_wmi_exec` | **3** | WMI 远程执行 | `target: 10.0.0.5, user: admin, pass: P@ss, cmd: whoami` |
| `nxc_asrep` | 2 | AS-REP Roasting | `target: dc01.corp.local` |
| `nxc_smb_shares` | 1 | SMB 共享枚举 | `target: 10.0.0.5, user: alice, pass: P@ss` |

### 后渗透 (3 个工具)

| 工具名 | Level | 功能 | 用法 |
|--------|-------|------|------|
| `secretsdump` | 2 | NTLM/Kerberos 提取 | `target: 10.0.0.5, user: admin, pass: P@ss` |
| `lsass_dump` | 2 | LSASS 内存凭证 | `target: 10.0.0.5` |
| `sam_dump` | 2 | SAM 数据库提取 | `target: 10.0.0.5` |

### Metasploit 集成 (3 个工具)

| 工具名 | Level | 功能 | 用法 |
|--------|-------|------|------|
| `msf_search` | 1 | 搜索 exploit 模块 | `query: ms17_010, msf_url: http://127.0.0.1:55553` |
| `msf_execute` | **3** | 执行漏洞利用 | `module: windows/smb/ms17_010_eternalblue, target: 10.0.0.5` |
| `msf_get_sessions` | 2 | 列出 Meterpreter 会话 | `msf_url: http://127.0.0.1:55553` |

### 云环境侦察 (4 个工具)

| 工具名 | Level | 功能 | 用法 |
|--------|-------|------|------|
| `aws_imds_enum` | 1 | AWS EC2 元数据提取 | 无参数 (需在 EC2 内运行) |
| `azure_imds_enum` | 1 | Azure VM 元数据提取 | 无参数 (需在 Azure VM 内) |
| `gcp_imds_enum` | 1 | GCP 实例元数据提取 | 无参数 (需在 GCP 内) |
| `s3_bucket_enum` | 1 | S3 公开访问检测 | `bucket: my-bucket` |

### 容器逃逸 (3 个工具)

| 工具名 | Level | 功能 | 用法 |
|--------|-------|------|------|
| `docker_escape_check` | 1 | Docker 逃逸向量检测 | 无参数 (需在容器内) |
| `k8s_sa_enum` | 2 | K8s ServiceAccount 提取 | `api_url: https://kubernetes.default.svc` |
| `k8s_node_exploit` | **3** | K8s 节点逃逸利用 | 无参数 (需在 pod 内) |

### 基础工具 (6 个)

| 工具名 | Level | 功能 | 用法 |
|--------|-------|------|------|
| `port_scan` | 1 | TCP 端口扫描 | `target: 10.0.0.5, ports: 80,443,22` |
| `nmap_scan` | 1 | Nmap 完整扫描 | `target: 10.0.0.5` |
| `fake_scan` | 1 | 模拟扫描 (演示用) | `target: 10.0.0.5` |

---

## 💻 命令行使用

### 基础模式

```bash
# 启动 Web 作战室 (默认端口 8000)
.\redcell.exe

# 指定端口
.\redcell.exe -port 9000

# 指定数据库路径
.\redcell.exe -db /path/to/redcell.db
```

### 独立工具模式

#### 扫描类
```bash
# TCP 端口扫描
.\redcell.exe -scan 10.0.0.5

# Nmap 完整扫描
.\redcell.exe -nmap 10.0.0.5

# HTTP 指纹 + Nuclei 漏扫
.\redcell.exe -webscan http://example.com
```

#### 云环境
```bash
# AWS IMDS 元数据 (需在 EC2 内)
.\redcell.exe -cloud-aws enum

# Azure IMDS 元数据 (需在 Azure VM 内)
.\redcell.exe -cloud-azure enum

# GCP IMDS 元数据 (需在 GCP 内)
.\redcell.exe -cloud-gcp enum

# S3 bucket 公开访问检测
.\redcell.exe -cloud-s3 my-bucket
```

#### 容器逃逸
```bash
# Docker 容器逃逸检测 (需在容器内)
.\redcell.exe -container-escape check

# K8s ServiceAccount 提取 (需在 pod 内)
.\redcell.exe -k8s-sa enum
```

#### Metasploit
```bash
# 搜索 exploit 模块 (需 msfrpcd 运行)
.\redcell.exe -msf-search ms17_010
```

#### 自主渗透
```bash
# LLM 驱动的自主渗透 (需 API key)
export ANTHROPIC_API_KEY="sk-ant-..."
.\redcell.exe -agent http://example.com
```

### 调试模式

```bash
# 离线自检
.\redcell.exe -selfcheck

# 直接测试 SQLi 工具
.\redcell.exe -exploit http://localhost:3000

# HTTP 探测调试
.\redcell.exe -probe http://example.com
```

---

## 📦 场景包系统

### 查看激活的场景包

在 Web 作战室中，场景包会根据发现的服务自动激活。

### 场景包列表

| 场景包名 | 激活条件 | 工具数 | 说明 |
|----------|----------|--------|------|
| **web** | http/https 服务 | 5 | Web 渗透全套 |
| **ad** | SMB/LDAP/Kerberos | 2 | AD 基础枚举 |
| **ad_enhanced** | SMB/LDAP/Kerberos | 6 | NetExec 增强包 |
| **post_exploit** | 手动激活 | 3 | 凭证提取 |
| **exploit** | 手动激活 | 3 | Metasploit 集成 |
| **cloud** | **总是激活** | 4 | 云环境侦察 |
| **container** | 容器环境 | 3 | 容器逃逸 |

### 自定义场景包

创建 `custom_pack.go`:

```go
package scenarios

import "redcell/internal/tools"

func CustomPack() Pack {
    return Pack{
        Name: "custom",
        Tools: []*tools.Tool{
            {
                Name:  "my_tool",
                Level: tools.LevelScan,
                Desc:  "我的自定义工具",
                Run: func(args map[string]any) tools.ToolResult {
                    // 实现逻辑
                    return tools.ToolResult{Success: true, Stdout: "结果"}
                },
                Parse: func(stdout string, args map[string]any) []tools.Observation {
                    // 解析输出
                    return []tools.Observation{{Kind: "finding", Key: "custom:result", Label: "发现"}}
                },
            },
        },
        Fingerprint: func(services map[string]bool) bool {
            return services["custom-service"]
        },
    }
}
```

注册到 `scenarios.go`:

```go
func RegisterDefaults(m *Manager, reg *tools.Registry) {
    // ... 现有场景包
    m.Register(reg, CustomPack())  // 添加自定义包
}
```

---

## 🔒 安全注意事项

### 1. 授权范围

**⚠️ 警告**: 未经授权使用 Vero 进行渗透测试是**违法行为**。

**使用前必须获得**:
- 书面授权许可
- 明确的测试范围
- 应急联系人

### 2. Level 3 工具审批

以下工具需要人工审批 (HITL):

```
exploit_sqli         - SQLi 登录绕过
nxc_wmi_exec         - WMI 远程执行
msf_execute          - Metasploit 漏洞利用
k8s_node_exploit     - K8s 节点逃逸
```

在 Web 作战室中，执行这些工具前会弹出审批对话框。

### 3. 审计日志

所有操作记录在 `audit.jsonl`:

```json
{"ts":"2026-07-28T12:00:00Z","action":"exploit_sqli","args":{"target":"http://example.com"},"level":3,"approved":true}
```

回滚日志记录在 `rollback.jsonl`。

### 4. 云环境特别注意

**AWS IMDS**:
- 默认提取 IAM 凭证 → 高风险
- 仅在授权渗透测试中使用
- 提取的凭证应立即上报并轮换

**容器逃逸**:
- 特权容器检测可能触发 EDR 告警
- K8s API 访问可能被审计日志记录
- Docker socket 操作不可逆

### 5. Metasploit 集成

**msfrpcd 安全**:
- 使用强密码 (16+ 字符)
- 绑定 127.0.0.1 (不对外暴露)
- 定期更新 Metasploit Framework

```bash
# 安全启动 msfrpcd
msfrpcd -P $(openssl rand -base64 32) -U msf -a 127.0.0.1 -p 55553 -S
```

---

## 🐛 故障排除

### 常见问题

#### 1. "tool not registered"

**原因**: 工具未注册到场景包

**解决**:
```bash
# 检查工具列表
go test ./internal/tools -v -run TestRegistry

# 确认场景包注册
go test ./internal/scenarios -v -run TestPackRegister
```

#### 2. "ANTHROPIC_API_KEY not found"

**原因**: 缺少 LLM API key

**解决**:
```bash
# 使用 DeepSeek (更便宜)
export DEEPSEEK_API_KEY="sk-..."

# 或使用离线模式 (确定性规划器)
.\redcell.exe -selfcheck
```

#### 3. Cloud 工具超时

**原因**: 不在云环境中运行

**预期行为**: 
- AWS IMDS: 10s 超时 (非 EC2 环境)
- Azure IMDS: 5s 超时 (非 Azure VM)
- GCP IMDS: 5s 超时 (非 GCP 实例)

**验证**:
```bash
# AWS
curl -s --max-time 2 http://169.254.169.254/latest/meta-data/

# Azure
curl -s --max-time 2 -H "Metadata:true" http://169.254.169.254/metadata/instance?api-version=2021-02-01

# GCP
curl -s --max-time 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/
```

#### 4. Container 工具失败

**原因**: 不在容器环境

**解决**:
```bash
# 在 Docker 容器内运行
docker run -v $(pwd):/app -w /app alpine sh
# 容器内
./redcell.exe -container-escape check
```

#### 5. Metasploit RPC 连接失败

**原因**: msfrpcd 未运行或端口错误

**解决**:
```bash
# 启动 msfrpcd
msfrpcd -P password -U msf -a 127.0.0.1 -p 55553

# 测试连接
curl -X POST http://127.0.0.1:55553/api/1.0/auth.login \
  -H "Content-Type: application/json" \
  -d '{"username":"msf","password":"password"}'
```

### 性能问题

#### Nuclei 扫描慢

**优化**:
```bash
# 减少并发
nuclei -u http://target -c 10

# 只扫描关键模板
nuclei -u http://target -tags cve,misconfig
```

#### Parser 超时

**诊断**:
```bash
# 运行 benchmark
go test ./internal/scenarios -bench=BenchmarkParse -benchtime=3s

# 预期: 所有 Parser < 15 µs
```

---

## 📚 API 参考

### Go 包导入

```go
import (
    "redcell/internal/core"
    "redcell/internal/tools"
    "redcell/internal/scenarios"
    "redcell/internal/llm"
    "redcell/internal/planner"
)
```

### 创建工具

```go
tool := &tools.Tool{
    Name:  "my_scanner",
    Level: tools.LevelScan,
    Desc:  "扫描目标端口",
    Run: func(args map[string]any) tools.ToolResult {
        target := tools.ArgStr(args, "target", "")
        // 执行扫描
        return tools.ToolResult{
            Success: true,
            Stdout:  "扫描结果",
        }
    },
    Parse: func(stdout string, args map[string]any) []tools.Observation {
        return []tools.Observation{
            {
                Kind:    "service",
                Key:     "target:80",
                Label:   "HTTP 服务",
                Excerpt: "80/tcp open http",
            },
        }
    },
}
```

### 运行自主渗透

```go
reg := tools.NewRegistry()
sm := scenarios.NewManager()
scenarios.RegisterDefaults(sm, reg)

llm := llm.NewClaude(reg)
goal := "对目标 10.0.0.5 进行渗透测试"

approve := func(action core.Action, level int) bool {
    return level < 3 // 自动批准 Level < 3
}

emit := func(e core.Event) {
    fmt.Printf("[%s] %v\n", e.Kind, e.Data)
}

graph, trace := core.RunAgent(goal, llm, reg, approve, emit, 10)
```

### 构建攻击图

```go
g := core.NewGraph()

// 添加观测
obs := tools.Observation{
    Kind:    "service",
    Key:     "10.0.0.5:80",
    Label:   "HTTP 服务",
    Excerpt: "80/tcp open http",
}

action := core.Action{Tool: "port_scan", Args: map[string]any{"target": "10.0.0.5"}}
evidence := "Nmap scan report for 10.0.0.5\n80/tcp open http"

g.Add(obs, action, evidence)

// 验证证据
violations := core.VerifyEvidence(g, trace)
```

---

## 🔗 资源链接

- **GitHub**: https://github.com/your-org/redcell
- **文档**: https://docs.redcell.io
- **社区**: https://discord.gg/redcell
- **问题报告**: https://github.com/your-org/redcell/issues

---

## 📄 许可证

MIT License - 仅用于合法授权的安全测试。

---

**最后更新**: 2026-07-28  
**维护者**: Vero Team
