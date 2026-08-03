// 报告浏览器: 独立 tab, 自行拉取 /api/reports 列表(不再依赖 campaign 状态)。
// 字段对齐后端 server/reports.go 的 ListReports 响应结构。
import { useState, useEffect } from 'react'

interface Report {
  campaign_id: string
  target: string
  started_at: string
  duration_sec: number
  finding_count: number
  risk_score: number
}

const fmtTime = (s: string) => {
  try {
    return new Date(s).toLocaleString('zh-CN', { hour12: false })
  } catch {
    return s
  }
}

export function ReportViewer() {
  const [reports, setReports] = useState<Report[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')

  const [genBusy, setGenBusy] = useState<string | null>(null)

  // 点击生成报告(不再随战役自动生成)。
  const genReport = async (id: string) => {
    setGenBusy(id)
    try {
      const r = await fetch(`/api/campaigns/${encodeURIComponent(id)}/report`, { method: 'POST' })
      const body = (await r.json().catch(() => ({}))) as { ok?: boolean; file?: string; error?: string }
      if (!r.ok || !body.ok) throw new Error(body.error ?? `HTTP ${r.status}`)
      setError('')
      setGenBusy(null)
      setNotice(`报告已生成: ${body.file} (Markdown/HTML/JSON 可在线查看)`)
    } catch (err) {
      setGenBusy(null)
      setNotice(`生成失败: ${String(err)}`)
    }
  }

  const load = () => {
    setBusy(true)
    setError('')
    fetch('/api/reports')
      .then(async (r) => {
        const body = await r.json().catch(() => ({}))
        if (!r.ok) throw new Error(body.error ?? `HTTP ${r.status}`)
        setReports(body.reports ?? [])
      })
      .catch((e) => setError(String(e)))
      .finally(() => setBusy(false))
  }

  useEffect(load, [])

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-disp font-semibold tracking-wider text-ink2 uppercase">报告</h2>
        <button
          onClick={load}
          disabled={busy}
          className="px-3 py-1.5 text-[11px] font-disp tracking-wider uppercase rounded-sm border border-live text-live hover:bg-live hover:text-ink transition disabled:opacity-50"
        >
          {busy ? '刷新中…' : '刷新'}
        </button>
      </div>

      {error && <div className="text-xs text-alert">加载失败: {error}</div>}
      {notice && <div className="text-xs text-live">{notice}</div>}

      {reports.length === 0 && !error && (
        <div className="border border-dashed border-line rounded-sm p-8 text-center text-muted text-xs">
          暂无报告 — 完成一次战役后, JSON / Markdown / HTML 报告会出现在这里
        </div>
      )}

      <ul className="space-y-2">
        {reports.map((r) => (
          <li key={r.campaign_id} className="border border-line rounded-sm bg-panel px-4 py-3 flex items-center justify-between gap-4">
            <div className="min-w-0">
              <div className="font-mono text-xs text-ink2 truncate" title={r.campaign_id}>
                战役 #{r.campaign_id}
              </div>
              <div className="font-mono text-[11px] text-muted mt-0.5 truncate" title={r.target}>{r.target}</div>
              <div className="text-[11px] text-ghost mt-0.5">
                {r.finding_count} 项发现 · 风险 {r.risk_score} · 耗时 {r.duration_sec}s
              </div>
            </div>
            <div className="flex items-center gap-3 shrink-0">
              <span className="text-[11px] text-ghost">{fmtTime(r.started_at)}</span>
              <div className="flex gap-2.5 whitespace-nowrap items-center">
                <button
                  onClick={() => void genReport(r.campaign_id)}
                  className="text-[11px] px-2 py-0.5 rounded border border-signal text-signal hover:bg-signal/10 transition-colors"
                >
                  {genBusy === r.campaign_id ? '生成中…' : '生成报告'}
                </button>
                <a
                  href={`/api/campaigns/${encodeURIComponent(r.campaign_id)}/report.html`}
                  target="_blank"
                  rel="noreferrer"
                  className="text-[11px] text-signal hover:text-ink2 underline underline-offset-2"
                >
                  在线查看
                </a>
                <a
                  href={`/api/campaigns/${encodeURIComponent(r.campaign_id)}/report.json`}
                  target="_blank"
                  rel="noreferrer"
                  className="text-[11px] text-live hover:text-ink2 underline underline-offset-2"
                >
                  JSON
                </a>
                <a
                  href={`/api/campaigns/${encodeURIComponent(r.campaign_id)}/report.md`}
                  className="text-[11px] text-live hover:text-ink2 underline underline-offset-2"
                >
                  Markdown
                </a>
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}
