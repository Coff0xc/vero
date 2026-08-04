package scenarios

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// TestCloudPackEnhanced —— 验证云渗透增强包。
func TestCloudPackEnhanced(t *testing.T) {
	reg := tools.NewRegistry()
	m := NewManager()
	m.Register(reg, CloudPackEnhanced())

	// 1) 验证工具注册
	expectedTools := []string{"aws_s3_enum", "aws_iam_privesc", "azure_tenant_enum", "gcp_project_enum", "cloud_metadata_exploit"}
	for _, name := range expectedTools {
		if !reg.Has(name) {
			t.Errorf("工具 %s 未注册", name)
		}
	}

	// 2) 验证 Args 规格
	s3Tool, _ := reg.Get("aws_s3_enum")
	if len(s3Tool.Args) < 1 || s3Tool.Args[0].Name != "profile" {
		t.Error("aws_s3_enum 缺少 profile 参数规格")
	}

	metadataTool, _ := reg.Get("cloud_metadata_exploit")
	if len(metadataTool.Args) < 1 || metadataTool.Args[0].Name != "target" {
		t.Error("cloud_metadata_exploit 缺少 target 参数规格")
	}

	// 3) 验证 Parser (S3)
	mockS3Output := `✓ 私有桶: secure-bucket
✗ 公开桶: public-data (ACL 含 AllUsers)
✗ 公开桶: logs-backup (ACL 含 AllUsers)
✓ 私有桶: internal-files`

	obs := ParseCloudS3(mockS3Output, map[string]any{})
	if len(obs) != 2 {
		t.Fatalf("ParseCloudS3 应解析 2 个公开桶, got %d", len(obs))
	}
	if obs[0].Severity != "high" {
		t.Errorf("S3 公开桶应为 high 严重度, got %s", obs[0].Severity)
	}

	// 4) 验证 Parser (IAM Privesc)
	mockPrivescOutput := `当前身份: arn:aws:iam::123456789012:user/alice
⚠ 危险权限: iam:AttachUserPolicy
⚠ 危险权限: iam:PassRole`

	obsPriv := ParseCloudPrivesc(mockPrivescOutput, map[string]any{})
	if len(obsPriv) != 2 {
		t.Fatalf("ParseCloudPrivesc 应解析 2 个危险权限, got %d", len(obsPriv))
	}
	if obsPriv[0].Severity != "critical" {
		t.Errorf("IAM 提权路径应为 critical, got %s", obsPriv[0].Severity)
	}
	if obsPriv[0].Technique != "T1078" {
		t.Errorf("IAM 提权应映射 T1078, got %s", obsPriv[0].Technique)
	}

	// 5) 验证 Parser (Metadata)
	mockMetadataOutput := `云元数据服务利用:
✓ AWS IMDS 可访问 (IMDSv1)
  IAM Role: ec2-admin-role
  ⚠ 凭证已泄露 (包含 AccessKeyId)`

	obsMeta := ParseCloudMetadata(mockMetadataOutput, map[string]any{})
	if len(obsMeta) != 2 {
		t.Fatalf("ParseCloudMetadata 应解析 2 个观察, got %d", len(obsMeta))
	}

	var credLeakFound bool
	for _, o := range obsMeta {
		if o.Key == "cloud-metadata:cred-leak" {
			credLeakFound = true
			if o.Severity != "critical" {
				t.Errorf("凭证泄露应为 critical, got %s", o.Severity)
			}
			if o.Technique != "T1552.005" {
				t.Errorf("元数据凭证应映射 T1552.005, got %s", o.Technique)
			}
		}
	}
	if !credLeakFound {
		t.Error("未找到凭证泄露观察")
	}

	// 6) 验证指纹函数
	pack := CloudPackEnhanced()
	if !pack.Fingerprint(map[string]bool{"aws": true}) {
		t.Error("CloudPackEnhanced 应对 aws 服务指纹激活")
	}
	if !pack.Fingerprint(map[string]bool{"cloud": true}) {
		t.Error("CloudPackEnhanced 应对 cloud 服务指纹激活")
	}
	if pack.Fingerprint(map[string]bool{"http": true}) {
		t.Error("CloudPackEnhanced 不应对纯 http 激活")
	}
}

// TestExtractUserName —— 测试 ARN 用户名提取。
func TestExtractUserName(t *testing.T) {
	testCases := []struct {
		arn      string
		expected string
	}{
		{"arn:aws:iam::123456789012:user/alice", "alice"},
		{"arn:aws:iam::123456789012:user/ops/bob", "bob"},
		{"arn:aws:sts::123456789012:assumed-role/role-name/session", "session"},
		{"invalid-arn", "invalid-arn"},
		{"", ""},
	}

	for _, tc := range testCases {
		got := extractUserName(tc.arn)
		if got != tc.expected {
			t.Errorf("extractUserName(%s) = %s, want %s", tc.arn, got, tc.expected)
		}
	}
}

// TestCloudParserEdgeCases —— 测试 Parser 边界情况。
func TestCloudParserEdgeCases(t *testing.T) {
	// 1) 空输出
	obs := ParseCloudS3("", map[string]any{})
	if len(obs) != 0 {
		t.Error("空输出应返回 0 个观察")
	}

	// 2) 无公开桶
	obs = ParseCloudS3("✓ 私有桶: bucket1\n✓ 私有桶: bucket2", map[string]any{})
	if len(obs) != 0 {
		t.Error("无公开桶应返回 0 个观察")
	}

	// 3) 无危险权限
	obs = ParseCloudPrivesc("当前身份: arn:aws:iam::123:user/test\n策略: ReadOnlyAccess", map[string]any{})
	if len(obs) != 0 {
		t.Error("无危险权限应返回 0 个观察")
	}

	// 4) 元数据不可达
	obs = ParseCloudMetadata("云元数据服务利用:\n✗ 元数据服务不可达", map[string]any{})
	if len(obs) != 0 {
		t.Error("元数据不可达应返回 0 个观察")
	}

	// 5) 元数据可访问但无凭证泄露
	obs = ParseCloudMetadata("云元数据服务利用:\n✓ AWS IMDS 可访问 (IMDSv1)", map[string]any{})
	if len(obs) != 1 || obs[0].Severity != "medium" {
		t.Error("元数据可访问但无凭证应为 medium 严重度")
	}
}
