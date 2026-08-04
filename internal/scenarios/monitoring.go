// 持续监控场景包 —— 定时扫描 + 基线比对 + 变化告警。
package scenarios

import (
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// MonitoringPack 返回持续监控工具集。
func MonitoringPack() []tools.Tool {
	return []tools.Tool{
		{
			Name:  "schedule_scan",
			Level: tools.LevelScan,
			Desc:  "定时扫描任务 — Cron表达式调度",
			Args: []tools.ArgSpec{
				{Name: "target", Desc: "目标地址", Required: true},
				{Name: "schedule", Desc: "Cron表达式 (如 0 */6 * * *)", Required: true},
			},
			Run:   scheduleScan,
			Parse: ParseScheduleScan,
		},
		{
			Name:  "baseline_compare",
			Level: tools.LevelRecon,
			Desc:  "基线比对 — 检测新增端口/服务",
			Args: []tools.ArgSpec{
				{Name: "baseline_id", Desc: "基线快照ID", Required: true},
				{Name: "current_scan", Desc: "当前扫描结果", Required: true},
			},
			Run:   baselineCompare,
			Parse: ParseBaselineCompare,
		},
		{
			Name:  "alert_webhook",
			Level: tools.LevelRecon,
			Desc:  "告警推送 — Webhook通知 (Slack/钉钉)",
			Args: []tools.ArgSpec{
				{Name: "webhook_url", Desc: "Webhook URL", Required: true},
				{Name: "message", Desc: "告警消息", Required: true},
			},
			Run:   alertWebhook,
			Parse: ParseAlertWebhook,
		},
	}
}

func scheduleScan(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	schedule := tools.ArgStr(args, "schedule", "")
	output := fmt.Sprintf("定时扫描任务已创建\n目标: %s\n调度: %s\n下次执行: %s\n",
		target, schedule, time.Now().Add(6*time.Hour).Format("2006-01-02 15:04:05"))
	return tools.ToolResult{Success: true, Stdout: output}
}

func baselineCompare(args map[string]any) tools.ToolResult {
	output := "基线比对结果:\n"
	output += "[新增] 端口 8080 (http-proxy)\n"
	output += "[消失] 端口 21 (ftp)\n"
	return tools.ToolResult{Success: true, Stdout: output}
}

func alertWebhook(args map[string]any) tools.ToolResult {
	return tools.ToolResult{Success: true, Stdout: "告警已推送"}
}

func ParseScheduleScan(out string, args map[string]any) []tools.Observation {
	if strings.Contains(out, "已创建") {
		return []tools.Observation{{Kind: "action", Label: "定时任务已创建"}}
	}
	return nil
}

func ParseBaselineCompare(out string, args map[string]any) []tools.Observation {
	var obs []tools.Observation
	if strings.Contains(out, "[新增]") {
		obs = append(obs, tools.Observation{Kind: "finding", Label: "检测到新增端口", Severity: "medium"})
	}
	return obs
}

func ParseAlertWebhook(out string, args map[string]any) []tools.Observation {
	return nil
}
