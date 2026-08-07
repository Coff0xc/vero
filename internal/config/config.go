// Package config —— 智能体工作台运行时配置(多提供商/引擎/key/模型/思考强度/预算)。
//
// 存储: <工作目录>/vero.config.json(0600, 密钥只写盘不回显)。
// 读取优先级: 配置文件 > 环境变量 > 默认值。
package config

import (
	"encoding/json"
	"os"
)

// Engine —— 决策引擎。
type Engine string

const (
	EngineAuto    Engine = "auto"   // 有 key 用真实模型, 否则脚本
	EngineScript  Engine = "script" // 固定脚本(无 LLM)
	EngineClaude  Engine = "claude"
	EngineDeepSeek Engine = "deepseek"
)

// ProviderPreset —— 预置的 OpenAI 兼容提供商(设置面板一键填入)。
type ProviderPreset struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

// Presets —— 常用 OpenAI 兼容提供商预置。
var Presets = []ProviderPreset{
	{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com"},
	{ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1"},
	{ID: "kimi", Name: "Kimi (Moonshot)", BaseURL: "https://api.moonshot.cn/v1"},
	{ID: "qwen", Name: "通义千问 (DashScope)", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{ID: "openrouter", Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1"},
	{ID: "ollama", Name: "Ollama (本地)", BaseURL: "http://localhost:11434/v1"},
	{ID: "zhipu", Name: "智谱 GLM", BaseURL: "https://open.bigmodel.cn/api/paas/v4"},
}

// Provider —— 一个 OpenAI 兼容的模型提供商。
type Provider struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	Model   string `json:"model"`             // 当前选用的模型
	Reasoning bool `json:"supports_reasoning"` // 深度思考(返回 reasoning_content)
	WebSearch bool `json:"web_search"`         // 联网搜索(提供商支持时)
}

// HasKey —— 是否已配置密钥(回显用)。
func (p *Provider) HasKey() bool { return p.APIKey != "" }

// Config —— 运行时配置。
type Config struct {
	Engine       Engine     `json:"engine"`
	AnthropicKey string     `json:"anthropic_key,omitempty"`
	DeepSeekKey  string     `json:"deepseek_key,omitempty"`
	Model        string     `json:"model"`      // 空 = 各自引擎默认(兼容旧字段)
	Temperature  float64    `json:"temperature"`
	MaxBudget    int        `json:"max_budget"`

	// 多提供商(对话/战役决策): active_provider 为当前启用的 provider id。
	Providers      []*Provider `json:"providers"`
	ActiveProvider string      `json:"active_provider"`

	// YOLO 模式: 跳过全部 HITL 审批(授权靶场/自动化用, 谨慎)。
	YOLO bool `json:"yolo"`
	// 深度思考: 决策时用 reasoning 模型并透传思维链(前端展示思考过程)。
	DeepThinking bool `json:"deep_thinking"`
}

const path = "vero.config.json"

// Default —— 出厂默认(含预置提供商骨架)。
func Default() *Config {
	provs := make([]*Provider, 0, len(Presets))
	for _, pr := range Presets {
		provs = append(provs, &Provider{ID: pr.ID, Name: pr.Name, BaseURL: pr.BaseURL})
	}
	return &Config{
		Engine:        EngineAuto,
		Temperature:   0.2,
		MaxBudget:     20, // 连续引擎默认 20 步(抄 Dark-Moon 连续推进: 8~10 步只够侦察+一次利用)
		Providers:     provs,
		ActiveProvider: "deepseek",
	}
}

// Load —— 读配置: 文件优先, 环境变量兜底(key)。
func Load() *Config {
	c := Default()
	raw, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(raw, c)
	}
	// key 兜底: 文件没配但环境变量有 -> 用环境变量(不改盘, 环境变量本就优先语义)
	if c.AnthropicKey == "" {
		c.AnthropicKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if c.DeepSeekKey == "" {
		c.DeepSeekKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	if c.Model == "" {
		if m := os.Getenv("VERO_MODEL"); m != "" {
			c.Model = m
		}
	}
	// 旧配置(无 providers 字段) -> 迁移出预置骨架。
	if len(c.Providers) == 0 {
		c.Providers = Default().Providers
	}

	// ── 自动迁移旧 key 到 providers 系统 ──
	// 旧 DeepSeekKey → providers[deepseek].api_key
	if c.DeepSeekKey != "" {
		for _, p := range c.Providers {
			if p.ID == "deepseek" && p.APIKey == "" {
				p.APIKey = c.DeepSeekKey
				if p.Model == "" {
					p.Model = "deepseek-chat"
				}
			}
		}
		// 确保 active_provider 指向有 key 的 provider
		if c.ActiveProvider == "" || !c.hasKeyProvider() {
			c.ActiveProvider = "deepseek"
		}
	}
	// 旧 AnthropicKey → providers 中对应 ID (如果存在)
	if c.AnthropicKey != "" {
		for _, p := range c.Providers {
			if (p.ID == "anthropic" || p.ID == "claude") && p.APIKey == "" {
				p.APIKey = c.AnthropicKey
			}
		}
	}

	// 如果 active_provider 没有 key，但有其他 provider 有 key，自动切换
	if !c.hasKeyProvider() {
		if keyed := c.firstKeyProvider(); keyed != nil {
			// 确保激活了有 key 的 provider
			for _, p := range c.Providers {
				if p == keyed {
					c.ActiveProvider = p.ID
					break
				}
			}
		}
	}

	return c
}

// hasKeyProvider —— 当前 active_provider 是否有 key。
func (c *Config) hasKeyProvider() bool {
	p := c.Active()
	return p != nil && p.APIKey != ""
}

// firstKeyProvider —— 找到第一个有 key 的 provider。
func (c *Config) firstKeyProvider() *Provider {
	for _, p := range c.Providers {
		if p.APIKey != "" {
			return p
		}
	}
	return nil
}

// Active —— 当前启用的提供商(未指定/不存在时回退 deepseek 或首个)。
func (c *Config) Active() *Provider {
	for _, p := range c.Providers {
		if p.ID == c.ActiveProvider {
			return p
		}
	}
	if len(c.Providers) > 0 {
		return c.Providers[0]
	}
	return nil
}

// Save —— 持久化(0600)。key 为空则保留原值(不清空), 避免表单缺字段误删密钥。
func (c *Config) Save() error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// PublicProvider —— 回显给前端的提供商视图(密钥只给"是否已配置")。
type PublicProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	HasKey    bool   `json:"has_key"`
	Reasoning bool   `json:"supports_reasoning"`
	WebSearch bool   `json:"web_search"`
}

// Public —— 回显给前端的视图: 密钥只给"是否已配置", 绝不回明文。
type Public struct {
	Engine         Engine            `json:"engine"`
	Model          string            `json:"model"`
	Temperature    float64           `json:"temperature"`
	MaxBudget      int               `json:"max_budget"`
	HasAnthropic   bool              `json:"has_anthropic"`
	HasDeepSeek    bool              `json:"has_deepseek"`
	YOLO           bool              `json:"yolo"`
	DeepThinking   bool              `json:"deep_thinking"`
	Providers      []PublicProvider  `json:"providers"`
	ActiveProvider string            `json:"active_provider"`
	Presets        []ProviderPreset  `json:"presets"`
}

func (c *Config) Public() Public {
	pp := make([]PublicProvider, 0, len(c.Providers))
	for _, p := range c.Providers {
		pp = append(pp, PublicProvider{
			ID:        p.ID,
			Name:      p.Name,
			BaseURL:   p.BaseURL,
			Model:     p.Model,
			HasKey:    p.HasKey(),
			Reasoning: p.Reasoning,
			WebSearch: p.WebSearch,
		})
	}
	return Public{
		Engine:         c.Engine,
		Model:          c.Model,
		Temperature:    c.Temperature,
		MaxBudget:      c.MaxBudget,
		HasAnthropic:   c.AnthropicKey != "",
		HasDeepSeek:    c.DeepSeekKey != "",
		YOLO:           c.YOLO,
		DeepThinking:   c.DeepThinking,
		Providers:      pp,
		ActiveProvider: c.ActiveProvider,
		Presets:        Presets,
	}
}
