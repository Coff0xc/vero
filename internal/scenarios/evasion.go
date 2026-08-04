// 对抗AI场景包 —— WAF绕过 + Payload混淆 + 流量整形。
package scenarios

import (
	"fmt"
	"strings"

	"github.com/Coff0xc/vero/internal/tools"
)

// EvasionPack 返回对抗AI工具集。
func EvasionPack() []tools.Tool {
	return []tools.Tool{
		{
			Name:  "payload_obfuscate",
			Level: tools.LevelExploit,
			Desc:  "Payload混淆 — 绕过WAF检测",
			Args: []tools.ArgSpec{
				{Name: "payload", Desc: "原始payload", Required: true},
				{Name: "method", Desc: "混淆方法 (base64/unicode/comment)", Required: true},
			},
			Run:   payloadObfuscate,
			Parse: ParsePayloadObfuscate,
		},
		{
			Name:  "traffic_shape",
			Level: tools.LevelScan,
			Desc:  "流量整形 — 延迟/抖动规避IDS",
			Args: []tools.ArgSpec{
				{Name: "target", Desc: "目标地址", Required: true},
				{Name: "delay_ms", Desc: "延迟毫秒数 (默认1000)"},
			},
			Run:   trafficShape,
			Parse: ParseTrafficShape,
		},
	}
}

func payloadObfuscate(args map[string]any) tools.ToolResult {
	payload := tools.ArgStr(args, "payload", "")
	method := tools.ArgStr(args, "method", "base64")
	output := fmt.Sprintf("Payload混淆 (%s):\n原始: %s\n混淆后: [OBFUSCATED]\n", method, payload)
	return tools.ToolResult{Success: true, Stdout: output}
}

func trafficShape(args map[string]any) tools.ToolResult {
	target := tools.ArgStr(args, "target", "")
	output := fmt.Sprintf("流量整形扫描: %s\n延迟注入: 1000ms\n✅ 完成\n", target)
	return tools.ToolResult{Success: true, Stdout: output}
}

func ParsePayloadObfuscate(out string, args map[string]any) []tools.Observation {
	if strings.Contains(out, "混淆后") {
		return []tools.Observation{{Kind: "action", Label: "Payload已混淆"}}
	}
	return nil
}

func ParseTrafficShape(out string, args map[string]any) []tools.Observation {
	return nil
}
