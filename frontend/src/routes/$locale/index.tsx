import { createFileRoute, Link } from '@tanstack/react-router'
import {
  ArrowRight,
  ArrowUUpRight,
  PaperPlaneTilt,
  SealCheck,
  GlobeHemisphereWest,
} from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import {
  PorcelainFigure,
  PorcelainLandscape,
  ArtistMedallion,
} from '~/components/artwork/PorcelainFigure'
import { ActivityCard, ArtistCard, ProductCard } from '~/components/cards'
import {
  BrushRule,
  CloudScroll,
  CornerFrame,
  PetalScatter,
  SealMark,
  WaveBand,
} from '~/components/ornaments'
import { Button, ButtonLink } from '~/components/common/ui'
import { Spine } from '~/components/layout/Spine'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'
import { useLoopScroller } from '~/lib/loop-scroller'
import { loaderCurrency } from '~/lib/utils'
import { CONTACT } from '~/mocks/data'
import { cn } from '~/lib/utils'
import type { CatalogKey } from '~/i18n/en-US'

/**
 * Landing — a horizontal, infinitely-looping magazine (canals-amsterdam /
 * yourbana feel): wheel/drag/keyboard-driven panels on a modulo circle,
 * left spine chrome, parallax figures, reveal-on-approach text, and the
 * footer as the final "tail" panel. Loader data unchanged (parallel
 * fetches per TDD §11.1 #1).
 */
export const Route = createFileRoute('/$locale/')({
  loader: async ({ params }) => {
    const locale = params.locale
    const currency = await loaderCurrency()
    const [featured, destinations, artists, catalog] = await Promise.all([
      api.getProducts({ locale, currency, page: 1, limit: 5, sort: 'featured' }),
      api.getActivities(locale, 'destination'),
      api.getArtists(locale),
      api.getProducts({ locale, currency, page: 1, limit: 48 }),
    ])
    return {
      featured: featured.data,
      destinations: destinations.slice(0, 3),
      artists,
      catalog: catalog.data,
    }
  },
  head: () => ({
    meta: [
      { title: 'Jingdezhen Ceramics — The Kiln City Journal' },
      {
        name: 'description',
        content:
          'Centuries of porcelain craft from Jingdezhen — qinghua, fencai, linglong and colored glazes. Discover, collect, and visit.',
      },
    ],
  }),
  component: LandingMagazine,
})

/* ------------------------------------------------------------------ */
/* Panel registry (spine dots + counter)                               */
/* ------------------------------------------------------------------ */

const CHAPTERS: CatalogKey[] = [
  'mag.cover',
  'mag.families',
  'mag.gallery',
  'mag.heritage',
  'mag.destinations',
  'mag.artists',
  'mag.journey',
  'mag.end',
]

const FAMILIES = [
  {
    key: 'qinghua',
    tag: 'qinghua',
    figure: 'vase',
    seed: 101,
    band: 'bg-cobalt-600',
    chip: 'bg-cobalt-50 text-cobalt-700',
    zh: '青花',
    nameKey: 'mag.fam.qinghua',
    bodyKey: 'mag.fam.qinghuaBody',
  },
  {
    key: 'fencai',
    tag: 'fencai',
    figure: 'bowl',
    seed: 107,
    band: 'bg-rose-500',
    chip: 'bg-rose-100 text-rose-600',
    zh: '粉彩',
    nameKey: 'mag.fam.fencai',
    bodyKey: 'mag.fam.fencaiBody',
  },
  {
    key: 'linglong',
    tag: 'linglong',
    figure: 'jar',
    seed: 110,
    band: 'bg-celadon-500',
    chip: 'bg-celadon-100 text-celadon-600',
    zh: '玲珑',
    nameKey: 'mag.fam.linglong',
    bodyKey: 'mag.fam.linglongBody',
  },
  {
    key: 'yanseyou',
    tag: 'yanseyou',
    figure: 'teapot',
    seed: 111,
    band: 'bg-cinnabar-500',
    chip: 'bg-cinnabar-100 text-cinnabar-600',
    zh: '颜色釉',
    nameKey: 'mag.fam.yanseyou',
    bodyKey: 'mag.fam.yanseyouBody',
  },
] as const

/* ------------------------------------------------------------------ */
/* Page                                                                */
/* ------------------------------------------------------------------ */

function LandingMagazine() {
  const { t, locale } = useI18n()
  const data = Route.useLoaderData()
  const scroller = useLoopScroller(CHAPTERS.length)

  // lock the page scroll — the magazine owns all motion
  useEffect(() => {
    document.body.classList.add('magazine-lock')
    return () => document.body.classList.remove('magazine-lock')
  }, [])

  const panels = (ariaHidden: boolean) => (
    <div className="flex h-full" aria-hidden={ariaHidden || undefined}>
      {/* ------------------------------ 1 · cover ------------------------------ */}
      <section
        data-panel
        className="relative h-full w-screen shrink-0 overflow-hidden bg-gradient-to-b from-wash via-paper to-porcelain/70"
      >
        <div className="qinghua-watermark absolute inset-y-0 right-0 w-2/3 opacity-80" />
        <PetalScatter
          seed={11}
          count={9}
          className="pointer-events-none absolute top-14 right-[38%] opacity-60"
          width={200}
          height={120}
        />
        <div className="relative grid h-full grid-cols-[1.05fr_0.95fr] items-center gap-8 pr-10 pl-10 sm:pl-14">
          <div className="max-w-xl" data-reveal>
            <p className="eyebrow flex items-center gap-2.5">
              <CloudScroll size={22} opacity={0.9} />
              {t('mag.tagline')} · {t('mag.issue')}
            </p>
            <h1 className="mt-5 text-display text-ink-900">{t('landing.heroTitle')}</h1>
            <p className="mt-6 max-w-md text-[1.05rem] leading-relaxed text-ink-500">
              {t('landing.heroSub')}
            </p>
            <div className="mt-9 flex flex-wrap items-center gap-3.5">
              <Button size="lg" onClick={() => scroller.scrollToPanel(2)}>
                {t('landing.ctaGallery')}
                <ArrowRight size={16} weight="bold" />
              </Button>
              <ButtonLink to="/$locale/itinerary" params={{ locale }} variant="secondary" size="lg">
                {t('landing.ctaTravel')}
              </ButtonLink>
            </div>
            <dl className="mt-12 flex max-w-lg gap-8 border-t border-cobalt-100 pt-6">
              {(
                [
                  ['landing.statYears', 'landing.statYearsLabel'],
                  ['landing.statProcesses', 'landing.statProcessesLabel'],
                  ['landing.statKiln', 'landing.statKilnLabel'],
                ] as const
              ).map(([v, l]) => (
                <div key={v}>
                  <dd className="text-[1.35rem] font-semibold tracking-tight text-cobalt-700">
                    {t(v)}
                  </dd>
                  <dt className="mt-1 max-w-28 text-[0.72rem] leading-snug text-ink-400">{t(l)}</dt>
                </div>
              ))}
            </dl>
          </div>
          <div className="relative mx-auto aspect-square w-full max-w-[26rem]" data-parallax="0.1">
            <div className="relative h-full w-full overflow-hidden rounded-md border border-cobalt-100 bg-white shadow-pop">
              <CornerFrame inset={14} />
              <PorcelainFigure kind="vase" seed={101} className="h-full w-full" />
            </div>
            <div className="absolute -bottom-5 left-1/2 -translate-x-1/2 rounded-sm border border-cobalt-100 bg-white/95 px-4 py-2 shadow-lift backdrop-blur">
              <p className="flex items-center gap-2 text-[0.78rem] font-medium text-ink-600">
                <SealCheck size={16} className="text-gold-500" weight="duotone" />
                {t('landing.trustAuthenticity')}
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ------------------------------ 2 · families ------------------------------ */}
      <section data-panel className="relative h-full w-[118vw] shrink-0 bg-paper">
        <div className="flex h-full flex-col justify-center gap-8 pr-12 pl-10 sm:pl-14">
          <div className="max-w-xl" data-reveal>
            <p className="eyebrow">{t('mag.familiesEyebrow')}</p>
            <h2 className="mt-2.5 text-display-sm text-ink-900">{t('mag.familiesTitle')}</h2>
            <BrushRule className="mt-4" />
          </div>
          <div className="flex gap-5" data-reveal>
            {FAMILIES.map((f) => (
              <Link
                key={f.key}
                to="/$locale/catalog"
                params={{ locale }}
                search={{ tag: f.tag }}
                className="card-surface group flex w-60 shrink-0 flex-col overflow-hidden transition duration-300 hover:-translate-y-1.5 hover:shadow-lift"
              >
                <div className={cn('h-2 w-full', f.band)} />
                <div className="relative aspect-square bg-gradient-to-b from-wash to-porcelain">
                  <div style={{ filter: 'hue-rotate(0deg)' }} className="h-full w-full">
                    <FamilyFigure family={f.key} seed={f.seed} figure={f.figure} />
                  </div>
                  <span
                    className={cn(
                      'absolute top-2.5 right-2.5 rounded-sm px-2 py-0.5 text-[0.72rem] font-bold',
                      f.chip,
                    )}
                  >
                    {f.zh}
                  </span>
                </div>
                <div className="flex flex-1 flex-col p-4">
                  <h3 className="text-[0.95rem] font-semibold text-ink-800 group-hover:text-cobalt-700">
                    {t(f.nameKey)}
                  </h3>
                  <p className="mt-1.5 flex-1 text-[0.8rem] leading-relaxed text-ink-500">
                    {t(f.bodyKey)}
                  </p>
                  <span className="mt-3 inline-flex items-center gap-1 text-[0.78rem] font-medium text-cobalt-600">
                    {t('mag.familiesBrowse')}
                    <ArrowRight size={12} className="transition group-hover:translate-x-0.5" />
                  </span>
                </div>
              </Link>
            ))}
          </div>
        </div>
      </section>

      {/* ------------------------------ 3 · gallery ------------------------------ */}
      <section data-panel className="relative h-full w-[132vw] shrink-0 bg-mist/60">
        <div className="qinghua-watermark absolute inset-0 opacity-40" />
        <div className="relative flex h-full flex-col justify-center gap-8 pr-12 pl-10 sm:pl-14">
          <div className="flex items-end justify-between pr-4" data-reveal>
            <div className="max-w-lg">
              <p className="eyebrow">{t('landing.featuredEyebrow')}</p>
              <h2 className="mt-2.5 text-display-sm text-ink-900">{t('landing.featuredTitle')}</h2>
              <p className="mt-3 text-[0.92rem] leading-relaxed text-ink-500">
                {t('landing.featuredSub')}
              </p>
            </div>
            <ButtonLink
              to="/$locale/catalog"
              params={{ locale }}
              variant="secondary"
              size="sm"
              className="shrink-0"
            >
              {t('common.viewAll')}
              <ArrowUUpRight size={13} weight="bold" />
            </ButtonLink>
          </div>
          <div className="flex items-stretch gap-6" data-reveal>
            {data.featured.map((p, i) => (
              <div key={p.id} className={cn('w-64 shrink-0', i % 2 === 1 ? 'mt-8' : '')}>
                <ProductCard product={p} />
              </div>
            ))}
            <div className="flex w-56 shrink-0 flex-col items-center justify-center gap-4 border-l border-cobalt-100 pl-6 text-center">
              <WaveBand width={96} />
              <p className="text-[0.8rem] leading-relaxed text-ink-400">{t('mag.dragHint')}</p>
            </div>
          </div>
        </div>
      </section>

      {/* ------------------------------ 4 · heritage ------------------------------ */}
      <section
        data-panel
        className="relative h-full w-screen shrink-0 overflow-hidden bg-cobalt-800"
      >
        <div
          className="qinghua-watermark absolute inset-0 opacity-[0.14]"
          style={{ filter: 'invert(1)' }}
        />
        <PetalScatter
          seed={23}
          count={8}
          className="pointer-events-none absolute top-10 right-10 opacity-20"
        />
        <div className="relative grid h-full grid-cols-2 items-center gap-10 pr-12 pl-10 sm:pl-14">
          <div className="max-w-lg" data-reveal>
            <p className="text-[0.72rem] font-semibold tracking-[0.2em] text-white/60 uppercase">
              {t('landing.heritageEyebrow')}
            </p>
            <h2 className="mt-4 text-display-sm text-white">{t('landing.heritageTitle')}</h2>
            <p className="mt-5 leading-relaxed text-white/70">{t('landing.heritageBody')}</p>
            <ButtonLink
              to="/$locale/ceramicstory"
              params={{ locale }}
              size="lg"
              className="mt-8 bg-white text-cobalt-700 hover:bg-porcelain"
            >
              {t('landing.heritageCta')}
              <ArrowRight size={16} weight="bold" />
            </ButtonLink>
          </div>
          <div
            className="relative mx-auto w-full max-w-[30rem] overflow-hidden rounded-md border border-white/15 shadow-pop"
            data-parallax="0.08"
          >
            <PorcelainLandscape seed={201} tone="deep" className="h-auto w-full" />
          </div>
        </div>
      </section>

      {/* ------------------------------ 5 · destinations ------------------------------ */}
      <section data-panel className="relative h-full w-[124vw] shrink-0 bg-paper">
        <div className="flex h-full flex-col justify-center gap-8 pr-12 pl-10 sm:pl-14">
          <div className="flex items-end justify-between pr-4" data-reveal>
            <div className="max-w-lg">
              <p className="eyebrow">{t('landing.visitEyebrow')}</p>
              <h2 className="mt-2.5 text-display-sm text-ink-900">{t('landing.visitTitle')}</h2>
            </div>
            <ButtonLink
              to="/$locale/engage"
              params={{ locale }}
              variant="secondary"
              size="sm"
              className="shrink-0"
            >
              {t('landing.visitCta')}
              <ArrowUUpRight size={13} weight="bold" />
            </ButtonLink>
          </div>
          <div className="flex gap-6" data-reveal>
            {data.destinations.map((a) => (
              <div key={a.id} className="w-80 shrink-0">
                <ActivityCard activity={a} />
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ------------------------------ 6 · artists ------------------------------ */}
      <section
        data-panel
        className="relative h-full w-screen shrink-0 bg-gradient-to-l from-porcelain/60 to-paper"
      >
        <div className="flex h-full flex-col items-start justify-center gap-8 pr-12 pl-10 sm:pl-14">
          <div className="max-w-lg" data-reveal>
            <p className="eyebrow">{t('landing.artistsEyebrow')}</p>
            <h2 className="mt-2.5 text-display-sm text-ink-900">{t('landing.artistsTitle')}</h2>
          </div>
          <div className="flex items-stretch gap-6" data-reveal>
            {data.artists.slice(0, 3).map((a, i) => (
              <div
                key={a.id}
                className={cn('w-72 shrink-0', i === 1 ? 'mt-10' : i === 2 ? 'mt-4' : '')}
              >
                <ArtistCard
                  artist={a}
                  works={data.catalog.filter((p) => p.artist_id === a.id).length}
                />
              </div>
            ))}
            <div className="flex w-52 shrink-0 flex-col items-center justify-center gap-3 border-l border-cobalt-100 pl-6">
              <ArtistMedallion glyph="景" seed={7} size={64} />
              <p className="text-center text-[0.8rem] leading-relaxed text-ink-400">
                <Link to="/$locale/artists" params={{ locale }} className="link-quiet">
                  {t('common.viewAll')} →
                </Link>
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* ------------------------------ 7 · journey ------------------------------ */}
      <section
        data-panel
        className="bg-cobalt-band relative h-full w-screen shrink-0 overflow-hidden"
      >
        <div
          className="qinghua-watermark absolute inset-0 opacity-[0.12]"
          style={{ filter: 'invert(1)' }}
        />
        <PetalScatter
          seed={42}
          count={9}
          className="pointer-events-none absolute bottom-10 left-1/3 opacity-20"
        />
        <div className="relative flex h-full flex-col items-start justify-center gap-9 pr-12 pl-10 sm:pl-14">
          <div className="max-w-xl" data-reveal>
            <p className="text-[0.72rem] font-semibold tracking-[0.2em] text-white/70 uppercase">
              {t('landing.travelEyebrow')}
            </p>
            <h2 className="mt-4 text-display-sm text-white">{t('landing.travelTitle')}</h2>
            <p className="mt-5 leading-relaxed text-white/75">{t('landing.travelBody')}</p>
          </div>
          <div className="flex gap-4" data-reveal>
            {(
              [
                ['landing.travelStep1', 'landing.travelStep1Body', 'bg-cobalt-600'],
                ['landing.travelStep2', 'landing.travelStep2Body', 'bg-rose-500'],
                ['landing.travelStep3', 'landing.travelStep3Body', 'bg-celadon-500'],
                ['landing.travelStep4', 'landing.travelStep4Body', 'bg-imperial-400'],
              ] as const
            ).map(([title, body, chip], i) => (
              <div
                key={title}
                className="w-56 rounded-md border border-white/15 bg-white/10 px-4 py-5 backdrop-blur-sm"
              >
                <p className="flex items-center gap-2 text-[0.85rem] font-semibold text-white">
                  <span
                    className={cn(
                      'flex h-5 w-5 items-center justify-center rounded-sm text-[0.68rem] font-bold text-white',
                      chip,
                    )}
                  >
                    {i + 1}
                  </span>
                  {t(title)}
                </p>
                <p className="mt-2 text-[0.78rem] leading-relaxed text-white/65">{t(body)}</p>
              </div>
            ))}
          </div>
          <ButtonLink
            to="/$locale/itinerary"
            params={{ locale }}
            size="lg"
            className="bg-white text-cobalt-700 shadow-pop hover:bg-porcelain"
          >
            {t('landing.travelCta')}
            <ArrowRight size={16} weight="bold" />
          </ButtonLink>
        </div>
      </section>

      {/* ------------------------------ 8 · tail (footer) ------------------------------ */}
      <section data-panel className="relative h-full w-screen shrink-0 bg-mist">
        <div className="flex h-full flex-col justify-between py-10 pr-12 pl-10 sm:pl-14">
          <div className="flex items-start justify-between gap-10 pt-4" data-reveal>
            <div className="max-w-md">
              <div className="flex items-center gap-2.5">
                <SealMark size={38} />
                <span className="flex flex-col leading-none">
                  <span className="text-[0.95rem] font-semibold text-ink-900">
                    {t('common.brand')}
                  </span>
                  <span className="mt-1 text-[0.62rem] font-medium tracking-[0.16em] text-cobalt-600 uppercase">
                    {t('common.brandSub')}
                  </span>
                </span>
              </div>
              <p className="mt-4 text-sm leading-relaxed text-ink-500">{t('footer.tagline')}</p>
              <p className="mt-4 flex items-center gap-2 text-sm text-ink-500">
                <GlobeHemisphereWest size={15} className="text-cobalt-400" />
                {CONTACT.email} · {CONTACT.phone}
              </p>
              <NewsletterForm />
            </div>
            <FooterCols />
          </div>
          <div
            className="flex items-center justify-between border-t border-cobalt-100 pt-5"
            data-reveal
          >
            <WaveBand width={140} />
            <p className="text-[0.75rem] text-ink-400">{t('footer.rights')}</p>
            <p className="text-[0.75rem] text-ink-300">{t('common.prototypeNote')}</p>
          </div>
        </div>
      </section>
    </div>
  )

  const activeIdx = ((scroller.activeIndex % CHAPTERS.length) + CHAPTERS.length) % CHAPTERS.length

  return (
    <div className="magazine-page relative h-[100dvh] w-screen overflow-hidden bg-paper">
      <Spine
        chapters={CHAPTERS.map((labelKey) => ({ labelKey }))}
        activeIndex={activeIdx}
        onJump={scroller.scrollToPanel}
      />
      <div ref={scroller.viewportRef} className="loop-viewport absolute inset-0 pl-14 sm:pl-16">
        <div ref={scroller.trackRef} className="flex h-full w-max will-change-transform">
          {panels(false)}
          {panels(true)}
        </div>
      </div>

      {/* issue chip + panel counter */}
      <div className="pointer-events-none absolute top-4 right-5 z-30 flex items-center gap-3 text-[0.72rem] font-semibold tracking-[0.14em] text-ink-500 uppercase">
        <span className="hidden sm:inline">{t('mag.issue')}</span>
        <span className="rounded-sm border border-cobalt-100 bg-white/90 px-2.5 py-1 tabular-nums backdrop-blur">
          {String(activeIdx + 1).padStart(2, '0')} / {String(CHAPTERS.length).padStart(2, '0')}
        </span>
      </div>
      <p className="pointer-events-none absolute right-5 bottom-4 z-30 hidden items-center gap-2 text-[0.72rem] text-ink-300 sm:flex">
        <ArrowRight size={13} />
        {t('mag.dragHint')}
      </p>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/* Family figure — PorcelainFigure tinted per porcelain family         */
/* ------------------------------------------------------------------ */

const FAMILY_TINTS: Record<string, string> = {
  qinghua: 'none',
  fencai: 'hue-rotate(115deg) saturate(1.25)',
  linglong: 'hue-rotate(285deg) saturate(1.05)',
  yanseyou: 'hue-rotate(150deg) saturate(1.35)',
}

function FamilyFigure({
  family,
  seed,
  figure,
}: {
  family: string
  seed: number
  figure: 'vase' | 'bowl' | 'plate' | 'teapot' | 'jar'
}) {
  const tint = FAMILY_TINTS[family] ?? 'none'
  return (
    <div className="h-full w-full" style={{ filter: tint === 'none' ? undefined : tint }}>
      <PorcelainFigure
        kind={figure}
        seed={seed}
        className="h-full w-full transition duration-500 [div:hover>&]:scale-[1.04]"
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */

function NewsletterForm() {
  const { t } = useI18n()
  const [email, setEmail] = useState('')
  const [done, setDone] = useState(false)

  if (done) {
    return (
      <p className="mt-6 inline-flex items-center gap-2 rounded-sm bg-[color:var(--color-success-bg)] px-4 py-2.5 text-sm font-medium text-[color:var(--color-success)]">
        <SealCheck size={17} weight="duotone" />
        {t('landing.newsletterThanks')}
      </p>
    )
  }
  return (
    <form
      className="mt-6 flex max-w-md gap-2.5"
      onSubmit={(e) => {
        e.preventDefault()
        if (email.includes('@')) setDone(true)
      }}
    >
      <input
        type="email"
        required
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder={t('landing.newsletterPlaceholder')}
        className="input-base flex-1"
        aria-label={t('landing.newsletterPlaceholder')}
      />
      <Button type="submit" className="shrink-0">
        <PaperPlaneTilt size={15} />
        {t('landing.newsletterCta')}
      </Button>
    </form>
  )
}

function FooterCols() {
  const { t, locale } = useI18n()
  const base = `/${locale}`
  const cols = [
    {
      title: t('footer.explore'),
      links: [
        { label: t('nav.gallery'), to: `${base}/catalog` },
        { label: t('nav.artists'), to: `${base}/artists` },
        { label: t('nav.heritage'), to: `${base}/ceramicstory` },
        { label: t('nav.visit'), to: `${base}/engage` },
      ],
    },
    {
      title: t('footer.support'),
      links: [
        { label: t('footer.contact'), to: `${base}/engage` },
        { label: t('footer.shipping'), to: `${base}/catalog` },
        { label: t('footer.refunds'), to: `${base}/orders` },
      ],
    },
    {
      title: t('footer.legal'),
      links: [
        { label: t('footer.privacy'), to: `${base}/account` },
        { label: t('footer.terms'), to: `${base}/account` },
      ],
    },
  ]
  return (
    <div className="flex gap-14">
      {cols.map((col) => (
        <nav key={col.title} aria-label={col.title}>
          <h3 className="text-[0.7rem] font-semibold tracking-[0.18em] text-ink-400 uppercase">
            {col.title}
          </h3>
          <ul className="mt-4 flex flex-col gap-2.5">
            {col.links.map((l) => (
              <li key={l.label}>
                <Link
                  to={l.to as never}
                  className="text-sm text-ink-600 transition hover:text-cobalt-700"
                >
                  {l.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      ))}
    </div>
  )
}
