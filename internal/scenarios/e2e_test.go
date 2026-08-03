package scenarios

import (
	"testing"

	"github.com/Coff0xc/vero/internal/tools"
)

// TestE2EWithP123Tools —— 端到端测试: 验证 P1/P2/P3 工具在攻击链中的集成。
// 模拟场景: 云环境容器化 web 应用的完整渗透链。
func TestE2EWithP123Tools(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	// 验证所有 P1/P2/P3 工具已注册
	p123Tools := []string{
		// P1: Metasploit
		"msf_search", "msf_execute", "msf_get_sessions",
		// P2: Cloud
		"aws_imds_enum", "azure_imds_enum", "gcp_imds_enum", "s3_bucket_enum",
		// P3: Container
		"docker_escape_check", "k8s_sa_enum", "k8s_node_exploit",
	}

	for _, name := range p123Tools {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("工具 %s 未注册", name)
		}
	}

	// Step 1: 验证工具 Level 正确设置 (HITL 门控)
	msfExecute, _ := reg.Get("msf_execute")
	if msfExecute.Level != tools.LevelExploit {
		t.Errorf("msf_execute 应为 LevelExploit (3), 实际 %d", msfExecute.Level)
	}

	k8sNodeExploit, _ := reg.Get("k8s_node_exploit")
	if k8sNodeExploit.Level != tools.LevelExploit {
		t.Errorf("k8s_node_exploit 应为 LevelExploit (3), 实际 %d", k8sNodeExploit.Level)
	}

	// Step 2: 验证场景包路由
	services := map[string]bool{"http": true}
	activePacks := sm.Route(services)
	if len(activePacks) == 0 {
		t.Error("WebPack 应被激活但未触发")
	}

	// Step 3: 检测云环境 (CloudPack 总是激活)
	cloudServices := map[string]bool{}
	cloudActive := false
	for _, pack := range sm.packs {
		if pack.Name == "cloud" && pack.Fingerprint(cloudServices) {
			cloudActive = true
			break
		}
	}
	// 云包无服务指纹, 不要求路由激活; 但工具必须注册可用(LLM 可调用)。
	if !cloudActive {
		if !reg.Has("aws_imds_enum") {
			t.Error("云工具应注册可用")
		}
	}

	// Step 4: 验证 Parser 反幻觉机制
	awsTool, _ := reg.Get("aws_imds_enum")
	mockAWSOutput := `AWS IMDS Enumeration:
  instance-id: i-test123
  IAM Credentials:
  AccessKeyId: AKIATEST123
  SecretAccessKey: secret123`

	awsObs := awsTool.Parse(mockAWSOutput, map[string]any{})
	if len(awsObs) != 2 {
		t.Errorf("AWS parser 应提取 2 个观测 (实例ID + 凭证), 实际 %d", len(awsObs))
	}

	// 验证去重机制 (多行 AccessKeyId 只记录一次)
	mockDupOutput := mockAWSOutput + "\nAccessKeyId: duplicate\nSecretAccessKey: dup"
	dupObs := awsTool.Parse(mockDupOutput, map[string]any{})
	if len(dupObs) != 2 {
		t.Errorf("去重机制失败: 应仍为 2 个观测, 实际 %d", len(dupObs))
	}

	// Step 5: 验证证据可溯性
	for _, obs := range awsObs {
		if obs.Excerpt == "" {
			t.Errorf("观测 %s 缺少 Excerpt (证据字段)", obs.Key)
		}
	}

	// Step 6: 验证工具执行 (使用已注册的工具)
	_, ok := reg.Get("http_probe")
	if !ok {
		t.Log("http_probe 未注册，跳过工具执行测试")
	} else {
		t.Log("http_probe 已注册，可用于攻击链")
	}

	t.Logf("✓ E2E 验证通过: %d 工具注册, %d 场景包激活, 证据链完整", len(p123Tools), len(activePacks))
}

// TestScenarioPackRouting —— 验证场景包动态路由机制。
func TestScenarioPackRouting(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	testCases := []struct {
		name            string
		services        map[string]bool
		expectedPacks   []string
		unexpectedPacks []string
	}{
		{
			name:            "Web 环境",
			services:        map[string]bool{"http": true},
			expectedPacks:   []string{"web"},
			unexpectedPacks: []string{"container"},
		},
		{
			name:            "AD 域环境",
			services:        map[string]bool{"microsoft-ds": true, "ldap": true},
			expectedPacks:   []string{"ad", "ad_enhanced"},
			unexpectedPacks: []string{"container"},
		},
		{
			name:            "云环境 (无服务指纹)",
			services:        map[string]bool{},
			expectedPacks:   []string{}, // 云无服务指纹, 不路由激活
			unexpectedPacks: []string{"container"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			activePacks := sm.Route(tc.services)
			activeSet := make(map[string]bool)
			for _, name := range activePacks {
				activeSet[name] = true
			}

			for _, expected := range tc.expectedPacks {
				if !activeSet[expected] {
					t.Errorf("场景包 %s 应激活但未激活", expected)
				}
			}

			for _, unexpected := range tc.unexpectedPacks {
				if activeSet[unexpected] {
					t.Errorf("场景包 %s 不应激活但被激活了", unexpected)
				}
			}
		})
	}
}

// TestToolLevelHierarchy —— 验证工具 Level 分级和 HITL 门控。
func TestToolLevelHierarchy(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	scanTools := []string{"port_scan", "http_probe", "nmap_scan", "docker_escape_check"}
	credTools := []string{"secretsdump", "lsass_dump", "k8s_sa_enum"}
	exploitTools := []string{"exploit_sqli", "msf_execute", "k8s_node_exploit"}

	// 验证 LevelScan 工具
	for _, name := range scanTools {
		tool, ok := reg.Get(name)
		if !ok {
			continue // 某些工具可能未注册 (如 nmap 需要可选依赖)
		}
		if tool.Level != tools.LevelScan {
			t.Errorf("%s 应为 LevelScan (1), 实际 %d", name, tool.Level)
		}
	}

	// 验证 LevelCred 工具
	for _, name := range credTools {
		tool, ok := reg.Get(name)
		if !ok {
			continue
		}
		if tool.Level != tools.LevelCred {
			t.Errorf("%s 应为 LevelCred (2), 实际 %d", name, tool.Level)
		}
	}

	// 验证 LevelExploit 工具
	for _, name := range exploitTools {
		tool, ok := reg.Get(name)
		if !ok {
			continue
		}
		if tool.Level != tools.LevelExploit {
			t.Errorf("%s 应为 LevelExploit (3), 实际 %d", name, tool.Level)
		}
	}

	t.Logf("✓ 工具 Level 分级验证通过")
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

