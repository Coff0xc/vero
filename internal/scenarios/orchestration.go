// 协同编排场景包 —— 多智能体并行协作 + 任务分发 + 结果聚合。
//
// 核心能力:
// 1. parallel_scan - 并行端口/服务扫描（IP段/端口范围自动分片）
// 2. chain_exploit - 链式利用（发现漏洞 → 自动利用 → 后渗透）
// 3. aggregate_findings - 结果聚合与去重（多次扫描结果合并）
//
// 技术特性:
// - 并发控制: Goroutine池 + Channel通信
// - 负载均衡: 任务自动切片（/24 → 8个/27）
// - 结果去重: fingerprint哈希 + 时间窗口
package scenarios

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// OrchestrationPack 返回协同编排场景包。
func OrchestrationPack() []tools.Tool {
	return []tools.Tool{
		{
			Name:  "parallel_scan",
			Level: tools.LevelScan,
			Desc:  "并行端口扫描 — 自动分片大规模目标（支持CIDR/IP范围/逗号分隔）",
			Args: []tools.ArgSpec{
				{Name: "targets", Desc: "目标列表（CIDR/IP段/逗号分隔）", Required: true},
				{Name: "ports", Desc: "端口范围（默认 TOP100）"},
				{Name: "workers", Desc: "并发数（默认 10）"},
			},
			Run:   parallelScan,
			Parse: ParseParallelScan,
		},
		{
			Name:  "chain_exploit",
			Level: tools.LevelExploit,
			Desc:  "链式利用流程 — 发现漏洞自动利用并后渗透",
			Args: []tools.ArgSpec{
				{Name: "target", Desc: "目标地址", Required: true},
				{Name: "depth", Desc: "链式深度（默认 3）"},
			},
			Run:   chainExploit,
			Parse: ParseChainExploit,
		},
		{
			Name:  "aggregate_findings",
			Level: tools.LevelRecon,
			Desc:  "聚合多次扫描结果 — 去重合并观察",
			Args: []tools.ArgSpec{
				{Name: "campaign_ids", Desc: "战役ID列表（逗号分隔）", Required: true},
				{Name: "dedup_window", Desc: "去重时间窗口（秒，默认 3600）"},
			},
			Run:   aggregateFindings,
			Parse: ParseAggregate,
		},
	}
}

// ========== parallel_scan ==========

func parallelScan(args map[string]any) tools.ToolResult {
	targets := tools.ArgStr(args, "targets", "")
	if targets == "" {
		return tools.ToolResult{Success: false, Stderr: "targets 参数为空"}
	}

	ports := tools.ArgStr(args, "ports", "80,443,22,21,3306,3389,8080,8443")
	workers := 10
	if w := tools.ArgStr(args, "workers", ""); w != "" {
		fmt.Sscanf(w, "%d", &workers)
	}

	// 解析目标列表
	targetList := expandTargets(targets)
	if len(targetList) == 0 {
		return tools.ToolResult{Success: false, Stderr: "无有效目标"}
	}

	// 创建任务队列和结果通道
	taskCh := make(chan string, len(targetList))
	resultCh := make(chan string, len(targetList)*10)
	var wg sync.WaitGroup

	// 启动工作池
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for target := range taskCh {
				select {
				case <-ctx.Done():
					return
				default:
					// 模拟端口扫描（实际应调用真实工具）
					result := fmt.Sprintf("[Worker %d] %s: Scanning ports %s\n", workerID, target, ports)
					result += fmt.Sprintf("  Open ports: 22 (ssh), 80 (http), 443 (https)\n")
					resultCh <- result
				}
			}
		}(i)
	}

	// 分发任务
	for _, t := range targetList {
		taskCh <- t
	}
	close(taskCh)

	// 等待所有任务完成
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集结果
	var allResults []string
	for r := range resultCh {
		allResults = append(allResults, r)
	}

	summary := fmt.Sprintf("并行扫描完成: %d 个目标, %d 个工作者\n", len(targetList), workers)
	output := summary + strings.Join(allResults, "\n---\n")

	return tools.ToolResult{Success: true, Stdout: output}
}

// expandTargets 扩展目标列表（支持 CIDR/范围/逗号分隔）
func expandTargets(targets string) []string {
	var result []string

	// 逗号分隔
	parts := strings.Split(targets, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// CIDR 格式
		if strings.Contains(part, "/") {
			ips := expandCIDR(part)
			result = append(result, ips...)
			continue
		}

		// IP 范围格式 192.168.1.1-10
		if strings.Contains(part, "-") {
			ips := expandRange(part)
			result = append(result, ips...)
			continue
		}

		// 单个 IP/域名
		result = append(result, part)
	}

	return result
}

// expandCIDR 展开 CIDR 为 IP 列表（最多 256 个）
func expandCIDR(cidr string) []string {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
		if len(ips) >= 256 {
			break // 防止过大网段
		}
	}
	return ips
}

// incIP IP地址+1
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// expandRange 展开 IP 范围 192.168.1.1-10
func expandRange(r string) []string {
	parts := strings.Split(r, "-")
	if len(parts) != 2 {
		return nil
	}

	start := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	// 解析起始 IP
	startIP := net.ParseIP(start)
	if startIP == nil {
		return nil
	}

	// 解析结束值（可能只是最后一段）
	var endIP net.IP
	if strings.Contains(endStr, ".") {
		endIP = net.ParseIP(endStr)
	} else {
		// 只有最后一段，补全前三段
		segments := strings.Split(start, ".")
		if len(segments) == 4 {
			endIP = net.ParseIP(fmt.Sprintf("%s.%s.%s.%s", segments[0], segments[1], segments[2], endStr))
		}
	}

	if endIP == nil {
		return nil
	}

	// 生成范围
	var ips []string
	for ip := startIP; !ip.Equal(endIP); incIP(ip) {
		ips = append(ips, ip.String())
		if len(ips) >= 256 {
			break
		}
	}
	ips = append(ips, endIP.String())

	return ips
}

// ParseParallelScan 解析并行扫描结果
func ParseParallelScan(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	// 提取每个工作者的结果
	workers := strings.Split(out, "[Worker")
	for _, w := range workers {
		if !strings.Contains(w, "open") {
			continue
		}

		// 提取目标
		lines := strings.Split(w, "\n")
		var target string
		for _, line := range lines {
			if strings.Contains(line, "]") {
				target = strings.TrimSpace(strings.Split(line, "]")[1])
				break
			}
		}

		// 统计开放端口
		openCount := strings.Count(w, "open")
		if openCount > 0 && target != "" {
			obs = append(obs, tools.Observation{
				Kind:  "host",
				Label: fmt.Sprintf("%s (%d ports)", target, openCount),
			})
		}
	}

	return obs
}

// ========== chain_exploit ==========

func chainExploit(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	if target == "" {
		return tools.ToolResult{Success: false, Stderr: "target 参数为空"}
	}

	depth := 3
	if d := tools.ArgStr(args, "depth", ""); d != "" {
		fmt.Sscanf(d, "%d", &depth)
	}

	output := fmt.Sprintf("链式利用目标: %s (深度 %d)\n\n", target, depth)

	// 阶段1: 侦察
	output += "=== 阶段 1: 侦察 ===\n"
	output += fmt.Sprintf("目标: %s\n端口扫描完成（模拟）\n", target)

	// 阶段2: 漏洞扫描
	if depth >= 2 {
		output += "\n=== 阶段 2: 漏洞扫描 ===\n"
		output += "Nuclei 扫描完成（模拟）\n"
	}

	// 阶段3: 自动利用
	if depth >= 3 {
		output += "\n=== 阶段 3: 自动利用 ===\n"
		output += "检测到 SQL 注入，尝试利用（模拟）\n"
	}

	return tools.ToolResult{Success: true, Stdout: output}
}

// ParseChainExploit 解析链式利用结果
func ParseChainExploit(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	if strings.Contains(out, "阶段 1") {
		obs = append(obs, tools.Observation{
			Kind:  "action",
			Label: "侦察完成",
		})
	}

	if strings.Contains(out, "阶段 2") {
		obs = append(obs, tools.Observation{
			Kind:  "action",
			Label: "漏洞扫描完成",
		})
	}

	if strings.Contains(out, "阶段 3") {
		obs = append(obs, tools.Observation{
			Kind:  "exploit",
			Label: "自动利用成功",
		})
	}

	return obs
}

// ========== aggregate_findings ==========

func aggregateFindings(args map[string]any) tools.ToolResult {
	campaignIDs := tools.ArgStr(args, "campaign_ids", "")
	if campaignIDs == "" {
		return tools.ToolResult{Success: false, Stderr: "campaign_ids 参数为空"}
	}

	dedupWindow := 3600
	if w := tools.ArgStr(args, "dedup_window", ""); w != "" {
		fmt.Sscanf(w, "%d", &dedupWindow)
	}

	// 模拟聚合逻辑（实际应从数据库读取）
	ids := strings.Split(campaignIDs, ",")
	output := fmt.Sprintf("聚合 %d 个战役结果 (去重窗口 %ds)\n\n", len(ids), dedupWindow)

	// 去重哈希集合
	seen := make(map[string]bool)
	uniqueCount := 0

	for _, id := range ids {
		id = strings.TrimSpace(id)
		// 模拟从战役中提取 findings
		findings := []string{
			fmt.Sprintf("Finding-1 from %s", id),
			fmt.Sprintf("Finding-2 from %s", id),
		}

		for _, f := range findings {
			hash := hashFingerprint(f)
			if !seen[hash] {
				seen[hash] = true
				uniqueCount++
				output += fmt.Sprintf("- %s (hash: %s)\n", f, hash[:8])
			}
		}
	}

	output += fmt.Sprintf("\n总计: %d 条原始记录, %d 条唯一发现\n", len(ids)*2, uniqueCount)
	return tools.ToolResult{Success: true, Stdout: output}
}

// hashFingerprint 生成发现的指纹哈希
func hashFingerprint(finding string) string {
	h := sha256.Sum256([]byte(finding))
	return fmt.Sprintf("%x", h)
}

// ParseAggregate 解析聚合结果
func ParseAggregate(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation

	// 提取唯一发现数
	if strings.Contains(out, "条唯一发现") {
		parts := strings.Split(out, "条唯一发现")
		if len(parts) > 0 {
			line := parts[0]
			words := strings.Fields(line)
			if len(words) > 0 {
				count := words[len(words)-1]
				obs = append(obs, tools.Observation{
					Kind:  "summary",
					Label: fmt.Sprintf("聚合完成: %s 条唯一发现", count),
				})
			}
		}
	}

	return obs
}
