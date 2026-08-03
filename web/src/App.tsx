import { useSSE } from './lib/sse'
import { ChatSidebar } from './components/ChatSidebar'
import { ChatView } from './components/ChatView'
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

// 对话式 AI 助手布局(ChatGPT/Claude 风格):
//   左侧边栏(会话历史) + 中间对话消息流 + 右侧攻击图可视化(动画更新)。
export default function App() {
  useSSE()
  const [activeTab, setActiveTab] = useState<Tab>('campaign')

  useEffect(() => {
    const onKey = (raw: unknown) => {
      const ev = raw as { ctrlKey?: boolean; metaKey?: boolean; altKey?: boolean; shiftKey?: boolean; key?: string }
      if (!(ev.ctrlKey || ev.metaKey) || ev.altKey || ev.shiftKey) return
      const idx = Number(ev.key)
      if (idx >= 1 && idx <= 5) {
        setActiveTab((['campaign', 'tools', 'workflows', 'reports', 'settings'] as Tab[])[idx - 1])
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="flex h-screen overflow-hidden">
      <ErrorBoundary>
        <ChatSidebar activeTab={activeTab} onTab={(t) => setActiveTab(t as Tab)} />
      </ErrorBoundary>

      {/* 主区域 */}
      <div className="flex-1 flex flex-col min-w-0 min-h-0">
        {/* 对话视图: 消息流 + 右侧攻击图 */}
        <main className={`flex-1 min-h-0 ${activeTab === 'campaign' ? '' : 'hidden'}`}>
          <div className="h-full grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_400px]">
            <section className="min-h-0 min-w-0 flex flex-col border-r border-line">
              <ErrorBoundary>
                <ChatView />
              </ErrorBoundary>
            </section>
            <section className="hidden lg:flex flex-col min-h-0 border-line">
              <div className="text-[10px] font-disp tracking-[2.5px] uppercase text-muted px-3 pt-2 pb-1 flex items-center justify-between">
                <span>攻击图 · 实时</span>
                <span className="text-ghost lowercase tracking-normal font-mono">主路径高亮流动</span>
              </div>
              <div className="relative flex-1 min-h-0">
                <ErrorBoundary>
                  <AttackGraph />
                </ErrorBoundary>
                <ErrorBoundary>
                  <EvidenceDrawer />
                </ErrorBoundary>
              </div>
              <div className="max-h-[30%] min-h-0 overflow-auto">
                <ErrorBoundary>
                  <FindingsTable />
                </ErrorBoundary>
              </div>
            </section>
          </div>
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
      </div>

      <ErrorBoundary>
        <HitlModal />
      </ErrorBoundary>
    </div>
  )
}
