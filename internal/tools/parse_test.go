package tools

import "testing"

func TestParseNmap(t *testing.T) {
	obs := ParseNmap("Host 10.0.0.5 is up\n22/tcp open ssh\n80/tcp open http",
		map[string]any{"target": "10.0.0.5"})
	if len(obs) != 3 {
		t.Fatalf("应提取 1 host + 2 service, got %d", len(obs))
	}
	if obs[0].Kind != "host" {
		t.Fatal("首条应为 host")
	}
	if obs[1].Kind != "service" || obs[1].Key != "10.0.0.5:22" {
		t.Fatalf("应提取 ssh service, got %+v", obs[1])
	}
	if obs[1].Excerpt != "22/tcp open ssh" {
		t.Fatalf("service 应留逐字来源, got %q", obs[1].Excerpt)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltins(r)
	if !r.Has("fake_scan") {
		t.Fatal("应注册 fake_scan")
	}
	if _, ok := r.Get("nmap_ping"); !ok {
		t.Fatal("应能取到 nmap_ping")
	}
	if !r.Has("nmap_scan") {
		t.Fatal("应注册 nmap_scan")
	}
	if names := r.Names(); len(names) != 3 {
		t.Fatalf("应有 3 个内置工具, got %v", names)
	}
}
