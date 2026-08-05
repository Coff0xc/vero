package scenarios

import (
	"strings"
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

func TestParseEndpoints(t *testing.T) {
	out := `endpoint: /rest/user/login
endpoint: /api/products
endpoint: /products
param: email
param: password
js: /main.js
endpoint: /admin
endpoint: /logo.png
`
	obs := ParseEndpoints(out, map[string]any{"target": "http://t:3000"})
	if len(obs) < 6 {
		t.Fatalf("应提取至少 6 条观察, got %d", len(obs))
	}
	// 敏感路径提升为 finding + medium; 普通路径留 endpoint; 静态资源被过滤。
	var adminHit, restHit, endpointHit, paramHit, jsHit, staticFiltered bool
	for _, o := range obs {
		if o.Kind == "finding" && strings.Contains(o.Label, "/admin") {
			adminHit = true
			if o.Severity != "medium" {
				t.Errorf("/admin 应为 medium, got %q", o.Severity)
			}
		}
		if o.Kind == "finding" && strings.Contains(o.Label, "/rest/user/login") {
			restHit = true // /rest/ 命中敏感特征 -> 提升为 finding(正确行为)
		}
		if o.Kind == "endpoint" && strings.Contains(o.Label, "/products") {
			endpointHit = true
		}
		if o.Kind == "endpoint" && strings.Contains(o.Label, "param email") {
			paramHit = true
		}
		if o.Kind == "endpoint" && strings.Contains(o.Label, "js /main.js") {
			jsHit = true
		}
		if strings.Contains(o.Key, "logo.png") {
			staticFiltered = true
		}
	}
	if !adminHit || !restHit || !endpointHit || !paramHit || !jsHit {
		t.Fatalf("未覆盖全部类型: admin=%v rest=%v endpoint=%v param=%v js=%v", adminHit, restHit, endpointHit, paramHit, jsHit)
	}
	if staticFiltered {
		t.Error("静态资源 .png 不应出现在端点/观察中")
	}
	// 证据逐字回查: excerpt 必须存在于原始输出。
	for _, o := range obs {
		if !strings.Contains(out, o.Excerpt) {
			t.Errorf("excerpt %q 不在原始输出中(证据断裂)", o.Excerpt)
		}
	}
}

func TestParseProbe(t *testing.T) {
	out := "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{\"access_token\":\"abc\"}\n---HTTP 200 len=28 ct=application/json---\n"
	obs := ParseProbe(out, map[string]any{"target": "http://t:3000", "path": "/api/secret"})
	if len(obs) == 0 {
		t.Fatal("应至少产出 endpoint 观察")
	}
	// D24: 强信号 access_token 应触发 finding(evidence=响应内容)。
	var sensitive bool
	for _, o := range obs {
		if o.Kind == "finding" && strings.Contains(o.Label, "access_token") {
			sensitive = true
			if !strings.Contains(out, o.Excerpt) {
				t.Errorf("sensitive finding 的 excerpt 不在输出中: %q", o.Excerpt)
			}
		}
	}
	if !sensitive {
		t.Error("含 access_token 的响应应触发敏感词 finding")
	}
}

// TestParseProbeNoFalsePositive —— D24: 含宽泛词(error/admin/debug/token/password)的
// 正常页面不应触发 finding(旧词表会误报刷屏)。
func TestParseProbeNoFalsePositive(t *testing.T) {
	out := "HTTP/1.1 200 OK\r\n\r\n<html><body>Error: 0 - welcome to admin dashboard, debug info below. Your password field is empty. token expires in 1h</body></html>\n---HTTP 200 len=120 ct=text/html---\n"
	obs := ParseProbe(out, map[string]any{"target": "http://t:3000", "path": "/"})
	for _, o := range obs {
		if o.Kind == "finding" {
			t.Errorf("正常页面含宽泛词不应触发 finding, got %q (label=%q)", o.Key, o.Label)
		}
	}
}

func TestExtractEndpointsRealFetch(t *testing.T) {
	// 工具自包含(内部 curl): 对可达目标真实抓取 + 提取。
	res := extractEndpoints(map[string]any{"target": "https://example.com"})
	if !res.Success {
		t.Skipf("网络不可达, 跳过真实抓取: %s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, "endpoint:") && !strings.Contains(res.Stdout, "js:") {
		t.Logf("example.com 提取结果: %s", tools.Clip(res.Stdout, 300))
	}
}

func TestExtractEndpointsStaticFilter(t *testing.T) {
	// 静态资源(如 .png)不应出现在端点列表。
	out := `endpoint: /logo.png
endpoint: /app
`
	obs := ParseEndpoints(out, map[string]any{"target": "http://t"})
	for _, o := range obs {
		if strings.Contains(o.Key, "logo.png") {
			t.Errorf("静态资源不应作为端点: %s", o.Key)
		}
	}
}
