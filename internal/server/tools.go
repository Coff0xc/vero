// Web 端工具管理 API
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tooltest"
	"github.com/Coff0xc/vero/internal/tools"
	"github.com/Coff0xc/vero/internal/workflow"
)

// handleToolList —— GET /api/tools - 列出所有工具
func (s *Server) handleToolList(w http.ResponseWriter, r *http.Request) {
	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)

	specs := reg.Specs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total": len(specs),
		"tools": specs,
	})
}

// handleToolVerify —— POST /api/tools/verify - 验证工具可用性
func (s *Server) handleToolVerify(w http.ResponseWriter, r *http.Request) {
	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)

	results := tooltest.VerifyAll(reg)

	available := 0
	for _, r := range results {
		if r.Available {
			available++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total":     len(results),
		"available": available,
		"results":   results,
		"summary":   tooltest.Summary(results),
	})
}

// handleWorkflowList —— GET /api/workflows - 列出所有工作流模板
func (s *Server) handleWorkflowList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total":     len(workflow.Templates),
		"workflows": workflow.Templates,
	})
}

// handleWorkflowGet —— GET /api/workflows/:id - 获取工作流详情
func (s *Server) handleWorkflowGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tpl := workflow.GetByID(id)

	if tpl == nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}

	// 验证工具可用性
	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)

	missing := workflow.ValidateTemplate(*tpl, reg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"workflow":      tpl,
		"missing_tools": missing,
		"valid":         len(missing) == 0,
	})
}

// handleWorkflowExecute —— POST /api/workflows/:id/execute - 执行工作流
func (s *Server) handleWorkflowExecute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tpl := workflow.GetByID(id)

	if tpl == nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}

	// TODO: 实现工作流执行逻辑（集成到现有 campaign 系统）
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "pending",
		"message": "workflow execution will be implemented",
		"workflow": tpl,
	})
}
