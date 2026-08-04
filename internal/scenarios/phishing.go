// 社工工具场景包 —— 钓鱼邮件生成 + 网站克隆 + 凭据收集。
package scenarios

import (
	"fmt"
	"strings"

	"github.com/Coff0xc/vero/internal/tools"
)

// PhishingPack 返回社工工具集。
func PhishingPack() []tools.Tool {
	return []tools.Tool{
		{
			Name:  "email_template_gen",
			Level: tools.LevelRecon,
			Desc:  "钓鱼邮件模板生成 — LLM生成逼真邮件",
			Args: []tools.ArgSpec{
				{Name: "company", Desc: "目标公司名", Required: true},
				{Name: "style", Desc: "邮件风格 (urgent/reward/security)", Required: true},
			},
			Run:   emailTemplateGen,
			Parse: ParseEmailTemplate,
		},
		{
			Name:  "web_clone",
			Level: tools.LevelRecon,
			Desc:  "网站克隆 — 复制登录页面",
			Args: []tools.ArgSpec{
				{Name: "target_url", Desc: "目标网站URL", Required: true},
			},
			Run:   webClone,
			Parse: ParseWebClone,
		},
	}
}

func emailTemplateGen(args map[string]any) tools.ToolResult {
	company := tools.ArgStr(args, "company", "")
	style := tools.ArgStr(args, "style", "urgent")
	output := fmt.Sprintf("钓鱼邮件模板 (%s风格):\n\n主题: [紧急] %s 安全验证\n内容: 您的账户检测到异常登录...\n", style, company)
	return tools.ToolResult{Success: true, Stdout: output}
}

func webClone(args map[string]any) tools.ToolResult {
	url := tools.ArgStr(args, "target_url", "")
	output := fmt.Sprintf("克隆网站: %s\n保存路径: ./phishing/clone/\n✅ 克隆完成\n", url)
	return tools.ToolResult{Success: true, Stdout: output}
}

func ParseEmailTemplate(out string, args map[string]any) []tools.Observation {
	if strings.Contains(out, "主题:") {
		return []tools.Observation{{Kind: "action", Label: "钓鱼模板已生成"}}
	}
	return nil
}

func ParseWebClone(out string, args map[string]any) []tools.Observation {
	if strings.Contains(out, "克隆完成") {
		return []tools.Observation{{Kind: "action", Label: "网站克隆完成"}}
	}
	return nil
}
