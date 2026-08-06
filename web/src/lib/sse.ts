import { useEffect } from 'react'
import { useStore, parseEvent } from '../store'

// useSSE —— 订阅后端 /events, 把每条事件灌进 store。断线由浏览器 EventSource 自动重连。
export function useSSE() {
  const ingest = useStore((s) => s.ingest)
  useEffect(() => {
    const es = new EventSource('/events')
    es.onmessage = (m) => {
      let raw: unknown
      try {
        raw = JSON.parse(m.data)
      } catch {
        return /* 忽略半包/心跳 */
      }
      const e = parseEvent(raw)
      if (!e) return
      try {
        ingest(e)
      } catch (err) {
        console.error('[SSE] ingest failed for event', e?.kind, err)
      }
    }
    es.onerror = () => {
      // EventSource 自动重连, 这里只打日志
      console.warn('[SSE] connection error, waiting for auto-reconnect')
    }
    return () => es.close()
  }, [ingest])
}
