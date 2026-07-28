package tools

import (
	"strings"
	"testing"
)

// TestParseNmapXML —— 验证 XML parser 提取 host/service/OS/NSE 脚本。
func TestParseNmapXML(t *testing.T) {
	// 真实 nmap -oX 输出样本(简化)
	xml := `<?xml version="1.0"?>
<nmaprun scanner="nmap" version="7.94">
  <host>
    <status state="up"/>
    <address addr="192.168.1.100" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenSSH" version="8.2p1" extrainfo="Ubuntu Linux"/>
      </port>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http" product="Apache httpd" version="2.4.41"/>
        <script id="http-shellshock" output="VULNERABLE: CVE-2014-6271"/>
        <script id="http-title" output="Welcome"/>
      </port>
      <port protocol="tcp" portid="445">
        <state state="open"/>
        <service name="microsoft-ds"/>
        <script id="smb-vuln-ms17-010" output="VULNERABLE: Remote Code Execution vulnerability in SMB"/>
      </port>
    </ports>
    <os>
      <osmatch name="Linux 5.4" accuracy="95"/>
    </os>
  </host>
</nmaprun>`

	obs := ParseNmapXML(xml, map[string]any{"target": "192.168.1.100"})

	// 验证: 1 host + 3 service + 1 OS finding + 2 vuln scripts = 7
	if len(obs) < 6 {
		t.Fatalf("期望至少 6 个观测, 实际 %d", len(obs))
	}

	// 验证 host 节点
	hasHost := false
	for _, o := range obs {
		if o.Kind == "host" && o.Key == "192.168.1.100" {
			hasHost = true
			if !strings.Contains(o.Excerpt, "192.168.1.100") {
				t.Errorf("host excerpt 应包含 IP, 实际: %s", o.Excerpt)
			}
		}
	}
	if !hasHost {
		t.Error("缺失 host 节点")
	}

	// 验证 service 节点(含版本信息)
	hasSSH := false
	for _, o := range obs {
		if o.Kind == "service" && o.Key == "192.168.1.100:22" {
			hasSSH = true
			if !strings.Contains(o.Label, "OpenSSH") || !strings.Contains(o.Label, "8.2p1") {
				t.Errorf("SSH service label 应含版本信息, 实际: %s", o.Label)
			}
		}
	}
	if !hasSSH {
		t.Error("缺失 SSH service 节点")
	}

	// 验证 OS 指纹 finding
	hasOS := false
	for _, o := range obs {
		if o.Kind == "finding" && strings.Contains(o.Key, ":os:") {
			hasOS = true
			if !strings.Contains(o.Label, "Linux 5.4") {
				t.Errorf("OS finding label 应含系统名, 实际: %s", o.Label)
			}
		}
	}
	if !hasOS {
		t.Error("缺失 OS finding")
	}

	// 验证 NSE 漏洞脚本 finding
	hasShellshock := false
	hasMS17 := false
	for _, o := range obs {
		if o.Kind == "finding" && strings.Contains(o.Key, ":script:") {
			if strings.Contains(o.Key, "http-shellshock") {
				hasShellshock = true
				if !strings.Contains(o.Label, "shellshock") {
					t.Errorf("shellshock finding label 应含脚本信息, 实际: %s", o.Label)
				}
			}
			if strings.Contains(o.Key, "smb-vuln-ms17-010") {
				hasMS17 = true
			}
			// http-title 非漏洞脚本, 应被过滤
			if strings.Contains(o.Key, "http-title") {
				t.Error("非漏洞脚本 http-title 不应作为 finding")
			}
		}
	}
	if !hasShellshock {
		t.Error("缺失 shellshock 漏洞 finding")
	}
	if !hasMS17 {
		t.Error("缺失 MS17-010 漏洞 finding")
	}
}

// TestParseNmapXMLEmpty —— 空输出不应 panic。
func TestParseNmapXMLEmpty(t *testing.T) {
	obs := ParseNmapXML("", map[string]any{})
	if obs != nil {
		t.Errorf("空输入应返回 nil, 实际: %v", obs)
	}
}

// TestParseNmapXMLHostDown —— host down 不应产生观测。
func TestParseNmapXMLHostDown(t *testing.T) {
	xml := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <status state="down"/>
    <address addr="10.0.0.1" addrtype="ipv4"/>
  </host>
</nmaprun>`

	obs := ParseNmapXML(xml, map[string]any{})
	if len(obs) != 0 {
		t.Errorf("host down 不应产生观测, 实际: %d", len(obs))
	}
}

// TestIsVulnScript —— 验证漏洞脚本识别逻辑。
func TestIsVulnScript(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"http-shellshock", true},
		{"smb-vuln-ms17-010", true},
		{"ssl-heartbleed", true},
		{"http-vuln-cve2021-12345", true},
		{"ftp-anon", true},
		{"http-title", false},
		{"ssh-hostkey", false},
		{"ssl-cert", false},
	}

	for _, c := range cases {
		got := isVulnScript(c.id)
		if got != c.want {
			t.Errorf("isVulnScript(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

// TestNmapScanMissingTarget —— 缺 target 应返回错误。
func TestNmapScanMissingTarget(t *testing.T) {
	res := NmapScan(map[string]any{})
	if res.Success {
		t.Error("缺 target 应失败")
	}
	if !strings.Contains(res.Stderr, "缺 target") {
		t.Errorf("错误信息应含 '缺 target', 实际: %s", res.Stderr)
	}
}
