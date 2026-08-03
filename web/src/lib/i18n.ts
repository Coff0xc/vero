// 全站中文文案映射 —— 集中管理, 展示层专用。
// 注意: EVENT_KINDS / kind 标识 / NodeState 枚举值(confirmed/hypothesis/refuted)必须保持英文,
// parseEvent 校验与 store switch 依赖英文值; 中文只出现在这里的 label 与 fmt() 文本里。

// 事件 kind → 中文徽标标签(信号流与各处复用)。
export const EVENT_LABELS: Record<string, string> = {
  engine: '引擎',
  step: '思考',
  tool: '工具',
  graph: '图更新',
  edge: '关联',
  hitl_request: '授权请求',
  route: '路由',
  summary: '摘要',
  done: '完成',
  plan: '计划',
  workflow_start: '工作流启动',
  workflow_stage: '工作流阶段',
  workflow_complete: '工作流完成',
  workflow_cancelled: '工作流取消',
  tool_result: '工具结果',
  tool_error: '工具错误',
  path: '路径',
  phase: '阶段',
  error: '错误',
  reflect: '反思',
  thinking: '思考',
}

// 工具级别 → 短标签(ToolManager 等复用)。
export const LEVEL_NAMES = ['L0-侦察', 'L1-扫描', 'L2-凭证', 'L3-利用', 'L4-破坏']

// 工具级别 → 中文口语化级别(HITL 弹窗「利用级」等)。
export const LEVEL_ZH = ['侦察级', '扫描级', '凭证级', '利用级', '破坏级']

// 攻击图节点状态 → 中文。
export const NODE_STATE_LABELS: Record<string, string> = {
  confirmed: '已证实',
  hypothesis: '待验证',
  refuted: '已证伪',
}

// 配置引擎枚举 → 中文说明。
export const ENGINE_ZH: Record<string, string> = {
  auto: '自动',
  script: '脚本',
  claude: 'Claude',
  deepseek: 'DeepSeek',
}

export const ENGINE_DESC: Record<string, string> = {
  auto: '自动(有 key 用真实模型, 否则脚本)',
  claude: 'Claude(需 ANTHROPIC_API_KEY)',
  deepseek: 'DeepSeek(需 DEEPSEEK_API_KEY)',
  script: '脚本(固定脚本, 无需 key)',
}

// 战役阶段: 待命 → 侦察 → 扫描 → 利用 → 完成。
export const STAGES = [
  { id: 'idle', label: '待命' },
  { id: 'recon', label: '侦察' },
  { id: 'scan', label: '扫描' },
  { id: 'exploit', label: '利用' },
  { id: 'done', label: '完成' },
] as const

export type StageId = (typeof STAGES)[number]['id']

export function stageLabel(id: string): string {
  return STAGES.find((s) => s.id === id)?.label ?? '待命'
}

// 工具级别 → 战役阶段(0=侦察, 1=扫描, 2+=利用)。
export function stageOfLevel(level: number): StageId {
  if (level <= 0) return 'recon'
  if (level === 1) return 'scan'
  return 'exploit'
}
