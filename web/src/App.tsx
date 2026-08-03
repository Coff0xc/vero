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
import { ErrorBoundary } from './components/ErrorBoundary'
import { useState } from 'react'

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
        <section className="relative min-h-0 flex">
          <div className="flex-1 relative min-w-0">
            <ErrorBoundary>
              <AttackGraph />
            </ErrorBoundary>
          </div>
          <ErrorBoundary>
            <EvidenceDrawer />
          </ErrorBoundary>
        </section>      </main>

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
