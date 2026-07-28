import { useEffect } from 'react'
import { useStore } from '../store'

// useSSE —— 订阅后端 /events, 把每条事件灌进 store。断线由浏览器 EventSource 自动重连。
export function useSSE() {
  const ingest = useStore((s) => s.ingest)
  useEffect(() => {
    const es = new EventSource('/events')
    es.onmessage = (m) => {
      try {
        ingest(JSON.parse(m.data))
      } catch {
        /* 忽略半包/心跳 */
      }
    }
    return () => es.close()
  }, [ingest])
}
