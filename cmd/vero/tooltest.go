package main

import (
	"fmt"
	"time"

	"github.com/Coff0xc/vero/internal/cli"
	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tooltest"
	"github.com/Coff0xc/vero/internal/tools"
)

func runToolTest() {
	cli.PrintBanner()
	cli.PrintSection("工具集成验证")

	// 注册所有场景包
	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)

	total := len(reg.Names())
	cli.PrintInfo(fmt.Sprintf("已注册 %d 个工具", total))
	fmt.Println()

	// 执行验证（带进度条）
	results := []tooltest.ToolStatus{}
	for i, name := range reg.Names() {
		cli.PrintProgress(i, total, fmt.Sprintf("验证 %s...", name))
		tool, _ := reg.Get(name)

		start := time.Now()
		status := tooltest.ToolStatus{
			Name:   name,
			Level:  tool.Level,
			Tested: true,
		}

		// 简化验证（使用安全参数）
		args := tooltest.GetSafeTestArgs(name)
		result := tool.Run(args)
		status.Duration = time.Since(start)
		status.Available = result.Success
		if !result.Success {
			status.Error = result.Stderr
			if status.Error == "" {
				status.Error = result.Stdout
			}
		}

		results = append(results, status)
	}
	cli.PrintProgress(total, total, "完成")
	fmt.Println()

	// 打印摘要
	cli.PrintSection("验证结果")

	available := 0
	for _, r := range results {
		if r.Available {
			available++
		}
	}

	cli.PrintInfo(fmt.Sprintf("总工具数: %d", total))
	cli.PrintSuccess(fmt.Sprintf("可用: %d (%.1f%%)", available, float64(available)/float64(total)*100))
	cli.PrintError(fmt.Sprintf("不可用: %d (%.1f%%)", total-available, float64(total-available)/float64(total)*100))
	fmt.Println()

	// 按级别统计
	cli.PrintSection("按级别统计")
	byLevel := map[int]int{0: 0, 1: 0, 2: 0, 3: 0, 4: 0}
	availByLevel := map[int]int{0: 0, 1: 0, 2: 0, 3: 0, 4: 0}

	for _, r := range results {
		byLevel[r.Level]++
		if r.Available {
			availByLevel[r.Level]++
		}
	}

	levelNames := []string{"L0-侦察", "L1-扫描", "L2-凭证", "L3-利用", "L4-破坏"}
	for i := 0; i <= 4; i++ {
		if byLevel[i] > 0 {
			fmt.Printf("  %s: %d/%d 可用\n", levelNames[i], availByLevel[i], byLevel[i])
		}
	}
	fmt.Println()

	// 详细状态
	cli.PrintSection("详细状态")
	for _, r := range results {
		cli.PrintToolStatus(r.Name, r.Available, r.Level, r.Duration.Milliseconds())
	}
}
