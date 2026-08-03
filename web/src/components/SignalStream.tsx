import { useEffect, useRef } from 'react'
import { useStore } from '../store'
import { EVENT_LABELS } from '../lib/i18n'

// 色映射键仍用英文 kind(仅决定边框/文字色), label 换成中文。
const BORDER: Record<string, string> = {
  step: 'border-l-ghost',
  tool: 'border-l-live',
  graph: 'border-l-live',
  edge: 'border-l-ghost',
  hitl_request: 'border-l-alert',
  route: 'border-l-signal',
  summary: 'border-l-signal',
  done: 'border-l-signal',
  plan: 'border-l-signal',
  workflow_start: 'border-l-signal',
  workflow_stage: 'border-l-ghost',
  workflow_complete: 'border-l-signal',
  workflow_cancelled: 'border-l-alert',
  tool_result: 'border-l-live',
  tool_error: 'border-l-alert',
}
const KIND: Record<string, string> = {
  step: 'text-ghost',
  tool: 'text-live',
  graph: 'text-live',
  edge: 'text-ghost',
  hitl_request: 'text-alert',
  route: 'text-signal',
  summary: 'text-signal',
  done: 'text-signal',
  plan: 'text-signal',
  workflow_start: 'text-signal',
  workflow_stage: 'text-ghost',
  workflow_complete: 'text-signal',
  workflow_cancelled: 'text-alert',
  tool_result: 'text-live',
  tool_error: 'text-alert',
}

export function SignalStream() {
  const log = useStore((s) => s.log)
  const ref = useRef<HTMLDivElement>(null)
  const stick = useRef(true)

  // 粘性滚动: 只在用户位于底部时自动跟随; 上翻看历史不被拉回(修原版无条件吸底)。
  const onScroll = () => {
    const el = ref.current
    if (!el) return
    stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48
  }

  useEffect(() => {
    if (stick.current) ref.current?.scrollTo(0, ref.current.scrollHeight)
  }, [log.length])

  return (
    <div
      ref={ref}
      onScroll={onScroll}
      className="flex-1 overflow-auto px-4 pb-4 bg-panel/60"
    >
      {log.length === 0 && (
        <div className="px-2.5 py-6 text-muted text-xs text-center">等待事件流…</div>
      )}
      {log.map((l) => {
        const meta = l.meta
        const border = BORDER[l.kind] ?? 'border-l-line'
        const color = KIND[l.kind] ?? 'text-muted'
        const label = EVENT_LABELS[l.kind] ?? l.kind

        // step: 首行「思考 L{n} · {tool}」+ 缩进第二行「▍推理 {why}」。
        if (l.kind === 'step') {
          // 徽标「思考」+ 首行 L{n} · {tool} = 设计要求的「思考 L{n} · {tool}」。
          return (
            <div key={l.id} className={`px-2.5 py-2 my-1 border-l-2 ${border} bg-white/[0.04] text-[12.5px] leading-relaxed whitespace-pre-wrap break-all rounded-r-sm`}>
              <div>
                <span className={`font-disp font-semibold tracking-wider text-[10px] mr-1.5 ${color}`}>{label}</span>
                <span className="text-muted">
                  L{meta?.level ?? '?'} · {meta?.tool ?? '—'}
                </span>
              </div>
              <div className="mt-1 pl-3 text-signal border-l border-line/60">
                ▍推理 {meta?.why || '—'}
              </div>
            </div>
          )
        }

        // plan: 高亮块(signal 边框), 整段展示计划 rationale。
        if (l.kind === 'plan') {
          return (
            <div key={l.id} className={`px-2.5 py-2 my-1 border-l-2 ${border} bg-signal/[0.06] text-[12.5px] leading-relaxed whitespace-pre-wrap break-all rounded-r-sm`}>
              <div>
                <span className={`font-disp font-semibold tracking-wider text-[10px] mr-1.5 ${color}`}>{label}</span>
                <span className="text-muted">共 {meta?.count ?? '?'} 步行动</span>
              </div>
              {meta?.rationale && (
                <div className="mt-1 pl-3 text-ink2 border-l border-signal/40">▍{meta.rationale}</div>
              )}
            </div>
          )
        }

        return (
          <div key={l.id} className={`px-2.5 py-2 my-1 border-l-2 ${border} bg-white/[0.04] text-[12.5px] leading-relaxed whitespace-pre-wrap break-all rounded-r-sm`}>
            <span className={`font-disp font-semibold tracking-wider text-[10px] mr-1.5 ${color}`}>{label}</span>
            <span className="text-muted">{l.text}</span>
          </div>
        )
      })}
    </div>
  )
}
