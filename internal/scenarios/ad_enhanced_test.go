package scenarios

import (
	"strings"
	"testing"
)

// TestParseNXCCreds —— 验证凭证提取。
func TestParseNXCCreds(t *testing.T) {
	output := `SMB         192.168.1.100    445    DC01             [*] Windows Server 2019 Build 17763 x64
SMB         192.168.1.100    445    DC01             [+] CORP\alice:Password123
SMB         192.168.1.100    445    DC01             [-] CORP\bob:wrongpass
SMB         192.168.1.100    445    DC01             [*] CORP\charlie:Summer2024`

	obs := ParseNXCCreds(output, map[string]any{"target": "192.168.1.100"})

	// 应提取 2 个成功凭证([+] 和 [*])
	if len(obs) != 2 {
		t.Fatalf("应提取 2 个凭证, 实际 %d", len(obs))
	}

	// 验证第一个凭证
	if obs[0].Kind != "cred" {
		t.Errorf("应为 cred 类型, 实际 %s", obs[0].Kind)
	}
	if obs[0].Key != "CORP\\alice" {
		t.Errorf("Key 应为 CORP\\alice, 实际 %s", obs[0].Key)
	}
	if !strings.Contains(obs[0].Label, "Password123") {
		t.Errorf("Label 应含密码, 实际 %s", obs[0].Label)
	}
	if !strings.Contains(obs[0].Excerpt, "[+] CORP\\alice:Password123") {
		t.Errorf("Excerpt 应含原始行, 实际 %s", obs[0].Excerpt)
	}

	// 验证第二个凭证
	if obs[1].Key != "CORP\\charlie" {
		t.Errorf("第二个凭证 Key 错误: %s", obs[1].Key)
	}
}

// TestParseNXCUsers —— 验证域用户提取。
func TestParseNXCUsers(t *testing.T) {
	output := `LDAP        192.168.1.100    389    DC01             [*] Username: alice                    badPwdCount: 0
LDAP        192.168.1.100    389    DC01             [*] Username: bob                      badPwdCount: 2
LDAP        192.168.1.100    389    DC01             [*] Username: alice                    badPwdCount: 0`

	obs := ParseNXCUsers(output, map[string]any{"target": "192.168.1.100"})

	// 应提取 2 个不重复用户
	if len(obs) != 2 {
		t.Fatalf("应提取 2 个用户, 实际 %d", len(obs))
	}

	// 验证用户节点
	if obs[0].Kind != "finding" {
		t.Errorf("应为 finding 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "alice") {
		t.Errorf("Label 应含用户名, 实际 %s", obs[0].Label)
	}
	if !strings.Contains(obs[0].Excerpt, "Username: alice") {
		t.Errorf("Excerpt 应含原始行, 实际 %s", obs[0].Excerpt)
	}
}

// TestParseNXCComputers —— 验证域计算机提取。
func TestParseNXCComputers(t *testing.T) {
	output := `LDAP        192.168.1.100    389    DC01             [*] Computer: DC01.corp.local       dNSHostName: DC01.corp.local
LDAP        192.168.1.100    389    DC01             [*] Computer: WS01.corp.local       dNSHostName: WS01.corp.local
LDAP        192.168.1.100    389    DC01             [*] Computer: SQL01.corp.local      dNSHostName: SQL01.corp.local`

	obs := ParseNXCComputers(output, map[string]any{})

	// 应提取 3 台主机
	if len(obs) != 3 {
		t.Fatalf("应提取 3 台主机, 实际 %d", len(obs))
	}

	// 验证主机节点
	if obs[0].Kind != "host" {
		t.Errorf("应为 host 类型, 实际 %s", obs[0].Kind)
	}
	if obs[0].Key != "DC01" {
		t.Errorf("Key 应为短主机名 DC01, 实际 %s", obs[0].Key)
	}
	if obs[0].Label != "DC01.corp.local" {
		t.Errorf("Label 应为完整 FQDN, 实际 %s", obs[0].Label)
	}
}

// TestParseNXCShares —— 验证 SMB 共享提取。
func TestParseNXCShares(t *testing.T) {
	output := `SMB         192.168.1.100    445    DC01             [*] ADMIN$          READ            Administrative share
SMB         192.168.1.100    445    DC01             [*] C$              READ,WRITE      Default share
SMB         192.168.1.100    445    DC01             [*] Public          READ            Public folder
SMB         192.168.1.100    445    DC01             [*] SYSVOL          READ,WRITE      Logon server share`

	obs := ParseNXCShares(output, map[string]any{"target": "192.168.1.100"})

	// 应提取 4 个共享
	if len(obs) != 4 {
		t.Fatalf("应提取 4 个共享, 实际 %d", len(obs))
	}

	// 验证敏感共享识别
	hasHighSeverity := false
	for _, o := range obs {
		if o.Kind != "finding" {
			t.Errorf("应为 finding 类型, 实际 %s", o.Kind)
		}
		if strings.Contains(o.Label, "[high]") {
			hasHighSeverity = true
		}
		if strings.Contains(o.Key, "ADMIN$") && !strings.Contains(o.Excerpt, "ADMIN$") {
			t.Errorf("Excerpt 应含共享名, 实际 %s", o.Excerpt)
		}
	}

	// C$ 和 SYSVOL 可写应标记为高危
	if !hasHighSeverity {
		t.Error("应至少有一个 [high] 严重级共享")
	}
}

// TestADPackEnhanced —— 验证场景包注册。
func TestADPackEnhanced(t *testing.T) {
	pack := ADPackEnhanced()

	if pack.Name != "ad_enhanced" {
		t.Errorf("包名应为 ad_enhanced, 实际 %s", pack.Name)
	}

	// 应有 8 个工具(2 原有 + 6 新增)
	if len(pack.Tools) != 8 {
		t.Fatalf("应有 8 个工具, 实际 %d", len(pack.Tools))
	}

	// 验证新工具存在
	toolNames := make(map[string]bool)
	for _, tool := range pack.Tools {
		toolNames[tool.Name] = true
	}

	required := []string{
		"nxc_smb_spray",
		"nxc_ldap_enum",
		"nxc_ldap_computers",
		"nxc_wmi_exec",
		"nxc_asrep",
		"nxc_smb_shares",
	}

	for _, name := range required {
		if !toolNames[name] {
			t.Errorf("缺失工具: %s", name)
		}
	}

	// 验证指纹函数
	services := map[string]bool{"microsoft-ds": true}
	if !pack.Fingerprint(services) {
		t.Error("SMB 服务应激活 AD 场景包")
	}
}

// TestParseNXCCredsEmpty —— 空输出不应 panic。
func TestParseNXCCredsEmpty(t *testing.T) {
	obs := ParseNXCCreds("", map[string]any{})
	if obs != nil && len(obs) != 0 {
		t.Errorf("空输入应返回空数组, 实际 %d", len(obs))
	}
}
