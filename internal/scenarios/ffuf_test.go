package scenarios

import (
	"strings"
	"testing"
)

// TestParseFFUF —— 验证目录爆破结果解析。
func TestParseFFUF(t *testing.T) {
	// 真实 ffuf -of json 输出样本(简化)
	jsonOutput := `{
  "results": [
    {
      "input": {"FUZZ": "admin"},
      "position": 1,
      "status": 200,
      "length": 1234,
      "words": 50,
      "lines": 20,
      "url": "http://example.com/admin"
    },
    {
      "input": {"FUZZ": "backup.zip"},
      "position": 2,
      "status": 200,
      "length": 5678,
      "words": 100,
      "lines": 30,
      "url": "http://example.com/backup.zip"
    },
    {
      "input": {"FUZZ": "api"},
      "position": 3,
      "status": 401,
      "length": 234,
      "words": 10,
      "lines": 5,
      "url": "http://example.com/api"
    },
    {
      "input": {"FUZZ": "public"},
      "position": 4,
      "status": 301,
      "length": 0,
      "words": 0,
      "lines": 0,
      "url": "http://example.com/public"
    }
  ]
}`

	obs := ParseFFUF(jsonOutput, map[string]any{"target": "http://example.com"})

	// 应提取 4 个路径
	if len(obs) != 4 {
		t.Fatalf("应提取 4 个路径, 实际 %d", len(obs))
	}

	// 验证第一个路径(admin - 高危)
	if obs[0].Kind != "finding" {
		t.Errorf("应为 finding 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "admin") {
		t.Errorf("Label 应含路径名, 实际 %s", obs[0].Label)
	}
	if !strings.Contains(obs[0].Label, "[high]") {
		t.Errorf("admin 应标记为 high 严重级, 实际 %s", obs[0].Label)
	}
	if !strings.Contains(obs[0].Excerpt, `"status":200`) {
		t.Errorf("Excerpt 应含状态码, 实际 %s", obs[0].Excerpt)
	}

	// 验证第二个路径(backup.zip - 高危)
	hasBackup := false
	for _, o := range obs {
		if strings.Contains(o.Key, "backup.zip") {
			hasBackup = true
			if !strings.Contains(o.Label, "[high]") {
				t.Errorf("backup 应标记为 high, 实际 %s", o.Label)
			}
		}
	}
	if !hasBackup {
		t.Error("应提取 backup.zip 路径")
	}

	// 验证第三个路径(api - 401 需认证)
	hasAPI := false
	for _, o := range obs {
		if strings.Contains(o.Key, ":path:api") {
			hasAPI = true
			if !strings.Contains(o.Label, "401") {
				t.Errorf("应显示状态码 401, 实际 %s", o.Label)
			}
		}
	}
	if !hasAPI {
		t.Error("应提取 api 路径")
	}
}

// TestParseFFUFVhost —— 验证虚拟主机枚举解析。
func TestParseFFUFVhost(t *testing.T) {
	jsonOutput := `{
  "results": [
    {
      "input": {"FUZZ": "dev"},
      "position": 1,
      "status": 200,
      "length": 1234,
      "url": "http://192.168.1.100"
    },
    {
      "input": {"FUZZ": "staging"},
      "position": 2,
      "status": 302,
      "length": 567,
      "url": "http://192.168.1.100"
    }
  ]
}`

	obs := ParseFFUFVhost(jsonOutput, map[string]any{"domain": "example.com"})

	// 应提取 2 个虚拟主机
	if len(obs) != 2 {
		t.Fatalf("应提取 2 个虚拟主机, 实际 %d", len(obs))
	}

	// 验证第一个虚拟主机
	if obs[0].Kind != "finding" {
		t.Errorf("应为 finding 类型, 实际 %s", obs[0].Kind)
	}
	if !strings.Contains(obs[0].Label, "dev.example.com") {
		t.Errorf("Label 应含完整域名, 实际 %s", obs[0].Label)
	}
	if !strings.Contains(obs[0].Key, "dev.example.com") {
		t.Errorf("Key 应含 FQDN, 实际 %s", obs[0].Key)
	}
}

// TestParseFFUFEmpty —— 空输出不应 panic。
func TestParseFFUFEmpty(t *testing.T) {
	obs := ParseFFUF("", map[string]any{})
	if obs != nil && len(obs) != 0 {
		t.Errorf("空输入应返回 nil 或空数组, 实际 %d", len(obs))
	}
}

// TestParseFFUFInvalidJSON —— 非法 JSON 不应 panic。
func TestParseFFUFInvalidJSON(t *testing.T) {
	obs := ParseFFUF("not a json", map[string]any{})
	if obs != nil && len(obs) != 0 {
		t.Errorf("非法 JSON 应返回 nil, 实际 %d", len(obs))
	}
}

// TestFFUFSensitiveKeywords —— 验证敏感路径识别。
func TestFFUFSensitiveKeywords(t *testing.T) {
	testCases := []struct {
		path     string
		expected string
	}{
		{"admin", "high"},
		{"backup", "high"},
		{".env", "high"},
		{"config.php", "high"},
		{"upload", "high"},
		{"public", "medium"}, // 普通 200 响应
		{"about", "medium"},
	}

	for _, tc := range testCases {
		jsonOutput := `{"results":[{"input":{"FUZZ":"` + tc.path + `"},"status":200,"length":100,"url":"http://test.com/` + tc.path + `"}]}`
		obs := ParseFFUF(jsonOutput, map[string]any{"target": "http://test.com"})

		if len(obs) != 1 {
			t.Fatalf("[%s] 应提取 1 个路径, 实际 %d", tc.path, len(obs))
		}

		if !strings.Contains(obs[0].Label, "["+tc.expected+"]") {
			t.Errorf("[%s] 应为 [%s] 严重级, 实际 %s", tc.path, tc.expected, obs[0].Label)
		}
	}
}
