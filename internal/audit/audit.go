// Package audit —— 审计日志 + 回滚登记 + 防注入检测(对应 Python audit.py)。
//
//   - 每个动作 append-only 写 JSONL(谁/何时/什么/结果), 事后可追溯。
//   - L4 破坏/持久化动作登记可回滚项, 收尾能清理。
//   - 目标衍生文本过注入检测, 命中标记(不阻断, 交 HITL 判断; 系统提示是第一层)。
//
// 挂接: 谁跑 agent 谁把 Record 挂到 emit 回调即可, 不侵入内核循环。
package audit

import (
	"encoding/json"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/Coff0xc/vero/internal/tools"
)

// Auditor —— append-only 审计器(文件路径可配, 并发安全)。
type Auditor struct {
	logPath      string
	rollbackPath string
	mu           sync.Mutex
}

func New(logPath, rollbackPath string) *Auditor {
	return &Auditor{logPath: logPath, rollbackPath: rollbackPath}
}

// Record —— 审计一个动作。
// success 用 *bool: nil=未知(执行前意图), &true/&false=结果 —— 修复 Python 原缺陷
// (原实现在 step 事件即写 success=null, 结果永不回填, 日志只见意图不见成败)。
func (a *Auditor) Record(tool string, args map[string]any, level int, success *bool, extra map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	rec := map[string]any{
		"ts": time.Now().Unix(), "tool": tool, "args": args, "level": level, "success": success,
	}
	for k, v := range extra {
		rec[k] = v
	}
	if err := appendJSONL(a.logPath, rec); err != nil {
		return err
	}
	if level >= tools.LevelDestruct { // L4 破坏/持久化 -> 登记可回滚项
		return appendJSONL(a.rollbackPath, map[string]any{
			"ts": rec["ts"], "tool": tool, "args": args, "undo": "手动清理 " + tool,
		})
	}
	return nil
}

// appendJSONL —— 追加一条 JSON(每行一条)。SetEscapeHTML(false) 保持 URL/中文原样, 不转义。
func appendJSONL(path string, rec any) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(rec) // Encode 自带换行, 正好一条一行
}

// reInjection —— prompt 注入特征(对应 Python INJECTION_PAT)。
var reInjection = regexp.MustCompile(`(?i)(ignore\s+(previous|above|all)|disregard|system\s*prompt|you\s+are\s+now|###\s*instruction|<\|.*?\|>)`)

// ScanInjection —— 目标衍生文本过注入检测, 返回命中的片段(不阻断, 交 HITL)。
func ScanInjection(text string) []string {
	return reInjection.FindAllString(text, -1)
}
