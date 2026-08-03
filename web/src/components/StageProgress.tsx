// 战役阶段进度条 —— 待命→侦察→扫描→利用→完成。
// 阶段由 store 根据 SSE 事件(engine/step/tool/route/summary/done)推断, 只前进不后退。
import { useStore } from '../store'
import { STAGES } from '../lib/i18n'

// 各阶段在 STAGES 数组中的下标。
const INDEX: Record<string, number> = { idle: 0, recon: 1, scan: 2, exploit: 3, done: 4 }

export function StageProgress() {
  const stage = useStore((s) => s.stage)
  const status = useStore((s) => s.status)
  const log = useStore((s) => s.log)

  // 当前动作: 取最近一条带工具名的 step/tool 事件。
  const current = [...log].reverse().find((l) => (l.kind === 'step' || l.kind === 'tool') && l.meta?.tool)
  const curIdx = Math.max(0, INDEX[stage] ?? 0)

  return (
    <div className="px-4 pt-2.5 pb-1.5">
      <div className="flex items-center gap-1.5">
        {STAGES.map((s, i) => {
          const active = i <= curIdx
          const isNow = i === curIdx
          const color =
            s.id === 'done' ? (status === 'done' ? 'bg-signal' : 'bg-ghost/50')
            : s.id === 'exploit' ? 'bg-alert'
            : active ? 'bg-live' : 'bg-ghost/50'
          return (
            <div key={s.id} className="flex-1 flex items-center gap-1.5">
              <div
                className={`flex-1 h-1 rounded-full transition-colors ${color} ${isNow && status === 'running' ? 'animate-pulse' : ''}`}
                title={s.label}
              />
              <span className={`text-[9px] font-disp tracking-wider whitespace-nowrap ${isNow ? 'text-live' : 'text-muted'}`}>
                {s.label}
              </span>
            </div>
          )
        })}
      </div>
      <div className="mt-1.5 flex items-center gap-2 min-w-0">
        {current ? (
          <>
            <span className="text-[10px] text-live shrink-0">▸ 当前动作</span>
            <span className="font-mono text-[11px] text-ink2 truncate">{current.meta!.tool}</span>
            {typeof current.meta!.level === 'number' && (
              <span className="text-[10px] text-muted shrink-0">L{current.meta!.level}</span>
            )}
          </>
        ) : (
          <span className="text-[10px] text-ghost">{status === 'running' ? '等待决策引擎规划…' : '尚未发起战役'}</span>
        )}
      </div>
    </div>
  )
}
