// Web 端工具管理组件
import { useState, useEffect } from 'react'

interface Tool {
  Name: string
  Level: number
  Desc: string
}

interface ToolStatus {
  name: string
  level: number
  available: boolean
  error?: string
  duration: number
  tested: boolean
}

export function ToolManager() {
  const [tools, setTools] = useState<Tool[]>([])
  const [verification, setVerification] = useState<ToolStatus[] | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetch('/api/tools')
      .then(r => r.json())
      .then(data => setTools(data.tools || []))
  }, [])

  const runVerification = () => {
    setLoading(true)
    fetch('/api/tools/verify', { method: 'POST' })
      .then(r => r.json())
      .then(data => {
        setVerification(data.results || [])
        setLoading(false)
      })
  }

  const levelColors = ['bg-blue-500', 'bg-green-500', 'bg-yellow-500', 'bg-orange-500', 'bg-red-500']
  const levelNames = ['L0-侦察', 'L1-扫描', 'L2-凭证', 'L3-利用', 'L4-破坏']

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">工具管理</h2>
        <button
          onClick={runVerification}
          disabled={loading}
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? '验证中...' : '验证工具'}
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {tools.map(tool => {
          const status = verification?.find(v => v.name === tool.Name)
          return (
            <div key={tool.Name} className="border rounded-lg p-4 space-y-2">
              <div className="flex items-center justify-between">
                <h3 className="font-semibold">{tool.Name}</h3>
                <span className={`px-2 py-1 text-xs rounded text-white ${levelColors[tool.Level]}`}>
                  {levelNames[tool.Level]}
                </span>
              </div>
              <p className="text-sm text-gray-600">{tool.Desc}</p>
              {status && (
                <div className="pt-2 border-t">
                  {status.available ? (
                    <span className="text-green-600 text-sm">✓ 可用</span>
                  ) : (
                    <div>
                      <span className="text-red-600 text-sm">✗ 不可用</span>
                      {status.error && (
                        <p className="text-xs text-gray-500 mt-1 truncate" title={status.error}>
                          {status.error}
                        </p>
                      )}
                    </div>
                  )}
                  <span className="text-xs text-gray-400 ml-2">
                    {status.duration / 1000000}ms
                  </span>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {verification && (
        <div className="bg-gray-100 rounded-lg p-4">
          <h3 className="font-semibold mb-2">验证摘要</h3>
          <div className="text-sm space-y-1">
            <p>总工具数: {verification.length}</p>
            <p>可用: {verification.filter(v => v.available).length}</p>
            <p>不可用: {verification.filter(v => !v.available).length}</p>
          </div>
        </div>
      )}
    </div>
  )
}
