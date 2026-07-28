// CLI 界面优化：彩色输出、进度条、交互式菜单
package cli

import (
	"fmt"
	"strings"
)

// Color codes
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
)

// PrintBanner —— 打印 Vero 横幅
func PrintBanner() {
	banner := `
 ██╗   ██╗███████╗██████╗  ██████╗
 ██║   ██║██╔════╝██╔══██╗██╔═══██╗
 ██║   ██║█████╗  ██████╔╝██║   ██║
 ╚██╗ ██╔╝██╔══╝  ██╔══██╗██║   ██║
  ╚████╔╝ ███████╗██║  ██║╚██████╔╝
   ╚═══╝  ╚══════╝╚═╝  ╚═╝ ╚═════╝

  Evidence-Driven AI Red Team Agent
  https://github.com/Coff0xc/vero
`
	fmt.Println(ColorCyan + banner + ColorReset)
}

// PrintSuccess —— 成功消息
func PrintSuccess(msg string) {
	fmt.Printf("%s✓%s %s\n", ColorGreen, ColorReset, msg)
}

// PrintError —— 错误消息
func PrintError(msg string) {
	fmt.Printf("%s✗%s %s\n", ColorRed, ColorReset, msg)
}

// PrintWarning —— 警告消息
func PrintWarning(msg string) {
	fmt.Printf("%s⚠%s %s\n", ColorYellow, ColorReset, msg)
}

// PrintInfo —— 信息消息
func PrintInfo(msg string) {
	fmt.Printf("%sℹ%s %s\n", ColorBlue, ColorReset, msg)
}

// PrintSection —— 打印章节标题
func PrintSection(title string) {
	fmt.Printf("\n%s%s═══ %s ═══%s\n\n", ColorBold, ColorCyan, title, ColorReset)
}

// PrintProgress —— 打印进度条
func PrintProgress(current, total int, label string) {
	percent := float64(current) / float64(total) * 100
	width := 40
	filled := int(percent / 100 * float64(width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	fmt.Printf("\r%s[%s]%s %3.0f%% %s", ColorCyan, bar, ColorReset, percent, label)

	if current == total {
		fmt.Println()
	}
}

// PrintTable —— 打印表格
func PrintTable(headers []string, rows [][]string) {
	// 计算列宽
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// 打印表头
	fmt.Print(ColorBold)
	for i, h := range headers {
		fmt.Printf("%-*s  ", colWidths[i], h)
	}
	fmt.Println(ColorReset)

	// 打印分隔线
	for _, w := range colWidths {
		fmt.Print(strings.Repeat("─", w) + "  ")
	}
	fmt.Println()

	// 打印行
	for _, row := range rows {
		for i, cell := range row {
			// 根据内容着色
			color := ""
			if strings.Contains(cell, "✓") || strings.Contains(cell, "可用") {
				color = ColorGreen
			} else if strings.Contains(cell, "✗") || strings.Contains(cell, "不可用") {
				color = ColorRed
			} else if strings.Contains(cell, "critical") || strings.Contains(cell, "Critical") {
				color = ColorRed
			} else if strings.Contains(cell, "high") || strings.Contains(cell, "High") {
				color = ColorYellow
			}

			fmt.Printf("%s%-*s%s  ", color, colWidths[i], cell, ColorReset)
		}
		fmt.Println()
	}
}

// PrintMenu —— 打印交互式菜单
func PrintMenu(title string, options []string) {
	PrintSection(title)
	for i, opt := range options {
		fmt.Printf("%s%d.%s %s\n", ColorCyan, i+1, ColorReset, opt)
	}
	fmt.Print("\n选择选项: ")
}

// PrintToolStatus —— 打印工具状态
func PrintToolStatus(name string, available bool, level int, duration int64) {
	levelNames := []string{"L0-侦察", "L1-扫描", "L2-凭证", "L3-利用", "L4-破坏"}
	levelColors := []string{ColorBlue, ColorGreen, ColorYellow, ColorMagenta, ColorRed}

	status := ""
	statusColor := ""
	if available {
		status = "✓ 可用"
		statusColor = ColorGreen
	} else {
		status = "✗ 不可用"
		statusColor = ColorRed
	}

	levelLabel := levelNames[level]
	if level < len(levelColors) {
		levelLabel = levelColors[level] + levelLabel + ColorReset
	}

	fmt.Printf("  %-30s [%s] %s%s%s (%dms)\n",
		name,
		levelLabel,
		statusColor,
		status,
		ColorReset,
		duration)
}

// Spinner —— 简单旋转器
type Spinner struct {
	frames []string
	index  int
}

// NewSpinner —— 创建新的旋转器
func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		index:  0,
	}
}

// Next —— 显示下一帧
func (s *Spinner) Next(label string) {
	frame := s.frames[s.index]
	s.index = (s.index + 1) % len(s.frames)
	fmt.Printf("\r%s%s%s %s", ColorCyan, frame, ColorReset, label)
}

// Stop —— 停止旋转器
func (s *Spinner) Stop() {
	fmt.Print("\r")
}
