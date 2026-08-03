// Web 端工具管理组件(与战役控制台统一暗色主题)
// 缺失工具分两类处理: installable=自动安装二进制, pip_hint=一键 pip 安装;
// 顶部「全部自动安装」调 POST /api/tools/install-all 批量安装, 逐条回填结果。
import { useState, useEffect } from 'react'
import { LEVEL_NAMES } from '../lib/i18n'
import type { InstallResult, InstallAllResponse } from '../types'

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
  installable?: string
  install_type?: string
  pip_hint?: string
}

const LEVEL_STYLE = [
  'bg-ghost/30 text-ghost',
  'bg-live/15 text-live',
  'bg-signal/15 text-signal',
  'bg-alert/20 text-alert',
  'bg-alert/40 text-alert',
]

export function ToolManager() {
  const [tools, setTools] = useState<Tool[]>([])
  const [verification, setVerification] = useState<ToolStatus[] | null>(null)
  const [loading, setLoading] = useState(false)
  const [installing, setInstalling] = useState<string | null>(null) // "name:type", 防并发
  const [installAllBusy, setInstallAllBusy] = useState(false)
  const [installResults, setInstallResults] = useState<Record<string, { ok: boolean; msg: string }>>({})
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  useEffect(() => {
    fetch('/api/tools')
      .then((r) => r.json())
      .then((data) => setTools(data.tools || []))
      .catch((e) => setError(String(e)))
  }, [])

  const runVerification = () => {
    setLoading(true)
    setError('')
    fetch('/api/tools/verify', { method: 'POST' })
      .then((r) => r.json())
      .then((data) => setVerification(data.results || []))
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false))
  }

  // 单工具安装: type=binary 默认(向后兼容), pip 需显式 {type:'pip'}。
  // feedbackKey = 工具卡片名(与 install-all 的 result.name 对齐), 与安装的二进制名可能不同。
  const install = async (name: string, type: 'binary' | 'pip', feedbackKey: string) => {
    const key = `${name}:${type}`
    setInstalling(key)
    setNotice('')
    try {
      const r = await fetch('/api/tools/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, type }),
      })
      const body = (await r.json().catch(() => ({}))) as Partial<InstallResult>
      if (!r.ok || body.ok === false) throw new Error(body.error ?? body.detail ?? `HTTP ${r.status}`)
      setInstallResults((m) => ({ ...m, [feedbackKey]: { ok: true, msg: body.path ? `已安装 → ${body.path}` : '已安装, 请重新验证' } }))
    } catch (err) {
      setInstallResults((m) => ({ ...m, [feedbackKey]: { ok: false, msg: `安装失败: ${String(err)}` } }))
    } finally {
      setInstalling(null)
      runVerification()
    }
  }

  // 全部自动安装: 一次请求批量安装 binary + pip 两类缺失工具。
  const installAll = async () => {
    setInstallAllBusy(true)
    setNotice('')
    setInstallResults({})
    try {
      const r = await fetch('/api/tools/install-all', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ types: ['binary', 'pip'] }),
      })
      const body = (await r.json().catch(() => ({}))) as Partial<InstallAllResponse>
      if (!r.ok) throw new Error(body.error ?? `HTTP ${r.status}`)
      const results = body.results ?? []
      const byName: Record<string, { ok: boolean; msg: string }> = {}
      results.forEach((res) => {
        byName[res.name] = res.ok
          ? { ok: true, msg: (res.type === 'pip' ? res.message : res.path) || '已安装' }
          : { ok: false, msg: res.error ?? res.detail ?? '未知错误' }
      })
      setInstallResults(byName)
      const ok = results.filter((x) => x.ok).length
      const fail = results.length - ok
      setNotice(
        `批量安装完成: 成功 ${ok} 项, 失败 ${fail} 项` + (fail > 0 ? '。失败详情见各工具卡片' : '。请重新验证'),
      )
    } catch (err) {
      setNotice(`批量安装失败: ${String(err)}`)
    } finally {
      setInstallAllBusy(false)
      runVerification()
    }
  }

  const unavailable = (verification ?? []).filter((v) => !v.available)
  const canInstallAll = unavailable.some((v) => v.installable || v.pip_hint)

  return (
    <div className="p-6 space-y-6">
      <div className="flex justify-between items-center flex-wrap gap-3">
        <h2 className="text-lg font-disp font-semibold tracking-wider text-ink2 uppercase">工具管理</h2>
        <div className="flex gap-2.5">
          <button
            onClick={installAll}
            disabled={loading || installAllBusy || !canInstallAll}
            title={canInstallAll ? '批量安装全部缺失工具(二进制 + pip)' : '暂无缺失工具可自动安装'}
            className="px-4 py-2 text-xs font-disp tracking-wider uppercase rounded-sm border border-signal text-signal hover:bg-signal hover:text-ink transition disabled:opacity-50"
          >
            {installAllBusy ? '安装中…' : '全部自动安装'}
          </button>
          <button
            onClick={runVerification}
            disabled={loading || installAllBusy}
            className="px-4 py-2 text-xs font-disp tracking-wider uppercase rounded-sm border border-live text-live hover:bg-live hover:text-ink transition disabled:opacity-50"
          >
            {loading ? '验证中…' : '验证工具'}
          </button>
        </div>
      </div>

      {error && <div className="text-xs text-alert">加载失败: {error}</div>}
      {notice && (
        <div className="text-xs text-live border border-live/40 bg-live/5 rounded-sm px-3 py-2">{notice}</div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {tools.map((tool) => {
          const status = verification?.find((v) => v.name === tool.Name)
          const feedback = installResults[tool.Name]
          const installingBinKey = status?.installable ? `${status.installable}:binary` : ''
          const installingPipKey = `${tool.Name}:pip`
          const busy = installing !== null || installAllBusy
          return (
            <div key={tool.Name} className="border border-line rounded-sm p-4 space-y-2 bg-panel">
              <div className="flex items-center justify-between gap-2">
                <h3 className="font-mono text-sm text-ink2 break-all">{tool.Name}</h3>
                <span className={`px-2 py-0.5 text-[10px] rounded-sm whitespace-nowrap ${LEVEL_STYLE[tool.Level] ?? LEVEL_STYLE[0]}`}>
                  {LEVEL_NAMES[tool.Level] ?? `L${tool.Level}`}
                </span>
              </div>
              <p className="text-xs text-muted leading-relaxed">{tool.Desc}</p>
              {status && (
                <div className="pt-2 border-t border-line">
                  {status.available ? (
                    <span className="text-live text-xs">✓ 可用</span>
                  ) : (
                    <div>
                      <span className="text-alert text-xs">✗ 不可用</span>
                      {status.error && (
                        <p className="text-[11px] text-muted mt-1 truncate" title={status.error}>
                          {status.error}
                        </p>
                      )}
                      {/* 自动安装按钮: installable=二进制, pip_hint=pip */}
                      <div className="mt-2 flex flex-wrap gap-2">
                        {status.installable && (
                          <button
                            onClick={() => install(status.installable!, 'binary', tool.Name)}
                            disabled={busy}
                            className="text-[11px] font-disp tracking-wider uppercase rounded-sm border border-live text-live px-2.5 py-1 hover:bg-live hover:text-ink transition disabled:opacity-50"
                          >
                            {installing === installingBinKey ? '安装中…' : `自动安装(二进制) ${status.installable}`}
                          </button>
                        )}
                        {status.pip_hint && (
                          <button
                            onClick={() => install(tool.Name, 'pip', tool.Name)}
                            disabled={busy}
                            title={status.pip_hint}
                            className="text-[11px] font-disp tracking-wider uppercase rounded-sm border border-signal text-signal px-2.5 py-1 hover:bg-signal hover:text-ink transition disabled:opacity-50"
                          >
                            {installing === installingPipKey ? '安装中…' : '一键安装(pip)'}
                          </button>
                        )}
                      </div>
                      {status.pip_hint && (
                        <p className="text-[11px] text-ghost mt-1.5">
                          安装命令: <code className="text-signal">{status.pip_hint}</code>
                        </p>
                      )}
                    </div>
                  )}
                  {feedback && (
                    <p className={`text-[11px] mt-1.5 ${feedback.ok ? 'text-live' : 'text-alert'}`}>
                      {feedback.ok ? '✓ ' : '✗ '}
                      {feedback.msg}
                    </p>
                  )}
                  <span className="text-[11px] text-ghost ml-2">{(status.duration / 1e6).toFixed(0)}ms</span>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {verification && (
        <div className="border border-line rounded-sm p-4 bg-panel">
          <h3 className="font-disp text-xs tracking-wider uppercase text-ink2 mb-2">验证摘要</h3>
          <div className="text-xs text-muted space-y-1">
            <p>总工具数: {verification.length}</p>
            <p className="text-live">可用: {verification.filter((v) => v.available).length}</p>
            <p className="text-alert">不可用: {verification.filter((v) => !v.available).length}</p>
          </div>
        </div>
      )}
    </div>
  )
}
