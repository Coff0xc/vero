import { useStore } from '../store'

// 战役动作(命令面板 / 对话输入框共用)—— 统一入口, 错误可见。
// 先请求后端再重置视图: busy 拒绝时保留当前窗口内容(不先清空再失败)。

// 目标判定: scheme URL / 点分域名 / localhost / IPv4(均可带端口与路径)。
// 不匹配裸单词(避免把 "stop" 等指令误判为目标)。
export const TARGET_RE = /^(https?:\/\/\S+|([\w-]+\.)+[a-zA-Z]{2,}(:\d+)?(\/\S*)?|localhost(:\d+)?(\/\S*)?|(\d{1,3}\.){3}\d{1,3}(:\d+)?(\/\S*)?)$/i

// 停止类指令(优先于目标判定)。
export const STOP_RE = /^(停止|取消|停|stop|cancel|别再)\s*$/i

export interface ActionResult {
  ok: boolean
  err?: string
}

// startCampaign —— 请求后端开战, 受理后重置视图; 返回是否成功受理。
export async function startCampaign(target: string): Promise<ActionResult> {
  const t = target.trim()
  if (!t) return { ok: false, err: '目标为空' }
  try {
    const r = await fetch('/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target: t }),
    })
    const body = (await r.json().catch(() => ({}))) as { ok?: boolean; err?: string }
    if (!r.ok || body.ok === false) return { ok: false, err: body.err ?? `HTTP ${r.status}` }
    useStore.getState().reset(t)
    return { ok: true }
  } catch (err) {
    return { ok: false, err: String(err) }
  }
}

// cancelCampaign —— 停止当前战役(幂等, 无战役时后端空转)。
export async function cancelCampaign(): Promise<void> {
  try {
    await fetch('/cancel', { method: 'POST' })
  } catch {
    /* 网络失败静默: 状态灯会如实反映 */
  }
}
