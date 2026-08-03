// Package config —— 智能体工作台运行时配置(引擎/API key/模型/思考强度/预算)。
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
	EngineAuto   Engine = "auto"   // 有 key 用真实模型, 否则脚本
	EngineScript Engine = "script" // 固定脚本(无 LLM)
	EngineClaude Engine = "claude"
	EngineDeepSeek Engine = "deepseek"
)

// Config —— 运行时配置。
type Config struct {
	Engine      Engine  `json:"engine"`
	AnthropicKey string `json:"anthropic_key,omitempty"`
	DeepSeekKey string  `json:"deepseek_key,omitempty"`
	Model       string  `json:"model"`        // 空 = 各自引擎默认
	Temperature float64 `json:"temperature"`  // 思考强度: 低=稳健, 高=发散
	MaxBudget   int     `json:"max_budget"`   // 战役决策轮数上限
}

const path = "vero.config.json"

// Default —— 出厂默认。
func Default() *Config {
	return &Config{
		Engine:      EngineAuto,
		Temperature: 0.2,
		MaxBudget:   10,
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
	return c
}

// Save —— 持久化(0600)。key 为空则保留原值(不清空), 避免表单缺字段误删密钥。
func (c *Config) Save() error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Public —— 回显给前端的视图: 密钥只给"是否已配置", 绝不回明文。
type Public struct {
	Engine       Engine  `json:"engine"`
	Model        string  `json:"model"`
	Temperature  float64 `json:"temperature"`
	MaxBudget    int     `json:"max_budget"`
	HasAnthropic bool    `json:"has_anthropic"`
	HasDeepSeek  bool    `json:"has_deepseek"`
}

func (c *Config) Public() Public {
	return Public{
		Engine:       c.Engine,
		Model:        c.Model,
		Temperature:  c.Temperature,
		MaxBudget:    c.MaxBudget,
		HasAnthropic: c.AnthropicKey != "",
		HasDeepSeek:  c.DeepSeekKey != "",
	}
}
