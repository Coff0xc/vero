import { useStore } from '../store'

export function EvidenceDrawer() {
  const selected = useStore((s) => s.selected)
  const nodes = useStore((s) => s.nodes)
  const select = useStore((s) => s.select)
  const node = selected ? nodes[selected] : null
  const open = !!selected

  return (
    <aside
      className={`absolute top-0 right-0 bottom-0 w-[min(380px,82%)] bg-gradient-to-b from-panel to-panel2 border-l border-line shadow-2xl transition-transform duration-200 z-20 flex flex-col p-4 ${
        open ? 'translate-x-0' : 'translate-x-[103%]'
      }`}
    >
      <div className="flex items-center justify-between">
        <span className="font-disp text-[10px] tracking-[2.5px] uppercase text-muted">证据检视 · Evidence Chain</span>
        <button onClick={() => select(null)} className="text-muted hover:text-alert text-base" aria-label="关闭">
          ✕
        </button>
      </div>
      <div className="text-sm text-signal my-2.5 break-all">{selected ?? '—'}</div>
      {node && (
        <div className="text-[11px] text-muted mb-3.5">
          {node.type} ·{' '}
          <b
            className={`font-disp tracking-wider px-2 py-0.5 rounded-sm uppercase text-[10px] border ${
              node.state === 'confirmed' ? 'text-live border-live' : 'text-ghost border-ghost'
            }`}
          >
            {node.state}
          </b>
        </div>
      )}
      <div className="overflow-auto flex-1">
        {node && node.evidence.length > 0 ? (
          node.evidence.map((ev, i) => (
            <div key={i} className="my-2 border-l-2 border-l-live bg-live/5 rounded-r-sm">
              <span className="block font-disp text-[10px] tracking-wider uppercase text-live px-2.5 pt-1.5 pb-0.5">
                {ev.tool}
              </span>
              <pre className="m-0 px-2.5 pb-2 font-mono text-[11.5px] text-ink2 whitespace-pre-wrap break-all">{ev.excerpt}</pre>
            </div>
          ))
        ) : node ? (
          <div className="border-l-2 border-l-ghost bg-ghost/5 pl-2.5 text-muted text-xs py-3.5 leading-relaxed">
            尚未坐实 — 该节点是 hypothesis，还没有可回查的证据。需要一次独立验证动作才能升为 confirmed。
          </div>
        ) : (
          <div className="text-muted text-xs py-3.5 leading-relaxed">
            点击攻击图里的任一节点，查看它如何被证实——逐字证据来自哪次工具输出。
          </div>
        )}
      </div>
    </aside>
  )
}
