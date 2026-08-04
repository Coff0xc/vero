import { useEffect, useMemo, useRef, useState } from 'react'
import { useStore, type Tab } from '../store'
import { TARGET_RE, startCampaign, cancelCampaign } from '../lib/actions'
import {
  IconPlus, IconStop, IconChat, IconTools, IconWorkflow, IconReport,
  IconSettings, IconHistory, IconFullscreen, IconTarget, IconSearch, IconCommand,
  type IconComponent,
} from './Icon'

interface Cmd {
  id: string
  title: string
  Icon: IconComponent
  hint?: string // 右侧快捷键/说明
  run: () => void | Promise<void>
}

// 命令面板(Ctrl K)—— 全局动作中枢: 导航 / 战役控制 / 图全屏 / 节点搜索。
// 输入为 URL/IP 时动态置顶「发起战役」; 普通文本按标题模糊过滤。
export function CommandPalette() {
  const open = useStore((s) => s.paletteOpen)
  const toggle = useStore((s) => s.togglePalette)
  const status = useStore((s) => s.status)
  const setTab = useStore((s) => s.setTab)
  const reset = useStore((s) => s.reset)
  const toggleHistory = useStore((s) => s.toggleHistory)
  const toggleGraphFull = useStore((s) => s.toggleGraphFull)
  const graphFull = useStore((s) => s.graphFull)
  const setNodeQuery = useStore((s) => s.setNodeQuery)

  const [q, setQ] = useState('')
  const [cursor, setCursor] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (open) {
      setQ('')
      setCursor(0)
      // 等动画帧再聚焦, 避免渲染前 focus 失败。
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [open])

  const cmds = useMemo<Cmd[]>(() => {
    const go = (t: Tab) => () => { setTab(t); toggle(false) }
    const list: Cmd[] = [
      { id: 'new', title: '新会话', Icon: IconPlus, hint: '', run: () => { reset(''); go('campaign')() } },
      ...(status === 'running'
        ? [{ id: 'stop', title: '停止当前战役', Icon: IconStop, hint: '', run: async () => { await cancelCampaign(); toggle(false) } }]
        : []),
      { id: 't-campaign', title: '转到: 对话作战', Icon: IconChat, hint: 'Ctrl+1', run: go('campaign') },
      { id: 't-tools', title: '转到: 工具管理', Icon: IconTools, hint: 'Ctrl+2', run: go('tools') },
      { id: 't-workflows', title: '转到: 工作流', Icon: IconWorkflow, hint: 'Ctrl+3', run: go('workflows') },
      { id: 't-reports', title: '转到: 报告', Icon: IconReport, hint: 'Ctrl+4', run: go('reports') },
      { id: 't-settings', title: '转到: 设置', Icon: IconSettings, hint: 'Ctrl+5', run: go('settings') },
      { id: 'history', title: '历史战役', Icon: IconHistory, hint: 'Ctrl+H', run: () => { toggle(false); toggleHistory(true) } },
      {
        id: 'graph-full',
        title: graphFull ? '退出攻击图全屏' : '攻击图全屏',
        Icon: IconFullscreen,
        hint: 'G',
        run: () => { setTab('campaign'); toggleGraphFull(); toggle(false) },
      },
    ]
    const query = q.trim()
    if (!query) return list
    // 动态动作: 目标是 URL/IP -> 直接开战; 其他文本 -> 节点搜索(跳到对话页)。
    const dyn: Cmd[] = []
    if (TARGET_RE.test(query)) {
      dyn.push({
        id: 'start', title: `发起战役: ${query}`, Icon: IconTarget,
        run: async () => { toggle(false); await startCampaign(query) },
      })
    }
    dyn.push({
      id: 'grep-node', title: `搜索节点: ${query}`, Icon: IconSearch,
      run: () => { setTab('campaign'); setNodeQuery(query); toggle(false) },
    })
    const low = query.toLowerCase()
    return [...dyn, ...list.filter((c) => c.title.toLowerCase().includes(low))]
  }, [q, status, graphFull, setTab, reset, toggle, toggleHistory, toggleGraphFull, setNodeQuery])

  // 键盘导航: ↑↓ 移动, Enter 执行, Esc 关闭。
  const onKey = (ev: { key: string; preventDefault: () => void }) => {
    if (ev.key === 'ArrowDown') { ev.preventDefault(); setCursor((c) => Math.min(c + 1, cmds.length - 1)) }
    else if (ev.key === 'ArrowUp') { ev.preventDefault(); setCursor((c) => Math.max(c - 1, 0)) }
    else if (ev.key === 'Enter') { ev.preventDefault(); void cmds[cursor]?.run() }
    else if (ev.key === 'Escape') { ev.preventDefault(); toggle(false) }
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-start justify-center pt-[14vh]" onClick={() => toggle(false)}>
      <div className="w-[min(560px,92vw)] glass border border-line rounded-xl shadow-card panel-in overflow-hidden" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center gap-2.5 px-4 border-b border-line/70">
          <span className="text-signal"><IconCommand size={14} /></span>
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => { setQ(e.target.value); setCursor(0) }}
            onKeyDown={onKey}
            placeholder="输入命令, 或粘贴目标 URL/IP 发起战役…"
            className="flex-1 bg-transparent outline-none py-3.5 text-[13.5px] text-ink2 placeholder:text-ghost/80 font-mono"
          />
          <span className="kbd">ESC</span>
        </div>
        <div className="max-h-[46vh] overflow-y-auto py-1.5">
          {cmds.length === 0 && <div className="px-4 py-3 text-[12px] text-ghost">无匹配命令</div>}
          {cmds.map((c, i) => (
            <button
              key={c.id}
              onMouseEnter={() => setCursor(i)}
              onClick={() => void c.run()}
              className={`w-full flex items-center gap-3 px-4 py-2.5 text-left text-[12.5px] transition-colors ${
                i === cursor ? 'bg-signal/10 text-ink2' : 'text-muted'
              }`}
            >
              <span className={i === cursor ? 'text-signal' : ''}>
                <c.Icon size={15} />
              </span>
              <span className="flex-1 truncate">{c.title}</span>
              {c.hint && <span className="kbd">{c.hint}</span>}
            </button>
          ))}
        </div>
        <div className="px-4 py-2 border-t border-line/60 flex items-center gap-3 text-[9.5px] text-ghost/90 font-mono">
          <span><span className="kbd">↑↓</span> 选择</span>
          <span><span className="kbd">Enter</span> 执行</span>
          <span className="ml-auto">目标=URL/IP 直接开战 · 其他文本=搜节点</span>
        </div>
      </div>
    </div>
  )
}
