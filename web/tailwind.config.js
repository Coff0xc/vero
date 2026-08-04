/** @type {import('tailwindcss').Config} */
// Vero 设计 token —— 「珠光白」亮色主题(Google Material 风)。
// token 名稳定(signal/live/alert 等语义不变), 全组件类名零改动即可全局换肤。
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#ffffff', // 最浅底(画布/代码块)
        panel: '#fdfcf9', // 珠光白面板底
        panel2: '#f5f4f0', // 次面板/嵌套底(微暖灰)
        line: '#e3e1db', // 描边(柔和)
        signal: '#1a73e8', // 主强调: Google 蓝(聚焦/高亮/主路径/MITRE)
        live: '#188038', // 成功/已证实: Google 绿
        alert: '#d93025', // 危险/失败: Google 红
        violet: '#7627bb', // 洞察/分析: Google 紫
        warn: '#b06000', // 运行中/警告: 琥珀(白底对比度)
        ghost: '#5f6368', // 弱文字/假设(灰)
        ink2: '#202124', // 主文字(近黑)
        muted: '#5f6368', // 次文字(Google 灰)
      },
      fontFamily: {
        disp: ['"Segoe UI"', '"Microsoft YaHei"', 'system-ui', 'sans-serif'],
        mono: ['ui-monospace', '"Cascadia Mono"', 'Consolas', '"Courier New"', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 2px rgba(60, 64, 67, 0.14), 0 4px 12px rgba(60, 64, 67, 0.10)',
        'glow-live': '0 1px 3px rgba(24, 128, 56, 0.22)',
        'glow-alert': '0 1px 3px rgba(217, 48, 37, 0.22)',
        'glow-signal': '0 1px 4px rgba(26, 115, 232, 0.22)',
        'glow-violet': '0 1px 4px rgba(118, 39, 187, 0.20)',
        'inner-line': 'inset 0 1px 0 rgba(60, 64, 67, 0.04)',
      },
    },
  },
  plugins: [],
}
