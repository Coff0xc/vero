package scenarios

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

func TestPackRegisterAndRoute(t *testing.T) {
	reg := tools.NewRegistry()
	m := NewManager()
	RegisterDefaults(m, reg)

	// 1) 注册: 工具并入 Registry
	if !reg.Has("http_probe") || !reg.Has("smb_enum") {
		t.Fatal("场景包工具应注册进 Registry")
	}
	if kt, _ := reg.Get("kerberoast"); kt.Level != tools.LevelCred {
		t.Fatal("kerberoast 应是 L2 凭证操作")
	}

	// 2) 场景路由: 按 service 指纹激活对应包
	// 注意: CloudPack 总是激活 (fingerprint 返回 true)
	webRoutes := m.Route(map[string]bool{"http": true})
	if !containsStr(webRoutes, "web") {
		t.Fatalf("http 应路由到 web, got %v", webRoutes)
	}
	if !containsStr(webRoutes, "cloud") {
		t.Fatalf("cloud 应总是激活, got %v", webRoutes)
	}

	adRoutes := m.Route(map[string]bool{"ldap": true, "kerberos-sec": true})
	if !containsStr(adRoutes, "ad") {
		t.Fatalf("ldap/kerberos 应路由到 ad, got %v", adRoutes)
	}

	multiRoutes := m.Route(map[string]bool{"http": true, "microsoft-ds": true})
	if len(multiRoutes) < 3 {
		t.Fatalf("多指纹应激活多包 (至少 web/ad/cloud), got %v", multiRoutes)
	}

	noMatchRoutes := m.Route(map[string]bool{"ssh": true})
	if !containsStr(noMatchRoutes, "cloud") {
		t.Fatalf("无匹配指纹仍应激活 cloud 包, got %v", noMatchRoutes)
	}
}

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestParsers(t *testing.T) {
	o := ParseHTTP("HTTP/1.1 200 OK\nServer: nginx\nX-Powered-By: PHP/8.1", map[string]any{"target": "t"})
	if len(o) != 2 || o[0].Excerpt != "Server: nginx" {
		t.Fatalf("parse_http 应提取 2 条 tech 且留逐字来源, got %+v", o)
	}
	o2 := ParseSMB("SMB  10.0.0.6  (name:DC01) (domain:corp.local)", map[string]any{"target": "10.0.0.6"})
	if len(o2) == 0 || o2[0].Kind != "host" {
		t.Fatalf("parse_smb 应提取 host, got %+v", o2)
	}
	if o2[0].Label != "DC01.corp.local" {
		t.Fatalf("host label 应为 DC01.corp.local, got %q", o2[0].Label)
	}
}

// ParseNuclei 吃真实 nuclei -j (JSONL): 提取 finding, excerpt=matched-at(供逐字回查)。
// 样例取自 juice-shop 实扫的真实一行。
func TestParseNuclei(t *testing.T) {
	line := `{"template-id":"swagger-api","matched-at":"http://localhost:3000/api-docs/swagger.json","info":{"name":"Public Swagger API - Detect","severity":"info"},"type":"http"}`
	obs := ParseNuclei(line+"\nnot-json-noise", map[string]any{"target": "x"})
	if len(obs) != 1 {
		t.Fatalf("应解析 1 个 finding(跳过非 JSON 行), got %d", len(obs))
	}
	if obs[0].Kind != "finding" || obs[0].Label != "[info] Public Swagger API - Detect" {
		t.Fatalf("finding 解析错误: %+v", obs[0])
	}
	if obs[0].Excerpt != "http://localhost:3000/api-docs/swagger.json" {
		t.Fatalf("excerpt 应为 matched-at(供逐字回查), got %q", obs[0].Excerpt)
	}
}

func TestParseSQLi(t *testing.T) {
	ok := `{"authentication":{"token":"eyJ0eXAiOiJKV1Q...","umail":"admin@juice-sh.op"}}`
	obs := ParseSQLi(ok, map[string]any{"target": "http://localhost:3000"})
	if len(obs) != 1 || obs[0].Kind != "finding" {
		t.Fatalf("成功利用应产出 finding: %+v", obs)
	}
	if len(ParseSQLi(`{"error":"Invalid email or password"}`, map[string]any{"target": "x"})) != 0 {
		t.Fatal("失败响应不应误报利用成功")
	}
}

func TestBaseURL(t *testing.T) {
	cases := map[string]string{
		"http://localhost:3000/rest/user/login": "http://localhost:3000",
		"http://localhost:3000":                 "http://localhost:3000",
		"localhost:3000":                        "http://localhost:3000",
	}
	for in, want := range cases {
		if got := baseURL(in); got != want {
			t.Fatalf("baseURL(%q)=%q, want %q", in, got, want)
		}
	}
}
