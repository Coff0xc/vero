// 设置面板 —— 决策引擎 / API key / 模型 / 思考强度 / 决策预算。
// 密钥不回显明文: 后端只给 has_anthropic/has_deepseek 布尔; 前端据此显示「已配置/未配置」徽标。
// 清除语义: 空 key 字段 = 不改, 显式清空必须发 clear_anthropic/clear_deepseek:true。
import { useEffect, useState, type FormEvent } from 'react'
import { useStore } from '../store'
import type { ConfigPublic } from '../types'
import { ENGINE_ZH, ENGINE_DESC } from '../lib/i18n'
import { ProviderSection } from './ProviderSection'

interface KeyState {
  value: string // 新输入的 key(空 = 未改)
  cleared: boolean // 已点「清除」(待保存)
}

// 各引擎的合法模型选项(空值 = 引擎默认; 保留自定义值以兼容历史配置)。
const MODEL_OPTIONS: Record<string, { value: string; label: string }[]> = {
  claude: [
    { value: '', label: '默认 (claude-opus-4-8)' },
    { value: 'claude-opus-4-8', label: 'claude-opus-4-8' },
    { value: 'claude-sonnet-5', label: 'claude-sonnet-5' },
  ],
  deepseek: [
    { value: '', label: '默认 (deepseek-chat)' },
    { value: 'deepseek-chat', label: 'deepseek-chat' },
    { value: 'deepseek-reasoner', label: 'deepseek-reasoner' },
  ],
}

export function SettingsPanel() {
  const applyConfig = useStore((s) => s.applyConfig)

  const [engine, setEngine] = useState('auto')
  const [model, setModel] = useState('')
  const [temperature, setTemperature] = useState(0.2)
  const [maxBudget, setMaxBudget] = useState(10)
  const [hasAnthropic, setHasAnthropic] = useState(false)
  const [hasDeepSeek, setHasDeepSeek] = useState(false)
  const [anthropic, setAnthropic] = useState<KeyState>({ value: '', cleared: false })
  const [deepseek, setDeepseek] = useState<KeyState>({ value: '', cleared: false })
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState<{ ok: boolean; msg: string } | null>(null)

  const load = () => {
    setLoading(true)
    fetch('/api/config')
      .then(async (r) => {
        const body = (await r.json().catch(() => ({}))) as Partial<ConfigPublic> & { error?: string }
        if (!r.ok) throw new Error(body.error ?? `HTTP ${r.status}`)
        setEngine(body.engine ?? 'auto')
        setModel(body.model ?? '')
        setTemperature(body.temperature ?? 0.2)
        setMaxBudget(body.max_budget ?? 10)
        setHasAnthropic(!!body.has_anthropic)
        setHasDeepSeek(!!body.has_deepseek)
        setAnthropic({ value: '', cleared: false })
        setDeepseek({ value: '', cleared: false })
        applyConfig({
          engine: body.engine ?? 'auto',
          model: body.model ?? '',
          temperature: body.temperature ?? 0.2,
          max_budget: body.max_budget ?? 10,
          has_anthropic: !!body.has_anthropic,
          has_deepseek: !!body.has_deepseek,
          yolo: !!body.yolo,
          deep_thinking: !!body.deep_thinking,
          providers: body.providers ?? [],
          active_provider: body.active_provider ?? '',
        })
      })
      .catch((e) => setNotice({ ok: false, msg: `读取配置失败: ${String(e)}` }))
      .finally(() => setLoading(false))
  }

  // applyConfig 是 zustand 稳定引用, 加进 deps 不会导致重复拉取, 满足 exhaustive-deps。
  useEffect(load, [applyConfig])

  const submit = async (payload: Record<string, unknown>) => {
    setSaving(true)
    setNotice(null)
    try {
      const r = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      const body = (await r.json().catch(() => ({}))) as { ok?: boolean; config?: ConfigPublic; error?: string }
      if (!r.ok || !body.ok) throw new Error(body.error ?? `HTTP ${r.status}`)
      // 以响应 config 整体回填表单(权威值); 后端契约保证带 config, 缺省则回读本地值兜底。
      const c = body.config
      if (!c) throw new Error('保存成功但响应缺少 config')
      setEngine(c.engine)
      setModel(c.model)
      setTemperature(c.temperature)
      setMaxBudget(c.max_budget)
      setHasAnthropic(c.has_anthropic)
      setHasDeepSeek(c.has_deepseek)
      setAnthropic({ value: '', cleared: false })
      setDeepseek({ value: '', cleared: false })
      applyConfig(c)
      setNotice({ ok: true, msg: '配置已保存' })
    } catch (err) {
      setNotice({ ok: false, msg: `保存失败: ${String(err)}` })
    } finally {
      setSaving(false)
    }
  }

  // 只提交变更字段; 空 key = 不改。
  const onSave = (e: FormEvent) => {
    e.preventDefault()
    const patch: Record<string, unknown> = {}
    patch.engine = engine
    if (anthropic.cleared) patch.clear_anthropic = true
    else if (anthropic.value) patch.anthropic_key = anthropic.value
    if (deepseek.cleared) patch.clear_deepseek = true
    else if (deepseek.value) patch.deepseek_key = deepseek.value
    patch.model = model
    patch.temperature = temperature
    patch.max_budget = maxBudget
    submit(patch)
  }

  // 恢复默认: engine=auto / temp=0.2 / max_budget=10 / model=''(本地重置 + 提交)。
  const onRestore = () => {
    setEngine('auto')
    setModel('')
    setTemperature(0.2)
    setMaxBudget(10)
    setAnthropic({ value: '', cleared: false })
    setDeepseek({ value: '', cleared: false })
    submit({ engine: 'auto', temperature: 0.2, max_budget: 10 })
  }

  // 引擎回退提示: 选了具体模型但 key 缺失 → 发起战役会回退脚本模式。
  const fallbackHint = (() => {
    if (engine === 'auto') return '自动模式: 有 key 用真实模型, 否则回退脚本。'
    if (engine === 'claude' && !hasAnthropic) return '未配置 ANTHROPIC_API_KEY, 发起战役将回退脚本模式。'
    if (engine === 'deepseek' && !hasDeepSeek) return '未配置 DEEPSEEK_API_KEY, 发起战役将回退脚本模式。'
    return null
  })()

  // 模型下拉选项: 按引擎显示合法模型, 兼容历史自定义值。
  const modelOptions = (() => {
    const opts = MODEL_OPTIONS[engine] ?? MODEL_OPTIONS.claude
    const all = [...opts]
    if (model !== '' && !opts.some((o) => o.value === model)) all.push({ value: model, label: `${model} (自定义)` })
    return all
  })()

  const keyBadge = (configured: boolean, ks: KeyState) => {
    if (ks.cleared) return <span className="px-2 py-0.5 text-[10px] rounded border border-alert text-alert whitespace-nowrap">已清除(待保存)</span>
    if (ks.value) return <span className="px-2 py-0.5 text-[10px] rounded border border-signal text-signal whitespace-nowrap">待更新</span>
    return configured ? (
      <span className="px-2 py-0.5 text-[10px] rounded border border-live text-live whitespace-nowrap">已配置</span>
    ) : (
      <span className="px-2 py-0.5 text-[10px] rounded border border-ghost text-muted whitespace-nowrap">未配置</span>
    )
  }

  return (
    <form onSubmit={onSave} className="p-6 max-w-2xl space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-[17px] font-semibold text-ink2">设置</h2>
        <div className="flex gap-2.5">
          <button
            type="button"
            onClick={onRestore}
            disabled={saving || loading}
            className="px-3 py-1.5 text-[11.5px] font-medium rounded-md border border-muted text-muted hover:border-ink2 hover:text-ink2 transition-colors disabled:opacity-50"
          >
            恢复默认
          </button>
          <button
            type="submit"
            disabled={saving || loading}
            className="btn-accent px-4 py-1.5 text-[11.5px] rounded-md disabled:opacity-50"
          >
            {saving ? '保存中…' : '保存'}
          </button>
        </div>
      </div>

      {loading && <div className="text-xs text-muted">加载配置…</div>}
      {notice && (
        <div
          className={`text-xs border rounded-md px-3 py-2 ${
            notice.ok ? 'text-live border-live/40 bg-live/5' : 'text-alert border-alert/40 bg-alert/5'
          }`}
        >
          {notice.msg}
        </div>
      )}

      {/* 决策引擎 */}
      <section className="border border-line rounded-lg bg-panel p-4 shadow-inner-line space-y-3">
        <div className="section-title text-[11px] font-medium text-muted">决策引擎</div>
        <div className="flex items-center gap-3">
          <label className="w-16 text-xs text-ink2 shrink-0" htmlFor="cfg-engine">
            引擎
          </label>
          <select
            id="cfg-engine"
            value={engine}
            onChange={(e) => setEngine(e.target.value)}
            className="flex-1 bg-panel2 border border-line text-ink2 text-xs px-2.5 py-2 rounded-md outline-none focus:border-signal transition-colors"
          >
            <option value="auto">{ENGINE_ZH['auto']} — {ENGINE_DESC['auto']}</option>
            <option value="claude">{ENGINE_ZH['claude']} — {ENGINE_DESC['claude']}</option>
            <option value="deepseek">{ENGINE_ZH['deepseek']} — {ENGINE_DESC['deepseek']}</option>
            <option value="script">{ENGINE_ZH['script']} — {ENGINE_DESC['script']}</option>
          </select>
        </div>
        {fallbackHint && <div className="text-[11px] text-ghost">将回退脚本模式: {fallbackHint}</div>}
      </section>

      {/* API Key */}
      <section className="border border-line rounded-lg bg-panel p-4 shadow-inner-line space-y-4">
        <div className="section-title text-[11px] font-medium text-muted">API Key</div>
        <div className="space-y-1">
          <div className="flex items-center gap-2.5">
            <label className="w-16 text-xs text-ink2 shrink-0" htmlFor="cfg-ak">
              Anthropic
            </label>
            <input
              id="cfg-ak"
              type="password"
              value={anthropic.value}
              onChange={(e) => setAnthropic((s) => ({ ...s, value: e.target.value, cleared: false }))}
              placeholder={hasAnthropic ? '已配置 — 输入新值可替换' : '粘贴 ANTHROPIC_API_KEY'}
              autoComplete="new-password"
              spellCheck={false}
              className="flex-1 min-w-0 bg-panel2 border border-line text-ink2 text-xs px-2.5 py-2 rounded-md font-mono outline-none focus:border-signal transition-colors"
            />
            {keyBadge(hasAnthropic, anthropic)}
            <button
              type="button"
              onClick={() => setAnthropic({ value: '', cleared: true })}
              className="text-[11px] text-alert hover:text-ink2 whitespace-nowrap border border-line rounded px-2 py-1.5 hover:border-alert transition-colors"
            >
              清除
            </button>
          </div>
          <p className="text-[11px] text-muted pl-[76px]">密钥仅存本机文件(0600), 界面不回显明文</p>
        </div>
        <div className="space-y-1">
          <div className="flex items-center gap-2.5">
            <label className="w-16 text-xs text-ink2 shrink-0" htmlFor="cfg-dk">
              DeepSeek
            </label>
            <input
              id="cfg-dk"
              type="password"
              value={deepseek.value}
              onChange={(e) => setDeepseek((s) => ({ ...s, value: e.target.value, cleared: false }))}
              placeholder={hasDeepSeek ? '已配置 — 输入新值可替换' : '粘贴 DEEPSEEK_API_KEY'}
              autoComplete="new-password"
              spellCheck={false}
              className="flex-1 min-w-0 bg-panel2 border border-line text-ink2 text-xs px-2.5 py-2 rounded-md font-mono outline-none focus:border-signal transition-colors"
            />
            {keyBadge(hasDeepSeek, deepseek)}
            <button
              type="button"
              onClick={() => setDeepseek({ value: '', cleared: true })}
              className="text-[11px] text-alert hover:text-ink2 whitespace-nowrap border border-line rounded px-2 py-1.5 hover:border-alert transition-colors"
            >
              清除
            </button>
          </div>
          <p className="text-[11px] text-muted pl-[76px]">留空 = 不修改已配置的密钥(空串会被后端忽略)</p>
        </div>
      </section>

      {/* 模型 + 思考强度 */}
      <section className="border border-line rounded-lg bg-panel p-4 shadow-inner-line space-y-4">
        <div className="section-title text-[11px] font-medium text-muted">模型 + 思考强度</div>
        <div className="flex items-center gap-3">
          <label className="w-16 text-xs text-ink2 shrink-0" htmlFor="cfg-model">
            模型
          </label>
          <select
            id="cfg-model"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            className="flex-1 bg-panel2 border border-line text-ink2 text-xs px-2.5 py-2 rounded-md outline-none focus:border-signal transition-colors"
          >
            {modelOptions.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-3">
          <label className="w-16 text-xs text-ink2 shrink-0" htmlFor="cfg-temp">
            思考强度
          </label>
          <input
            id="cfg-temp"
            type="range"
            min={0}
            max={1}
            step={0.05}
            value={temperature}
            onChange={(e) => setTemperature(Number(e.target.value))}
            className="flex-1 accent-[#a8781c]"
          />
          <span className="font-mono text-sm text-signal w-12 text-right tabular-nums">{temperature.toFixed(2)}</span>
        </div>
        <div className="text-[11px] text-ghost pl-[76px]">低 = 稳健, 高 = 发散</div>
      </section>

      {/* 决策轮数上限 */}
      <section className="border border-line rounded-lg bg-panel p-4 shadow-inner-line space-y-3">
        <div className="section-title text-[11px] font-medium text-muted">决策预算</div>
        <div className="flex items-center gap-3">
          <label className="w-16 text-xs text-ink2 shrink-0" htmlFor="cfg-budget">
            决策轮数上限
          </label>
          <input
            id="cfg-budget"
            type="number"
            min={1}
            max={200}
            value={maxBudget}
            onChange={(e) => setMaxBudget(Math.max(1, Number(e.target.value) || 1))}
            className="w-28 bg-panel2 border border-line text-ink2 text-xs px-2.5 py-2 rounded-md font-mono outline-none focus:border-signal transition-colors"
          />
          <span className="text-[11px] text-muted">单次战役的决策迭代次数上限</span>
        </div>
      </section>

      <ProviderSection />
    </form>
  )
}
