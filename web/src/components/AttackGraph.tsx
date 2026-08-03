import { useMemo } from 'react'
import { ReactFlow, Background, type Node, type Edge } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { useStore } from '../store'
import type { GraphNode } from '../types'

// 杀伤链阶段: 节点按 type 映射到列, 从左到右 = 攻击推进方向。
const STAGE: Record<string, number> = {
  host: 0, service: 0, finding: 1, web_shell: 2, cred: 3, claim: 3, foothold: 4,
}
const CHAIN = ['service', 'web_shell', 'cred', 'foothold']

function shortId(id: string): string {
  return id.length > 24 ? id.slice(0, 22) + '…' : id
}

function hostOf(id: string): string {
  const i = id.indexOf(':')
  const type = id.slice(0, i)
  const key = id.slice(i + 1)
  return type === 'service' ? key.split(':')[0] : key
}

// addKillChainEdges —— 按 host 分组, 连出 service→web_shell→cred→foothold 的推进主干(signal 色虚线)。
function addKillChainEdges(nodes: Record<string, GraphNode>, edges: Edge[]) {
  const byHost: Record<string, Record<string, string>> = {}
  Object.values(nodes).forEach((n) => {
    if (CHAIN.includes(n.type)) {
      const h = hostOf(n.id)
      ;(byHost[h] ??= {})[n.type] = n.id
    }
  })
  Object.values(byHost).forEach((chain) => {
    for (let i = 0; i < CHAIN.length - 1; i++) {
      const from = chain[CHAIN[i]]
      if (!from) continue
      for (let j = i + 1; j < CHAIN.length; j++) {
        const to = chain[CHAIN[j]]
        if (to) {
          edges.push({
            id: `kc:${from}->${to}`,
            source: from,
            target: to,
            animated: true,
            style: { stroke: '#e8b23a', strokeDasharray: '4 3' },
          })
          break
        }
      }
    }
  })
}

export function AttackGraph() {
  const nodes = useStore((s) => s.nodes)
  const edges = useStore((s) => s.edges)
  const select = useStore((s) => s.select)
  const selected = useStore((s) => s.selected)

  const { rfNodes, rfEdges } = useMemo(() => {
    const byStage: Record<number, string[]> = {}
    Object.values(nodes).forEach((n) => {
      const st = STAGE[n.type] ?? 1
      ;(byStage[st] ??= []).push(n.id)
    })
    const rfNodes: Node[] = Object.values(nodes).map((n) => {
      const st = STAGE[n.type] ?? 1
      const idx = byStage[st].indexOf(n.id)
      const confirmed = n.state === 'confirmed'
      const isSel = n.id === selected
      return {
        id: n.id,
        position: { x: st * 210 + 40, y: idx * 92 + 48 },
        data: { label: shortId(n.id) },
        connectable: false,
        // 选中态: 亮边框 + 光晕(修原版点击无任何视觉反馈, 不知道选中了啥)
        style: {
          background: confirmed ? '#0e2b27' : '#18222e',
          border: `${isSel ? 2.5 : confirmed ? 2 : 1}px solid ${
            isSel ? '#e8b23a' : confirmed ? '#4ec9b0' : '#5b6b7a'
          }`,
          borderRadius: 6,
          color: '#cdd6e0',
          fontSize: 11,
          fontFamily: 'ui-monospace, Consolas, monospace',
          width: 168,
          padding: 8,
          boxShadow: isSel
            ? '0 0 16px rgba(232,178,58,.5)'
            : confirmed
              ? '0 0 14px rgba(78,201,176,.35)'
              : 'none',
          transition: 'box-shadow .15s, border-color .15s',
        },
      }
    })
    const rfEdges: Edge[] = Object.values(edges).map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      style: { stroke: '#2a3a4a' },
    }))
    addKillChainEdges(nodes, rfEdges)
    return { rfNodes, rfEdges }
  }, [nodes, edges, selected])

  return (
    <div className="absolute inset-0">
      <div className="absolute left-4 top-3 z-10 font-disp text-[10px] tracking-[2.5px] uppercase text-muted pointer-events-none">
        攻击图
        <span className="block text-ghost lowercase tracking-normal mt-1 font-mono">点击节点 → 查看证据链</span>
      </div>
      <ReactFlow
        nodes={rfNodes}
        edges={rfEdges}
        fitView
        minZoom={0.3}
        onNodeClick={(_, n) => select(n.id)}
        onPaneClick={() => select(null)}
        nodesDraggable={false}
        nodesConnectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#1f2b38" gap={22} />
      </ReactFlow>
      <Legend />
    </div>
  )
}

function Legend() {
  return (
    <div className="absolute right-3.5 bottom-3 text-[10px] text-muted font-disp tracking-wider bg-ink/70 px-2.5 py-2 border border-line rounded-sm leading-loose">
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle bg-ghost" />
        假设, 未坐实
      </div>
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle bg-live" style={{ boxShadow: '0 0 6px #4ec9b0' }} />
        已证实
      </div>
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle bg-signal" />
        杀伤链推进
      </div>
      <div>
        <i className="inline-block w-2 h-2 rounded-full mr-1.5 align-middle" style={{ border: '2px solid #e8b23a' }} />
        当前选中
      </div>
    </div>
  )
}
