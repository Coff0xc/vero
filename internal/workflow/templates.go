// Package workflow —— 预定义渗透测试工作流模板
package workflow

import (
	"github.com/Coff0xc/vero/internal/tools"
)

// Stage —— 工作流阶段
type Stage struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`       // 工具名列表
	Sequential  bool     `json:"sequential"`  // 是否顺序执行（false=并行）
	Critical    bool     `json:"critical"`    // 是否关键阶段（失败则停止）
}

// Template —— 工作流模板
type Template struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"` // web/ad/cloud/container
	Stages      []Stage  `json:"stages"`
	Tags        []string `json:"tags"`
}

// Templates —— 所有预定义模板
var Templates = []Template{
	WebReconTemplate(),
	WebFullTemplate(),
	ADReconTemplate(),
	CloudReconTemplate(),
	ContainerEscapeTemplate(),
}

// WebReconTemplate —— Web 应用侦察工作流
func WebReconTemplate() Template {
	return Template{
		ID:          "web-recon",
		Name:        "Web 应用侦察",
		Description: "HTTP 指纹 → 漏洞扫描 → 目录爆破",
		Category:    "web",
		Tags:        []string{"web", "recon", "safe"},
		Stages: []Stage{
			{
				Name:        "HTTP 指纹",
				Description: "探测 Web 服务器技术栈",
				Tools:       []string{"http_probe"},
				Sequential:  true,
				Critical:    true,
			},
			{
				Name:        "漏洞扫描",
				Description: "Nuclei 扫描已知漏洞",
				Tools:       []string{"web_vuln_scan"},
				Sequential:  false,
				Critical:    false,
			},
			{
				Name:        "目录枚举",
				Description: "发现隐藏路径和敏感文件",
				Tools:       []string{"ffuf_dir_brute"},
				Sequential:  false,
				Critical:    false,
			},
		},
	}
}

// WebFullTemplate —— Web 应用完整渗透
func WebFullTemplate() Template {
	return Template{
		ID:          "web-full",
		Name:        "Web 应用完整渗透",
		Description: "侦察 → 扫描 → 利用 → 后渗透",
		Category:    "web",
		Tags:        []string{"web", "full", "exploit"},
		Stages: []Stage{
			{
				Name:        "侦察",
				Description: "被动信息收集",
				Tools:       []string{"http_probe"},
				Sequential:  true,
				Critical:    true,
			},
			{
				Name:        "扫描",
				Description: "主动漏洞发现",
				Tools:       []string{"web_vuln_scan", "ffuf_dir_brute"},
				Sequential:  false,
				Critical:    false,
			},
			{
				Name:        "利用",
				Description: "尝试已知漏洞利用",
				Tools:       []string{"exploit_sqli"},
				Sequential:  true,
				Critical:    false,
			},
		},
	}
}

// ADReconTemplate —— Active Directory 侦察
func ADReconTemplate() Template {
	return Template{
		ID:          "ad-recon",
		Name:        "Active Directory 侦察",
		Description: "SMB 枚举 → LDAP 查询 → 用户枚举",
		Category:    "ad",
		Tags:        []string{"ad", "recon", "internal"},
		Stages: []Stage{
			{
				Name:        "SMB 枚举",
				Description: "发现域内主机",
				Tools:       []string{"smb_enum"},
				Sequential:  true,
				Critical:    true,
			},
			{
				Name:        "LDAP 枚举",
				Description: "查询域对象和计算机",
				Tools:       []string{"nxc_ldap_enum", "nxc_ldap_computers"},
				Sequential:  false,
				Critical:    false,
			},
			{
				Name:        "凭证攻击",
				Description: "尝试获取凭证",
				Tools:       []string{"nxc_asrep", "kerberoast"},
				Sequential:  true,
				Critical:    false,
			},
		},
	}
}

// CloudReconTemplate —— 云环境侦察
func CloudReconTemplate() Template {
	return Template{
		ID:          "cloud-recon",
		Name:        "云环境侦察",
		Description: "IMDS 元数据提取 → S3 枚举",
		Category:    "cloud",
		Tags:        []string{"cloud", "aws", "azure", "gcp"},
		Stages: []Stage{
			{
				Name:        "元数据提取",
				Description: "从 IMDS 获取实例信息",
				Tools:       []string{"aws_imds_enum", "azure_imds_enum", "gcp_imds_enum"},
				Sequential:  false,
				Critical:    false,
			},
			{
				Name:        "存储枚举",
				Description: "查找公开访问的存储桶",
				Tools:       []string{"s3_bucket_enum"},
				Sequential:  false,
				Critical:    false,
			},
		},
	}
}

// ContainerEscapeTemplate —— 容器逃逸
func ContainerEscapeTemplate() Template {
	return Template{
		ID:          "container-escape",
		Name:        "容器逃逸检测",
		Description: "Docker 逃逸 → K8s ServiceAccount 提取",
		Category:    "container",
		Tags:        []string{"container", "docker", "kubernetes"},
		Stages: []Stage{
			{
				Name:        "容器检测",
				Description: "确认容器环境和逃逸向量",
				Tools:       []string{"docker_escape_check"},
				Sequential:  true,
				Critical:    true,
			},
			{
				Name:        "K8s 枚举",
				Description: "提取 ServiceAccount 凭证",
				Tools:       []string{"k8s_sa_enum"},
				Sequential:  true,
				Critical:    false,
			},
		},
	}
}

// GetByID —— 根据 ID 获取模板
func GetByID(id string) *Template {
	for _, t := range Templates {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

// GetByCategory —— 根据分类获取模板
func GetByCategory(category string) []Template {
	result := []Template{}
	for _, t := range Templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// ValidateTemplate —— 验证模板中的工具是否已注册
func ValidateTemplate(t Template, reg *tools.Registry) []string {
	missing := []string{}
	for _, stage := range t.Stages {
		for _, toolName := range stage.Tools {
			if !reg.Has(toolName) {
				missing = append(missing, toolName)
			}
		}
	}
	return missing
}
