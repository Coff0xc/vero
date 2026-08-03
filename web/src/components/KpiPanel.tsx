import { useStore } from '../store'
import { useEffect, useState } from 'react'
import { StageProgress } from './StageProgress'

// 服务名截断: 过长不撑破布局(修原版 join 后无限长)。
function clipList(items: string[], max: number): string {
  const total = items.join(' · ')
  return total.length > max ? total.slice(0, max) + '…' : total || '—'
}

// 态势统计条 —— 三统计块横排(已证实/待验证/证据违规), 沿用 kpi-flash 闪烁 + 服务/激活场景行。
export function KpiPanel() {
  const kpi = useStore((s) => s.kpi)
  const status = useStore((s) => s.status)
  const [flashKey, setFlashKey] = useState(0)

  // KPI 变化时触发一次闪烁动画(修原版数字变化无任何提示)。
  useEffect(() => {
    setFlashKey((k) => k + 1)
  }, [kpi.confirmed, kpi.hypothesis, kpi.evidenceViolations])

  return (
    <>
      <div className="font-disp text-[10px] tracking-[2.5px] uppercase text-muted px-4 pt-3 pb-1.5">
        态势
      </div>
      <div className="mx-4 mb-2 px-3 py-2.5 border border-line bg-panel rounded-sm">
        {/* 三统计块横排 */}
        <div className="grid grid-cols-3 gap-2 text-center">
          <div className="border border-live/30 bg-live/5 rounded-sm py-1.5">
            <div key={`c-${flashKey}`} className="text-lg font-disp font-semibold text-live kpi-flash leading-none">
              {status === 'idle' ? '—' : kpi.confirmed}
            </div>
            <div className="text-[10px] text-muted mt-1 tracking-wider">已证实</div>
          </div>
          <div className="border border-ghost/30 bg-ghost/5 rounded-sm py-1.5">
            <div key={`h-${flashKey}`} className="text-lg font-disp font-semibold text-ink2 kpi-flash leading-none">
              {status === 'idle' ? '—' : kpi.hypothesis}
            </div>
            <div className="text-[10px] text-muted mt-1 tracking-wider">待验证</div>
          </div>
          <div className={`border rounded-sm py-1.5 ${kpi.evidenceViolations ? 'border-alert/40 bg-alert/5' : 'border-line bg-white/[0.02]'}`}>
            <div
              key={`v-${flashKey}`}
              className={`text-lg font-disp font-semibold kpi-flash leading-none ${
                kpi.evidenceViolations ? 'text-alert' : 'text-live'
              }`}
            >
              {status === 'idle' ? '—' : kpi.evidenceViolations}
            </div>
            <div className="text-[10px] text-muted mt-1 tracking-wider">证据违规</div>
          </div>
        </div>

        {/* 服务 / 激活场景行 */}
        <div className="mt-2.5 pt-2 border-t border-line text-[11px] leading-5 text-muted">
          <div>
            服务 <b className="text-ink2">{clipList(kpi.services, 64)}</b> → 激活场景包{' '}
            <b className="text-live">{clipList(kpi.activated, 48)}</b>
          </div>
        </div>
      </div>

      {/* 战役阶段进度 */}
      <StageProgress />

      <div className="font-disp text-[10px] tracking-[2.5px] uppercase text-muted px-4 pt-2 pb-1.5">
        推理 / 行动信号流
      </div>
    </>
  )
}
