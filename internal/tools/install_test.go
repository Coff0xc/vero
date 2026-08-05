package tools

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestExtractSkipsDirEntries —— D14/D15: zip 里目录条目名含 inner 时,
// 必须跳过目录提取真正的文件, 不得产出空文件/报 "device does not exist"。
func TestExtractSkipsDirEntries(t *testing.T) {
	// 构造 zip: 目录条目(nuclei_3.3.1_linux_amd64/)在前, 真二进制在后。
	tmp, err := os.CreateTemp(t.TempDir(), "art-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(tmp)
	w, err := z.Create("nuclei_3.3.1_linux_amd64/")
	if err != nil {
		t.Fatal(err)
	}
	w, err = z.Create("nuclei_3.3.1_linux_amd64/nuclei")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("#!/bin/sh\necho nuclei-ok\n")); err != nil {
		t.Fatal(err)
	}
	z.Close()
	tmp.Close()

	target := filepath.Join(t.TempDir(), "nuclei")
	err = extract(tmp.Name(), &toolArtifact{Kind: "zip", InnerRe: "nuclei"}, target)
	if err != nil {
		t.Fatalf("extract 应跳过目录条目成功: %v", err)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("产物缺失: %v", err)
	}
	if !strings.Contains(string(b), "nuclei-ok") {
		t.Errorf("产物内容应为真实文件而非空文件, got %q", b)
	}
}
