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
	missing := tools.CheckDependencies()
	writeJSON(w, map[string]any{
		"dependencies":  deps,
		"missing_count": len(missing),
		"all_ready":     len(missing) == 0,
	})
}
