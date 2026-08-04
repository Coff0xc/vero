package tools

import "runtime"

// GetPlatform —— 获取当前运行平台
func GetPlatform() string {
	return runtime.GOOS // windows, linux, darwin
}

// IsPlatformCompatible —— 检查工具是否与当前平台兼容
func IsPlatformCompatible(toolPlatform string) bool {
	if toolPlatform == "" || toolPlatform == "all" {
		return true
	}
	return toolPlatform == runtime.GOOS
}
