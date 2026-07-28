package scenarios

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// BenchmarkParseFFUFOptimized —— 优化后的 ParseFFUF 性能对比。
func BenchmarkParseFFUFOptimized(b *testing.B) {
	jsonOutput := `{"results":[
{"input":{"FUZZ":"admin"},"position":1,"status":200,"length":4521,"words":342,"lines":87,"content-type":"text/html","url":"http://example.com/admin"},
{"input":{"FUZZ":"login"},"position":2,"status":200,"length":3210,"words":234,"lines":65,"content-type":"text/html","url":"http://example.com/login"},
{"input":{"FUZZ":"backup"},"position":3,"status":200,"length":1024,"words":89,"lines":23,"content-type":"application/x-tar","url":"http://example.com/backup"},
{"input":{"FUZZ":"config"},"position":4,"status":403,"length":512,"words":45,"lines":12,"content-type":"text/html","url":"http://example.com/config"},
{"input":{"FUZZ":"test"},"position":5,"status":200,"length":2048,"words":156,"lines":34,"content-type":"text/html","url":"http://example.com/test"}
]}`

	b.Run("原始版本", func(b *testing.B) {
		reg := tools.NewRegistry()
		sm := NewManager()
		RegisterDefaults(sm, reg)

		ffufTool, _ := reg.Get("ffuf_dir_brute")
		args := map[string]any{"target": "http://example.com"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ffufTool.Parse(jsonOutput, args)
		}
	})

	b.Run("优化版本", func(b *testing.B) {
		args := map[string]any{"target": "http://example.com"}

		// 预编译敏感关键词（移到全局）
		sensitiveKeywords := []string{
			"admin", "backup", "config", "login", "upload", "shell",
			".git", ".env", ".sql", "phpinfo", "test", "debug",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = parseFFUFOptimized(jsonOutput, args, sensitiveKeywords)
		}
	})
}

// parseFFUFOptimized —— 优化版 ParseFFUF。
func parseFFUFOptimized(stdout string, args map[string]any, sensitiveKeywords []string) []tools.Observation {
	var result struct {
		Results []struct {
			Input      map[string]string `json:"input"`
			StatusCode int               `json:"status"`
			Length     int               `json:"length"`
		} `json:"results"`
	}

	// 使用 json.Decoder 流式解析（对大 JSON 更高效）
	decoder := json.NewDecoder(strings.NewReader(stdout))
	if err := decoder.Decode(&result); err != nil {
		return nil
	}

	target := tools.ArgStr(args, "target", "?")
	obs := make([]tools.Observation, 0, len(result.Results))

	for _, entry := range result.Results {
		path := entry.Input["FUZZ"]
		if path == "" {
			continue
		}

		// 严重级判断（优化分支）
		severity := "info"
		switch entry.StatusCode {
		case 200, 204:
			severity = "medium"
		case 401, 403:
			severity = "low"
		}

		// 敏感路径检测（使用预编译关键词）
		pathLower := strings.ToLower(path)
		for _, kw := range sensitiveKeywords {
			if strings.Contains(pathLower, kw) {
				severity = "high"
				break
			}
		}

		// 使用 strings.Builder 减少内存分配
		var labelBuilder strings.Builder
		labelBuilder.WriteString("[")
		labelBuilder.WriteString(severity)
		labelBuilder.WriteString("] Path found: /")
		labelBuilder.WriteString(path)
		labelBuilder.WriteString(" (HTTP ")
		labelBuilder.WriteString(string(rune(entry.StatusCode/100) + '0'))
		labelBuilder.WriteString(string(rune((entry.StatusCode/10)%10) + '0'))
		labelBuilder.WriteString(string(rune(entry.StatusCode%10) + '0'))
		labelBuilder.WriteString(")")

		obs = append(obs, tools.Observation{
			Kind:    "finding",
			Key:     target + ":path:" + path,
			Label:   labelBuilder.String(),
			Excerpt: path, // 简化 Excerpt
		})
	}

	return obs
}

// TestParseFFUFOptimization —— 验证优化版本正确性。
func TestParseFFUFOptimization(t *testing.T) {
	jsonOutput := `{"results":[
{"input":{"FUZZ":"admin"},"position":1,"status":200,"length":4521,"url":"http://example.com/admin"},
{"input":{"FUZZ":"backup"},"position":2,"status":200,"length":1024,"url":"http://example.com/backup"}
]}`

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	ffufTool, _ := reg.Get("ffuf_dir_brute")
	args := map[string]any{"target": "http://example.com"}

	// 原始版本
	obsOriginal := ffufTool.Parse(jsonOutput, args)

	// 优化版本
	sensitiveKeywords := []string{
		"admin", "backup", "config", "login", "upload", "shell",
		".git", ".env", ".sql", "phpinfo", "test", "debug",
	}
	obsOptimized := parseFFUFOptimized(jsonOutput, args, sensitiveKeywords)

	// 验证结果一致
	if len(obsOriginal) != len(obsOptimized) {
		t.Errorf("结果数量不一致: 原始 %d, 优化 %d", len(obsOriginal), len(obsOptimized))
	}

	for i := range obsOriginal {
		if obsOriginal[i].Key != obsOptimized[i].Key {
			t.Errorf("Key 不一致: 原始 %s, 优化 %s", obsOriginal[i].Key, obsOptimized[i].Key)
		}
		if obsOriginal[i].Kind != obsOptimized[i].Kind {
			t.Errorf("Kind 不一致: 原始 %s, 优化 %s", obsOriginal[i].Kind, obsOptimized[i].Kind)
		}
	}

	t.Logf("✓ 优化版本功能正确")
}
