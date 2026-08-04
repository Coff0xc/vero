// go:build browser
//go:build browser

package browser

// Package browser —— 浏览器自动化 Agent(抄 PentAGI browser agent):
// Playwright 驱动, 执行动态 Web 渗透(填表单/点击/截图/抓包),
// 突破纯 HTTP 工具(curl/nuclei)无法覆盖的 JS 渲染/CSRF/多步流程。
//
// 设计要点:
//   - 每个战役独立 browser context(隔离 cookie/缓存), 避免状态污染。
//   - 工具粒度: browser_navigate / browser_fill / browser_click / browser_screenshot。
//   - 证据采集: 截图(base64) + DOM 快照 + 网络日志, 逐字可溯源。
//   - 无头模式: 默认 headless, 调试时可开 headed。

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Agent —— 浏览器自动化 Agent 实例(生命周期绑定战役)。
type Agent struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	page    playwright.Page
	ctx     context.Context
}

// New —— 创建 browser agent(启动 Playwright + Chromium)。
// headless=false 时显示浏览器窗口(调试用)。
func New(ctx context.Context, headless bool) (*Agent, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("playwright 启动失败: %w", err)
	}
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: &headless,
	})
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("chromium 启动失败: %w", err)
	}
	page, err := browser.NewPage()
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, fmt.Errorf("page 创建失败: %w", err)
	}
	return &Agent{pw: pw, browser: browser, page: page, ctx: ctx}, nil
}

// Close —— 释放资源(关闭浏览器 + Playwright)。
func (a *Agent) Close() error {
	if a.page != nil {
		_ = a.page.Close()
	}
	if a.browser != nil {
		_ = a.browser.Close()
	}
	if a.pw != nil {
		return a.pw.Stop()
	}
	return nil
}

// Navigate —— 导航到目标 URL(等待页面加载完成)。
// 返回: 最终 URL(重定向后) + 页面标题。
func (a *Agent) Navigate(url string, timeout time.Duration) (string, string, error) {
	if _, err := a.page.Goto(url, playwright.PageGotoOptions{
		Timeout:       playwright.Float(float64(timeout.Milliseconds())),
		WaitUntil:     playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return "", "", fmt.Errorf("导航失败: %w", err)
	}
	finalURL := a.page.URL()
	title, _ := a.page.Title()
	return finalURL, title, nil
}

// Fill —— 填充表单字段(CSS 选择器定位)。
func (a *Agent) Fill(selector, value string) error {
	return a.page.Locator(selector).Fill(value)
}

// Click —— 点击元素(CSS 选择器定位)。
func (a *Agent) Click(selector string) error {
	return a.page.Locator(selector).Click()
}

// Screenshot —— 截取页面截图(返回 base64 编码的 PNG, 供证据存储)。
func (a *Agent) Screenshot() (string, error) {
	data, err := a.page.Screenshot()
	if err != nil {
		return "", fmt.Errorf("截图失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// Content —— 获取页面 HTML 源码(DOM 快照, 供 LLM 分析)。
func (a *Agent) Content() (string, error) {
	return a.page.Content()
}

// Evaluate —— 执行 JS 代码并返回结果(如提取 localStorage/cookie)。
func (a *Agent) Evaluate(script string) (interface{}, error) {
	return a.page.Evaluate(script)
}

// WaitForSelector —— 等待元素出现(动态加载/AJAX 场景)。
func (a *Agent) WaitForSelector(selector string, timeout time.Duration) error {
	_, err := a.page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
	return err
}
