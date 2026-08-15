import type { Config } from 'tailwindcss'

/**
 * Design tokens (TDD §6 / §12): the four Jingdezhen porcelain families —
 * qinghua cobalt as the accent (deep, Yuan/Mong sumali-blue), plus
 * famille-rose / celadon / cinnabar / imperial-yellow secondaries — over
 * porcelain white and an ink text ramp. Radii are tight (4–10px).
 *
 * Colors are hex literals here (not var()) so Tailwind's alpha modifiers
 * (`/60`) work; src/styles/tokens.css mirrors them as CSS variables for
 * SVG fills and gradients — keep the two in sync.
 */
const config: Config = {
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        cobalt: {
          50: '#eef3fb',
          100: '#dce6f6',
          200: '#b9ccec',
          300: '#8fabe0',
          400: '#6188d2',
          500: '#3d63bc',
          600: '#2a4aa6',
          700: '#1f3a87',
          800: '#182e6c',
          900: '#121f49',
          950: '#0b1531',
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
        porcelain: '#e7eef9',
        wash: '#f9fbfe',
        rose: {
          50: '#fdf1f5',
          100: '#fbe3ea',
          400: '#e8798f',
          500: '#d95c77',
          600: '#b94461',
        },
        celadon: {
          50: '#eef6f2',
          100: '#d7e9e2',
          400: '#a3c8b8',
          500: '#7fb5a0',
          600: '#5e9a85',
        },
        cinnabar: {
          50: '#fbefed',
          100: '#f2d4d0',
          400: '#c4544a',
          500: '#b23b32',
          600: '#96302a',
        },
        imperial: {
          50: '#fdf8ea',
          100: '#f5e6c4',
          400: '#d9a93f',
          500: '#c08f27',
          600: '#9c7420',
        },
        gold: {
          400: '#d9a93f',
          500: '#c08f27',
          600: '#9c7420',
        },
      },
      borderRadius: {
        none: '0',
        sm: '3px',
        DEFAULT: '4px',
        md: '4px',
        lg: '6px',
        xl: '8px',
        '2xl': '10px',
        '3xl': '12px',
        full: '9999px',
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
        shell: '76rem',
      },
      backgroundImage: {
        'cobalt-band':
          'linear-gradient(115deg, #182e6c 0%, #2a4aa6 55%, #6188d2 100%)',
        'porcelain-sheen':
          'radial-gradient(120% 120% at 85% 8%, #e7eef9 0%, rgba(231, 238, 249, 0) 55%)',
      },
      keyframes: {
        fadein: { from: { opacity: '0' }, to: { opacity: '1' } },
      },
    },
  },
  plugins: [],
}

export default config
