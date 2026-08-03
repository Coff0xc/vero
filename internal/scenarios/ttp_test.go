package scenarios

import (
	"testing"
)

// TTP/severity 结构化填充: parser 直接填 Observation.Severity/Technique/Tactic(不从 label 解析)。
func TestTTPSeverityStructured(t *testing.T) {
	// ParseNuclei: info.severity 入 Severity; tech 标签模板 -> T1190/initial-access。
	obs := ParseNuclei(`{"template-id":"tech-detect","matched-at":"http://x/","info":{"name":"Tech Detect","severity":"info","tags":["tech","detect"]}}`, map[string]any{})
	if len(obs) != 1 {
		t.Fatalf("ParseNuclei 应产出 1 条, got %d", len(obs))
	}
	if obs[0].Severity != "info" || obs[0].Technique != "T1190" || obs[0].Tactic != "initial-access" {
		t.Fatalf("ParseNuclei TTP 不对: %+v", obs[0])
	}
	// 非 tech 标签: severity 仍填, technique 留空(未映射留空)。
	obs = ParseNuclei(`{"template-id":"cve-x","matched-at":"http://x/","info":{"name":"CVE","severity":"high","tags":["cve"]}}`, map[string]any{})
	if obs[0].Severity != "high" || obs[0].Technique != "" || obs[0].Tactic != "" {
		t.Fatalf("非 tech finding 应留空 technique/tactic, got %+v", obs[0])
	}

	// ParseSQLi: critical / T1190 / initial-access。
	obs = ParseSQLi(`{"authentication":true,"token":"abc123"}`, map[string]any{"target": "http://x"})
	if len(obs) != 1 || obs[0].Severity != "critical" || obs[0].Technique != "T1190" || obs[0].Tactic != "initial-access" {
		t.Fatalf("ParseSQLi TTP 不对: %+v", obs)
	}

	// 凭证转储: T1003.001 / credential-access。
	obs = ParseSecretsdump("DOMAIN\\admin:500:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c1:::", map[string]any{"target": "x"})
	if len(obs) == 0 {
		t.Fatal("ParseSecretsdump 应产出凭证")
	}
	for _, o := range obs {
		if o.Technique != "T1003.001" || o.Tactic != "credential-access" {
			t.Fatalf("ParseSecretsdump TTP 不对: %+v", o)
		}
	}

	// foothold/shell: T1021.002 / lateral-movement。
	obs = ParseMSFSessions("[1] shell @ 192.168.1.100:4444", map[string]any{})
	if len(obs) != 1 || obs[0].Technique != "T1021.002" || obs[0].Tactic != "lateral-movement" {
		t.Fatalf("ParseMSFSessions TTP 不对: %+v", obs)
	}
}
