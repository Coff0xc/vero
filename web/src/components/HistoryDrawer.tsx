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

// 历史战役抽屉 —— 从左侧导航滑出; 点击进入只读回放, 删除移除记录。
// 回放期间与实时战役隔离(回放时实时 SSE 不灌入)。纯文字, 无图标。
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
      <div className="fixed inset-0 bg-black/40 z-30" onClick={() => toggle(false)} />
      <aside className="fixed left-[84px] top-12 bottom-0 w-[300px] z-40 bg-white border-r border-line drawer-in flex flex-col shadow-pop">
        <div className="flex items-center justify-between px-4 py-3 border-b border-line">
          <span className="section-title text-[12px] font-medium text-muted">历史战役</span>
          <button
            onClick={() => toggle(false)}
            className="text-muted hover:text-alert hover:bg-alert/8 rounded px-1.5 py-0.5 text-[11px] transition-colors"
          >
            关闭
          </button>
        </div>
        {status === 'running' && (
          <div className="mx-3 mt-2.5 text-[11px] text-warn border border-warn/30 bg-warn/6 rounded-md px-2.5 py-1.5 leading-relaxed">
            战役进行中, 结束后才能回放历史
          </div>
        )}
        <div className="flex-1 min-h-0 overflow-y-auto p-2">
          {campaigns.length === 0 && <div className="text-[12px] text-ghost px-2 py-3">暂无历史战役</div>}
          {campaigns.map((c) => (
            <div
              key={c.id}
              className="group w-full text-left px-2.5 py-2 rounded-md hover:bg-panel2 transition-colors duration-150 mb-0.5 flex items-center gap-1"
            >
              <button
                className={`flex-1 min-w-0 text-left ${status === 'running' ? 'opacity-40 cursor-not-allowed' : ''}`}
                onClick={() => void openCampaign(c.id)}
                disabled={status === 'running'}
              >
                <div className="text-[12.5px] text-muted group-hover:text-ink2 truncate transition-colors">{c.goal}</div>
                <div className="text-[10.5px] text-ghost mt-0.5">
                  #{c.id} · 证实 <span className="text-live">{c.confirmed}</span> · 假设 {c.hypothesis} · {c.status}
                </div>
              </button>
              <button
                onClick={() => void del(c.id)}
                title="删除此会话"
                className="shrink-0 text-[10.5px] text-ghost hover:text-alert opacity-0 group-hover:opacity-100 transition-opacity px-1.5 py-1 rounded hover:bg-alert/8"
              >
                删除
              </button>
            </div>
          ))}
        </div>
      </aside>
    </>
  )
}
