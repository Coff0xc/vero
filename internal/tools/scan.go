package tools

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// commonSvc —— 常见端口→服务名(供输出兼容 ParseNmap)。
var commonSvc = map[int]string{
	21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp", 53: "domain", 80: "http",
	110: "pop3", 139: "netbios-ssn", 143: "imap", 443: "https", 445: "microsoft-ds",
	993: "imaps", 995: "pop3s", 1433: "ms-sql-s", 3000: "http", 3306: "mysql",
	3389: "ms-wbt-server", 5432: "postgresql", 6379: "redis", 8080: "http-proxy", 8443: "https-alt",
}

var defaultPorts = []int{21, 22, 23, 25, 53, 80, 110, 139, 143, 443, 445, 993, 995, 1433, 3000, 3306, 3389, 5432, 6379, 8080, 8443}

// sentinelPorts —— 哨兵端口: 随机高位, 真实主机几乎不可能监听。
// 若哨兵也"connect 成功", 说明链路上有中间设备伪造所有 TCP 连接 —— 扫描结果不可信。
// 这是把"环境欺骗"变成 agent 自检能力: 证据回查防 LLM 幻觉, 哨兵防工具/环境欺骗。
var sentinelPorts = []int{47123, 51987, 61999}

// PortScan —— Go 原生 TCP connect 端口扫描(不依赖 nmap/pcap, 跨平台可控)。
// 真实建立 TCP 连接探测端口开放; 输出格式刻意兼容 ParseNmap, 复用同一 parser。
// 附带哨兵自检: 识破中间设备伪造 connect 的假阳性, 拒绝污染攻击图。
func PortScan(args map[string]any) ToolResult {
	target := normalizeHost(ArgStr(args, "target", ""))
	if target == "" {
		return ToolResult{Success: false, Stderr: "port_scan: 缺 target", RC: -1}
	}
	ports := portsFrom(args)
	_, explicit := args["ports"]

	// 哨兵假阳性自检仅在默认批量扫描时启用; 定向扫描(显式 ports)信任用户,
	// 不加哨兵 —— 也避免测试受占用端口干扰(消除 flaky)。
	scanList := ports
	if !explicit {
		scanList = append(append([]int{}, ports...), sentinelPorts...)
	}
	openSet := scanPorts(target, scanList)

	if !explicit {
		sentinelOpen := 0
		for _, sp := range sentinelPorts {
			if openSet[sp] {
				sentinelOpen++
			}
		}
		if sentinelOpen >= 2 { // 多数哨兵"开放" -> connect 被伪造, 结果不可信, 抑制
			return ToolResult{Success: true, Stdout: fmt.Sprintf(
				"Host %s is up\n[!] 哨兵端口 %d/%d 响应 open → 链路存在 TCP connect 伪造(中间设备/透明代理), "+
					"端口探测结果不可信, 已抑制以免污染攻击图\n", target, sentinelOpen, len(sentinelPorts))}
		}
	}

	open := []int{}
	for _, p := range ports {
		if openSet[p] {
			open = append(open, p)
		}
	}
	sort.Ints(open)

	var b strings.Builder
	fmt.Fprintf(&b, "Host %s is up\n", target)
	for _, p := range open {
		svc := commonSvc[p]
		if svc == "" {
			svc = "unknown"
		}
		fmt.Fprintf(&b, "%d/tcp open %s\n", p, svc)
	}
	return ToolResult{Success: true, Stdout: b.String()}
}

// scanPorts —— 并发 TCP connect 一批端口, 返回开放集合。
func scanPorts(target string, ports []int) map[int]bool {
	open := make(map[int]bool)
	var mu sync.Mutex
	sem := make(chan struct{}, 64)
	var wg sync.WaitGroup
	for _, p := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(target, strconv.Itoa(p)), 2*time.Second)
			if err == nil {
				conn.Close()
				mu.Lock()
				open[p] = true
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	return open
}

// portsFrom —— 从 args["ports"] 取端口列表(支持 []int / JSON []any); 缺省用 defaultPorts。
func portsFrom(args map[string]any) []int {
	v, ok := args["ports"]
	if !ok {
		return defaultPorts
	}
	switch vv := v.(type) {
	case []int:
		return vv
	case []any:
		var ps []int
		for _, x := range vv {
			switch n := x.(type) {
			case float64:
				ps = append(ps, int(n))
			case int:
				ps = append(ps, n)
			}
		}
		if len(ps) == 0 {
			return defaultPorts
		}
		return ps
	}
	return defaultPorts
}

// normalizeHost —— 把 URL / host:port 归一化为纯 host(端口扫描要 host)。
// D8 修复: 裸 IPv6(多冒号, 如 2001:db8::1)不做端口剥离, 否则末段数字会被误剥成 2001:db8:。
func normalizeHost(t string) string {
	if i := strings.Index(t, "://"); i >= 0 {
		t = t[i+3:]
	}
	if i := strings.IndexByte(t, '/'); i >= 0 {
		t = t[:i]
	}
	if strings.HasPrefix(t, "[") { // [::1]:80 → [::1]
		if i := strings.Index(t, "]"); i >= 0 {
			return t[:i+1]
		}
	}
	if strings.Count(t, ":") > 1 {
		return t // 裸 IPv6: 冒号是地址的一部分, 不剥端口
	}
	if i := strings.LastIndex(t, ":"); i >= 0 {
		if _, err := strconv.Atoi(t[i+1:]); err == nil { // 冒号后是数字端口才剥离
			t = t[:i]
		}
	}
	return t
}
