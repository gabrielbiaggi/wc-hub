import type { Config } from 'tailwindcss'

export default {
  darkMode: ['class'],
  content: ['./index.html', './src/**/*.{vue,ts,tsx}'],
  theme: {
    extend: {
      colors: {
        void: '#05080f', panel: '#0b111d', line: '#1a2638', muted: '#8491a5',
        signal: '#49e29d', pulse: '#56a8ff', warning: '#f0b35a', danger: '#ff6577',
      },
      fontFamily: { sans: ['Inter', 'ui-sans-serif', 'system-ui'], mono: ['JetBrains Mono', 'ui-monospace', 'monospace'] },
      boxShadow: { signal: '0 0 32px rgba(73,226,157,.12)', panel: '0 20px 60px rgba(0,0,0,.22)' },
    },
  },
  plugins: [],
} satisfies Config

