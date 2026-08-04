import { useStore } from '../store'
import { STAGES, ENGINE_ZH } from '../lib/i18n'

// 顶部状态栏 —— 指挥台式布局的全局状态中枢:
// 左: Logo + 目标 | 中: 阶段进度条 | 右: KPI 计数 + 引擎 chip + 状态灯 + ⌘K。
const STAGE_COLOR: Record<string, string> = {
  idle: 'bg-ghost/40',
  recon: 'bg-signal',
  scan: 'bg-live',
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
    <header className="h-11 shrink-0 flex items-center gap-4 px-3.5 border-b border-line/80 glass relative z-20">
      {/* Logo + 目标 */}
      <div className="flex items-center gap-2.5 min-w-0">
        <span className="w-[26px] h-[26px] rounded-md bg-gradient-to-br from-signal/30 to-violet/20 border border-signal/40 flex items-center justify-center text-[13px] shadow-glow-signal">
          🦅
        </span>
        <span className="font-disp text-[13px] font-semibold text-ink2 tracking-wide hidden sm:inline">Vero</span>
        <span className="text-ghost text-[11px] hidden md:inline">·</span>
        <span className="font-mono text-[11px] text-muted truncate max-w-[220px] hidden md:inline" title={goal}>
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
                  ? `${STAGE_COLOR[s.id]} ${i === stageIdx && status === 'running' ? 'w-7 animate-pulse' : 'w-4'}`
                  : 'w-4 bg-line'
              }`}
            />
            <span className={`text-[9px] font-disp tracking-wider uppercase ${i === stageIdx && stage !== 'idle' ? 'text-ink2' : 'text-ghost/70'}`}>
              {s.label}
            </span>
          </div>
        ))}
      </div>

      {/* KPI 迷你计数 */}
      <div className="hidden lg:flex items-center gap-3 font-mono text-[10.5px]">
        <span className="text-muted">
          服务 <span className="text-signal">{kpi.services.length}</span>
        </span>
        <span className="text-muted">
          证实 <span className="text-live">{kpi.confirmed}</span>
        </span>
        <span className="text-muted">
          假设 <span className="text-ghost">{kpi.hypothesis}</span>
        </span>
        {kpi.evidenceViolations > 0 && (
          <span className="text-alert" title="证据违规(疑似幻觉)">
            ⚠ {kpi.evidenceViolations}
          </span>
        )}
      </div>

      {/* 引擎 chip + 状态灯 */}
      <div className="flex items-center gap-2">
        <span className="hidden sm:inline-flex items-center gap-1.5 text-[10px] font-disp tracking-wider text-violet border border-violet/30 bg-violet/10 rounded-full px-2.5 py-0.5">
          {engine}
        </span>
        <span
          className={`inline-block w-2 h-2 rounded-full ${
            status === 'running' ? 'bg-warn ring-pulse' : status === 'done' ? 'bg-live shadow-glow-live' : 'bg-ghost'
          }`}
          title={status === 'running' ? '战役进行中' : status === 'done' ? '战役完成' : '待命'}
        />
        <button
          onClick={() => togglePalette()}
          className="flex items-center gap-1.5 text-[10.5px] text-muted hover:text-ink2 border border-line/80 bg-panel2/70 hover:border-signal/40 rounded-md px-2 py-1 transition-colors"
          title="命令面板 (Ctrl+K)"
        >
          <span className="text-[11px]">⌘</span>
          <span className="kbd">Ctrl K</span>
        </button>
      </div>
    </header>
  )
}
