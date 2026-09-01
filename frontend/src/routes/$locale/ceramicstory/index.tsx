import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowRight, ArrowUUpRight, BookOpenText, MapPin, Airplane } from '@phosphor-icons/react'
import { useEffect } from 'react'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { BrushRule, CornerFrame, PetalScatter, SealMark, WaveBand } from '~/components/ornaments'
import { Button, ButtonLink } from '~/components/common/ui'
import { Spine } from '~/components/layout/Spine'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'
import { useLoopScroller } from '~/lib/loop-scroller'
import { CONTACT } from '~/mocks/data'
import { cn } from '~/lib/utils'
import type { CatalogKey } from '~/i18n/en-US'

/**
 * Heritage index — a horizontal magazine of dynasty chapters (the
 * canals-amsterdam feel, matching the landing): cover → one editorial
 * spread per dynasty (color-coded to the porcelain-family palette) →
 * tail panel that carries the footer. Detail articles stay vertical
 * reading pages with the normal chrome.
 */
export const Route = createFileRoute('/$locale/ceramicstory/')({
  loader: async ({ params }) => api.getStories(params.locale),
  head: () => ({
    meta: [
      { title: 'Heritage — Jingdezhen Ceramics' },
      { name: 'description', content: 'A thousand years of porcelain, dynasty by dynasty — the history of the kiln city.' },
    ],
  }),
  component: HeritageMagazine,
})

/* ------------------------------------------------------------------ */
/* Dynasty accents — each chapter keyed to a porcelain family color    */
/* ------------------------------------------------------------------ */

const DYNASTY_ACCENTS = [
  { numeral: 'text-celadon-500/15', chip: 'bg-celadon-500 text-white', band: 'bg-celadon-500', eyebrow: 'text-celadon-600' },
  { numeral: 'text-cobalt-500/15', chip: 'bg-cobalt-600 text-white', band: 'bg-cobalt-600', eyebrow: 'text-cobalt-600' },
  { numeral: 'text-cinnabar-500/15', chip: 'bg-cinnabar-500 text-white', band: 'bg-cinnabar-500', eyebrow: 'text-cinnabar-600' },
  { numeral: 'text-rose-500/15', chip: 'bg-rose-500 text-white', band: 'bg-rose-500', eyebrow: 'text-rose-600' },
] as const

const accentOf = (i: number) => DYNASTY_ACCENTS[i % DYNASTY_ACCENTS.length]

/* ------------------------------------------------------------------ */

function HeritageMagazine() {
  const { t, locale } = useI18n()
  const stories = Route.useLoaderData()
  const total = stories.length + 2 // cover + dynasties + tail
  const scroller = useLoopScroller(total)

  useEffect(() => {
    document.body.classList.add('magazine-lock')
    return () => document.body.classList.remove('magazine-lock')
  }, [])

  const panels = (ariaHidden: boolean) => (
    <div className="flex h-full" aria-hidden={ariaHidden || undefined}>
      {/* ------------------------------ cover ------------------------------ */}
      <section data-panel className="relative h-full w-screen shrink-0 overflow-hidden bg-gradient-to-b from-porcelain/70 via-paper to-wash">
        <div className="qinghua-watermark absolute inset-y-0 right-0 w-2/3 opacity-80" />
        <PetalScatter seed={33} count={8} className="pointer-events-none absolute top-16 right-[36%] opacity-50" width={190} height={120} />
        <div className="relative grid h-full grid-cols-[1.05fr_0.95fr] items-center gap-8 pr-10 pl-10 sm:pl-14">
          <div className="max-w-xl" data-reveal>
            <p className="eyebrow">
              {t('story.title')} · {t('mag.tagline')}
            </p>
            <h1 className="mt-5 text-display text-ink-900">{t('landing.heritageTitle')}</h1>
            <p className="mt-6 max-w-md text-[1.02rem] leading-relaxed text-ink-500">{t('story.subtitle')}</p>
            <div className="mt-9 flex flex-wrap items-center gap-3.5">
              <Button size="lg" onClick={() => scroller.scrollToPanel(1)}>
                <BookOpenText size={17} weight="duotone" />
                {t('story.timeline')}
              </Button>
              <ButtonLink to="/$locale" params={{ locale }} variant="secondary" size="lg">
                {t('common.brand')}
              </ButtonLink>
            </div>
            <ul className="mt-12 flex max-w-lg gap-7 border-t border-cobalt-100 pt-6">
              {stories.map((s, i) => (
                <li key={s.id}>
                  <button
                    type="button"
                    onClick={() => scroller.scrollToPanel(i + 1)}
                    className="group flex flex-col items-start gap-1 text-left"
                  >
                    <span className={cn('text-[1.1rem] font-semibold tabular-nums', accentOf(i).eyebrow)}>
                      {s.dynasty_start_year}
                    </span>
                    <span className="max-w-24 text-[0.7rem] leading-snug text-ink-400 transition group-hover:text-cobalt-600">
                      {s.title}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
          <div className="relative mx-auto aspect-square w-full max-w-[26rem]" data-parallax="0.1">
            <div className="relative h-full w-full overflow-hidden rounded-md border border-cobalt-100 bg-white shadow-pop">
              <CornerFrame inset={14} />
              <PorcelainFigure kind="jar" seed={204} className="h-full w-full" />
            </div>
            <div className="absolute -bottom-5 left-1/2 -translate-x-1/2 rounded-sm border border-cobalt-100 bg-white/95 px-4 py-2 shadow-lift backdrop-blur">
              <p className="text-[0.78rem] font-medium text-ink-600">{t('story.subtitle')}</p>
            </div>
          </div>
        </div>
      </section>

      {/* ------------------------------ dynasty chapters ------------------------------ */}
      {stories.map((story, i) => {
        const accent = accentOf(i)
        return (
          <section
            key={story.id}
            data-panel
            className={cn(
              'relative h-full w-[112vw] shrink-0 overflow-hidden',
              i % 2 === 1 ? 'bg-mist/70' : 'bg-paper',
            )}
          >
            {/* oversized year numeral */}
            <span
              aria-hidden="true"
              className={cn(
                'pointer-events-none absolute -top-6 right-6 text-[11rem] leading-none font-bold tabular-nums select-none',
                accent.numeral,
              )}
            >
              {story.dynasty_start_year}
            </span>
            <div className={cn('absolute inset-y-0 left-0 w-1.5', accent.band)} />

            <div className="relative grid h-full grid-cols-[1fr_0.85fr] items-center gap-10 py-8 pr-12 pl-12 sm:pl-16">
              <div className="max-w-xl" data-reveal>
                <p className="flex items-center gap-3">
                  <span className={cn('rounded-sm px-2 py-0.5 text-[0.7rem] font-bold tracking-wide uppercase', accent.chip)}>
                    {t('heritage.chapterN', { n: String(i + 1).padStart(2, '0') })}
                  </span>
                  <span className={cn('text-[0.72rem] font-semibold tracking-[0.2em] tabular-nums uppercase', accent.eyebrow)}>
                    {story.dynasty_start_year}
                  </span>
                </p>
                <h2 className="mt-5 text-display-sm text-ink-900">{story.title}</h2>
                <BrushRule className="mt-4" />
                <p className="mt-5 max-w-lg leading-relaxed text-ink-500">{story.summary}</p>
                <ButtonLink
                  to="/$locale/ceramicstory/$slug"
                  params={{ locale, slug: story.slug }}
                  size="lg"
                  className="mt-8"
                >
                  {t('story.read')}
                  <ArrowRight size={16} weight="bold" />
                </ButtonLink>
              </div>
              <div className="relative mx-auto aspect-square w-full max-w-[24rem]" data-parallax="0.09">
                <div className="relative h-full w-full overflow-hidden rounded-md border border-cobalt-100 bg-white shadow-lift">
                  <CornerFrame inset={12} />
                  <PorcelainFigure kind="vase" seed={story.figure_seed} className="h-full w-full" />
                  <span className={cn('absolute inset-x-0 top-0 h-1.5', accent.band)} />
                </div>
              </div>
            </div>
          </section>
        )
      })}

      {/* ------------------------------ tail ------------------------------ */}
      <section data-panel className="relative h-full w-screen shrink-0 bg-mist">
        <div className="flex h-full flex-col justify-between py-10 pr-12 pl-10 sm:pl-14">
          <div className="flex items-start justify-between gap-10 pt-4" data-reveal>
            <div className="max-w-md">
              <div className="flex items-center gap-2.5">
                <SealMark size={38} />
                <span className="flex flex-col leading-none">
                  <span className="text-[0.95rem] font-semibold text-ink-900">{t('common.brand')}</span>
                  <span className="mt-1 text-[0.62rem] font-medium tracking-[0.16em] text-cobalt-600 uppercase">
                    {t('common.brandSub')}
                  </span>
                </span>
              </div>
              <h2 className="mt-6 text-[1.45rem] font-semibold tracking-tight text-ink-900">{t('heritage.tailTitle')}</h2>
              <div className="mt-4 flex flex-col gap-2.5">
                <Link to="/$locale/catalog" params={{ locale }} className="flex items-center gap-2.5 text-[0.95rem] font-medium text-ink-700 transition hover:text-cobalt-700">
                  <span className="flex h-8 w-8 items-center justify-center rounded-sm bg-white shadow-card"><BookOpenText size={16} className="text-cobalt-500" weight="duotone" /></span>
                  {t('nav.gallery')}
                </Link>
                <Link to="/$locale/engage" params={{ locale }} className="flex items-center gap-2.5 text-[0.95rem] font-medium text-ink-700 transition hover:text-cobalt-700">
                  <span className="flex h-8 w-8 items-center justify-center rounded-sm bg-white shadow-card"><MapPin size={16} className="text-celadon-500" weight="duotone" /></span>
                  {t('nav.visit')}
                </Link>
                <Link to="/$locale/itinerary" params={{ locale }} className="flex items-center gap-2.5 text-[0.95rem] font-medium text-ink-700 transition hover:text-cobalt-700">
                  <span className="flex h-8 w-8 items-center justify-center rounded-sm bg-white shadow-card"><Airplane size={16} className="text-rose-500" weight="duotone" /></span>
                  {t('nav.travel')}
                </Link>
              </div>
              <p className="mt-6 text-[0.82rem] text-ink-400">{CONTACT.email} · {CONTACT.phone}</p>
            </div>

            {/* dynasty recap index */}
            <div className="max-w-md">
              <h3 className="text-[0.7rem] font-semibold tracking-[0.18em] text-ink-400 uppercase">{t('story.timeline')}</h3>
              <ol className="mt-4 flex flex-col divide-y divide-cobalt-100">
                {stories.map((s, i) => (
                  <li key={s.id}>
                    <Link
                      to="/$locale/ceramicstory/$slug"
                      params={{ locale, slug: s.slug }}
                      className="group flex items-baseline gap-4 py-3"
                    >
                      <span className={cn('w-14 text-[0.95rem] font-semibold tabular-nums', accentOf(i).eyebrow)}>
                        {s.dynasty_start_year}
                      </span>
                      <span className="flex-1 text-[0.95rem] text-ink-700 transition group-hover:text-cobalt-700">{s.title}</span>
                      <ArrowUUpRight size={13} className="text-ink-300 transition group-hover:text-cobalt-600" />
                    </Link>
                  </li>
                ))}
              </ol>
            </div>
          </div>
          <div className="flex items-center justify-between border-t border-cobalt-100 pt-5" data-reveal>
            <WaveBand width={140} />
            <p className="text-[0.75rem] text-ink-400">{t('footer.rights')}</p>
            <p className="text-[0.75rem] text-ink-300">{t('common.prototypeNote')}</p>
          </div>
        </div>
      </section>
    </div>
  )

  const chapters: Array<{ labelKey?: CatalogKey; label?: string }> = [
    { labelKey: 'mag.cover' },
    ...stories.map((s) => ({ label: String(s.dynasty_start_year) })),
    { labelKey: 'mag.end' },
  ]

  const activeIdx = ((scroller.activeIndex % total) + total) % total

  return (
    <div className="magazine-page relative h-[100dvh] w-screen overflow-hidden bg-paper">
      <Spine chapters={chapters} activeIndex={activeIdx} onJump={scroller.scrollToPanel} />
      <div ref={scroller.viewportRef} className="loop-viewport absolute inset-0 pl-14 sm:pl-16">
        <div ref={scroller.trackRef} className="flex h-full w-max will-change-transform">
          {panels(false)}
          {panels(true)}
        </div>
      </div>

      {/* issue chip + chapter counter */}
      <div className="pointer-events-none absolute top-4 right-5 z-30 flex items-center gap-3 text-[0.72rem] font-semibold tracking-[0.14em] text-ink-500 uppercase">
        <span className="hidden sm:inline">{t('story.title')}</span>
        <span className="rounded-sm border border-cobalt-100 bg-white/90 px-2.5 py-1 tabular-nums backdrop-blur">
          {String(activeIdx + 1).padStart(2, '0')} / {String(total).padStart(2, '0')}
        </span>
      </div>
      <p className="pointer-events-none absolute right-5 bottom-4 z-30 hidden items-center gap-2 text-[0.72rem] text-ink-300 sm:flex">
        <ArrowRight size={13} />
        {t('mag.dragHint')}
      </p>
    </div>
  )
}
