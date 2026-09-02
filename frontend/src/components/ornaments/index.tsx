/**
 * Qinghua (青花瓷) ornament kit — small cobalt decorations used sparingly:
 * section eyebrows, dividers, corner brackets, the seal-stamp brand mark,
 * and a wave-band texture. All vector, deterministic, SSR-safe.
 */
import { cn, seededRandom } from '~/lib/utils'

/* ------------------------------------------------------------------ */
/* Seal stamp (印章) — the brand mark.                                 */
/* ------------------------------------------------------------------ */

export function SealMark({
  chars = ['景德', '镇印'],
  size = 34,
  className,
}: {
  chars?: [string, string] | string[]
  size?: number
  className?: string
}) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" className={className} aria-hidden="true">
      <rect x="3" y="3" width="58" height="58" rx="5" fill="var(--cobalt-600)" />
      <rect
        x="8.5"
        y="8.5"
        width="47"
        height="47"
        rx="3"
        fill="none"
        stroke="#fff"
        strokeOpacity="0.55"
        strokeWidth="1.8"
      />
      <text
        x="32"
        y="28"
        fontFamily="'Noto Sans SC','PingFang SC','Microsoft YaHei',sans-serif"
        fontSize="17.5"
        fontWeight="700"
        fill="#fff"
        textAnchor="middle"
      >
        {chars[0]}
      </text>
      <text
        x="32"
        y="47"
        fontFamily="'Noto Sans SC','PingFang SC','Microsoft YaHei',sans-serif"
        fontSize="17.5"
        fontWeight="700"
        fill="#fff"
        textAnchor="middle"
      >
        {chars[1]}
      </text>
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/* Wave band (波涛纹) — a repeating cobalt wave divider.               */
/* ------------------------------------------------------------------ */

export function WaveBand({
  className,
  width = 260,
  opacity = 0.5,
}: {
  className?: string
  width?: number
  opacity?: number
}) {
  return (
    <svg
      width={width}
      height={14}
      viewBox="0 0 260 14"
      fill="none"
      className={className}
      aria-hidden="true"
    >
      <g stroke="var(--cobalt-500)" strokeOpacity={opacity} strokeWidth="1.4" strokeLinecap="round">
        <path d="M2 11c7-8 14-8 21 0s14 8 21 0 14-8 21 0 14 8 21 0 14-8 21 0 14 8 21 0 14-8 21 0 14 8 21 0 14-8 21 0 14 8 21 0 14-8 21 0 14 8 21 0" />
        <path
          d="M16 4c4.5-5 9-5 13.5 0M79 4c4.5-5 9-5 13.5 0M142 4c4.5-5 9-5 13.5 0M205 4c4.5-5 9-5 13.5 0"
          strokeOpacity={opacity * 0.45}
          strokeWidth="1.2"
        />
      </g>
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/* Cloud scroll (祥云) — a single auspicious-cloud motif.              */
/* ------------------------------------------------------------------ */

export function CloudScroll({
  className,
  size = 28,
  opacity = 0.85,
}: {
  className?: string
  size?: number
  opacity?: number
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      className={className}
      aria-hidden="true"
    >
      <g
        stroke="var(--cobalt-500)"
        strokeOpacity={opacity}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M6 21c-2.4 0-4-1.6-4-3.7 0-1.9 1.4-3.4 3.3-3.7.2-4 3.5-7.1 7.6-7.1 3.4 0 6.3 2.2 7.3 5.2.4-.1.8-.2 1.2-.2 2.3 0 4.1 1.8 4.1 4s-1.8 4-4.1 4" />
        <path d="M8 25h16M12 28.5h8" strokeOpacity={opacity * 0.4} />
      </g>
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/* Brush rule — a short hand-brushed divider under headings.          */
/* ------------------------------------------------------------------ */

export function BrushRule({ className, width = 72 }: { className?: string; width?: number }) {
  return (
    <svg
      width={width}
      height="6"
      viewBox="0 0 72 6"
      fill="none"
      className={className}
      aria-hidden="true"
    >
      <path
        d="M1 3.6C14 1.6 34 1.2 46 2.2c8 .7 17 1.3 24 2.2-9 .9-19 1.2-29 1.1C28 5.4 12 5 1 3.6Z"
        fill="var(--cobalt-500)"
        fillOpacity="0.75"
      />
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/* Lotus corner brackets (莲花瓣尖角) — frames imagery cards.          */
/* ------------------------------------------------------------------ */

export function LotusCorners({ className, size = 14 }: { className?: string; size?: number }) {
  const corner = (
    <g
      stroke="var(--cobalt-500)"
      strokeOpacity="0.5"
      strokeWidth="1.4"
      strokeLinecap="round"
      fill="none"
    >
      <path d="M2 14V8C2 4.4 4.4 2 8 2h6" />
      <path d="M5.5 14v-4.5c0-2.2 1.8-4 4-4H14" strokeOpacity="0.25" />
    </g>
  )
  return (
    <svg
      className={cn('pointer-events-none absolute', className)}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      aria-hidden="true"
    >
      {corner}
    </svg>
  )
}

/** Four-corner frame overlay (render inside a relative container). */
export function CornerFrame({ inset = 10 }: { inset?: number }) {
  const base = 'absolute h-3.5 w-3.5 text-cobalt-500/40'
  return (
    <div aria-hidden="true">
      <LotusCorners className={cn(base, 'left-[' + inset + 'px] top-[' + inset + 'px]')} />
      <LotusCorners
        className={cn(base, 'right-[' + inset + 'px] top-[' + inset + 'px] rotate-90')}
      />
      <LotusCorners
        className={cn(base, 'bottom-[' + inset + 'px] right-[' + inset + 'px] rotate-180')}
      />
      <LotusCorners
        className={cn(base, 'bottom-[' + inset + 'px] left-[' + inset + 'px] -rotate-90')}
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/* Blooming lotus — tiny decorative flower for empty states / lists.  */
/* ------------------------------------------------------------------ */

export function LotusBloom({ className, size = 20 }: { className?: string; size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      className={className}
      aria-hidden="true"
    >
      <g stroke="var(--cobalt-500)" strokeWidth="1.4" strokeLinejoin="round" strokeLinecap="round">
        <path
          d="M12 4c1.8 2.2 2.6 4.6 2.4 7.2C13.7 9.4 13 7.8 12 6.4c-1 1.4-1.7 3-2.4 4.8C9.4 8.6 10.2 6.2 12 4Z"
          fill="var(--cobalt-500)"
          fillOpacity="0.14"
        />
        <path d="M12 4c1.8 2.2 2.6 4.6 2.4 7.2C13.7 9.4 13 7.8 12 6.4c-1 1.4-1.7 3-2.4 4.8C9.4 8.6 10.2 6.2 12 4Z" />
        <path
          d="M5 8.5c2.6.4 4.6 1.6 6 3.6-2.3-.5-4.4-.6-6.2-.2"
          fill="var(--cobalt-500)"
          fillOpacity="0.08"
        />
        <path d="M5 8.5c2.6.4 4.6 1.6 6 3.6-2.3-.5-4.4-.6-6.2-.2" />
        <path
          d="M19 8.5c-2.6.4-4.6 1.6-6 3.6 2.3-.5 4.4-.6 6.2-.2"
          fill="var(--cobalt-500)"
          fillOpacity="0.08"
        />
        <path d="M19 8.5c-2.6.4-4.6 1.6-6 3.6 2.3-.5 4.4-.6 6.2-.2" />
        <path d="M6.5 15.5c1.8-.8 3.6-1 5.5-.6 1.9-.4 3.7-.2 5.5.6" />
        <path d="M8.5 18.5c1.2-.5 2.3-.7 3.5-.6 1.2-.1 2.3.1 3.5.6" strokeOpacity="0.5" />
      </g>
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/* Scatter — a few seeded dots/petals, for hero and CTA band corners. */
/* ------------------------------------------------------------------ */

export function PetalScatter({
  seed = 7,
  count = 9,
  className,
  width = 220,
  height = 120,
  opacity = 0.16,
}: {
  seed?: number
  count?: number
  className?: string
  width?: number
  height?: number
  opacity?: number
}) {
  const rand = seededRandom(seed)
  const petals = Array.from({ length: count }, (_, i) => {
    const x = 12 + rand() * (width - 24)
    const y = 12 + rand() * (height - 24)
    const r = 4 + rand() * 5
    const rot = Math.floor(rand() * 180)
    return (
      <path
        key={i}
        d={`M0 ${-r}C${r * 0.9} ${-r * 0.4},${r * 0.9} ${r * 0.4},0 ${r}C${-r * 0.9} ${r * 0.4},${-r * 0.9} ${-r * 0.4},0 ${-r}`}
        stroke="var(--cobalt-600)"
        strokeOpacity={opacity}
        strokeWidth="1.2"
        fill="var(--cobalt-500)"
        fillOpacity={opacity * 0.45}
        transform={`translate(${x.toFixed(1)} ${y.toFixed(1)}) rotate(${rot})`}
      />
    )
  })
  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className={className}
      aria-hidden="true"
      fill="none"
    >
      {petals}
    </svg>
  )
}
