// Command redcell —— 自主红队渗透智能体作战室(单一二进制, embed 前端)。
//
// 用法:
//
//	redcell                     启动作战室(默认 http://127.0.0.1:8000)
//	redcell -port 9000          指定端口
//	redcell -selfcheck          离线自检(跑内核闭环, 无需 API key)后退出
//	redcell -probe <url>        真实工具闭环: curl 真实探测目标 HTTP 指纹后退出
//
// 有 ANTHROPIC_API_KEY 时战役用真实 Claude(opus)决策; 否则用确定性规划器离线跑通。
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/audit"
	"github.com/Coff0xc/vero/internal/core"
	"github.com/Coff0xc/vero/internal/llm"
	"github.com/Coff0xc/vero/internal/planner"
	"github.com/Coff0xc/vero/internal/report"
	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/server"
	"github.com/Coff0xc/vero/internal/store"
	"github.com/Coff0xc/vero/internal/tools"
	"github.com/Coff0xc/vero/internal/webui"
)

func main() {
	port := flag.Int("port", 8000, "监听端口")
	dbPath := flag.String("db", "vero.db", "SQLite 数据库路径")
	selfcheck := flag.Bool("selfcheck", false, "离线自检后退出")
	probe := flag.String("probe", "", "真实 HTTP 指纹侦察目标(curl -sI), 侦察后退出")
	scan := flag.String("scan", "", "真实 TCP 端口扫描目标(Go 原生, 无 nmap 依赖), 扫描后退出")
	nmapScan := flag.String("nmap", "", "真实 nmap 完整扫描目标(服务版本+NSE脚本+漏洞检测), 扫描后退出")
	webscan := flag.String("webscan", "", "真实 web 侦察目标(curl 指纹 + nuclei 漏扫), 侦察后退出")
	agent := flag.String("agent", "", "真实 LLM 自主侦察目标(DeepSeek/Claude 决策 + 真实工具), 结束后退出")
	exploitT := flag.String("exploit", "", "调试: 直接跑 exploit_sqli 打印完整结果, 退出")

	// P1/P2/P3 新增命令行入口
	msfSearch := flag.String("msf-search", "", "Metasploit 搜索 exploit 模块 (需 msfrpcd 运行), 例: -msf-search ms17_010")
	cloudAWS := flag.String("cloud-aws", "", "AWS IMDS 元数据提取 (需在 EC2 实例内运行), 例: -cloud-aws enum")
	cloudAzure := flag.String("cloud-azure", "", "Azure IMDS 元数据提取 (需在 Azure VM 内运行), 例: -cloud-azure enum")
	cloudGCP := flag.String("cloud-gcp", "", "GCP IMDS 元数据提取 (需在 GCP 实例内运行), 例: -cloud-gcp enum")
	cloudS3 := flag.String("cloud-s3", "", "S3 bucket 公开访问检测, 例: -cloud-s3 my-bucket")
	containerEscape := flag.String("container-escape", "", "Docker 容器逃逸检测 (需在容器内运行), 例: -container-escape check")
	k8sSA := flag.String("k8s-sa", "", "Kubernetes ServiceAccount 提取 (需在 pod 内运行), 例: -k8s-sa enum")

	// 工具验证
	toolTest := flag.Bool("tooltest", false, "验证所有工具集成状态")

	flag.Parse()

	if *toolTest {
		runToolTest()
		return
	}

	if *exploitT != "" {
		runExploit(*exploitT)
		return
	}

	if *selfcheck {
		runSelfcheck()
		return
	}
	if *probe != "" {
		runProbe(*probe)
		return
	}
	if *scan != "" {
		runScan(*scan)
		return
	}
	if *nmapScan != "" {
		runNmapScan(*nmapScan)
		return
	}
	if *webscan != "" {
		runWebScan(*webscan)
		return
	}
	if *agent != "" {
		runRealAgent(*agent)
		return
	}

	// P1/P2/P3 入口
	if *msfSearch != "" {
		runMSFSearch(*msfSearch)
		return
	}
	if *cloudAWS != "" {
		runCloudAWS()
		return
	}
	if *cloudAzure != "" {
		runCloudAzure()
		return
	}
	if *cloudGCP != "" {
		runCloudGCP()
		return
	}
	if *cloudS3 != "" {
		runCloudS3(*cloudS3)
		return
	}
	if *containerEscape != "" {
		runContainerEscape()
		return
	}
	if *k8sSA != "" {
		runK8sSA()
		return
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()

	auditor := audit.New("audit.jsonl", "rollback.jsonl")

	sub, err := fs.Sub(webui.Dist, "dist")
	if err != nil {
		log.Fatalf("加载前端资源失败: %v", err)
	}

	srv := server.New(st, auditor, sub)
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	fmt.Printf("REDCELL 作战室: http://%s  (Ctrl+C 停)\n", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Fatal(err)
	}
}

// runSelfcheck —— 离线跑一场规划战役, 验证核心闭环(侦察→web→凭证→SMB失败→WMI→foothold)。
func runSelfcheck() {
	reg := tools.NewRegistry()
	tools.RegisterBuiltins(reg)
	reg.Register(&tools.Tool{Name: "web_vuln_scan", Level: tools.LevelScan,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "CVE-2021-x RCE 命中"} }})
	reg.Register(&tools.Tool{Name: "fake_dump", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "corp\\admin:hash"} }})
	reg.Register(&tools.Tool{Name: "psexec_smb", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: false, Stderr: "SMB signing required, 失败"} }})
	reg.Register(&tools.Tool{Name: "wmiexec", Level: tools.LevelExploit,
		Run: func(map[string]any) tools.ToolResult { return tools.ToolResult{Success: true, Stdout: "WMI shell obtained"} }})

	g, trace := core.RunAgent("拿下 foothold", planner.NewPlanner("foothold"),
		reg, core.AutoApprove, core.DiscardEmit, 15)

	foothold := false
	for _, n := range g.Nodes {
		if n.Type == "foothold" {
			foothold = true
		}
	}
	viol := core.VerifyEvidence(g, trace)
	fmt.Println(g.Snapshot())
	fmt.Printf("\n自检: foothold=%v  证据违规=%d\n", foothold, len(viol))
	if !foothold || len(viol) != 0 {
		fmt.Println("FAIL: 闭环未跑通")
		os.Exit(1)
	}
	fmt.Println("OK: 规划端到端(侦察→web→凭证→SMB失败→WMI→foothold) ✓  证据逐字可回查 ✓  动态重规划 ✓")
}

// runProbe —— 真实工具闭环: 用 curl 真实探测目标 HTTP 指纹, parser 吃真实输出,
// 攻击图记录真实证据, 证据逐字回查验证真实数据也可溯源。
// 只做被动指纹(curl -sI HEAD), 无攻击动作 —— 等同浏览器访问, 授权边界清晰。
func runProbe(target string) {
	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg) // 注册真实 http_probe(curl) / web_vuln_scan(nuclei) 等
	if !reg.Has("http_probe") {
		fmt.Println("http_probe 未注册")
		os.Exit(1)
	}
	fmt.Printf("真实 HTTP 指纹侦察: %s  (curl -sI · 被动无攻击)\n", target)

	emit := func(e core.Event) {
		if e.Kind == "tool" {
			fmt.Printf("· 工具 %v  success=%v\n", e.Data["tool"], e.Data["success"])
		}
	}
	script := []core.Action{{Tool: "http_probe", Args: map[string]any{"target": target}, Rationale: "真实指纹侦察"}}
	g, trace := core.RunAgent("侦察 "+target, llm.NewMock(script), reg, core.AutoApprove, emit, 3)

	fmt.Println("\n" + g.Snapshot())
	fmt.Println("\n真实证据链(逐字来自 curl 原始输出):")
	found := false
	for _, id := range g.Order {
		for _, ev := range g.Nodes[id].Evidence {
			found = true
			fmt.Printf("  %s  ←  [%s] %q\n", id, ev.Tool, ev.Excerpt)
		}
	}
	if !found {
		fmt.Println("  (目标无 Server/X-Powered-By 响应头, 或 curl 未装/不可达)")
	}
	viol := core.VerifyEvidence(g, trace)
	fmt.Printf("\n证据逐字回查违规: %d  (0 = 全部证据可溯回 curl 真实输出, 无幻觉)\n", len(viol))
	if len(viol) > 0 {
		os.Exit(1)
	}
}

// runScan —— 真实端口扫描闭环: Go 原生 TCP connect 探测端口, ParseNmap 吃真实输出,
// 攻击图记录真实 host/service + 证据。无 nmap/pcap 依赖, 跨平台可控。
func runScan(target string) {
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{Name: "port_scan", Level: tools.LevelScan, Desc: "TCP connect 端口扫描, 发现开放端口/服务(target 用 host, 自动去 scheme/端口)", Run: tools.PortScan, Parse: tools.ParseNmap})
	fmt.Printf("真实 TCP 端口扫描: %s  (Go 原生 connect · 无 nmap/pcap 依赖)\n", target)

	emit := func(e core.Event) {
		if e.Kind == "tool" {
			fmt.Printf("· 工具 %v  success=%v\n", e.Data["tool"], e.Data["success"])
		}
	}
	script := []core.Action{{Tool: "port_scan", Args: map[string]any{"target": target}, Rationale: "真实端口扫描"}}
	g, trace := core.RunAgent("扫描 "+target, llm.NewMock(script), reg, core.AutoApprove, emit, 3)

	fmt.Println("\n" + g.Snapshot())
	fmt.Println("\n真实证据链(逐字来自扫描输出):")
	for _, id := range g.Order {
		for _, ev := range g.Nodes[id].Evidence {
			fmt.Printf("  %s  ←  [%s] %q\n", id, ev.Tool, ev.Excerpt)
		}
	}
	fmt.Printf("\n证据逐字回查违规: %d\n", len(core.VerifyEvidence(g, trace)))
}

// runWebScan —— 真实 web 侦察闭环: curl 真实指纹 + nuclei 真实漏扫, parser 吃真实输出,
// 攻击图记录真实 finding + 证据。针对本地授权靶(如 juice-shop), 本地回环结果可信。
func runWebScan(target string) {
	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)
	fmt.Printf("真实 web 侦察: %s  (curl 指纹 + nuclei 漏扫, 可能耗时)\n", target)

	emit := func(e core.Event) {
		if e.Kind == "tool" {
			fmt.Printf("· 工具 %v  success=%v", e.Data["tool"], e.Data["success"])
			if s, _ := e.Data["stderr"].(string); s != "" {
				fmt.Printf("  stderr=%s", s)
			}
			fmt.Println()
		}
	}
	script := []core.Action{
		{Tool: "http_probe", Args: map[string]any{"target": target}, Rationale: "HTTP 指纹"},
		{Tool: "web_vuln_scan", Args: map[string]any{"target": target}, Rationale: "nuclei 漏扫"},
		{Tool: "exploit_sqli", Args: map[string]any{"target": target}, Rationale: "SQLi 登录绕过尝试"},
	}
	g, trace := core.RunAgent("web 侦察+利用 "+target, llm.NewMock(script), reg, core.AutoApprove, emit, 5)

	findings := 0
	fmt.Println("\n真实 finding(逐字来自工具输出):")
	for _, id := range g.Order {
		n := g.Nodes[id]
		if n.Type == "finding" {
			findings++
			ev := ""
			if len(n.Evidence) > 0 {
				ev = n.Evidence[len(n.Evidence)-1].Excerpt
			}
			fmt.Printf("  %s  [%s]  ←  %q\n", n.Label, n.State, ev)
		}
	}
	viol := len(core.VerifyEvidence(g, trace))
	fmt.Printf("\n共 %d 个真实 finding · 证据逐字回查违规: %d\n", findings, viol)
	writeReport(target, g, viol)
}

// runNmapScan —— 真实 nmap 完整扫描闭环: 服务版本识别 + NSE 脚本 + 漏洞检测。
// 比 Go 原生 TCP connect 更强: 精准服务指纹 + OS 检测 + 漏洞脚本, 是场景路由的基础。
func runNmapScan(target string) {
	reg := tools.NewRegistry()
	tools.RegisterBuiltins(reg) // 注册 nmap_scan
	if !reg.Has("nmap_scan") {
		fmt.Println("nmap_scan 未注册(需安装 nmap)")
		os.Exit(1)
	}
	fmt.Printf("真实 nmap 完整扫描: %s  (服务版本 + NSE 脚本 + 漏洞检测, 可能耗时)\n", target)

	emit := func(e core.Event) {
		if e.Kind == "tool" {
			fmt.Printf("· 工具 %v  success=%v\n", e.Data["tool"], e.Data["success"])
		}
	}
	script := []core.Action{
		{Tool: "nmap_scan", Args: map[string]any{"target": target}, Rationale: "nmap 完整扫描"},
	}
	g, trace := core.RunAgent("nmap 扫描 "+target, llm.NewMock(script), reg, core.AutoApprove, emit, 3)

	fmt.Println("\n攻击图:")
	services, findings := 0, 0
	for _, id := range g.Order {
		n := g.Nodes[id]
		switch n.Type {
		case "service":
			services++
			fmt.Printf("  [服务] %s\n", n.Label)
		case "finding":
			findings++
			sev := "INFO"
			if strings.Contains(n.Label, "VULNERABLE") || strings.Contains(n.Label, "critical") {
				sev = "CRITICAL"
			}
			fmt.Printf("  [%s] %s\n", sev, n.Label)
		}
	}

	fmt.Println("\n证据链(服务版本示例):")
	count := 0
	for _, id := range g.Order {
		if count >= 5 {
			break
		}
		n := g.Nodes[id]
		if n.Type == "service" && len(n.Evidence) > 0 {
			fmt.Printf("  %s  ←  [nmap_scan] %q\n", n.Label, n.Evidence[0].Excerpt)
			count++
		}
	}

	viol := len(core.VerifyEvidence(g, trace))
	fmt.Printf("\n发现: %d 服务 · %d finding · 证据逐字回查违规: %d\n", services, findings, viol)
	if viol == 0 {
		fmt.Println("✓ 全部证据可溯回 nmap XML 输出, 无幻觉")
	}
	writeReport(target, g, viol)
}

// runExploit —— 调试: 直接跑 exploit_sqli, 打印完整结果(定位工具问题)。
func runExploit(target string) {
	reg := tools.NewRegistry()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg)
	tool, ok := reg.Get("exploit_sqli")
	if !ok {
		fmt.Println("exploit_sqli 未注册")
		return
	}
	res := tool.Run(map[string]any{"target": target})
	out := res.Stdout
	if len(out) > 400 {
		out = out[:400]
	}
	fmt.Printf("success=%v  rc=%d\nstdout: %s\nstderr: %s\n", res.Success, res.RC, out, res.Stderr)
}

// runRealAgent —— 真智能 + 真工具合体: 真实 LLM(DeepSeek/Claude)自主决策驱动真实工具打目标。
// LLM 每轮看 ReAct 轨迹(含上一步真实观察)自主选下一步, target 由注入包装兜底。
func runRealAgent(target string) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" && os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Println("需要 DEEPSEEK_API_KEY 或 ANTHROPIC_API_KEY 环境变量")
		os.Exit(1)
	}
	reg := tools.NewRegistry()
	reg.Register(&tools.Tool{Name: "port_scan", Level: tools.LevelScan, Desc: "TCP connect 端口扫描, 发现开放端口/服务(target 用 host, 自动去 scheme/端口)", Run: tools.PortScan, Parse: tools.ParseNmap})
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, reg) // http_probe / web_vuln_scan(nuclei) 真实

	var base core.LLM
	engine := "DeepSeek"
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		base = llm.NewDeepSeek(reg)
	} else {
		base = llm.NewClaude(reg)
		engine = "Claude"
	}
	chosen := llm.WithTarget(base, target)

	fmt.Printf("真实 LLM 自主侦察 [%s]: %s\n\n", engine, target)
	emit := func(e core.Event) {
		switch e.Kind {
		case "step":
			fmt.Printf("· [L%v] %v → %v\n", e.Data["level"], e.Data["tool"], e.Data["why"])
		case "tool":
			fmt.Printf("  ⤷ %v success=%v\n", e.Data["tool"], e.Data["success"])
		}
	}
	approve := func(a core.Action, level int) bool {
		fmt.Printf("  ⚠ [HITL] L%d %s%v 已批准(演示自动放行; 生产走 Web 审批门控)\n", level, a.Tool, a.Args)
		return true
	}
	goal := "对目标 " + target + " 做红队侦察与漏洞验证: 端口扫描→HTTP 指纹→漏扫→发现可利用点后尝试验证利用(如 SQLi 登录绕过 exploit_sqli)。逐步推进, 用真实证据坐实; 达成关键利用或已充分则停止。"
	g, trace := core.RunAgent(goal, chosen, reg, approve, emit, 8)

	fmt.Println("\n=== 攻击图 ===")
	fmt.Println(g.Snapshot())
	findings, services := 0, 0
	for _, n := range g.Nodes {
		switch n.Type {
		case "finding":
			findings++
		case "service":
			services++
		}
	}
	viol := len(core.VerifyEvidence(g, trace))
	fmt.Printf("\n自主发现: %d 服务 · %d finding · 证据逐字回查违规: %d\n", services, findings, viol)
	writeReport(target, g, viol)
}

// writeReport —— 生成渗透报告并落盘(交付物)。
func writeReport(target string, g *core.AttackGraph, violations int) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	md := report.Markdown(target, g, violations, ts)
	if err := os.WriteFile("vero-report.md", []byte(md), 0o644); err != nil {
		fmt.Printf("报告写入失败: %v\n", err)
		return
	}
	fmt.Printf("\n📄 渗透报告已生成: vero-report.md (%d 字节)\n", len(md))
}
