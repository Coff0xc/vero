import { useEffect } from 'react'
import { useSSE } from './lib/sse'
import { useStore, type Tab } from './store'
import { TopBar } from './components/TopBar'
import { IconRail } from './components/IconRail'
import { HistoryDrawer } from './components/HistoryDrawer'
import { CommandPalette } from './components/CommandPalette'
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
import { IconFullscreen, IconClose } from './components/Icon'

const TABS: Tab[] = ['campaign', 'tools', 'workflows', 'reports', 'settings']

// 指挥台式布局:
//   顶部状态栏(阶段/KPI/引擎/⌘K) + 左图标轨 + 中央对话 + 右攻击图(可全屏)。
//   历史战役 -> 抽屉; 全局动作 -> ⌘K 命令面板。
export default function App() {
  useSSE()
  const activeTab = useStore((s) => s.activeTab)
  const setTab = useStore((s) => s.setTab)
  const graphFull = useStore((s) => s.graphFull)
  const toggleGraphFull = useStore((s) => s.toggleGraphFull)
  const togglePalette = useStore((s) => s.togglePalette)
  const toggleHistory = useStore((s) => s.toggleHistory)

  // 全局快捷键:
  //   Ctrl+K 命令面板 | Ctrl+H 历史抽屉 | Ctrl+1..5 切分区 | G 图全屏(非输入态) | Esc 关浮层
  useEffect(() => {
    const onKey = (raw: unknown) => {
      const ev = raw as KeyboardEvent
      const typing = /^(INPUT|TEXTAREA|SELECT)$/.test((ev.target as HTMLElement)?.tagName ?? '')
      if ((ev.ctrlKey || ev.metaKey) && !ev.altKey && !ev.shiftKey) {
        const k = ev.key.toLowerCase()
        if (k === 'k') { ev.preventDefault(); togglePalette(); return }
        if (k === 'h') { ev.preventDefault(); toggleHistory(); return }
        const idx = Number(ev.key)
        if (idx >= 1 && idx <= 5) { ev.preventDefault(); setTab(TABS[idx - 1]); return }
      }
      if (ev.key === 'Escape') {
        togglePalette(false)
        toggleHistory(false)
        if (graphFull) toggleGraphFull(false)
        return
      }
      // 单键快捷: 仅在非输入态生效。
      if (!typing && !ev.ctrlKey && !ev.metaKey && !ev.altKey) {
        if (ev.key === 'g' || ev.key === 'G') {
          if (activeTab === 'campaign') toggleGraphFull()
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeTab, graphFull, setTab, togglePalette, toggleHistory, toggleGraphFull])

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <TopBar />
      <div className="flex-1 flex min-h-0">
        <IconRail />

        {/* 主区域 */}
        <div className="flex-1 flex flex-col min-w-0 min-h-0">
          {/* 对话作战: 消息流 + 右侧攻击图(全屏时独占) */}
          <main className={`flex-1 min-h-0 ${activeTab === 'campaign' ? '' : 'hidden'}`}>
            {graphFull ? (
              <div className="h-full relative">
                <ErrorBoundary>
                  <AttackGraph />
                </ErrorBoundary>
                <button
                  onClick={() => toggleGraphFull(false)}
                  className="absolute right-4 top-3 z-20 inline-flex items-center gap-1.5 text-[11px] text-muted hover:text-ink2 border border-line bg-ink/80 rounded-md px-2.5 py-1.5 transition-colors"
                  title="退出全屏 (Esc / G)"
                >
                  <IconClose size={12} /> 退出全屏
                </button>
                <ErrorBoundary>
                  <EvidenceDrawer />
                </ErrorBoundary>
              </div>
            ) : (
              <div className="h-full grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_420px]">
                <section className="min-h-0 min-w-0 flex flex-col border-r border-line/60">
                  <ErrorBoundary>
                    <ChatView />
                  </ErrorBoundary>
                </section>
                <section className="hidden lg:flex flex-col min-h-0 border-line bg-panel2/30">
                  <div className="text-[10px] font-disp uppercase text-muted px-3 pt-2.5 pb-1.5 flex items-center justify-between border-b border-line/50">
                    <span className="section-title">攻击图 · 实时</span>
                    <button
                      onClick={() => toggleGraphFull(true)}
                      className="inline-flex items-center gap-1 text-ghost hover:text-signal transition-colors font-mono text-[10px]"
                      title="全屏 (G)"
                    >
                      <IconFullscreen size={11} /> 全屏
                    </button>
                  </div>
                  <div className="relative flex-1 min-h-0 flex">
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
            )}
          </main>

          <main className={`flex-1 overflow-auto bg-panel2/40 ${activeTab === 'tools' ? '' : 'hidden'}`}>
            <ErrorBoundary>
              <ToolManager />
            </ErrorBoundary>
          </main>

          <main className={`flex-1 overflow-auto bg-panel2/40 ${activeTab === 'workflows' ? '' : 'hidden'}`}>
            <ErrorBoundary>
              <WorkflowManager />
            </ErrorBoundary>
          </main>

          <main className={`flex-1 overflow-auto bg-panel2/40 ${activeTab === 'reports' ? '' : 'hidden'}`}>
            <ErrorBoundary>
              <ReportViewer />
            </ErrorBoundary>
          </main>

          <main className={`flex-1 overflow-auto bg-panel2/40 ${activeTab === 'settings' ? '' : 'hidden'}`}>
            <ErrorBoundary>
              <SettingsPanel />
            </ErrorBoundary>
          </main>
        </div>
      </div>

      {/* 浮层 */}
      <ErrorBoundary>
        <HistoryDrawer />
      </ErrorBoundary>
      <CommandPalette />
      <ErrorBoundary>
        <HitlModal />
      </ErrorBoundary>
    </div>
  )
}
