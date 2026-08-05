package tools

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// NmapScan —— 真实 nmap 扫描: -sV 服务版本 + -sC 默认脚本 + XML 输出。
// 比 Go 原生 TCP connect 更强: 服务指纹/版本/OS/NSE 脚本结果, 是精准场景路由的基础。
func NmapScan(args map[string]any) ToolResult {
	target := ArgStr(args, "target", "")
	if target == "" {
		return ToolResult{Success: false, Stderr: "nmap_scan: 缺 target", RC: -1}
	}
	// 标准扫描: 服务版本 + 默认脚本 + top 1000 端口 + XML 输出
	// -Pn 跳过 ping(防火墙可能拦截), --open 只输出开放端口(减少噪音)
	return Sh([]string{"nmap", "-sV", "-sC", "--top-ports", "1000",
		"-Pn", "--open", "-oX", "-", target}, 600*time.Second)
}

// NmapRun —— Nmap XML 输出根结构(仅映射需要的字段, 不求完整)。
type NmapRun struct {
	Hosts []NmapHost `xml:"host"`
}

type NmapHost struct {
	Status    NmapStatus    `xml:"status"`
	Addresses []NmapAddress `xml:"address"`
	Ports     NmapPorts     `xml:"ports"`
	OS        NmapOS        `xml:"os"`
}

type NmapStatus struct {
	State string `xml:"state,attr"`
}

type NmapAddress struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"` // ipv4/ipv6/mac
}

type NmapPorts struct {
	Ports []NmapPort `xml:"port"`
}

type NmapPort struct {
	Protocol string        `xml:"protocol,attr"`
	PortID   string        `xml:"portid,attr"`
	State    NmapPortState `xml:"state"`
	Service  NmapService   `xml:"service"`
	Scripts  []NmapScript  `xml:"script"`
}

type NmapPortState struct {
	State string `xml:"state,attr"` // open/closed/filtered
}

type NmapService struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
	ExtraInfo string `xml:"extrainfo,attr"`
}

type NmapScript struct {
	ID     string `xml:"id,attr"`
	Output string `xml:"output,attr"`
}

type NmapOS struct {
	Matches []NmapOSMatch `xml:"osmatch"`
}

type NmapOSMatch struct {
	Name     string `xml:"name,attr"`
	Accuracy string `xml:"accuracy,attr"`
}

// ParseNmapXML —— 解析 nmap -oX 输出, 提取 host/service/finding(漏洞脚本)。
// 每个观测带逐字来源(Excerpt), 保证证据可回查到 XML 原文片段。
func ParseNmapXML(stdout string, args map[string]any) []Observation {
	var run NmapRun
	if err := xml.Unmarshal([]byte(stdout), &run); err != nil {
		return nil
	}

	var obs []Observation
	for _, h := range run.Hosts {
		if h.Status.State != "up" {
			continue
		}

		// 提取主机 IP (优先 ipv4)
		hostIP := ""
		for _, addr := range h.Addresses {
			if addr.AddrType == "ipv4" {
				hostIP = addr.Addr
				break
			}
		}
		if hostIP == "" && len(h.Addresses) > 0 {
			hostIP = h.Addresses[0].Addr
		}
		if hostIP == "" {
			continue
		}

		// Host 节点(用原文真实行作 excerpt, 修 D7: 构造片段属性顺序可能与原文不一致)
		obs = append(obs, Observation{
			Kind:    "host",
			Key:     hostIP,
			Label:   hostIP,
			Excerpt: excerptLine(stdout, `addr="`+hostIP+`"`),
		})

		// OS 指纹 -> finding
		if len(h.OS.Matches) > 0 {
			osMatch := h.OS.Matches[0]
			obs = append(obs, Observation{
				Kind:    "finding",
				Key:     hostIP + ":os:" + osMatch.Name,
				Label:   fmt.Sprintf("[info] OS: %s (accuracy: %s%%)", osMatch.Name, osMatch.Accuracy),
				Excerpt: excerptLine(stdout, `name="`+osMatch.Name+`"`),
			})
		}

		// 服务节点
		for _, p := range h.Ports.Ports {
			if p.State.State != "open" {
				continue
			}

			port := p.PortID
			svcName := p.Service.Name
			if svcName == "" {
				svcName = "unknown"
			}

			// Service 节点: host:port
			svcKey := fmt.Sprintf("%s:%s", hostIP, port)
			svcLabel := fmt.Sprintf("%s:%s/%s", hostIP, port, svcName)

			// 补充版本信息到 label
			if p.Service.Product != "" {
				svcLabel += fmt.Sprintf(" (%s", p.Service.Product)
				if p.Service.Version != "" {
					svcLabel += " " + p.Service.Version
				}
				if p.Service.ExtraInfo != "" {
					svcLabel += " " + p.Service.ExtraInfo
				}
				svcLabel += ")"
			}

			// Excerpt: 从原文提取该端口所在行(逐字可回查, 修 D7: 构造属性片段
			// 顺序与原文 XML 不一致(protocol 在前 portid 在后)导致证据回查假阴性)
			excerpt := excerptLine(stdout, `portid="`+port+`"`)
			if svcName != "unknown" {
				excerpt = excerptLine(stdout, `name="`+svcName+`"`)
			}

			obs = append(obs, Observation{
				Kind:    "service",
				Key:     svcKey,
				Label:   svcLabel,
				Excerpt: excerpt,
			})

			// NSE 脚本结果 -> finding(尤其是漏洞检测)
			for _, script := range p.Scripts {
				// 只提取关键漏洞脚本(vuln 类), 过滤掉信息类脚本减少噪音
				if !isVulnScript(script.ID) {
					continue
				}
				scriptOut := strings.TrimSpace(script.Output)
				if scriptOut == "" {
					continue
				}

				obs = append(obs, Observation{
					Kind:    "finding",
					Key:     svcKey + ":script:" + script.ID,
					Label:   fmt.Sprintf("[nse] %s: %s", script.ID, oneline(scriptOut, 100)),
					Excerpt: excerptLine(stdout, `id="`+script.ID+`"`), // 脚本行原文, 逐字可回查
				})
			}
		}
	}

	return obs
}

// excerptLine —— 从工具原文中提取包含 marker 的第一行原文(去首尾空白)。
// 替代手工构造属性片段(修 D7): 保证 Excerpt 逐字存在于原文, 证据回查不产生假阴性。
// 找不到时返回 marker 本身 —— 回查会如实报"未找到", 而非静默掩盖。
func excerptLine(stdout, marker string) string {
	for _, l := range strings.Split(stdout, "\n") {
		if strings.Contains(l, marker) {
			return strings.TrimSpace(l)
		}
	}
	return marker
}

// isVulnScript —— 判断 NSE 脚本是否为漏洞检测类(值得作为 finding)。
// 策略: 脚本名包含 vuln/exploit/cve/dos 等关键词, 或已知重要脚本。
func isVulnScript(id string) bool {
	vulnKeywords := []string{"vuln", "exploit", "cve", "dos", "backdoor"}
	idLower := strings.ToLower(id)
	for _, kw := range vulnKeywords {
		if strings.Contains(idLower, kw) {
			return true
		}
	}
	// 已知重要脚本白名单
	important := map[string]bool{
		"http-shellshock":    true,
		"ssl-heartbleed":     true,
		"smb-vuln-ms17-010":  true,
		"smb-vuln-ms08-067":  true,
		"ftp-anon":           true,
		"mysql-empty-password": true,
	}
	return important[id]
}

// oneline —— 把多行输出压缩成单行(供 label 展示), 截断到 max rune。
func oneline(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " | ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}
