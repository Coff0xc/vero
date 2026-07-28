package scenarios

import (
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// ---------- 云环境侦察包 (Cloud) ----------

// awsIMDSEnum —— AWS EC2 实例元数据服务枚举 (IMDSv1/v2)。
// 提取实例 ID、角色凭证、安全组等敏感信息。
func awsIMDSEnum(args map[string]any) tools.ToolResult {
	baseURL := "http://169.254.169.254/latest/meta-data/"

	// 尝试 IMDSv1 (直接访问)
	endpoints := []string{
		"instance-id",
		"hostname",
		"local-ipv4",
		"public-ipv4",
		"iam/security-credentials/",
		"placement/availability-zone",
	}

	var output strings.Builder
	output.WriteString("AWS IMDS Enumeration:\n")

	for _, ep := range endpoints {
		url := baseURL + ep
		result := tools.Sh([]string{"curl", "-s", "--max-time", "2", url}, 5*time.Second)
		if result.Success && result.Stdout != "" {
			output.WriteString(fmt.Sprintf("  %s: %s\n", ep, strings.TrimSpace(result.Stdout)))

			// 如果是 IAM 角色, 进一步获取凭证
			if ep == "iam/security-credentials/" && result.Stdout != "" {
				roleName := strings.TrimSpace(result.Stdout)
				credURL := baseURL + "iam/security-credentials/" + roleName
				credResult := tools.Sh([]string{"curl", "-s", "--max-time", "2", credURL}, 5*time.Second)
				if credResult.Success {
					output.WriteString(fmt.Sprintf("  IAM Credentials (%s):\n%s\n", roleName, credResult.Stdout))
				}
			}
		}
	}

	stdout := output.String()
	if strings.Contains(stdout, "instance-id") {
		return tools.ToolResult{Success: true, Stdout: stdout}
	}

	return tools.ToolResult{Success: false, Stderr: "Not running in AWS EC2 or IMDS blocked", RC: 1}
}

// azureIMDSEnum —— Azure 实例元数据服务枚举。
func azureIMDSEnum(args map[string]any) tools.ToolResult {
	url := "http://169.254.169.254/metadata/instance?api-version=2021-02-01"

	// Azure IMDS 需要 Metadata: true 头
	result := tools.Sh([]string{"curl", "-s", "-H", "Metadata: true", "--max-time", "2", url}, 5*time.Second)

	if result.Success && result.Stdout != "" {
		return tools.ToolResult{Success: true, Stdout: "Azure IMDS Response:\n" + result.Stdout}
	}

	return tools.ToolResult{Success: false, Stderr: "Not running in Azure or IMDS blocked", RC: 1}
}

// gcpIMDSEnum —— GCP 实例元数据服务枚举。
func gcpIMDSEnum(args map[string]any) tools.ToolResult {
	baseURL := "http://metadata.google.internal/computeMetadata/v1/"

	endpoints := []string{
		"instance/id",
		"instance/hostname",
		"instance/zone",
		"project/project-id",
		"instance/service-accounts/default/token",
	}

	var output strings.Builder
	output.WriteString("GCP Metadata Enumeration:\n")

	for _, ep := range endpoints {
		url := baseURL + ep
		// GCP 需要 Metadata-Flavor: Google 头
		result := tools.Sh([]string{"curl", "-s", "-H", "Metadata-Flavor: Google", "--max-time", "2", url}, 5*time.Second)
		if result.Success && result.Stdout != "" {
			output.WriteString(fmt.Sprintf("  %s: %s\n", ep, strings.TrimSpace(result.Stdout)))
		}
	}

	stdout := output.String()
	if strings.Contains(stdout, "instance/id") || strings.Contains(stdout, "project-id") {
		return tools.ToolResult{Success: true, Stdout: stdout}
	}

	return tools.ToolResult{Success: false, Stderr: "Not running in GCP or metadata blocked", RC: 1}
}

// s3BucketEnum —— S3 bucket 公开访问检测。
// 尝试列出 bucket 内容, 检测权限配置错误。
func s3BucketEnum(args map[string]any) tools.ToolResult {
	bucket := tools.ArgStr(args, "bucket", "")
	if bucket == "" {
		return tools.ToolResult{Success: false, Stderr: "s3_bucket_enum: 缺 bucket", RC: -1}
	}

	// 尝试匿名访问
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/", bucket)
	result := tools.Sh([]string{"curl", "-s", "-I", url}, 10*time.Second)

	var output strings.Builder
	output.WriteString(fmt.Sprintf("S3 Bucket: %s\n", bucket))

	if result.Success {
		stdout := result.Stdout
		output.WriteString(stdout)

		// 检测公开访问
		if strings.Contains(stdout, "200 OK") {
			output.WriteString("\n[!] Bucket is publicly accessible (anonymous read)\n")

			// 尝试列出对象
			listResult := tools.Sh([]string{"curl", "-s", url}, 10*time.Second)
			if listResult.Success && strings.Contains(listResult.Stdout, "<ListBucketResult>") {
				output.WriteString("[!] Bucket listing enabled\n")
				output.WriteString(listResult.Stdout[:500] + "...\n")
			}
		} else if strings.Contains(stdout, "403 Forbidden") {
			output.WriteString("\n[+] Bucket exists but access denied (private)\n")
		} else if strings.Contains(stdout, "404 Not Found") {
			output.WriteString("\n[-] Bucket does not exist\n")
		}
	}

	return tools.ToolResult{Success: true, Stdout: output.String()}
}

// ---------- Parsers ----------

// ParseAWSIMDS —— 解析 AWS 元数据, 提取实例信息和凭证。
func ParseAWSIMDS(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	foundCred := false

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 提取实例 ID
		if strings.Contains(line, "instance-id:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				instanceID := strings.TrimSpace(parts[1])
				obs = append(obs, tools.Observation{
					Kind:    "finding",
					Key:     "aws:instance:" + instanceID,
					Label:   fmt.Sprintf("[info] AWS Instance ID: %s", instanceID),
					Excerpt: line,
				})
			}
		}

		// 检测 IAM 凭证泄露 (只记录一次)
		if !foundCred && (strings.Contains(line, "AccessKeyId") || strings.Contains(line, "SecretAccessKey")) {
			obs = append(obs, tools.Observation{
				Kind:    "cred",
				Key:     "aws:iam:credentials",
				Label:   "[critical] AWS IAM credentials exposed via IMDS",
				Excerpt: "AccessKeyId/SecretAccessKey",
			})
			foundCred = true
		}
	}

	return obs
}

// ParseAzureIMDS —— 解析 Azure 元数据。
func ParseAzureIMDS(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	// Azure 返回 JSON, 简化: 检测是否包含 vmId
	if strings.Contains(stdout, "vmId") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "azure:vm:metadata",
			Label:   "[info] Running in Azure VM",
			Excerpt: "vmId",
		})
	}

	// 检测管理身份凭证
	if strings.Contains(stdout, "access_token") {
		obs = append(obs, tools.Observation{
			Kind:    "cred",
			Key:     "azure:managed_identity",
			Label:   "[high] Azure Managed Identity token accessible",
			Excerpt: "access_token",
		})
	}

	return obs
}

// ParseGCPIMDS —— 解析 GCP 元数据。
func ParseGCPIMDS(stdout string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 提取项目 ID
		if strings.Contains(line, "project-id:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				projectID := strings.TrimSpace(parts[1])
				obs = append(obs, tools.Observation{
					Kind:    "finding",
					Key:     "gcp:project:" + projectID,
					Label:   fmt.Sprintf("[info] GCP Project ID: %s", projectID),
					Excerpt: line,
				})
			}
		}

		// 检测服务账户 token
		if strings.Contains(line, "service-accounts") && strings.Contains(line, "token") {
			obs = append(obs, tools.Observation{
				Kind:    "cred",
				Key:     "gcp:service_account:token",
				Label:   "[high] GCP Service Account token accessible",
				Excerpt: "service-accounts/default/token",
			})
		}
	}

	return obs
}

// ParseS3Bucket —— 解析 S3 bucket 访问结果。
func ParseS3Bucket(stdout string, args map[string]any) []tools.Observation {
	bucket := tools.ArgStr(args, "bucket", "?")
	var obs []tools.Observation

	// 检测公开访问
	if strings.Contains(stdout, "publicly accessible") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "s3:" + bucket + ":public",
			Label:   fmt.Sprintf("[critical] S3 bucket %s is publicly accessible", bucket),
			Excerpt: "publicly accessible",
		})
	}

	// 检测 bucket 列表泄露
	if strings.Contains(stdout, "Bucket listing enabled") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "s3:" + bucket + ":listing",
			Label:   fmt.Sprintf("[high] S3 bucket %s listing enabled", bucket),
			Excerpt: "Bucket listing enabled",
		})
	}

	return obs
}

// CloudPack —— 云环境侦察场景包。
func CloudPack() Pack {
	return Pack{
		Name: "cloud",
		Tools: []*tools.Tool{
			{Name: "aws_imds_enum", Level: tools.LevelRecon,
				Desc: "AWS EC2 实例元数据服务枚举(IMDS), 提取实例 ID/IAM 凭证/安全组, 仅在 AWS 环境有效",
				Run: awsIMDSEnum, Parse: ParseAWSIMDS},
			{Name: "azure_imds_enum", Level: tools.LevelRecon,
				Desc: "Azure 实例元数据服务枚举, 提取 VM 信息/管理身份 token, 仅在 Azure 环境有效",
				Run: azureIMDSEnum, Parse: ParseAzureIMDS},
			{Name: "gcp_imds_enum", Level: tools.LevelRecon,
				Desc: "GCP 实例元数据服务枚举, 提取项目 ID/服务账户 token, 仅在 GCP 环境有效",
				Run: gcpIMDSEnum, Parse: ParseGCPIMDS},
			{Name: "s3_bucket_enum", Level: tools.LevelScan,
				Desc: "S3 bucket 公开访问检测, 检测权限配置错误/数据泄露, 需 bucket 参数",
				Run: s3BucketEnum, Parse: ParseS3Bucket},
		},
		Fingerprint: func(s map[string]bool) bool {
			// 云环境场景: 自动激活(侦察阶段始终尝试)
			return true
		},
	}
}
