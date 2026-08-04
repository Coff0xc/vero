# 平台专用工具包实现报告

## 新增功能

### 1. 平台检测机制
**文件**: `internal/tools/platform.go`
- `GetPlatform()` - 检测当前运行平台 (windows/linux/darwin)
- `IsPlatformCompatible()` - 检查工具是否与当前平台兼容

### 2. Tool结构增强
**修改**: `internal/tools/tool.go`
- 新增 `Platform` 字段 - 工具支持的平台标识
- Registry自动过滤不兼容平台的工具
- 默认值 "all" 表示跨平台工具

### 3. Windows专用工具包
**文件**: `internal/scenarios/windows.go`
**10个工具**:
1. `powershell_enum` - 系统信息枚举 (用户/组/进程/服务)
2. `defender_check` - Windows Defender 状态检测
3. `amsi_bypass` - AMSI 绕过检测
4. `mimikatz` - 凭证提取 (需单独安装)
5. `registry_persist` - 注册表持久化检测
6. `wmi_query` - WMI 查询
7. `seatbelt` - 全面安全审计 (需单独安装)
8. `sharphound` - 域环境收集 (需单独安装)
9. `netsh_portproxy` - 端口转发设置
10. `uac_check` - UAC 配置检测

### 4. Linux专用工具包
**文件**: `internal/scenarios/linux.go`
**12个工具**:
1. `linpeas` - 提权路径枚举 (需下载)
2. `pspy` - 无root进程监控 (需下载)
3. `linux_exploit_suggester` - 内核漏洞检测
4. `find_suid` - SUID/SGID 二进制查找
5. `search_sensitive` - 敏感文件搜索 (.ssh/keys/.env)
6. `sudo_check` - sudo 配置检测
7. `cron_enum` - cron 任务枚举
8. `container_escape_check` - 容器逃逸检测
9. `tcpdump_capture` - 网络流量捕获 (需root)
10. `ssh_key_harvest` - SSH 私钥收集
11. `history_analysis` - 命令历史分析
12. `writable_paths` - 可写路径检测

---

## 工作原理

### 平台自动过滤
```go
// Registry.Register() 自动过滤
func (r *Registry) Register(t *Tool) {
    if t.Platform == "" {
        t.Platform = "all"
    }
    
    // 只注册兼容当前平台的工具
    if t.Platform != "all" && t.Platform != r.runtime {
        return // 跳过
    }
    
    r.tools[t.Name] = t
}
```

### 使用示例

**Windows环境**:
```go
reg := tools.NewRegistry() // runtime = "windows"
scenarios.RegisterDefaults(m, reg)

// 只注册 Windows 兼容工具:
// - Windows专用工具 (Platform: "windows")
// - 跨平台工具 (Platform: "all")
// - Linux工具被自动跳过
```

**Linux环境**:
```go
reg := tools.NewRegistry() // runtime = "linux"
scenarios.RegisterDefaults(m, reg)

// 只注册 Linux 兼容工具:
// - Linux专用工具 (Platform: "linux")
// - 跨平台工具 (Platform: "all")
// - Windows工具被自动跳过
```

---

## 工具统计

### 总工具数
- **Windows专用**: 10个
- **Linux专用**: 12个
- **跨平台工具**: 原有工具 (约30个)
- **总计新增**: 22个平台专用工具

### 按杀伤力分级
**Windows**:
- Recon (侦察): 6个
- Cred (凭证): 1个
- Exploit (利用): 3个

**Linux**:
- Recon (侦察): 9个
- Scan (扫描): 1个
- Cred (凭证): 1个
- Exploit (利用): 1个

---

## 与现有系统集成

### 1. 自动注册
```go
// internal/scenarios/scenarios.go
func RegisterDefaults(m *Manager, reg *tools.Registry) {
    // ... 原有包 ...
    
    // 平台专用工具包
    m.Register(reg, WindowsToolsPack())  // 仅Windows注册
    m.Register(reg, LinuxToolsPack())    // 仅Linux注册
}
```

### 2. 工具可见性
- LLM只能看到当前平台兼容的工具
- Windows上看不到Linux工具，反之亦然
- 避免LLM选择不可用的工具

### 3. 依赖检测
- Windows工具检测PowerShell可用性
- Linux工具检测bash/shell可用性
- 外部工具提示下载/安装路径

---

## 安全设计

### 1. 授权检查
- 危险操作提示手动执行 (Mimikatz/tcpdump)
- 不自动执行破坏性命令
- 仅提供检测和建议

### 2. 证据收集
- 所有工具提供 Parse 函数
- 提取结构化 Observation
- 记录 Severity 和 MITRE ATT&CK 映射

### 3. 错误处理
- 工具不存在时返回安装提示
- 权限不足时返回提权建议
- 清晰的错误消息

---

## 使用场景

### Windows渗透测试
1. `powershell_enum` - 初始侦察
2. `defender_check` - 防御检测
3. `registry_persist` - 持久化位置
4. `uac_check` - 权限提升路径
5. `mimikatz` - 凭证提取

### Linux渗透测试
1. `find_suid` - SUID提权
2. `search_sensitive` - 敏感文件
3. `sudo_check` - sudo配置
4. `cron_enum` - cron劫持
5. `ssh_key_harvest` - SSH密钥

---

## 代码统计

| 文件 | 行数 | 功能 |
|------|------|------|
| `tools/platform.go` | 15 | 平台检测 |
| `tools/tool.go` | +15 | 平台字段+过滤 |
| `scenarios/windows.go` | 220 | Windows工具包 |
| `scenarios/linux.go` | 330 | Linux工具包 |
| **总计** | **580行** | **平台专用功能** |

---

## 测试验证

### Windows测试
```powershell
# 编译
go build ./cmd/vero

# 检查注册的工具
./vero.exe -list-tools | findstr "powershell|defender|mimikatz"

# 启动战役
./vero.exe -target http://localhost:3000
```

### Linux测试
```bash
# 编译
go build ./cmd/vero

# 检查注册的工具
./vero -list-tools | grep "linpeas\|suid\|ssh_key"

# 启动战役
./vero -target http://localhost:3000
```

---

## 未来扩展

### macOS工具包
- 添加 `Platform: "darwin"` 工具
- Keychain凭证提取
- SIP/Gatekeeper检测
- LaunchAgent/LaunchDaemon枚举

### 跨平台工具增强
- 自动适配不同平台命令格式
- 统一输出解析
- 智能工具选择

---

## 兼容性

- ✅ 向后兼容：原有工具默认 `Platform: "all"`
- ✅ 渐进式：可逐步添加平台标识
- ✅ 透明过滤：Registry自动处理，无需修改调用代码
- ✅ 灵活扩展：支持添加更多平台 (freebsd/openbsd等)

---

**提交**: 新增平台专用工具包，Windows 10个工具，Linux 12个工具  
**影响**: 工具总数增加22个，平台隔离避免不兼容问题  
**测试**: 编译通过，平台过滤机制正常工作
