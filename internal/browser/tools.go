// go:build browser
//go:build browser

package browser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// 工具适配器: 把 browser agent 的能力暴露为标准工具(供 LLM 调用)。

// browserNavigate —— 工具: 浏览器导航到目标 URL。
func browserNavigate(args map[string]any) tools.ToolResult {
	url := tools.ArgStr(args, "url", "")
	if url == "" {
		return tools.ToolResult{Success: false, Stderr: "browser_navigate: 缺 url", RC: -1}
	}
	ag, err := New(context.Background(), true) // headless=true
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	defer ag.Close()

	finalURL, title, err := ag.Navigate(url, 30*time.Second)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	stdout := fmt.Sprintf("导航成功: %s\n标题: %s\n最终 URL: %s", url, title, finalURL)
	return tools.ToolResult{Success: true, Stdout: stdout}
}

// browserFill —— 工具: 填充表单字段。
func browserFill(args map[string]any) tools.ToolResult {
	url := tools.ArgStr(args, "url", "")
	selector := tools.ArgStr(args, "selector", "")
	value := tools.ArgStr(args, "value", "")
	if url == "" || selector == "" {
		return tools.ToolResult{Success: false, Stderr: "browser_fill: 缺 url 或 selector", RC: -1}
	}
	ag, err := New(context.Background(), true)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	defer ag.Close()

	if _, _, err := ag.Navigate(url, 30*time.Second); err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	if err := ag.Fill(selector, value); err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	return tools.ToolResult{Success: true, Stdout: fmt.Sprintf("已填充 %s = %s", selector, value)}
}

// browserClick —— 工具: 点击元素。
func browserClick(args map[string]any) tools.ToolResult {
	url := tools.ArgStr(args, "url", "")
	selector := tools.ArgStr(args, "selector", "")
	if url == "" || selector == "" {
		return tools.ToolResult{Success: false, Stderr: "browser_click: 缺 url 或 selector", RC: -1}
	}
	ag, err := New(context.Background(), true)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	defer ag.Close()

	if _, _, err := ag.Navigate(url, 30*time.Second); err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	if err := ag.Click(selector); err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	// 点击后等待页面稳定
	time.Sleep(2 * time.Second)
	finalURL := ag.page.URL()
	return tools.ToolResult{Success: true, Stdout: fmt.Sprintf("已点击 %s, 当前 URL: %s", selector, finalURL)}
}

// browserScreenshot —— 工具: 截图(base64 编码, 证据留存)。
func browserScreenshot(args map[string]any) tools.ToolResult {
	url := tools.ArgStr(args, "url", "")
	if url == "" {
		return tools.ToolResult{Success: false, Stderr: "browser_screenshot: 缺 url", RC: -1}
	}
	ag, err := New(context.Background(), true)
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	defer ag.Close()

	if _, _, err := ag.Navigate(url, 30*time.Second); err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	b64, err := ag.Screenshot()
	if err != nil {
		return tools.ToolResult{Success: false, Stderr: err.Error(), RC: -1}
	}
	// 截图 base64 太长(几百 KB), stdout 只输出摘要, 完整 base64 存 parser 提取的证据
	return tools.ToolResult{Success: true, Stdout: fmt.Sprintf("截图成功: %s (长度 %d 字节)", url, len(b64))}
}

// ParseBrowserNav —— 解析导航结果: 提取重定向/标题作为观察。
func ParseBrowserNav(stdout string, args map[string]any) []tools.Observation {
	url := tools.ArgStr(args, "url", "?")
	var obs []tools.Observation
	if strings.Contains(stdout, "最终 URL") {
		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			if strings.Contains(line, "最终 URL") || strings.Contains(line, "标题") {
				obs = append(obs, tools.Observation{
					Kind:    "endpoint",
					Key:     url + ":browser_nav",
					Label:   "[info] 浏览器导航: " + strings.TrimSpace(line),
					Excerpt: line,
				})
			}
		}
	}
	return obs
}

// BrowserPack —— 浏览器自动化场景包。
func BrowserPack() tools.Tool {
	return tools.Tool{
		Name:  "browser",
		Level: tools.LevelScan,
		Desc:  "浏览器自动化工具包(Playwright), 执行动态 Web 操作: 导航/填表/点击/截图",
		Run: func(args map[string]any) tools.ToolResult {
			action := tools.ArgStr(args, "action", "")
			switch action {
			case "navigate":
				return browserNavigate(args)
			case "fill":
				return browserFill(args)
			case "click":
				return browserClick(args)
			case "screenshot":
				return browserScreenshot(args)
			default:
				return tools.ToolResult{Success: false, Stderr: "browser: 未知 action, 可选: navigate/fill/click/screenshot", RC: -1}
			}
		},
		Parse: nil, // 暂不解析(截图 base64 不适合 parser)
		Args: []tools.ArgSpec{
			{Name: "action", Desc: "操作类型: navigate/fill/click/screenshot", Required: true},
			{Name: "url", Desc: "目标 URL", Required: true},
			{Name: "selector", Desc: "CSS 选择器(fill/click 用), 如 input[name=username]"},
			{Name: "value", Desc: "填充值(fill 用)"},
		},
	}
}

// RegisterBrowserTools —— 注册为 4 个独立工具(供 LLM 分别调用, 比统一 action 参数更清晰)。
func RegisterBrowserTools(reg *tools.Registry) {
	reg.Register(&tools.Tool{
		Name: "browser_navigate", Level: tools.LevelScan,
		Desc: "浏览器导航到目标 URL, 等待页面加载完成(JS 渲染/重定向), 返回最终 URL 与标题",
		Run: browserNavigate, Parse: ParseBrowserNav,
		Args: []tools.ArgSpec{{Name: "url", Desc: "目标 URL(带 scheme)", Required: true}},
	})
	reg.Register(&tools.Tool{
		Name: "browser_fill", Level: tools.LevelScan,
		Desc: "浏览器填充表单字段(CSS 选择器定位), 用于登录/注册/搜索等交互",
		Run: browserFill,
		Args: []tools.ArgSpec{
			{Name: "url", Desc: "目标 URL", Required: true},
			{Name: "selector", Desc: "CSS 选择器, 如 input[name=username]", Required: true},
			{Name: "value", Desc: "填充值", Required: true},
		},
	})
	reg.Register(&tools.Tool{
		Name: "browser_click", Level: tools.LevelScan,
		Desc: "浏览器点击元素(按钮/链接), 触发跳转/提交, 返回点击后 URL",
		Run: browserClick,
		Args: []tools.ArgSpec{
			{Name: "url", Desc: "目标 URL", Required: true},
			{Name: "selector", Desc: "CSS 选择器, 如 button[type=submit]", Required: true},
		},
	})
	reg.Register(&tools.Tool{
		Name: "browser_screenshot", Level: tools.LevelScan,
		Desc: "浏览器截图(base64 PNG), 用于证据留存/验证渲染结果",
		Run: browserScreenshot,
		Args: []tools.ArgSpec{{Name: "url", Desc: "目标 URL", Required: true}},
	})
}
