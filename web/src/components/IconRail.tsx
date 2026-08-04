import { useStore, type Tab } from '../store'
import { IconChat, IconTools, IconWorkflow, IconReport, IconSettings, IconHistory, IconPlus, type IconComponent } from './Icon'

// 左侧图标轨 —— 窄导航(52px): 历史抽屉 / 新会话 / 五个功能分区。
// 宽 Sidebar 改为抽屉(HistoryDrawer), 把横向空间还给对话与攻击图。
const NAV: { id: Tab; label: string; Icon: IconComponent; kbd?: string }[] = [
  { id: 'campaign', label: '对话作战', Icon: IconChat, kbd: '1' },
  { id: 'tools', label: '工具管理', Icon: IconTools, kbd: '2' },
  { id: 'workflows', label: '工作流', Icon: IconWorkflow, kbd: '3' },
  { id: 'reports', label: '报告', Icon: IconReport, kbd: '4' },
  { id: 'settings', label: '设置', Icon: IconSettings, kbd: '5' },
]

export function IconRail() {
  const activeTab = useStore((s) => s.activeTab)
  const setTab = useStore((s) => s.setTab)
  const toggleHistory = useStore((s) => s.toggleHistory)
  const reset = useStore((s) => s.reset)
  const status = useStore((s) => s.status)

  return (
    <nav className="w-[52px] shrink-0 flex flex-col items-center py-3 gap-1 border-r border-line/80 bg-panel2/60">
      {/* 历史抽屉开关 */}
      <button
        onClick={() => toggleHistory()}
        className="w-10 h-10 rounded-lg flex items-center justify-center text-muted hover:text-ink2 hover:bg-ink/60 border border-transparent hover:border-line/60 transition-all"
        title="历史战役 (Ctrl+H)"
      >
        <IconHistory size={18} />
      </button>
      {/* 新会话 */}
      <button
        onClick={() => reset('')}
        className="w-10 h-10 rounded-lg flex items-center justify-center text-signal hover:bg-signal/10 border border-transparent hover:border-signal/40 hover:shadow-glow-signal transition-all"
        title="新会话"
      >
        <IconPlus size={18} />
      </button>

      <div className="w-6 h-px bg-line/80 my-1.5" />

      {NAV.map(({ id, label, Icon, kbd }) => (
        <button
          key={id}
          onClick={() => setTab(id)}
          className={`relative w-10 h-10 rounded-lg flex items-center justify-center transition-all ${
            activeTab === id
              ? 'bg-signal/12 text-signal border border-signal/35 shadow-glow-signal'
              : 'text-muted hover:text-ink2 hover:bg-ink/60 border border-transparent'
          }`}
          title={`${label} (Ctrl+${kbd})`}
        >
          <Icon size={18} />
          {activeTab === id && <span className="absolute left-[-7px] top-1/2 -translate-y-1/2 w-[3px] h-5 rounded-r bg-signal" />}
        </button>
      ))}

      {/* 底部: 运行状态灯 */}
      <div className="mt-auto pb-1">
        <span
          className={`inline-block w-2.5 h-2.5 rounded-full ${
            status === 'running' ? 'bg-warn ring-pulse' : status === 'done' ? 'bg-live shadow-glow-live' : 'bg-ghost/60'
          }`}
          title={status === 'running' ? '战役进行中' : status === 'done' ? '战役完成' : '待命'}
        />
      </div>
    </nav>
  )
}
