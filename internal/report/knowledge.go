package report

import "strings"

// vulnRemediation —— 漏洞关键词 → 描述/修复/建议(单一事实来源, 修 D14)。
// recommendations / generateDescription / generateRemediation 三处共享,
// 避免各自硬编码导致覆盖面不一致。
type vulnRemediation struct {
	keywords       []string // 小写匹配关键词
	description    string   // 漏洞描述
	remediation    string   // 修复措施
	recommendation string   // 报告"修复建议"条目
}

var vulnRemediations = []vulnRemediation{
	{
		keywords:       []string{"sqli", "sql 注入"},
		description:    "SQL 注入漏洞允许攻击者通过恶意输入操纵数据库查询，可能导致数据泄露、数据篡改或完全控制数据库。",
		remediation:    "使用参数化查询或 ORM 框架，对用户输入进行严格校验，部署 Web 应用防火墙（WAF）。",
		recommendation: "- **SQL 注入**: 改用参数化查询/ORM, 登录接口做严格输入校验, 部署 WAF。",
	},
	{
		keywords:       []string{"xss"},
		description:    "跨站脚本（XSS）允许攻击者在受害者浏览器中执行任意脚本，可窃取会话、篡改页面或发起钓鱼。",
		remediation:    "对输出进行上下文相关编码（HTML/JS/URL），实施 CSP，过滤富文本输入。",
		recommendation: "- **XSS**: 输出编码 + CSP + 输入过滤, 对富文本走白名单清洗。",
	},
	{
		keywords:       []string{"swagger", "api-docs", "openapi", "api 文档"},
		description:    "API 文档（Swagger/OpenAPI）公开暴露，可能泄露敏感的 API 端点、参数结构和业务逻辑，为攻击者提供攻击面信息。",
		remediation:    "在生产环境关闭 Swagger UI 公开访问，或添加认证机制（BasicAuth/OAuth2）。",
		recommendation: "- **API 文档暴露**: 生产环境关闭 Swagger/api-docs 公开访问或加认证。",
	},
	{
		keywords:       []string{"security headers", "csp", "hsts", "安全头"},
		description:    "缺失关键安全响应头，可能导致 XSS、点击劫持、MIME 类型嗅探等攻击。",
		remediation:    "配置 CSP、HSTS、X-Frame-Options、X-Content-Type-Options 等安全响应头。",
		recommendation: "- **缺失安全头**: 补齐 CSP / HSTS / X-Frame-Options 等安全响应头。",
	},
	{
		keywords:       []string{"prometheus", "metrics", "监控端点"},
		description:    "监控/指标端点公开暴露，泄露系统内部状态、版本与资源信息，辅助攻击者精准打击。",
		remediation:    "限制 /metrics 等端点为内网或认证访问，关闭匿名抓取。",
		recommendation: "- **监控端点暴露**: 限制 /metrics 为内网/认证访问, 防信息泄露。",
	},
	{
		keywords:       []string{"path traversal", "traversal", "路径遍历", "任意文件"},
		description:    "路径遍历允许攻击者读取服务器任意文件（如配置文件、源码），可进一步升级为 RCE。",
		remediation:    "规范化并校验用户传入路径，禁止绝对路径与 ../ 穿越，使用白名单目录。",
		recommendation: "- **路径遍历**: 路径规范化校验, 禁止 ../ 穿越, 按白名单目录提供服务。",
	},
	{
		keywords:       []string{"cors"},
		description:    "CORS 配置过于宽松（如允许任意 Origin），可被恶意站点利用发起跨域请求窃取数据。",
		remediation:    "CORS 白名单限定可信来源，禁止反射任意 Origin，敏感接口不携带凭证。",
		recommendation: "- **CORS 配置**: 限定可信 Origin 白名单, 禁止反射任意来源。",
	},
	{
		keywords:       []string{"ssrf"},
		description:    "服务端请求伪造（SSRF）允许攻击者以内网身份访问内部服务或云元数据，横向打击内部资产。",
		remediation:    "校验请求目标（IP/域名白名单），禁止访问内网/链路本地地址，禁用重定向跟随。",
		recommendation: "- **SSRF**: 请求目标白名单校验, 阻断内网/元数据地址与重定向。",
	},
	{
		keywords:       []string{"rce", "remote code", "命令执行", "code execution"},
		description:    "远程代码/命令执行允许攻击者在服务器上执行任意命令，等同于完全控制主机。",
		remediation:    "避免拼接系统命令，使用白名单参数校验，最小化服务运行权限，及时打补丁。",
		recommendation: "- **命令/代码执行**: 禁止拼接系统命令, 白名单校验参数, 最小权限运行。",
	},
	{
		keywords:       []string{"upload", "文件上传"},
		description:    "文件上传校验缺失，攻击者可上传 WebShell 或恶意文件获得代码执行能力。",
		remediation:    "上传做扩展名/MIME/内容白名单校验，文件存隔离目录并禁用脚本执行。",
		recommendation: "- **文件上传**: 扩展名/MIME 白名单, 隔离存储, 禁止上传目录执行脚本。",
	},
	{
		keywords:       []string{"sensitive", "信息泄露", "information disclosure", "泄露"},
		description:    "敏感信息泄露（源码、凭据、内部路径等）为攻击者提供进一步渗透的跳板。",
		remediation:    "移除响应中的敏感字段与调试信息，密钥轮换并迁移到密钥管理系统。",
		recommendation: "- **信息泄露**: 移除敏感字段/调试信息, 密钥轮换并集中管理。",
	},
	{
		keywords:       []string{"deserialization", "反序列化"},
		description:    "不安全的反序列化可被构造恶意对象触发任意代码执行。",
		remediation:    "对反序列化输入做类型白名单与完整性校验（签名/加密），升级组件版本。",
		recommendation: "- **反序列化**: 类型白名单 + 完整性校验, 升级受影响组件。",
	},
}

// matchVuln —— 按关键词匹配漏洞知识(小写不敏感); 未命中返回 nil。
func matchVuln(text string) *vulnRemediation {
	t := strings.ToLower(text)
	for i := range vulnRemediations {
		for _, kw := range vulnRemediations[i].keywords {
			if strings.Contains(t, kw) {
				return &vulnRemediations[i]
			}
		}
	}
	return nil
}
