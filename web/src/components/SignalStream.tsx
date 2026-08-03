import { useEffect, useMemo, useRef, useState } from 'react'
import { useStore } from '../store'
import { EVENT_LABELS } from '../lib/i18n'
import type { LogLine } from '../types'

// 视图模式: M=消息流(平铺), T=瀑布树(plan→step→tool 嵌套), D=默认(同消息流)。
type ViewMode = 'M' | 'T' | 'D'

const MODES: { id: ViewMode; label: string; title: string }[] = [
  { id: 'M', label: '消息流', title: '消息流视图' },
  { id: 'T', label: '瀑布树', title: '瀑布/运行树视图' },
  { id: 'D', label: '默认', title: '默认视图' },
]

// 色映射键仍用英文 kind(仅决定边框/文字色), label 换成中文。
const BORDER: Record<string, string> = {
  step: 'border-l-ghost',
  tool: 'border-l-live',
  graph: 'border-l-live',
  edge: 'border-l-ghost',
  hitl_request: 'border-l-alert',
  route: 'border-l-signal',
  summary: 'border-l-signal',
  done: 'border-l-signal',
  plan: 'border-l-signal',
  workflow_start: 'border-l-signal',
  workflow_stage: 'border-l-ghost',
  workflow_complete: 'border-l-signal',
  workflow_cancelled: 'border-l-alert',
  tool_result: 'border-l-live',
  tool_error: 'border-l-alert',
  path: 'border-l-signal',
  phase: 'border-l-signal',
  error: 'border-l-alert',
}
const KIND: Record<string, string> = {
  step: 'text-ghost',
  tool: 'text-live',
  graph: 'text-live',
  edge: 'text-ghost',
  hitl_request: 'text-alert',
  route: 'text-signal',
  summary: 'text-signal',
  done: 'text-signal',
  plan: 'text-signal',
  workflow_start: 'text-signal',
  workflow_stage: 'text-ghost',
  workflow_complete: 'text-signal',
  workflow_cancelled: 'text-alert',
  tool_result: 'text-live',
  tool_error: 'text-alert',
  path: 'text-signal',
  phase: 'text-signal',
  error: 'text-alert',
}

// ---- 瀑布/运行树 ----
// 把 plan→step→tool→tool_result 事件按父子关系组装成嵌套树:
//   plan 为根 → 其下 step → 再下 tool/tool_result;
//   其余事件(workflow_*/graph/route 等)就近挂到当前 plan 或作为独立根。
type WKind = 'plan' | 'step' | 'tool' | 'result' | 'other'

interface WNode {
  id: number
  kind: WKind
  line: LogLine // 代表行(工具取其起始 tool 行; 结果合并进 result 字段)
  result?: LogLine // tool 的完成结果(tool_result/tool_error), 用于成功/失败与耗时
  children: WNode[]
  depth: number
  startTs?: number
  endTs?: number
  durationMs?: number
}

function buildWaterfall(log: LogLine[]): WNode[] {
  const roots: WNode[] = []
  let plan: WNode | null = null
  let step: WNode | null = null
  let openTool: WNode | null = null

  const childDepth = (base: number) => (step ? base + 1 : plan ? base : base)

  for (const line of log) {
    const ts = line.ts
    switch (line.kind) {
      case 'plan': {
        plan = { id: line.id, kind: 'plan', line, children: [], depth: 0, startTs: ts, endTs: ts }
        roots.push(plan)
        step = null
        openTool = null
        break
      }
      case 'step': {
        step = { id: line.id, kind: 'step', line, children: [], depth: plan ? 1 : 0, startTs: ts, endTs: ts }
        if (plan) plan.children.push(step)
        else roots.push(step)
        openTool = null
        break
      }
      case 'tool': {
        openTool = {
          id: line.id,
          kind: 'tool',
          line,
          children: [],
          depth: childDepth(1),
          startTs: ts,
          endTs: ts,
        }
        if (step) step.children.push(openTool)
        else if (plan) plan.children.push(openTool)
        else roots.push(openTool)
        break
      }
      case 'tool_result':
      case 'tool_error': {
        if (openTool && openTool.line.kind === 'tool' && openTool.line.meta?.tool === line.meta?.tool) {
          // 结果归属到正在等待的 tool 节点(成功/失败 + 耗时都在此显示)。
          openTool.result = line
          openTool.endTs = ts
        } else {
          const n: WNode = { id: line.id, kind: 'result', line, children: [], depth: childDepth(1), startTs: ts, endTs: ts }
          if (step) step.children.push(n)
          else if (plan) plan.children.push(n)
          else roots.push(n)
        }
        break
      }
      case 'hitl_request': {
        const n: WNode = { id: line.id, kind: 'other', line, children: [], depth: childDepth(1), startTs: ts, endTs: ts }
        if (step) step.children.push(n)
        else if (plan) plan.children.push(n)
        else roots.push(n)
        break
      }
      case 'workflow_start':
      case 'workflow_stage':
      case 'workflow_complete':
      case 'workflow_cancelled': {
        const n: WNode = { id: line.id, kind: 'other', line, children: [], depth: plan ? 1 : 0, startTs: ts, endTs: ts }
        if (plan) plan.children.push(n)
        else roots.push(n)
        break
      }
      default: {
        // graph/route/summary/phase/engine 等: 就近挂到当前 plan, 否则独立根。
        const n: WNode = { id: line.id, kind: 'other', line, children: [], depth: plan ? 1 : 0, startTs: ts, endTs: ts }
        if (plan) plan.children.push(n)
        else roots.push(n)
        break
      }
    }
  }

  // 后序计算每个节点的持续时长: 父级 = 其子树最后事件时间 - 自身开始时间(参考 Langfuse 瀑布)。
  const resolve = (n: WNode): number | undefined => {
    let end = n.endTs
    for (const c of n.children) {
      const ce = resolve(c)
      if (ce !== undefined && (end === undefined || ce > end)) end = ce
    }
    n.endTs = end
    if (n.startTs !== undefined && n.endTs !== undefined && n.endTs > n.startTs) {
      n.durationMs = n.endTs - n.startTs
    }
    return end
  }
  for (const r of roots) resolve(r)
  return roots
}

function fmtDur(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

// 色条: 工具行按成功/失败染绿/红, 其余按层级中性色。
function barColor(n: WNode): string {
  if (n.kind === 'tool' || n.kind === 'result') {
    const failed = n.line.kind === 'tool_error' || n.line.meta?.success === false
    return failed ? 'bg-alert' : 'bg-live'
  }
  if (n.kind === 'step') return 'bg-ghost'
  if (n.kind === 'plan') return 'bg-signal'
  return 'bg-ghost'
}

function WaterfallTree({ log }: { log: LogLine[] }) {
  const roots = useMemo(() => buildWaterfall(log), [log])
  const [collapsed, setCollapsed] = useState<ReadonlySet<number>>(() => new Set())

  const toggle = (id: number) => {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  if (log.length === 0) {
    return (
      <div className="h-full flex items-center justify-center text-muted text-xs">
        等待事件流…
      </div>
    )
  }
  if (roots.length === 0) {
    return (
      <div className="h-full flex items-center justify-center text-muted text-xs">
        暂无计划树
      </div>
    )
  }

  const renderNode = (n: WNode) => {
    const collapsible = n.kind === 'plan' || n.kind === 'step'
    const open = !collapsed.has(n.id)
    const label = EVENT_LABELS[n.line.kind] ?? n.line.kind
    const color = KIND[n.line.kind] ?? 'text-muted'
    const bar = barColor(n)

    // 工具行: 名 + 成功/失败; 结果合并展示。
    let main = ''
    let sub: string | undefined
    if (n.kind === 'tool' || n.kind === 'result') {
      const t = n.result?.meta?.tool ?? n.line.meta?.tool ?? '—'
      const err = n.line.kind === 'tool_error' ? n.line.meta?.error : n.result?.kind === 'tool_error' ? n.result.meta?.error : undefined
      const ok = err ? false : n.line.kind === 'tool_result' ? n.line.meta?.success : n.result?.meta?.success
      main = `${t} ${err ? `✗ ${err}` : ok === false ? '✗ 失败' : ok ? '✓ 成功' : '…'}`
      const out = n.line.kind === 'tool_result' ? n.line.meta?.stdout : n.result?.meta?.stdout
      if (out) sub = out.replace(/\n/g, ' ↵ ')
    } else if (n.kind === 'step') {
      main = `L${n.line.meta?.level ?? '?'} · ${n.line.meta?.tool ?? '—'}`
      if (n.line.meta?.why) sub = `▍推理 ${n.line.meta.why}`
    } else if (n.kind === 'plan') {
      main = `共 ${n.line.meta?.count ?? '?'} 步行动`
      if (n.line.meta?.rationale) sub = `▍${n.line.meta.rationale}`
    } else {
      main = n.line.text
    }

    return (
      <div key={n.id}>
        <div
          className="flex items-center gap-2 py-1 text-[12px] leading-snug"
          style={{ paddingLeft: 8 + n.depth * 18 }}
        >
          {/* 折叠开关(仅父级) */}
          {collapsible ? (
            <button
              onClick={() => toggle(n.id)}
              className="w-3.5 shrink-0 text-muted hover:text-ink2 text-[10px] leading-none select-none"
              title={open ? '折叠' : '展开'}
            >
              {open ? '▾' : '▸'}
            </button>
          ) : (
            <span className="w-3.5 shrink-0 inline-block" />
          )}
          {/* 成功/失败色条 */}
          <span className={`w-1 shrink-0 self-stretch rounded-sm ${bar}`} />
          {/* kind 中文标签 */}
          <span
            className={`shrink-0 font-disp font-semibold tracking-wider text-[9px] px-1 py-0.5 rounded-sm border border-line/60 bg-white/[0.04] ${color}`}
          >
            {label}
          </span>
          {/* 主文本 */}
          <span className="text-ink2 min-w-0 truncate whitespace-nowrap">{main}</span>
          {/* 耗时 */}
          {n.durationMs !== undefined && (
            <span className="ml-auto shrink-0 font-mono text-[10px] text-muted">{fmtDur(n.durationMs)}</span>
          )}
        </div>
        {sub && (
          <div
            className="text-[11px] text-muted truncate"
            style={{ paddingLeft: 8 + n.depth * 18 + 16 }}
            title={sub}
          >
            {sub}
          </div>
        )}
        {collapsible && open && n.children.length > 0 && (
          <div className="relative">
            {/* 子树连接线 */}
            <span className="absolute left-[11px] top-0 bottom-0 w-px bg-line/50" />
            <div style={{ marginLeft: 8 + n.depth * 18 + 11 }} className="pl-3">
              {n.children.map(renderNode)}
            </div>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto px-2 py-1 pb-4 bg-panel/60">
      <div className="px-1 pb-1 text-[9px] font-disp tracking-wider text-muted/70">
        运行树: 计划 → 思考 → 工具(成功/失败·耗时), 父级可折叠
      </div>
      {roots.map(renderNode)}
    </div>
  )
}

// ---- 消息流(默认视图) ----
function MessageStream({ query }: { query: string }) {
  const log = useStore((s) => s.log)
  const ref = useRef<HTMLDivElement>(null)
  const stick = useRef(true)

  // 粘性滚动: 只在用户位于底部时自动跟随; 上翻看历史不被拉回(修原版无条件吸底)。
  const onScroll = () => {
    const el = ref.current
    if (!el) return
    stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48
  }

  // 搜索过滤: 命中事件文本或工具名(粘性滚动仍跟随全量日志增长, 不受过滤影响)。
  const q = query.trim().toLowerCase()
  const shown = useMemo(
    () => (q ? log.filter((l) => l.text.toLowerCase().includes(q) || (l.meta?.tool ?? '').toLowerCase().includes(q)) : log),
    [log, q],
  )

  useEffect(() => {
    if (stick.current) ref.current?.scrollTo(0, ref.current.scrollHeight)
  }, [log.length])

  return (
    <div ref={ref} onScroll={onScroll} className="h-full overflow-auto px-4 pb-4 bg-panel/60">
      {shown.length === 0 && (
        <div className="px-2.5 py-6 text-muted text-xs text-center">{q ? '无匹配事件' : '等待事件流…'}</div>
      )}
      {shown.map((l) => {
        const meta = l.meta
        const border = BORDER[l.kind] ?? 'border-l-line'
        const color = KIND[l.kind] ?? 'text-muted'
        const label = EVENT_LABELS[l.kind] ?? l.kind

        // step: 首行「思考 L{n} · {tool}」+ 缩进第二行「▍推理 {why}」。
        if (l.kind === 'step') {
          // 徽标「思考」+ 首行 L{n} · {tool} = 设计要求的「思考 L{n} · {tool}」。
          return (
            <div key={l.id} className={`px-2.5 py-2 my-1 border-l-2 ${border} bg-white/[0.04] text-[12.5px] leading-relaxed whitespace-pre-wrap break-all rounded-r-sm`}>
              <div>
                <span className={`font-disp font-semibold tracking-wider text-[10px] mr-1.5 ${color}`}>{label}</span>
                <span className="text-muted">
                  L{meta?.level ?? '?'} · {meta?.tool ?? '—'}
                </span>
              </div>
              <div className="mt-1 pl-3 text-signal border-l border-line/60">
                ▍推理 {meta?.why || '—'}
              </div>
            </div>
          )
        }

        // plan: 高亮块(signal 边框), 整段展示计划 rationale。
        if (l.kind === 'plan') {
          return (
            <div key={l.id} className={`px-2.5 py-2 my-1 border-l-2 ${border} bg-signal/[0.06] text-[12.5px] leading-relaxed whitespace-pre-wrap break-all rounded-r-sm`}>
              <div>
                <span className={`font-disp font-semibold tracking-wider text-[10px] mr-1.5 ${color}`}>{label}</span>
                <span className="text-muted">共 {meta?.count ?? '?'} 步行动</span>
              </div>
              {meta?.rationale && (
                <div className="mt-1 pl-3 text-ink2 border-l border-signal/40">▍{meta.rationale}</div>
              )}
            </div>
          )
        }

        return (
          <div key={l.id} className={`px-2.5 py-2 my-1 border-l-2 ${border} bg-white/[0.04] text-[12.5px] leading-relaxed whitespace-pre-wrap break-all rounded-r-sm`}>
            <span className={`font-disp font-semibold tracking-wider text-[10px] mr-1.5 ${color}`}>{label}</span>
            <span className="text-muted">{l.text}</span>
          </div>
        )
      })}
    </div>
  )
}

export function SignalStream() {
  const log = useStore((s) => s.log)
  const [mode, setMode] = useState<ViewMode>('D') // 默认 = 消息流
  // 事件流搜索词(Ctrl+F 聚焦 #signal-search)。
  const [query, setQuery] = useState('')

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* 三视图切换: M 消息流 / T 瀑布树 / D 默认 + 事件流搜索框 */}
      <div className="flex items-center gap-1.5 px-4 pt-2 pb-1.5 shrink-0 border-b border-line/40">
        {MODES.map((m) => (
          <button
            key={m.id}
            onClick={() => setMode(m.id)}
            title={m.title}
            className={`px-2.5 py-1 text-[10px] font-disp tracking-wider rounded-sm border transition-colors ${
              mode === m.id
                ? 'border-live/60 bg-live/10 text-live'
                : 'border-line bg-panel text-muted hover:text-ink2 hover:border-line/70'
            }`}
          >
            {m.id}·{m.label}
          </button>
        ))}
        <span className="flex-1" />
        <input
          id="signal-search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="过滤事件流 (Ctrl+F)"
          spellCheck={false}
          title="按事件文本或工具名过滤(Ctrl+F 聚焦)"
          className="w-40 bg-panel border border-line text-ink2 text-[11px] px-2.5 py-1 rounded-sm font-mono outline-none focus:border-signal placeholder:text-ghost"
        />
      </div>
      <div className="flex-1 min-h-0">
        {mode === 'T' ? <WaterfallTree log={log} /> : <MessageStream query={query} />}
      </div>
    </div>
  )
}
