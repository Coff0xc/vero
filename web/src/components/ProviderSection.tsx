// 多提供商管理(OpenAI 兼容): URL+Key -> 测试连接 -> 自动拉模型 + YOLO/深度思考开关。
import { useEffect, useState } from 'react'
import type { ConfigPublic, ProviderPreset } from '../types'

interface LocalProvider {
  id: string
  name: string
  base_url: string
  api_key: string // 新输入(空 = 保留原值)
  model: string
  supports_reasoning: boolean
  web_search: boolean
}

export function ProviderSection() {
  const [providers, setProviders] = useState<LocalProvider[]>([])
  const [presets, setPresets] = useState<ProviderPreset[]>([])
  const [activeId, setActiveId] = useState('')
  const [yolo, setYolo] = useState(false)
  const [deepThinking, setDeepThinking] = useState(false)
  const [models, setModels] = useState<Record<string, string[]>>({})
  const [testing, setTesting] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState<{ ok: boolean; msg: string } | null>(null)

  const load = () => {
    fetch('/api/config')
      .then(async (r) => {
        const c = (await r.json()) as ConfigPublic
        setPresets(c.presets ?? [])
        setActiveId(c.active_provider ?? '')
        setYolo(!!c.yolo)
        setDeepThinking(!!c.deep_thinking)
        setProviders(
          (c.providers ?? []).map((p) => ({
            id: p.id,
            name: p.name,
            base_url: p.base_url,
            api_key: '',
            model: p.model ?? '',
            supports_reasoning: !!p.supports_reasoning,
            web_search: !!p.web_search,
          })),
        )
      })
      .catch((e) => setNotice({ ok: false, msg: `读取提供商失败: ${String(e)}` }))
  }
  useEffect(load, [])

  const testConn = async (p: LocalProvider) => {
    setTesting(p.id)
    setNotice(null)
    try {
      const r = await fetch('/api/providers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: p.id, base_url: p.base_url, api_key: p.api_key }),
      })
      const body = (await r.json()) as { ok?: boolean; models?: string[]; error?: string }
      if (!body.ok) throw new Error(body.error ?? '连接失败')
      setModels((m) => ({ ...m, [p.id]: body.models ?? [] }))
      setNotice({ ok: true, msg: `连接成功, 拉取到 ${(body.models ?? []).length} 个模型` })
    } catch (err) {
      setNotice({ ok: false, msg: `测试失败: ${String(err)}` })
    } finally {
      setTesting(null)
    }
  }

  const save = async () => {
    setSaving(true)
    setNotice(null)
    try {
      const r = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          providers: providers.map((p) => ({
            id: p.id,
            name: p.name,
            base_url: p.base_url,
            api_key: p.api_key,
            model: p.model,
            supports_reasoning: p.supports_reasoning,
            web_search: p.web_search,
          })),
          active_provider: activeId,
          yolo,
          deep_thinking: deepThinking,
        }),
      })
      const body = (await r.json().catch(() => ({}))) as { ok?: boolean; error?: string }
      if (!r.ok || !body.ok) throw new Error(body.error ?? `HTTP ${r.status}`)
      setNotice({ ok: true, msg: '提供商配置已保存' })
      setProviders((ps) => ps.map((p) => ({ ...p, api_key: '' })))
    } catch (err) {
      setNotice({ ok: false, msg: `保存失败: ${String(err)}` })
    } finally {
      setSaving(false)
    }
  }

  const patch = (id: string, fn: (p: LocalProvider) => LocalProvider) =>
    setProviders((ps) => ps.map((p) => (p.id === id ? fn(p) : p)))

  return (
    <section className="border border-line rounded-sm bg-panel p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div className="font-disp text-[10px] tracking-[2.5px] uppercase text-muted">模型提供商 (OpenAI 兼容)</div>
        <div className="flex gap-3 text-[11px]">
          <label className="flex items-center gap-1.5 text-muted cursor-pointer">
            <input type="checkbox" checked={yolo} onChange={(e) => setYolo(e.target.checked)} className="accent-[#e8b23a]" />
            <span className={yolo ? 'text-alert font-semibold' : ''}>YOLO 免审批</span>
          </label>
          <label className="flex items-center gap-1.5 text-muted cursor-pointer">
            <input type="checkbox" checked={deepThinking} onChange={(e) => setDeepThinking(e.target.checked)} className="accent-[#e8b23a]" />
            <span className={deepThinking ? 'text-live font-semibold' : ''}>深度思考</span>
          </label>
        </div>
      </div>

      {yolo && (
        <div className="text-[11px] text-alert border border-alert/40 bg-alert/8 rounded px-2.5 py-1.5">
          ⚠ YOLO 模式已开启: 所有攻击动作(L3/L4)自动放行, 不再请求人工审批 — 仅用于授权靶场/自动化。
        </div>
      )}

      <div className="flex flex-wrap gap-1.5">
        {presets.map((pr) => (
          <button
            key={pr.id}
            onClick={() =>
              setProviders((ps) => ps.map((p) => (p.id === pr.id ? { ...p, base_url: pr.base_url, name: pr.name } : p)))
            }
            className="px-2 py-1 text-[10px] font-mono rounded border border-line text-muted hover:text-live hover:border-live/40 transition-colors"
            title={`填入 ${pr.base_url}`}
          >
            {pr.name}
          </button>
        ))}
      </div>

      <div className="space-y-2.5">
        {providers.map((p) => (
          <div key={p.id} className="border border-line/60 rounded p-3 space-y-2 bg-ink/30">
            <div className="flex items-center gap-2">
              <button
                onClick={() => setActiveId(p.id)}
                className={`flex items-center gap-1.5 text-[12px] font-mono px-2 py-0.5 rounded border transition-colors ${
                  activeId === p.id ? 'border-live text-live bg-live/10' : 'border-line text-muted hover:text-ink2'
                }`}
              >
                {activeId === p.id ? '● 生效中' : '○ 设为生效'}
              </button>
              <span className="text-[12px] text-ink2 font-mono">{p.name}</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              <label className="block">
                <span className="text-[10px] text-ghost">Base URL</span>
                <input
                  value={p.base_url}
                  onChange={(e) => patch(p.id, (x) => ({ ...x, base_url: e.target.value }))}
                  className="w-full bg-panel2 border border-line text-ink2 text-[11px] px-2 py-1.5 rounded-sm font-mono outline-none focus:border-signal"
                  placeholder="https://api.example.com/v1"
                />
              </label>
              <label className="block">
                <span className="text-[10px] text-ghost">API Key</span>
                <div className="flex gap-1.5">
                  <input
                    type="password"
                    value={p.api_key}
                    onChange={(e) => patch(p.id, (x) => ({ ...x, api_key: e.target.value }))}
                    className="flex-1 bg-panel2 border border-line text-ink2 text-[11px] px-2 py-1.5 rounded-sm font-mono outline-none focus:border-signal"
                    placeholder={p.id.includes('ollama') ? '(本地无需 key)' : 'sk-... (留空 = 保留原值)'}
                  />
                  <button
                    onClick={() => void testConn(p)}
                    disabled={testing === p.id}
                    className="px-2.5 py-1 text-[10px] font-disp uppercase tracking-wider rounded border border-signal text-signal hover:bg-signal/10 disabled:opacity-40 shrink-0"
                  >
                    {testing === p.id ? '…' : '测试连接'}
                  </button>
                </div>
              </label>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <label className="flex-1 min-w-[180px]">
                <span className="text-[10px] text-ghost">模型</span>
                <input
                  list={`models-${p.id}`}
                  value={p.model}
                  onChange={(e) => patch(p.id, (x) => ({ ...x, model: e.target.value }))}
                  className="w-full bg-panel2 border border-line text-ink2 text-[11px] px-2 py-1.5 rounded-sm font-mono outline-none focus:border-signal"
                  placeholder="测试连接后自动拉取, 或手动输入"
                />
                <datalist id={`models-${p.id}`}>
                  {(models[p.id] ?? []).map((m) => (
                    <option key={m} value={m} />
                  ))}
                </datalist>
              </label>
              <label className="flex items-center gap-1.5 text-[11px] text-muted cursor-pointer shrink-0">
                <input
                  type="checkbox"
                  checked={p.supports_reasoning}
                  onChange={(e) => patch(p.id, (x) => ({ ...x, supports_reasoning: e.target.checked }))}
                  className="accent-[#e8b23a]"
                />
                深度思考
              </label>
              <label className="flex items-center gap-1.5 text-[11px] text-muted cursor-pointer shrink-0">
                <input
                  type="checkbox"
                  checked={p.web_search}
                  onChange={(e) => patch(p.id, (x) => ({ ...x, web_search: e.target.checked }))}
                  className="accent-[#e8b23a]"
                />
                联网搜索
              </label>
            </div>
          </div>
        ))}
      </div>

      {notice && (
        <div className={`text-[11px] rounded px-2.5 py-1.5 border ${notice.ok ? 'text-live border-live/40 bg-live/5' : 'text-alert border-alert/40 bg-alert/5'}`}>
          {notice.msg}
        </div>
      )}

      <div className="flex justify-end">
        <button
          onClick={() => void save()}
          disabled={saving}
          className="px-4 py-1.5 text-[11px] font-disp tracking-wider uppercase rounded-sm border border-live text-live hover:bg-live hover:text-ink transition disabled:opacity-50"
        >
          {saving ? '保存中…' : '保存提供商配置'}
        </button>
      </div>
    </section>
  )
}
