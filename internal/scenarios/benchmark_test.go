package scenarios

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// BenchmarkParserPerformance —— 测试 Parser 性能。
func BenchmarkParserPerformance(b *testing.B) {
	mockAWSOutput := `AWS IMDS Enumeration:
  instance-id: i-test123
  hostname: ip-172-31-10-20.ec2.internal
  local-ipv4: 172.31.10.20
  public-ipv4: 54.123.45.67
  iam/security-credentials/: MyEC2Role
  IAM Credentials (MyEC2Role):
{
  "AccessKeyId": "ASIATESTACCESSKEY123",
  "SecretAccessKey": "testsecretkey1234567890",
  "Token": "testtoken..."
}`

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	awsTool, _ := reg.Get("aws_imds_enum")
	args := map[string]any{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = awsTool.Parse(mockAWSOutput, args)
	}
}

func BenchmarkParseNXCCreds(b *testing.B) {
	output := `SMB         10.0.1.50       445    DC01    [+] ACME\admin:P@ssw0rd (Pwn3d!)
SMB         10.0.1.51       445    WEB01   [+] ACME\backup:backup123
SMB         10.0.1.52       445    SQL01   [+] ACME\service:svc123`

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	nxcTool, _ := reg.Get("nxc_smb_spray")
	args := map[string]any{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = nxcTool.Parse(output, args)
	}
}

func BenchmarkParseFFUF(b *testing.B) {
	jsonOutput := `{"results":[
{"input":{"FUZZ":"admin"},"position":1,"status":200,"length":4521,"words":342,"lines":87,"content-type":"text/html","redirectlocation":"","url":"http://example.com/admin","resultfile":""},
{"input":{"FUZZ":"login"},"position":2,"status":200,"length":3210,"words":234,"lines":65,"content-type":"text/html","redirectlocation":"","url":"http://example.com/login","resultfile":""},
{"input":{"FUZZ":"backup"},"position":3,"status":200,"length":1024,"words":89,"lines":23,"content-type":"application/x-tar","redirectlocation":"","url":"http://example.com/backup","resultfile":""}
]}`

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	ffufTool, _ := reg.Get("ffuf_dir_brute")
	args := map[string]any{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ffufTool.Parse(jsonOutput, args)
	}
}

func BenchmarkParseDockerEscape(b *testing.B) {
	stdout := `Docker Escape Check:
  [+] Running in Docker container
  Capabilities: CapEff:	0000003fffffffff
  [!] PRIVILEGED CONTAINER DETECTED
  [!] Docker socket mounted at /var/run/docker.sock
      -> Can control host Docker daemon
  [!] Host filesystem mounted (possible at /host or /rootfs)
  [+] /proc is mounted
  [-] AppArmor not active`

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	dockerTool, _ := reg.Get("docker_escape_check")
	args := map[string]any{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dockerTool.Parse(stdout, args)
	}
}

func BenchmarkParseMSFSearch(b *testing.B) {
	output := `MSF Search Results:
exploit/windows/smb/ms17_010_eternalblue - MS17-010 EternalBlue SMB Remote Windows Kernel Pool Corruption
exploit/windows/smb/ms08_067_netapi - MS08-067 Microsoft Server Service Relative Path Stack Corruption
exploit/windows/local/ms16_032_secondary_logon_handle_privesc - MS16-032 Secondary Logon Handle Privilege Escalation`

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	msfTool, _ := reg.Get("msf_search")
	args := map[string]any{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = msfTool.Parse(output, args)
	}
}

func BenchmarkParseK8sServiceAccount(b *testing.B) {
	stdout := `Kubernetes ServiceAccount Enumeration:
  [+] ServiceAccount found at /var/run/secrets/kubernetes.io/serviceaccount
  [!] Token: eyJhbGciOiJSUzI1NiIsImtpZCI6IkR...
  Namespace: default
  [+] CA certificate available

  Trying to access K8s API at https://kubernetes.default.svc...
  [!] API accessible - can list namespaces
{"kind":"NamespaceList","apiVersion":"v1","items":[...]}`

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	k8sTool, _ := reg.Get("k8s_sa_enum")
	args := map[string]any{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = k8sTool.Parse(stdout, args)
	}
}

// BenchmarkScenarioPackRouting —— 测试场景包路由性能。
func BenchmarkScenarioPackRouting(b *testing.B) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	services := map[string]bool{
		"http":         true,
		"https":        true,
		"microsoft-ds": true,
		"ldap":         true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.Route(services)
	}
}

// BenchmarkToolRegistryLookup —— 测试工具查找性能。
func BenchmarkToolRegistryLookup(b *testing.B) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	toolNames := []string{
		"http_probe", "web_vuln_scan", "nxc_smb_spray",
		"msf_search", "aws_imds_enum", "docker_escape_check",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range toolNames {
			_, _ = reg.Get(name)
		}
	}
}
