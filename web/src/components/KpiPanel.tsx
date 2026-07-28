import { useStore } from '../store'

export function KpiPanel() {
  const kpi = useStore((s) => s.kpi)
  const status = useStore((s) => s.status)

  return (
    <>
      <div className="font-disp text-[10px] tracking-[2.5px] uppercase text-muted px-4 pt-3 pb-1.5">
        态势 · Situational KPI
      </div>
      <div className="mx-4 mb-2 px-3 py-2.5 border border-line border-l-2 border-l-signal bg-panel text-xs leading-7 text-muted rounded-sm">
        {status === 'idle' ? (
          '待命 — 尚未发起战役'
        ) : (
          <>
            <div>
              服务 <b className="text-ink2">{kpi.services.join(' · ') || '—'}</b> &nbsp;→&nbsp; 激活场景包{' '}
              <b className="text-live">{kpi.activated.join(' · ') || '无'}</b>
            </div>
            <div>
              已证实 <b className="text-live">{kpi.confirmed}</b> · 待验证 <b className="text-ink2">{kpi.hypothesis}</b> · 证据违规{' '}
              <b className={kpi.evidenceViolations ? 'text-alert' : 'text-live'}>{kpi.evidenceViolations}</b>
            </div>
          </>
        )}
      </div>
      <div className="font-disp text-[10px] tracking-[2.5px] uppercase text-muted px-4 pt-2 pb-1.5">
        推理 / 行动信号流 · Reasoning Stream
      </div>
    </>
  )
}
