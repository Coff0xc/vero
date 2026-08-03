// 与后端 core.Event 对齐的前端类型。
// 修原版 data: Record<string, any> 的裸类型: SSE 数据改为判别联合,
// 拼写错误在编译期暴露(switch 收窄后字段类型即受校验)。

export type EventKind =
  | 'engine' | 'step' | 'tool' | 'graph' | 'edge' | 'hitl_request'
  | 'route' | 'summary' | 'done'
  | 'plan'
  | 'workflow_start' | 'workflow_stage' | 'workflow_complete' | 'workflow_cancelled'
  | 'tool_result' | 'tool_error'
  | 'path' | 'phase'
  | 'error' | 'reflect'

export interface EngineData { engine: string; target: string }
export interface StepData { step: number; tool: string; args: Record<string, unknown>; level: number; why?: string }
export interface ToolData { tool: string; success: boolean; stdout?: string; stderr?: string; level?: number }
export interface GraphData {
  confirm?: string
  hypothesis?: string
  type?: string
  state?: NodeState
  evidence?: Evidence[]
  severity?: string
  technique?: string
  tactic?: string
  confidence?: number
  created_at?: number
  updated_at?: number
}
export interface EdgeData { src: string; dst: string; rel: string }
export interface HitlData { key: string; tool: string; args: Record<string, unknown>; level: number; why?: string }
export interface RouteData { services: string[]; activated: string[] }
export interface SummaryData { confirmed: number; hypothesis: number; evidence_violations: number; report?: string }
export interface DoneData { reason?: string }
export interface PlanData { count: number; rationale: string }
export interface WorkflowStartData { workflow: string; target: string }
export interface WorkflowStageData { stage: string; desc?: string }
export interface WorkflowDoneData { workflow: string; target: string }
export interface ToolResultData { tool: string; success: boolean; stdout?: string; stderr?: string }
export interface ToolErrorData { tool: string; error: string }
export interface PathData { nodes: string[] } // 主路径: 连通节点 id 序列
export interface PhaseData { phase: string } // init/recon/scan/exploit/done
export interface ErrorData { msg: string }
export interface ReflectData { text: string }

// ChatMessage —— 对话消息(对话式 UI 的消息流): 用户输入 + 事件渲染成的助手消息。
export interface ChatMessage {
  id: number
  role: 'user' | 'assistant'
  kind: EventKind | 'user'
  text: string
  meta?: LogLine['meta']
  ts: number
}

// SSEEvent —— 判别联合: kind 决定 data 的确切形状。
export type SSEEvent =
  | { kind: 'engine'; data: EngineData }
  | { kind: 'step'; data: StepData }
  | { kind: 'tool'; data: ToolData }
  | { kind: 'graph'; data: GraphData }
  | { kind: 'edge'; data: EdgeData }
  | { kind: 'hitl_request'; data: HitlData }
  | { kind: 'route'; data: RouteData }
  | { kind: 'summary'; data: SummaryData }
  | { kind: 'done'; data: DoneData }
  | { kind: 'plan'; data: PlanData }
  | { kind: 'workflow_start'; data: WorkflowStartData }
  | { kind: 'workflow_stage'; data: WorkflowStageData }
  | { kind: 'workflow_complete'; data: WorkflowDoneData }
  | { kind: 'workflow_cancelled'; data: WorkflowDoneData }
  | { kind: 'tool_result'; data: ToolResultData }
  | { kind: 'tool_error'; data: ToolErrorData }
  | { kind: 'path'; data: PathData }
  | { kind: 'phase'; data: PhaseData }
  | { kind: 'error'; data: ErrorData }
  | { kind: 'reflect'; data: ReflectData }

export const EVENT_KINDS: readonly EventKind[] = [
  'engine', 'step', 'tool', 'graph', 'edge', 'hitl_request',
  'route', 'summary', 'done', 'plan',
  'workflow_start', 'workflow_stage', 'workflow_complete', 'workflow_cancelled',
  'tool_result', 'tool_error',
  'path', 'phase',
  'error',
  'reflect',
]

export interface Evidence {
  tool: string
  excerpt: string
  at?: number // 捕获时间(unix 秒)
  confidence?: number // 0..1
}

export type NodeState = 'confirmed' | 'hypothesis' | 'refuted'

export interface GraphNode {
  id: string
  type: string
  state: NodeState
  evidence: Evidence[]
  severity?: string // critical/high/medium/low/info
  technique?: string // MITRE ATT&CK technique ID(如 T1190)
  tactic?: string // MITRE ATT&CK tactic(如 initial-access)
  confidence?: number // 0..1
  createdAt?: number // unix 秒
  updatedAt?: number // unix 秒
}

export interface GraphEdge {
  id: string
  source: string
  target: string
  rel: string
}

export interface HitlRequest {
  key: string
  tool: string
  args: Record<string, unknown>
  level: number
  why: string
}

export interface Kpi {
  services: string[]
  activated: string[]
  confirmed: number
  hypothesis: number
  evidenceViolations: number
}

export interface LogLine {
  id: number
  kind: EventKind
  text: string
  ts: number // 事件到达时间戳(unix ms), 瀑布树用其计算工具/步骤耗时
  // 结构化字段: step.why / plan.rationale 等必须保留, 不能只压成一行文本。
  // SignalStream 据此做两行推理展示 / 计划高亮块; 缺省字段需渲染健壮。
  meta?: {
    tool?: string
    level?: number
    why?: string
    args?: Record<string, unknown>
    rationale?: string
    success?: boolean
    stdout?: string
    error?: string
    stage?: string
    count?: number
  }
}

// ---- 配置 API(GET/POST /api/config)----
// 密钥后端只回显布尔, 绝不明文。
export interface ConfigPublic {
  engine: string // auto|script|claude|deepseek
  model: string // 空 = 引擎默认
  temperature: number // 0~1
  max_budget: number
  has_anthropic: boolean
  has_deepseek: boolean
}

// POST /api/config 请求体 —— 仅提交变更字段; 空 key 字段 = 不改, 清空显式用 clear_*:true。
export interface ConfigPatch {
  engine?: string
  anthropic_key?: string
  deepseek_key?: string
  clear_anthropic?: boolean
  clear_deepseek?: boolean
  model?: string
  temperature?: number
  max_budget?: number
}

// ---- 工具安装 API(POST /api/tools/install[-all])----
export type InstallType = 'binary' | 'pip'

export interface InstallResult {
  name: string
  ok: boolean
  error?: string
  path?: string
  detail?: string
  message?: string
  type?: InstallType
}

export interface InstallAllResponse {
  ok: boolean
  total?: number
  installed?: number
  failed?: number
  results?: InstallResult[]
  error?: string
}
