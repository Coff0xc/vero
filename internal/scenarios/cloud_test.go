package scenarios

import (
	"strings"
	"testing"
)

// TestParseAWSIMDS —— 验证 AWS 元数据解析。
func TestParseAWSIMDS(t *testing.T) {
	output := `AWS IMDS Enumeration:
  instance-id: i-0abcd1234efgh5678
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

	obs := ParseAWSIMDS(output, map[string]any{})

	// 应提取 2 个观测: 1 个实例 ID + 1 个凭证
	if len(obs) != 2 {
		t.Fatalf("应提取 2 个观测, 实际 %d", len(obs))
	}

	// 验证实例 ID
	hasInstanceID := false
	hasCred := false

	for _, o := range obs {
		if o.Kind == "finding" && strings.Contains(o.Key, "i-0abcd1234efgh5678") {
			hasInstanceID = true
			if !strings.Contains(o.Label, "Instance ID") {
				t.Errorf("Label 应含实例 ID, 实际 %s", o.Label)
			}
		}

		if o.Kind == "cred" && strings.Contains(o.Label, "IAM credentials") {
			hasCred = true
			if !strings.Contains(o.Label, "critical") {
				t.Errorf("IAM 凭证应为 critical, 实际 %s", o.Label)
			}
		}
	}

	if !hasInstanceID {
		t.Error("应提取实例 ID")
	}
	if !hasCred {
		t.Error("应提取 IAM 凭证")
	}
}

// TestParseAzureIMDS —— 验证 Azure 元数据解析。
func TestParseAzureIMDS(t *testing.T) {
	output := `Azure IMDS Response:
{
  "vmId": "12345678-1234-1234-1234-123456789abc",
  "name": "myvm",
  "location": "eastus",
  "resourceGroupName": "myresourcegroup"
}`

	obs := ParseAzureIMDS(output, map[string]any{})

	// 应提取 1 个观测
	if len(obs) != 1 {
		t.Fatalf("应提取 1 个观测, 实际 %d", len(obs))
	}

	if obs[0].Kind != "finding" {
		t.Errorf("应为 finding 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "Azure VM") {
		t.Errorf("Label 应含 Azure VM, 实际 %s", obs[0].Label)
	}
}

// TestParseAzureIMDSWithToken —— 验证 Azure 管理身份 token。
func TestParseAzureIMDSWithToken(t *testing.T) {
	output := `{
  "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
  "expires_in": "3599",
  "resource": "https://management.azure.com/"
}`

	obs := ParseAzureIMDS(output, map[string]any{})

	// 应提取 1 个凭证
	if len(obs) != 1 {
		t.Fatalf("应提取 1 个凭证, 实际 %d", len(obs))
	}

	if obs[0].Kind != "cred" {
		t.Errorf("应为 cred 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "Managed Identity") {
		t.Errorf("Label 应含 Managed Identity, 实际 %s", obs[0].Label)
	}
}

// TestParseGCPIMDS —— 验证 GCP 元数据解析。
func TestParseGCPIMDS(t *testing.T) {
	output := `GCP Metadata Enumeration:
  instance/id: 1234567890123456789
  instance/hostname: myvm.c.myproject.internal
  instance/zone: projects/123456789/zones/us-central1-a
  project/project-id: myproject-123456
  instance/service-accounts/default/token: {"access_token":"ya29.c.test...","expires_in":3599}`

	obs := ParseGCPIMDS(output, map[string]any{})

	// 应提取 2 个观测: 1 个项目 ID + 1 个 token
	if len(obs) != 2 {
		t.Fatalf("应提取 2 个观测, 实际 %d", len(obs))
	}

	hasProjectID := false
	hasToken := false

	for _, o := range obs {
		if o.Kind == "finding" && strings.Contains(o.Key, "myproject-123456") {
			hasProjectID = true
		}
		if o.Kind == "cred" && strings.Contains(o.Label, "Service Account") {
			hasToken = true
		}
	}

	if !hasProjectID {
		t.Error("应提取项目 ID")
	}
	if !hasToken {
		t.Error("应提取服务账户 token")
	}
}

// TestParseS3Bucket —— 验证 S3 bucket 解析。
func TestParseS3Bucket(t *testing.T) {
	output := `S3 Bucket: mybucket
HTTP/1.1 200 OK
Content-Type: application/xml

[!] Bucket is publicly accessible (anonymous read)
[!] Bucket listing enabled
<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>...</ListBucketResult>`

	obs := ParseS3Bucket(output, map[string]any{"bucket": "mybucket"})

	// 应提取 2 个发现: 公开访问 + 列表泄露
	if len(obs) != 2 {
		t.Fatalf("应提取 2 个发现, 实际 %d", len(obs))
	}

	hasPublic := false
	hasListing := false

	for _, o := range obs {
		if o.Kind != "finding" {
			t.Errorf("应为 finding 类型, 实际 %s", o.Kind)
		}

		if strings.Contains(o.Label, "publicly accessible") {
			hasPublic = true
			if !strings.Contains(o.Label, "critical") {
				t.Errorf("公开访问应为 critical, 实际 %s", o.Label)
			}
		}

		if strings.Contains(o.Label, "listing enabled") {
			hasListing = true
		}
	}

	if !hasPublic {
		t.Error("应检测到公开访问")
	}
	if !hasListing {
		t.Error("应检测到列表泄露")
	}
}

// TestParseS3BucketPrivate —— 验证私有 bucket。
func TestParseS3BucketPrivate(t *testing.T) {
	output := `S3 Bucket: privatebucket
HTTP/1.1 403 Forbidden

[+] Bucket exists but access denied (private)`

	obs := ParseS3Bucket(output, map[string]any{"bucket": "privatebucket"})

	// 私有 bucket 不应产生观测
	if len(obs) != 0 {
		t.Errorf("私有 bucket 不应产生观测, 实际 %d", len(obs))
	}
}

// TestCloudPack —— 验证场景包注册。
func TestCloudPack(t *testing.T) {
	pack := CloudPack()

	if pack.Name != "cloud" {
		t.Errorf("包名应为 cloud, 实际 %s", pack.Name)
	}

	// 应有 4 个工具
	if len(pack.Tools) != 4 {
		t.Fatalf("应有 4 个工具, 实际 %d", len(pack.Tools))
	}

	// 验证工具存在
	toolNames := make(map[string]bool)
	for _, tool := range pack.Tools {
		toolNames[tool.Name] = true
	}

	required := []string{"aws_imds_enum", "azure_imds_enum", "gcp_imds_enum", "s3_bucket_enum"}
	for _, name := range required {
		if !toolNames[name] {
			t.Errorf("缺失工具: %s", name)
		}
	}

	// 云环境无标准服务指纹(IMDS 在元数据地址而非扫描服务): Fingerprint 应恒 false,
	// 但工具始终注册可用(LLM 可按需调用, 工具内部快速判非云环境)。
	if pack.Fingerprint(map[string]bool{}) {
		t.Error("云包 Fingerprint 不应在无服务指纹时激活(误导路由展示)")
	}
}
