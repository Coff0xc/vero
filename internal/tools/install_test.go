package tools

import "testing"

// TestPipPackage —— 锁定工具名 -> pip 包名映射(设计规格 risk #7)。
func TestPipPackage(t *testing.T) {
	cases := map[string]string{
		"nxc_smb_spray": "nxc", // nxc_* 前缀族
		"nxc_ldap_enum": "nxc",
		"smb_enum":      "nxc",
		"kerberoast":    "nxc",
		"secretsdump":   "impacket",
		"sam_dump":      "impacket",
		"lsass_dump":    "pypykatz",
		// 无 pip 途径者恒为空
		"web_vuln_scan": "",
		"ffuf_dir_brute": "",
		"nmap_scan":     "",
		"port_scan":     "",
		"msf_search":    "",
		"k8s_sa_enum":   "",
		"docker_escape_check": "",
	}
	for name, want := range cases {
		if got := PipPackage(name); got != want {
			t.Errorf("PipPackage(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestInstallType —— 锁定三态安装途径判定(binary|pip|none)。
func TestInstallType(t *testing.T) {
	cases := map[string]string{
		"web_vuln_scan": "binary",
		"ffuf_dir_brute": "binary",
		"ffuf_vhost_enum": "binary",
		"secretsdump":   "pip",
		"nxc_smb_spray": "pip",
		"lsass_dump":    "pip",
		"sam_dump":      "pip",
		"nmap_scan":     "none",
		"msf_search":    "none",
		"k8s_sa_enum":   "none",
		"port_scan":     "none",
		"docker_escape_check": "none",
	}
	for name, want := range cases {
		if got := InstallType(name); got != want {
			t.Errorf("InstallType(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestInstallPipHint —— 修 bug 后 secretsdump/lsass_dump 系应出现提示(旧版恒为空)。
func TestInstallPipHint(t *testing.T) {
	cases := map[string]string{
		"secretsdump":    "pip install --user impacket",
		"sam_dump":       "pip install --user impacket",
		"lsass_dump":     "pip install --user pypykatz",
		"nxc_smb_spray":  "pip install --user nxc",
		"smb_enum":       "pip install --user nxc",
		"web_vuln_scan":  "",
		"nmap_scan":      "",
		"msf_search":     "",
		"port_scan":      "",
	}
	for name, want := range cases {
		if got := InstallPipHint(name); got != want {
			t.Errorf("InstallPipHint(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestInstallableBinary —— 仅 nuclei/ffuf 可自动下载。
func TestInstallableBinary(t *testing.T) {
	if got := InstallableBinary("web_vuln_scan"); got != "nuclei" {
		t.Errorf("web_vuln_scan = %q, want nuclei", got)
	}
	if got := InstallableBinary("ffuf_dir_brute"); got != "ffuf" {
		t.Errorf("ffuf_dir_brute = %q, want ffuf", got)
	}
	for _, name := range []string{"secretsdump", "nxc_smb_spray", "nmap_scan", "msf_search", "k8s_sa_enum"} {
		if got := InstallableBinary(name); got != "" {
			t.Errorf("InstallableBinary(%q) = %q, want empty", name, got)
		}
	}
}
