import { useEffect, useState } from 'react'
import { useStore } from '../store'

interface Campaign {
  id: number
  goal: string
  started_at: number
  confirmed: number
  hypothesis: number
  status: string
}

// 左侧会话边栏(ChatGPT/Claude 风格): New Chat + 历史战役列表 + 辅助导航。
export function ChatSidebar({
  activeTab,
  onTab,
}: {
  activeTab: string
  onTab: (t: string) => void
}) {
  const status = useStore((s) => s.status)
  const goal = useStore((s) => s.goal)
  const [campaigns, setCampaigns] = useState<Campaign[]>([])

  const loadCampaigns = async () => {
    try {
      const r = await fetch('/api/campaigns')
      if (r.ok) setCampaigns((await r.json()) as Campaign[])
    } catch {
      /* 静默: 列表加载失败不影响主流程 */
    }
  }
  useEffect(() => {
    void loadCampaigns()
    const t = setInterval(() => void loadCampaigns(), 5000)
    return () => clearInterval(t)
  }, [])

  const reset = useStore((s) => s.reset)
  const newChat = () => reset('')

  const nav = [
    { id: 'campaign', label: '对话', icon: '💬' },
    { id: 'tools', label: '工具管理', icon: '🧰' },
    { id: 'workflows', label: '工作流', icon: '⚡' },
    { id: 'reports', label: '报告', icon: '📄' },
    { id: 'settings', label: '设置', icon: '⚙' },
  ]

  return (
    <aside className="w-[230px] shrink-0 flex flex-col border-r border-line bg-[#0d1218] min-h-0">
      {/* Logo + New Chat */}
      <div className="px-3 pt-3 pb-2">
        <div className="flex items-center gap-2 px-1 mb-3">
          <span className="w-7 h-7 rounded-lg bg-live/15 border border-live/40 flex items-center justify-center text-[15px]">
            🦅
          </span>
          <div>
            <div className="font-disp text-[13px] text-ink2 tracking-wide leading-none">Vero</div>
            <div className="text-[9.5px] text-ghost tracking-wider uppercase mt-0.5">AI Pentest Agent</div>
          </div>
        </div>
        <button
          onClick={newChat}
          className="w-full flex items-center gap-2 px-3 py-2 rounded-lg border border-line text-[12.5px] text-muted hover:text-ink2 hover:border-live/40 transition-colors"
        >
          <span className="text-live">✚</span> 新会话
        </button>
      </div>

      {/* 会话历史 */}
      <div className="flex-1 min-h-0 overflow-y-auto px-3 pb-2">
        <div className="text-[9.5px] text-ghost tracking-[2px] uppercase px-1 py-2">历史战役</div>
        {campaigns.length === 0 && (
          <div className="text-[11px] text-ghost px-1 py-1">暂无历史战役</div>
        )}
        {campaigns.map((c) => (
          <button
            key={c.id}
            className="w-full text-left px-2.5 py-2 rounded-md hover:bg-ink/50 transition-colors mb-0.5"
          >
            <div className="text-[12px] text-muted truncate">{c.goal}</div>
            <div className="text-[10px] text-ghost font-mono mt-0.5">
              #{c.id} · ✓{c.confirmed} · ○{c.hypothesis} · {c.status}
            </div>
          </button>
        ))}
      </div>

      {/* 运行状态 + 底部导航 */}
      <div className="border-t border-line px-3 py-2.5">
        <div className="flex items-center gap-2 mb-2 px-1">
          <span
            className={`inline-block w-2 h-2 rounded-full ${
              status === 'running' ? 'bg-signal animate-pulse' : status === 'done' ? 'bg-live' : 'bg-ghost'
            }`}
          />
          <span className="text-[10.5px] text-muted font-mono truncate">
            {status === 'running' ? '战役进行中' : status === 'done' ? '战役完成' : '待命'}
            {goal && goal !== '—' ? ` · ${goal}` : ''}
          </span>
        </div>
        <nav className="flex flex-col gap-0.5">
          {nav.map((n) => (
            <button
              key={n.id}
              onClick={() => onTab(n.id)}
              className={`flex items-center gap-2 px-2.5 py-1.5 rounded-md text-[12px] transition-colors ${
                activeTab === n.id ? 'bg-live/10 text-live' : 'text-muted hover:text-ink2 hover:bg-ink/40'
              }`}
            >
              <span className="text-[13px]">{n.icon}</span>
              {n.label}
            </button>
          ))}
        </nav>
      </div>
    </aside>
  )
}
