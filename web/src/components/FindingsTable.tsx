import { useMemo, useState } from 'react'
import { useStore } from '../store'
import { NODE_STATE_LABELS } from '../lib/i18n'
import type { GraphNode, NodeState } from '../types'

// 发现节点表 —— 从攻击图节点里筛 type==='finding', 可排序 / 按严重度过滤。
// 颜色沿用全局色板: 严重度 alert 红 / 高 signal 黄 / 中 low live 青。

// severity → 排序权重(未知 = 0)。
const SEV_WEIGHT: Record<string, number> = { critical: 5, high: 4, medium: 3, low: 2, info: 1 }

// severity → 徽标 Tailwind 静态类(避免 JIT 漏扫动态类名)。
const SEV_BADGE: Record<string, string> = {
  critical: 'text-alert border-alert',
  high: 'text-[#ff9d5c] border-[#ff9d5c]',
  medium: 'text-signal border-signal',
  low: 'text-live border-live',
  info: 'text-ghost border-ghost',
}

// severity → 中文展示名(全中文文案)。
const SEV_ZH: Record<string, string> = {
  critical: '严重',
  high: '高危',
  medium: '中危',
  low: '低危',
  info: '信息',
}

const SEV_ORDER = ['critical', 'high', 'medium', 'low', 'info'] as const

// 节点状态排序权重。
const STATE_RANK: Record<NodeState, number> = { hypothesis: 0, confirmed: 1, refuted: 2 }

// 无 severity 字段时, 从节点 label 兜底推断([severity] 前缀 / 行首 severity 词)。
function inferSeverity(label: string): string | undefined {
  const m = label.match(/\[(critical|high|medium|low|info)\]/i)
  if (m) return m[1].toLowerCase()
  const m2 = label.match(/^\s*(critical|high|medium|low|info)\b/i)
  if (m2) return m2[1].toLowerCase()
  return undefined
}

// 去掉节点 label 的 [severity] 前缀(仅展示, 不污染原数据)。
function cleanLabel(label: string): string {
  const cleaned = label.replace(/^\s*\[[^\]]*\]\s*/, '').trim()
  return cleaned || label
}

function sevOf(n: GraphNode): string | undefined {
  return n.severity ?? inferSeverity(n.id)
}

// unix 秒 → 本地时间字符串(空/非法返回 '—')。
function fmtTime(unix?: number): string {
  if (!unix || unix <= 0) return '—'
  const d = new Date(unix * 1000)
  const p = (x: number) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

function StateBadge({ state }: { state: NodeState }) {
  const cls =
    state === 'confirmed'
      ? 'text-live border-live'
      : state === 'refuted'
        ? 'text-alert border-alert'
        : 'text-ghost border-ghost'
  return (
    <span className={`font-disp tracking-wider px-1.5 py-0.5 rounded-sm uppercase text-[10px] border whitespace-nowrap ${cls}`}>
      {NODE_STATE_LABELS[state] ?? state}
    </span>
  )
}

type SortKey = 'severity' | 'state' | 'label' | 'tool' | 'time'

const COLUMNS: { key: SortKey; label: string }[] = [
  { key: 'severity', label: '严重度' },
  { key: 'state', label: '状态' },
  { key: 'label', label: '节点' },
  { key: 'tool', label: '来源工具' },
  { key: 'time', label: '发现时间' },
]

interface Row {
  id: string
  sev?: string
  state: NodeState
  label: string
  tool: string
  createdAt?: number
}

export function FindingsTable() {
  const nodes = useStore((s) => s.nodes)
  const select = useStore((s) => s.select)
  const [collapsed, setCollapsed] = useState(false)
  const [sevFilter, setSevFilter] = useState<string>('all')
  const [sortKey, setSortKey] = useState<SortKey>('severity')
  const [asc, setAsc] = useState(false)

  // 筛选 + 排序: 全量派生, 状态变化即时生效。
  const rows = useMemo<Row[]>(() => {
    const out = Object.values(nodes)
      .filter((n) => n.type === 'finding')
      .map((n) => ({
        id: n.id,
        sev: sevOf(n),
        state: n.state,
        label: cleanLabel(n.id),
        tool: n.evidence[0]?.tool ?? '—',
        createdAt: n.createdAt,
      }))
      .filter((r) => sevFilter === 'all' || r.sev === sevFilter)
    out.sort((a, b) => {
      let cmp = 0
      switch (sortKey) {
        case 'severity':
          cmp = (SEV_WEIGHT[a.sev ?? ''] ?? 0) - (SEV_WEIGHT[b.sev ?? ''] ?? 0)
          break
        case 'state':
          cmp = STATE_RANK[a.state] - STATE_RANK[b.state]
          break
        case 'label':
          cmp = a.label.localeCompare(b.label, 'zh-Hans-CN')
          break
        case 'tool':
          cmp = a.tool.localeCompare(b.tool, 'zh-Hans-CN')
          break
        case 'time':
          cmp = (a.createdAt ?? 0) - (b.createdAt ?? 0)
          break
      }
      return asc ? cmp : -cmp
    })
    return out
  }, [nodes, sevFilter, sortKey, asc])

  const onSort = (key: SortKey) => {
    if (key === sortKey) {
      setAsc((v) => !v)
    } else {
      setSortKey(key)
      setAsc(key === 'severity' || key === 'time' ? false : true)
    }
  }

  return (
    <div className="shrink-0 border-t border-line bg-panel2">
      <div className="flex items-center gap-3 px-4 py-2">
        <button
          onClick={() => setCollapsed((c) => !c)}
          className="flex-1 flex items-center gap-2 font-disp text-[10px] tracking-[2.5px] uppercase text-muted hover:text-ink2 transition-colors text-left"
          title={collapsed ? '展开发现列表' : '折叠发现列表'}
        >
          <span className={`inline-block transition-transform ${collapsed ? '-rotate-90' : ''}`}>▾</span>
          发现 <span className="text-signal">({rows.length})</span>
        </button>
        <label className="text-[10px] font-disp tracking-wider text-muted">严重度</label>
        <select
          value={sevFilter}
          onChange={(e) => setSevFilter(e.target.value)}
          className="bg-panel border border-line text-ink2 text-[10.5px] px-2 py-1 rounded-sm outline-none focus:border-signal font-disp"
        >
          <option value="all">全部</option>
          {SEV_ORDER.map((s) => (
            <option key={s} value={s}>
              {SEV_ZH[s]}
            </option>
          ))}
        </select>
      </div>

      {!collapsed && (
        <div className="max-h-64 overflow-auto border-t border-line">
          <table className="w-full text-left text-[11.5px] border-collapse">
            <thead className="sticky top-0 z-10 bg-panel">
              <tr>
                {COLUMNS.map((c) => (
                  <th
                    key={c.key}
                    onClick={() => onSort(c.key)}
                    className="px-3 py-1.5 font-disp text-[10px] tracking-wider uppercase text-muted hover:text-signal cursor-pointer select-none whitespace-nowrap"
                  >
                    {c.label}
                    {sortKey === c.key && <span className="ml-1 text-signal">{asc ? '▲' : '▼'}</span>}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-3 py-4 text-center text-muted text-xs">
                    暂无发现
                  </td>
                </tr>
              ) : (
                rows.map((r) => (
                  <tr
                    key={r.id}
                    onClick={() => select(r.id)}
                    className="border-t border-line/60 cursor-pointer hover:bg-white/[0.04] transition-colors"
                    title={`点击定位到攻击图: ${r.id}`}
                  >
                    <td className="px-3 py-1.5">
                      {r.sev ? (
                        <span className={`font-disp text-[10px] uppercase border px-1.5 py-0.5 rounded-sm font-mono whitespace-nowrap ${SEV_BADGE[r.sev] ?? 'text-muted border-line'}`}>
                          {SEV_ZH[r.sev] ?? r.sev}
                        </span>
                      ) : (
                        <span className="text-ghost">—</span>
                      )}
                    </td>
                    <td className="px-3 py-1.5">
                      <StateBadge state={r.state} />
                    </td>
                    <td className="px-3 py-1.5 text-ink2 max-w-[220px] truncate">{r.label}</td>
                    <td className="px-3 py-1.5 font-mono text-muted text-[11px] whitespace-nowrap">{r.tool}</td>
                    <td className="px-3 py-1.5 text-ghost text-[11px] whitespace-nowrap">{fmtTime(r.createdAt)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
