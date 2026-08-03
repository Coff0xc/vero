import { create } from 'zustand'
import type { GraphNode, GraphEdge, HitlRequest, LogLine, Kpi, SSEEvent, EventKind, ConfigPublic } from './types'
import { EVENT_KINDS } from './types'
import { stageOfLevel, ENGINE_ZH } from './lib/i18n'

const emptyKpi: Kpi = { services: [], activated: [], confirmed: 0, hypothesis: 0, evidenceViolations: 0 }

// 日志上限: 超过则丢弃最旧, 防止无限增长(全量复制数组 + 万条后卡顿/OOM)。
const LOG_LIMIT = 500

let seq = 0

// parseEvent —— 校验并收窄任意 JSON 为类型安全的 SSEEvent; 未知/畸形事件直接丢弃。
export function parseEvent(raw: unknown): SSEEvent | null {
  if (typeof raw !== 'object' || raw === null) return null
  const e = raw as { kind?: unknown; data?: unknown }
  if (typeof e.kind !== 'string' || !(EVENT_KINDS as readonly string[]).includes(e.kind)) return null
  const data = (typeof e.data === 'object' && e.data !== null ? e.data : {}) as Record<string, unknown>
  return { kind: e.kind as EventKind, data } as SSEEvent
}

// fmt —— 把一个事件渲染成信号流里的一行简洁文本(全中文)。
// 结构化字段(why/rationale 等)另存到 LogLine.meta, 供 SignalStream 做两行推理/计划块展示。
function fmt(e: SSEEvent): string {
  switch (e.kind) {
    case 'engine': return `引擎: ${e.data.engine} · 目标: ${e.data.target}`
    case 'step': return `思考 L${e.data.level} ${e.data.tool}: ${e.data.why ?? ''}`
    case 'tool': return `${e.data.tool} ${e.data.success ? '✓ 成功' : '✗ 失败'} ${(e.data.stdout ?? '').replace(/\n/g, ' ↵ ')}`
    case 'graph': return `${e.data.confirm ? '✓ 已证实' : '○ 待验证'} ${e.data.confirm ?? e.data.hypothesis ?? ''}`
    case 'edge': return `${e.data.src} —${e.data.rel}→ ${e.data.dst}`
    case 'hitl_request': return `⚠ 需授权 L${e.data.level} ${e.data.tool}`
    case 'route': return `服务 ${e.data.services.join(' · ') || '—'} → 激活 ${e.data.activated.join(' · ') || '无'}`
    case 'summary': return `已证实 ${e.data.confirmed} · 待验证 ${e.data.hypothesis} · 证据违规 ${e.data.evidence_violations}`
    case 'done': return `战役结束: ${e.data.reason ?? ''}`
    case 'plan': return `计划 ${e.data.count} 步: ${e.data.rationale ?? ''}`
    case 'workflow_start': return `工作流启动: ${e.data.workflow} · 目标 ${e.data.target}`
    case 'workflow_stage': return `阶段: ${e.data.stage}${e.data.desc ? ` — ${e.data.desc}` : ''}`
    case 'workflow_complete': return `工作流完成: ${e.data.workflow}`
    case 'workflow_cancelled': return `工作流已取消: ${e.data.workflow}`
    case 'tool_result': return `${e.data.tool} ${e.data.success ? '✓ 成功' : '✗ 失败'} ${(e.data.stdout ?? '').replace(/\n/g, ' ↵ ')}`
    case 'tool_error': return `${e.data.tool} ✗ ${e.data.error}`
  }
}

// metaOf —— 把事件的结构化字段写进 LogLine.meta(step.why / plan.rationale 等必须保留)。
function metaOf(e: SSEEvent): LogLine['meta'] {
  switch (e.kind) {
    case 'step':
      return { tool: e.data.tool, level: e.data.level, why: e.data.why, args: e.data.args }
    case 'tool':
      return { tool: e.data.tool, level: e.data.level, success: e.data.success, stdout: e.data.stdout }
    case 'plan':
      return { count: e.data.count, rationale: e.data.rationale }
    case 'hitl_request':
      return { tool: e.data.tool, level: e.data.level, why: e.data.why, args: e.data.args }
    case 'tool_result':
      return { tool: e.data.tool, success: e.data.success, stdout: e.data.stdout }
    case 'tool_error':
      return { tool: e.data.tool, error: e.data.error }
    case 'workflow_stage':
      return { stage: e.data.stage }
    default:
      return undefined
  }
}

// 战役阶段推断 —— 只前进不后退(待命→侦察→扫描→利用→完成)。
const STAGE_ORDER: Record<string, number> = { idle: 0, recon: 1, scan: 2, exploit: 3, done: 4 }
function advanceStage(cur: string, cand: string): string {
  return (STAGE_ORDER[cand] ?? 0) > (STAGE_ORDER[cur] ?? 0) ? cand : cur
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
  engineLabel: string // 'engine' SSE 事件写入的真实生效引擎中文标签(如「DeepSeek 自主」)
  engineSel: string // 配置里的引擎枚举(auto|script|claude|deepseek)
  temperature: number // 思考强度(来自 /api/config)
  stage: string // 战役阶段推断(idle|recon|scan|exploit|done)
  reset: (target: string) => void
  ingest: (e: SSEEvent) => void
  select: (id: string | null) => void
  clearHitl: () => void
  applyConfig: (c: ConfigPublic) => void
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
  engineLabel: '',
  engineSel: 'auto',
  temperature: 0.2,
  stage: 'idle',

  reset: (target) =>
    set({
      status: 'running',
      goal: target,
      log: [],
      nodes: {},
      edges: {},
      kpi: emptyKpi,
      hitl: null,
      selected: null,
      engineLabel: '',
      stage: 'idle',
    }),

  ingest: (e) =>
    set((s) => {
      const nextLog = [...s.log, { id: seq++, kind: e.kind, text: fmt(e), meta: metaOf(e) }]
      const patch: Partial<State> = { log: nextLog.length > LOG_LIMIT ? nextLog.slice(-LOG_LIMIT) : nextLog }
      switch (e.kind) {
        case 'graph': {
          const id = e.data.confirm ?? e.data.hypothesis
          if (id) {
            patch.nodes = {
              ...s.nodes,
              [id]: { id, type: e.data.type ?? 'node', state: e.data.confirm ? 'confirmed' : 'hypothesis', evidence: e.data.evidence ?? [] },
            }
          }
          break
        }
        case 'edge': {
          const eid = `${e.data.src}->${e.data.dst}`
          patch.edges = { ...s.edges, [eid]: { id: eid, source: e.data.src, target: e.data.dst, rel: e.data.rel } }
          break
        }
        case 'route':
          patch.kpi = { ...s.kpi, services: e.data.services, activated: e.data.activated }
          patch.stage = advanceStage(s.stage, 'recon')
          break
        case 'summary':
          patch.kpi = { ...s.kpi, confirmed: e.data.confirmed, hypothesis: e.data.hypothesis, evidenceViolations: e.data.evidence_violations }
          patch.stage = advanceStage(s.stage, 'done')
          break
        case 'hitl_request':
          patch.hitl = { key: e.data.key, tool: e.data.tool, args: e.data.args, level: e.data.level, why: e.data.why ?? '' }
          break
        case 'done':
          patch.status = 'done'
          patch.stage = 'done'
          break
        case 'engine':
          patch.engineLabel = e.data.engine
          patch.stage = advanceStage(s.stage, 'recon')
          break
        case 'step':
        case 'tool':
          patch.stage = advanceStage(s.stage, stageOfLevel(e.data.level ?? 0))
          break
      }
      return patch
    }),

  select: (id) => set({ selected: id }),
  clearHitl: () => set({ hitl: null }),

  applyConfig: (c) =>
    set((s) => ({
      engineSel: c.engine,
      temperature: c.temperature,
      // 未发起战役时, 指示 chip 用配置的引擎名; 战役已选实际引擎则不覆盖。
      engineLabel: s.engineLabel || ENGINE_ZH[c.engine] || c.engine,
    })),
}))
