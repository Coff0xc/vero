// 工作台配置 API —— 引擎/API key/模型/思考强度/预算。
// 密钥只写盘(0600)不回显: GET 只返回"是否已配置"。
package server

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/Coff0xc/vero/internal/config"
)

// handleConfigGet —— GET /api/config
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.cfg.Public())
}

// handleConfigSet —— POST /api/config
// 请求体字段可部分提交; 空 key 字段表示"不改"(不清空), 显式清空用 "clear":true。
func (s *Server) handleConfigSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Engine      *config.Engine `json:"engine"`
		AnthropicKey string        `json:"anthropic_key"`
		DeepSeekKey string         `json:"deepseek_key"`
		ClearAnthropic bool        `json:"clear_anthropic"`
		ClearDeepSeek bool         `json:"clear_deepseek"`
		Model       string         `json:"model"`
		Temperature *float64       `json:"temperature"`
		MaxBudget   *int           `json:"max_budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
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
	if err := s.cfg.Save(); err != nil {
		http.Error(w, "保存失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "config": s.cfg.Public()})
}
