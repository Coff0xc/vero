import { useStore } from '../store'

export function HitlModal() {
  const hitl = useStore((s) => s.hitl)
  const clear = useStore((s) => s.clearHitl)
  if (!hitl) return null

  const decide = async (approved: boolean) => {
    await fetch('/approve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: hitl.key, approved }),
    })
    clear()
  }

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
      <div className="bg-panel border border-alert border-t-[3px] rounded-md px-6 py-5 max-w-md w-full shadow-[0_0_40px_rgba(255,92,77,.18)]">
        <div className="font-disp font-bold tracking-wider text-sm text-alert uppercase mb-1">
          ⚠ 需要授权 · Action Requires Authorization
        </div>
        <div className="text-[11px] text-muted mb-4">agent 请求执行高危动作，等待操作员裁决</div>
        <div className="text-sm leading-relaxed mb-4 break-all">
          <b className="text-alert">L{hitl.level}</b> &nbsp;<code className="text-signal">{hitl.tool}</code>{' '}
          {JSON.stringify(hitl.args)}
          <br />
          <span className="text-muted">理由:</span> {hitl.why}
        </div>
        <div className="flex gap-2.5">
          <button
            onClick={() => decide(true)}
            className="flex-1 font-disp font-semibold tracking-wider text-sm py-2.5 rounded-sm uppercase border border-live text-live hover:bg-live hover:text-ink"
          >
            批准执行
          </button>
          <button
            onClick={() => decide(false)}
            className="flex-1 font-disp font-semibold tracking-wider text-sm py-2.5 rounded-sm uppercase border border-muted text-muted hover:border-alert hover:text-alert"
          >
            拒绝
          </button>
        </div>
      </div>
    </div>
  )
}
