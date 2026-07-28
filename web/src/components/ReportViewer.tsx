// Web 端报告增强组件：时间线和攻击路径
import { useState, useEffect } from 'react'

interface TimelineEvent {
  timestamp: string
  phase: string
  action: string
  description: string
  critical: boolean
}

interface PathNode {
  id: string
  type: string
  label: string
  critical: boolean
}

interface PathEdge {
  from: string
  to: string
  label: string
  method: string
}

interface ReportData {
  meta: {
    target: string
    generated_at: string
    duration_sec: number
  }
  executive: {
    total_findings: number
    critical_count: number
    high_count: number
    medium_count: number
    low_count: number
    risk_score: number
  }
  timeline?: {
    events: TimelineEvent[]
  }
  attack_path?: {
    nodes: PathNode[]
    edges: PathEdge[]
  }
}

export function ReportViewer({ campaignId }: { campaignId: string }) {
  const [report, setReport] = useState<ReportData | null>(null)
  const [activeView, setActiveView] = useState<'summary' | 'timeline' | 'graph'>('summary')

  useEffect(() => {
    fetch(`/api/campaigns/${campaignId}/report.json`)
      .then(r => r.json())
      .then(data => setReport(data))
  }, [campaignId])

  if (!report) return <div className="p-6">加载中...</div>

  const phaseColors: Record<string, string> = {
    reconnaissance: 'bg-blue-500',
    exploitation: 'bg-orange-500',
    'post-exploitation': 'bg-red-500',
  }

  const severityColors: Record<string, string> = {
    critical: 'bg-red-600',
    high: 'bg-orange-500',
    medium: 'bg-yellow-500',
    low: 'bg-blue-500',
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-2xl font-bold">渗透测试报告</h2>
          <p className="text-gray-600">目标: {report.meta.target}</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setActiveView('summary')}
            className={`px-4 py-2 rounded ${activeView === 'summary' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}
          >
            摘要
          </button>
          <button
            onClick={() => setActiveView('timeline')}
            className={`px-4 py-2 rounded ${activeView === 'timeline' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}
          >
            时间线
          </button>
          <button
            onClick={() => setActiveView('graph')}
            className={`px-4 py-2 rounded ${activeView === 'graph' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}
          >
            攻击路径
          </button>
        </div>
      </div>

      {/* Summary View */}
      {activeView === 'summary' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div className="bg-white rounded-lg shadow p-6">
            <h3 className="text-lg font-semibold mb-4">风险评分</h3>
            <div className="flex items-center justify-center">
              <div className="relative w-32 h-32">
                <div className="absolute inset-0 flex items-center justify-center">
                  <span className="text-4xl font-bold">{report.executive.risk_score.toFixed(1)}</span>
                </div>
                <svg className="w-32 h-32 transform -rotate-90">
                  <circle
                    cx="64"
                    cy="64"
                    r="56"
                    stroke="#e5e7eb"
                    strokeWidth="8"
                    fill="none"
                  />
                  <circle
                    cx="64"
                    cy="64"
                    r="56"
                    stroke={report.executive.risk_score > 7 ? '#dc2626' : report.executive.risk_score > 4 ? '#f59e0b' : '#10b981'}
                    strokeWidth="8"
                    fill="none"
                    strokeDasharray={`${(report.executive.risk_score / 10) * 351.86} 351.86`}
                  />
                </svg>
              </div>
            </div>
          </div>

          <div className="bg-white rounded-lg shadow p-6">
            <h3 className="text-lg font-semibold mb-4">发现统计</h3>
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <span className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-red-600"></span>
                  Critical
                </span>
                <span className="font-semibold">{report.executive.critical_count}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-orange-500"></span>
                  High
                </span>
                <span className="font-semibold">{report.executive.high_count}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-yellow-500"></span>
                  Medium
                </span>
                <span className="font-semibold">{report.executive.medium_count}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-blue-500"></span>
                  Low
                </span>
                <span className="font-semibold">{report.executive.low_count}</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Timeline View */}
      {activeView === 'timeline' && report.timeline && (
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold mb-4">攻击时间线</h3>
          <div className="space-y-4">
            {report.timeline.events.map((event, idx) => (
              <div key={idx} className="flex gap-4">
                <div className="flex flex-col items-center">
                  <div className={`w-4 h-4 rounded-full ${phaseColors[event.phase] || 'bg-gray-400'}`}></div>
                  {idx < report.timeline!.events.length - 1 && (
                    <div className="w-0.5 h-full bg-gray-300 mt-2"></div>
                  )}
                </div>
                <div className="flex-1 pb-4">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold">{event.action}</span>
                    {event.critical && (
                      <span className="px-2 py-0.5 text-xs bg-red-100 text-red-700 rounded">
                        关键
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-gray-600">{event.description}</p>
                  <span className="text-xs text-gray-400">{event.phase}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Attack Graph View */}
      {activeView === 'graph' && report.attack_path && (
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold mb-4">攻击路径图</h3>
          <div className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {report.attack_path.nodes.map(node => (
                <div
                  key={node.id}
                  className={`border rounded-lg p-4 ${node.critical ? 'border-red-500 bg-red-50' : 'border-gray-300'}`}
                >
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-sm font-semibold">{node.type}</span>
                    {node.critical && (
                      <span className="px-2 py-0.5 text-xs bg-red-600 text-white rounded">
                        CRITICAL
                      </span>
                    )}
                  </div>
                  <p className="text-sm">{node.label}</p>
                </div>
              ))}
            </div>

            <div className="mt-6">
              <h4 className="font-semibold mb-2">攻击链</h4>
              {report.attack_path.edges.map((edge, idx) => (
                <div key={idx} className="flex items-center gap-2 text-sm py-1">
                  <span className="font-mono text-xs bg-gray-100 px-2 py-1 rounded">
                    {edge.from.split(':').pop()}
                  </span>
                  <span className="text-gray-400">→</span>
                  <span className="text-xs text-gray-600">{edge.label}</span>
                  <span className="text-gray-400">→</span>
                  <span className="font-mono text-xs bg-gray-100 px-2 py-1 rounded">
                    {edge.to.split(':').pop()}
                  </span>
                  <span className="text-xs text-gray-500">via {edge.method}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
