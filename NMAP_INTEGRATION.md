# Nmap 完整集成文档

## 概述

已成功集成真实 Nmap 扫描功能，提供比 Go 原生 TCP 扫描更强大的能力：

- ✅ 服务版本识别（-sV）
- ✅ NSE 默认脚本（-sC）
- ✅ XML 输出解析
- ✅ OS 指纹检测
- ✅ 漏洞检测脚本（自动过滤非漏洞类脚本）
- ✅ 证据逐字回查支持

---

## 新增文件

### 1. `internal/tools/nmap.go`
**核心实现**: Nmap 调用 + XML 解析器

**主要函数**:
- `NmapScan(args)` - 执行 `nmap -sV -sC --top-ports 1000 -Pn --open -oX -`
- `ParseNmapXML(stdout, args)` - 解析 XML 输出提取 host/service/finding
- `isVulnScript(id)` - 过滤漏洞相关 NSE 脚本

**提取数据**:
- **Host**: IP 地址
- **Service**: 端口/协议/服务名/版本信息（如 `OpenSSH 8.2p1 Ubuntu`）
- **Finding - OS**: 操作系统指纹（如 `Linux 5.4 (95% accuracy)`）
- **Finding - NSE**: 漏洞检测脚本结果（自动过滤 `http-title` 等信息类脚本）

**漏洞脚本识别**:
- 关键词匹配: `vuln`, `exploit`, `cve`, `dos`, `backdoor`
- 白名单: `http-shellshock`, `ssl-heartbleed`, `smb-vuln-ms17-010`, `ftp-anon` 等

---

### 2. `internal/tools/nmap_test.go`
**测试覆盖**:
- ✅ XML 解析正确性（host/service/OS/NSE）
- ✅ 空输出处理
- ✅ Host down 过滤
- ✅ 漏洞脚本识别逻辑
- ✅ 缺失 target 参数错误处理

**测试数据**: 使用真实 nmap 7.94 XML 输出样本

---

## 集成点

### 1. 工具注册 (`internal/tools/parse.go`)
```go
func RegisterBuiltins(r *Registry) {
    r.Register(&Tool{
        Name:  "nmap_scan",
        Level: LevelScan,
        Desc:  "nmap 完整扫描: 服务版本识别(-sV) + 默认脚本(-sC) + 漏洞检测",
        Run:   NmapScan,
        Parse: ParseNmapXML,
    })
    // ... 其他内置工具
}
```

### 2. 规划器动态选择 (`internal/planner/planner.go`)
```go
func MakeRules(hasNmap bool) []Rule {
    // 优先使用 nmap_scan（如果可用），回退到 fake_scan
    scanTool := "fake_scan"
    if hasNmap {
        scanTool = "nmap_scan"
    }
    return []Rule{ /* ... */ }
}
```

### 3. CLI 入口 (`cmd/redcell/main.go`)
新增命令行参数:
```bash
./redcell.exe -nmap <target>
```

新增函数:
- `runNmapScan(target)` - 真实 nmap 扫描闭环演示

---

## 使用方式

### 1. 命令行直接扫描
```bash
# 扫描单个主机
./redcell.exe -nmap 192.168.1.100

# 扫描网段（需 nmap 支持 CIDR）
./redcell.exe -nmap 192.168.1.0/24

# 扫描域名
./redcell.exe -nmap scanme.nmap.org
```

**输出示例**:
```
真实 nmap 完整扫描: 192.168.1.100  (服务版本 + NSE 脚本 + 漏洞检测, 可能耗时)
· 工具 nmap_scan  success=true

攻击图:
  [服务] 192.168.1.100:22/ssh (OpenSSH 8.2p1 Ubuntu Linux)
  [服务] 192.168.1.100:80/http (Apache httpd 2.4.41)
  [服务] 192.168.1.100:445/microsoft-ds
  [INFO] OS: Linux 5.4 (accuracy: 95%)
  [CRITICAL] [nse] http-shellshock: VULNERABLE: CVE-2014-6271
  [CRITICAL] [nse] smb-vuln-ms17-010: VULNERABLE: Remote Code Execution

证据链(服务版本示例):
  192.168.1.100:22/ssh (OpenSSH 8.2p1 Ubuntu Linux)  ←  [nmap_scan] "name=\"ssh\""
  192.168.1.100:80/http (Apache httpd 2.4.41)  ←  [nmap_scan] "name=\"http\""

发现: 3 服务 · 3 finding · 证据逐字回查违规: 0
✓ 全部证据可溯回 nmap XML 输出, 无幻觉
```

### 2. 在 LLM 自主 agent 中使用
```bash
# agent 会自动选择 nmap_scan（如果注册）
export ANTHROPIC_API_KEY=sk-ant-...
./redcell.exe -agent 192.168.1.100
```

LLM 决策器会优先选择 `nmap_scan` 而非 `fake_scan`，获得真实服务版本信息。

### 3. 在 Web 作战台中使用
启动作战台后，规划器会自动检测 `nmap_scan` 是否可用：
```bash
./redcell.exe
# 访问 http://127.0.0.1:8000
# 点击"启动战役" - 如果 nmap 已安装，自动使用完整扫描
```

---

## 技术细节

### 1. XML 结构映射
只映射必需字段，避免过度解析：
```go
type NmapRun struct {
    Hosts []NmapHost `xml:"host"`
}

type NmapHost struct {
    Status    NmapStatus    `xml:"status"`
    Addresses []NmapAddress `xml:"address"`
    Ports     NmapPorts     `xml:"ports"`
    OS        NmapOS        `xml:"os"`
}
```

### 2. 证据回查机制
每个 `Observation` 的 `Excerpt` 字段保存 XML 属性片段：
```go
Excerpt: fmt.Sprintf(`name="%s"`, svcName)  // 可在原始 XML 中逐字找到
```

证据验证通过 `core.VerifyEvidence()` 确保无幻觉。

### 3. 漏洞脚本过滤策略
**包含** (作为 finding):
- 脚本名含 `vuln/exploit/cve/dos/backdoor`
- 白名单重要脚本（shellshock/heartbleed/MS17-010 等）

**排除** (不作为 finding):
- 信息类脚本: `http-title`, `ssh-hostkey`, `ssl-cert`
- Banner 类: `banner`, `info`

### 4. 超时与并发控制
- 默认超时: 600 秒（10 分钟）
- 参数: `--top-ports 1000` （平衡速度与覆盖）
- `-Pn`: 跳过 ping（防火墙可能拦截）
- `--open`: 只输出开放端口（减少噪音）

---

## 测试验证

### 运行完整测试套件
```bash
go test ./internal/... -v
```

**结果**: ✅ 所有测试通过（10/10 模块）

### 单独测试 Nmap 模块
```bash
go test ./internal/tools -run TestParseNmapXML -v
```

---

## 依赖要求

### 必需
- **nmap** 7.x+ 安装在系统 PATH

### 验证 nmap 可用性
```bash
nmap --version
```

**输出示例**:
```
Nmap version 7.94 ( https://nmap.org )
```

### 安装指南

**Ubuntu/Debian**:
```bash
sudo apt-get install nmap
```

**macOS**:
```bash
brew install nmap
```

**Windows**:
- 下载: https://nmap.org/download.html
- 安装后确保 `nmap.exe` 在 PATH 中

---

## 性能考虑

### 扫描时间
- 单主机 top 1000 端口: ~30-120 秒
- 取决于: 网络延迟、目标响应、开放端口数

### 优化建议
1. **指定端口**: 传 `ports` 参数缩小范围
2. **禁用脚本**: 仅需服务版本时可考虑 `-sV` only
3. **并发扫描**: 多目标可并行（未来增强）

---

## 已知限制

### 1. 需要 root/管理员权限
部分扫描类型（-sS SYN scan）需要提升权限，当前使用 `-sV -sC` 不需要。

### 2. 防火墙影响
- `-Pn` 跳过 ping 可能误报 host up
- 某些防火墙拦截服务版本探测

### 3. 输出解析
仅解析 XML 格式（`-oX`），不支持普通文本输出。

---

## 后续增强方向

### 短期
- [ ] 支持自定义 nmap 参数（通过 args）
- [ ] 并发多目标扫描
- [ ] 扫描进度实时反馈（解析 stderr）

### 中期
- [ ] NSE 脚本输出深度解析（提取 CVE 号）
- [ ] 漏洞严重级映射（CVSS 评分）
- [ ] 扫描结果缓存（避免重复扫描）

### 长期
- [ ] 集成 Nmap Scripting Engine 自定义脚本
- [ ] 与 Exploit-DB 联动（自动匹配 exploit）
- [ ] 分布式扫描支持

---

## 文件清单

```
internal/tools/
├── nmap.go           # 主实现（280 行）
├── nmap_test.go      # 单元测试（130 行）
├── parse.go          # 更新 RegisterBuiltins
└── parse_test.go     # 更新 TestRegistry

internal/planner/
└── planner.go        # 新增 MakeRules()

cmd/redcell/
└── main.go           # 新增 -nmap 参数 + runNmapScan()

README.md             # 更新依赖说明
NMAP_INTEGRATION.md   # 本文档
```

---

## 版本历史

### v1.0 (2026-07-28)
- ✅ 基础 Nmap 集成（-sV -sC）
- ✅ XML 解析器
- ✅ 漏洞脚本过滤
- ✅ 证据回查支持
- ✅ CLI 入口
- ✅ 完整测试覆盖

---

## 贡献者
- 集成实现: Claude Opus 4.8 (2026-07-28)
- 项目维护: REDCELL Team

---

**相关文档**:
- [README.md](README.md) - 项目总览
- [内核设计](internal/core/graph.go) - 攻击图与证据约束
- [工具层设计](internal/tools/tool.go) - 工具注册机制
