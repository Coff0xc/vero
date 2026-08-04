package scenarios

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// TestK8sPackEnhanced —— 验证 K8s/容器渗透增强包。
func TestK8sPackEnhanced(t *testing.T) {
	reg := tools.NewRegistry()
	m := NewManager()
	m.Register(reg, K8sPackEnhanced())

	// 1) 验证工具注册
	expectedTools := []string{"k8s_enum_pods", "k8s_rbac_check", "k8s_node_exploit", "helm_scan", "docker_escape_exploit"}
	for _, name := range expectedTools {
		if !reg.Has(name) {
			t.Errorf("工具 %s 未注册", name)
		}
	}

	// 2) 验证 Args 规格
	nodeExploitTool, _ := reg.Get("k8s_node_exploit")
	if len(nodeExploitTool.Args) < 1 || nodeExploitTool.Args[0].Name != "pod_name" {
		t.Error("k8s_node_exploit 缺少 pod_name 参数规格")
	}
	if !nodeExploitTool.Args[0].Required {
		t.Error("k8s_node_exploit 的 pod_name 应为必填")
	}

	// 3) 验证 Parser (Pods)
	mockPodsOutput := `K8s Pod 枚举 (namespace: default):
  Pod 总数: 3
  - nginx-pod (SA: default)
    ⚠ 容器 nginx 使用 :latest 标签
  - redis-pod (SA: redis-sa)
    ⚠ 容器 redis 使用 :latest 标签`

	obs := ParseK8sPods(mockPodsOutput, map[string]any{})
	if len(obs) != 2 {
		t.Fatalf("ParseK8sPods 应解析 2 个 :latest 警告, got %d", len(obs))
	}
	if obs[0].Severity != "medium" {
		t.Errorf(":latest 标签应为 medium 严重度, got %s", obs[0].Severity)
	}

	// 4) 验证 Parser (RBAC)
	mockRBACOutput := `K8s RBAC 分析 (SA: default/admin-sa):
  ✓ ServiceAccount 有权限绑定
  ⚠ 危险权限: get secrets
  ⚠ 危险权限: create pods
  🔴 ServiceAccount 拥有 cluster-admin 权限 (完全控制)`

	obsRBAC := ParseK8sRBAC(mockRBACOutput, map[string]any{})
	if len(obsRBAC) < 3 {
		t.Fatalf("ParseK8sRBAC 应解析至少 3 个观察, got %d", len(obsRBAC))
	}

	var clusterAdminFound bool
	for _, o := range obsRBAC {
		if o.Key == "k8s:rbac:cluster-admin" {
			clusterAdminFound = true
			if o.Severity != "critical" {
				t.Errorf("cluster-admin 应为 critical, got %s", o.Severity)
			}
			if o.Technique != "T1078" {
				t.Errorf("RBAC 提权应映射 T1078, got %s", o.Technique)
			}
		}
	}
	if !clusterAdminFound {
		t.Error("未找到 cluster-admin 观察")
	}

	// 5) 验证 Parser (Node Exploit)
	mockNodeOutput := `K8s 节点提权利用 (Pod: default/evil-pod):
  🔴 容器运行在特权模式 (可逃逸到节点)
     利用: nsenter -t 1 -m -u -i -n -- bash
  ⚠ hostPath 挂载: /var/run/docker.sock
     🔴 危险挂载点 (可完全控制节点)`

	obsNode := ParseK8sNodeExploitEnhanced(mockNodeOutput, map[string]any{})
	if len(obsNode) != 2 {
		t.Fatalf("ParseK8sNodeExploit 应解析 2 个逃逸向量, got %d", len(obsNode))
	}

	var privFound, hostPathFound bool
	for _, o := range obsNode {
		if o.Key == "k8s:escape:privileged" {
			privFound = true
			if o.Technique != "T1611" {
				t.Errorf("容器逃逸应映射 T1611, got %s", o.Technique)
			}
		}
		if o.Key == "k8s:escape:hostpath" {
			hostPathFound = true
		}
	}
	if !privFound || !hostPathFound {
		t.Error("未找到特权容器或 hostPath 观察")
	}

	// 6) 验证 Parser (Docker Escape)
	mockDockerOutput := `Docker 容器逃逸检测:
  ✓ 检测到容器环境
  🔴 特权容器检测: 可能运行在特权模式
     利用路径: nsenter/cgroup release_agent
  🔴 Docker socket 已挂载 (/var/run/docker.sock)
     利用: docker -H unix:///var/run/docker.sock run --privileged ...`

	obsDocker := ParseDockerEscapeEnhanced(mockDockerOutput, map[string]any{})
	if len(obsDocker) != 2 {
		t.Fatalf("ParseDockerEscape 应解析 2 个逃逸向量, got %d", len(obsDocker))
	}

	for _, o := range obsDocker {
		if o.Severity != "critical" {
			t.Errorf("Docker 逃逸应为 critical, got %s", o.Severity)
		}
	}

	// 7) 验证指纹函数
	pack := K8sPackEnhanced()
	if !pack.Fingerprint(map[string]bool{"kubernetes": true}) {
		t.Error("K8sPackEnhanced 应对 kubernetes 服务指纹激活")
	}
	if !pack.Fingerprint(map[string]bool{"docker": true}) {
		t.Error("K8sPackEnhanced 应对 docker 服务指纹激活")
	}
	if pack.Fingerprint(map[string]bool{"http": true}) {
		t.Error("K8sPackEnhanced 不应对纯 http 激活")
	}
}

// TestK8sParserEdgeCases —— 测试 Parser 边界情况。
func TestK8sParserEdgeCases(t *testing.T) {
	// 1) 空输出
	obs := ParseK8sPods("", map[string]any{})
	if len(obs) != 0 {
		t.Error("空输出应返回 0 个观察")
	}

	// 2) 无危险配置
	obs = ParseK8sPods("K8s Pod 枚举:\n  - nginx-pod (SA: default)\n    镜像: nginx:1.21", map[string]any{})
	if len(obs) != 0 {
		t.Error("无 :latest 标签应返回 0 个观察")
	}

	// 3) 无危险权限
	obs = ParseK8sRBAC("K8s RBAC 分析:\n  ✓ ServiceAccount 有权限绑定", map[string]any{})
	if len(obs) != 0 {
		t.Error("无危险权限应返回 0 个观察")
	}

	// 4) 安全 Pod 配置
	obs = ParseK8sNodeExploitEnhanced("K8s 节点提权利用:\n  ✓ Pod 配置安全 (无明显逃逸路径)", map[string]any{})
	if len(obs) != 0 {
		t.Error("安全配置应返回 0 个观察")
	}

	// 5) 非容器环境
	obs = ParseDockerEscapeEnhanced("Docker 容器逃逸检测:\n  ✗ 当前不在容器环境", map[string]any{})
	if len(obs) != 0 {
		t.Error("非容器环境应返回 0 个观察")
	}
}
