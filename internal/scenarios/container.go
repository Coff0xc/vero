package scenarios

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- 容器逃逸场景包 (Container Escape) ----------

// dockerEscapeCheck —— 检测容器逃逸向量。
// 检查: 特权容器、危险 capabilities、Docker socket 挂载、宿主机 procfs 挂载等。
func dockerEscapeCheck(args map[string]any) tools.ToolResult {
	var output strings.Builder
	output.WriteString("Docker Escape Check:\n")

	// 1. 检测是否在容器内
	if _, err := os.Stat("/.dockerenv"); err == nil {
		output.WriteString("  [+] Running in Docker container\n")
	} else {
		output.WriteString("  [-] Not in Docker container\n")
		return tools.ToolResult{Success: false, Stderr: "Not in container", RC: 1}
	}

	// 2. 检测特权容器 (通过 /proc/self/status CapEff)
	capResult := tools.Sh([]string{"grep", "CapEff", "/proc/self/status"}, 5*time.Second)
	if capResult.Success {
		output.WriteString(fmt.Sprintf("  Capabilities: %s", capResult.Stdout))
		// CapEff: 0000003fffffffff 表示所有 capabilities (特权容器)
		if strings.Contains(capResult.Stdout, "0000003fffffffff") || strings.Contains(capResult.Stdout, "0000001fffffffff") {
			output.WriteString("  [!] PRIVILEGED CONTAINER DETECTED\n")
		}
	}

	// 3. 检测 Docker socket 挂载
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		output.WriteString("  [!] Docker socket mounted at /var/run/docker.sock\n")
		output.WriteString("      -> Can control host Docker daemon\n")
	}

	// 4. 检测宿主机文件系统挂载
	mountResult := tools.Sh([]string{"mount"}, 5*time.Second)
	if mountResult.Success {
		// 检测宿主机根目录挂载
		if strings.Contains(mountResult.Stdout, "/host") || strings.Contains(mountResult.Stdout, "/rootfs") {
			output.WriteString("  [!] Host filesystem mounted (possible at /host or /rootfs)\n")
		}
		// 检测 /proc 挂载 (宿主机 procfs)
		if strings.Contains(mountResult.Stdout, "proc on /proc") {
			output.WriteString("  [+] /proc is mounted\n")
		}
	}

	// 5. 检测 AppArmor/SELinux 状态
	if _, err := os.Stat("/sys/kernel/security/apparmor"); err == nil {
		output.WriteString("  [+] AppArmor enabled\n")
	} else {
		output.WriteString("  [-] AppArmor not active\n")
	}

	// 6. 检测 cgroup 逃逸向量 (CVE-2022-0492)
	cgroupResult := tools.Sh([]string{"cat", "/proc/self/cgroup"}, 5*time.Second)
	if cgroupResult.Success {
		output.WriteString(fmt.Sprintf("  cgroup: %s", strings.Split(cgroupResult.Stdout, "\n")[0]))
	}

	return tools.ToolResult{Success: true, Stdout: output.String()}
}

// k8sServiceAccountEnum —— 提取 Kubernetes ServiceAccount token。
// 默认挂载在 /var/run/secrets/kubernetes.io/serviceaccount/
func k8sServiceAccountEnum(args map[string]any) tools.ToolResult {
	saPath := "/var/run/secrets/kubernetes.io/serviceaccount"
	tokenPath := saPath + "/token"
	caPath := saPath + "/ca.crt"
	nsPath := saPath + "/namespace"

	var output strings.Builder
	output.WriteString("Kubernetes ServiceAccount Enumeration:\n")

	// 检测 ServiceAccount 挂载
	if _, err := os.Stat(saPath); err != nil {
		output.WriteString("  [-] No ServiceAccount mounted\n")
		return tools.ToolResult{Success: false, Stderr: "Not in K8s pod or SA not mounted", RC: 1}
	}

	output.WriteString(fmt.Sprintf("  [+] ServiceAccount found at %s\n", saPath))

	// 读取 token
	if tokenData, err := os.ReadFile(tokenPath); err == nil {
		tokenPreview := string(tokenData)
		if len(tokenPreview) > 50 {
			tokenPreview = tokenPreview[:50] + "..."
		}
		output.WriteString(fmt.Sprintf("  [!] Token: %s\n", tokenPreview))
	}

	// 读取 namespace
	if nsData, err := os.ReadFile(nsPath); err == nil {
		output.WriteString(fmt.Sprintf("  Namespace: %s\n", string(nsData)))
	}

	// 检测 CA cert
	if _, err := os.Stat(caPath); err == nil {
		output.WriteString("  [+] CA certificate available\n")
	}

	// 尝试访问 Kubernetes API
	apiURL := tools.ArgStr(args, "api_url", "https://kubernetes.default.svc")
	output.WriteString(fmt.Sprintf("\n  Trying to access K8s API at %s...\n", apiURL))

	tokenData, _ := os.ReadFile(tokenPath)
	apiResult := tools.Sh([]string{
		"curl", "-s", "-k",
		"-H", fmt.Sprintf("Authorization: Bearer %s", string(tokenData)),
		apiURL + "/api/v1/namespaces",
	}, 10*time.Second)

	if apiResult.Success {
		if strings.Contains(apiResult.Stdout, "\"kind\":\"NamespaceList\"") {
			output.WriteString("  [!] API accessible - can list namespaces\n")
			output.WriteString(tools.Clip(apiResult.Stdout, 200) + "...\n")
		} else if strings.Contains(apiResult.Stdout, "Forbidden") {
			output.WriteString("  [+] API reachable but access forbidden\n")
		} else {
			output.WriteString("  [-] API unreachable or unexpected response\n")
		}
	}

	return tools.ToolResult{Success: true, Stdout: output.String()}
}


// ---------- Parsers ----------

// ParseDockerEscape —— 解析容器逃逸检测结果。
func ParseDockerEscape(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	// 检测特权容器
	if strings.Contains(stdout, "PRIVILEGED CONTAINER") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "container:privileged",
			Label:   "[critical] Running in privileged container - full host access",
			Excerpt: "PRIVILEGED CONTAINER DETECTED",
		})
	}

	// 检测 Docker socket 挂载
	if strings.Contains(stdout, "Docker socket mounted") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "container:docker_socket",
			Label:   "[critical] Docker socket mounted - can control host daemon",
			Excerpt: "Docker socket mounted at /var/run/docker.sock",
		})
	}

	// 检测宿主机文件系统挂载
	if strings.Contains(stdout, "Host filesystem mounted") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "container:host_fs",
			Label:   "[high] Host filesystem mounted - potential escape",
			Excerpt: "Host filesystem mounted",
		})
	}

	return obs
}

// ParseK8sServiceAccount —— 解析 K8s ServiceAccount token。
func ParseK8sServiceAccount(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	// 检测 ServiceAccount token
	if strings.Contains(stdout, "[!] Token:") {
		obs = append(obs, tools.Observation{
			Kind:    "cred",
			Key:     "k8s:serviceaccount:token",
			Label:   "[high] Kubernetes ServiceAccount token extracted",
			Excerpt: "Token:",
		})
	}

	// 检测 API 访问权限
	if strings.Contains(stdout, "can list namespaces") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "k8s:api:access",
			Label:   "[critical] K8s API accessible with ServiceAccount - can list namespaces",
			Excerpt: "can list namespaces",
		})
	}

	return obs
}


// ContainerPack —— 容器逃逸场景包。
func ContainerPack() Pack {
	return Pack{
		Name: "container",
		Tools: []*tools.Tool{
			{Name: "docker_escape_check", Level: tools.LevelScan,
				Desc: "Docker 容器逃逸检测, 检查特权容器/Docker socket/宿主机文件系统挂载等向量",
				Run: dockerEscapeCheck, Parse: ParseDockerEscape},
			{Name: "k8s_sa_enum", Level: tools.LevelCred,
				Desc: "Kubernetes ServiceAccount 提取, 获取 pod 的 SA token 并尝试访问 K8s API",
				Run: k8sServiceAccountEnum, Parse: ParseK8sServiceAccount},
			// D28: k8s_node_exploit 不再在此注册 —— K8sPackEnhanced 提供更完整的同名实现,
			// 覆盖式注册使本条目恒被覆盖(双注册语义混乱), 由 Enhanced 包唯一提供。
		},
		Fingerprint: func(s map[string]bool) bool {
			// 检测容器环境: 存在 /.dockerenv 或 /var/run/secrets/kubernetes.io
			if _, err := os.Stat("/.dockerenv"); err == nil {
				return true
			}
			if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
				return true
			}
			return false
		},
	}
}
