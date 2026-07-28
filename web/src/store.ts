import { create } from 'zustand'
import type { GraphNode, GraphEdge, HitlRequest, LogLine, Kpi, SSEEvent } from './types'

const emptyKpi: Kpi = { services: [], activated: [], confirmed: 0, hypothesis: 0, evidenceViolations: 0 }

let seq = 0

// fmt —— 把一个事件渲染成信号流里的一行文本。
function fmt(e: SSEEvent): string {
  const d = e.data
  switch (e.kind) {
    case 'engine': return `引擎 ${d.engine} · 目标 ${d.target}`
    case 'step': return `L${d.level} ${d.tool} → ${d.why ?? ''}`
    case 'tool': return `${d.tool} ${d.success ? '✓' : '✗'} ${(d.stdout ?? '').replace(/\n/g, ' ↵ ')}`
    case 'graph': return `${d.confirm ? '✓ confirmed' : '○ hypothesis'} ${d.confirm ?? d.hypothesis ?? ''}`
    case 'edge': return `${d.src} —${d.rel}→ ${d.dst}`
    case 'hitl_request': return `⚠ 需授权 L${d.level} ${d.tool}`
    case 'route': return `服务 ${(d.services ?? []).join(' · ') || '—'} → 激活 ${(d.activated ?? []).join(' · ') || '无'}`
    case 'summary': return `已证实 ${d.confirmed} · 待验证 ${d.hypothesis} · 证据违规 ${d.evidence_violations}`
    case 'done': return `战役结束: ${d.reason ?? ''}`
    default: return JSON.stringify(d)
  }
}

interface State {
  status: 'idle' | 'running' | 'done'
  goal: string
  log: LogLine[]
  nodes: Record<string, GraphNode>
  edges: Record<string, GraphEdge>
  kpi: Kpi
  hitl: HitlRequest | null
  selected: string | null
  reset: (target: string) => void
  ingest: (e: SSEEvent) => void
  select: (id: string | null) => void
  clearHitl: () => void
}

// 单一 store: 所有 UI 由 SSE 事件累积驱动(信号流/攻击图/KPI/证据/HITL)。
export const useStore = create<State>((set) => ({
  status: 'idle',
  goal: '—',
  log: [],
  nodes: {},
  edges: {},
  kpi: emptyKpi,
  hitl: null,
  selected: null,

  reset: (target) =>
    set({ status: 'running', goal: target, log: [], nodes: {}, edges: {}, kpi: emptyKpi, hitl: null, selected: null }),

  ingest: (e) =>
    set((s) => {
      const patch: Partial<State> = { log: [...s.log, { id: seq++, kind: e.kind, text: fmt(e) }] }
      const d = e.data
      switch (e.kind) {
        case 'graph': {
          const id = d.confirm ?? d.hypothesis
          if (id) {
            patch.nodes = {
              ...s.nodes,
              [id]: { id, type: d.type ?? 'node', state: d.confirm ? 'confirmed' : 'hypothesis', evidence: d.evidence ?? [] },
            }
          }
          break
        }
        case 'edge': {
          const eid = `${d.src}->${d.dst}`
          patch.edges = { ...s.edges, [eid]: { id: eid, source: d.src, target: d.dst, rel: d.rel } }
          break
        }
        case 'route':
          patch.kpi = { ...s.kpi, services: d.services ?? [], activated: d.activated ?? [] }
          break
        case 'summary':
          patch.kpi = { ...s.kpi, confirmed: d.confirmed ?? 0, hypothesis: d.hypothesis ?? 0, evidenceViolations: d.evidence_violations ?? 0 }
          break
        case 'hitl_request':
          patch.hitl = { key: d.key, tool: d.tool, args: d.args ?? {}, level: d.level, why: d.why ?? '' }
          break
        case 'done':
          patch.status = 'done'
          break
      }
      return patch
    }),

  select: (id) => set({ selected: id }),
  clearHitl: () => set({ hitl: null }),
}))
