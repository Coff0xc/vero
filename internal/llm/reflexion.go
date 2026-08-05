package llm

// Package reflexion —— 反射学习增强 (抄 Xalgorix Reflexion):
// 失败案例自动总结 + 策略库持久化 + 智能 retry 参数调整。
//
// 设计要点:
//   - 失败模式分类: 网络超时 / 权限不足 / 工具不存在 / 参数错误 / 误报。
//   - 策略库持久化: SQLite lessons 表存储工具级教训，跨战役复用。
//   - 自动 retry: 检测可恢复失败（超时/暂时性错误），自动调整参数重试。
//   - Few-shot 注入: 动态从 lessons 表提取相关失败案例，注入 prompt。

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Coff0xc/vero/internal/core"
)

// FailureMode —— 失败模式分类（用于智能 retry 决策）。
type FailureMode int

const (
	FailureUnknown      FailureMode = iota // 未知失败
	FailureNetwork                          // 网络超时/连接失败
	FailurePermission                       // 权限不足 (403/401/HITL 拒绝)
	FailureToolMissing                      // 工具不存在/未安装
	FailureArgument                         // 参数错误/校验失败
	FailureFalsePositive                    // 误报/工具输出异常
	FailureTargetDown                       // 目标不可达/服务宕机
)

// LessonRecord —— 持久化的教训记录（存入 SQLite）。
type LessonRecord struct {
	ID        int64     `json:"id"`
	Tool      string    `json:"tool"`       // 工具名
	Args      string    `json:"args"`       // 参数 JSON
	Reason    string    `json:"reason"`     // 失败原因
	Mode      string    `json:"mode"`       // 失败模式 (network/permission/...)
	Solution  string    `json:"solution"`   // 解决方案 (调整参数/换工具/...)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReflexionEnhanced —— 增强版反射学习器（持久化 + 分类 + retry）。
type ReflexionEnhanced struct {
	db *sql.DB
}

// NewReflexionEnhanced —— 创建增强反射学习器。
func NewReflexionEnhanced(db *sql.DB) (*ReflexionEnhanced, error) {
	r := &ReflexionEnhanced{db: db}

	// 初始化 lessons 表
	if err := r.initDB(); err != nil {
		return nil, fmt.Errorf("初始化 lessons 表失败: %w", err)
	}

	return r, nil
}

// initDB —— 创建 lessons 表（如果不存在）。
func (r *ReflexionEnhanced) initDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS lessons (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tool TEXT NOT NULL,
		args TEXT NOT NULL,
		reason TEXT NOT NULL,
		mode TEXT NOT NULL,
		solution TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_lessons_tool ON lessons(tool);
	CREATE INDEX IF NOT EXISTS idx_lessons_mode ON lessons(mode);
	`
	_, err := r.db.Exec(schema)
	return err
}

// ClassifyFailure —— 失败模式分类（根据失败原因自动识别）。
func ClassifyFailure(reason string) FailureMode {
	reason = strings.ToLower(reason)

	// 静默失败: 命令执行但无任何输出(如 curl -sI 连接被拒时 stdout/stderr 都为空)。
	// 无输出通常意味着连接层失败, 按目标不可达处理, 允许延迟重试(U3)。
	if strings.Contains(reason, "无输出") || strings.Contains(reason, "no output") ||
		strings.Contains(reason, "empty output") || reason == "" {
		return FailureTargetDown
	}

	// 网络超时
	if strings.Contains(reason, "timeout") || strings.Contains(reason, "connection refused") ||
		strings.Contains(reason, "no route to host") || strings.Contains(reason, "connection reset") {
		return FailureNetwork
	}

	// 权限不足
	if strings.Contains(reason, "permission denied") || strings.Contains(reason, "forbidden") ||
		strings.Contains(reason, "unauthorized") || strings.Contains(reason, "401") ||
		strings.Contains(reason, "403") || strings.Contains(reason, "hitl") ||
		strings.Contains(reason, "审批") {
		return FailurePermission
	}

	// 工具不存在
	if strings.Contains(reason, "not found") || strings.Contains(reason, "command not found") ||
		strings.Contains(reason, "no such file") || strings.Contains(reason, "未安装") {
		return FailureToolMissing
	}

	// 参数错误
	if strings.Contains(reason, "invalid argument") || strings.Contains(reason, "missing required") ||
		strings.Contains(reason, "参数") || strings.Contains(reason, "缺") {
		return FailureArgument
	}

	// 目标不可达
	if strings.Contains(reason, "host unreachable") || strings.Contains(reason, "service unavailable") ||
		strings.Contains(reason, "connection timed out") {
		return FailureTargetDown
	}

	return FailureUnknown
}

// SuggestSolution —— 根据失败模式建议解决方案。
func SuggestSolution(mode FailureMode, action core.Action, reason string) string {
	switch mode {
	case FailureNetwork:
		return "增加超时时间 (--max-time 60) 或 retry 次数 (--retry 3)"
	case FailurePermission:
		if strings.Contains(reason, "hitl") || strings.Contains(reason, "审批") {
			return "操作员拒绝, 尝试其他工具或调整参数降低风险等级"
		}
		return "尝试其他认证方式或提权工具"
	case FailureToolMissing:
		return fmt.Sprintf("工具 %s 未安装, 运行依赖检测或使用替代工具", action.Tool)
	case FailureArgument:
		return "检查参数规格 (ArgSpec), 补全必填参数或修正格式"
	case FailureTargetDown:
		return "目标不可达, 验证网络连通性或更换目标"
	default:
		return "未知失败, 检查工具输出详情"
	}
}

// RecordFailure —— 记录失败教训到 SQLite。
func (r *ReflexionEnhanced) RecordFailure(action core.Action, reason string) error {
	mode := ClassifyFailure(reason)
	solution := SuggestSolution(mode, action, reason)

	argsJSON, _ := json.Marshal(action.Args)

	// 插入数据库
	_, err := r.db.Exec(`
		INSERT INTO lessons (tool, args, reason, mode, solution)
		VALUES (?, ?, ?, ?, ?)
	`, action.Tool, string(argsJSON), reason, failureModeString(mode), solution)

	if err != nil {
		return fmt.Errorf("插入 lesson 失败: %w", err)
	}

	return nil
}

// QueryLessons —— 查询相关教训（用于 Few-shot 注入 prompt）。
func (r *ReflexionEnhanced) QueryLessons(tool string, limit int) ([]LessonRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, tool, args, reason, mode, solution, created_at, updated_at
		FROM lessons
		WHERE tool = ?
		ORDER BY updated_at DESC
		LIMIT ?
	`, tool, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []LessonRecord
	for rows.Next() {
		var l LessonRecord
		if err := rows.Scan(&l.ID, &l.Tool, &l.Args, &l.Reason, &l.Mode, &l.Solution, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}

	return lessons, nil
}

// QueryRecentLessons —— 查询最近教训（用于全局 Few-shot）。
func (r *ReflexionEnhanced) QueryRecentLessons(limit int) ([]LessonRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, tool, args, reason, mode, solution, created_at, updated_at
		FROM lessons
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lessons []LessonRecord
	for rows.Next() {
		var l LessonRecord
		if err := rows.Scan(&l.ID, &l.Tool, &l.Args, &l.Reason, &l.Mode, &l.Solution, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}

	return lessons, nil
}

// ShouldRetry —— 判断是否应自动 retry（可恢复失败）。
func ShouldRetry(reason string) bool {
	mode := ClassifyFailure(reason)

	// 可恢复失败: 网络超时 / 目标暂时不可达
	if mode == FailureNetwork || mode == FailureTargetDown {
		return true
	}

	// 参数错误且不是必填参数缺失（格式错误可以retry）
	if mode == FailureArgument && !strings.Contains(strings.ToLower(reason), "required") {
		return true
	}

	return false
}

// AdjustArgsForRetry —— 自动调整参数用于 retry。
func AdjustArgsForRetry(action core.Action, reason string) map[string]any {
	mode := ClassifyFailure(reason)
	newArgs := make(map[string]any)
	for k, v := range action.Args {
		newArgs[k] = v
	}

	switch mode {
	case FailureNetwork:
		// 增加超时时间
		if _, ok := newArgs["timeout"]; !ok {
			newArgs["timeout"] = "60"
		}
		// 增加 retry 次数
		if _, ok := newArgs["retry"]; !ok {
			newArgs["retry"] = "3"
		}

	case FailureTargetDown:
		// 延迟重试
		time.Sleep(5 * time.Second)
	}

	return newArgs
}

// FormatLessonsForPrompt —— 格式化持久化教训用于注入 prompt。
func (r *ReflexionEnhanced) FormatLessonsForPrompt(limit int) string {
	lessons, err := r.QueryRecentLessons(limit)
	if err != nil || len(lessons) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 跨战役历史教训 (持久化知识库)\n")

	for _, l := range lessons {
		sb.WriteString(fmt.Sprintf("- [%s] %s 失败: %s\n", l.Mode, l.Tool, l.Reason))
		if l.Solution != "" {
			sb.WriteString(fmt.Sprintf("  建议: %s\n", l.Solution))
		}
	}

	return sb.String()
}

// failureModeString —— 失败模式枚举转字符串。
func failureModeString(mode FailureMode) string {
	switch mode {
	case FailureNetwork:
		return "network"
	case FailurePermission:
		return "permission"
	case FailureToolMissing:
		return "tool_missing"
	case FailureArgument:
		return "argument"
	case FailureFalsePositive:
		return "false_positive"
	case FailureTargetDown:
		return "target_down"
	default:
		return "unknown"
	}
}

