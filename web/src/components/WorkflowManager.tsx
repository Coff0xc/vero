// Web 端工作流管理组件
import { useState, useEffect } from 'react'

interface Stage {
  name: string
  description: string
  tools: string[]
  sequential: boolean
  critical: boolean
}

interface Workflow {
  id: string
  name: string
  description: string
  category: string
  stages: Stage[]
  tags: string[]
}

export function WorkflowManager() {
  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [selectedWorkflow, setSelectedWorkflow] = useState<Workflow | null>(null)
  const [executing, setExecuting] = useState(false)

  useEffect(() => {
    fetch('/api/workflows')
      .then(r => r.json())
      .then(data => setWorkflows(data.workflows || []))
  }, [])

  const selectWorkflow = (id: string) => {
    fetch(`/api/workflows/${id}`)
      .then(r => r.json())
      .then(data => setSelectedWorkflow(data.workflow))
  }

  const executeWorkflow = (id: string) => {
    setExecuting(true)
    fetch(`/api/workflows/${id}/execute`, { method: 'POST' })
      .then(r => r.json())
      .then(data => {
        alert(`工作流执行状态: ${data.status}`)
        setExecuting(false)
      })
  }

  const categoryColors: Record<string, string> = {
    web: 'bg-blue-500',
    ad: 'bg-purple-500',
    cloud: 'bg-cyan-500',
    container: 'bg-green-500',
  }

  return (
    <div className="p-6 space-y-6">
      <h2 className="text-2xl font-bold">工作流模板</h2>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* 工作流列表 */}
        <div className="space-y-4">
          <h3 className="text-lg font-semibold">预定义模板</h3>
          {workflows.map(wf => (
            <div
              key={wf.id}
              className="border rounded-lg p-4 cursor-pointer hover:bg-gray-50"
              onClick={() => selectWorkflow(wf.id)}
            >
              <div className="flex items-center justify-between mb-2">
                <h4 className="font-semibold">{wf.name}</h4>
                <span className={`px-2 py-1 text-xs rounded text-white ${categoryColors[wf.category]}`}>
                  {wf.category}
                </span>
              </div>
              <p className="text-sm text-gray-600 mb-2">{wf.description}</p>
              <div className="flex gap-2">
                {wf.tags.map(tag => (
                  <span key={tag} className="px-2 py-1 text-xs bg-gray-200 rounded">
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>

        {/* 工作流详情 */}
        {selectedWorkflow && (
          <div className="border rounded-lg p-6 space-y-4">
            <div className="flex justify-between items-center">
              <h3 className="text-xl font-bold">{selectedWorkflow.name}</h3>
              <button
                onClick={() => executeWorkflow(selectedWorkflow.id)}
                disabled={executing}
                className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 disabled:opacity-50"
              >
                {executing ? '执行中...' : '执行工作流'}
              </button>
            </div>

            <p className="text-gray-600">{selectedWorkflow.description}</p>

            <div className="space-y-4">
              <h4 className="font-semibold">执行阶段</h4>
              {selectedWorkflow.stages.map((stage, idx) => (
                <div key={idx} className="border-l-4 border-blue-500 pl-4 py-2">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-semibold">{idx + 1}. {stage.name}</span>
                    {stage.critical && (
                      <span className="px-2 py-0.5 text-xs bg-red-100 text-red-700 rounded">
                        关键
                      </span>
                    )}
                    {stage.sequential ? (
                      <span className="px-2 py-0.5 text-xs bg-yellow-100 text-yellow-700 rounded">
                        顺序
                      </span>
                    ) : (
                      <span className="px-2 py-0.5 text-xs bg-green-100 text-green-700 rounded">
                        并行
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-gray-600 mb-2">{stage.description}</p>
                  <div className="flex flex-wrap gap-2">
                    {stage.tools.map(tool => (
                      <span key={tool} className="px-2 py-1 text-xs bg-gray-100 rounded font-mono">
                        {tool}
                      </span>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
