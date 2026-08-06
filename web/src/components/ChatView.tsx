import { useEffect, useMemo, useRef, useState } from 'react'
import { useStore } from '../store'
import { LEVEL_ZH, EVENT_LABELS } from '../lib/i18n'
import { TARGET_RE, STOP_RE, startCampaign, cancelCampaign } from '../lib/actions'
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
  idle: 'text-ghost border-line',
  init: 'text-ghost border-line',
  recon: 'text-info border-info/50',
  scan: 'text-warn border-warn/50',
  exploit: 'text-alert border-alert/50',
  done: 'text-live border-live/50',
}

// severity → 徽章样式(珍珠白亮色: 朱红/橙/琥珀/绿/灰)。
const SEV_BADGE: Record<string, string> = {
  critical: 'bg-alert/10 text-alert border-alert/40',
  high: 'bg-[#d06a1f]/10 text-[#d06a1f] border-[#d06a1f]/40',
  medium: 'bg-warn/10 text-warn border-warn/40',
  low: 'bg-live/10 text-live border-live/40',
  info: 'bg-ghost/10 text-muted border-ghost/40',
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
    <div className={`rounded-md border px-3 py-2 mb-1.5 msg-in bg-white ${ok ? 'border-line' : 'border-alert/40'}`}>
      <div className="flex items-center gap-2">
        <span className={`inline-block w-1.5 h-1.5 rounded-full shrink-0 ${ok ? 'bg-live' : 'bg-alert'}`} />
        <span className="font-mono text-[12.5px] text-ink2 font-medium">{m.meta?.tool ?? 'tool'}</span>
        {m.meta?.level !== undefined && (
          <span className="text-[10px] text-ghost border border-line bg-panel2 px-1.5 py-px rounded font-mono">
            L{m.meta.level} {LEVEL_ZH[m.meta.level] ?? ''}
          </span>
        )}
        <span className={`text-[11px] font-medium ${ok ? 'text-live' : 'text-alert'}`}>{ok ? '成功' : '失败'}</span>
        {stdout && (
          <button onClick={() => setOpen((v) => !v)} className="ml-auto text-[11px] text-muted hover:text-signal transition-colors">
            {open ? '收起' : '输出'}
          </button>
        )}
      </div>
      {open && stdout && (
        <pre className="mt-2 text-[11px] font-mono text-muted whitespace-pre-wrap max-h-40 overflow-auto bg-panel2 rounded p-2.5 border border-line leading-relaxed">
          {stdout}
        </pre>
      )}
    </div>
  )
}

function FindingCard({ m }: { m: ChatMessage }) {
  const nodes = useStore((s) => s.nodes)
  const select = useStore((s) => s.select)
  // text 形如 "已证实 finding:xxx" / "待验证 claim:xxx" —— 取节点 id(冒号后部分)。
  const id = useMemo(() => {
    const t = m.text
    const i = t.indexOf(':')
    return i >= 0 ? t.slice(i + 1).trim() : ''
  }, [m.text])
  const n = nodes[id]
  const sev = n?.severity ?? 'info'
  const confirmed = m.kind === 'graph' && m.text.startsWith('已证实')
  return (
    <button
      onClick={() => select(id || null)}
      className="block w-full text-left rounded-md border px-3 py-2 mb-1.5 msg-in border-line bg-white hover:border-signal/60 transition-colors"
    >
      <div className="flex items-center gap-2">
        <span className={`inline-block w-1.5 h-1.5 rounded-full shrink-0 ${confirmed ? 'bg-live' : 'bg-ghost/70'}`} />
        <span className={`text-[10px] ${SEV_BADGE[sev] ?? SEV_BADGE.info} border px-1.5 py-0.5 rounded font-medium`}>
          {sev}
        </span>
        <span className="font-mono text-[12px] text-ink2 truncate">{clip(id, 60)}</span>
        <span className="ml-auto text-[10px] text-muted shrink-0">{confirmed ? '已证实 · 点击检视' : '假设 · 待验证'}</span>
      </div>
      {n?.technique && (
        <div className="mt-1 text-[10px] text-signal font-mono">MITRE: {n.technique} {n.tactic ? `· ${n.tactic}` : ''}</div>
      )}
      {n && n.evidence.length > 0 && (
        <div className="mt-1.5 text-[11px] text-muted font-mono bg-panel2 rounded px-2.5 py-1.5 border-l-2 border-live/50 leading-relaxed">
          证据[{n.evidence[0].tool}]: {clip(n.evidence[0].excerpt, 90)}
        </div>
      )}
    </button>
  )
}

function ReflectCard({ m }: { m: ChatMessage }) {
  const text = m.meta?.rationale ?? m.text.replace(/^反思:\s*/, '')
  return (
    <div className="rounded-md border border-violet/30 bg-violet/6 px-3 py-2.5 mb-1.5 msg-in">
      <div className="text-[10px] font-medium text-violet mb-1">战役反思</div>
      <div className="text-[12.5px] leading-relaxed text-ink2 whitespace-pre-wrap">{text}</div>
    </div>
  )
}

function ThinkingCard({ m }: { m: ChatMessage }) {
  const text = m.meta?.rationale ?? m.text.replace(/^思考:\s*/, '')
  const [open, setOpen] = useState(false)
  return (
    <div className="rounded-md border border-violet/20 bg-violet/5 px-3 py-1.5 mb-1.5 msg-in">
      <button onClick={() => setOpen((v) => !v)} className="text-[10px] font-medium text-violet/90">
        决策推理{open ? ' − 收起' : ' + 展开'}
      </button>
      {open && (
        <div className="mt-1.5 text-[12px] leading-relaxed text-muted whitespace-pre-wrap border-t border-violet/15 pt-1.5">
          {text}
        </div>
      )}
    </div>
  )
}

function PhaseChip({ m }: { m: ChatMessage }) {
  const ph = m.meta?.stage ?? ''
  return (
    <div className="flex justify-center my-2">
      <span className={`text-[10.5px] border px-3 py-1 rounded-full msg-in bg-white ${PHASE_COLOR[ph] ?? 'text-ghost border-line'}`}>
        {PHASE_ZH[ph] ?? ph}
      </span>
    </div>
  )
}

function PlanCard({ m }: { m: ChatMessage }) {
  const count = m.meta?.count
  const why = m.meta?.rationale
  return (
    <div className="rounded-md border border-signal/30 bg-signal/6 px-3 py-2 mb-1.5 msg-in">
      <div className="text-[10px] text-signal font-medium mb-0.5">行动计划</div>
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
      clearHitl() // 裁决后同步清除全局弹窗(双审批 UI 状态一致)
    } finally {
      setBusy(false)
    }
  }
  const tool = m.meta?.tool ?? hitl?.tool ?? 'tool'
  const why = m.meta?.why ?? hitl?.why ?? ''
  return (
    <div className="rounded-md border border-alert/50 bg-alert/8 px-3.5 py-3 mb-1.5 msg-in">
      <div className="text-[11px] text-alert font-medium mb-1.5">需要人工授权</div>
      <div className="font-mono text-[13px] text-ink2 font-medium">{tool}</div>
      {why && <div className="text-[12px] text-muted mt-1 leading-relaxed">{why}</div>}
      <div className="flex gap-2 mt-2.5">
        <button
          onClick={() => decide(true)}
          disabled={busy}
          className="btn-accent px-3.5 py-1.5 text-[12px] rounded-md disabled:opacity-40"
        >
          放行
        </button>
        <button
          onClick={() => decide(false)}
          disabled={busy}
          className="btn-danger px-3.5 py-1.5 text-[12px] rounded-md border border-alert/50 text-alert disabled:opacity-40"
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
    return (
      <div className="text-[12px] text-muted mb-1 msg-in">
        <span className="text-live font-medium mr-2">战役启动</span>
        {m.text}
      </div>
    )
  }
  const tone =
    kind === 'done' ? 'text-live' :
    kind === 'warning' ? 'text-warning' :
    kind === 'error' || kind === 'tool_error' ? 'text-alert' : 'text-muted'
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
        <div className="max-w-[85%] rounded-lg bg-signal/10 border border-signal/30 px-3.5 py-2 text-[13px] text-ink2 whitespace-pre-wrap">
          {m.text}
        </div>
      </div>
    )
  }
  if (m.kind === 'chat') {
    // 问答回复: 普通文本气泡(打字机逐字填充中也会显示)。
    return (
      <div className="flex mb-3 msg-in">
        <div className="max-w-[92%] rounded-lg bg-white border border-line px-3.5 py-2.5 text-[13px] leading-relaxed text-ink2 whitespace-pre-wrap">
          {m.text || '…'}
          {m.text === '' && <span className="inline-block w-1.5 h-4 bg-signal/70 align-middle caret-blink ml-0.5 rounded-[1px]" />}
        </div>
      </div>
    )
  }
  switch (m.kind) {
    case 'step':
      return (
        <div className="text-[12px] text-muted mb-0.5 msg-in">
          <span className="font-mono text-ink2/80">{m.meta?.tool}</span>
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
  const pushMsg = useStore((s) => s.pushMsg)
  const patchMsg = useStore((s) => s.patchMsg)
  const engineLabel = useStore((s) => s.engineLabel)
  const [input, setInput] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const [thinking, setThinking] = useState(false) // 问答请求中(禁用发送/显示等待)

  // 战役结束/停止后自动清空输入框, 恢复到待命状态
  const prevStatusRef = useRef(status)
  useEffect(() => {
    if (prevStatusRef.current === 'running' && status === 'idle') {
      setInput('')
    }
    prevStatusRef.current = status
  }, [status])

  // 新消息自动滚动到底部。
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length, thinking])

  const start = async (target: string) => {
    const t = target.trim()
    if (!t) return
    const r = await startCampaign(t)
    if (!r.ok) {
      // 拒绝时保留当前窗口并提示(busy 拒绝/网络错误可见)。
      pushMsg('assistant', 'chat', '无法启动战役: ' + (r.err ?? '未知错误'))
      return
    }
    setInput('')
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
      typewrite(id, (th ? '\n\n[决策推理] ' + th + '\n\n' : '') + ans)
    } catch (err) {
      patchMsg(id, '对话请求失败: ' + String(err))
    } finally {
      setThinking(false)
    }
  }

  // 意图分流: 停止类指令优先(避免裸词被目标正则吞掉) -> 目标跑战役 -> 其余问答。
  const submit = (raw: string) => {
    const t = raw.trim()
    if (!t || thinking) return
    if (STOP_RE.test(t)) {
      pushMsg('user', 'user', t)
      void cancelCampaign()
      return
    }
    if (TARGET_RE.test(t) && !t.includes(' ')) {
      void start(t)
      return
    }
    // 一般问题/指令 -> 问答。
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
          <div className="h-full flex flex-col items-center justify-center text-center gap-4 msg-in px-4">
            <div>
              <div className="text-[24px] font-semibold tracking-tight text-ink2">
                Vero 渗透测试助手
              </div>
              <div className="text-[13px] text-muted mt-2">
                粘贴目标开始自主渗透, 或直接提问
              </div>
              <div className="text-[11.5px] text-ghost mt-1.5">
                例: 「http://localhost:3000」跑战役 · 「这个 SQLi 严重吗」问答 · 「停止」取消
              </div>
            </div>
            <div className="flex flex-wrap gap-2 justify-center mt-2 max-w-lg">
              {['http://localhost:3000', 'http://localhost:8080'].map((t) => (
                <button
                  key={t}
                  onClick={() => void start(t)}
                  className="px-3.5 py-1.5 text-[12px] font-mono rounded-md border border-line bg-white text-muted hover:text-signal hover:border-signal/60 transition-colors card-lift"
                >
                  {t}
                </button>
              ))}
              <button
                onClick={() => void ask('渗透测试中 SQL 注入的原理、危害与防御是什么？')}
                className="px-3.5 py-1.5 text-[12px] rounded-md border border-line bg-white text-muted hover:text-violet hover:border-violet/50 transition-colors card-lift"
              >
                试试问我: SQLi 是什么
              </button>
            </div>
            <div className="flex items-center gap-4 mt-2 text-[11px] text-ghost">
              <span><span className="kbd">Ctrl K</span> 命令面板</span>
              <span><span className="kbd">Ctrl H</span> 历史战役</span>
              <span><span className="kbd">G</span> 攻击图全屏</span>
            </div>
          </div>
        ) : (
          messages.map((m) => <Message key={m.id} m={m} />)
        )}
        <div ref={bottomRef} />
      </div>

      {/* 输入区 */}
      <div className="border-t border-line bg-white px-4 md:px-8 py-3.5">
        <div className="max-w-3xl mx-auto">
          {engineChip && status !== 'idle' && (
            <div className="text-[11px] text-muted mb-1.5 flex items-center gap-1.5">
              <span className={`inline-block w-1.5 h-1.5 rounded-full ${status === 'running' ? 'bg-warn ring-pulse' : 'bg-live'}`} />
              {engineChip} · {EVENT_LABELS[status] ?? status}
            </div>
          )}
          <div className="flex items-end gap-2 rounded-lg border border-line bg-panel focus-within:border-signal/60 transition-colors px-3 py-2">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={onKey}
              rows={1}
              placeholder="粘贴目标跑战役, 或提问…"
              className="flex-1 bg-transparent outline-none resize-none text-[13.5px] text-ink2 placeholder:text-ghost/90 leading-relaxed"
              style={{ maxHeight: 120 }}
            />
            {status === 'running' ? (
              <button
                onClick={() => void cancelCampaign()}
                className="btn-danger px-3.5 py-2 text-[12px] rounded-md border border-alert/50 text-alert shrink-0 font-medium"
                title="停止当前战役"
              >
                停止
              </button>
            ) : (
              <button
                onClick={() => submit(input)}
                disabled={thinking}
                className="btn-accent px-4 py-2 text-[12px] rounded-md shrink-0"
              >
                {thinking ? (
                  <span className="flex gap-0.5 px-1">
                    <span className="dot-pulse">●</span>
                    <span className="dot-pulse">●</span>
                    <span className="dot-pulse">●</span>
                  </span>
                ) : (
                  '发送'
                )}
              </button>
            )}
          </div>
          <div className="text-[10.5px] text-ghost mt-2 flex items-center gap-1.5 flex-wrap">
            <span className="kbd">Enter</span>
            <span>发送 · 目标 = URL/IP · 其他内容 = 提问 · L3+ 攻击动作会请求授权</span>
          </div>
        </div>
      </div>
    </div>
  )
}
