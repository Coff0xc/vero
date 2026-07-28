import { useState } from 'react'
import { useStore } from '../store'

export function Header() {
  const status = useStore((s) => s.status)
  const reset = useStore((s) => s.reset)
  const [target, setTarget] = useState('http://localhost:3000')

  const start = async () => {
    reset(target)
    await fetch('/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target }),
    })
  }

  return (
    <header className="flex items-center gap-4 px-5 py-3 border-b border-line bg-gradient-to-b from-panel to-panel2">
      <span
        className={`w-2 h-2 rounded-full ${status === 'running' ? 'bg-live animate-pulse' : 'bg-ghost'}`}
        style={{ boxShadow: status === 'running' ? '0 0 8px #4ec9b0' : 'none' }}
        title="agent 状态"
      />
      <span className="font-disp font-bold text-lg tracking-[3px]">
        RED<span className="text-signal">CELL</span>
      </span>
      <span className="hidden md:inline font-disp text-[11px] tracking-[2px] text-muted uppercase">
        自主渗透作战台
      </span>
      <span className="flex-1" />
      <input
        value={target}
        onChange={(e) => setTarget(e.target.value)}
        placeholder="目标 URL / host"
        spellCheck={false}
        className="bg-panel2 border border-line text-ink2 text-xs px-2.5 py-1.5 rounded-sm w-52 sm:w-64 font-mono outline-none focus:border-signal"
      />
      <button
        onClick={start}
        className="font-disp font-semibold tracking-wider text-sm text-signal border border-signal px-4 py-2 rounded-sm uppercase hover:bg-signal hover:text-ink transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-live"
      >
        ▷ 启动战役
      </button>
    </header>
  )
}
