package scenarios

import (
	"strings"
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// TestCloudToolsDeepDive —— P2 云工具深度测试。
func TestCloudToolsDeepDive(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	t.Run("AWS IMDS 多实例场景", func(t *testing.T) {
		awsTool, _ := reg.Get("aws_imds_enum")

		// 模拟多角色 IAM 凭证泄露
		multiRoleOutput := `AWS IMDS Enumeration:
  instance-id: i-prod-web-01
  hostname: ip-10-0-1-50.ec2.internal
  iam/security-credentials/: WebServerRole
  IAM Credentials (WebServerRole):
  AccessKeyId: AKIAWEB123
  SecretAccessKey: webkey123
  Token: webtoken...

  instance-id: i-prod-db-01
  iam/security-credentials/: DatabaseRole
  IAM Credentials (DatabaseRole):
  AccessKeyId: AKIADB456
  SecretAccessKey: dbkey456`

		obs := awsTool.Parse(multiRoleOutput, map[string]any{})

		// 应提取 2 个实例 ID + 1 个凭证 (去重机制)
		instanceCount := 0
		credCount := 0
		for _, o := range obs {
			if strings.Contains(o.Key, "aws:instance:") {
				instanceCount++
			}
			if o.Key == "aws:iam:credentials" {
				credCount++
			}
		}

		if instanceCount != 2 {
			t.Errorf("应提取 2 个实例 ID, 实际 %d", instanceCount)
		}
		if credCount != 1 {
			t.Errorf("去重机制应只记录 1 个凭证观测, 实际 %d", credCount)
		}
	})

	t.Run("Azure Managed Identity 完整链", func(t *testing.T) {
		azureTool, _ := reg.Get("azure_imds_enum")

		fullAzureOutput := `Azure IMDS Enumeration:
{
  "compute": {
    "vmId": "12345678-1234-1234-1234-123456789012",
    "vmSize": "Standard_D2s_v3",
    "location": "eastus",
    "resourceGroupName": "prod-rg"
  },
  "network": {
    "interface": [
      {
        "ipv4": {
          "ipAddress": [
            {
              "privateIpAddress": "10.0.1.4",
              "publicIpAddress": "20.81.45.67"
            }
          ]
        }
      }
    ]
  }
}

Managed Identity Token:
{
  "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
  "expires_in": "3599",
  "resource": "https://management.azure.com/"
}`

		obs := azureTool.Parse(fullAzureOutput, map[string]any{})

		// 应提取 VM 元数据 + Managed Identity token
		hasVM := false
		hasToken := false
		for _, o := range obs {
			if o.Key == "azure:vm:metadata" {
				hasVM = true
			}
			if o.Key == "azure:managed_identity" {
				hasToken = true
			}
		}

		if !hasVM {
			t.Error("应识别 Azure VM 元数据")
		}
		if !hasToken {
			t.Error("应识别 Managed Identity token")
		}
	})

	t.Run("GCP Service Account 权限枚举", func(t *testing.T) {
		gcpTool, _ := reg.Get("gcp_imds_enum")

		gcpOutput := `GCP IMDS Enumeration:
  project-id: my-gcp-project-123456
  instance-id: 1234567890123456789
  service-accounts/default/email: default-sa@my-gcp-project.iam.gserviceaccount.com
  service-accounts/default/scopes:
    - https://www.googleapis.com/auth/cloud-platform
    - https://www.googleapis.com/auth/compute
  service-accounts/default/token:
    {
      "access_token": "ya29.c.Kp8B...",
      "expires_in": 3599,
      "token_type": "Bearer"
    }`

		obs := gcpTool.Parse(gcpOutput, map[string]any{})

		hasProject := false
		hasToken := false
		for _, o := range obs {
			if strings.Contains(o.Key, "gcp:project:") {
				hasProject = true
			}
			if o.Key == "gcp:service_account:token" {
				hasToken = true
			}
		}

		if !hasProject {
			t.Error("应提取 GCP project ID")
		}
		if !hasToken {
			t.Error("应识别 Service Account token")
		}
	})

	t.Run("S3 公开访问多场景", func(t *testing.T) {
		s3Tool, _ := reg.Get("s3_bucket_enum")

		// 场景 1: 完全公开
		publicOutput := `S3 Bucket: public-data
HTTP/1.1 200 OK
<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
  <Name>public-data</Name>
  <Contents>
    <Key>backup.tar.gz</Key>
  </Contents>
</ListBucketResult>

[!] Bucket is publicly accessible`

		obs := s3Tool.Parse(publicOutput, map[string]any{"bucket": "public-data"})
		if len(obs) == 0 {
			t.Error("应检测到公开访问")
		}
		if !strings.Contains(obs[0].Label, "publicly accessible") {
			t.Errorf("Label 应标明公开访问, 实际: %s", obs[0].Label)
		}

		// 场景 2: 私有 bucket
		privateOutput := `S3 Bucket: private-data
HTTP/1.1 403 Forbidden
[+] Bucket exists but access denied (private)`

		obs = s3Tool.Parse(privateOutput, map[string]any{"bucket": "private-data"})
		if len(obs) != 0 {
			t.Error("私有 bucket 不应产生 finding")
		}

		// 场景 3: Bucket 不存在
		notExistOutput := `S3 Bucket: nonexistent
HTTP/1.1 404 Not Found
[-] Bucket does not exist`

		obs = s3Tool.Parse(notExistOutput, map[string]any{"bucket": "nonexistent"})
		if len(obs) != 0 {
			t.Error("不存在的 bucket 不应产生 finding")
		}
	})
}

// TestContainerToolsDeepDive —— P3 容器工具深度测试。
func TestContainerToolsDeepDive(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	t.Run("Docker 特权容器完整检测", func(t *testing.T) {
		dockerTool, _ := reg.Get("docker_escape_check")

		// 完整的特权容器输出
		privilegedOutput := `Docker Escape Check:
  [+] Running in Docker container
  Capabilities: CapEff:	0000003fffffffff
  [!] PRIVILEGED CONTAINER DETECTED
  [!] Docker socket mounted at /var/run/docker.sock
      -> Can control host Docker daemon
  [!] Host filesystem mounted (possible at /host or /rootfs)
  [+] /proc is mounted
  [-] AppArmor not active
  cgroup: 0::/docker/abc123def456`

		obs := dockerTool.Parse(privilegedOutput, map[string]any{})

		// 应检测到 3 个主要逃逸向量
		findingKeys := make(map[string]bool)
		for _, o := range obs {
			findingKeys[o.Key] = true
		}

		expectedFindings := []string{
			"container:privileged",
			"container:docker_socket",
			"container:host_fs",
		}

		for _, key := range expectedFindings {
			if !findingKeys[key] {
				t.Errorf("应检测到 %s", key)
			}
		}
	})

	t.Run("安全容器基线验证", func(t *testing.T) {
		dockerTool, _ := reg.Get("docker_escape_check")

		// 安全配置的容器
		secureOutput := `Docker Escape Check:
  [+] Running in Docker container
  Capabilities: CapEff:	00000000a80425fb
  [+] AppArmor enabled
  [+] SELinux enforcing
  [-] Docker socket not mounted
  [-] No host filesystem access
  [+] Running as non-root user (uid=1000)`

		obs := dockerTool.Parse(secureOutput, map[string]any{})

		if len(obs) != 0 {
			t.Errorf("安全容器不应产生 finding, 实际: %d", len(obs))
		}
	})

	t.Run("K8s ServiceAccount 完整权限链", func(t *testing.T) {
		k8sTool, _ := reg.Get("k8s_sa_enum")

		// 高权限 ServiceAccount
		adminSAOutput := `Kubernetes ServiceAccount Enumeration:
  [+] ServiceAccount found at /var/run/secrets/kubernetes.io/serviceaccount
  [!] Token: eyJhbGciOiJSUzI1NiIsImtpZCI6Ik...
  Namespace: kube-system
  [+] CA certificate available

  Trying to access K8s API at https://kubernetes.default.svc...
  [!] API accessible - can list namespaces
{"kind":"NamespaceList","apiVersion":"v1","items":[
  {"metadata":{"name":"default"}},
  {"metadata":{"name":"kube-system"}},
  {"metadata":{"name":"kube-public"}}
]}`

		obs := k8sTool.Parse(adminSAOutput, map[string]any{})

		hasToken := false
		hasAPIAccess := false
		for _, o := range obs {
			if o.Key == "k8s:serviceaccount:token" {
				hasToken = true
				if o.Kind != "cred" {
					t.Errorf("Token 应为 cred 类型, 实际: %s", o.Kind)
				}
			}
			if o.Key == "k8s:api:access" {
				hasAPIAccess = true
				if !strings.Contains(o.Label, "critical") {
					t.Error("API 访问应标记为 critical")
				}
			}
		}

		if !hasToken {
			t.Error("应提取 ServiceAccount token")
		}
		if !hasAPIAccess {
			t.Error("应检测到 API 访问权限")
		}
	})

	t.Run("K8s 受限权限 ServiceAccount", func(t *testing.T) {
		k8sTool, _ := reg.Get("k8s_sa_enum")

		// 受限权限 (Forbidden)
		restrictedOutput := `Kubernetes ServiceAccount Enumeration:
  [+] ServiceAccount found at /var/run/secrets/kubernetes.io/serviceaccount
  [!] Token: eyJhbGciOiJSUzI1NiIsImtpZCI6Ik...
  Namespace: default

  Trying to access K8s API at https://kubernetes.default.svc...
  [+] API reachable but access forbidden
{"kind":"Status","status":"Failure","reason":"Forbidden"}`

		obs := k8sTool.Parse(restrictedOutput, map[string]any{})

		// 应只提取 token, 没有 API 访问
		if len(obs) != 1 {
			t.Errorf("应只提取 token, 实际观测数: %d", len(obs))
		}
		if obs[0].Key != "k8s:serviceaccount:token" {
			t.Errorf("应只有 token 观测, 实际: %s", obs[0].Key)
		}
	})

	t.Run("K8s Node 逃逸多向量", func(t *testing.T) {
		nodeTool, _ := reg.Get("k8s_node_exploit")

		// 多种逃逸向量共存
		multiVectorOutput := `Kubernetes Node Exploitation Check:
  [!] Host filesystem mounted at /host
  [+] Attempting to chroot to host...
  [!] Can access host /etc/passwd
root:x:0:0:root:/root:/bin/bash
  [!] Docker socket available
  [+] Can create privileged container on host
  [!] Docker command works:
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS
abc123         nginx     "nginx"   1h        Up 1h
  [!] Kubelet API accessible at :10250 (unauthenticated)
{"kind":"PodList","items":[...]}`

		obs := nodeTool.Parse(multiVectorOutput, map[string]any{})

		findingKeys := make(map[string]bool)
		for _, o := range obs {
			findingKeys[o.Key] = true
		}

		expectedFindings := []string{
			"k8s:host_access",
			"k8s:kubelet_unauth",
		}

		for _, key := range expectedFindings {
			if !findingKeys[key] {
				t.Errorf("应检测到 %s", key)
			}
		}
	})
}

// TestCrossScenarioIntegration —— 跨场景集成测试。
func TestCrossScenarioIntegration(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	t.Run("云容器混合场景", func(t *testing.T) {
		// 场景: AWS ECS/EKS 环境
		services := map[string]bool{}
		activePacks := sm.Route(services)

		// 应激活 cloud (总是) 但不激活 container (需容器环境)
		hasCloud := false
		hasContainer := false
		for _, pack := range activePacks {
			if pack == "cloud" {
				hasCloud = true
			}
			if pack == "container" {
				hasContainer = true
			}
		}

		if !hasCloud {
			t.Error("云环境应激活 CloudPack")
		}
		// container 需真实容器环境，此处不应激活
		if hasContainer {
			t.Log("ContainerPack 已激活 (检测到容器环境)")
		}
	})

	t.Run("工具链协同", func(t *testing.T) {
		// 模拟攻击链: AWS IMDS → S3 枚举
		awsTool, _ := reg.Get("aws_imds_enum")
		s3Tool, _ := reg.Get("s3_bucket_enum")

		// Step 1: 从 IMDS 获取 IAM 凭证
		awsOutput := `AWS IMDS Enumeration:
  AccessKeyId: AKIATEST123
  SecretAccessKey: testsecret`

		awsObs := awsTool.Parse(awsOutput, map[string]any{})
		if len(awsObs) == 0 {
			t.Fatal("应提取 AWS 凭证")
		}

		// Step 2: 使用凭证枚举 S3 (模拟)
		s3Output := `S3 Bucket: company-backups
HTTP/1.1 200 OK
[!] Bucket is publicly accessible`

		s3Obs := s3Tool.Parse(s3Output, map[string]any{"bucket": "company-backups"})
		if len(s3Obs) == 0 {
			t.Fatal("应检测到公开 S3 bucket")
		}

		t.Logf("✓ 攻击链完成: AWS 凭证 → S3 枚举 → 发现公开 bucket")
	})
}
