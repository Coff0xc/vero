import { useState } from 'react'
import { useStore } from '../store'

export function Header() {
  const status = useStore((s) => s.status)
  const reset = useStore((s) => s.reset)
  const engineLabel = useStore((s) => s.engineLabel)
  const temperature = useStore((s) => s.temperature)
  const [target, setTarget] = useState('http://localhost:3000')

  const start = async () => {
    reset(target)
    try {
      await fetch('/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target }),
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

  return (
    <header className="flex items-center gap-4 px-5 py-3 border-b border-line bg-gradient-to-b from-panel to-panel2">
      <span
        className={`w-2 h-2 rounded-full ${status === 'running' ? 'bg-live animate-pulse' : 'bg-ghost'}`}
        style={{ boxShadow: status === 'running' ? '0 0 8px #4ec9b0' : 'none' }}
        title="agent 状态"
      />
      <span className="font-disp font-bold text-lg tracking-[3px]">
        VERO<span className="text-signal">·</span>
      </span>
      <span className="hidden md:inline font-disp text-[11px] tracking-[2px] text-muted uppercase">
        自主渗透作战台
      </span>
      {/* 引擎 / 思考强度指示 chip(读 store: 'engine' SSE 事件 + /api/config 温度) */}
      <span
        className="hidden sm:inline-flex items-center gap-1.5 border border-line rounded-sm px-2.5 py-1 text-[10px] font-disp tracking-wider text-muted bg-panel"
        title="当前决策引擎与思考强度(设置页可调整)"
      >
        <i className="w-1.5 h-1.5 rounded-full bg-signal inline-block" />
        <span className="text-ink2">{engineLabel || '引擎 —'}</span>
        <span className="text-ghost">·</span>
        <span className="text-ghost">思考 {temperature.toFixed(2)}</span>
      </span>
      <span className="flex-1" />
      <input
        value={target}
        onChange={(e) => setTarget(e.target.value)}
        placeholder="目标 URL / host"
        spellCheck={false}
        disabled={status === 'running'}
        className="bg-panel2 border border-line text-ink2 text-xs px-2.5 py-1.5 rounded-sm w-52 sm:w-64 font-mono outline-none focus:border-signal disabled:opacity-50"
      />
      {status === 'running' ? (
        <button
          onClick={cancel}
          className="font-disp font-semibold tracking-wider text-sm text-alert border border-alert px-4 py-2 rounded-sm uppercase hover:bg-alert hover:text-ink transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-alert"
        >
          ■ 停止
        </button>
      ) : (
        <button
          onClick={start}
          className="font-disp font-semibold tracking-wider text-sm text-signal border border-signal px-4 py-2 rounded-sm uppercase hover:bg-signal hover:text-ink transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-live"
        >
          ▷ 启动战役
        </button>
      )}
    </header>
  )
}
