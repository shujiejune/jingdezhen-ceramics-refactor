import type { Config } from 'tailwindcss'

/**
 * Design tokens (TDD §6): blue-and-white porcelain (青花瓷) — a cobalt
 * scale over porcelain white, ink text, one restrained antique-gold
 * accent (seal / stars only). Stripe-clean geometry: hairline borders,
 * soft layered shadows, generous whitespace.
 *
 * NOTE: colors are hex literals here (not var()) so Tailwind's alpha
 * modifiers (`/60`) work. src/styles/tokens.css mirrors these values as
 * CSS variables for SVG fills and gradients — keep the two in sync.
 */
const config: Config = {
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        cobalt: {
          50: '#f0f4fc',
          100: '#e2eaf9',
          200: '#c3d5f1',
          300: '#9ab7e6',
          400: '#6c93d8',
          500: '#4872c4',
          600: '#3559ae',
          700: '#2a4790',
          800: '#243b76',
          900: '#1f3260',
          950: '#141f3d',
        },
        ink: {
          300: '#9aa4bd',
          400: '#6f7c9a',
          500: '#525f7d',
          600: '#3b4661',
          700: '#2a3450',
          800: '#1a2338',
          900: '#0d1730',
        },
        paper: '#ffffff',
        mist: '#f6f8fc',
        porcelain: '#eef3fb',
        wash: '#f9fbfe',
        gold: {
          400: '#c9a55c',
          500: '#b08a3e',
          600: '#96722d',
        },
      },
      fontFamily: {
        sans: [
          'Inter Variable',
          'Inter',
          '-apple-system',
          'BlinkMacSystemFont',
          '"Segoe UI"',
          '"PingFang SC"',
          '"Hiragino Sans GB"',
          '"Microsoft YaHei"',
          '"Noto Sans SC"',
          'sans-serif',
        ],
      },
      fontSize: {
        // Display + editorial scale, tuned for English-first typography
        // with zh-CN-appropriate fallbacks (PRD §4.1).
        display: [
          '3.25rem',
          { lineHeight: '1.06', letterSpacing: '-0.025em', fontWeight: '640' },
        ],
        'display-sm': [
          '2.25rem',
          { lineHeight: '1.12', letterSpacing: '-0.02em', fontWeight: '640' },
        ],
      },
      boxShadow: {
        card: 'var(--shadow-card)',
        lift: 'var(--shadow-lift)',
        pop: 'var(--shadow-pop)',
        inset: 'var(--shadow-inset)',
      },
      maxWidth: {
        shell: '76rem', // 1216px content shell
      },
      backgroundImage: {
        'cobalt-band':
          'linear-gradient(115deg, #243b76 0%, #3559ae 52%, #6c93d8 100%)',
        'porcelain-sheen':
          'radial-gradient(120% 120% at 85% 8%, #eef3fb 0%, rgba(238, 243, 251, 0) 55%)',
      },
      keyframes: {
        fadein: { from: { opacity: '0' }, to: { opacity: '1' } },
      },
    },
  },
  plugins: [],
}

export default config
