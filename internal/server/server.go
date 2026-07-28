// Package server —— 操作员作战室后端(对应 Python server.py)。
//
// chi 路由 + SSE 广播 + Web HITL + 战役编排 + SQLite 持久化。
// SSE 单向推送够用(前端 -> 后端用普通 POST), 比 WebSocket 更贴场景。
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"redcell/internal/audit"
	"redcell/internal/scenarios"
	"redcell/internal/store"
	"redcell/internal/tools"
)

// Server —— 作战室后端: SSE 推送 + Web HITL + 战役编排 + 持久化。
type Server struct {
	broker    *Broker
	gate      *WebGate
	store     *store.Store
	auditor   *audit.Auditor
	scenarios *scenarios.Manager
	webFS     fs.FS

	mu   sync.Mutex
	busy bool // 同一时刻只跑一个战役
}

// New —— 组装 server。webFS 为前端静态资源(embed 或 os.DirFS), 其根含 index.html。
func New(st *store.Store, auditor *audit.Auditor, webFS fs.FS) *Server {
	broker := NewBroker()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, tools.NewRegistry()) // manager 仅用于 Route 展示
	return &Server{
		broker:    broker,
		gate:      NewWebGate(broker),
		store:     st,
		auditor:   auditor,
		scenarios: sm,
		webFS:     webFS,
	}
}

// Router —— 装配路由。
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/events", s.handleEvents)
	r.Post("/start", s.handleStart)
	r.Post("/approve", s.handleApprove)
	r.Get("/api/campaigns", s.handleCampaigns)

	// 报告导出端点（新增）
	r.Get("/api/reports", s.handleReportsList)
	r.Get("/api/campaigns/{id}/report.json", s.handleReportJSON)
	r.Get("/api/campaigns/{id}/report.md", s.handleReportMarkdown)
	r.Get("/api/campaigns/{id}/report.html", s.handleReportHTML)

	r.Handle("/*", s.handleStatic())
	return r
}

// handleEvents —— SSE 事件流: 订阅 broker, 逐条推送直到断开。
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsub := s.broker.Subscribe()
	defer unsub()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(map[string]any{"kind": e.Kind, "data": e.Data})
			if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// handleStart —— 启动一场战役(后台 goroutine; 拒绝并发战役)。
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		writeJSON(w, map[string]any{"ok": false, "err": "campaign already running"})
		return
	}
	s.busy = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.busy = false
			s.mu.Unlock()
		}()
		s.RunCampaign(body.Target)
	}()
	writeJSON(w, map[string]any{"ok": true})
}

// handleApprove —— 操作员对某待审批动作裁决。
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key      string `json:"key"`
		Approved bool   `json:"approved"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.gate.Resolve(body.Key, body.Approved)
	writeJSON(w, map[string]any{"ok": true})
}

// handleCampaigns —— 最近战役列表(回溯用)。
func (s *Server) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	cs, err := s.store.ListCampaigns(20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, cs)
}

// handleStatic —— 前端静态资源, SPA fallback 到 index.html。
func (s *Server) handleStatic() http.Handler {
	fileServer := http.FileServer(http.FS(s.webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.webFS, p); err != nil {
			b, err := fs.ReadFile(s.webFS, "index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
