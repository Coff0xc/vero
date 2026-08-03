import { useEffect, useState } from 'react'
import { useStore, parseEvent } from '../store'

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

  // 历史会话点击 -> 拉事件流回放(恢复查看那次战役的对话/攻击图)。
  const openCampaign = async (id: number) => {
    // 进行中战役不切换回放(避免实时 SSE 混入回放视图)。
    if (useStore.getState().status === 'running') return
    try {
      const r = await fetch(`/api/campaigns/${id}/events`)
      if (!r.ok) return
      const evs = (await r.json()) as { kind: string; data: Record<string, unknown> }[]
      const parsed = evs
        .map((e) => {
          const raw = { kind: e.kind, data: e.data }
          return parseEvent(raw)
        })
        .filter((e): e is NonNullable<ReturnType<typeof parseEvent>> => e !== null)
      if (parsed.length > 0) useStore.getState().replay(parsed)
    } catch {
      /* 静默 */
    }
  }

  // 删除历史会话(后端 DELETE /api/campaigns/{id})。
  const del = async (id: number) => {
    try {
      const r = await fetch(`/api/campaigns/${id}`, { method: 'DELETE' })
      if (r.ok) setCampaigns((cs) => cs.filter((c) => c.id !== id))
    } catch {
      /* 静默 */
    }
  }

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
      <div className="px-3 pt-3.5 pb-2">
        <div className="flex items-center gap-2.5 px-1 mb-3.5">
          <span className="w-8 h-8 rounded-lg bg-gradient-to-br from-live/25 to-live/5 border border-live/40 flex items-center justify-center text-[16px] shadow-glow-live">
            🦅
          </span>
          <div>
            <div className="font-disp text-[14px] font-semibold text-ink2 tracking-wide leading-none">Vero</div>
            <div className="text-[9.5px] text-ghost tracking-wider uppercase mt-1">AI Pentest Agent</div>
          </div>
        </div>
        <button
          onClick={newChat}
          className="w-full flex items-center gap-2 px-3 py-2 rounded-lg border border-line bg-panel2/50 text-[12.5px] text-muted hover:text-ink2 hover:border-live/50 hover:bg-live/5 hover:shadow-glow-live transition-all duration-200"
        >
          <span className="text-live font-medium">✚</span> 新会话
        </button>
      </div>

      {/* 会话历史 */}
      <div className="flex-1 min-h-0 overflow-y-auto px-3 pb-2">
        <div className="section-title text-[9.5px] text-ghost uppercase px-1 py-2">历史战役</div>
        {campaigns.length === 0 && (
          <div className="text-[11px] text-ghost px-1 py-1">暂无历史战役</div>
        )}
        {campaigns.map((c) => (
          <div
            key={c.id}
            className="group w-full text-left px-2.5 py-2 rounded-md hover:bg-ink/60 hover:shadow-inner-line transition-all duration-150 mb-0.5 flex items-center gap-1 border border-transparent hover:border-line/60"
          >
            <button className="flex-1 min-w-0" onClick={() => void openCampaign(c.id)}>
              <div className="text-[12px] text-muted group-hover:text-ink2 truncate transition-colors">{c.goal}</div>
              <div className="text-[10px] text-ghost font-mono mt-0.5">
                #{c.id} · <span className="text-live/80">✓{c.confirmed}</span> · <span className="text-ghost">○{c.hypothesis}</span> · {c.status}
              </div>
            </button>
            <button
              onClick={() => void del(c.id)}
              title="删除此会话"
              className="shrink-0 text-[11px] text-ghost hover:text-alert opacity-0 group-hover:opacity-100 transition-all px-1.5 py-0.5 rounded hover:bg-alert/10"
            >
              ✕
            </button>
          </div>
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
