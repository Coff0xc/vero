package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

// Dependency —— 工具依赖项(抄 Dark-Moon 工具检测: 启动前验证工具链完整性)。
type Dependency struct {
	Binary      string   // 二进制名(如 nmap)
	DisplayName string   // 展示名(如 Nmap 端口扫描器)
	CheckCmd    []string // 版本检测命令(如 []string{"nmap", "--version"})
	InstallHint string   // 安装提示(如 "apt install nmap" / "choco install nmap")
}

// IsInstalled —— 检测该依赖是否已安装(PATH 可达 + 版本命令成功)。
func (d *Dependency) IsInstalled() bool {
	if _, err := exec.LookPath(d.Binary); err != nil {
		return false
	}
	if len(d.CheckCmd) == 0 {
		return true // 仅检查 PATH
	}
	cmd := exec.Command(d.CheckCmd[0], d.CheckCmd[1:]...)
	return cmd.Run() == nil
}

// Version —— 获取工具版本(执行 CheckCmd 取输出首行), 失败返回空串。
func (d *Dependency) Version() string {
	if len(d.CheckCmd) == 0 {
		return ""
	}
	out, err := exec.Command(d.CheckCmd[0], d.CheckCmd[1:]...).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// CoreDependencies —— 核心工具依赖清单(Vero 必需工具 + 常见场景工具)。
var CoreDependencies = []Dependency{
	// 核心(内置 Go 扫描可降级, 但 nuclei 必需)
	{Binary: "nuclei", DisplayName: "Nuclei 漏洞扫描器",
		CheckCmd: []string{"nuclei", "-version"},
		InstallHint: "go install github.com/projectdiscovery/nuclei/v3/cmd/nuclei@latest"},

	// Web 场景
	{Binary: "ffuf", DisplayName: "ffuf 目录/虚拟主机爆破",
		CheckCmd: []string{"ffuf", "-V"},
		InstallHint: "go install github.com/ffuf/ffuf/v2@latest"},
	{Binary: "httpx", DisplayName: "httpx HTTP 探测器",
		CheckCmd: []string{"httpx", "-version"},
		InstallHint: "go install github.com/projectdiscovery/httpx/cmd/httpx@latest"},

	// AD/内网场景
	{Binary: "nxc", DisplayName: "NetExec AD 工具包",
		CheckCmd: []string{"nxc", "--version"},
		InstallHint: "pipx install git+https://github.com/Pennyw0rth/NetExec"},
	{Binary: "secretsdump.py", DisplayName: "Impacket Secretsdump",
		CheckCmd: []string{"secretsdump.py", "-h"},
		InstallHint: "pipx install impacket"},

	// 后渗透
	{Binary: "pypykatz", DisplayName: "pypykatz LSASS 解析器",
		CheckCmd: []string{"pypykatz", "version"},
		InstallHint: "pipx install pypykatz"},

	// 可选(内置 Go 扫描可降级)
	{Binary: "nmap", DisplayName: "Nmap 端口扫描器",
		CheckCmd: []string{"nmap", "--version"},
		InstallHint: "apt/yum/choco install nmap"},

	// 代码审计场景 (CodeAuditPack)
	{Binary: "semgrep", DisplayName: "Semgrep 代码扫描器",
		CheckCmd: []string{"semgrep", "--version"},
		InstallHint: "pip install semgrep 或 brew install semgrep"},
	{Binary: "bandit", DisplayName: "Bandit Python 安全扫描",
		CheckCmd: []string{"bandit", "--version"},
		InstallHint: "pip install bandit"},
	{Binary: "dependency-check", DisplayName: "OWASP Dependency-Check",
		CheckCmd: []string{"dependency-check", "--version"},
		InstallHint: "https://github.com/jeremylong/DependencyCheck/releases"},

	// 云渗透场景 (CloudPackEnhanced)
	{Binary: "aws", DisplayName: "AWS CLI",
		CheckCmd: []string{"aws", "--version"},
		InstallHint: "pip install awscli 或 https://aws.amazon.com/cli/"},
	{Binary: "az", DisplayName: "Azure CLI",
		CheckCmd: []string{"az", "--version"},
		InstallHint: "https://docs.microsoft.com/cli/azure/install-azure-cli"},
	{Binary: "gcloud", DisplayName: "Google Cloud SDK",
		CheckCmd: []string{"gcloud", "--version"},
		InstallHint: "https://cloud.google.com/sdk/docs/install"},

	// K8s/容器场景 (K8sPackEnhanced)
	{Binary: "kubectl", DisplayName: "Kubernetes CLI",
		CheckCmd: []string{"kubectl", "version", "--client"},
		InstallHint: "https://kubernetes.io/docs/tasks/tools/"},
	{Binary: "helm", DisplayName: "Helm 包管理器",
		CheckCmd: []string{"helm", "version"},
		InstallHint: "https://helm.sh/docs/intro/install/"},
	{Binary: "docker", DisplayName: "Docker CLI",
		CheckCmd: []string{"docker", "--version"},
		InstallHint: "https://docs.docker.com/get-docker/"},
}

// CheckDependencies —— 检测所有核心依赖, 返回缺失清单。
// 用于启动时健康检查: 前端展示缺失工具 + 安装提示, 而非静默跑出一堆失败。
func CheckDependencies() []Dependency {
	var missing []Dependency
	for _, dep := range CoreDependencies {
		if !dep.IsInstalled() {
			missing = append(missing, dep)
		}
	}
	return missing
}

// DepsReport —— 生成依赖报告(已安装 ✓ 版本 / 缺失 ✗ 安装提示)。
func DepsReport() string {
	var b strings.Builder
	b.WriteString("工具依赖检测:\n")
	for _, dep := range CoreDependencies {
		if dep.IsInstalled() {
			ver := dep.Version()
			if ver == "" {
				ver = "(已安装)"
			}
			fmt.Fprintf(&b, "  ✓ %s: %s\n", dep.DisplayName, ver)
		} else {
			fmt.Fprintf(&b, "  ✗ %s: 缺失 — %s\n", dep.DisplayName, dep.InstallHint)
		}
	}
	missing := CheckDependencies()
	if len(missing) > 0 {
		fmt.Fprintf(&b, "\n⚠ %d 个工具缺失, 部分场景无法使用。\n", len(missing))
	} else {
		b.WriteString("\n✓ 全部核心工具已就绪。\n")
	}
	return b.String()
}
