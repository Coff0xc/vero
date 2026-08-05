package tools

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

// PortScan 真实建立 TCP 连接: 起一个本地 listener, 扫它应检测为 open,
// 且输出能被 ParseNmap 吃成 service —— 真实工具 + 复用 parser 的闭环。
func TestPortScanDetectsOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	res := PortScan(map[string]any{"target": "127.0.0.1", "ports": []int{port}})
	if !res.Success {
		t.Fatalf("扫描应成功: %+v", res)
	}
	if !strings.Contains(res.Stdout, fmt.Sprintf("%d/tcp open", port)) {
		t.Fatalf("应检测到开放端口 %d: %q", port, res.Stdout)
	}

	// 复用 ParseNmap 吃 PortScan 输出
	obs := ParseNmap(res.Stdout, map[string]any{"target": "127.0.0.1"})
	hasService := false
	for _, o := range obs {
		if o.Kind == "service" {
			hasService = true
		}
	}
	if !hasService {
		t.Fatalf("ParseNmap 应能吃 PortScan 输出提取 service: %+v", obs)
	}
}

func TestPortScanClosedPort(t *testing.T) {
	// 极不可能开放的高端口
	res := PortScan(map[string]any{"target": "127.0.0.1", "ports": []int{1}})
	if !res.Success {
		t.Fatal("扫描本身应成功(即使无开放端口)")
	}
	if strings.Contains(res.Stdout, "/tcp open") {
		t.Skip("端口 1 意外开放, 跳过")
	}
}

// TestNormalizeHostIPv6 —— D8: 裸 IPv6(多冒号)不得被误剥端口, [::1]:80 正确归一化。
func TestNormalizeHostIPv6(t *testing.T) {
	cases := map[string]string{
		"2001:db8::1":                  "2001:db8::1", // 裸 IPv6: 末段数字是地址不是端口
		"[::1]:80":                     "[::1]",
		"http://[::1]:8080/x":          "[::1]",
		"10.0.0.5:80":                  "10.0.0.5",
		"http://file.nciyuan.net:8080": "file.nciyuan.net",
		"file.nciyuan.net":             "file.nciyuan.net",
	}
	for in, want := range cases {
		if got := normalizeHost(in); got != want {
			t.Errorf("normalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}
