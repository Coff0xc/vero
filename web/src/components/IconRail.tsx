import { useStore, type Tab } from '../store'

// 左侧导航轨 —— 纵向文字导航(84px): 历史 / 新建 + 五个功能分区。
// 去图标化: 用简洁中文文字标签, 选中态为左侧琥珀刻度 + 文字加深。
const NAV: { id: Tab; label: string; kbd?: string }[] = [
  { id: 'campaign', label: '对话', kbd: '1' },
  { id: 'tools', label: '工具', kbd: '2' },
  { id: 'workflows', label: '工作流', kbd: '3' },
  { id: 'reports', label: '报告', kbd: '4' },
  { id: 'settings', label: '设置', kbd: '5' },
]

export function IconRail() {
  const activeTab = useStore((s) => s.activeTab)
  const setTab = useStore((s) => s.setTab)
  const toggleHistory = useStore((s) => s.toggleHistory)
  const reset = useStore((s) => s.reset)
  const status = useStore((s) => s.status)

  const itemBase = 'relative w-full flex items-center justify-center py-2.5 text-[12.5px] rounded-md transition-colors duration-150'

  return (
    <nav className="w-[84px] shrink-0 flex flex-col items-stretch px-2 py-3 gap-0.5 border-r border-line bg-panel">
      {/* 历史 / 新建 */}
      <button
        onClick={() => toggleHistory()}
        className={`${itemBase} text-muted hover:text-ink2 hover:bg-panel2`}
        title="历史战役 (Ctrl+H)"
      >
        历史
      </button>
      <button
        onClick={() => reset('')}
        className={`${itemBase} text-signal font-medium hover:bg-signal/8`}
        title="新建会话"
      >
        新建
      </button>

      <div className="mx-2 my-2 h-px bg-line" />

      {NAV.map((n) => {
        const active = activeTab === n.id
        return (
          <button
            key={n.id}
            onClick={() => setTab(n.id)}
            className={`${itemBase} ${active ? 'text-signal font-semibold bg-signal/8' : 'text-muted hover:text-ink2 hover:bg-panel2'}`}
            title={`${n.label} (Ctrl+${n.kbd})`}
          >
            {active && <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 rounded-r bg-signal" />}
            {n.label}
          </button>
        )
      })}

      {/* 底部: 运行状态 */}
      <div className="mt-auto flex items-center justify-center gap-1.5 pb-1 text-[10px] text-ghost">
        <span
          className={`inline-block w-2 h-2 rounded-full ${
            status === 'running' ? 'bg-warn ring-pulse' : status === 'done' ? 'bg-live' : 'bg-ghost/50'
          }`}
        />
        {status === 'running' ? '运行' : status === 'done' ? '完成' : '待命'}
      </div>
    </nav>
  )
}
