import { useCallback, useMemo, useState } from 'react'
import type { MouseEvent as ReactMouseEvent } from 'react'
import { ReactFlow, Background, MiniMap, type Node, type NodeProps, type Edge } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { useStore } from '../store'
import type { GraphEdge, NodeState } from '../types'

// 杀伤链阶段: 节点按 type 映射到列, 从左到右 = 攻击推进方向。
const STAGE: Record<string, number> = {
  host: 0, service: 0, finding: 1, web_shell: 2, cred: 3, claim: 3, foothold: 4,
}

// 主题色(与 tailwind token 同步, 珍珠白亮色): 琥珀=主路径/聚焦, 翡翠=已证实, 朱红=证伪/严重。
const C = {
  signal: '#a8781c',
  live: '#1a7f4b',
  alert: '#cf3825',
  warn: '#a8781c',
  high: '#d06a1f',
  ghost: '#8a8a80',
  edgeDim: '#d9d5c9',
  ink: '#ffffff',
  text: '#26241d',
  refutedText: '#a02215',
  bgRefuted: '#fbe9e5',
  bgMain: '#f6efdd',
  bgConfirmed: '#e3f3e9',
  bgHypo: '#f2f0e9',
}

// severity → 十六进制(画布导出用)。
const SEV_HEX: Record<string, string> = {
  critical: C.alert,
  high: C.high,
  medium: C.warn,
  low: C.live,
  info: C.ghost,
}
// severity → Tailwind 静态类(徽标用)。high 用橙(C.high 同步, 无独立 token)。
const SEV_STYLE: Record<string, string> = {
  critical: 'text-alert border-alert',
  high: 'text-[#d06a1f] border-[#d06a1f]',
  medium: 'text-warn border-warn',
  low: 'text-live border-live',
  info: 'text-ghost border-ghost',
}

function shortId(id: string): string {
  return id.length > 24 ? id.slice(0, 22) + '…' : id
}

// RFNodeData —— 自定义节点 data: 完整 GraphNode 字段 + 前端展示标志。
// extends Record<string, unknown> 以满足 @xyflow Node 的 data 约束。
interface RFNodeData extends Record<string, unknown> {
  id: string
  type: string
  state: NodeState
  evidence: { tool: string; excerpt: string; at?: number; confidence?: number }[]
  severity?: string
  technique?: string
  tactic?: string
  confidence?: number
  createdAt?: number
  updatedAt?: number
  onMainPath?: boolean
  dimmed?: boolean
  matched?: boolean // 搜索命中(青色高亮环)
}

function nodeColorFor(d: { state?: NodeState; onMainPath?: boolean }): string {
  if (d.state === 'refuted') return C.alert
  if (d.onMainPath) return C.signal
  if (d.state === 'confirmed') return C.live
  return C.ghost
}

// 自定义节点: 按 state 着色(confirmed→翡翠, hypothesis→灰, refuted→玫红+删除线),
// mainPath 节点青色高亮, 搜索命中加光环; 有 severity/technique 时追加徽标。
function AttackGraphNodeView({ data, selected }: NodeProps) {
  const d = data as unknown as RFNodeData
  const refuted = d.state === 'refuted'
  const border = refuted ? C.alert : d.matched ? C.signal : d.onMainPath ? C.signal : d.state === 'confirmed' ? C.live : C.ghost
  const bg = refuted ? C.bgRefuted : d.onMainPath ? C.bgMain : d.state === 'confirmed' ? C.bgConfirmed : C.bgHypo
  const glow = d.matched
    ? '0 0 0 2px rgba(168, 120, 28, 0.5)'
    : refuted
      ? '0 0 0 1.5px rgba(207, 56, 37, 0.4)'
      : d.onMainPath
        ? '0 0 0 1.5px rgba(168, 120, 28, 0.45)'
        : d.state === 'confirmed'
          ? '0 0 0 1px rgba(26, 127, 75, 0.3)'
          : 'none'
  return (
    <div
      className="rounded-md px-2 py-1.5 font-mono text-[11px] leading-snug"
      style={{
        background: bg,
        border: `${selected ? 2.5 : refuted || d.matched || d.onMainPath ? 2 : 1.5}px solid ${border}`,
        color: refuted ? C.refutedText : C.text,
        boxShadow: selected ? '0 0 0 3px rgba(168, 120, 28, 0.32)' : glow,
        width: '100%',
        transition: 'box-shadow .15s, border-color .15s',
      }}
    >
      <div
        className="whitespace-nowrap overflow-hidden text-ellipsis"
        style={{ textDecoration: refuted ? 'line-through' : 'none' }}
      >
        {shortId(d.id)}
      </div>
      {(d.severity || d.technique) && (
        <div className="flex flex-wrap gap-1 mt-1">
          {d.severity && (
            <span className={`text-[9px] uppercase border px-1 rounded ${SEV_STYLE[d.severity] ?? 'text-muted border-line'}`}>
              {d.severity}
            </span>
          )}
          {d.technique && (
            <span className="text-[9px] uppercase border px-1 rounded text-signal border-signal/60">
              {d.technique}
            </span>
          )}
        </div>
      )}
    </div>
  )
}

// nodeTypes 须在模块级定义(或 useMemo), 避免 ReactFlow 每次渲染重建导致节点重挂载。
const nodeTypes = { attack: AttackGraphNodeView }

// closureOf —— 以 start 为起点, 沿真实边(双向)做 BFS, 返回连通闭包。
function closureOf(start: string, edges: Record<string, GraphEdge>): Set<string> {
  const adj = new Map<string, string[]>()
  for (const e of Object.values(edges)) {
    if (!adj.has(e.source)) adj.set(e.source, [])
    if (!adj.has(e.target)) adj.set(e.target, [])
    adj.get(e.source)!.push(e.target)
    adj.get(e.target)!.push(e.source)
  }
  const seen = new Set<string>([start])
  const queue = [start]
  while (queue.length) {
    const cur = queue.shift()!
    for (const nb of adj.get(cur) ?? []) {
      if (!seen.has(nb)) {
        seen.add(nb)
        queue.push(nb)
      }
    }
  }
  return seen
}

// mainPathEdgeIdsOf —— 主路径相邻节点之间(任意方向)的真实边集合。
function mainPathEdgeIdsOf(mainPath: string[], edges: Record<string, GraphEdge>): Set<string> {
  const direct = new Map<string, GraphEdge>()
  for (const e of Object.values(edges)) direct.set(`${e.source}\x00${e.target}`, e)
  const out = new Set<string>()
  for (let i = 0; i < mainPath.length - 1; i++) {
    const a = mainPath[i]
    const b = mainPath[i + 1]
    const e = direct.get(`${a}\x00${b}`) ?? direct.get(`${b}\x00${a}`)
    if (e) out.add(e.id)
  }
  return out
}

// exportPNG —— 用 rfNodes/rfEdges(即 ReactFlow 渲染的那份)在离屏 canvas 绘制并下载。
function exportPNG(nodes: Node<RFNodeData>[], edges: Edge[], mainPathEdges: Set<string>) {
  if (nodes.length === 0) return
  const pad = 48
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
  for (const n of nodes) {
    const w = n.measured?.width ?? 168
    const h = n.measured?.height ?? 60
    minX = Math.min(minX, n.position.x)
    minY = Math.min(minY, n.position.y)
    maxX = Math.max(maxX, n.position.x + w)
    maxY = Math.max(maxY, n.position.y + h)
  }
  if (!Number.isFinite(minX)) return
  const W = Math.ceil(maxX - minX + pad * 2)
  const H = Math.ceil(maxY - minY + pad * 2)
  const scale = Math.max(0.2, Math.min(1, 2400 / W, 2400 / H))
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.ceil(W * scale))
  canvas.height = Math.max(1, Math.ceil(H * scale))
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.scale(scale, scale)
  ctx.fillStyle = C.ink
  ctx.fillRect(0, 0, W, H)
  ctx.translate(pad - minX, pad - minY)

  // 边
  for (const e of edges) {
    const s = nodes.find((n) => n.id === e.source)
    const t = nodes.find((n) => n.id === e.target)
    if (!s || !t) continue
    const sw = s.measured?.width ?? 168
    const sh = s.measured?.height ?? 60
    const tw = t.measured?.width ?? 168
    const th = t.measured?.height ?? 60
    const main = mainPathEdges.has(e.id)
    ctx.strokeStyle = main ? C.signal : C.edgeDim
    ctx.lineWidth = main ? 2.5 : 1.5
    ctx.beginPath()
    ctx.moveTo(s.position.x + sw / 2, s.position.y + sh / 2)
    ctx.lineTo(t.position.x + tw / 2, t.position.y + th / 2)
    ctx.stroke()
  }

  // 节点
  for (const n of nodes) {
    const d = n.data
    const w = n.measured?.width ?? 168
    const h = n.measured?.height ?? 60
    const x = n.position.x
    const y = n.position.y
    const refuted = d.state === 'refuted'
    const main = !!d.onMainPath
    const border = refuted ? C.alert : main ? C.signal : d.state === 'confirmed' ? C.live : C.ghost
    const bg = refuted ? C.bgRefuted : main ? C.bgMain : d.state === 'confirmed' ? C.bgConfirmed : C.bgHypo

    ctx.save()
    ctx.shadowColor = border
    ctx.shadowBlur = main || refuted || d.state === 'confirmed' ? 8 : 0
    roundRectPath(ctx, x, y, w, h, 6)
    ctx.fillStyle = bg
    ctx.fill()
    ctx.lineWidth = main ? 2.5 : 1.5
    ctx.strokeStyle = border
    ctx.stroke()
    ctx.restore()

    ctx.textBaseline = 'top'
    ctx.fillStyle = refuted ? C.refutedText : C.text
    ctx.font = '11px ui-monospace, Consolas, "Courier New", monospace'
    const label = shortId(d.id)
    ctx.fillText(label, x + 8, y + 7)
    if (refuted) {
      const tw = ctx.measureText(label).width
      ctx.strokeStyle = C.alert
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(x + 8, y + 16)
      ctx.lineTo(x + 8 + tw, y + 16)
      ctx.stroke()
    }

    // severity / technique 徽标
    if (d.severity || d.technique) {
      ctx.font = '9px ui-monospace, Consolas, "Courier New", monospace'
      let bx = x + 8
      const by = y + h - 16
      const badge = (text: string, color: string) => {
        const tw = ctx.measureText(text).width + 8
        roundRectPath(ctx, bx, by, tw, 12, 3)
        ctx.fillStyle = 'rgba(0,0,0,0.25)'
        ctx.fill()
        ctx.strokeStyle = color
        ctx.lineWidth = 1
        ctx.stroke()
        ctx.fillStyle = color
        ctx.fillText(text, bx + 4, by + 2)
        bx += tw + 4
      }
      if (d.severity) badge(d.severity.toUpperCase(), SEV_HEX[d.severity] ?? '#8a8a80')
      if (d.technique) badge(d.technique, C.signal)
    }
  }

  const a = document.createElement('a')
  a.download = `attack-graph-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.png`
  a.href = canvas.toDataURL('image/png')
  a.click()
}

function roundRectPath(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  const rr = Math.min(r, w / 2, h / 2)
  ctx.beginPath()
  ctx.moveTo(x + rr, y)
  ctx.arcTo(x + w, y, x + w, y + h, rr)
  ctx.arcTo(x + w, y + h, x, y + h, rr)
  ctx.arcTo(x, y + h, x, y, rr)
  ctx.arcTo(x, y, x + w, y, rr)
  ctx.closePath()
}

export function AttackGraph() {
  const nodes = useStore((s) => s.nodes)
  const edges = useStore((s) => s.edges)
  const mainPath = useStore((s) => s.mainPath)
  const select = useStore((s) => s.select)
  const nodeQuery = useStore((s) => s.nodeQuery)
  const setNodeQuery = useStore((s) => s.setNodeQuery)
  // focus: 连通闭包聚焦锚点(本地状态, 点击切换/取消)。
  const [focus, setFocus] = useState<string | null>(null)

  // 闭包外的节点降透明度到 0.25。
  const closure = useMemo(() => (focus ? closureOf(focus, edges) : null), [focus, edges])

  // 搜索命中集: id/severity/technique 含查询词(大小写不敏感)。
  const query = nodeQuery.trim().toLowerCase()
  const matched = useMemo(() => {
    if (!query) return null
    const set = new Set<string>()
    for (const n of Object.values(nodes)) {
      if (
        n.id.toLowerCase().includes(query) ||
        (n.severity ?? '').toLowerCase().includes(query) ||
        (n.technique ?? '').toLowerCase().includes(query) ||
        n.type.toLowerCase().includes(query)
      ) {
        set.add(n.id)
      }
    }
    return set
  }, [nodes, query])

  const mainPathSet = useMemo(() => new Set(mainPath), [mainPath])
  const mainPathEdgeIds = useMemo(() => mainPathEdgeIdsOf(mainPath, edges), [mainPath, edges])

  const { rfNodes, rfEdges } = useMemo(() => {
    const byStage: Record<number, string[]> = {}
    Object.values(nodes).forEach((n) => {
      const st = STAGE[n.type] ?? 1
      ;(byStage[st] ??= []).push(n.id)
    })
    const rfNodes: Node<RFNodeData>[] = Object.values(nodes).map((n) => {
      const st = STAGE[n.type] ?? 1
      const idx = byStage[st].indexOf(n.id)
      const hit = matched ? matched.has(n.id) : false
      // 搜索优先于闭包: 有查询时未命中即淡化; 无查询时走闭包聚焦。
      const dimmed = matched ? !hit : !!closure && !closure.has(n.id)
      return {
        id: n.id,
        type: 'attack',
        position: { x: st * 210 + 40, y: idx * 92 + 48 },
        data: {
          id: n.id,
          type: n.type,
          state: n.state,
          evidence: n.evidence,
          severity: n.severity,
          technique: n.technique,
          tactic: n.tactic,
          confidence: n.confidence,
          createdAt: n.createdAt,
          updatedAt: n.updatedAt,
          onMainPath: mainPathSet.has(n.id),
          dimmed,
          matched: matched ? hit : false,
        },
        connectable: false,
        style: {
          opacity: dimmed ? (matched ? 0.12 : 0.25) : 1,
          width: 168,
          // 只用 box-shadow 呼吸动画(不动 transform —— 修黑块: 节点 animation 里的
          // transform:scale 会覆盖 ReactFlow 的定位 transform, 导致渲染错位成黑块)。
          ...(n.state === 'confirmed' ? { animation: 'node-glow 2.4s ease-in-out infinite' } : {}),
        },
      }
    })
    const rfEdges: Edge[] = Object.values(edges).map((e) => {
      const main = mainPathEdgeIds.has(e.id)
      // 搜索时: 两端均命中的边保持亮色, 其余淡化。
      const bothHit = matched ? matched.has(e.source) && matched.has(e.target) : true
      return {
        id: e.id,
        source: e.source,
        target: e.target,
        animated: main,
        style: main
          ? { stroke: C.signal, strokeWidth: 2.5, opacity: bothHit ? 1 : 0.15 }
          : { stroke: C.edgeDim, opacity: bothHit ? 0.9 : 0.08 },
      }
    })
    return { rfNodes, rfEdges }
  }, [nodes, edges, mainPathSet, mainPathEdgeIds, closure, matched])

  const handleNodeClick = useCallback(
    (_: ReactMouseEvent, n: Node<RFNodeData>) => {
      select(n.id)
      // 再次点击同一节点取消聚焦(恢复全亮)。
      setFocus((f) => (f === n.id ? null : n.id))
    },
    [select],
  )

  const handlePaneClick = useCallback(() => {
    select(null)
    setFocus(null)
  }, [select])

  const handleExport = useCallback(() => {
    exportPNG(rfNodes, rfEdges, mainPathEdgeIds)
  }, [rfNodes, rfEdges, mainPathEdgeIds])

  return (
    <div className="absolute inset-0">
      {/* 搜索框: 命中高亮, 未命中淡化; 计数提示 */}
      <div className="absolute left-4 top-3 z-10 flex items-center gap-2">
        <div className="flex items-center gap-1.5 bg-white border border-line focus-within:border-signal/60 rounded-md px-2.5 py-1.5 shadow-card transition-colors">
          <input
            value={nodeQuery}
            onChange={(e) => setNodeQuery(e.target.value)}
            placeholder="搜索节点…"
            className="w-32 bg-transparent outline-none text-[11.5px] font-mono text-ink2 placeholder:text-ghost/90"
          />
          {nodeQuery && (
            <button onClick={() => setNodeQuery('')} className="text-[10.5px] text-ghost hover:text-alert" aria-label="清空搜索">
              清空
            </button>
          )}
        </div>
        {matched && (
          <span className="text-[10.5px] font-mono text-signal bg-white border border-signal/40 rounded-md px-2 py-1.5 shadow-card">
            {matched.size} 命中
          </span>
        )}
      </div>
      <div className="absolute left-4 top-11 z-10 text-[10px] text-ghost pointer-events-none mt-0.5">
        点击节点 → 聚焦连通闭包 / 检视证据
      </div>
      <button
        onClick={handleExport}
        className="absolute right-4 top-3 z-10 text-[11px] text-muted hover:text-signal border border-line bg-white px-2.5 py-1.5 rounded-md shadow-card transition-colors"
        title="导出当前攻击图为 PNG"
      >
        导出 PNG
      </button>
      <ReactFlow
        nodes={rfNodes}
        edges={rfEdges}
        nodeTypes={nodeTypes}
        fitView
        minZoom={0.3}
        onNodeClick={handleNodeClick}
        onPaneClick={handlePaneClick}
        nodesDraggable={false}
        nodesConnectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#eae7dd" gap={22} />
        <MiniMap
          position="bottom-left"
          pannable
          zoomable
          bgColor="#f4f2ec"
          maskColor="rgba(45, 41, 32, 0.10)"
          nodeColor={(n) => nodeColorFor((n.data as Partial<RFNodeData>) ?? {})}
        />
      </ReactFlow>
      <Legend />
    </div>
  )
}

function Legend() {
  return (
    <div className="absolute right-3.5 bottom-3 text-[10.5px] text-muted bg-white px-2.5 py-2 border border-line rounded-md shadow-card leading-loose">
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle bg-ghost" />
        假设, 未坐实
      </div>
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle bg-live" />
        已证实
      </div>
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle bg-alert" />
        已证伪
      </div>
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle bg-signal" />
        主攻击路径
      </div>
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle border-2 border-signal" />
        当前选中
      </div>
    </div>
  )
}
