import { useEffect, useRef } from 'react'
import { useStore } from '../store'

const BORDER: Record<string, string> = {
  step: 'border-l-ghost',
  tool: 'border-l-live',
  graph: 'border-l-live',
  edge: 'border-l-ghost',
  hitl_request: 'border-l-alert',
  route: 'border-l-signal',
  summary: 'border-l-signal',
  done: 'border-l-signal',
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
}

export function SignalStream() {
  const log = useStore((s) => s.log)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    ref.current?.scrollTo(0, ref.current.scrollHeight)
  }, [log.length])

  return (
    <div ref={ref} className="flex-1 overflow-auto px-4 pb-4">
      {log.map((l) => (
        <div
          key={l.id}
          className={`px-2.5 py-1.5 my-1 border-l-2 ${BORDER[l.kind] ?? 'border-l-line'} bg-gradient-to-r from-white/[0.015] to-transparent text-[11.5px] leading-relaxed whitespace-pre-wrap break-all`}
        >
          <span className={`font-disp font-semibold tracking-wider uppercase text-[10px] mr-1.5 ${KIND[l.kind] ?? 'text-muted'}`}>
            {l.kind}
          </span>
          <span className="text-muted">{l.text}</span>
        </div>
      ))}
    </div>
  )
}
