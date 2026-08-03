/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#0b0f14',
        panel: '#121a24',
        panel2: '#0e141c',
        line: '#1f2b38',
        signal: '#e8b23a',
        live: '#4ec9b0',
        alert: '#ff5c4d',
        ghost: '#5b6b7a',
        ink2: '#cdd6e0',
        muted: '#728496',
      },
      fontFamily: {
        disp: ['"Segoe UI"', '"Microsoft YaHei"', 'system-ui', 'sans-serif'],
        mono: ['ui-monospace', '"Cascadia Mono"', 'Consolas', '"Courier New"', 'monospace'],
      },
      boxShadow: {
        card: '0 4px 18px rgba(0, 0, 0, 0.35)',
        'glow-live': '0 0 14px rgba(78, 201, 176, 0.35)',
        'glow-alert': '0 0 14px rgba(255, 92, 77, 0.3)',
        'glow-signal': '0 0 14px rgba(232, 178, 58, 0.28)',
        'inner-line': 'inset 0 1px 0 rgba(255, 255, 255, 0.03)',
      },
    },
  },
  plugins: [],
}
