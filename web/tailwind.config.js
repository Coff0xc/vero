/** @type {import('tailwindcss').Config} */
// Vero 设计 token —— 「珍珠白」亮色主题。
// token 名稳定(signal/live/alert 等语义不变), 全组件类名零改动即可全局换肤。
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#ffffff', // 最浅底(画布/代码块)
        panel: '#fffdfa', // 珍珠白面板(微暖)
        panel2: '#f4f2ec', // 次面板/嵌套底(暖灰)
        line: '#e6e3d9', // 描边(柔和暖灰)
        signal: '#a8781c', // 主强调: 琥珀金(珍珠温润感, 聚焦/高亮/主路径)
        live: '#1a7f4b', // 成功/已证实: 翡翠绿(白底对比度)
        alert: '#cf3825', // 危险/失败: 朱红
        violet: '#7748b8', // 洞察/分析: 紫
        warn: '#a8781c', // 运行中/警告: 与琥珀统一
        ghost: '#8a8a80', // 弱文字/假设(暖灰)
        ink2: '#26241d', // 主文字(暖近黑)
        muted: '#6f6d62', // 次文字(暖灰)
        info: '#2e6da8', // 信息/链接: 静蓝
      },
      fontFamily: {
        disp: ['"Segoe UI"', '"Microsoft YaHei"', 'system-ui', 'sans-serif'],
        mono: ['ui-monospace', '"Cascadia Mono"', 'Consolas', '"Courier New"', 'monospace'],
      },
      boxShadow: {
        card: '0 1px 2px rgba(45, 41, 32, 0.08), 0 2px 10px rgba(45, 41, 32, 0.06)',
        pop: '0 4px 6px rgba(45, 41, 32, 0.08), 0 12px 28px rgba(45, 41, 32, 0.14)',
        'glow-live': '0 0 0 1px rgba(26, 127, 75, 0.25)',
        'glow-alert': '0 0 0 1px rgba(207, 56, 37, 0.25)',
        'glow-signal': '0 0 0 1px rgba(168, 120, 28, 0.28)',
        'glow-violet': '0 0 0 1px rgba(119, 72, 184, 0.24)',
        'inner-line': 'inset 0 1px 0 rgba(45, 41, 32, 0.03)',
      },
    },
  },
  plugins: [],
}
