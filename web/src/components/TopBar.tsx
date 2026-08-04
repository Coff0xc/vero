import { useStore } from '../store'
import { STAGES, ENGINE_ZH } from '../lib/i18n'

// 顶部状态栏 —— 全局状态中枢:
// 左: 产品名 + 目标 | 中: 阶段进度 | 右: KPI 计数 + 引擎 + 状态 + 命令面板入口。
const STAGE_COLOR: Record<string, string> = {
  idle: 'bg-ghost/40',
  recon: 'bg-info',
  scan: 'bg-warn',
  exploit: 'bg-alert',
  done: 'bg-live',
}

export function TopBar() {
  const status = useStore((s) => s.status)
  const goal = useStore((s) => s.goal)
  const stage = useStore((s) => s.stage)
  const kpi = useStore((s) => s.kpi)
  const engineLabel = useStore((s) => s.engineLabel)
  const engineSel = useStore((s) => s.engineSel)
  const togglePalette = useStore((s) => s.togglePalette)

  const stageIdx = STAGES.findIndex((s) => s.id === stage)
  const engine = engineLabel || ENGINE_ZH[engineSel] || engineSel

  return (
    <header className="h-12 shrink-0 flex items-center gap-5 px-4 border-b border-line bg-white relative z-20">
      {/* 产品名 + 目标 */}
      <div className="flex items-center gap-2.5 min-w-0">
        <span className="text-[14px] font-semibold text-ink2 tracking-tight">Vero</span>
        <span className="text-ghost/70 text-[12px] hidden md:inline">·</span>
        <span className="font-mono text-[12px] text-muted truncate max-w-[260px] hidden md:inline" title={goal}>
          {goal === '—' ? '未选定目标' : goal}
        </span>
      </div>

      {/* 阶段进度: 五段灯条 */}
      <div className="flex items-center gap-1 mx-auto" title={`当前阶段: ${STAGES[stageIdx]?.label ?? '待命'}`}>
        {STAGES.map((s, i) => (
          <div key={s.id} className="flex items-center gap-1">
            <span
              className={`inline-block h-1.5 rounded-full transition-all duration-300 ${
                i <= stageIdx && stage !== 'idle'
                  ? `${STAGE_COLOR[s.id]} ${i === stageIdx && status === 'running' ? 'w-6 animate-pulse' : 'w-4'}`
                  : 'w-4 bg-line'
              }`}
            />
            <span className={`text-[10px] ${i === stageIdx && stage !== 'idle' ? 'text-ink2 font-medium' : 'text-ghost/80'}`}>
              {s.label}
            </span>
          </div>
        ))}
      </div>

      {/* KPI 迷你计数 - 增强视觉对比 */}
      <div className="hidden lg:flex items-center gap-3.5 font-mono text-[11px]">
        <span className="text-muted">
          服务 <span className="text-blue-600 font-semibold">{kpi.services.length}</span>
        </span>
        <span className="text-muted">
          证实 <span className="text-green-600 font-semibold">{kpi.confirmed}</span>
        </span>
        <span className="text-muted">
          假设 <span className="text-gray-500">{kpi.hypothesis}</span>
        </span>
        {kpi.evidenceViolations > 0 && (
          <span className="text-alert font-medium" title="证据违规(疑似幻觉)">
            违规 {kpi.evidenceViolations}
          </span>
        )}
      </div>

      {/* 引擎 + 状态灯 + 命令入口 */}
      <div className="flex items-center gap-2.5">
        <span className="hidden sm:inline text-[11px] text-violet border border-violet/30 bg-violet/6 rounded-md px-2 py-0.5">
          {engine}
        </span>
        <span
          className={`inline-block w-2 h-2 rounded-full ${
            status === 'running' ? 'bg-warn ring-pulse' : status === 'done' ? 'bg-live' : 'bg-ghost/60'
          }`}
          title={status === 'running' ? '战役进行中' : status === 'done' ? '战役完成' : '待命'}
        />
        <button
          onClick={() => togglePalette()}
          className="flex items-center gap-1.5 text-[11.5px] text-muted hover:text-ink2 border border-line bg-white hover:border-signal/50 rounded-md px-2.5 py-1 transition-colors"
          title="命令面板 (Ctrl+K)"
        >
          命令 <span className="kbd">Ctrl K</span>
        </button>
      </div>
    </header>
  )
}
