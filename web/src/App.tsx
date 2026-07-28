import { useSSE } from './lib/sse'
import { Header } from './components/Header'
import { KpiPanel } from './components/KpiPanel'
import { SignalStream } from './components/SignalStream'
import { AttackGraph } from './components/AttackGraph'
import { EvidenceDrawer } from './components/EvidenceDrawer'
import { HitlModal } from './components/HitlModal'

export default function App() {
  useSSE()
  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <Header />
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
      <HitlModal />
    </div>
  )
}
