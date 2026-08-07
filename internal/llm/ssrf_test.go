package llm

import (
	"strings"
	"testing"

	"github.com/Coff0xc/vero/internal/config"
)

// TestValidateBaseURL_BlockedAddresses —— 回归: validateBaseURL 必须拒绝所有内网/环回/链路本地地址及非法协议。
// 对应 sec-02: handleProviderTest SSRF 修复锁死测试。
func TestValidateBaseURL_BlockedAddresses(t *testing.T) {
	blocked := []struct {
		name string
		url  string
	}{
		// AWS IMDS(链路本地)
		{"AWS IMDS IPv4", "http://169.254.169.254"},
		{"AWS IMDS path", "http://169.254.169.254/latest/meta-data"},
		// 环回
		{"loopback 127.0.0.1", "http://127.0.0.1:8080"},
		{"loopback 127.0.0.2", "http://127.0.0.2"},
		{"loopback 127.255.255.255", "http://127.255.255.255"},
		// RFC1918
		{"RFC1918 10.x", "http://10.0.0.1"},
		{"RFC1918 10.10.10.10", "http://10.10.10.10/api"},
		{"RFC1918 172.16.x", "http://172.16.0.1"},
		{"RFC1918 172.31.255.255", "http://172.31.255.255"},
		{"RFC1918 192.168.x", "http://192.168.1.1"},
		{"RFC1918 192.168.1.1 admin", "http://192.168.1.1/admin"},
		// 非 http/https 协议
		{"file protocol", "file:///etc/passwd"},
		{"ftp protocol", "ftp://example.com"},
		{"gopher", "gopher://example.com"},
	}

	for _, tc := range blocked {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateBaseURL(tc.url)
			if err == nil {
				t.Errorf("validateBaseURL(%q) 应返回错误(拒绝 SSRF), 但返回 nil", tc.url)
			}
		})
	}
}

// TestValidateBaseURL_AllowedAddresses —— 允许正常公网/合法地址。
func TestValidateBaseURL_AllowedAddresses(t *testing.T) {
	allowed := []struct {
		name string
		url  string
	}{
		{"OpenAI", "https://api.openai.com"},
		{"DeepSeek", "https://api.deepseek.com/v1"},
		{"HTTP public", "http://example.com"},
		{"custom port", "https://my-llm-provider.example.com:8443"},
	}

	for _, tc := range allowed {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// 公网地址可能需 DNS 解析, 解析失败时 validateBaseURL 放行(不拦截合法域名)
			// 此处只断言不因"非法协议"或"私有 IP"被拒绝
			err := validateBaseURL(tc.url)
			if err != nil && (strings.Contains(err.Error(), "协议不支持") || strings.Contains(err.Error(), "受限地址")) {
				t.Errorf("validateBaseURL(%q) 不应拒绝合法公网地址: %v", tc.url, err)
			}
		})
	}
}

// TestListModels_SSRF_Blocked —— 回归: ListModels 对内网地址必须短路返回错误, 不发起出站请求。
func TestListModels_SSRF_Blocked(t *testing.T) {
	ssrfURLs := []string{
		"http://169.254.169.254",
		"http://192.168.0.1",
		"http://10.0.0.1",
		"http://127.0.0.1:11434",
	}
	for _, u := range ssrfURLs {
		u := u
		t.Run(u, func(t *testing.T) {
			p := &config.Provider{BaseURL: u, APIKey: ""}
			_, err := ListModels(p)
			if err == nil {
				t.Errorf("ListModels(%q) 应返回 SSRF 拒绝错误, 但返回 nil", u)
			}
		})
	}
}
