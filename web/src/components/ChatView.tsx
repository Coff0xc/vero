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
    <div className={`rounded-lg border px-3 py-2 mb-1 msg-in ${ok ? 'border-live/25 bg-live/5' : 'border-alert/30 bg-alert/5'}`}>
      <div className="flex items-center gap-2">
        <span className={`inline-block w-2 h-2 rounded-full ${ok ? 'bg-live' : 'bg-alert'} ${ok ? 'animate-pulse' : ''}`} />
        <span className="font-mono text-[13px] text-ink2">{m.meta?.tool ?? 'tool'}</span>
        {m.meta?.level !== undefined && (
          <span className="text-[10px] text-ghost border border-line px-1 rounded">L{m.meta.level} {LEVEL_ZH[m.meta.level] ?? ''}</span>
        )}
        <span className={`text-[11px] ${ok ? 'text-live' : 'text-alert'}`}>{ok ? '✓ 成功' : '✗ 失败'}</span>
        {stdout && (
          <button onClick={() => setOpen((v) => !v)} className="ml-auto text-[10px] text-muted hover:text-signal">
            {open ? '收起输出' : '查看输出'}
          </button>
        )}
      </div>
      {open && stdout && (
        <pre className="mt-2 text-[11px] font-mono text-muted whitespace-pre-wrap max-h-40 overflow-auto bg-ink/60 rounded p-2 border border-line/50">
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
      className="block w-full text-left rounded-lg border px-3 py-2 mb-1 msg-in card-pop border-line/60 bg-panel2 hover:border-signal/50 transition-colors"
    >
      <div className="flex items-center gap-2">
        <span className={`inline-block w-2 h-2 rounded-full ${confirmed ? 'bg-live animate-pulse' : 'bg-ghost'}`} />
        <span className={`text-[10px] uppercase font-disp tracking-wider ${SEV_BADGE[sev] ?? SEV_BADGE.info} border px-1.5 py-0.5 rounded`}>
          {sev}
        </span>
        <span className="font-mono text-[12px] text-ink2 truncate">{clip(id, 60)}</span>
        <span className="ml-auto text-[10px] text-muted">{confirmed ? '已证实 · 点我检视' : '假设 · 待验证'}</span>
      </div>
      {n?.technique && (
        <div className="mt-1 text-[10px] text-signal font-mono">MITRE: {n.technique} {n.tactic ? `· ${n.tactic}` : ''}</div>
      )}
      {n && n.evidence.length > 0 && (
        <div className="mt-1.5 text-[11px] text-muted font-mono bg-ink/50 rounded px-2 py-1 border border-line/40">
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
  const decide = async (approved: boolean) => {
    if (!hitl) return
    setBusy(true)
    try {
      await fetch('/approve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: hitl.key, approved }),
      })
    } finally {
      setBusy(false)
    }
  }
  const tool = m.meta?.tool ?? hitl?.tool ?? 'tool'
  const why = m.meta?.why ?? hitl?.why ?? ''
  return (
    <div className="rounded-lg border border-alert/40 bg-alert/8 px-3 py-2.5 mb-1 msg-in">
      <div className="flex items-center gap-2 text-[11px] text-alert font-disp tracking-wider uppercase mb-1">
        <span className="animate-pulse">⚠</span> 需要人工授权
      </div>
      <div className="font-mono text-[13px] text-ink2">{tool}</div>
      {why && <div className="text-[12px] text-muted mt-0.5">{why}</div>}
      <div className="flex gap-2 mt-2">
        <button
          onClick={() => decide(true)}
          disabled={busy}
          className="px-3 py-1 text-[12px] rounded border border-live/50 text-live hover:bg-live/10 disabled:opacity-40"
        >
          放行
        </button>
        <button
          onClick={() => decide(false)}
          disabled={busy}
          className="px-3 py-1 text-[12px] rounded border border-alert/50 text-alert hover:bg-alert/10 disabled:opacity-40"
        >
          拒绝
        </button>
      </div>
    </div>
  )
}

function StatusLine({ m }: { m: ChatMessage }) {
  const kind = m.kind
  if (kind === 'engine') {
    const target = m.text.includes('目标') ? m.text : m.text
    return (
      <div className="text-[12px] text-muted mb-1 msg-in">
        <span className="text-live font-disp tracking-wider uppercase text-[10px] mr-2">▶ 战役启动</span>
        {target}
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
        <div className="max-w-[85%] rounded-2xl rounded-tr-sm bg-live/12 border border-live/25 px-4 py-2.5 text-[13px] text-ink2 whitespace-pre-wrap">
          <div className="text-[10px] text-live/70 font-disp tracking-wider uppercase mb-1">目标</div>
          {m.text}
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
  const engineLabel = useStore((s) => s.engineLabel)
  const [input, setInput] = useState('http://localhost:3000')
  const bottomRef = useRef<HTMLDivElement>(null)

  // 新消息自动滚动到底部。
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length])

  const start = async (target: string) => {
    const t = target.trim()
    if (!t) return
    reset(t)
    setInput(t)
    try {
      await fetch('/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target: t }),
      })
    } catch (err) {
      console.error('启动战役失败:', err)
    }
  }

  const cancel = async () => {
    try {
      await fetch('/cancel', { method: 'POST' })
    } catch (err) {
      console.error('取消防败:', err)
    }
  }

  const onKey = (ev: { key: string; shiftKey: boolean; preventDefault: () => void }) => {
    if (ev.key === 'Enter' && !ev.shiftKey) {
      ev.preventDefault()
      void start(input)
    }
  }

  const empty = messages.length === 0
  const engineChip = engineLabel ? `${engineLabel} 引擎` : ''

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* 消息流 */}
      <div className="flex-1 min-h-0 overflow-y-auto px-4 md:px-8 py-6">
        {empty ? (
          <div className="h-full flex flex-col items-center justify-center text-center gap-4 msg-in">
            <div className="w-16 h-16 rounded-2xl border border-live/40 bg-live/10 flex items-center justify-center text-3xl animate-pulse">
              🦅
            </div>
            <div>
              <div className="font-disp text-lg text-ink2 tracking-wide">Vero 自主渗透助手</div>
              <div className="text-[12.5px] text-muted mt-1">告诉我目标，我会自主侦察 · 生成假设 · 验证利用 · 全程证据可溯</div>
            </div>
            <div className="flex flex-wrap gap-2 justify-center mt-2">
              {['http://localhost:3000', 'http://localhost:8080', 'http://localhost:8000'].map((t) => (
                <button
                  key={t}
                  onClick={() => void start(t)}
                  className="px-3 py-1.5 text-[12px] font-mono rounded-full border border-line text-muted hover:text-signal hover:border-signal/50 transition-colors"
                >
                  {t}
                </button>
              ))}
            </div>
          </div>
        ) : (
          messages.map((m) => <Message key={m.id} m={m} />)
        )}
        <div ref={bottomRef} />
      </div>

      {/* 输入区 */}
      <div className="border-t border-line bg-panel2/90 backdrop-blur px-4 md:px-8 py-3">
        <div className="max-w-3xl mx-auto">
          {engineChip && status !== 'idle' && (
            <div className="text-[10px] text-muted mb-1.5 font-mono">{engineChip} · {EVENT_LABELS[status] ?? status}</div>
          )}
          <div className="flex items-end gap-2 rounded-xl border border-line bg-ink/50 focus-within:border-signal/50 transition-colors px-3 py-2">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={onKey}
              rows={1}
              placeholder="输入目标(URL/IP)后回车，如 http://localhost:3000"
              className="flex-1 bg-transparent outline-none resize-none text-[13.5px] text-ink2 placeholder:text-ghost font-mono"
              style={{ maxHeight: 120 }}
            />
            {status === 'running' ? (
              <button
                onClick={() => void cancel()}
                className="px-3.5 py-2 text-[12px] rounded-lg border border-alert/50 text-alert hover:bg-alert/10 transition-colors shrink-0"
                title="停止当前战役"
              >
                ■ 停止
              </button>
            ) : (
              <button
                onClick={() => void start(input)}
                className="px-3.5 py-2 text-[12px] rounded-lg border border-live/50 text-live hover:bg-live/10 transition-colors shrink-0"
              >
                ⟶ 发送
              </button>
            )}
          </div>
          <div className="text-[10px] text-ghost mt-1.5 font-mono">
            Enter 发送 · Shift+Enter 换行 · 攻击动作 L3+ 会请求你授权
          </div>
        </div>
      </div>
    </div>
  )
}
