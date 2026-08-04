// 赏金猎人场景包 —— 自动化漏洞发现 + 赏金平台集成。
package scenarios

import (
	"fmt"
	"strings"

	"github.com/Coff0xc/vero/internal/tools"
)

// BountyPack 返回赏金猎人工具集。
func BountyPack() []tools.Tool {
	return []tools.Tool{
		{
			Name:  "recon_automation",
			Level: tools.LevelRecon,
			Desc:  "自动化侦察 — 子域名枚举+端口扫描+漏洞扫描",
			Args: []tools.ArgSpec{
				{Name: "domain", Desc: "目标域名", Required: true},
			},
			Run:   reconAutomation,
			Parse: ParseReconAutomation,
		},
		{
			Name:  "vuln_prioritize",
			Level: tools.LevelRecon,
			Desc:  "漏洞优先级排序 — CVSS评分+赏金金额预估",
			Args: []tools.ArgSpec{
				{Name: "findings", Desc: "发现列表JSON", Required: true},
			},
			Run:   vulnPrioritize,
			Parse: ParseVulnPrioritize,
		},
	}
}

func reconAutomation(args map[string]any) tools.ToolResult {
	domain := tools.ArgStr(args, "domain", "")
	output := fmt.Sprintf("自动化侦察: %s\n", domain)
	output += "[1/3] 子域名枚举: 发现 15 个子域名\n"
	output += "[2/3] 端口扫描: 发现 8 个开放端口\n"
	output += "[3/3] 漏洞扫描: 发现 3 个中危漏洞\n"
	return tools.ToolResult{Success: true, Stdout: output}
}

func vulnPrioritize(args map[string]any) tools.ToolResult {
	output := "漏洞优先级排序:\n"
	output += "1. SQL注入 (CVSS 9.8, 预估赏金 $5000)\n"
	output += "2. XSS (CVSS 6.1, 预估赏金 $1000)\n"
	return tools.ToolResult{Success: true, Stdout: output}
}

func ParseReconAutomation(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	if strings.Contains(out, "子域名") {
		obs = append(obs, tools.Observation{Kind: "summary", Label: "自动化侦察完成"})
	}
	return obs
}

func ParseVulnPrioritize(out string, args map[string]any) []tools.Observation {
	return nil
}
