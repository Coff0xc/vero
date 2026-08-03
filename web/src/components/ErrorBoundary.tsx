// ErrorBoundary —— 组件级错误护栏: 任何小组件崩溃只降级该区域,
// 不再整页白屏(修原版无任何边界, 一个小组件抛错全站不可用)。
import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: unknown) {
    console.error('[VERO] 组件崩溃, 已降级显示:', error)
  }

  private retry = () => this.setState({ hasError: false })

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <div className="flex flex-col items-center justify-center gap-3 h-full min-h-[120px] text-muted text-xs">
            <span>组件渲染失败, 数据流未受影响</span>
            <button
              onClick={this.retry}
              className="font-disp tracking-wider text-signal border border-signal px-4 py-1.5 rounded-sm uppercase hover:bg-signal hover:text-ink transition"
            >
              重新加载
            </button>
          </div>
        )
      )
    }
    return this.props.children
  }
}
