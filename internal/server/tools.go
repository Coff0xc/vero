// Web 端工具管理 API
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Coff0xc/vero/internal/core"
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

	// 解析请求体（获取目标）
	var body struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Target == "" {
		http.Error(w, "target required", http.StatusBadRequest)
		return
	}

	// 验证工具可用性
	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)

	missing := workflow.ValidateTemplate(*tpl, reg)
	if len(missing) > 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":        "error",
			"message":       "missing tools",
			"missing_tools": missing,
		})
		return
	}

	// 转换工作流为 Campaign 并异步执行
	go func() {
		// 广播开始事件
		s.broker.Emit(core.Event{
			Kind: "workflow_start",
			Data: map[string]any{
				"workflow": tpl.Name,
				"target":   body.Target,
			},
		})

		// 执行每个阶段
		for _, stage := range tpl.Stages {
			s.broker.Emit(core.Event{
				Kind: "workflow_stage",
				Data: map[string]any{
					"stage": stage.Name,
					"desc":  stage.Description,
				},
			})

			// 按顺序或并行执行工具
			if stage.Sequential {
				for _, toolName := range stage.Tools {
					s.executeWorkflowTool(toolName, body.Target, reg)
				}
			} else {
				// 并行执行（简化版）
				for _, toolName := range stage.Tools {
					go s.executeWorkflowTool(toolName, body.Target, reg)
				}
			}
		}

		s.broker.Emit(core.Event{
			Kind: "workflow_complete",
			Data: map[string]any{
				"workflow": tpl.Name,
				"target":   body.Target,
			},
		})
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "started",
		"workflow": tpl.Name,
		"target":   body.Target,
	})
}

// executeWorkflowTool —— 执行单个工具
func (s *Server) executeWorkflowTool(toolName, target string, reg *tools.Registry) {
	tool, ok := reg.Get(toolName)
	if !ok {
		s.broker.Emit(core.Event{
			Kind: "tool_error",
			Data: map[string]any{
				"tool":  toolName,
				"error": "tool not found",
			},
		})
		return
	}

	// 构造参数
	args := map[string]any{"target": target}

	// 执行工具
	result := tool.Run(args)

	// 广播结果
	s.broker.Emit(core.Event{
		Kind: "tool_result",
		Data: map[string]any{
			"tool":    toolName,
			"success": result.Success,
			"stdout":  result.Stdout,
			"stderr":  result.Stderr,
		},
	})
}
