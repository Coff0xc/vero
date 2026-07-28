// 与后端 core.Event 对齐的前端类型。

export type EventKind =
  | 'engine' | 'step' | 'tool' | 'graph' | 'edge' | 'hitl_request'
  | 'route' | 'summary' | 'done'

export interface SSEEvent {
  kind: EventKind
  data: Record<string, any>
}

export interface Evidence {
  tool: string
  excerpt: string
}

export type NodeState = 'confirmed' | 'hypothesis'

export interface GraphNode {
  id: string
  type: string
  state: NodeState
  evidence: Evidence[]
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
  args: Record<string, any>
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
}
