// Web 端工作流管理组件 —— 提供 target 输入(后端要求 target body 必填,
// 原版不发 body 导致执行必 400) + 暗色主题 + 执行错误提示。
import { useState, useEffect, type FormEvent } from 'react'

interface Stage {
  name: string
  description?: string
  tools: string[]
  sequential: boolean
  critical?: boolean
}

interface Workflow {
  id: string
  name: string
  description: string
  category?: string
  stages: Stage[]
  tags?: string[]
}

interface WFResult {
  summary: string
  duration: string
  error?: string
}

export function WorkflowManager() {
  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [error, setError] = useState('')
  const [running, setRunning] = useState<string | null>(null)
  const [result, setResult] = useState<WFResult | null>(null)
  const [target, setTarget] = useState('http://localhost:3000')

  useEffect(() => {
    fetch('/api/workflows')
      .then((r) => r.json())
      .then((data) => setWorkflows(data.workflows || []))
      .catch((e) => setError(String(e)))
  }, [])

  const execute = async (e: FormEvent, name: string) => {
    e.preventDefault()
    if (!target.trim()) {
      setResult({ summary: '需要目标地址', duration: '', error: 'target 不能为空' })
      return
    }
    setRunning(name)
    setResult(null)
    try {
      const r = await fetch(`/api/workflows/${encodeURIComponent(name)}/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: target.trim() }),
      })
      const body = await r.json().catch(() => ({}))
      if (!r.ok) {
        setResult({ summary: `执行失败 (HTTP ${r.status})`, duration: '', error: body.error ?? body.message ?? '未知错误' })
      } else {
        setResult({
          summary: `战役“${name}”已开始, 进度与审批请求请查看战役控制台`,
          duration: '',
          error: body.error,
        })
      }
    } catch (err) {
      setResult({ summary: '请求失败', duration: '', error: String(err) })
    } finally {
      setRunning(null)
    }
  }

  return (
    <div className="p-6 space-y-6">
      <h2 className="text-[17px] font-semibold text-ink2">工作流模板</h2>
      {error && <div className="text-xs text-alert">加载失败: {error}</div>}

      <div className="max-w-md">
        <label className="block text-[12px] font-medium text-muted mb-1.5">目标地址 *</label>
        <input
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          placeholder="http://host:port"
          spellCheck={false}
          className="w-full bg-white border border-line text-ink2 text-xs px-3 py-2 rounded-md font-mono outline-none focus:border-signal"
        />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {workflows.map((wf) => (
          <form key={wf.id} onSubmit={(e) => execute(e, wf.name)} className="border border-line rounded-lg bg-white p-4 space-y-3 shadow-card card-lift">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="font-mono text-sm text-ink2 break-all">{wf.name}</h3>
                <p className="text-xs text-muted mt-1 leading-relaxed">{wf.description}</p>
              </div>
              <button
                type="submit"
                disabled={running !== null}
                className="px-3 py-1.5 text-[12px] font-medium rounded-md border border-signal text-signal hover:bg-signal hover:text-white transition-colors disabled:opacity-50 whitespace-nowrap"
              >
                {running === wf.name ? '执行中…' : '执行'}
              </button>
            </div>
            <div className="space-y-1.5">
              {(wf.stages ?? []).map((st, i) => (
                <div key={i} className="border-l-2 border-l-ghost pl-2.5">
                  <div className="text-[11px] text-ghost font-medium">
                    {i + 1}. {st.name}
                    {!st.sequential ? <span className="ml-1.5 text-signal">(并行)</span> : null}
                  </div>
                  <div className="font-mono text-[11px] text-muted truncate" title={(st.tools ?? []).join(' · ')}>
                    {(st.tools ?? []).join(' · ')}
                  </div>
                </div>
              ))}
            </div>
          </form>
        ))}
      </div>

      {result && (
        <div className={`border rounded-md p-4 text-xs leading-relaxed ${result.error ? 'border-alert text-alert' : 'border-line text-live'}`}>
          <div className="text-[10.5px] font-medium mb-1">
            {result.error ? '执行失败' : '已受理'}
          </div>
          {result.error ?? result.summary}
          {result.duration && <span className="text-muted block mt-1">耗时 {result.duration}</span>}
        </div>
      )}
    </div>
  )
}
