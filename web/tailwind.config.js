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
        disp: ['"Chakra Petch"', 'system-ui', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'monospace'],
      },
    },
  },
  plugins: [],
}
