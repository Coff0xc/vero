package tools

import (
	"testing"
)

func TestDependencyCheck(t *testing.T) {
	// 测试依赖检测逻辑(不依赖实际工具安装)
	fakeDep := Dependency{
		Binary:      "nonexistent_tool_xyz",
		DisplayName: "假工具",
		InstallHint: "不存在",
	}
	if fakeDep.IsInstalled() {
		t.Error("不存在的工具应判定为未安装")
	}

	// Go 本身应该存在(运行测试的前提)
	goDep := Dependency{
		Binary:   "go",
		CheckCmd: []string{"go", "version"},
	}
	if !goDep.IsInstalled() {
		t.Error("Go 应该已安装(运行测试的前提)")
	}
	if ver := goDep.Version(); ver == "" {
		t.Error("Go 版本应能获取")
	}
}

func TestDepsReport(t *testing.T) {
	report := DepsReport()
	if report == "" {
		t.Error("依赖报告不应为空")
	}
	t.Logf("依赖报告:\n%s", report)
}
