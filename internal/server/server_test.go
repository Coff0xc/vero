package server

import "testing"

// TestValidateTarget —— D15: 拒绝空/畸形/非法字符 target, 合法输入归一化补 scheme。
func TestValidateTarget(t *testing.T) {
	valid := map[string]string{
		"file.nciyuan.net":              "http://file.nciyuan.net",
		"10.0.0.5:8080":                 "http://10.0.0.5:8080",
		"http://file.nciyuan.net:80/x":  "http://file.nciyuan.net:80/x",
		"https://host:8443/login":       "https://host:8443/login",
		"[::1]:80":                      "http://[::1]:80",
		"http://[::1]:8080":             "http://[::1]:8080",
	}
	for in, want := range valid {
		got, err := validateTarget(in)
		if err != nil {
			t.Errorf("validateTarget(%q) 应通过: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("validateTarget(%q) = %q, want %q", in, got, want)
		}
	}

	invalid := []string{
		"",                    // 空
		"   ",                 // 纯空白
		"http://",             // 无 host
		"http:///path",        // 空 host
		"ftp://host/x",        // 非 http(s) scheme
		"http://a b",          // host 含空格
		"http://host\tx",      // 含制表符
		"http://[::1",         // IPv6 括号未闭合
		"http://h!st:80",      // 非法字符
		"http://host:port",    // 非数字端口
	}
	for _, in := range invalid {
		if got, err := validateTarget(in); err == nil {
			t.Errorf("validateTarget(%q) 应被拒绝, got %q", in, got)
		}
	}
}
