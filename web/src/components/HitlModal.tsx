import { useStore } from '../store'
import { LEVEL_ZH } from '../lib/i18n'

// 工具级别 → 中文口语化: L0=侦察级, L1=扫描级, L2=凭证级, L3=利用级, L4=破坏级。
function levelZh(level: number): string {
  return LEVEL_ZH[level] ?? `L${level}`
}

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
          ⚠ 需要授权
        </div>
        <div className="text-[11px] text-muted mb-4">agent 请求执行高危动作, 等待操作员裁决</div>

        <div className="text-sm leading-relaxed mb-4">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="px-2 py-0.5 text-[10px] rounded-sm border border-alert text-alert font-disp tracking-wider">
              {levelZh(hitl.level)}
            </span>
            <code className="text-signal font-mono">{hitl.tool}</code>
          </div>
          <div className="mt-3 space-y-2">
            <div className="text-xs text-muted">调用参数</div>
            <pre className="m-0 bg-panel2 border border-line rounded-sm px-3 py-2 font-mono text-[11.5px] text-ink2 whitespace-pre-wrap break-all max-h-40 overflow-auto">
              {JSON.stringify(hitl.args, null, 2)}
            </pre>
          </div>
          <div className="mt-3">
            <div className="text-xs text-muted mb-1">操作理由</div>
            <div className="text-[12.5px] text-ink2 leading-relaxed border-l-2 border-l-signal pl-2.5">
              {hitl.why || '—'}
            </div>
          </div>
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
