// Web 端工具管理 API
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

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

// handleToolInstall —— POST /api/tools/install {name, type?} - 自动下载缺失工具。
// body.type 可选: 缺省按 name 自动判定(binary|pip); 提供时必须与 name 的 install_type 一致。
// binary → InstallBinary(SHA256 白名单下载到 tools/bin); pip → InstallPip(--user 安装)。
func (s *Server) handleToolInstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "body 解析失败"})
		return
	}
	if body.Name == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"ok": false, "name": body.Name, "error": "name required"})
		return
	}

	it := tools.InstallType(body.Name)
	if body.Type != "" {
		if body.Type != "binary" && body.Type != "pip" {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"ok": false, "name": body.Name, "error": "invalid type: 期望 binary|pip"})
			return
		}
		if body.Type != it {
			writeJSONStatus(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "name": body.Name, "type": body.Type, "error": fmt.Sprintf("type 不匹配: %s 为 %s 而非 %s", body.Name, it, body.Type)})
			return
		}
	} else {
		body.Type = it
	}
	if it == "none" {
		writeJSONStatus(w, http.StatusUnprocessableEntity, map[string]any{"ok": false, "name": body.Name, "type": "none", "error": fmt.Sprintf("%s 不可自动安装 (install_type=none)", body.Name)})
		return
	}

	s.installMu.Lock()
	defer s.installMu.Unlock() // 单装/批量共用, 防并发下载

	switch body.Type {
	case "binary":
		path, err := tools.InstallBinary(body.Name)
		if err != nil {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]any{"ok": false, "name": body.Name, "type": body.Type, "error": "安装失败: " + err.Error()})
			return
		}
		writeJSON(w, map[string]any{
			"ok": true, "name": body.Name, "type": body.Type, "path": path,
			"detail": fmt.Sprintf("下载 %s (SHA256 校验通过, 已解压到 tools/bin)", body.Name),
		})
	case "pip":
		path, err := tools.InstallPip(body.Name)
		if err != nil {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]any{"ok": false, "name": body.Name, "type": body.Type, "error": "安装失败: " + err.Error()})
			return
		}
		writeJSON(w, map[string]any{
			"ok": true, "name": body.Name, "type": body.Type, "path": path,
			"detail": tools.PipInstallCommand(body.Name),
		})
	}
}

// installAllItem —— 批量安装结果中的单项。
type installAllItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	OK     bool   `json:"ok"`
	Path   string `json:"path,omitempty"`  // ok=true 时
	Detail string `json:"detail,omitempty"` // ok=true 时动作描述
	Error  string `json:"error"`           // ok=false 时非空, ok=true 时 ""
}

// handleToolInstallAll —— POST /api/tools/install-all - 批量自动安装缺失工具。
// 无 body(或空 {}): 对 verify 中 available=false 且 install_type!=none 的工具按序安装;
// 支持 {names?: string[], types?: ("binary"|"pip")[]} 过滤。
// 全程持 installMu 串行; 单项失败不影响其余项; 部分失败整体仍 200; 前端随后重新 verify。
func (s *Server) handleToolInstallAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Names []string `json:"names"`
		Types []string `json:"types"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	for _, t := range body.Types {
		if t != "binary" && t != "pip" {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "names 或 types 非法"})
			return
		}
	}
	nameFilter := map[string]bool{}
	for _, n := range body.Names {
		nameFilter[n] = true
	}
	typeFilter := map[string]bool{}
	for _, t := range body.Types {
		typeFilter[t] = true
	}

	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)

	// 与 verify 等价的即时扫描, 收集缺失且可自动安装的工具。
	type job struct {
		name string
		typ  string
	}
	var jobs []job
	for _, r := range tooltest.VerifyAll(reg) {
		if r.Available || r.InstallType == "" || r.InstallType == "none" {
			continue
		}
		if len(nameFilter) > 0 && !nameFilter[r.Name] {
			continue
		}
		if len(typeFilter) > 0 && !typeFilter[r.InstallType] {
			continue
		}
		jobs = append(jobs, job{name: r.Name, typ: r.InstallType})
	}

	s.installMu.Lock()
	defer s.installMu.Unlock()

	resp := struct {
		OK        bool             `json:"ok"`
		Total     int              `json:"total"`
		Succeeded int              `json:"succeeded"`
		Installed int              `json:"installed"` // 兼容旧前端字段名
		Failed    int              `json:"failed"`
		Results   []installAllItem `json:"results"`
	}{OK: true, Results: []installAllItem{}}

	for _, j := range jobs {
		item := installAllItem{Name: j.name, Type: j.typ}
		switch j.typ {
		case "binary":
			path, err := tools.InstallBinary(j.name)
			if err != nil {
				item.Error = err.Error()
				break
			}
			item.OK = true
			item.Path = path
			item.Detail = fmt.Sprintf("下载 %s (SHA256 校验通过, 已解压到 tools/bin)", j.name)
		case "pip":
			path, err := tools.InstallPip(j.name)
			if err != nil {
				item.Error = err.Error()
				break
			}
			item.OK = true
			item.Path = path
			item.Detail = tools.PipInstallCommand(j.name)
		}
		if item.OK {
			resp.Succeeded++
			resp.Installed++
		} else {
			resp.Failed++
		}
		resp.Results = append(resp.Results, item)
	}
	resp.Total = len(resp.Results)
	writeJSON(w, resp)
}

// writeJSONStatus —— 写 JSON 响应并指定状态码。
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
// 安全约束: 与战役同规则 —— L3+ 动作必须人工审批(HITL), 全部动作走审计, 拒绝并发执行。
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

	// 拒绝并发: 战役或工作流运行中不接受新任务(修原版可无限并发启动)。
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		http.Error(w, "campaign already running", http.StatusConflict)
		return
	}
	s.busy = true
	ctx, cancel := context.WithCancel(context.Background())
	s.stop = cancel
	s.mu.Unlock()

	// 转换工作流为 Campaign 并异步执行
	go func() {
		defer func() {
			s.mu.Lock()
			s.busy = false
			s.stop = nil
			s.mu.Unlock()
		}()
		s.runWorkflow(ctx, tpl, body.Target, reg)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "started",
		"workflow": tpl.Name,
		"target":   body.Target,
	})
}

// runWorkflow —— 执行工作流所有阶段。并行分支用 WaitGroup 等待完成,
// 修原版"提前发 complete、后台任务无人等待"的 goroutine 泄漏; 支持 ctx 取消。
func (s *Server) runWorkflow(ctx context.Context, tpl *workflow.Template, target string, reg *tools.Registry) {
	// 广播开始事件
	s.broker.Emit(core.Event{
		Kind: "workflow_start",
		Data: map[string]any{
			"workflow": tpl.Name,
			"target":   target,
		},
	})

	// 执行每个阶段
	for _, stage := range tpl.Stages {
		if ctx.Err() != nil {
			s.broker.Emit(core.Event{Kind: "workflow_cancelled", Data: map[string]any{"workflow": tpl.Name, "target": target}})
			return
		}
		s.broker.Emit(core.Event{
			Kind: "workflow_stage",
			Data: map[string]any{
				"stage": stage.Name,
				"desc":  stage.Description,
			},
		})

		var wg sync.WaitGroup
		for _, toolName := range stage.Tools {
			wg.Add(1)
			go func(n string) {
				defer wg.Done()
				s.executeWorkflowTool(ctx, n, target, reg)
			}(toolName)
		}
		wg.Wait() // 无论顺序/并行都等本阶段全部完成后再进下一阶段

		if ctx.Err() != nil {
			s.broker.Emit(core.Event{Kind: "workflow_cancelled", Data: map[string]any{"workflow": tpl.Name, "target": target}})
			return
		}
	}

	s.broker.Emit(core.Event{
		Kind: "workflow_complete",
		Data: map[string]any{
			"workflow": tpl.Name,
			"target":   target,
		},
	})
}

// executeWorkflowTool —— 执行单个工具。与 RunAgent 同安全规则:
// L3+(利用)必须先过 HITL 人工审批, 每个执行动作写审计日志。
func (s *Server) executeWorkflowTool(ctx context.Context, toolName, target string, reg *tools.Registry) {
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

	// HITL 门控: 与战役同一阈值(L3+), 修"工作流绕过审批直接执行高危动作"。
	action := core.Action{Tool: toolName, Args: args, Rationale: "工作流执行"}
	if tool.Level >= core.HITLThreshold {
		approved := s.gate.ApproveCtx(ctx, action, tool.Level)
		if !approved {
			s.broker.Emit(core.Event{Kind: "hitl", Data: map[string]any{"action": toolName, "approved": false}})
			return
		}
	}

	// 执行工具
	result := tool.Run(args)

	// 审计(与战役一致)
	succ := result.Success
	_ = s.auditor.Record(toolName, args, tool.Level, &succ, nil)

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
