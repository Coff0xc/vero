import { useSSE } from './lib/sse'
import { Header } from './components/Header'
import { KpiPanel } from './components/KpiPanel'
import { SignalStream } from './components/SignalStream'
import { AttackGraph } from './components/AttackGraph'
import { EvidenceDrawer } from './components/EvidenceDrawer'
import { HitlModal } from './components/HitlModal'
import { ToolManager } from './components/ToolManager'
import { WorkflowManager } from './components/WorkflowManager'
import { useState } from 'react'

export default function App() {
  useSSE()
  const [activeTab, setActiveTab] = useState<'campaign' | 'tools' | 'workflows'>('campaign')

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <Header />

      {/* Tab Navigation */}
      <div className="border-b border-line bg-panel2">
        <div className="flex gap-4 px-4">
          <button
            onClick={() => setActiveTab('campaign')}
            className={`py-3 px-4 border-b-2 ${
              activeTab === 'campaign'
                ? 'border-blue-500 text-blue-600 font-semibold'
                : 'border-transparent text-gray-600 hover:text-gray-900'
            }`}
          >
            战役控制台
          </button>
          <button
            onClick={() => setActiveTab('tools')}
            className={`py-3 px-4 border-b-2 ${
              activeTab === 'tools'
                ? 'border-blue-500 text-blue-600 font-semibold'
                : 'border-transparent text-gray-600 hover:text-gray-900'
            }`}
          >
            工具管理
          </button>
          <button
            onClick={() => setActiveTab('workflows')}
            className={`py-3 px-4 border-b-2 ${
              activeTab === 'workflows'
                ? 'border-blue-500 text-blue-600 font-semibold'
                : 'border-transparent text-gray-600 hover:text-gray-900'
            }`}
          >
            工作流模板
          </button>
        </div>
      </div>

      {/* Tab Content */}
      {activeTab === 'campaign' && (
        <main className="flex-1 grid grid-cols-1 md:grid-cols-[minmax(340px,38%)_1fr] min-h-0">
          <section className="flex flex-col min-h-0 border-b md:border-b-0 md:border-r border-line bg-panel2">
            <KpiPanel />
            <SignalStream />
          </section>
          <section className="relative min-h-0">
            <AttackGraph />
            <EvidenceDrawer />
          </section>
        </main>
      )}

      {activeTab === 'tools' && (
        <main className="flex-1 overflow-auto bg-gray-50">
          <ToolManager />
        </main>
      )}

      {activeTab === 'workflows' && (
        <main className="flex-1 overflow-auto bg-gray-50">
          <WorkflowManager />
        </main>
      )}

      <HitlModal />
    </div>
  )
}
