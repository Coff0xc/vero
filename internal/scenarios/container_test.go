package scenarios

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- Docker Escape Parser Tests ----------

func TestParseDockerEscape(t *testing.T) {
	stdout := `Docker Escape Check:
  [+] Running in Docker container
  Capabilities: CapEff:	0000003fffffffff
  [!] PRIVILEGED CONTAINER DETECTED
  [!] Docker socket mounted at /var/run/docker.sock
      -> Can control host Docker daemon
  [!] Host filesystem mounted (possible at /host or /rootfs)
  [+] /proc is mounted
  [-] AppArmor not active`

	obs := ParseDockerEscape(stdout, nil)

	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

	// 检查特权容器发现
	found := false
	for _, o := range obs {
		if o.Key == "container:privileged" && o.Kind == "finding" {
			found = true
			if o.Label != "[critical] Running in privileged container - full host access" {
				t.Errorf("wrong label: %s", o.Label)
			}
		}
	}
	if !found {
		t.Error("privileged container finding not detected")
	}

	// 检查 Docker socket 发现
	found = false
	for _, o := range obs {
		if o.Key == "container:docker_socket" {
			found = true
		}
	}
	if !found {
		t.Error("docker socket finding not detected")
	}

	// 检查宿主机文件系统发现
	found = false
	for _, o := range obs {
		if o.Key == "container:host_fs" {
			found = true
		}
	}
	if !found {
		t.Error("host filesystem finding not detected")
	}
}

func TestParseDockerEscapeNoFindings(t *testing.T) {
	stdout := `Docker Escape Check:
  [+] Running in Docker container
  Capabilities: CapEff:	00000000a80425fb
  [+] AppArmor enabled`

	obs := ParseDockerEscape(stdout, nil)

	if len(obs) != 0 {
		t.Errorf("expected 0 observations for safe container, got %d", len(obs))
	}
}

// ---------- K8s ServiceAccount Parser Tests ----------

func TestParseK8sServiceAccount(t *testing.T) {
	stdout := `Kubernetes ServiceAccount Enumeration:
  [+] ServiceAccount found at /var/run/secrets/kubernetes.io/serviceaccount
  [!] Token: eyJhbGciOiJSUzI1NiIsImtpZCI6IkR...
  Namespace: default
  [+] CA certificate available

  Trying to access K8s API at https://kubernetes.default.svc...
  [!] API accessible - can list namespaces
{"kind":"NamespaceList","apiVersion":"v1","items":[...]}`

	obs := ParseK8sServiceAccount(stdout, nil)

	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}

	// 检查 token 提取
	found := false
	for _, o := range obs {
		if o.Key == "k8s:serviceaccount:token" && o.Kind == "cred" {
			found = true
		}
	}
	if !found {
		t.Error("ServiceAccount token not extracted")
	}

	// 检查 API 访问
	found = false
	for _, o := range obs {
		if o.Key == "k8s:api:access" {
			found = true
		}
	}
	if !found {
		t.Error("K8s API access not detected")
	}
}

func TestParseK8sServiceAccountNoAccess(t *testing.T) {
	stdout := `Kubernetes ServiceAccount Enumeration:
  [+] ServiceAccount found at /var/run/secrets/kubernetes.io/serviceaccount
  [!] Token: eyJhbGciOiJSUzI1NiIsImtpZCI6IkR...
  Namespace: default

  Trying to access K8s API at https://kubernetes.default.svc...
  [+] API reachable but access forbidden`

	obs := ParseK8sServiceAccount(stdout, nil)

	// 应该只提取到 token，没有 API 访问权限
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation (token only), got %d", len(obs))
	}

	if obs[0].Key != "k8s:serviceaccount:token" {
		t.Errorf("expected token observation, got %s", obs[0].Key)
	}
}

// ---------- K8s Node Exploit Parser Tests ----------

func TestParseK8sNodeExploit(t *testing.T) {
	stdout := `Kubernetes Node Exploitation Check:
  [!] Host filesystem mounted at /host
  [+] Attempting to chroot to host...
  [!] Can access host /etc/passwd
root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
  [!] Docker socket available
  [+] Can create privileged container on host
  [!] Docker command works:
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS
abc123def456   nginx     "nginx"   2 hours   Up 2 hours
  [!] Kubelet API accessible at :10250 (unauthenticated)`

	obs := ParseK8sNodeExploitEnhanced(stdout, nil)

	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}

	// 检查宿主机访问
	found := false
	for _, o := range obs {
		if o.Key == "k8s:host_access" {
			found = true
			if o.Label != "[critical] Can access host filesystem from pod" {
				t.Errorf("wrong label: %s", o.Label)
			}
		}
	}
	if !found {
		t.Error("host access finding not detected")
	}

	// 检查 Kubelet API
	found = false
	for _, o := range obs {
		if o.Key == "k8s:kubelet_unauth" {
			found = true
		}
	}
	if !found {
		t.Error("kubelet API finding not detected")
	}
}

func TestParseK8sNodeExploitNoFindings(t *testing.T) {
	stdout := `Kubernetes Node Exploitation Check:
  [-] No host filesystem mounted
  [-] Docker socket not available
  [-] Kubelet API not accessible`

	obs := ParseK8sNodeExploitEnhanced(stdout, nil)

	if len(obs) != 0 {
		t.Errorf("expected 0 observations for secure pod, got %d", len(obs))
	}
}

// ---------- Container Pack Tests ----------

func TestContainerPack(t *testing.T) {
	pack := ContainerPack()

	if pack.Name != "container" {
		t.Errorf("expected pack name 'container', got %s", pack.Name)
	}

	if len(pack.Tools) != 2 { // D28: k8s_node_exploit 移出, 由 K8sPackEnhanced 唯一注册
		t.Fatalf("expected 2 tools, got %d", len(pack.Tools))
	}

	// 验证工具名称
	toolNames := make(map[string]bool)
	for _, tool := range pack.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{"docker_escape_check", "k8s_sa_enum"} // D28: k8s_node_exploit 移出 ContainerPack(由 K8sPackEnhanced 唯一注册)
	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing tool: %s", name)
		}
	}

	// 验证工具 Level
	for _, tool := range pack.Tools {
		switch tool.Name {
		case "docker_escape_check":
			if tool.Level != tools.LevelScan {
				t.Errorf("docker_escape_check should be LevelScan")
			}
		case "k8s_sa_enum":
			if tool.Level != tools.LevelCred {
				t.Errorf("k8s_sa_enum should be LevelCred")
			}
		}
	}

	// Fingerprint 总是返回 false (除非真的在容器内)
	// 这里只测试不 panic
	services := map[string]bool{}
	_ = pack.Fingerprint(services)
}
