package tools

import (
	"regexp"
	"strings"
	"time"
)

var reNmapPort = regexp.MustCompile(`(\d+)/tcp\s+open\s+(\S+)`)

// ParseNmap 把 nmap/存活扫描输出结构化, 每条 observation 记住逐字来源行(对应 Python parse_nmap)。
// 逐字来源(Excerpt)是证据回查的锚点, 绝不能改写工具原文。
func ParseNmap(stdout string, args map[string]any) []Observation {
	host := normalizeHost(ArgStr(args, "target", "?")) // URL/host:port 归一化, 避免节点 ID 被 : 切错
	up := host
	for _, l := range strings.Split(stdout, "\n") {
		if strings.Contains(l, "up") {
			up = strings.TrimSpace(l)
			break
		}
	}
	obs := []Observation{{Kind: "host", Key: host, Label: host, Excerpt: up}}
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(raw)
		if m := reNmapPort.FindStringSubmatch(line); m != nil {
			port, svc := m[1], m[2]
			obs = append(obs, Observation{
				Kind:    "service",
				Key:     host + ":" + port,
				Label:   svc + " on " + host + ":" + port,
				Excerpt: line,
			})
		}
	}
	return obs
}

// NmapPing 真实存活扫描(需本机装 nmap)。
func NmapPing(args map[string]any) ToolResult {
	return Sh([]string{"nmap", "-sn", ArgStr(args, "target", "")}, 60*time.Second)
}

// FakeScan 离线仿真, 不碰真实网络(自检/演示用)。
func FakeScan(args map[string]any) ToolResult {
	return ToolResult{Success: true, Stdout: "Host " + ArgStr(args, "target", "?") + " is up\n22/tcp open ssh"}
}

// RegisterBuiltins 注册内核自带工具(对应 Python 默认 REGISTRY)。
func RegisterBuiltins(r *Registry) {
	r.Register(&Tool{Name: "nmap_scan", Level: LevelScan, Desc: "nmap 完整扫描: 服务版本识别(-sV) + 默认脚本(-sC) + 漏洞检测, 产出精准服务指纹/OS/NSE 脚本结果", Run: NmapScan, Parse: ParseNmapXML})
	r.Register(&Tool{Name: "nmap_ping", Level: LevelScan, Desc: "nmap 主机存活扫描(需 pcap 驱动)", Run: NmapPing, Parse: ParseNmap})
	r.Register(&Tool{Name: "fake_scan", Level: LevelScan, Desc: "离线仿真扫描(自检用, 不碰真实网络)", Run: FakeScan, Parse: ParseNmap})
}
