import { useEffect, useMemo, useRef, useState } from 'react'
import { useStore } from '../store'
import { LEVEL_ZH, EVENT_LABELS } from '../lib/i18n'
import type { ChatMessage } from '../types'

// 阶段 → 中文 + 颜色。
const PHASE_ZH: Record<string, string> = {
  idle: '待命',
  init: '初始化',
  recon: '侦察',
  scan: '扫描',
  exploit: '利用',
  done: '完成',
}
const PHASE_COLOR: Record<string, string> = {
  idle: 'text-ghost border-ghost',
  init: 'text-ghost border-ghost',
  recon: 'text-signal border-signal',
  scan: 'text-live border-live',
  exploit: 'text-alert border-alert',
  done: 'text-live border-live',
}

// severity → 徽章样式。
const SEV_BADGE: Record<string, string> = {
  critical: 'bg-[#ff5c4d]/15 text-[#ff5c4d] border-[#ff5c4d]/40',
  high: 'bg-[#ff9d5c]/15 text-[#ff9d5c] border-[#ff9d5c]/40',
  medium: 'bg-[#e8b23a]/15 text-[#e8b23a] border-[#e8b23a]/40',
  low: 'bg-[#4ec9b0]/15 text-[#4ec9b0] border-[#4ec9b0]/40',
  info: 'bg-[#5b6b7a]/15 text-[#728496] border-[#5b6b7a]/40',
}

function clip(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '…' : s
}

// ---- 消息渲染 ----

function ToolCard({ m }: { m: ChatMessage }) {
  const [open, setOpen] = useState(false)
  const ok = m.meta?.success
  const stdout = m.meta?.stdout ?? ''
  return (
    <div
      className={`relative rounded-lg border pl-3.5 pr-3 py-2 mb-1 msg-in overflow-hidden ${
        ok ? 'border-live/25 bg-live/5' : 'border-alert/30 bg-alert/5'
      }`}
    >
      {/* 左侧状态色条 */}
      <span className={`absolute left-0 top-0 bottom-0 w-[3px] ${ok ? 'bg-gradient-to-b from-live to-live/30' : 'bg-gradient-to-b from-alert to-alert/30'}`} />
      <div className="flex items-center gap-2">
        <span className={`inline-block w-2 h-2 rounded-full shrink-0 ${ok ? 'bg-live shadow-glow-live' : 'bg-alert'} ${ok ? 'animate-pulse' : ''}`} />
        <span className="font-mono text-[13px] text-ink2 font-medium">{m.meta?.tool ?? 'tool'}</span>
        {m.meta?.level !== undefined && (
          <span className="text-[10px] text-ghost border border-line/80 bg-panel2/60 px-1.5 py-px rounded font-mono">
            L{m.meta.level} {LEVEL_ZH[m.meta.level] ?? ''}
          </span>
        )}
        <span className={`text-[11px] font-medium ${ok ? 'text-live' : 'text-alert'}`}>{ok ? '✓ 成功' : '✗ 失败'}</span>
        {stdout && (
          <button onClick={() => setOpen((v) => !v)} className="ml-auto text-[10px] text-muted hover:text-signal transition-colors font-mono">
            {open ? '▾ 收起输出' : '▸ 查看输出'}
          </button>
        )}
      </div>
      {open && stdout && (
        <pre className="mt-2 text-[11px] font-mono text-muted whitespace-pre-wrap max-h-40 overflow-auto bg-ink/70 rounded-md p-2.5 border border-line/50 leading-relaxed">
          {stdout}
        </pre>
      )}
    </div>
  )
}

function FindingCard({ m }: { m: ChatMessage }) {
  const nodes = useStore((s) => s.nodes)
  const select = useStore((s) => s.select)
  // text 形如 "✓ 已证实 finding:xxx" / "○ 待验证 claim:xxx" —— 取节点 id。
  const id = useMemo(() => {
    const t = m.text
    const i = t.indexOf(':')
    return i >= 0 ? t.slice(i + 1).trim() : ''
  }, [m.text])
  const n = nodes[id]
  const sev = n?.severity ?? 'info'
  const confirmed = m.kind === 'graph' && m.text.startsWith('✓')
  return (
    <button
      onClick={() => select(id || null)}
      className="block w-full text-left rounded-lg border px-3 py-2 mb-1 msg-in card-pop card-lift border-line/60 bg-panel2 hover:border-signal/60"
    >
      <div className="flex items-center gap-2">
        <span className={`inline-block w-2 h-2 rounded-full shrink-0 ${confirmed ? 'bg-live animate-pulse shadow-glow-live' : 'bg-ghost'}`} />
        <span className={`text-[10px] uppercase font-disp tracking-wider ${SEV_BADGE[sev] ?? SEV_BADGE.info} border px-1.5 py-0.5 rounded`}>
          {sev}
        </span>
        <span className="font-mono text-[12px] text-ink2 truncate">{clip(id, 60)}</span>
        <span className="ml-auto text-[10px] text-muted shrink-0">{confirmed ? '已证实 · 点我检视' : '假设 · 待验证'}</span>
      </div>
      {n?.technique && (
        <div className="mt-1 text-[10px] text-signal font-mono">MITRE: {n.technique} {n.tactic ? `· ${n.tactic}` : ''}</div>
      )}
      {n && n.evidence.length > 0 && (
        <div className="mt-1.5 text-[11px] text-muted font-mono bg-ink/60 rounded-md px-2.5 py-1.5 border-l-2 border-live/40 leading-relaxed">
          证据[{n.evidence[0].tool}]: {clip(n.evidence[0].excerpt, 90)}
        </div>
      )}
    </button>
  )
}

function ReflectCard({ m }: { m: ChatMessage }) {
  const text = m.meta?.rationale ?? m.text.replace(/^反思:\s*/, '')
  return (
    <div className="rounded-lg border border-[#a78bfa]/30 bg-[#a78bfa]/8 px-3 py-2.5 mb-1 msg-in">
      <div className="flex items-center gap-2 text-[10px] font-disp tracking-wider text-[#a78bfa] uppercase mb-1">
        <span className="animate-pulse">◉</span> AI 战略反思
      </div>
      <div className="text-[12.5px] leading-relaxed text-ink2 whitespace-pre-wrap">{text}</div>
    </div>
  )
}

function ThinkingCard({ m }: { m: ChatMessage }) {
  const text = m.meta?.rationale ?? m.text.replace(/^思考:\s*/, '')
  const [open, setOpen] = useState(false)
  return (
    <div className="rounded-lg border border-[#5b6b7a]/25 bg-[#0d1117]/60 px-3 py-1.5 mb-1 msg-in">
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 text-[10px] font-disp tracking-wider text-ghost uppercase"
      >
        <span className="inline-block w-1.5 h-3 bg-ghost/60 animate-pulse" />
        深度思考{open ? ' ▾' : ' ▸'} · 点击展开/收起
      </button>
      {open && (
        <div className="mt-1.5 text-[12px] leading-relaxed text-[#8fa3b8] whitespace-pre-wrap border-t border-line/40 pt-1.5">
          {text}
        </div>
      )}
    </div>
  )
}

function PhaseChip({ m }: { m: ChatMessage }) {
  const ph = m.meta?.stage ?? ''
  return (
    <div className="flex justify-center my-1.5">
      <span className={`text-[10px] font-disp tracking-[2px] uppercase border px-3 py-1 rounded-full msg-in ${PHASE_COLOR[ph] ?? 'text-ghost border-line'}`}>
        ◈ {PHASE_ZH[ph] ?? ph}
      </span>
    </div>
  )
}

function PlanCard({ m }: { m: ChatMessage }) {
  const count = m.meta?.count
  const why = m.meta?.rationale
  return (
    <div className="rounded-lg border border-signal/25 bg-signal/5 px-3 py-2 mb-1 msg-in">
      <div className="text-[10px] text-signal font-disp tracking-wider uppercase mb-1">▸ 行动计划</div>
      <div className="text-[12px] text-ink2">
        {count !== undefined ? `下一步计划 ${count} 步` : '计划'} {why ? `· ${clip(why, 120)}` : ''}
      </div>
    </div>
  )
}

function HitlCard({ m }: { m: ChatMessage }) {
  const hitl = useStore((s) => s.hitl)
  const [busy, setBusy] = useState(false)
  const clearHitl = useStore((s) => s.clearHitl)
  const decide = async (approved: boolean) => {
    if (!hitl) return
    setBusy(true)
    try {
      await fetch('/approve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: hitl.key, approved }),
      })
      clearHitl() // 裁决后同步清除全局弹窗(修: 双审批 UI 状态不同步)
    } finally {
      setBusy(false)
    }
  }
  const tool = m.meta?.tool ?? hitl?.tool ?? 'tool'
  const why = m.meta?.why ?? hitl?.why ?? ''
  return (
    <div className="rounded-lg border border-alert/50 bg-alert/10 px-3.5 py-3 mb-1 msg-in shadow-glow-alert">
      <div className="flex items-center gap-2 text-[11px] text-alert font-disp tracking-wider uppercase mb-1.5">
        <span className="animate-pulse text-[13px]">⚠</span> 需要人工授权
      </div>
      <div className="font-mono text-[13px] text-ink2 font-medium">{tool}</div>
      {why && <div className="text-[12px] text-muted mt-1 leading-relaxed">{why}</div>}
      <div className="flex gap-2 mt-2.5">
        <button
          onClick={() => decide(true)}
          disabled={busy}
          className="btn-accent px-3.5 py-1.5 text-[12px] rounded-md disabled:opacity-40"
        >
          ✓ 放行
        </button>
        <button
          onClick={() => decide(false)}
          disabled={busy}
          className="btn-danger px-3.5 py-1.5 text-[12px] rounded-md border border-alert/60 text-alert hover:bg-alert/15 disabled:opacity-40"
        >
          ✗ 拒绝
        </button>
      </div>
    </div>
  )
}

function StatusLine({ m }: { m: ChatMessage }) {
  const kind = m.kind
  if (kind === 'engine') {
    return (
      <div className="text-[12px] text-muted mb-1 msg-in">
        <span className="text-live font-disp tracking-wider uppercase text-[10px] mr-2">▶ 战役启动</span>
        {m.text}
      </div>
    )
  }
  const tone =
    kind === 'done' ? 'text-live' : kind === 'error' || kind === 'tool_error' ? 'text-alert' : 'text-muted'
  return (
    <div className={`text-[11.5px] ${tone} mb-0.5 msg-in font-mono`}>
      {m.text}
    </div>
  )
}

function Message({ m }: { m: ChatMessage }) {
  if (m.role === 'user') {
    return (
      <div className="flex justify-end mb-3 msg-in">
        <div className="max-w-[85%] rounded-2xl rounded-tr-sm bg-gradient-to-br from-live/20 to-live/8 border border-live/30 px-4 py-2.5 text-[13px] text-ink2 whitespace-pre-wrap shadow-card">
          {m.text}
        </div>
      </div>
    )
  }
  if (m.kind === 'chat') {
    // 问答回复: 普通文本气泡(打字机逐字填充中也会显示)。
    return (
      <div className="flex mb-3 msg-in">
        <div className="max-w-[92%] rounded-2xl rounded-tl-sm bg-panel2 border border-line/60 shadow-inner-line px-4 py-3 text-[13px] leading-relaxed text-ink2 whitespace-pre-wrap">
          {m.text || '…'}
          {m.text === '' && <span className="inline-block w-1.5 h-4 bg-live/80 align-middle caret-blink ml-0.5 rounded-[1px]" />}
        </div>
      </div>
    )
  }
  switch (m.kind) {
    case 'step':
      return (
        <div className="text-[12px] text-muted mb-0.5 msg-in">
          <span className="text-ghost mr-1.5">🧠</span>
          <span className="font-mono">{m.meta?.tool}</span>
          <span className="text-ghost mx-1.5">·</span>
          {m.meta?.why ?? m.text}
        </div>
      )
    case 'tool':
    case 'tool_result':
      return <ToolCard m={m} />
    case 'graph':
      return <FindingCard m={m} />
    case 'reflect':
      return <ReflectCard m={m} />
    case 'thinking':
      return <ThinkingCard m={m} />
    case 'phase':
      return <PhaseChip m={m} />
    case 'plan':
      return <PlanCard m={m} />
    case 'hitl_request':
      return <HitlCard m={m} />
    default:
      return <StatusLine m={m} />
  }
}

// ---- 主组件 ----

export function ChatView() {
  const messages = useStore((s) => s.messages)
  const status = useStore((s) => s.status)
  const reset = useStore((s) => s.reset)
  const pushMsg = useStore((s) => s.pushMsg)
  const patchMsg = useStore((s) => s.patchMsg)
  const engineLabel = useStore((s) => s.engineLabel)
  const [input, setInput] = useState('http://localhost:3000')
  const bottomRef = useRef<HTMLDivElement>(null)
  const [thinking, setThinking] = useState(false) // 问答请求中(禁用发送/显示等待)

  // 新消息自动滚动到底部。
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length, thinking])

  const start = async (target: string) => {
    const t = target.trim()
    if (!t) return
    try {
      const r = await fetch('/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: t }),
      })
      const body = (await r.json().catch(() => ({}))) as { ok?: boolean; err?: string }
      if (!r.ok || !body.ok) {
        // 拒绝时保留当前窗口并提示(修: 原实现先清空消息再静默发请求, busy 拒绝时无反馈)。
        pushMsg('assistant', 'chat', '无法启动战役: ' + (body.err ?? `HTTP ${r.status}`))
        return
      }
      reset(t)
      setInput(t)
    } catch (err) {
      pushMsg('assistant', 'chat', '启动战役失败: ' + String(err))
    }
  }

  const cancel = async () => {
    try {
      await fetch('/cancel', { method: 'POST' })
    } catch (err) {
      console.error('取消防败:', err)
    }
  }

  // 打字机效果: 逐字填充回答(每帧 2 字符)。
  const typewrite = (id: number, full: string) => {
    let i = 0
    const iv = window.setInterval(() => {
      i += 2
      patchMsg(id, full.slice(0, i))
      if (i >= full.length) window.clearInterval(iv)
    }, 16)
  }

  // 问答: 带多轮历史的 /api/chat 调用, 回答流式打字机展示。
  const ask = async (q: string) => {
    const id = pushMsg('assistant', 'chat', '')
    setThinking(true)
    try {
      // 取最近 8 条用户/问答消息作为多轮历史(排除空占位)。
      const history = useStore
        .getState()
        .messages.filter((m) => m.role === 'user' || m.kind === 'chat')
        .slice(-8)
        .map((m) => [m.role === 'user' ? 'user' : 'assistant', m.text] as [string, string])
        .filter((p) => p[1] !== '')
      const r = await fetch('/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ question: q, history }),
      })
      const body = await r.json().catch(() => ({}))
      if (!r.ok) {
        patchMsg(id, '对话服务不可用: ' + (body.error ?? `HTTP ${r.status}`))
        return
      }
      const ans = (body.answer ?? '(无回答)').trim()
      // 深度思考: 思维链折叠在回答上方。
      const th = body.thinking && String(body.thinking).trim() ? String(body.thinking).trim() : ''
      typewrite(id, (th ? '\n\n[深度思考] ' + th + '\n\n' : '') + ans)
    } catch (err) {
      patchMsg(id, '对话请求失败: ' + String(err))
    } finally {
      setThinking(false)
    }
  }

  // 意图分流: 目标(URL/主机) -> 跑战役; 停止类命令 -> 取消; 其余 -> 问答。
  const submit = (raw: string) => {
    const t = raw.trim()
    if (!t || thinking) return
    if (/^(https?:\/\/)?([\w-]+\.)*[\w-]+(\.[a-zA-Z]{2,})?(:\d+)?([/?#]\S*)?$/i.test(t) && !t.includes(' ')) {
      void start(t)
      return
    }
    if (/^(停止|取消|停|stop|cancel|别再)/i.test(t)) {
      pushMsg('user', 'user', t)
      void cancel()
      return
    }
    // 一般问题/指令 -> AI 问答。
    pushMsg('user', 'user', t)
    setInput('')
    void ask(t)
  }

  const onKey = (ev: { key: string; shiftKey: boolean; preventDefault: () => void }) => {
    if (ev.key === 'Enter' && !ev.shiftKey) {
      ev.preventDefault()
      submit(input)
    }
  }

  const empty = messages.length === 0
  const engineChip = engineLabel ? `${engineLabel} 引擎` : ''

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* 消息流 */}
      <div className="flex-1 min-h-0 overflow-y-auto px-4 md:px-8 py-6">
        {empty ? (
          <div className="h-full flex flex-col items-center justify-center text-center gap-5 msg-in px-4">
            <div className="hero-badge w-[72px] h-[72px] rounded-2xl border border-live/40 bg-gradient-to-br from-live/20 to-live/5 flex items-center justify-center text-[34px] shadow-glow-live">
              🦅
            </div>
            <div>
              <div className="font-disp text-[22px] font-semibold tracking-wide bg-gradient-to-r from-signal via-ink2 to-live bg-clip-text text-transparent">
                Vero 自主渗透助手
              </div>
              <div className="text-[13px] text-muted mt-2">
                发目标让我自主渗透 · 或直接问我任何问题
              </div>
              <div className="text-[11px] text-ghost mt-1.5 font-mono">
                例: 「http://localhost:3000」跑战役 · 「这个 SQLi 严重吗」问答 · 「停止」取消
              </div>
            </div>
            <div className="flex flex-wrap gap-2.5 justify-center mt-3 max-w-lg">
              {['http://localhost:3000', 'http://localhost:8080'].map((t) => (
                <button
                  key={t}
                  onClick={() => void start(t)}
                  className="group px-4 py-2 text-[12px] font-mono rounded-full border border-line bg-panel2/60 text-muted hover:text-live hover:border-live/50 hover:shadow-glow-live transition-all duration-200 card-lift"
                >
                  <span className="text-live/60 group-hover:text-live mr-1.5">▸</span>
                  {t}
                </button>
              ))}
              <button
                onClick={() => void ask('渗透测试中 SQL 注入的原理、危害与防御是什么？')}
                className="px-4 py-2 text-[12px] rounded-full border border-line bg-panel2/60 text-muted hover:text-signal hover:border-signal/50 hover:shadow-glow-signal transition-all duration-200 card-lift"
              >
                <span className="text-signal/70 mr-1.5">✦</span>
                试试问我: SQLi 是什么
              </button>
            </div>
          </div>
        ) : (
          messages.map((m) => <Message key={m.id} m={m} />)
        )}
        <div ref={bottomRef} />
      </div>

      {/* 输入区 */}
      <div className="border-t border-line/80 glass px-4 md:px-8 py-3.5">
        <div className="max-w-3xl mx-auto">
          {engineChip && status !== 'idle' && (
            <div className="text-[10px] text-muted mb-1.5 font-mono flex items-center gap-1.5">
              <span className={`inline-block w-1.5 h-1.5 rounded-full ${status === 'running' ? 'bg-signal ring-pulse' : 'bg-live'}`} />
              {engineChip} · {EVENT_LABELS[status] ?? status}
            </div>
          )}
          <div className="flex items-end gap-2 rounded-xl border border-line bg-ink/60 shadow-inner-line focus-within:border-live/60 focus-within:shadow-glow-live transition-all duration-200 px-3 py-2">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={onKey}
              rows={1}
              placeholder="发目标跑战役, 或问我任何问题…"
              className="flex-1 bg-transparent outline-none resize-none text-[13.5px] text-ink2 placeholder:text-ghost/80 font-mono leading-relaxed"
              style={{ maxHeight: 120 }}
            />
            {status === 'running' ? (
              <button
                onClick={() => void cancel()}
                className="btn-danger px-3.5 py-2 text-[12px] rounded-lg border border-alert/60 text-alert hover:bg-alert/15 shrink-0 font-medium"
                title="停止当前战役"
              >
                ■ 停止
              </button>
            ) : (
              <button
                onClick={() => submit(input)}
                disabled={thinking}
                className="btn-accent px-4 py-2 text-[12px] rounded-lg shrink-0"
              >
                {thinking ? (
                  <span className="flex gap-0.5 px-1">
                    <span className="dot-pulse">●</span>
                    <span className="dot-pulse">●</span>
                    <span className="dot-pulse">●</span>
                  </span>
                ) : (
                  '⟶ 发送'
                )}
              </button>
            )}
          </div>
          <div className="text-[10px] text-ghost/90 mt-2 font-mono flex items-center gap-1.5 flex-wrap">
            <kbd className="px-1.5 py-0.5 rounded border border-line/80 bg-panel2/80 text-[9.5px] text-muted">Enter</kbd>
            <span>发送 · 目标=URL/IP · 其他内容 = 问我 · 攻击动作 L3+ 会请求你授权</span>
          </div>
        </div>
      </div>
    </div>
  )
}
