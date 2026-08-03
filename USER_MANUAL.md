# Vero 用户手册

**版本**: 1.1.0  
**更新日期**: 2026-08-03

---

## 📖 目录

1. [快速开始](#快速开始)
2. [核心概念](#核心概念)
3. [工具列表](#工具列表)
4. [命令行使用](#命令行使用)
5. [Web 作战室](#web-作战室)
6. [场景包系统](#场景包系统)
7. [安全注意事项](#安全注意事项)
8. [故障排除](#故障排除)
9. [API 参考](#api-参考)

---

## 🚀 快速开始

### 安装

```bash
# 下载预编译二进制
wget https://github.com/your-org/VERO/releases/latest/VERO.exe

# 或从源码构建
git clone https://github.com/your-org/VERO
cd VERO
go build -o VERO.exe ./cmd/VERO
```

### 第一次运行

```bash
# 自检模式 (无需外部依赖)
.\VERO.exe -selfcheck

# 启动 Web 作战室
.\VERO.exe
# 访问 http://127.0.0.1:8000
```

### 环境配置

```bash
# 可选: LLM 决策引擎
export ANTHROPIC_API_KEY="sk-ant-..."
export DEEPSEEK_API_KEY="sk-..."

# 可选: 指定模型 (默认 claude-opus-4-8 / deepseek-chat)
export VERO_MODEL="claude-opus-4-8"

# 可选: Metasploit RPC
msfrpcd -P password -U msf -a 127.0.0.1 -p 55553
```

> **提示**: 上述 LLM 配置(决策引擎 / API key / 模型 / 思考强度 / 决策预算)也可在 Web 作战室「设置」页签中配置, 改动即时生效并持久化到本机 `vero.config.json`(0600, 密钥只写盘不回显), 无需重启。详见下文 [Web 作战室](#web-作战室)。

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

> **依赖与自动安装**: 多数工具依赖外部二进制。`nuclei` / `ffuf` 可在「工具管理」页签一键自动下载到 `tools/bin`(SHA256 白名单校验); Python 系工具(`nxc`→`netexec`、`impacket`、`pypykatz`、`secretsdump`、`lsass_dump`、`sam_dump`)可一键 `pip install --user` 安装。详见 [Web 作战室](#web-作战室)。

---

## 💻 命令行使用

### 基础模式

```bash
# 启动 Web 作战室 (默认端口 8000)
.\VERO.exe

# 指定端口
.\VERO.exe -port 9000

# 指定数据库路径
.\VERO.exe -db /path/to/VERO.db
```

### 独立工具模式

#### 扫描类
```bash
# TCP 端口扫描
.\VERO.exe -scan 10.0.0.5

# Nmap 完整扫描
.\VERO.exe -nmap 10.0.0.5

# HTTP 指纹 + Nuclei 漏扫
.\VERO.exe -webscan http://example.com
```

#### 云环境
```bash
# AWS IMDS 元数据 (需在 EC2 内)
.\VERO.exe -cloud-aws enum

# Azure IMDS 元数据 (需在 Azure VM 内)
.\VERO.exe -cloud-azure enum

# GCP IMDS 元数据 (需在 GCP 内)
.\VERO.exe -cloud-gcp enum

# S3 bucket 公开访问检测
.\VERO.exe -cloud-s3 my-bucket
```

#### 容器逃逸
```bash
# Docker 容器逃逸检测 (需在容器内)
.\VERO.exe -container-escape check

# K8s ServiceAccount 提取 (需在 pod 内)
.\VERO.exe -k8s-sa enum
```

#### Metasploit
```bash
# 搜索 exploit 模块 (需 msfrpcd 运行)
.\VERO.exe -msf-search ms17_010
```

#### 自主渗透
```bash
# LLM 驱动的自主渗透 (需 API key)
export ANTHROPIC_API_KEY="sk-ant-..."
.\VERO.exe -agent http://example.com
```

### 调试模式

```bash
# 离线自检
.\VERO.exe -selfcheck

# 直接测试 SQLi 工具
.\VERO.exe -exploit http://localhost:3000

# HTTP 探测调试
.\VERO.exe -probe http://example.com
```

---

## 🖥️ Web 作战室

Web 作战室(默认 http://127.0.0.1:8000)共 5 个页签: 战役控制台 / 工具管理 / 工作流模板 / 报告 / 设置。本节介绍界面新增的配置、工具安装、中文展示与阶段进度能力。

### 1. 工作台「设置」面板

第 5 个页签「设置」用于在浏览器中配置决策引擎、API key、模型、思考强度与决策预算, 保存后即时生效并写入本机 `vero.config.json`(0600, 密钥只写盘不回显)。

| 配置项 | 说明 |
|--------|------|
| 决策引擎 | 自动 / Claude / DeepSeek / 脚本 下拉选择(带中文说明); 自动 = 有 key 用真实模型, 否则脚本 |
| ANTHROPIC_API_KEY | 密码框, 显示「已配置 / 未配置」徽标, 不回显明文; 留空 = 不修改, 显式清空用「清除」按钮 |
| DEEPSEEK_API_KEY | 同上 |
| 模型 | 留空 = 引擎默认(claude-opus-4-8 / deepseek-chat); 也可用环境变量 `VERO_MODEL` 覆盖 |
| 思考强度 | 滑块 0~1, 低 = 稳健, 高 = 发散 |
| 决策预算 | 单次战役决策迭代轮数上限 |

- 引擎选择为 Claude/DeepSeek 但缺少对应 key 时, 发起战役会回退脚本模式(面板会给出提示)。
- 「恢复默认」: 引擎=auto、思考强度=0.2、决策预算=10(模型名仅本地清空表单, 不写入配置文件)。
- API: GET /api/config 返回 engine/model/temperature/max_budget 及密钥是否已配置(`has_anthropic`/`has_deepseek` 布尔), 密钥部分只回布尔、绝不回传明文。

### 2. 工具自动下载安装

「工具管理」页签在「验证工具」后, 缺失依赖的工具会给出对应的安装按钮:

- **自动下载二进制**(nuclei / ffuf): 从官方 Release 下载到 `tools/bin`, SHA256 白名单校验, 防供应链投毒; 仅支持 amd64, 校验失败拒绝安装。
- **一键 pip 安装**(nxc→netexec、impacket、pypykatz、secretsdump、lsass_dump、sam_dump): 执行 `pip install --user`, 优先 `python3`(Windows 兼容 `py`), 不污染系统环境; 遇到 PEP 668 托管环境自动追加 `--break-system-packages` 重试。
- 顶部「全部自动安装」: 调 POST /api/tools/install-all 批量安装全部缺失工具(binary + pip), 串行执行, 单项失败不影响其余。
- 安装后的 `tools/bin` 与 pip 用户 scripts 目录会注入进程 PATH(仅本进程, 不动系统 PATH)。
- 验证接口 GET /api/tools/verify 对每个工具输出 `install_type` 三态: `binary`(可自动下载二进制) / `pip`(需 pip 安装) / `none`(无自动安装途径)。

### 3. 全中文界面与思考展示

信号流与全站文案改为中文(文案集中映射于 `web/src/lib/i18n.ts`):

- 事件标签: 思考 / 工具 / 授权请求 / 计划 / 引擎 / 摘要 / 完成等。
- 工具级别: L0-侦察 ~ L4-破坏; 口语化级别(如「利用级」); 攻击图节点状态: 已证实 / 待验证。
- `step` 事件展开为两行: 「思考 L{级} · {工具}」+ 缩进「▍推理 {why}」。
- `plan` 事件高亮显示整段计划推理 `rationale`(即 LLM 每步思考内容), 并显示计划步数。

### 4. 战役阶段进度条

战役控制台 KPI 面板下方显示战役阶段进度条: 待命 → 侦察 → 扫描 → 利用 → 完成。

- 阶段由 SSE 事件(engine / step / tool / route / summary / done)推断, 只前进不后退。
- 进度条下方实时显示当前动作与工具名(含 L 级别); 利用阶段(Level ≥ 2)以警示色高亮。
- 战役未发起时显示「尚未发起战役」。

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

import "VERO/internal/tools"

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
> 也可在 Web 作战室「设置」页签配置 API key, 即时生效, 无需重启。
```bash
# 使用 DeepSeek (更便宜)
export DEEPSEEK_API_KEY="sk-..."

# 或使用离线模式 (确定性规划器 / 脚本引擎)
.\VERO.exe -selfcheck
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
./VERO.exe -container-escape check
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

#### 6. 工具不可用 (依赖缺失)

**原因**: 工具依赖的外部二进制或 Python 包未安装

**解决**:
- 在「工具管理」页签点击「验证工具」, 缺失工具会显示对应的一键安装按钮。
- `nuclei` / `ffuf`: 点击「自动安装(二进制)」下载到 `tools/bin`(仅 amd64, SHA256 白名单校验)。
- Python 系工具(`nxc` / `impacket` / `pypykatz` / `secretsdump` / `lsass_dump` / `sam_dump`): 点击「一键安装(pip)」执行 `pip install --user`。
- 批量场景: 顶部「全部自动安装」一次安装全部缺失工具, 单项失败不影响其余。

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
    "VERO/internal/core"
    "VERO/internal/tools"
    "VERO/internal/scenarios"
    "VERO/internal/llm"
    "VERO/internal/planner"
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

- **GitHub**: https://github.com/your-org/VERO
- **文档**: https://docs.VERO.io
- **社区**: https://discord.gg/VERO
- **问题报告**: https://github.com/your-org/VERO/issues

---

## 📄 许可证

MIT License - 仅用于合法授权的安全测试。

---

**最后更新**: 2026-08-03  
**维护者**: Vero Team
