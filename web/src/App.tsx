import { useSSE } from './lib/sse'
import { Header } from './components/Header'
import { KpiPanel } from './components/KpiPanel'
import { SignalStream } from './components/SignalStream'
import { AttackGraph } from './components/AttackGraph'
import { EvidenceDrawer } from './components/EvidenceDrawer'
import { HitlModal } from './components/HitlModal'
import { ToolManager } from './components/ToolManager'
import { WorkflowManager } from './components/WorkflowManager'
import { ReportViewer } from './components/ReportViewer'
import { SettingsPanel } from './components/SettingsPanel'
import { FindingsTable } from './components/FindingsTable'
import { ErrorBoundary } from './components/ErrorBoundary'
import { useEffect, useState } from 'react'

type Tab = 'campaign' | 'tools' | 'workflows' | 'reports' | 'settings'

const TABS: { id: Tab; label: string }[] = [
  { id: 'campaign', label: '战役控制台' },
  { id: 'tools', label: '工具管理' },
  { id: 'workflows', label: '工作流模板' },
  { id: 'reports', label: '报告' },
  { id: 'settings', label: '设置' },
]

export default function App() {
  useSSE()
  const [activeTab, setActiveTab] = useState<Tab>('campaign')

  // 键盘快捷键:
  //   Ctrl/Cmd+1..5 → 切换 5 个 Tab(阻止浏览器原生切标签行为)。
  //   Ctrl/Cmd+F     → 事件流区聚焦搜索框(存在且可见时; 否则放行浏览器查找)。
  // 注: 用结构化收窄而非 DOM 全局类型名(KeyboardEvent/HTMLInputElement),
  //     以避开 eslint.config 里手写 DOM 全局白名单之外的名字。
  useEffect(() => {
    const onKey = (raw: unknown) => {
      const ev = raw as {
        ctrlKey?: boolean
        metaKey?: boolean
        altKey?: boolean
        shiftKey?: boolean
        key?: string
        preventDefault?: () => void
      }
      if (!(ev.ctrlKey || ev.metaKey) || ev.altKey || ev.shiftKey) return
      const idx = Number(ev.key)
      if (idx >= 1 && idx <= TABS.length) {
        ev.preventDefault?.()
        setActiveTab(TABS[idx - 1].id)
        return
      }
      if ((ev.key ?? '').toLowerCase() === 'f') {
        const el = document.getElementById('signal-search')
        if (el && el.offsetWidth > 0) {
          ev.preventDefault?.()
          el.focus()
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <ErrorBoundary>
        <Header />
      </ErrorBoundary>

      {/* Tab Navigation */}
      <div className="border-b border-line bg-panel2">
        <div className="flex gap-4 px-4">
          {TABS.map((t) => (
            <button
              key={t.id}
              onClick={() => setActiveTab(t.id)}
              className={`py-3 px-4 border-b-2 transition-colors ${
                activeTab === t.id
                  ? 'border-live text-live font-semibold'
                  : 'border-transparent text-muted hover:text-ink2'
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab Content —— 保活渲染(display 切换而非卸载):
          修原版切 tab 卸载组件, 攻击图缩放/抽屉状态全丢 */}
      <main
        className={`flex-1 grid grid-cols-1 md:grid-cols-[minmax(340px,38%)_1fr] min-h-0 ${
          activeTab === 'campaign' ? '' : 'hidden'
        }`}
      >
        <section className="flex flex-col min-h-0 border-b md:border-b-0 md:border-r border-line bg-panel2">
          <ErrorBoundary>
            <KpiPanel />
          </ErrorBoundary>
          <ErrorBoundary>
            <SignalStream />
          </ErrorBoundary>
        </section>
        <section className="relative min-h-0 flex flex-col">
          <div className="relative min-h-0 flex flex-1">
            <div className="flex-1 relative min-w-0">
              <ErrorBoundary>
                <AttackGraph />
              </ErrorBoundary>
            </div>
            <ErrorBoundary>
              <EvidenceDrawer />
            </ErrorBoundary>
          </div>
          <ErrorBoundary>
            <FindingsTable />
          </ErrorBoundary>
        </section>
      </main>

      <main className={`flex-1 overflow-auto bg-panel2 ${activeTab === 'tools' ? '' : 'hidden'}`}>
        <ErrorBoundary>
          <ToolManager />
        </ErrorBoundary>
      </main>

      <main className={`flex-1 overflow-auto bg-panel2 ${activeTab === 'workflows' ? '' : 'hidden'}`}>
        <ErrorBoundary>
          <WorkflowManager />
        </ErrorBoundary>
      </main>

      <main className={`flex-1 overflow-auto bg-panel2 ${activeTab === 'reports' ? '' : 'hidden'}`}>
        <ErrorBoundary>
          <ReportViewer />
        </ErrorBoundary>
      </main>

      <main className={`flex-1 overflow-auto bg-panel2 ${activeTab === 'settings' ? '' : 'hidden'}`}>
        <ErrorBoundary>
          <SettingsPanel />
        </ErrorBoundary>
      </main>

      <ErrorBoundary>
        <HitlModal />
      </ErrorBoundary>
    </div>
  )
}
