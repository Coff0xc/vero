//go:build !windows

package tools

import "net/url"

// windowsProxy —— 非 Windows 平台无 IE 注册表, 返回 nil(走环境变量代理)。
func windowsProxy() (*url.URL, error) { return nil, nil }
