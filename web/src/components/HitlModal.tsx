import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { LEVEL_ZH } from '../lib/i18n'
import { IconAlert } from './Icon'

// 工具级别 → 中文口语化: L0=侦察级, L1=扫描级, L2=凭证级, L3=利用级, L4=破坏级。
function levelZh(level: number): string {
  return LEVEL_ZH[level] ?? `L${level}`
}

// 解析 JSON 文本; 失败返回 undefined。
function parseJson(text: string): unknown {
  try {
    return JSON.parse(text) as unknown
  } catch {
    return undefined
  }
}

export function HitlModal() {
  const hitl = useStore((s) => s.hitl)
  const clear = useStore((s) => s.clearHitl)
  // 参数编辑区: 初始填充 hitl.args 的美化 JSON, 操作员可改写。
  const [text, setText] = useState(() => (hitl ? JSON.stringify(hitl.args, null, 2) : ''))
  const [parseError, setParseError] = useState<string | null>(null)

  // 新授权请求到达时, 用原始参数重置编辑区与错误态。
  useEffect(() => {
    if (hitl) {
      setText(JSON.stringify(hitl.args, null, 2))
      setParseError(null)
    }
  }, [hitl])

  if (!hitl) return null

  const decide = async (approved: boolean) => {
    // /approve 只读 {key, approved}; 不改写参数。
    await fetch('/approve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: hitl.key, approved }),
    })
    clear()
  }

  const onEdit = (v: string) => {
    setText(v)
    setParseError(parseJson(v) === undefined ? 'JSON 格式错误, 无法放行' : null)
  }

  // 编辑区当前内容是否与原参数不一致(含非法 JSON)。
  const parsed = parseJson(text)
  const dirty = parsed === undefined || JSON.stringify(parsed) !== JSON.stringify(hitl.args)

  // 「以修改参数放行」: 后端 /approve 目前只收 {key, approved}, 仍以原参数执行;
  // 参数编辑仅作预览核对。附带 args 字段以便后端未来支持参数改写时无缝升级。
  const decideEdited = async () => {
    if (parsed === undefined) {
      setParseError('JSON 格式错误, 无法放行')
      return
    }
    await fetch('/approve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: hitl.key, approved: true, args: parsed }),
    })
    clear()
  }

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-white border border-alert/40 border-t-[3px] rounded-lg px-6 py-5 max-w-md w-full shadow-card card-pop">
        <div className="font-disp font-bold tracking-wider text-sm text-alert uppercase mb-1 flex items-center gap-2">
          <IconAlert size={15} />
          需要授权
        </div>
        <div className="text-[11px] text-muted mb-4">agent 请求执行高危动作, 等待操作员裁决</div>

        <div className="text-sm leading-relaxed">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="px-2 py-0.5 text-[10px] rounded-sm border border-alert text-alert font-disp tracking-wider">
              {levelZh(hitl.level)}
            </span>
            <code className="text-signal font-mono">{hitl.tool}</code>
          </div>

          {/* 可编辑参数区 */}
          <div className="mt-3">
            <div className="text-xs text-muted mb-1 flex items-center justify-between">
              <span>
                调用参数 <span className="text-ghost normal-case tracking-normal">(可编辑 JSON)</span>
              </span>
            </div>
            <textarea
              value={text}
              onChange={(e) => onEdit(e.target.value)}
              spellCheck={false}
              rows={Math.max(4, Math.min(10, text.split('\n').length + 1))}
              aria-label="可编辑的调用参数 JSON"
              className="w-full bg-panel2 border border-line rounded-sm px-3 py-2 font-mono text-[11.5px] text-ink2 outline-none focus:border-signal resize-y whitespace-pre break-all"
            />
            {dirty ? (
              <div className="mt-1.5 text-[11px] text-signal leading-relaxed">
                参数编辑为预览, 当前以原参数执行
              </div>
            ) : (
              <div className="mt-1.5 text-[11px] text-ghost leading-relaxed">
                可修改参数后「以修改参数放行」; 后端暂不支持改写, 放行仍以原参数执行
              </div>
            )}
            {parseError && <div className="mt-1.5 text-[11px] text-alert">{parseError}</div>}
          </div>

          <div className="mt-3">
            <div className="text-xs text-muted mb-1">操作理由</div>
            <div className="text-[12.5px] text-ink2 leading-relaxed border-l-2 border-l-signal pl-2.5">
              {hitl.why || '—'}
            </div>
          </div>
        </div>

        <div className="mt-4 space-y-2.5">
          <div className="flex gap-2.5">
            <button
              onClick={() => decide(true)}
              className="btn-accent flex-1 font-disp font-semibold tracking-wider text-sm py-2.5 rounded-md uppercase"
            >
              批准执行
            </button>
            <button
              onClick={() => decide(false)}
              className="btn-danger flex-1 font-disp font-semibold tracking-wider text-sm py-2.5 rounded-md uppercase border border-muted/60 text-muted hover:border-alert hover:text-alert"
            >
              拒绝
            </button>
          </div>
          {dirty && (
            <button
              onClick={decideEdited}
              disabled={parsed === undefined}
              title={parsed === undefined ? '参数 JSON 非法, 修正后才能放行' : '仍以原参数执行(编辑为预览)'}
              className={`w-full font-disp font-semibold tracking-wider text-sm py-2.5 rounded-md uppercase border transition-all ${
                parsed === undefined
                  ? 'border-line text-ghost cursor-not-allowed'
                  : 'border-signal text-signal hover:bg-signal hover:text-ink hover:shadow-glow-signal'
              }`}
            >
              以修改参数放行
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
