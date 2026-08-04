package scenarios

// Package k8s_enhanced —— K8s/容器渗透增强包(抄 Reaper + ThreatCanvas):
// K8s RBAC 提权 + Pod 逃逸 + ServiceAccount token 利用 + Helm 审计。
//
// 设计要点:
//   - K8s 攻击链: SA token → API 枚举 → RBAC 提权 → Pod 逃逸 → 节点控制。
//   - 容器逃逸: 特权容器/hostPath/CAP_SYS_ADMIN 检测 + 自动化利用。
//   - RBAC 分析: 权限矩阵生成 + 提权路径发现 (get secrets → cluster-admin)。
//   - Helm 审计: Chart 配置缺陷 (image:latest / privileged:true)。

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// k8sEnumPods —— K8s Pod 枚举 + ServiceAccount token 提取。
func k8sEnumPods(args map[string]any) tools.ToolResult {
	namespace := tools.ArgStr(args, "namespace", "default")

	// kubectl get pods -n <namespace> -o json
	podsCmd := []string{"kubectl", "get", "pods", "-n", namespace, "-o", "json"}
	podsRes := tools.Sh(podsCmd, 30*time.Second)
	if !podsRes.Success {
		return podsRes
	}

	var podList struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				ServiceAccountName string `json:"serviceAccountName"`
				Containers         []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(podsRes.Stdout), &podList); err != nil {
		return tools.ToolResult{Success: false, Stderr: "解析 Pod JSON 失败: " + err.Error(), RC: -1}
	}

	var findings []string
	findings = append(findings, fmt.Sprintf("K8s Pod 枚举 (namespace: %s):", namespace))
	findings = append(findings, fmt.Sprintf("  Pod 总数: %d", len(podList.Items)))

	for i, pod := range podList.Items {
		if i >= 10 {
			findings = append(findings, fmt.Sprintf("  ... (共 %d 个 Pod)", len(podList.Items)))
			break
		}
		findings = append(findings, fmt.Sprintf("  - %s (SA: %s)", pod.Metadata.Name, pod.Spec.ServiceAccountName))

		// 检测危险镜像标签
		for _, c := range pod.Spec.Containers {
			if strings.HasSuffix(c.Image, ":latest") {
				findings = append(findings, fmt.Sprintf("    ⚠ 容器 %s 使用 :latest 标签", c.Name))
			}
		}
	}

	return tools.ToolResult{Success: true, Stdout: strings.Join(findings, "\n")}
}

// k8sRBACCheck —— K8s RBAC 权限矩阵分析 + 提权路径检测。
func k8sRBACCheck(args map[string]any) tools.ToolResult {
	serviceAccount := tools.ArgStr(args, "service_account", "default")
	namespace := tools.ArgStr(args, "namespace", "default")

	var findings []string
	findings = append(findings, fmt.Sprintf("K8s RBAC 分析 (SA: %s/%s):", namespace, serviceAccount))

	// 1. 检测 ServiceAccount 绑定的 Role/ClusterRole
	// kubectl get rolebindings,clusterrolebindings --all-namespaces -o json
	bindingsCmd := []string{"kubectl", "get", "rolebindings,clusterrolebindings", "--all-namespaces", "-o", "json"}
	bindingsRes := tools.Sh(bindingsCmd, 60*time.Second)
	if !bindingsRes.Success {
		return bindingsRes
	}

	// 简化检测: 搜索包含目标 SA 的绑定
	if strings.Contains(bindingsRes.Stdout, fmt.Sprintf(`"name":"%s"`, serviceAccount)) {
		findings = append(findings, "  ✓ ServiceAccount 有权限绑定")
	} else {
		findings = append(findings, "  ✗ ServiceAccount 无权限绑定 (默认权限)")
	}

	// 2. 检测危险权限
	dangerousPerms := []string{
		"get secrets",         // 可读取所有 Secret (含 SA token)
		"create pods",         // 可创建特权 Pod
		"exec pods",           // 可在 Pod 中执行命令
		"patch deployments",   // 可修改部署注入后门
		"*",                   // 通配符权限
	}

	for _, perm := range dangerousPerms {
		if strings.Contains(bindingsRes.Stdout, perm) {
			findings = append(findings, fmt.Sprintf("  ⚠ 危险权限: %s", perm))
		}
	}

	// 3. 检测 cluster-admin 绑定
	if strings.Contains(bindingsRes.Stdout, "cluster-admin") && strings.Contains(bindingsRes.Stdout, serviceAccount) {
		findings = append(findings, "  🔴 ServiceAccount 拥有 cluster-admin 权限 (完全控制)")
	}

	return tools.ToolResult{Success: true, Stdout: strings.Join(findings, "\n")}
}

// k8sNodeExploitEnhanced —— K8s 节点提权利用 (hostPath / privileged Pod)。
func k8sNodeExploitEnhanced(args map[string]any) tools.ToolResult {
	namespace := tools.ArgStr(args, "namespace", "default")
	podName := tools.ArgStr(args, "pod_name", "")

	if podName == "" {
		return tools.ToolResult{Success: false, Stderr: "缺少 pod_name 参数", RC: -1}
	}

	var findings []string
	findings = append(findings, fmt.Sprintf("K8s 节点提权利用 (Pod: %s/%s):", namespace, podName))

	// 1. 检测 Pod 是否特权模式
	podCmd := []string{"kubectl", "get", "pod", podName, "-n", namespace, "-o", "json"}
	podRes := tools.Sh(podCmd, 15*time.Second)
	if !podRes.Success {
		return podRes
	}

	var pod struct {
		Spec struct {
			Containers []struct {
				SecurityContext *struct {
					Privileged *bool `json:"privileged"`
				} `json:"securityContext"`
			} `json:"containers"`
			Volumes []struct {
				Name     string `json:"name"`
				HostPath *struct {
					Path string `json:"path"`
				} `json:"hostPath"`
			} `json:"volumes"`
		} `json:"spec"`
	}

	if err := json.Unmarshal([]byte(podRes.Stdout), &pod); err != nil {
		return tools.ToolResult{Success: false, Stderr: "解析 Pod JSON 失败", RC: -1}
	}

	// 检测特权模式
	for _, c := range pod.Spec.Containers {
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			findings = append(findings, "  🔴 容器运行在特权模式 (可逃逸到节点)")
			findings = append(findings, "     利用: nsenter -t 1 -m -u -i -n -- bash")
		}
	}

	// 检测 hostPath 挂载
	for _, v := range pod.Spec.Volumes {
		if v.HostPath != nil {
			findings = append(findings, fmt.Sprintf("  ⚠ hostPath 挂载: %s", v.HostPath.Path))
			if v.HostPath.Path == "/" || v.HostPath.Path == "/var/run/docker.sock" {
				findings = append(findings, "     🔴 危险挂载点 (可完全控制节点)")
			}
		}
	}

	if len(findings) == 1 {
		findings = append(findings, "  ✓ Pod 配置安全 (无明显逃逸路径)")
	}

	return tools.ToolResult{Success: true, Stdout: strings.Join(findings, "\n")}
}

// helmScan —— Helm Chart 配置审计 (安全缺陷检测)。
func helmScan(args map[string]any) tools.ToolResult {
	release := tools.ArgStr(args, "release", "")
	namespace := tools.ArgStr(args, "namespace", "default")

	if release == "" {
		// 列举所有 release
		listCmd := []string{"helm", "list", "-n", namespace, "-o", "json"}
		listRes := tools.Sh(listCmd, 30*time.Second)
		if !listRes.Success {
			return listRes
		}

		var releases []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(listRes.Stdout), &releases); err != nil {
			return tools.ToolResult{Success: false, Stderr: "解析 Helm 列表失败", RC: -1}
		}

		var findings []string
		findings = append(findings, fmt.Sprintf("Helm Release 列表 (namespace: %s):", namespace))
		for _, r := range releases {
			findings = append(findings, fmt.Sprintf("  - %s (状态: %s)", r.Name, r.Status))
		}

		return tools.ToolResult{Success: true, Stdout: strings.Join(findings, "\n")}
	}

	// 审计指定 release
	valuesCmd := []string{"helm", "get", "values", release, "-n", namespace, "-o", "json"}
	valuesRes := tools.Sh(valuesCmd, 30*time.Second)
	if !valuesRes.Success {
		return valuesRes
	}

	var findings []string
	findings = append(findings, fmt.Sprintf("Helm Chart 审计 (release: %s):", release))

	// 检测不安全配置
	unsafePatterns := []string{
		`"privileged":true`,
		`"runAsUser":0`,
		`"allowPrivilegeEscalation":true`,
		`"hostNetwork":true`,
		`"hostPID":true`,
		`:latest`,
	}

	for _, pattern := range unsafePatterns {
		if strings.Contains(valuesRes.Stdout, pattern) {
			findings = append(findings, fmt.Sprintf("  ⚠ 不安全配置: %s", pattern))
		}
	}

	if len(findings) == 1 {
		findings = append(findings, "  ✓ Chart 配置安全")
	}

	return tools.ToolResult{Success: true, Stdout: strings.Join(findings, "\n")}
}

// dockerEscapeExploitEnhanced —— Docker 容器逃逸利用 (特权容器/cgroup)。
func dockerEscapeExploitEnhanced(args map[string]any) tools.ToolResult {
	var findings []string
	findings = append(findings, "Docker 容器逃逸检测:")

	// 1. 检测是否在容器内
	if !isInContainer() {
		return tools.ToolResult{Success: false, Stderr: "当前不在容器环境", RC: -1}
	}
	findings = append(findings, "  ✓ 检测到容器环境")

	// 2. 检测特权模式 (ip link add)
	privCmd := []string{"sh", "-c", "ip link add dummy0 type dummy 2>&1"}
	privRes := tools.Sh(privCmd, 5*time.Second)
	if privRes.Success || !strings.Contains(privRes.Stderr, "Operation not permitted") {
		findings = append(findings, "  🔴 特权容器检测: 可能运行在特权模式")
		findings = append(findings, "     利用路径: nsenter/cgroup release_agent")
	} else {
		findings = append(findings, "  ✓ 非特权容器")
	}

	// 3. 检测 Docker socket 挂载
	sockCmd := []string{"test", "-S", "/var/run/docker.sock"}
	sockRes := tools.Sh(sockCmd, 2*time.Second)
	if sockRes.Success {
		findings = append(findings, "  🔴 Docker socket 已挂载 (/var/run/docker.sock)")
		findings = append(findings, "     利用: docker -H unix:///var/run/docker.sock run --privileged ...")
	}

	// 4. 检测危险能力 (CAP_SYS_ADMIN)
	capCmd := []string{"sh", "-c", "cat /proc/self/status | grep CapEff"}
	capRes := tools.Sh(capCmd, 5*time.Second)
	if capRes.Success && capRes.Stdout != "" {
		findings = append(findings, fmt.Sprintf("  容器能力: %s", strings.TrimSpace(capRes.Stdout)))
	}

	return tools.ToolResult{Success: true, Stdout: strings.Join(findings, "\n")}
}

// isInContainer —— 检测是否运行在容器内。
func isInContainer() bool {
	// 检测 /.dockerenv 或 /proc/1/cgroup 含 docker/kubepods
	dockerEnvCmd := []string{"test", "-f", "/.dockerenv"}
	if res := tools.Sh(dockerEnvCmd, 2*time.Second); res.Success {
		return true
	}

	cgroupCmd := []string{"sh", "-c", "grep -q 'docker\\|kubepods' /proc/1/cgroup 2>/dev/null"}
	if res := tools.Sh(cgroupCmd, 2*time.Second); res.Success {
		return true
	}

	return false
}

// ParseK8sPods —— 解析 K8s Pod 枚举结果。
func ParseK8sPods(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ":latest") {
			obs = append(obs, tools.Observation{
				Kind:     "finding",
				Key:      "k8s:image:latest",
				Label:    "[medium] K8s Pod 使用 :latest 镜像标签",
				Excerpt:  line,
				Severity: "medium",
			})
		}
	}
	return obs
}

// ParseK8sRBAC —— 解析 K8s RBAC 权限检测结果。
func ParseK8sRBAC(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	if strings.Contains(out, "cluster-admin") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "k8s:rbac:cluster-admin",
			Label:    "[critical] K8s ServiceAccount 拥有 cluster-admin 权限",
			Excerpt:  "ServiceAccount 拥有 cluster-admin 权限 (完全控制)",
			Severity: "critical",
			Technique: "T1078", // Valid Accounts
			Tactic:   "privilege-escalation",
		})
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "⚠ 危险权限:") {
			perm := strings.TrimPrefix(strings.TrimSpace(line), "⚠ 危险权限: ")
			obs = append(obs, tools.Observation{
				Kind:     "finding",
				Key:      "k8s:rbac:" + perm,
				Label:    "[high] K8s 危险权限: " + perm,
				Excerpt:  line,
				Severity: "high",
			})
		}
	}

	return obs
}

// ParseK8sNodeExploitEnhanced —— 解析 K8s 节点提权结果。
// 同时兼容增强格式(特权模式/危险挂载点)和原有格式(Can access host/Kubelet API),
// 因为 K8sPackEnhanced 在 RegisterDefaults 中覆盖了 ContainerPack 的同名工具注册。
func ParseK8sNodeExploitEnhanced(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	// 增强格式: 特权模式
	if strings.Contains(out, "特权模式") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "k8s:escape:privileged",
			Label:    "[critical] K8s 特权 Pod 可逃逸到节点",
			Excerpt:  "容器运行在特权模式 (可逃逸到节点)",
			Severity: "critical",
			Technique: "T1611", // Escape to Host
			Tactic:   "privilege-escalation",
		})
	}

	// 增强格式: 危险挂载点
	if strings.Contains(out, "危险挂载点") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "k8s:escape:hostpath",
			Label:    "[critical] K8s 危险 hostPath 挂载",
			Excerpt:  "hostPath 挂载: / 或 /var/run/docker.sock",
			Severity: "critical",
		})
	}

	// 原有格式兼容: ContainerPack 的 k8sNodeExploit 输出
	if strings.Contains(out, "Can access host /etc/passwd") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "k8s:host_access",
			Label:    "[critical] Can access host filesystem from pod",
			Excerpt:  "Can access host /etc/passwd",
			Severity: "critical",
		})
	}
	if strings.Contains(out, "Kubelet API accessible") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "k8s:kubelet_unauth",
			Label:    "[high] Kubelet API accessible without authentication",
			Excerpt:  "Kubelet API accessible at :10250",
			Severity: "high",
		})
	}

	return obs
}

// ParseDockerEscapeEnhanced —— 解析 Docker 逃逸检测结果。
func ParseDockerEscapeEnhanced(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	if strings.Contains(out, "特权容器") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "docker:escape:privileged",
			Label:    "[critical] Docker 特权容器可逃逸",
			Excerpt:  "特权容器检测: 可能运行在特权模式",
			Severity: "critical",
			Technique: "T1611",
			Tactic:   "privilege-escalation",
		})
	}

	if strings.Contains(out, "Docker socket 已挂载") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "docker:escape:socket",
			Label:    "[critical] Docker socket 挂载可完全控制宿主机",
			Excerpt:  "Docker socket 已挂载 (/var/run/docker.sock)",
			Severity: "critical",
		})
	}

	return obs
}

// K8sPackEnhanced —— K8s/容器渗透增强包 (抄 Reaper + ThreatCanvas)。
func K8sPackEnhanced() Pack {
	return Pack{
		Name: "K8sEnhanced",
		Tools: []*tools.Tool{
			{
				Name:  "k8s_enum_pods",
				Level: tools.LevelScan,
				Desc:  "K8s Pod 枚举 + ServiceAccount token 提取, 发现配置缺陷 (latest 标签)",
				Run:   k8sEnumPods,
				Parse: ParseK8sPods,
				Args: []tools.ArgSpec{
					{Name: "namespace", Desc: "K8s namespace (默认 default)"},
				},
			},
			{
				Name:  "k8s_rbac_check",
				Level: tools.LevelScan,
				Desc:  "K8s RBAC 权限矩阵分析, 检测危险权限 (get secrets / create pods / cluster-admin)",
				Run:   k8sRBACCheck,
				Parse: ParseK8sRBAC,
				Args: []tools.ArgSpec{
					{Name: "service_account", Desc: "ServiceAccount 名称 (默认 default)"},
					{Name: "namespace", Desc: "K8s namespace (默认 default)"},
				},
			},
			{
				Name:  "k8s_node_exploit",
				Level: tools.LevelExploit,
				Desc:  "K8s 节点提权利用, 检测特权 Pod / hostPath 挂载并生成逃逸利用路径",
				Run:   k8sNodeExploitEnhanced,
				Parse: ParseK8sNodeExploitEnhanced,
				Args: []tools.ArgSpec{
					{Name: "pod_name", Desc: "目标 Pod 名称", Required: true},
					{Name: "namespace", Desc: "K8s namespace (默认 default)"},
				},
			},
			{
				Name:  "helm_scan",
				Level: tools.LevelScan,
				Desc:  "Helm Chart 配置审计, 检测不安全配置 (privileged / runAsUser:0 / :latest)",
				Run:   helmScan,
				Parse: nil,
				Args: []tools.ArgSpec{
					{Name: "release", Desc: "Helm release 名称 (留空列举所有)"},
					{Name: "namespace", Desc: "K8s namespace (默认 default)"},
				},
			},
			{
				Name:  "docker_escape_exploit",
				Level: tools.LevelExploit,
				Desc:  "Docker 容器逃逸利用, 检测特权模式 / Docker socket / CAP_SYS_ADMIN",
				Run:   dockerEscapeExploitEnhanced,
				Parse: ParseDockerEscapeEnhanced,
				Args:  []tools.ArgSpec{},
			},
		},
		// 指纹: 发现 K8s/容器环境特征时激活
		Fingerprint: func(services map[string]bool) bool {
			return services["kubernetes"] || services["k8s"] || services["docker"] || services["container"]
		},
	}
}
