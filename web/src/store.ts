import { create } from 'zustand'
import type { GraphNode, GraphEdge, HitlRequest, LogLine, Kpi, SSEEvent, EventKind, ConfigPublic, Evidence, NodeState, ChatMessage } from './types'
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
    case 'tool': return `${e.data.tool} ${e.data.success ? '成功' : '失败'} ${(e.data.stdout ?? '').replace(/\n/g, ' ↵ ')}`
    case 'graph': return `${e.data.confirm ? '已证实' : '待验证'} ${e.data.confirm ?? e.data.hypothesis ?? ''}`
    case 'edge': return `${e.data.src} —${e.data.rel}→ ${e.data.dst}`
    case 'hitl_request': return `需授权 L${e.data.level} ${e.data.tool}`
    case 'route': return `服务 ${(e.data.services ?? []).join(' · ') || '—'} → 激活 ${(e.data.activated ?? []).join(' · ') || '无'}`
    case 'summary': return `已证实 ${e.data.confirmed} · 待验证 ${e.data.hypothesis} · 证据违规 ${e.data.evidence_violations}`
    case 'done': return `战役结束: ${e.data.reason ?? ''}`
    case 'plan': return `计划 ${e.data.count} 步: ${e.data.rationale ?? ''}`
    case 'workflow_start': return `工作流启动: ${e.data.workflow} · 目标 ${e.data.target}`
    case 'workflow_stage': return `阶段: ${e.data.stage}${e.data.desc ? ` — ${e.data.desc}` : ''}`
    case 'workflow_complete': return `工作流完成: ${e.data.workflow}`
    case 'workflow_cancelled': return `工作流已取消: ${e.data.workflow}`
    case 'tool_result': return `${e.data.tool} ${e.data.success ? '成功' : '失败'} ${(e.data.stdout ?? '').replace(/\n/g, ' ↵ ')}`
    case 'tool_error': return `${e.data.tool} 失败: ${e.data.error}`
    case 'path': return `主路径: ${(e.data.nodes ?? []).length ? (e.data.nodes ?? []).join(' → ') : '—'}`
    case 'phase': return `阶段: ${e.data.phase}`
    case 'error': return `错误: ${e.data.msg}`
    case 'warning': return `⚠ 提示: ${e.data.msg}`
    case 'reflect': return `反思: ${(e.data as unknown as { text?: string }).text ?? ''}`
    case 'thinking': return `思考: ${(e.data as unknown as { text?: string }).text ?? ''}`
    case 'hitl':
      return `${e.data.approved ? '已放行' : '已拒绝'} ${e.data.action}`
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
    case 'phase':
      return { stage: e.data.phase }
    case 'reflect':
      return { rationale: (e.data as unknown as { text?: string }).text }
    case 'thinking':
      return { rationale: (e.data as unknown as { text?: string }).text }
    case 'hitl':
      return { tool: e.data.action, success: e.data.approved }
    default:
      return undefined
  }
}

// 战役阶段推断 —— 只前进不后退(待命→侦察→扫描→利用→完成)。
const STAGE_ORDER: Record<string, number> = { idle: 0, recon: 1, scan: 2, exploit: 3, done: 4 }
function advanceStage(cur: string, cand: string): string {
  return (STAGE_ORDER[cand] ?? 0) > (STAGE_ORDER[cur] ?? 0) ? cand : cur
}

// 节点状态等级: 只升不降(hypothesis→confirmed→refuted, 不接受降级)。
const STATE_RANK: Record<NodeState, number> = { hypothesis: 0, confirmed: 1, refuted: 2 }
function promoteState(cur: NodeState, next: NodeState): NodeState {
  return STATE_RANK[next] > STATE_RANK[cur] ? next : cur
}

// 证据合并 —— 后端 graph 事件携带的是该节点完整证据链, 按 tool+excerpt+at 去重追加,
// 避免整节点覆盖丢失旧证据, 也不因重放完整链而重复。
function mergeEvidence(old: Evidence[], incoming: Evidence[]): Evidence[] {
  const out = old.slice()
  const seen = new Set(out.map((ev) => `${ev.tool}\x00${ev.excerpt}\x00${ev.at ?? ''}`))
  for (const ev of incoming) {
    const k = `${ev.tool}\x00${ev.excerpt}\x00${ev.at ?? ''}`
    if (!seen.has(k)) {
      seen.add(k)
      out.push(ev)
    }
  }
  return out
}

// 主界面分区(图标轨导航)。
export type Tab = 'campaign' | 'tools' | 'workflows' | 'reports' | 'settings'

interface State {
  status: 'idle' | 'running' | 'done'
  goal: string
  log: LogLine[]
  messages: ChatMessage[] // 对话消息流(对话式 UI): 用户输入 + 事件渲染的助手消息
  nodes: Record<string, GraphNode>
  edges: Record<string, GraphEdge>
  kpi: Kpi
  hitl: HitlRequest | null
  selected: string | null
  engineLabel: string // 'engine' SSE 事件写入的真实生效引擎中文标签(如「DeepSeek 自主」)
  engineSel: string // 配置里的引擎枚举(auto|script|claude|deepseek)
  temperature: number // 思考强度(来自 /api/config)
  stage: string // 战役阶段推断(idle|recon|scan|exploit|done)
  mainPath: string[] // 主攻击路径节点 id(path SSE 事件写入)
  // ---- UI 壳状态(命令面板/导航/图全屏/节点过滤) ----
  activeTab: Tab
  paletteOpen: boolean // ⌘K 命令面板
  historyOpen: boolean // 历史战役抽屉
  graphFull: boolean // 攻击图全屏模式
  nodeQuery: string // 攻击图节点搜索词(空 = 不过滤)
  setTab: (t: Tab) => void
  togglePalette: (open?: boolean) => void
  toggleHistory: (open?: boolean) => void
  toggleGraphFull: (on?: boolean) => void
  setNodeQuery: (q: string) => void
  reset: (target: string) => void
  ingest: (e: SSEEvent) => void
  applyEvent: (s: State, e: SSEEvent) => Partial<State> // 单事件状态转移(纯函数, ingest/replay 共用)
  select: (id: string | null) => void
  clearHitl: () => void
  markRefuted: (id: string) => void
  setMainPath: (ids: string[]) => void
  applyConfig: (c: ConfigPublic) => void
  replay: (events: SSEEvent[]) => void // 历史会话回放
  // 对话智能: 直接插入/更新对话消息(问答回复、打字机效果)。
  pushMsg: (role: 'user' | 'assistant', kind: 'user' | 'chat', text: string) => number
  patchMsg: (id: number, text: string) => void
}

// 单一 store: 所有 UI 由 SSE 事件累积驱动(信号流/攻击图/KPI/证据/HITL)。
export const useStore = create<State>((set) => ({
  status: 'idle',
  goal: '—',
  log: [],
  messages: [],
  nodes: {},
  edges: {},
  kpi: emptyKpi,
  hitl: null,
  selected: null,
  engineLabel: '',
  engineSel: 'auto',
  temperature: 0.2,
  stage: 'idle',
  mainPath: [],
  activeTab: 'campaign',
  paletteOpen: false,
  historyOpen: false,
  graphFull: false,
  nodeQuery: '',
  setTab: (t) => set({ activeTab: t }),
  togglePalette: (open) => set((s) => ({ paletteOpen: open ?? !s.paletteOpen })),
  toggleHistory: (open) => set((s) => ({ historyOpen: open ?? !s.historyOpen })),
  toggleGraphFull: (on) => set((s) => ({ graphFull: on ?? !s.graphFull })),
  setNodeQuery: (q) => set({ nodeQuery: q }),

  reset: (target) =>
    set({
      status: 'running',
      goal: target,
      log: [],
      messages: [{ id: seq++, role: 'user', kind: 'user', text: target, ts: Date.now() }],
      nodes: {},
      edges: {},
      kpi: emptyKpi,
      hitl: null,
      selected: null,
      engineLabel: '',
      stage: 'idle',
      mainPath: [],
      activeTab: 'campaign',
      nodeQuery: '',
    }),

  // applyEvent —— 单事件状态转移(纯函数): ingest 与 replay 共用; replay 循环 reduce 后单次 set。
  applyEvent: (s: State, e: SSEEvent): Partial<State> => {
      const line: LogLine = { id: seq++, ts: Date.now(), kind: e.kind, text: fmt(e), meta: metaOf(e) }
      const nextLog = [...s.log, line]
      const nextMsg: ChatMessage = { id: seq++, role: 'assistant', kind: e.kind, text: line.text, meta: line.meta, ts: line.ts }
      // 对话消息也设上限(防长战役 OOM); 丢弃最旧但保留首条用户消息。
      let messages = [...s.messages, nextMsg]
      if (messages.length > LOG_LIMIT) {
        const drop = messages.length - LOG_LIMIT
        // 保底至少丢 1 条; 若首条是用户消息则从第 2 条起丢(保住"目标"上下文)。
        messages = messages[0].role === 'user' ? [messages[0], ...messages.slice(Math.max(1, drop))] : messages.slice(drop)
      }
      const patch: Partial<State> = { log: nextLog.length > LOG_LIMIT ? nextLog.slice(-LOG_LIMIT) : nextLog, messages }
      switch (e.kind) {
        case 'graph': {
          const id = e.data.confirm ?? e.data.hypothesis
          if (id) {
            const prev = s.nodes[id]
            const incoming: NodeState = e.data.state ?? (e.data.confirm ? 'confirmed' : 'hypothesis')
            const d = e.data
            patch.nodes = {
              ...s.nodes,
              [id]: {
                ...prev, // 保留旧字段(severity/createdAt 等), 避免整节点覆盖丢失
                id,
                type: d.type ?? prev?.type ?? 'node',
                state: promoteState(prev?.state ?? 'hypothesis', incoming), // 只升不降
                evidence: mergeEvidence(prev?.evidence ?? [], d.evidence ?? []), // 新证据追加去重
                ...(d.severity ? { severity: d.severity } : {}),
                ...(d.technique ? { technique: d.technique } : {}),
                ...(d.tactic ? { tactic: d.tactic } : {}),
                ...(typeof d.confidence === 'number' ? { confidence: d.confidence } : {}),
                ...(d.created_at ? { createdAt: d.created_at } : {}),
                ...(d.updated_at ? { updatedAt: d.updated_at } : {}),
              },
            }
          }
          break
        }
        case 'edge': {
          const eid = `${e.data.src}->${e.data.dst}:${e.data.rel}` // 含 rel: 同节点对的 runs/produces/verifies 不再互相覆盖
          patch.edges = { ...s.edges, [eid]: { id: eid, source: e.data.src, target: e.data.dst, rel: e.data.rel } }
          break
        }
        case 'path':
          patch.mainPath = e.data.nodes ?? []
          break
        case 'phase':
          patch.stage = advanceStage(s.stage, e.data.phase)
          break
        case 'route':
          patch.kpi = { ...s.kpi, services: e.data.services ?? [], activated: e.data.activated ?? [] }
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
  },

  // ingest —— 单事件入口(实时 SSE)。
  ingest: (e) => set((s) => ({ ...s, ...s.applyEvent(s, e) })),

  select: (id) => set({ selected: id }),
  clearHitl: () => set({ hitl: null }),

  markRefuted: (id) =>
    set((s) => {
      const n = s.nodes[id]
      if (!n) return {}
      return { nodes: { ...s.nodes, [id]: { ...n, state: 'refuted' } } }
    }),

  setMainPath: (ids) => set({ mainPath: ids }),

  // 历史会话回放: 按序重放某战役的事件流(对话 UI 点击左侧历史会话加载)。
  replay: (events: SSEEvent[]) => {
    set((s) => {
      let cur: State = {
        ...s, status: 'running', goal: s.goal, log: [], messages: [], nodes: {}, edges: {},
        kpi: emptyKpi, hitl: null, selected: null, stage: 'idle', mainPath: [],
      }
      for (const e of events) cur = { ...cur, ...s.applyEvent(cur, e) } // 批处理: 单次 set, 不逐事件渲染
      return { ...cur, status: 'done' }
    })
  },


  applyConfig: (c) =>
    set((s) => ({
      engineSel: c.engine,
      temperature: c.temperature,
      // 未发起战役时, 指示 chip 用配置的引擎名; 战役已选实际引擎则不覆盖。
      engineLabel: s.engineLabel || ENGINE_ZH[c.engine] || c.engine,
    })),

  pushMsg: (role, kind, text) => {
    const id = seq++
    useStore.setState((s) => ({ messages: [...s.messages, { id, role, kind, text, ts: Date.now() }] }))
    return id
  },
  patchMsg: (id, text) =>
    useStore.setState((s) => ({ messages: s.messages.map((m) => (m.id === id ? { ...m, text } : m)) })),
}))
