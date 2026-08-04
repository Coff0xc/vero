package server

import (
	"net/http"

	"github.com/Coff0xc/vero/internal/tools"
)

// handleDependencies —— GET /api/dependencies
// 工具依赖检测(抄 Dark-Moon): 返回核心工具清单 + 安装状态 + 缺失安装提示。
// 前端启动时调用, 在 UI 顶部展示缺失工具告警 + 一键安装指引(如 "3 个工具缺失, 点击查看")。
func (s *Server) handleDependencies(w http.ResponseWriter, r *http.Request) {
	type depItem struct {
		Binary      string `json:"binary"`
		DisplayName string `json:"display_name"`
		Installed   bool   `json:"installed"`
		Version     string `json:"version,omitempty"`
		InstallHint string `json:"install_hint,omitempty"`
		Platform    string `json:"platform,omitempty"` // 新增: 工具平台
	}
	var deps []depItem
	for _, d := range tools.CoreDependencies {
		item := depItem{
			Binary:      d.Binary,
			DisplayName: d.DisplayName,
			Installed:   d.IsInstalled(),
			InstallHint: d.InstallHint,
		}
		if item.Installed {
			item.Version = d.Version()
		}
		deps = append(deps, item)
	}

	// 统计平台工具数量
	reg, _ := buildLiveRegistry()
	platformTools := map[string]int{
		"windows": 0,
		"linux":   0,
		"darwin":  0,
		"all":     0,
	}

	for _, name := range reg.Names() {
		tool, _ := reg.Get(name)
		if tool == nil {
			continue
		}
		platform := tool.Platform
		if platform == "" {
			platform = "all"
		}
		platformTools[platform]++
	}

	missing := tools.CheckDependencies()
	writeJSON(w, map[string]any{
		"dependencies":   deps,
		"missing_count":  len(missing),
		"all_ready":      len(missing) == 0,
		"platform":       tools.GetPlatform(),
		"platform_tools": platformTools,
	})
}
