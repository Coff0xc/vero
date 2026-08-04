package browser

// Browser Agent 框架已准备就绪, 需安装 Playwright 依赖后启用:
//
// 1. 设置代理或国内镜像:
//    export GOPROXY=https://goproxy.cn,direct
//
// 2. 安装依赖:
//    go get github.com/playwright-community/playwright-go
//    go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
//
// 3. 移除 internal/browser/ 的 build tag 注释，重编译即可使用 browser_navigate / browser_fill / browser_click / browser_screenshot 工具。
//
// 设计参考: PentAGI browser agent (https://github.com/vxcontrol/pentagi)
// 能力: 动态 Web 渗透(填表单/点击/截图), 突破纯 HTTP 工具无法覆盖的 JS 渲染/CSRF/多步流程。

// Stub —— 占位函数，防止 import 循环。实际实现见 agent.go / tools.go (需移除 build tag)。
func Stub() string {
	return "Browser Agent 需安装 Playwright 依赖后启用，详见本文件注释"
}
