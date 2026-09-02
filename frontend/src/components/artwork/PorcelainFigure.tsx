/**
 * PorcelainFigure — procedural qinghua (blue-and-white) artwork imagery.
 *
 * Every product, artist, and destination renders a deterministic SVG
 * "photograph stand-in": a porcelain silhouette (vase / bowl / plate /
 * teapot / jar / landscape) decorated with seeded cobalt patterns —
 * lotus petal bands, wave bands, and a central medallion (peony /
 * landscape / lotus scroll / waves). Seeded by entity id so server and
 * client render identically (no hydration drift) and each artwork keeps
 * a stable identity across pages.
 */
import { seededRandom } from '~/lib/utils'

export type FigureKind = 'vase' | 'bowl' | 'plate' | 'teapot' | 'jar'

/* ------------------------------------------------------------------ */
/* Band helpers                                                        */
/* ------------------------------------------------------------------ */

/** A row of arc "petals" (half-round scallops) along a horizontal band. */
function petalBand(
  y: number,
  x1: number,
  x2: number,
  h: number,
  n: number,
  fill: string,
  opacity: number,
  up = false,
) {
  const w = (x2 - x1) / n
  let d = ''
  for (let i = 0; i < n; i++) {
    const x = x1 + i * w
    d += `M${x.toFixed(1)} ${y} C${(x + w * 0.12).toFixed(1)} ${(y + (up ? -h : h) * 0.9).toFixed(1)}, ${(x + w * 0.88).toFixed(1)} ${(y + (up ? -h : h) * 0.9).toFixed(1)}, ${(x + w).toFixed(1)} ${y} `
  }
  return (
    <path
      d={d}
      fill={fill}
      fillOpacity={opacity}
      stroke="var(--cobalt-600)"
      strokeOpacity={Math.min(1, opacity * 2.4)}
      strokeWidth="1"
    />
  )
}

/** Rolling wave arcs (波涛纹) along a horizontal line. */
function waveBand(y: number, x1: number, x2: number, amp: number, wave: number, opacity: number) {
  let d = `M${x1} ${y} `
  for (let x = x1; x < x2; x += wave) {
    d += `q${wave / 4} ${-amp} ${wave / 2} 0 q${wave / 4} ${amp} ${wave / 2} 0 `
  }
  return (
    <path
      d={d}
      fill="none"
      stroke="var(--cobalt-500)"
      strokeOpacity={opacity}
      strokeWidth="1.6"
      strokeLinecap="round"
    />
  )
}

/** A row of small dots. */
function dotRow(y: number, x1: number, x2: number, n: number, opacity: number) {
  const w = (x2 - x1) / (n - 1)
  return (
    <g fill="var(--cobalt-500)" fillOpacity={opacity}>
      {Array.from({ length: n }, (_, i) => (
        <circle key={i} cx={x1 + i * w} cy={y} r="2" />
      ))}
    </g>
  )
}

/** Vertical hatching (classic lotus-petal separations). */
function rayFan(cx: number, cy: number, rx: number, ry: number, n: number, opacity: number) {
  return (
    <g stroke="var(--cobalt-500)" strokeOpacity={opacity} strokeWidth="1.1">
      {Array.from({ length: n }, (_, i) => {
        const a = (Math.PI * i) / (n - 1)
        return (
          <line
            key={i}
            x1={cx - rx * Math.cos(a)}
            y1={cy - ry * Math.sin(a)}
            x2={cx - (rx - 26) * Math.cos(a)}
            y2={cy - (ry - 9) * Math.sin(a)}
          />
        )
      })}
    </g>
  )
}

/* ------------------------------------------------------------------ */
/* Medallion — the central body pattern                                */
/* ------------------------------------------------------------------ */

function Medallion({ cx, cy, r, seed }: { cx: number; cy: number; r: number; seed: number }) {
  const rand = seededRandom(seed)
  const variant = Math.floor(rand() * 4)
  const stroke = 'var(--cobalt-600)'
  const wash = 'var(--cobalt-500)'

  return (
    <g>
      <circle
        cx={cx}
        cy={cy}
        r={r}
        fill="none"
        stroke={stroke}
        strokeOpacity="0.5"
        strokeWidth="1.2"
      />
      <circle
        cx={cx}
        cy={cy}
        r={r - 5}
        fill="none"
        stroke={stroke}
        strokeOpacity="0.25"
        strokeWidth="0.8"
      />

      {variant === 0 && (
        /* Peony spray */
        <g>
          {[0, 60, 120, 180, 240, 300].map((a) => (
            <ellipse
              key={a}
              cx={cx}
              cy={cy - r * 0.36}
              rx={r * 0.2}
              ry={r * 0.4}
              fill={wash}
              fillOpacity="0.14"
              stroke={stroke}
              strokeOpacity="0.75"
              strokeWidth="1.1"
              transform={`rotate(${a} ${cx} ${cy})`}
            />
          ))}
          <circle
            cx={cx}
            cy={cy}
            r={r * 0.2}
            fill={wash}
            fillOpacity="0.3"
            stroke={stroke}
            strokeOpacity="0.8"
            strokeWidth="1.1"
          />
          <path
            d={`M${cx} ${cy + r * 0.6} q10 -14 24 -10 M${cx} ${cy + r * 0.62} q-6 -16 6 -24`}
            fill="none"
            stroke={stroke}
            strokeOpacity="0.55"
            strokeWidth="1.2"
            strokeLinecap="round"
          />
        </g>
      )}

      {variant === 1 && (
        /* River landscape — mountains, moon, boat */
        <g>
          <circle
            cx={cx + r * 0.42}
            cy={cy - r * 0.42}
            r={r * 0.16}
            fill={wash}
            fillOpacity="0.22"
            stroke={stroke}
            strokeOpacity="0.5"
            strokeWidth="1"
          />
          <path
            d={`M${cx - r * 0.7} ${cy + r * 0.18} L${cx - r * 0.28} ${cy - r * 0.42} L${cx + r * 0.02} ${cy + r * 0.1} L${cx + r * 0.3} ${cy - r * 0.3} L${cx + r * 0.68} ${cy + r * 0.18} Z`}
            fill={wash}
            fillOpacity="0.16"
            stroke={stroke}
            strokeOpacity="0.75"
            strokeWidth="1.2"
            strokeLinejoin="round"
          />
          <path
            d={`M${cx - r * 0.55} ${cy + r * 0.42} q${r * 0.3} ${-r * 0.1} ${r * 0.6} 0 q${r * 0.3} ${r * 0.1} ${r * 0.6} 0`}
            fill="none"
            stroke={stroke}
            strokeOpacity="0.5"
            strokeWidth="1.3"
            strokeLinecap="round"
          />
          <path
            d={`M${cx - r * 0.22} ${cy + r * 0.58} l${r * 0.44} 0 l${-r * 0.08} ${r * 0.1} l${-r * 0.28} 0 Z`}
            fill={wash}
            fillOpacity="0.28"
            stroke={stroke}
            strokeOpacity="0.7"
            strokeWidth="1"
          />
          <path
            d={`M${cx} ${cy + r * 0.58} l0 ${-r * 0.34}`}
            stroke={stroke}
            strokeOpacity="0.6"
            strokeWidth="1.2"
            strokeLinecap="round"
          />
        </g>
      )}

      {variant === 2 && (
        /* Lotus scroll */
        <g>
          {[0, 72, 144, 216, 288].map((a) => (
            <path
              key={a}
              d={`M${cx} ${cy - r * 0.55} C${cx + r * 0.28} ${cy - r * 0.4}, ${cx + r * 0.28} ${cy - r * 0.12}, ${cx} ${cy} C${cx - r * 0.28} ${cy - r * 0.12}, ${cx - r * 0.28} ${cy - r * 0.4}, ${cx} ${cy - r * 0.55} Z`}
              fill={wash}
              fillOpacity="0.12"
              stroke={stroke}
              strokeOpacity="0.7"
              strokeWidth="1.1"
              transform={`rotate(${a} ${cx} ${cy - r * 0.27}) translate(0 ${r * 0.27}) rotate(0)`}
            />
          ))}
          <circle
            cx={cx}
            cy={cy}
            r={r * 0.16}
            fill={wash}
            fillOpacity="0.3"
            stroke={stroke}
            strokeOpacity="0.8"
            strokeWidth="1.1"
          />
          {[30, 150, 210, 330].map((a) => (
            <path
              key={a}
              d={`M${cx} ${cy} q${r * 0.35 * Math.cos((a * Math.PI) / 180)} ${r * 0.35 * Math.sin((a * Math.PI) / 180)} ${r * 0.55 * Math.cos((a * Math.PI) / 180)} ${r * 0.55 * Math.sin((a * Math.PI) / 180)}`}
              fill="none"
              stroke={stroke}
              strokeOpacity="0.45"
              strokeWidth="1"
            />
          ))}
        </g>
      )}

      {variant === 3 && (
        /* Boiling waves */
        <g>
          {[0, 1, 2].map((row) => (
            <path
              key={row}
              d={`M${cx - r * 0.62} ${cy + r * 0.4 - row * r * 0.34} q${r * 0.16} ${-r * 0.3} ${r * 0.31} 0 q${r * 0.16} ${r * 0.3} ${r * 0.31} 0 q${r * 0.16} ${-r * 0.3} ${r * 0.31} 0 q${r * 0.16} ${r * 0.3} ${r * 0.31} 0`}
              fill="none"
              stroke={stroke}
              strokeOpacity={0.7 - row * 0.18}
              strokeWidth="1.3"
              strokeLinecap="round"
            />
          ))}
          <circle cx={cx - r * 0.3} cy={cy - r * 0.34} r="3" fill={wash} fillOpacity="0.5" />
          <circle cx={cx + r * 0.18} cy={cy - r * 0.5} r="2.4" fill={wash} fillOpacity="0.4" />
        </g>
      )}
    </g>
  )
}

/* ------------------------------------------------------------------ */
/* Vessel silhouettes + decoration layouts                             */
/* ------------------------------------------------------------------ */

interface VesselSpec {
  body: string
  extras?: React.ReactNode
  clip: string
  bands: React.ReactNode
  medallion: { cx: number; cy: number; r: number }
  shadowY: number
  shadowRx: number
}

function vesselSpec(kind: FigureKind, seed: number): VesselSpec {
  const rand = seededRandom(seed * 7 + 13)
  const jit = (v: number, m: number) => v + (rand() - 0.5) * m

  if (kind === 'vase') {
    // 梅瓶 meiping — small mouth, broad shoulder, tapered foot
    const body =
      'M172 70 L228 70 C232 88 236 96 244 104 C268 122 292 140 296 178 C300 214 286 268 252 318 C244 330 240 338 240 348 L160 348 C160 338 156 330 148 318 C114 268 100 214 104 178 C108 140 132 122 156 104 C164 96 168 88 172 70 Z'
    return {
      body,
      clip: body,
      medallion: { cx: 200, cy: jit(225, 8), r: jit(52, 6) },
      shadowY: 354,
      shadowRx: 84,
      bands: (
        <>
          {petalBand(140, 112, 288, 16, 9, 'var(--cobalt-500)', 0.16)}
          {dotRow(160, 128, 272, 7, 0.4)}
          {waveBand(310, 132, 268, 7, 34, 0.5)}
          {petalBand(336, 152, 248, 12, 6, 'var(--cobalt-500)', 0.14, true)}
        </>
      ),
    }
  }

  if (kind === 'bowl') {
    const body =
      'M108 148 C112 220 140 296 176 322 C182 327 190 330 200 330 C210 330 218 327 224 322 C260 296 288 220 292 148 C244 158 156 158 108 148 Z M176 330 L172 344 C171 348 173 352 177 352 L223 352 C227 352 229 348 228 344 L224 330'
    return {
      body,
      clip: 'M108 148 C112 220 140 296 176 322 C182 327 190 330 200 330 C210 330 218 327 224 322 C260 296 288 220 292 148 C244 158 156 158 108 148 Z',
      medallion: { cx: 200, cy: jit(240, 6), r: jit(44, 5) },
      shadowY: 358,
      shadowRx: 66,
      bands: (
        <>
          {petalBand(282, 128, 272, 14, 8, 'var(--cobalt-500)', 0.16, true)}
          {dotRow(172, 136, 264, 6, 0.4)}
        </>
      ),
    }
  }

  if (kind === 'plate') {
    // Front-facing plate: broad ellipse face + low foot
    const body =
      'M58 200 A142 46 0 1 0 342 200 A142 46 0 1 0 58 200 Z M164 246 L164 258 C164 262 168 264 200 264 C232 264 236 262 236 258 L236 246'
    return {
      body,
      clip: 'M58 200 A142 46 0 1 0 342 200 A142 46 0 1 0 58 200 Z',
      medallion: { cx: 200, cy: 200, r: 36 },
      shadowY: 272,
      shadowRx: 110,
      bands: (
        <>
          <ellipse
            cx="200"
            cy="200"
            rx="118"
            ry="37"
            fill="none"
            stroke="var(--cobalt-600)"
            strokeOpacity="0.45"
            strokeWidth="1.1"
          />
          {rayFan(200, 200, 112, 35, 14, 0.35)}
        </>
      ),
    }
  }

  if (kind === 'teapot') {
    const body =
      'M200 148 C245 148 283 186 283 234 C283 282 245 316 200 316 C155 316 117 282 117 234 C117 186 155 148 200 148 Z M283 214 C305 204 322 190 328 172 C332 188 326 214 292 232 M117 210 C96 202 82 214 78 236 C74 214 88 196 112 194'
    const clip =
      'M200 148 C245 148 283 186 283 234 C283 282 245 316 200 316 C155 316 117 282 117 234 C117 186 155 148 200 148 Z'
    return {
      body,
      clip,
      extras: (
        <g>
          <ellipse
            cx="200"
            cy="146"
            rx="56"
            ry="13"
            fill="#fff"
            stroke="var(--cobalt-700)"
            strokeOpacity="0.55"
            strokeWidth="1.4"
          />
          <circle
            cx="200"
            cy="132"
            r="9"
            fill="#fff"
            stroke="var(--cobalt-700)"
            strokeOpacity="0.55"
            strokeWidth="1.4"
          />
        </g>
      ),
      medallion: { cx: 200, cy: jit(234, 6), r: jit(40, 5) },
      shadowY: 324,
      shadowRx: 96,
      bands: (
        <>
          {petalBand(290, 132, 268, 12, 7, 'var(--cobalt-500)', 0.15, true)}
          {dotRow(176, 148, 252, 5, 0.4)}
        </>
      ),
    }
  }

  // jar (将军罐) with domed lid
  const body =
    'M132 168 C132 132 162 108 200 108 C238 108 268 132 268 168 C288 196 296 232 290 264 C282 306 246 336 200 336 C154 336 118 306 110 264 C104 232 112 196 132 168 Z'
  return {
    body,
    clip: body,
    extras: (
      <g>
        <path
          d="M148 128 C160 100 178 88 200 88 C222 88 240 100 252 128 C236 120 216 116 200 116 C184 116 164 120 148 128 Z"
          fill="#fff"
          stroke="var(--cobalt-700)"
          strokeOpacity="0.55"
          strokeWidth="1.4"
        />
        <circle
          cx="200"
          cy="82"
          r="8"
          fill="#fff"
          stroke="var(--cobalt-700)"
          strokeOpacity="0.55"
          strokeWidth="1.4"
        />
      </g>
    ),
    medallion: { cx: 200, cy: jit(232, 8), r: jit(50, 6) },
    shadowY: 344,
    shadowRx: 100,
    bands: (
      <>
        {petalBand(158, 132, 268, 14, 8, 'var(--cobalt-500)', 0.16)}
        {dotRow(178, 148, 252, 6, 0.4)}
        {waveBand(300, 138, 262, 6, 31, 0.45)}
      </>
    ),
  }
}

/* ------------------------------------------------------------------ */
/* Main component                                                      */
/* ------------------------------------------------------------------ */

export function PorcelainFigure({
  kind,
  seed,
  className,
  label,
}: {
  kind: FigureKind
  seed: number
  className?: string
  label?: string
}) {
  const spec = vesselSpec(kind, seed)
  const gid = `pf-${kind}-${seed}`
  const m = spec.medallion

  return (
    <svg
      viewBox="0 0 400 400"
      className={className}
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
    >
      <defs>
        <linearGradient id={`${gid}-body`} x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stopColor="#dfe8f6" />
          <stop offset="22%" stopColor="#ffffff" />
          <stop offset="62%" stopColor="#f6f9fe" />
          <stop offset="100%" stopColor="#d3deef" />
        </linearGradient>
        <radialGradient id={`${gid}-glaze`} cx="0.32" cy="0.24" r="0.75">
          <stop offset="0%" stopColor="#ffffff" stopOpacity="0.9" />
          <stop offset="55%" stopColor="#eaf0fa" stopOpacity="0.35" />
          <stop offset="100%" stopColor="#c4d3ea" stopOpacity="0.45" />
        </radialGradient>
        <clipPath id={`${gid}-clip`}>
          <path d={spec.clip} />
        </clipPath>
      </defs>

      {/* ground shadow */}
      <ellipse
        cx="200"
        cy={spec.shadowY}
        rx={spec.shadowRx}
        ry="11"
        fill="var(--ink-900)"
        opacity="0.08"
      />

      {/* vessel body */}
      <path
        d={spec.body}
        fill={`url(#${gid}-body)`}
        stroke="var(--cobalt-700)"
        strokeOpacity="0.5"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      <path
        d={spec.clip}
        fill={`url(#${gid}-glaze)`}
        style={{ mixBlendMode: 'multiply' }}
        opacity="0.7"
      />

      {/* decoration, clipped to the silhouette */}
      <g clipPath={`url(#${gid}-clip)`}>
        {spec.bands}
        <Medallion cx={m.cx} cy={m.cy} r={m.r} seed={seed} />
      </g>

      {/* re-stroke outline above decoration */}
      <path
        d={spec.clip}
        fill="none"
        stroke="var(--cobalt-700)"
        strokeOpacity="0.55"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      {spec.extras}
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/* Destination scene — cobalt-wash landscape for travel content        */
/* ------------------------------------------------------------------ */

export function PorcelainLandscape({
  seed,
  className,
  label,
  tone = 'light',
}: {
  seed: number
  className?: string
  label?: string
  tone?: 'light' | 'deep'
}) {
  const rand = seededRandom(seed * 31 + 5)
  const peak = 150 + rand() * 60
  const peak2 = 100 + rand() * 50
  const ox = (rand() - 0.5) * 40
  const moonX = 250 + rand() * 60
  const deep = tone === 'deep'
  const ink = deep ? '#ffffff' : 'var(--cobalt-700)'
  const wash = deep ? '#ffffff' : 'var(--cobalt-500)'

  return (
    <svg
      viewBox="0 0 400 300"
      className={className}
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      fill="none"
    >
      {/* sky wash */}
      {deep ? (
        <rect width="400" height="300" fill="url(#pl-sky-deep)" />
      ) : (
        <rect width="400" height="300" fill="#eef3fb" />
      )}
      <defs>
        <linearGradient id="pl-sky-deep" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--cobalt-800)" />
          <stop offset="100%" stopColor="var(--cobalt-500)" />
        </linearGradient>
      </defs>

      {/* moon */}
      <circle
        cx={moonX}
        cy="72"
        r="26"
        fill={wash}
        fillOpacity={deep ? 0.2 : 0.18}
        stroke={ink}
        strokeOpacity="0.5"
        strokeWidth="1.2"
      />

      {/* birds */}
      <g stroke={ink} strokeOpacity="0.55" strokeWidth="1.3" strokeLinecap="round">
        <path d="M120 84 q7 -6 14 0 q7 -6 14 0" />
        <path d="M158 66 q5 -4 10 0 q5 -4 10 0" />
      </g>

      {/* far mountains */}
      <path
        d={`M-20 210 L${60 + ox} ${210 - peak2} L${120 + ox} 196 L${180 + ox} ${210 - peak2 * 1.3} L260 210 Z`}
        fill={wash}
        fillOpacity={deep ? 0.24 : 0.14}
        stroke={ink}
        strokeOpacity="0.4"
        strokeWidth="1.1"
      />
      {/* main mountain */}
      <path
        d={`M40 240 L${150 + ox} ${240 - peak} L${205 + ox} 168 L${290 + ox} 240 Z`}
        fill={wash}
        fillOpacity={deep ? 0.34 : 0.22}
        stroke={ink}
        strokeOpacity="0.7"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      {/* mist lines */}
      <g stroke={ink} strokeOpacity="0.3" strokeWidth="1.1" strokeLinecap="round">
        <path d="M70 200 h70 M240 196 h60 M180 224 h90" />
      </g>

      {/* river */}
      <path
        d="M150 300 C170 262 230 262 250 238 C268 218 300 220 320 206"
        stroke={ink}
        strokeOpacity="0.45"
        strokeWidth="1.4"
      />
      {/* boat */}
      <g transform="translate(226 244) rotate(-4)">
        <path
          d="M0 0 l34 0 l-7 9 l-21 0 Z"
          fill={wash}
          fillOpacity={deep ? 0.4 : 0.3}
          stroke={ink}
          strokeOpacity="0.7"
          strokeWidth="1.1"
        />
        <path d="M16 0 l1 -18" stroke={ink} strokeOpacity="0.6" strokeWidth="1.2" />
        <path
          d="M17 -16 l10 12 l-10 0 Z"
          fill={wash}
          fillOpacity="0.2"
          stroke={ink}
          strokeOpacity="0.45"
          strokeWidth="1"
        />
      </g>

      {/* pagoda */}
      <g
        transform={`translate(${318 + ox * 0.4} 240) scale(0.9)`}
        stroke={ink}
        strokeOpacity="0.65"
        strokeWidth="1.2"
        fill={wash}
        fillOpacity={deep ? 0.25 : 0.16}
      >
        <path d="M-8 0 h16 l-3 -10 h-10 Z M-6 -10 h12 l-6 -8 Z" />
        <path d="M-11 -18 h22 M-4 -26 h8" strokeLinecap="round" />
      </g>

      {/* foreground wave band */}
      <path
        d="M-10 282 q20 -14 40 0 q20 14 40 0 q20 -14 40 0 q20 14 40 0 q20 -14 40 0 q20 14 40 0 q20 -14 40 0 q20 14 40 0 q20 -14 40 0 q20 14 40 0"
        stroke={ink}
        strokeOpacity="0.4"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/* Artist medallion — circular seal with the artist's surname glyph    */
/* ------------------------------------------------------------------ */

export function ArtistMedallion({
  glyph,
  seed,
  size = 72,
  className,
}: {
  glyph: string
  seed: number
  size?: number
  className?: string
}) {
  const rand = seededRandom(seed * 17 + 3)
  const dots = Array.from({ length: 8 }, (_, i) => {
    const a = (i / 8) * Math.PI * 2 + rand() * 0.4
    return (
      <circle
        key={i}
        cx={50 + 42 * Math.cos(a)}
        cy={50 + 42 * Math.sin(a)}
        r="1.6"
        fill="var(--cobalt-500)"
        fillOpacity="0.5"
      />
    )
  })
  return (
    <svg width={size} height={size} viewBox="0 0 100 100" className={className} aria-hidden="true">
      <circle cx="50" cy="50" r="48" fill="var(--porcelain)" />
      <circle
        cx="50"
        cy="50"
        r="48"
        fill="none"
        stroke="var(--cobalt-600)"
        strokeOpacity="0.5"
        strokeWidth="1.6"
      />
      <circle
        cx="50"
        cy="50"
        r="41"
        fill="none"
        stroke="var(--cobalt-500)"
        strokeOpacity="0.3"
        strokeWidth="1"
        strokeDasharray="3 4"
      />
      {dots}
      <text
        x="50"
        y="58"
        textAnchor="middle"
        fontFamily="'Noto Sans SC','PingFang SC','Microsoft YaHei',sans-serif"
        fontSize="26"
        fontWeight="600"
        fill="var(--cobalt-700)"
      >
        {glyph}
      </text>
    </svg>
  )
}
