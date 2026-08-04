// Package server —— 操作员作战室后端(对应 Python server.py)。
//
// chi 路由 + SSE 广播 + Web HITL + 战役编排 + SQLite 持久化。
// SSE 单向推送够用(前端 -> 后端用普通 POST), 比 WebSocket 更贴场景。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Coff0xc/vero/internal/audit"
	"github.com/Coff0xc/vero/internal/config"
	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/store"
	"github.com/Coff0xc/vero/internal/tools"
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
	busy bool             // 同一时刻只跑一个战役
	stop context.CancelFunc // 当前战役的取消句柄(操作员点"停止"时触发)

	installMu sync.Mutex // 工具自动安装防并发
	cfg       *config.Config

	ctxMu   sync.Mutex
	lastCtx *campaignCtx // 最近一次战役的上下文(对话式问答感知用)

	cfgMu sync.Mutex // 配置并发保护(handleConfigSet 与战役/chat 并发读, 修 data race)
}

// New —— 组装 server。webFS 为前端静态资源(embed 或 os.DirFS), 其根含 index.html。
func New(st *store.Store, auditor *audit.Auditor, webFS fs.FS) *Server {
	broker := NewBroker()
	sm := scenarios.NewManager()
	scenarios.RegisterDefaults(sm, tools.NewRegistry()) // manager 仅用于 Route 展示
	cfg := config.Load()
	if cfg.Model != "" { // 配置的模型名 -> 引擎侧 VERO_MODEL(兼容 env)
		_ = os.Setenv("VERO_MODEL", cfg.Model)
	}
	tools.EnsurePath()
	return &Server{
		broker:    broker,
		gate:      NewWebGate(broker),
		store:     st,
		auditor:   auditor,
		scenarios: sm,
		webFS:     webFS,
		cfg:       cfg,
	}
}

// Router —— 装配路由。
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(originGuard) // 跨站防护: 非本源的浏览器请求一律 403(SSE 监听 / 任意页面触发攻击)
	r.Get("/events", s.handleEvents)
	r.Post("/start", s.handleStart)
	r.Post("/approve", s.handleApprove)
	r.Get("/api/approvals/pending", s.handleApprovalsPending)
	r.Post("/cancel", s.handleCancel)
	r.Post("/chat", s.handleChat) // 对话式问答(感知当前战役上下文)
	r.Get("/healthz", s.handleHealth)
	r.Get("/api/campaigns", s.handleCampaigns)
	r.Delete("/api/campaigns/{id}", s.handleCampaignDelete)

	// 报告导出端点（新增）
	r.Get("/api/reports", s.handleReportsList)
	r.Get("/api/campaigns/{id}/report.json", s.handleReportJSON)
	r.Get("/api/campaigns/{id}/report.md", s.handleReportMarkdown)
	r.Get("/api/campaigns/{id}/events", s.handleCampaignEvents)
	r.Post("/api/campaigns/{id}/report", s.handleReportGenerate)
	r.Get("/api/campaigns/{id}/report.html", s.handleReportHTML)

	// 工具管理 API
	r.Get("/api/tools", s.handleToolList)
	r.Get("/api/tools/verify", s.handleToolVerify)  // 契约改为 GET
	r.Post("/api/tools/verify", s.handleToolVerify) // 兼容旧调用
	r.Post("/api/tools/install", s.handleToolInstall)
	r.Post("/api/tools/install-all", s.handleToolInstallAll)

	// 工作台配置 API
	r.Get("/api/config", s.handleConfigGet)
	r.Post("/api/config", s.handleConfigSet)
	r.Post("/api/providers/test", s.handleProviderTest) // 测试提供商连接 + 拉模型列表
	r.Get("/api/dependencies", s.handleDependencies)    // 工具依赖检测(抄 Dark-Moon)

	// 工作流模板 API
	r.Get("/api/workflows", s.handleWorkflowList)
	r.Get("/api/workflows/{id}", s.handleWorkflowGet)
	r.Post("/api/workflows/{id}/execute", s.handleWorkflowExecute)

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

	// 修复 D23: 校验 target 参数
	if body.Target == "" {
		writeJSON(w, map[string]any{"ok": false, "err": "target 参数为空"})
		return
	}

	// 校验 URL 格式
	target := strings.TrimSpace(body.Target)
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		// 尝试解析为主机名或IP
		if !strings.Contains(target, "://") {
			// 如果没有协议，默认添加 http://
			target = "http://" + target
		} else {
			writeJSON(w, map[string]any{"ok": false, "err": "target 必须以 http:// 或 https:// 开头，或提供主机名/IP"})
			return
		}
	}

	// 验证 URL 可解析
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		writeJSON(w, map[string]any{"ok": false, "err": "无效的 target URL: " + target})
		return
	}

	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		writeJSON(w, map[string]any{"ok": false, "err": "campaign already running"})
		return
	}
	s.busy = true
	ctx, cancel := context.WithCancel(context.Background())
	s.stop = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.busy = false
			s.stop = nil
			s.mu.Unlock()
		}()
		s.RunCampaign(ctx, target)
	}()
	writeJSON(w, map[string]any{"ok": true})
}

// handleCancel —— 操作员停止当前战役(取消上下文; 核心循环与 HITL 等待都会响应)。
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	stop := s.stop
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleHealth —— 真实健康检查: 探活 SQLite + 确认 HTTP 层可用。
// 修 docker-compose 只查 SPA 页面 200 的假健康: DB 挂了页面照样 200。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(); err != nil {
		http.Error(w, `{"status":"unhealthy","error":"`+err.Error()+`"}`, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]any{"status": "ok"})
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

// handleCampaignDelete —— 删除战役(对话 UI 的删除历史会话)。
func (s *Server) handleCampaignDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		http.Error(w, "无效的战役 ID", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteCampaign(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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

// devAllowedPorts —— 本地回环(loopback)开发前端的允许端口(vite dev 默认 5173)。
// 仅当 Origin 与本服务同为回环地址、仅端口不同(反代开发模式)时放行;
// 对外网/局域网来源仍按完整 host:port 严格比较, 防护不削弱。
var devAllowedPorts = map[string]bool{"5173": true}

// isLoopback —— host(可含端口)是否为回环地址(localhost / 127.x / ::1)。
func isLoopback(hostport string) bool {
	h := hostnameOf(hostport)
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// originGuard —— 跨站防护中间件: 浏览器发出的请求必带 Origin。
// Origin 与本服务不同源的请求直接 403(阻止任意网页: 偷听 SSE / 触发战役 / 冒名审批)。
// 例外: 回环 dev 前端(vite :5173 反代)在"双端均回环"前提下放行, 仅端口不同。
// 非浏览器客户端(curl/健康检查)不带 Origin, 不受影响。
func originGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !originAllowed(u, r.Host) {
				http.Error(w, "forbidden: cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed —— Origin 是否可信: 完整 host:port 相同, 或同为回环的 dev 端口白名单。
func originAllowed(u *url.URL, reqHost string) bool {
	if u.Host == reqHost { // 同源(生产: 前端 embed 进同一端口)
		return true
	}
	// dev 模式: 双端都是回环, 且前端端口在白名单(vite), 后端任意回环端口。
	_, oport, oerr := net.SplitHostPort(u.Host)
	if oerr == nil && devAllowedPorts[oport] && isLoopback(u.Host) && isLoopback(reqHost) {
		return true
	}
	return false
}

// hostnameOf —— 从 "host:port" 里剥离端口取 hostname。
func hostnameOf(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}

// handleApprovalsPending —— GET /api/approvals/pending: 当前挂起的审批 key 列表。
// 前端 SSE 重连时先拉一次, 补回断线期间丢失的 hitl_request(修 #6)。
func (s *Server) handleApprovalsPending(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"pending": s.gate.Pending()})
}
