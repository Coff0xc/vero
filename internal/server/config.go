// 工作台配置 API —— 引擎/多提供商/API key/模型/思考强度/预算/YOLO/深度思考。
// 密钥只写盘(0600)不回显: GET 只返回"是否已配置"。
package server

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/Coff0xc/vero/internal/config"
	"github.com/Coff0xc/vero/internal/llm"
)

// handleConfigGet —— GET /api/config
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	s.cfgMu.Lock()
	pub := s.cfg.Public()
	s.cfgMu.Unlock()
	writeJSON(w, pub)
}

// handleConfigSet —— POST /api/config
// 请求体字段可部分提交; 空 key 字段表示"不改"(不清空), 显式清空用 "clear":true。
// providers 支持整组替换(设置面板的提供商管理提交完整列表)。
func (s *Server) handleConfigSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Engine        *config.Engine    `json:"engine"`
		AnthropicKey  string            `json:"anthropic_key"`
		DeepSeekKey   string            `json:"deepseek_key"`
		ClearAnthropic bool             `json:"clear_anthropic"`
		ClearDeepSeek bool              `json:"clear_deepseek"`
		Model         string            `json:"model"`
		Temperature   *float64          `json:"temperature"`
		MaxBudget     *int              `json:"max_budget"`
		YOLO          *bool             `json:"yolo"`
		DeepThinking  *bool             `json:"deep_thinking"`
		Providers     []*config.Provider `json:"providers"`      // 整组替换(保留原 key: 空 api_key 字段 = 保持原值)
		ActiveProvider string           `json:"active_provider"` // 当前启用提供商
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// 指针替换式更新: clone 后改字段再整体替换, 读方拿到旧指针后不受后续写影响(修 data race)。
	s.cfgMu.Lock()
	nc := *s.cfg
	s.cfg = &nc
	if body.Engine != nil {
		s.cfg.Engine = *body.Engine
	}
	if body.AnthropicKey != "" {
		s.cfg.AnthropicKey = body.AnthropicKey
	}
	if body.DeepSeekKey != "" {
		s.cfg.DeepSeekKey = body.DeepSeekKey
	}
	if body.ClearAnthropic {
		s.cfg.AnthropicKey = ""
	}
	if body.ClearDeepSeek {
		s.cfg.DeepSeekKey = ""
	}
	if body.Model != "" {
		s.cfg.Model = body.Model
		_ = os.Setenv("VERO_MODEL", body.Model) // 即时生效, 无需重启
	}
	if body.Temperature != nil {
		s.cfg.Temperature = *body.Temperature
	}
	if body.MaxBudget != nil {
		s.cfg.MaxBudget = *body.MaxBudget
	}
	if body.YOLO != nil {
		s.cfg.YOLO = *body.YOLO
	}
	if body.DeepThinking != nil {
		s.cfg.DeepThinking = *body.DeepThinking
	}
	// 提供商整组替换: 前端提交时已把空 key 置为原 key(后端也在下方兜底)。
	if body.Providers != nil {
		keep := map[string]string{}
		for _, old := range s.cfg.Providers {
			keep[old.ID] = old.APIKey
		}
		for _, p := range body.Providers {
			if p.APIKey == "" { // 空 key = 保留原值(前端不回显明文)
				p.APIKey = keep[p.ID]
			}
		}
		s.cfg.Providers = body.Providers
	}
	if body.ActiveProvider != "" {
		s.cfg.ActiveProvider = body.ActiveProvider
	}
	if err := s.cfg.Save(); err != nil {
		s.cfgMu.Unlock()
		http.Error(w, "保存失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pub := s.cfg.Public()
	s.cfgMu.Unlock()
	writeJSON(w, map[string]any{"ok": true, "config": pub})
}

// handleProviderTest —— POST /api/providers/test {base_url, api_key, id?}
// 测试连接并自动拉取模型列表(OpenAI 兼容 /models)。
func (s *Server) handleProviderTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string `json:"id"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.BaseURL == "" {
		http.Error(w, "base_url required", http.StatusBadRequest)
		return
	}
	p := &config.Provider{ID: body.ID, BaseURL: body.BaseURL, APIKey: body.APIKey}
	models, err := llm.ListModels(p)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "models": models})
}
