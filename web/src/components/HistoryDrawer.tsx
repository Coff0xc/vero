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

// 历史战役抽屉 —— 从图标轨滑出; 点击进入只读回放, 删除移除记录。
// 回放期间置顶"返回实时"条, 避免与进行中战役状态混杂(回放时实时 SSE 不灌入)。
export function HistoryDrawer() {
  const open = useStore((s) => s.historyOpen)
  const toggle = useStore((s) => s.toggleHistory)
  const status = useStore((s) => s.status)
  const [campaigns, setCampaigns] = useState<Campaign[]>([])
  const [replaying, setReplaying] = useState<number | null>(null)

  useEffect(() => {
    if (!open) return
    let dead = false
    fetch('/api/campaigns')
      .then(async (r) => {
        if (r.ok && !dead) setCampaigns((await r.json()) as Campaign[])
      })
      .catch(() => {/* 静默: 列表加载失败不影响主流程 */})
    return () => { dead = true }
  }, [open])

  // 历史会话点击 -> 拉事件流回放。战役进行中禁止回放(防状态混杂)。
  const openCampaign = async (id: number) => {
    if (status === 'running') return
    try {
      const r = await fetch(`/api/campaigns/${id}/events`)
      if (!r.ok) return
      const evs = (await r.json()) as { kind: string; data: Record<string, unknown> }[]
      const parsed = evs
        .map((e) => parseEvent({ kind: e.kind, data: e.data }))
        .filter((e): e is NonNullable<ReturnType<typeof parseEvent>> => e !== null)
      if (parsed.length > 0) {
        useStore.getState().replay(parsed)
        setReplaying(id)
        toggle(false)
      }
    } catch {/* 静默 */}
  }

  const del = async (id: number) => {
    try {
      const r = await fetch(`/api/campaigns/${id}`, { method: 'DELETE' })
      if (r.ok) {
        setCampaigns((cs) => cs.filter((c) => c.id !== id))
        if (replaying === id) setReplaying(null)
      }
    } catch {/* 静默 */}
  }

  if (!open) return null

  return (
    <>
      {/* 遮罩: 点击关闭 */}
      <div className="fixed inset-0 bg-black/50 z-30" onClick={() => toggle(false)} />
      <aside className="fixed left-[52px] top-11 bottom-0 w-[280px] z-40 glass border-r border-line/80 drawer-in flex flex-col">
        <div className="flex items-center justify-between px-3.5 py-3 border-b border-line/60">
          <span className="section-title font-disp text-[10px] uppercase text-muted">历史战役</span>
          <button
            onClick={() => toggle(false)}
            className="text-muted hover:text-alert hover:bg-alert/10 rounded-md w-6 h-6 flex items-center justify-center text-sm transition-colors"
            aria-label="关闭"
          >
            ✕
          </button>
        </div>
        {status === 'running' && (
          <div className="mx-3 mt-2.5 text-[10.5px] text-warn/90 border border-warn/25 bg-warn/5 rounded-md px-2.5 py-1.5 leading-relaxed">
            战役进行中, 结束后才能回放历史
          </div>
        )}
        <div className="flex-1 min-h-0 overflow-y-auto p-2.5">
          {campaigns.length === 0 && <div className="text-[11px] text-ghost px-1.5 py-2">暂无历史战役</div>}
          {campaigns.map((c) => (
            <div
              key={c.id}
              className="group w-full text-left px-2.5 py-2 rounded-lg hover:bg-ink/60 transition-all duration-150 mb-1 flex items-center gap-1 border border-transparent hover:border-line/60"
            >
              <button
                className={`flex-1 min-w-0 ${status === 'running' ? 'opacity-40 cursor-not-allowed' : ''}`}
                onClick={() => void openCampaign(c.id)}
                disabled={status === 'running'}
              >
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
      </aside>
    </>
  )
}
