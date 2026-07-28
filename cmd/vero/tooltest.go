package main

import (
	"encoding/json"
	"fmt"

	"github.com/Coff0xc/vero/internal/scenarios"
	"github.com/Coff0xc/vero/internal/tooltest"
	"github.com/Coff0xc/vero/internal/tools"
)

func runToolTest() {
	fmt.Println("开始验证工具集成...")
	fmt.Println()

	// 注册所有场景包
	reg := tools.NewRegistry()
	mgr := scenarios.NewManager()
	scenarios.RegisterDefaults(mgr, reg)

	fmt.Printf("已注册 %d 个工具\n\n", len(reg.Names()))

	// 执行验证
	results := tooltest.VerifyAll(reg)

	// 打印摘要
	fmt.Println(tooltest.Summary(results))

	// 输出 JSON（供 Web 端使用）
	jsonData, _ := json.MarshalIndent(results, "", "  ")
	fmt.Printf("\nJSON 输出:\n%s\n", string(jsonData))
}
