//go:build windows

package tools

import (
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// windowsProxy —— 读 HKCU Internet Settings 的 ProxyEnable/ProxyServer(仅 Windows)。
// 用户常见场景: Clash 等代理只设了注册表, Go 默认不认, 下载必超时 —— 这里兜底。
func windowsProxy() (*url.URL, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil, nil
	}
	defer k.Close()
	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enable == 0 {
		return nil, nil
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		return nil, nil
	}
	// ProxyServer 可能是 "host:port" 或 "http=host:port;https=host:port" 格式
	proxy := server
	if i := strings.Index(server, "="); i > 0 {
		for _, part := range strings.Split(server, ";") {
			if strings.HasPrefix(part, "https=") {
				proxy = strings.TrimPrefix(part, "https=")
				break
			}
		}
	}
	if !strings.Contains(proxy, "://") {
		proxy = "http://" + proxy
	}
	return url.Parse(proxy)
}
