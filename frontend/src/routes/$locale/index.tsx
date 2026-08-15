import { createFileRoute } from '@tanstack/react-router'
import { ArrowRight, PaperPlaneTilt, SealCheck, GlobeHemisphereWest } from '@phosphor-icons/react'
import { useState } from 'react'

import { PorcelainFigure, PorcelainLandscape } from '~/components/artwork/PorcelainFigure'
import { ActivityCard, ArtistCard, ProductCard, WaveDivider } from '~/components/cards'
import { BrushRule, CloudScroll, CornerFrame, PetalScatter, WaveBand } from '~/components/ornaments'
import { Button, ButtonLink, SectionHeading } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'
import { loaderCurrency } from '~/lib/utils'

/**
 * Landing (SSR loader — parallel API calls per TDD §11.1 #1).
 */
export const Route = createFileRoute('/$locale/')({
  loader: async ({ params }) => {
    const locale = params.locale
    const currency = await loaderCurrency()
    const [featured, destinations, artists, catalog] = await Promise.all([
      api.getProducts({ locale, currency, page: 1, limit: 4, sort: 'featured' }),
      api.getActivities(locale, 'destination'),
      api.getArtists(locale),
      api.getProducts({ locale, currency, page: 1, limit: 48 }),
    ])
    return { featured: featured.data, destinations: destinations.slice(0, 3), artists, catalog: catalog.data }
  },
  component: LandingPage,
})

function LandingPage() {
  const { t } = useI18n()
  const data = Route.useLoaderData()

  return (
    <div>
      {/* ------------------------------ hero ------------------------------ */}
      <section className="relative overflow-hidden bg-gradient-to-b from-wash via-paper to-paper">
        <div className="qinghua-watermark absolute inset-x-0 top-0 h-64 opacity-80" />
        <div className="pointer-events-none absolute top-16 -left-6 opacity-70">
          <PetalScatter seed={11} count={8} width={200} height={110} />
        </div>
        <div className="relative mx-auto grid max-w-shell gap-14 px-4 pt-16 pb-20 sm:px-6 lg:grid-cols-[1.05fr_0.95fr] lg:pt-24 lg:pb-28">
          <div className="flex flex-col justify-center">
            <p className="eyebrow flex items-center gap-2.5">
              <CloudScroll size={22} opacity={0.9} />
              {t('landing.heroEyebrow')}
            </p>
            <h1 className="mt-5 max-w-xl text-display text-ink-900">{t('landing.heroTitle')}</h1>
            <p className="mt-6 max-w-lg text-[1.05rem] leading-relaxed text-ink-500">{t('landing.heroSub')}</p>
            <div className="mt-9 flex flex-wrap items-center gap-3.5">
              <ButtonLink to="./catalog" size="lg">
                {t('landing.ctaGallery')}
                <ArrowRight size={16} weight="bold" />
              </ButtonLink>
              <ButtonLink to="./itinerary" variant="secondary" size="lg">
                {t('landing.ctaTravel')}
              </ButtonLink>
            </div>

            <dl className="mt-14 grid max-w-lg grid-cols-3 divide-x divide-cobalt-100">
              {(
                [
                  ['landing.statYears', 'landing.statYearsLabel'],
                  ['landing.statProcesses', 'landing.statProcessesLabel'],
                  ['landing.statKiln', 'landing.statKilnLabel'],
                ] as const
              ).map(([v, l]) => (
                <div key={v} className="px-5 first:pl-0 last:pr-0">
                  <dt className="order-2 mt-1 text-[0.78rem] leading-snug text-ink-400">{t(l)}</dt>
                  <dd className="order-1 text-[1.45rem] font-semibold tracking-tight text-cobalt-700">{t(v)}</dd>
                </div>
              ))}
            </dl>
          </div>

          {/* hero art */}
          <div className="relative mx-auto w-full max-w-md">
            <div className="relative overflow-hidden rounded-2xl border border-cobalt-100 bg-gradient-to-b from-porcelain to-wash shadow-pop">
              <CornerFrame inset={14} />
              <PorcelainFigure kind="vase" seed={101} className="h-auto w-full" />
            </div>
            <div className="absolute -right-4 -bottom-6 hidden w-44 rotate-3 overflow-hidden rounded-xl border border-cobalt-100 bg-white shadow-lift sm:block">
              <PorcelainLandscape seed={301} className="h-auto w-full" />
            </div>
            <div className="absolute -top-5 -left-3 hidden rotate-[-6deg] rounded-lg border border-cobalt-100 bg-white/95 px-3.5 py-2 shadow-lift backdrop-blur sm:block">
              <p className="flex items-center gap-2 text-[0.78rem] font-medium text-ink-600">
                <SealCheck size={16} className="text-gold-500" weight="duotone" />
                {t('landing.trustAuthenticity')}
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* --------------------------- trust strip --------------------------- */}
      <section className="border-y border-cobalt-100/70 bg-mist/70">
        <div className="mx-auto grid max-w-shell gap-8 px-4 py-10 sm:grid-cols-3 sm:px-6">
          {(
            [
              [<SealCheck key="s" size={22} className="text-cobalt-500" weight="duotone" />, 'landing.trustAuthenticity', 'landing.trustAuthenticityBody'],
              [<PaperPlaneTilt key="p" size={22} className="text-cobalt-500" weight="duotone" />, 'landing.trustShipping', 'landing.trustShippingBody'],
              [<GlobeHemisphereWest key="g" size={22} className="text-cobalt-500" weight="duotone" />, 'landing.trustBilingual', 'landing.trustBilingualBody'],
            ] as const
          ).map(([icon, title, body]) => (
            <div key={title} className="flex items-start gap-3.5">
              <span className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-white shadow-card">
                {icon}
              </span>
              <div>
                <h3 className="text-[0.9rem] font-semibold text-ink-800">{t(title)}</h3>
                <p className="mt-1 text-[0.82rem] leading-relaxed text-ink-500">{t(body)}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* --------------------------- featured works --------------------------- */}
      <section className="mx-auto max-w-shell px-4 pt-20 sm:px-6">
        <SectionHeading
          eyebrow={t('landing.featuredEyebrow')}
          title={t('landing.featuredTitle')}
          sub={t('landing.featuredSub')}
          action={
            <ButtonLink to="./catalog" variant="secondary" size="sm">
              {t('common.viewAll')}
              <ArrowRight size={13} weight="bold" />
            </ButtonLink>
          }
        />
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {data.featured.map((p, i) => (
            <ProductCard key={p.id} product={p} priority={i < 2} />
          ))}
        </div>
      </section>

      {/* --------------------------- heritage editorial --------------------------- */}
      <section className="relative mt-24 overflow-hidden bg-porcelain/60">
        <div className="qinghua-watermark absolute inset-0 opacity-50" />
        <div className="relative mx-auto grid max-w-shell items-center gap-12 px-4 py-20 sm:px-6 lg:grid-cols-2">
          <div className="relative order-2 mx-auto w-full max-w-md lg:order-1">
            <div className="overflow-hidden rounded-2xl border border-cobalt-100 bg-white shadow-pop">
              <PorcelainFigure kind="jar" seed={204} className="h-auto w-full" />
            </div>
            <div className="absolute -bottom-6 left-1/2 -translate-x-1/2">
              <WaveBand width={130} opacity={0.8} />
            </div>
          </div>
          <div className="order-1 lg:order-2">
            <p className="eyebrow">{t('landing.heritageEyebrow')}</p>
            <h2 className="mt-3 max-w-md text-display-sm text-ink-900">{t('landing.heritageTitle')}</h2>
            <BrushRule className="mt-5" />
            <p className="mt-5 max-w-md leading-relaxed text-ink-500">{t('landing.heritageBody')}</p>
            <ButtonLink to="./ceramicstory" className="mt-8">
              {t('landing.heritageCta')}
              <ArrowRight size={15} weight="bold" />
            </ButtonLink>
          </div>
        </div>
      </section>

      {/* --------------------------- destinations --------------------------- */}
      <section className="mx-auto max-w-shell px-4 pt-20 sm:px-6">
        <SectionHeading
          eyebrow={t('landing.visitEyebrow')}
          title={t('landing.visitTitle')}
          action={
            <ButtonLink to="./engage" variant="secondary" size="sm">
              {t('common.viewAll')}
              <ArrowRight size={13} weight="bold" />
            </ButtonLink>
          }
        />
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {data.destinations.map((a) => (
            <ActivityCard key={a.id} activity={a} />
          ))}
        </div>
      </section>

      {/* --------------------------- artists --------------------------- */}
      <section className="mx-auto max-w-shell px-4 pt-20 sm:px-6">
        <SectionHeading
          eyebrow={t('landing.artistsEyebrow')}
          title={t('landing.artistsTitle')}
          action={
            <ButtonLink to="./artists" variant="secondary" size="sm">
              {t('common.viewAll')}
              <ArrowRight size={13} weight="bold" />
            </ButtonLink>
          }
        />
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {data.artists.slice(0, 3).map((a) => (
            <ArtistCard
              key={a.id}
              artist={a}
              works={data.catalog.filter((p) => p.artist_id === a.id).length}
            />
          ))}
        </div>
      </section>

      {/* --------------------------- custom travel band --------------------------- */}
      <section className="relative mt-24">
        <div className="bg-cobalt-band relative overflow-hidden">
          <div className="qinghua-watermark absolute inset-0 opacity-[0.12]" style={{ filter: 'invert(1)' }} />
          <div className="pointer-events-none absolute top-8 right-10 opacity-20">
            <PetalScatter seed={23} count={9} width={220} height={110} opacity={0.9} />
          </div>
          <div className="relative mx-auto flex max-w-shell flex-col items-center px-4 py-20 text-center sm:px-6">
            <p className="text-[0.72rem] font-semibold tracking-[0.2em] text-white/70 uppercase">
              {t('landing.travelEyebrow')}
            </p>
            <h2 className="mt-4 max-w-2xl text-display-sm text-white">{t('landing.travelTitle')}</h2>
            <p className="mt-5 max-w-xl leading-relaxed text-white/75">{t('landing.travelBody')}</p>

            <div className="mt-10 grid w-full max-w-3xl gap-4 sm:grid-cols-4">
              {(
                [
                  ['landing.travelStep1', 'landing.travelStep1Body'],
                  ['landing.travelStep2', 'landing.travelStep2Body'],
                  ['landing.travelStep3', 'landing.travelStep3Body'],
                  ['landing.travelStep4', 'landing.travelStep4Body'],
                ] as const
              ).map(([title, body], i) => (
                <div key={title} className="rounded-xl border border-white/15 bg-white/10 px-4 py-5 backdrop-blur-sm">
                  <p className="flex items-center justify-center gap-2 text-[0.85rem] font-semibold text-white">
                    <span className="flex h-5 w-5 items-center justify-center rounded-full bg-white/20 text-[0.7rem]">
                      {i + 1}
                    </span>
                    {t(title)}
                  </p>
                  <p className="mt-2 text-[0.78rem] leading-relaxed text-white/65">{t(body)}</p>
                </div>
              ))}
            </div>

            <ButtonLink
              to="./itinerary"
              size="lg"
              className="mt-10 bg-white text-cobalt-700 shadow-pop hover:bg-porcelain"
            >
              {t('landing.travelCta')}
              <ArrowRight size={16} weight="bold" />
            </ButtonLink>
          </div>
        </div>
      </section>

      {/* --------------------------- newsletter --------------------------- */}
      <section className="mx-auto max-w-shell px-4 pt-24 pb-4 sm:px-6">
        <WaveDivider className="mb-14" />
        <div className="mx-auto max-w-xl text-center">
          <h2 className="text-display-sm text-ink-900">{t('landing.newsletterTitle')}</h2>
          <p className="mt-3 text-ink-500">{t('landing.newsletterBody')}</p>
          <NewsletterForm />
        </div>
      </section>
    </div>
  )
}

function NewsletterForm() {
  const { t } = useI18n()
  const [email, setEmail] = useState('')
  const [done, setDone] = useState(false)

  if (done) {
    return (
      <p className="mt-7 inline-flex items-center gap-2 rounded-lg bg-[color:var(--color-success-bg)] px-4 py-3 text-sm font-medium text-[color:var(--color-success)]">
        <SealCheck size={17} weight="duotone" />
        {t('landing.newsletterThanks')}
      </p>
    )
  }

  return (
    <form
      className="mt-7 flex gap-2.5"
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
        {t('landing.newsletterCta')}
      </Button>
    </form>
  )
}
