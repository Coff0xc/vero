/** @type {import('tailwindcss').Config} */
// Vero 设计 token —— 「冷色霓虹作战台」主题。
// token 名稳定(signal/live/alert 等语义不变), 全组件类名零改动即可全局换肤。
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#05070c', // 最深底(画布/代码块)
        panel: '#0c1017', // 面板底
        panel2: '#080b11', // 次面板/嵌套底
        line: '#1b2432', // 描边
        signal: '#22d3ee', // 主强调: 青(聚焦/高亮/主路径/MITRE)
        live: '#34d399', // 成功/已证实: 翡翠
        alert: '#fb7185', // 危险/失败: 玫瑰红
        violet: '#a78bfa', // AI/反思/思考: 紫
        warn: '#fbbf24', // 运行中/警告: 琥珀
        ghost: '#54637a', // 弱文字/假设
        ink2: '#d7e1ec', // 主文字
        muted: '#7d8ea3', // 次文字
      },
      fontFamily: {
        disp: ['"Segoe UI"', '"Microsoft YaHei"', 'system-ui', 'sans-serif'],
        mono: ['ui-monospace', '"Cascadia Mono"', 'Consolas', '"Courier New"', 'monospace'],
      },
      boxShadow: {
        card: '0 4px 18px rgba(0, 0, 0, 0.4)',
        'glow-live': '0 0 14px rgba(52, 211, 153, 0.35)',
        'glow-alert': '0 0 14px rgba(251, 113, 133, 0.3)',
        'glow-signal': '0 0 14px rgba(34, 211, 238, 0.28)',
        'glow-violet': '0 0 14px rgba(167, 139, 250, 0.3)',
        'inner-line': 'inset 0 1px 0 rgba(255, 255, 255, 0.03)',
      },
    },
  },
  plugins: [],
}
