package core

import "testing"

// TestKeyHostPort —— U2: observation Key 的 host/port 提取(URL/裸 host:port/自定义格式)。
func TestKeyHostPort(t *testing.T) {
	cases := []struct {
		key      string
		wantHost string
		wantPort string
	}{
		// URL 带 scheme + 自定义段(U2 真实场景: finding Key = http://host:tech:Via)
		{"http://file.nciyuan.net:tech:Via: 0.0 Caddy", "file.nciyuan.net", ""},
		{"http://file.nciyuan.net:8080:tech:Via", "file.nciyuan.net", "8080"},
		// 裸 host:port
		{"10.0.0.5:80", "10.0.0.5", "80"},
		{"file.nciyuan.net:443", "file.nciyuan.net", "443"},
		// 带路径的 URL
		{"http://file.nciyuan.net/index.php?user/view/manifest", "file.nciyuan.net", ""},
		{"https://host:8443/login", "host", "8443"},
		// 空/无 host
		{"", "", ""},
		{":80", "", ""},
	}
	for _, tc := range cases {
		host, port := keyHostPort(tc.key)
		if host != tc.wantHost || port != tc.wantPort {
			t.Errorf("keyHostPort(%q) = (%q, %q), want (%q, %q)", tc.key, host, port, tc.wantHost, tc.wantPort)
		}
	}
}

// TestNormHost —— host 节点 ID 归一化(scheme/路径/端口剥离)。
func TestNormHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://file.nciyuan.net", "file.nciyuan.net"},
		{"http://file.nciyuan.net:8080", "file.nciyuan.net"},
		{"https://host:8443/login", "host"},
		{"10.0.0.5:80", "10.0.0.5"},
		{"10.0.0.5", "10.0.0.5"},
		{"[::1]:80", "[::1]"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normHost(tc.in); got != tc.want {
			t.Errorf("normHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNormTarget —— U4: 停滞检测 target 归一化(http://host:port 与 host:port 同 sig)。
func TestNormTarget(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://file.nciyuan.net:8080", "file.nciyuan.net:8080"},
		{"file.nciyuan.net:8080", "file.nciyuan.net:8080"},
		{"http://host/", "host"},
		{"  https://host:443/login/  ", "host:443/login"},
	}
	for _, tc := range cases {
		if got := normTarget(tc.in); got != tc.want {
			t.Errorf("normTarget(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestStableArgsSigTargetNormalized —— U4: 仅 target 格式不同(scheme/尾斜杠)时 sig 必须相同。
func TestStableArgsSigTargetNormalized(t *testing.T) {
	a := stableArgsSig(map[string]any{"tool": "http_probe", "target": "http://file.nciyuan.net:8080"})
	b := stableArgsSig(map[string]any{"tool": "http_probe", "target": "file.nciyuan.net:8080"})
	if a != b {
		t.Errorf("target 格式差异不应改变 sig: %q != %q", a, b)
	}
	// 控制组: 参数确实不同时 sig 应不同
	c := stableArgsSig(map[string]any{"tool": "http_probe", "target": "file.nciyuan.net:9090"})
	if a == c {
		t.Errorf("target 端口不同时 sig 应不同: %q == %q", a, c)
	}
}
