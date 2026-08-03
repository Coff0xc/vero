import { useStore } from '../store'
import { NODE_STATE_LABELS } from '../lib/i18n'

// 节点 type → 中文(仅展示层, type 标识本身保持英文)。
const NODE_TYPE_LABELS: Record<string, string> = {
  host: '主机',
  service: '服务',
  finding: '发现',
  web_shell: 'Web 后门',
  cred: '凭证',
  claim: '声明',
  foothold: '据点',
}

// 证据抽屉: 作为右栏的兄弟列展开(flex 布局让位), 不再绝对定位盖住攻击图。
export function EvidenceDrawer() {
  const selected = useStore((s) => s.selected)
  const nodes = useStore((s) => s.nodes)
  const select = useStore((s) => s.select)
  const node = selected ? nodes[selected] : null
  const open = !!selected

  return (
    <aside
      className={`shrink-0 border-l border-line bg-gradient-to-b from-panel to-panel2 flex flex-col overflow-hidden transition-[width] duration-200 ${
        open ? 'w-[min(360px,80vw)]' : 'w-0 border-l-0'
      }`}
      aria-hidden={!open}
    >
      <div className="flex items-center justify-between p-4 pb-0 whitespace-nowrap">
        <span className="font-disp text-[10px] tracking-[2.5px] uppercase text-muted">证据检视</span>
        <button onClick={() => select(null)} className="text-muted hover:text-alert text-base" aria-label="关闭">
          ✕
        </button>
      </div>
      <div className="text-sm text-signal my-2.5 px-4 break-all">{selected ?? '—'}</div>
      {node && (
        <div className="text-[11px] text-muted mb-3.5 px-4">
          {NODE_TYPE_LABELS[node.type] ?? node.type} ·{' '}
          <b
            className={`font-disp tracking-wider px-2 py-0.5 rounded-sm uppercase text-[10px] border ${
              node.state === 'confirmed' ? 'text-live border-live' : 'text-ghost border-ghost'
            }`}
          >
            {NODE_STATE_LABELS[node.state] ?? node.state}
          </b>
        </div>
      )}
      <div className="overflow-auto flex-1 px-4 pb-4">
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
            尚未坐实 — 该节点是假设, 还没有可回查的证据。需要一次独立验证动作才能升为已证实。
          </div>
        ) : (
          <div className="text-muted text-xs py-3.5 leading-relaxed">
            点击攻击图里的任一节点, 查看它如何被证实 — 逐字证据来自哪次工具输出。
          </div>
        )}
      </div>
    </aside>
  )
}
