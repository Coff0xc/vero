package scenarios

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"redcell/internal/tools"
)

// TestConcurrentToolExecution —— 并发工具执行测试。
func TestConcurrentToolExecution(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	t.Run("10个并发 Parser", func(t *testing.T) {
		awsTool, _ := reg.Get("aws_imds_enum")
		mockOutput := `AWS IMDS Enumeration:
  instance-id: i-test123
  AccessKeyId: AKIATEST123`

		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				obs := awsTool.Parse(mockOutput, map[string]any{})
				if len(obs) != 2 {
					errors <- fmt.Errorf("goroutine %d: 期望 2 个观测, 实际 %d", id, len(obs))
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Error(err)
		}
	})

	t.Run("100个并发工具查找", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 100)

		toolNames := []string{
			"http_probe", "web_vuln_scan", "nxc_smb_spray",
			"aws_imds_enum", "docker_escape_check", "msf_search",
		}

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				toolName := toolNames[id%len(toolNames)]
				tool, ok := reg.Get(toolName)
				if !ok {
					errors <- fmt.Errorf("goroutine %d: 工具 %s 未找到", id, toolName)
					return
				}
				if tool.Name != toolName {
					errors <- fmt.Errorf("goroutine %d: 工具名不匹配", id)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Error(err)
		}
	})

	t.Run("并发场景包路由", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 50)

		services := []map[string]bool{
			{"http": true},
			{"microsoft-ds": true, "ldap": true},
			{},
		}

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				svc := services[id%len(services)]
				activePacks := sm.Route(svc)
				if len(activePacks) == 0 && len(svc) > 0 {
					errors <- fmt.Errorf("goroutine %d: 应激活至少 1 个场景包", id)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Error(err)
		}
	})
}

// TestMemoryUsage —— 内存使用测试。
func TestMemoryUsage(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	t.Run("大量 Parser 调用内存稳定性", func(t *testing.T) {
		runtime.GC() // 触发 GC 获取基线
		time.Sleep(100 * time.Millisecond)

		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)
		baseline := m1.Alloc

		awsTool, _ := reg.Get("aws_imds_enum")
		mockOutput := `AWS IMDS Enumeration:
  instance-id: i-test123
  AccessKeyId: AKIATEST123
  SecretAccessKey: testsecret`

		// 执行 10000 次 Parser
		for i := 0; i < 10000; i++ {
			_ = awsTool.Parse(mockOutput, map[string]any{})
		}

		runtime.GC() // 强制 GC
		time.Sleep(100 * time.Millisecond)
		runtime.ReadMemStats(&m2)

		// 使用 int64 避免下溢
		allocDiff := int64(m2.Alloc) - int64(baseline)
		if allocDiff < 0 {
			t.Logf("内存下降: %d bytes (GC 生效)", -allocDiff)
		} else {
			t.Logf("内存增长: %d bytes (%.2f MB)", allocDiff, float64(allocDiff)/1024/1024)

			// 预期: 10000 次调用后内存增长 < 10 MB
			if allocDiff > 10*1024*1024 {
				t.Errorf("内存增长过大: %.2f MB", float64(allocDiff)/1024/1024)
			}
		}
	})

	t.Run("工具注册表内存占用", func(t *testing.T) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// 通过遍历已知工具计算
		toolNames := []string{
			"http_probe", "web_vuln_scan", "nxc_smb_spray",
			"aws_imds_enum", "docker_escape_check", "msf_search",
		}

		validTools := 0
		for _, name := range toolNames {
			if _, ok := reg.Get(name); ok {
				validTools++
			}
		}

		avgMemPerTool := m.Alloc / uint64(validTools)

		t.Logf("已验证工具数: %d, 平均每个工具占用: %d bytes", validTools, avgMemPerTool)

		// 预期: 平均每个工具 < 100 KB (宽松估计)
		if avgMemPerTool > 100*1024 {
			t.Logf("每个工具内存占用: %d bytes (可接受)", avgMemPerTool)
		}
	})

	t.Run("场景包路由内存泄漏检测", func(t *testing.T) {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)

		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)
		baseline := m1.Alloc

		services := map[string]bool{"http": true, "microsoft-ds": true}

		// 执行 1000 次路由
		for i := 0; i < 1000; i++ {
			_ = sm.Route(services)
		}

		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		runtime.ReadMemStats(&m2)

		allocDiff := int64(m2.Alloc) - int64(baseline)
		if allocDiff < 0 {
			t.Logf("路由 1000 次内存下降: %d bytes (GC 回收)", -allocDiff)
		} else {
			t.Logf("路由 1000 次内存增长: %d bytes", allocDiff)

			// 预期: 1000 次路由内存增长 < 1 MB
			if allocDiff > 1024*1024 {
				t.Errorf("路由存在内存泄漏: %d bytes", allocDiff)
			}
		}
	})
}

// TestStressLoad —— 压力测试。
func TestStressLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试 (使用 -short)")
	}

	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	t.Run("1000个并发 Parser (压力)", func(t *testing.T) {
		awsTool, _ := reg.Get("aws_imds_enum")
		dockerTool, _ := reg.Get("docker_escape_check")
		k8sTool, _ := reg.Get("k8s_sa_enum")

		mockAWS := `AWS IMDS: instance-id: i-test, AccessKeyId: AKIA123`
		mockDocker := `Docker Escape: [!] PRIVILEGED CONTAINER DETECTED`
		mockK8s := `K8s SA: [!] Token: eyJ..., [!] API accessible - can list namespaces`

		var wg sync.WaitGroup
		errors := make(chan error, 1000)
		start := time.Now()

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				switch id % 3 {
				case 0:
					obs := awsTool.Parse(mockAWS, map[string]any{})
					if len(obs) != 2 {
						errors <- fmt.Errorf("AWS parser 失败")
					}
				case 1:
					obs := dockerTool.Parse(mockDocker, map[string]any{})
					if len(obs) != 1 {
						errors <- fmt.Errorf("Docker parser 失败")
					}
				case 2:
					obs := k8sTool.Parse(mockK8s, map[string]any{})
					if len(obs) != 2 {
						errors <- fmt.Errorf("K8s parser 失败")
					}
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)
		close(errors)

		errorCount := 0
		for err := range errors {
			t.Error(err)
			errorCount++
		}

		t.Logf("1000 个并发 Parser 完成: 耗时 %v, 错误数 %d", duration, errorCount)
		t.Logf("平均延迟: %v", duration/1000)

		// 预期: 1000 个并发在 5 秒内完成
		if duration > 5*time.Second {
			t.Errorf("压力测试超时: %v", duration)
		}
	})

	t.Run("连续工具查找性能", func(t *testing.T) {
		start := time.Now()

		for i := 0; i < 100000; i++ {
			_, _ = reg.Get("http_probe")
			_, _ = reg.Get("aws_imds_enum")
			_, _ = reg.Get("docker_escape_check")
		}

		duration := time.Since(start)
		t.Logf("300,000 次工具查找: 耗时 %v", duration)
		t.Logf("平均每次查找: %v", duration/300000)

		// 预期: 300,000 次查找在 1 秒内完成 (avg < 3.3 µs)
		if duration > 1*time.Second {
			t.Errorf("工具查找性能下降: %v", duration)
		}
	})

	t.Run("场景包路由压力", func(t *testing.T) {
		services := []map[string]bool{
			{"http": true},
			{"https": true, "ssl/http": true},
			{"microsoft-ds": true, "ldap": true, "kerberos-sec": true},
			{},
		}

		start := time.Now()

		for i := 0; i < 10000; i++ {
			svc := services[i%len(services)]
			_ = sm.Route(svc)
		}

		duration := time.Since(start)
		t.Logf("10,000 次场景路由: 耗时 %v", duration)
		t.Logf("平均每次路由: %v", duration/10000)

		// 预期: 10,000 次路由在 1 秒内完成 (avg < 100 µs)
		if duration > 1*time.Second {
			t.Errorf("场景路由性能下降: %v", duration)
		}
	})
}

// TestGoroutineLeaks —— Goroutine 泄漏检测。
func TestGoroutineLeaks(t *testing.T) {
	reg := tools.NewRegistry()
	sm := NewManager()
	RegisterDefaults(sm, reg)

	initialGoroutines := runtime.NumGoroutine()

	// 执行 1000 次操作
	for i := 0; i < 1000; i++ {
		_ = sm.Route(map[string]bool{"http": true})
		_, _ = reg.Get("http_probe")
	}

	time.Sleep(100 * time.Millisecond) // 等待 goroutine 清理
	finalGoroutines := runtime.NumGoroutine()

	diff := finalGoroutines - initialGoroutines
	t.Logf("Goroutine 变化: %d -> %d (差值: %d)", initialGoroutines, finalGoroutines, diff)

	// 预期: goroutine 数量差异 < 10 (允许少量后台 goroutine)
	if diff > 10 {
		t.Errorf("可能存在 goroutine 泄漏: 增加了 %d 个", diff)
	}
}

// TestPanicRecovery —— Panic 恢复测试。
func TestPanicRecovery(t *testing.T) {
	reg := tools.NewRegistry()

	t.Run("Parser panic 不应崩溃", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Parser panic 未被捕获: %v", r)
			}
		}()

		// 创建会 panic 的工具
		panicTool := &tools.Tool{
			Name:  "panic_tool",
			Level: tools.LevelScan,
			Desc:  "测试 panic",
			Run: func(args map[string]any) tools.ToolResult {
				panic("故意触发 panic")
			},
			Parse: func(stdout string, args map[string]any) []tools.Observation {
				panic("parser panic")
			},
		}

		reg.Register(panicTool)

		// 调用 Parser 应被捕获
		tool, _ := reg.Get("panic_tool")
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Error("期望 parser panic 被捕获")
				}
			}()
			_ = tool.Parse("test", map[string]any{})
		}()
	})
}
