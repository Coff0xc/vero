import { useMemo, useState } from 'react'
import { useStore } from '../store'
import { LEVEL_ZH } from '../lib/i18n'
import type { LogLine } from '../types'

// 工具级别 → 中文口语化。
function levelZh(level: number): string {
  return LEVEL_ZH[level] ?? `L${level}`
}

// 从日志行提取错误消息(tool_error 在 meta.error, error 在文本里)。
function errorMsg(l: LogLine): string {
  if (l.meta?.error) return l.meta.error
  const t = l.text
  const idx = t.indexOf('✗')
  if (idx >= 0) return t.slice(idx + 1).trim()
  return t.replace(/^⚠\s*/, '')
}

// 通知铃 —— 角标 = 待批 HITL(0/1) + 最近 tool_error/error 事件数。
// 点击展开小面板: 待批项提示处理, 错误项展示消息。
export function Header() {
  const status = useStore((s) => s.status)
  const reset = useStore((s) => s.reset)
  const engineLabel = useStore((s) => s.engineLabel)
  const temperature = useStore((s) => s.temperature)
  const hitl = useStore((s) => s.hitl)
  const log = useStore((s) => s.log)
  const [target, setTarget] = useState('http://localhost:3000')
  const [notifOpen, setNotifOpen] = useState(false)

  const start = async () => {
    reset(target)
    try {
      await fetch('/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target }),
      })
    } catch (err) {
      console.error('启动战役失败:', err)
    }
  }

  const cancel = async () => {
    try {
      await fetch('/cancel', { method: 'POST' })
    } catch (err) {
      console.error('取消防败:', err)
    }
  }

  // 从最近日志派生错误项(最多展示 12 条, 新在前)。
  const { errorCount, errors } = useMemo(() => {
    const errs = log.filter((l) => l.kind === 'tool_error' || l.kind === 'error')
    return { errorCount: errs.length, errors: errs.slice(-12).reverse() }
  }, [log])

  const badge = (hitl ? 1 : 0) + errorCount

  return (
    <header className="flex items-center gap-4 px-5 py-3 border-b border-line bg-gradient-to-b from-panel to-panel2">
      <span
        className={`w-2 h-2 rounded-full ${status === 'running' ? 'bg-live animate-pulse' : 'bg-ghost'}`}
        style={{ boxShadow: status === 'running' ? '0 0 8px #4ec9b0' : 'none' }}
        title="agent 状态"
      />
      <span className="font-disp font-bold text-lg tracking-[3px]">
        VERO<span className="text-signal">·</span>
      </span>
      <span className="hidden md:inline font-disp text-[11px] tracking-[2px] text-muted uppercase">
        自主渗透作战台
      </span>
      {/* 引擎 / 思考强度指示 chip(读 store: 'engine' SSE 事件 + /api/config 温度) */}
      <span
        className="hidden sm:inline-flex items-center gap-1.5 border border-line rounded-sm px-2.5 py-1 text-[10px] font-disp tracking-wider text-muted bg-panel"
        title="当前决策引擎与思考强度(设置页可调整)"
      >
        <i className="w-1.5 h-1.5 rounded-full bg-signal inline-block" />
        <span className="text-ink2">{engineLabel || '引擎 —'}</span>
        <span className="text-ghost">·</span>
        <span className="text-ghost">思考 {temperature.toFixed(2)}</span>
      </span>
      <span className="flex-1" />
      <input
        value={target}
        onChange={(e) => setTarget(e.target.value)}
        placeholder="目标 URL / host"
        spellCheck={false}
        disabled={status === 'running'}
        className="bg-panel2 border border-line text-ink2 text-xs px-2.5 py-1.5 rounded-sm w-52 sm:w-64 font-mono outline-none focus:border-signal disabled:opacity-50"
      />

      {/* 通知铃 */}
      <div className="relative shrink-0">
        <button
          onClick={() => setNotifOpen((o) => !o)}
          aria-label="通知"
          title={badge > 0 ? `${badge} 条待处理通知` : '暂无通知'}
          className="relative p-2 text-muted hover:text-ink2 transition-colors rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-signal"
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="block"
          >
            <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
            <path d="M13.73 21a2 2 0 0 1-3.46 0" />
          </svg>
          {badge > 0 && (
            <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-alert text-ink text-[10px] font-disp font-semibold leading-4 text-center">
              {badge > 99 ? '99+' : badge}
            </span>
          )}
        </button>

        {notifOpen && (
          <>
            {/* 点击空白关闭 */}
            <div className="fixed inset-0 z-40" onClick={() => setNotifOpen(false)} />
            <div className="absolute right-0 top-full mt-2 w-80 max-h-[70vh] overflow-auto bg-panel border border-line rounded-md shadow-[0_8px_30px_rgba(0,0,0,.5)] z-50">
              <div className="px-3 py-2 border-b border-line font-disp text-[10px] tracking-[2px] uppercase text-muted sticky top-0 bg-panel">
                通知
              </div>
              {badge === 0 ? (
                <div className="px-3 py-5 text-xs text-muted text-center">暂无通知</div>
              ) : (
                <>
                  {hitl && (
                    <div className="px-3 py-2.5 border-b border-line/60">
                      <div className="text-[10px] font-disp tracking-wider uppercase text-alert mb-1.5">待批授权</div>
                      <button
                        onClick={() => setNotifOpen(false)}
                        className="w-full text-left group"
                        title="授权弹窗已打开, 请前往裁决"
                      >
                        <div className="flex items-center gap-1.5 flex-wrap">
                          <span className="px-1.5 py-0.5 text-[10px] rounded-sm border border-alert text-alert font-disp">
                            {levelZh(hitl.level)}
                          </span>
                          <code className="text-signal font-mono text-[11px]">{hitl.tool}</code>
                          <span className="ml-auto text-[10px] text-ghost group-hover:text-signal">裁决 →</span>
                        </div>
                        <div className="mt-1 text-[11px] text-muted leading-relaxed line-clamp-2">{hitl.why || '等待操作员裁决'}</div>
                      </button>
                    </div>
                  )}
                  {errors.length > 0 && (
                    <div className="px-3 py-2.5">
                      <div className="text-[10px] font-disp tracking-wider uppercase text-alert mb-1.5">
                        工具错误 ({errorCount})
                      </div>
                      {errors.map((l) => (
                        <div key={l.id} className="py-1.5 border-b border-line/40 last:border-b-0">
                          <div className="flex items-center gap-1.5">
                            <span className="text-alert shrink-0">✗</span>
                            <code className="font-mono text-[11px] text-ink2">{l.meta?.tool ?? '错误'}</code>
                          </div>
                          <div className="mt-0.5 pl-4 text-[11px] text-muted break-words leading-relaxed line-clamp-2">
                            {errorMsg(l)}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </>
              )}
            </div>
          </>
        )}
      </div>

      {status === 'running' ? (
        <button
          onClick={cancel}
          className="font-disp font-semibold tracking-wider text-sm text-alert border border-alert px-4 py-2 rounded-sm uppercase hover:bg-alert hover:text-ink transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-alert"
        >
          ■ 停止
        </button>
      ) : (
        <button
          onClick={start}
          className="font-disp font-semibold tracking-wider text-sm text-signal border border-signal px-4 py-2 rounded-sm uppercase hover:bg-signal hover:text-ink transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-live"
        >
          ▷ 启动战役
        </button>
      )}
    </header>
  )
}
