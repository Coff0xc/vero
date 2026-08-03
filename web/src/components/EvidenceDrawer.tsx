import { useStore } from '../store'
import { NODE_STATE_LABELS } from '../lib/i18n'
import type { NodeState } from '../types'

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

// severity → Tailwind 静态类(徽标用)。
const SEV_BADGE: Record<string, string> = {
  critical: 'text-alert border-alert',
  high: 'text-[#ff9d5c] border-[#ff9d5c]',
  medium: 'text-signal border-signal',
  low: 'text-live border-live',
  info: 'text-ghost border-ghost',
}

// unix 秒 → 本地时间字符串(空/非法返回空串)。
function fmtTime(unix?: number): string {
  if (!unix || unix <= 0) return ''
  const d = new Date(unix * 1000)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

function StateBadge({ state }: { state: NodeState }) {
  const cls =
    state === 'confirmed'
      ? 'text-live border-live'
      : state === 'refuted'
        ? 'text-alert border-alert'
        : 'text-ghost border-ghost'
  return (
    <b className={`font-disp tracking-wider px-2 py-0.5 rounded-sm uppercase text-[10px] border ${cls}`}>
      {NODE_STATE_LABELS[state] ?? state}
    </b>
  )
}

// 证据抽屉: 作为右栏的兄弟列展开(flex 布局让位), 不再绝对定位盖住攻击图。
export function EvidenceDrawer() {
  const selected = useStore((s) => s.selected)
  const nodes = useStore((s) => s.nodes)
  const select = useStore((s) => s.select)
  const markRefuted = useStore((s) => s.markRefuted)
  const node = selected ? nodes[selected] : null
  const open = !!selected
  const refuted = node?.state === 'refuted'

  return (
    <aside
      className={`shrink-0 border-l border-line bg-gradient-to-b from-panel to-panel2 flex flex-col overflow-hidden transition-[width] duration-200 ${
        open ? 'w-[min(400px,80vw)]' : 'w-0 border-l-0'
      }`}
      aria-hidden={!open}
    >
      <div className="flex items-center justify-between p-4 pb-0 whitespace-nowrap">
        <span className="section-title font-disp text-[10px] uppercase text-muted">证据检视</span>
        <button
          onClick={() => select(null)}
          className="text-muted hover:text-alert hover:bg-alert/10 rounded-md w-6 h-6 flex items-center justify-center text-sm transition-colors"
          aria-label="关闭"
        >
          ✕
        </button>
      </div>
      <div className="text-[12.5px] font-mono text-signal my-2.5 px-4 break-all leading-relaxed">{selected ?? '—'}</div>
      {node && (
        <div className="text-[11px] text-muted mb-3.5 px-4">
          <span className="mr-1.5">{NODE_TYPE_LABELS[node.type] ?? node.type}</span>
          <StateBadge state={node.state} />
          {(node.severity || node.technique || node.tactic) && (
            <div className="flex flex-wrap gap-1.5 mt-2">
              {node.severity && (
                <span className={`text-[10px] uppercase border px-1.5 py-0.5 rounded-sm font-mono ${SEV_BADGE[node.severity] ?? 'text-muted border-line'}`}>
                  {node.severity}
                </span>
              )}
              {node.technique && (
                <span className="text-[10px] uppercase border border-signal/60 text-signal px-1.5 py-0.5 rounded-sm font-mono">
                  TTP {node.technique}
                </span>
              )}
              {node.tactic && (
                <span className="text-[10px] border border-ghost/60 text-ghost px-1.5 py-0.5 rounded-sm font-mono">{node.tactic}</span>
              )}
            </div>
          )}
          {refuted && (
            <div className="mt-2.5 border border-alert/40 bg-alert/5 text-alert text-[11px] px-2.5 py-2 rounded-sm leading-relaxed">
              该节点已被证伪 — 下列证据构成反驳链。
            </div>
          )}
        </div>
      )}
      <div className="overflow-auto flex-1 px-4 pb-4">
        {node && node.evidence.length > 0 ? (
          // 完整证据链: 不做条数截断, 每条展示 tool + excerpt + 捕获时间 + 置信度。
          node.evidence.map((ev, i) => (
            <div key={i} className={`my-2 border-l-2 rounded-r-sm ${refuted ? 'border-l-alert bg-alert/5' : 'border-l-live bg-live/5'}`}>
              <span className={`block font-disp text-[10px] tracking-wider uppercase px-2.5 pt-1.5 pb-0.5 ${refuted ? 'text-alert' : 'text-live'}`}>
                {ev.tool}
                {fmtTime(ev.at) && <span className="ml-2 normal-case tracking-normal text-muted">{fmtTime(ev.at)}</span>}
                {typeof ev.confidence === 'number' && (
                  <span className="ml-2 normal-case tracking-normal text-muted">置信 {Math.round(ev.confidence * 100)}%</span>
                )}
              </span>
              <pre className="m-0 px-2.5 pb-2 font-mono text-[11.5px] text-ink2 whitespace-pre-wrap break-all">{ev.excerpt}</pre>
            </div>
          ))
        ) : node ? (
          <div className={`border-l-2 pl-2.5 text-muted text-xs py-3.5 leading-relaxed ${refuted ? 'border-l-alert' : 'border-l-ghost'}`}>
            {refuted
              ? '该节点已证伪, 但未附反驳证据记录。'
              : '尚未坐实 — 该节点是假设, 还没有可回查的证据。需要一次独立验证动作才能升为已证实。'}
          </div>
        ) : (
          <div className="text-muted text-xs py-3.5 leading-relaxed">
            点击攻击图里的任一节点, 查看它如何被证实 — 逐字证据来自哪次工具输出。
          </div>
        )}
      </div>
      {node && (
        <div className="px-4 pb-4 shrink-0">
          <button
            onClick={() => markRefuted(node.id)}
            className={`w-full text-[10px] font-disp tracking-wider uppercase border rounded-sm px-2.5 py-1.5 transition-colors ${
              refuted
                ? 'border-alert/40 text-alert/70 cursor-not-allowed'
                : 'border-alert/50 text-alert hover:bg-alert/10'
            }`}
            disabled={refuted}
            title={refuted ? '该节点已是证伪状态' : '标记该节点为已证伪'}
          >
            {refuted ? '已证伪' : '标记证伪'}
          </button>
        </div>
      )}
    </aside>
  )
}
