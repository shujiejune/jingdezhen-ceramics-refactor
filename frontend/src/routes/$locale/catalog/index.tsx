import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, ArrowRight, MagnifyingGlass, SealCheck, X } from '@phosphor-icons/react'
import { z } from 'zod'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { CornerFrame } from '~/components/ornaments'
import { Badge, Button, ButtonLink, EmptyState, HeartButton, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'
import { useLoopScroller } from '~/lib/loop-scroller'
import { useWishlist } from '~/lib/wishlist'
import { cn, loaderCurrency } from '~/lib/utils'
import type { Product, Tag } from '~/lib/types'

/**
 * Gallery — museum windows / record-crate browsing (freakmag feel): the
 * filtered works become a fanned deck of framed covers you flip through
 * (drag, horizontal trackpad swipe, arrows, or click a cover to center
 * it). The centered sleeve lifts and its "record" peeks out; a side
 * panel carries the museum label + actions. Filters live in a top bar
 * (search params stay type-safe via validateSearch + zod).
 */
const searchSchema = z.object({
  tag: z.string().optional(),
  artist: z.string().optional(),
  edition: z.enum(['one_of_a_kind', 'limited_edition', 'open_production']).optional(),
  priceBand: z.enum(['low', 'mid', 'high']).optional(),
  sort: z.enum(['featured', 'price_asc', 'price_desc', 'newest']).optional(),
  q: z.string().optional(),
})

/** family color for a tag chip / card spine */
const TAG_TONE: Record<string, string> = {
  qinghua: 'bg-cobalt-600',
  fencai: 'bg-rose-500',
  linglong: 'bg-celadon-500',
  yanseyou: 'bg-cinnabar-500',
  enamel: 'bg-imperial-400',
}
const spineOf = (tags?: Tag[]) => {
  const key = tags?.find((t) => TAG_TONE[t.key])?.key
  return key ? TAG_TONE[key] : 'bg-ink-300'
}

export const Route = createFileRoute('/$locale/catalog/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search: { tag, artist, edition, priceBand, sort, q } }) => ({
    tag,
    artist,
    edition,
    priceBand,
    sort,
    q,
  }),
  loader: async ({ params, deps }) => {
    const locale = params.locale
    const currency = await loaderCurrency()
    const [products, tags, artists] = await Promise.all([
      api.getProducts({
        locale,
        currency,
        page: 1,
        limit: 48, // the crate is the pagination — load the whole filtered set
        tag: deps.tag,
        artist: deps.artist,
        edition: deps.edition,
        priceBand: deps.priceBand,
        sort: deps.sort ?? 'featured',
        q: deps.q,
      }),
      api.getTags(locale),
      api.getArtists(locale),
    ])
    return { products, tags, artists }
  },
  component: CatalogPage,
})

function CatalogPage() {
  const { t, locale } = useI18n()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const data = Route.useLoaderData()
  const works = data.products.data
  const total = data.products.total
  const scroller = useLoopScroller(works.length, { wheel: 'horizontalOnly', center: true })

  const setParam = (patch: Record<string, string | undefined>) => {
    void navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })
  }

  const hasFilters = Boolean(
    search.tag || search.artist || search.edition || search.priceBand || search.q,
  )
  const activeIdx = works.length
    ? ((scroller.activeIndex % works.length) + works.length) % works.length
    : 0
  const active = works[activeIdx]

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 pb-16 sm:px-6">
      {/* ------------------------------ header ------------------------------ */}
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="eyebrow">{t('nav.gallery')}</p>
          <h1 className="mt-2 text-display-sm text-ink-900">{t('catalog.title')}</h1>
          <p className="mt-2 max-w-xl text-[0.92rem] text-ink-500">{t('catalog.subtitle')}</p>
        </div>
        <form
          className="relative"
          onSubmit={(e) => {
            e.preventDefault()
            const value = (new FormData(e.currentTarget).get('q') as string) ?? ''
            setParam({ q: value || undefined })
          }}
        >
          <MagnifyingGlass
            size={16}
            className="absolute top-1/2 left-3.5 -translate-y-1/2 text-ink-300"
          />
          <input
            name="q"
            defaultValue={search.q ?? ''}
            placeholder={t('common.search')}
            className="input-base h-10 w-56 pl-10"
            aria-label={t('common.search')}
          />
        </form>
      </div>

      {/* ------------------------------ filter bar ------------------------------ */}
      <div className="sticky top-16 z-30 -mx-4 mt-6 border-y border-cobalt-100/70 bg-white/92 px-4 py-3 backdrop-blur-md sm:-mx-6 sm:px-6">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2.5">
          {/* tag chips */}
          <div className="flex flex-wrap items-center gap-1.5">
            {data.tags.map((tag) => (
              <button
                key={tag.key}
                type="button"
                onClick={() => setParam({ tag: search.tag === tag.key ? undefined : tag.key })}
                className={cn(
                  'flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[0.76rem] font-medium transition',
                  search.tag === tag.key
                    ? 'border-cobalt-600 bg-cobalt-600 text-white'
                    : 'border-cobalt-100 bg-white text-ink-500 hover:border-cobalt-300 hover:text-cobalt-700',
                )}
              >
                <span
                  className={cn(
                    'h-1.5 w-1.5 rounded-full',
                    search.tag === tag.key ? 'bg-white' : (TAG_TONE[tag.key] ?? 'bg-ink-300'),
                  )}
                />
                {tag.name}
                <span
                  className={cn(
                    'tabular-nums',
                    search.tag === tag.key ? 'text-white/70' : 'text-ink-300',
                  )}
                >
                  {tag.product_count}
                </span>
              </button>
            ))}
          </div>

          <span className="hidden h-5 w-px bg-cobalt-100 sm:block" />

          {/* selects */}
          <label className="flex items-center gap-1.5 text-[0.78rem] text-ink-400">
            {t('catalog.filterArtist')}
            <select
              value={search.artist ?? ''}
              onChange={(e) => setParam({ artist: e.target.value || undefined })}
              className="rounded border border-ink-300/50 bg-white px-2 py-1 text-[0.8rem] text-ink-700"
            >
              <option value="">{t('catalog.filterArtist')}</option>
              {data.artists.map((a) => (
                <option key={a.id} value={a.slug}>
                  {a.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex items-center gap-1.5 text-[0.78rem] text-ink-400">
            {t('catalog.filterEdition')}
            <select
              value={search.edition ?? ''}
              onChange={(e) =>
                setParam({ edition: (e.target.value || undefined) as 'one_of_a_kind' })
              }
              className="rounded border border-ink-300/50 bg-white px-2 py-1 text-[0.8rem] text-ink-700"
            >
              <option value="">{t('catalog.filterEdition')}</option>
              <option value="one_of_a_kind">{t('catalog.edition.one_of_a_kind')}</option>
              <option value="limited_edition">{t('catalog.edition.limited_edition')}</option>
              <option value="open_production">{t('catalog.edition.open_production')}</option>
            </select>
          </label>
          <label className="flex items-center gap-1.5 text-[0.78rem] text-ink-400">
            {t('catalog.sort')}
            <select
              value={search.sort ?? 'featured'}
              onChange={(e) => setParam({ sort: e.target.value as 'featured' })}
              className="rounded border border-ink-300/50 bg-white px-2 py-1 text-[0.8rem] text-ink-700"
            >
              <option value="featured">{t('catalog.sortFeatured')}</option>
              <option value="price_asc">{t('catalog.sortPriceAsc')}</option>
              <option value="price_desc">{t('catalog.sortPriceDesc')}</option>
              <option value="newest">{t('catalog.sortNewest')}</option>
            </select>
          </label>

          <span className="ml-auto flex items-center gap-3 text-[0.78rem] text-ink-400">
            {search.q && (
              <span className="flex items-center gap-1">
                “{search.q}”
                <button
                  type="button"
                  onClick={() => setParam({ q: undefined })}
                  aria-label="clear search"
                >
                  <X size={13} weight="bold" className="text-cobalt-600" />
                </button>
              </span>
            )}
            {hasFilters && (
              <button
                type="button"
                className="font-medium text-cobalt-600 hover:underline"
                onClick={() =>
                  setParam({
                    tag: undefined,
                    artist: undefined,
                    edition: undefined,
                    priceBand: undefined,
                    q: undefined,
                  })
                }
              >
                {t('catalog.clearFilters')}
              </button>
            )}
            <span className="tabular-nums">{t('catalog.results', { count: total })}</span>
          </span>
        </div>
      </div>

      {/* ------------------------------ crate ------------------------------ */}
      {total === 0 ? (
        <div className="mt-12">
          <EmptyState
            title={t('catalog.noResults')}
            action={
              <Button
                variant="secondary"
                onClick={() =>
                  setParam({
                    tag: undefined,
                    artist: undefined,
                    edition: undefined,
                    priceBand: undefined,
                    q: undefined,
                  })
                }
              >
                {t('catalog.noResultsCta')}
              </Button>
            }
          />
        </div>
      ) : (
        <div className="mt-10 grid items-center gap-8 xl:grid-cols-[1fr_21rem]">
          <div>
            <div
              ref={scroller.viewportRef}
              className="loop-viewport relative h-[440px] overflow-hidden rounded-lg border border-cobalt-100/70 bg-gradient-to-b from-mist to-porcelain/60 sm:h-[500px]"
              style={{ perspective: '1400px' }}
            >
              <div className="qinghua-watermark absolute inset-x-0 top-6 h-24 opacity-60" />
              <div
                ref={scroller.trackRef}
                className="relative flex h-full w-max items-center gap-10 px-6 will-change-transform"
              >
                {works.map((p, i) => (
                  <DeckCard
                    key={p.id}
                    product={p}
                    active={i === activeIdx}
                    onClick={() => (i === activeIdx ? undefined : scroller.scrollToPanel(i))}
                  />
                ))}
                {works.map((p, i) => (
                  <DeckCard
                    key={`${p.id}-copy`}
                    product={p}
                    active={i === activeIdx}
                    ariaHidden
                    onClick={() => undefined}
                  />
                ))}
              </div>

              {/* counter chip */}
              <span className="pointer-events-none absolute top-3 right-3 rounded-sm border border-cobalt-100 bg-white/90 px-2.5 py-1 text-[0.72rem] font-semibold text-ink-600 tabular-nums backdrop-blur">
                {String(activeIdx + 1).padStart(2, '0')} / {String(works.length).padStart(2, '0')}
              </span>
            </div>

            {/* flip controls */}
            <div className="mt-5 flex items-center justify-between">
              <p className="text-[0.78rem] text-ink-300">{t('catalog.crateHint')}</p>
              <div className="flex items-center gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  aria-label={t('catalog.prev')}
                  onClick={() => scroller.scrollToPanel(activeIdx - 1)}
                >
                  <ArrowLeft size={15} weight="bold" />
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  aria-label={t('catalog.next')}
                  onClick={() => scroller.scrollToPanel(activeIdx + 1)}
                >
                  <ArrowRight size={15} weight="bold" />
                </Button>
              </div>
            </div>
          </div>

          {/* ------------------------------ museum label / actions ------------------------------ */}
          <aside>
            {active ? (
              <ActiveLabel product={active} />
            ) : (
              <Spinner className="h-6 w-6 text-cobalt-400" />
            )}
          </aside>
        </div>
      )}

      <div className="mt-14 border-t border-cobalt-50 pt-6">
        <ButtonLink to={`/${locale}`} variant="ghost" size="sm">
          ← {t('common.brand')}
        </ButtonLink>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/* Deck card — framed cover + record disc + museum label               */
/* ------------------------------------------------------------------ */

function DeckCard({
  product,
  active,
  onClick,
  ariaHidden,
}: {
  product: Product
  active: boolean
  onClick?: () => void
  ariaHidden?: boolean
}) {
  const { t, locale } = useI18n()
  const skus = product.skus ?? []
  const sku = skus.reduce<import('~/lib/types').SKU | undefined>(
    (min, s) => (!min || (s.price ?? s.price_cny) < (min.price ?? min.price_cny) ? s : min),
    undefined,
  )
  const edition = sku?.attributes.edition_type

  return (
    <button
      type="button"
      data-panel
      data-deck
      data-parallax="0.62"
      onClick={onClick}
      aria-hidden={ariaHidden || undefined}
      aria-label={product.title}
      className={cn(
        'group relative block w-[280px] shrink-0 text-left transition-shadow duration-300 focus-visible:outline-none sm:w-[320px]',
        active ? 'cursor-default' : 'cursor-pointer',
      )}
      style={{ transformStyle: 'preserve-3d' }}
    >
      {/* museum window frame */}
      <div
        className={cn(
          'relative rounded-md border bg-white shadow-card transition-all duration-500',
          active ? 'z-10 -translate-x-5 shadow-pop' : 'shadow-lift group-hover:shadow-pop',
        )}
      >
        {/* family-color spine */}
        <span
          className={cn('absolute inset-y-0 left-0 w-1.5 rounded-l-md', spineOf(product.tags))}
        />
        {/* record disc peeking out when active */}
        <span
          aria-hidden="true"
          className={cn(
            'absolute top-1/2 right-2 h-[74%] w-[74%] -translate-y-1/2 rounded-full transition-transform duration-500',
            active ? 'translate-x-[34%]' : 'translate-x-0',
          )}
          style={{ transitionTimingFunction: 'cubic-bezier(0.22, 1, 0.36, 1)' }}
        >
          <svg viewBox="0 0 100 100" className="h-full w-full">
            <circle cx="50" cy="50" r="49" fill="#121f49" />
            {Array.from({ length: 7 }, (_, i) => (
              <circle
                key={i}
                cx="50"
                cy="50"
                r={14 + i * 5}
                fill="none"
                stroke="#e7eef9"
                strokeOpacity="0.14"
                strokeWidth="0.8"
              />
            ))}
            <circle cx="50" cy="50" r="12" fill="var(--cobalt-600)" />
            <circle cx="50" cy="50" r="3" fill="#ffffff" />
          </svg>
        </span>
        {/* the sleeve / cover */}
        <div className="relative ml-1.5 aspect-square overflow-hidden rounded-r-md bg-gradient-to-b from-wash to-porcelain">
          <CornerFrame inset={10} />
          <PorcelainFigure
            kind={product.figure_kind}
            seed={product.figure_seed}
            className="h-full w-full transition duration-500 group-hover:scale-[1.03]"
          />
          {edition === 'one_of_a_kind' && (
            <Badge tone="gold" className="absolute top-2.5 left-2.5 shadow-card">
              {t('catalog.edition.one_of_a_kind')}
            </Badge>
          )}
        </div>
      </div>

      {/* museum label */}
      <div
        className={cn(
          'mx-auto mt-4 w-[86%] rounded-sm border border-cobalt-100 bg-white px-3.5 py-2.5 shadow-card transition-all duration-300',
          active && 'border-cobalt-200',
        )}
      >
        <p className="truncate text-[0.86rem] font-semibold text-ink-800">{product.title}</p>
        <p className="mt-0.5 flex items-baseline justify-between gap-2 text-[0.74rem] text-ink-400">
          <span className="truncate">{product.artist_name}</span>
          {sku && (
            <span className="shrink-0 font-semibold text-ink-700">
              {sku.price !== undefined ? `${locale === 'zh-CN' ? '起' : 'from'} ` : ''}
              {new Intl.NumberFormat(locale, {
                style: 'currency',
                currency: sku.price_currency ?? 'CNY',
                minimumFractionDigits: (sku.price ?? sku.price_cny) % 100 === 0 ? 0 : 2,
              }).format((sku.price ?? sku.price_cny) / 100)}
            </span>
          )}
        </p>
      </div>
    </button>
  )
}

/* ------------------------------------------------------------------ */
/* Side panel — details + actions for the centered work                */
/* ------------------------------------------------------------------ */

function ActiveLabel({ product }: { product: Product }) {
  const { t, locale } = useI18n()
  const { token } = useAuth()
  const { add, busy } = useCart()
  const { toggle, has, ready } = useWishlist()
  const { push } = useToast()

  const skus = product.skus ?? []
  const sku = skus.reduce<import('~/lib/types').SKU | undefined>(
    (min, s) => (!min || (s.price ?? s.price_cny) < (min.price ?? min.price_cny) ? s : min),
    undefined,
  )
  const edition = sku?.attributes.edition_type
  const editionLabel =
    edition === 'one_of_a_kind'
      ? t('catalog.edition.one_of_a_kind')
      : edition === 'limited_edition'
        ? t('catalog.edition.limited_edition')
        : edition === 'open_production'
          ? t('catalog.edition.open_production')
          : undefined

  return (
    <div className="card-surface p-6">
      <p className="eyebrow">{t('catalog.viewing')}</p>
      {product.artist_slug && (
        <Link
          to="/$locale/artists/$slug"
          params={{ locale, slug: product.artist_slug }}
          className="mt-3 block text-[0.82rem] font-medium text-cobalt-600 hover:underline"
        >
          {t('product.byArtist', { artist: product.artist_name ?? '' })}
        </Link>
      )}
      <Link to="/$locale/catalog/$slug" params={{ locale, slug: product.slug }}>
        <h2 className="mt-1.5 text-[1.35rem] leading-snug font-semibold tracking-tight text-ink-900 hover:text-cobalt-700">
          {product.title}
        </h2>
      </Link>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {edition === 'one_of_a_kind' && (
          <Badge tone="gold">
            <SealCheck size={11} weight="fill" />
            {editionLabel}
          </Badge>
        )}
        {edition === 'limited_edition' && <Badge tone="cobalt">{editionLabel}</Badge>}
        {product.tags?.slice(0, 3).map((tag) => (
          <Badge key={tag.id} tone="neutral">
            {tag.name}
          </Badge>
        ))}
      </div>

      {sku && (
        <p className="mt-4 text-[1.6rem] font-semibold tracking-tight text-ink-900">
          {new Intl.NumberFormat(locale, {
            style: 'currency',
            currency: sku.price_currency ?? 'CNY',
            minimumFractionDigits: (sku.price ?? sku.price_cny) % 100 === 0 ? 0 : 2,
          }).format((sku.price ?? sku.price_cny) / 100)}
        </p>
      )}
      {sku && sku.stock > 0 && sku.stock <= sku.low_stock_threshold && (
        <p className="mt-1.5 text-[0.78rem] text-[color:var(--color-warning)]">
          {t('product.lowStock', { count: sku.stock })}
        </p>
      )}

      <div className="mt-5 flex items-center gap-2.5">
        <Button
          className="flex-1"
          disabled={!sku || sku.stock === 0}
          loading={busy}
          onClick={() =>
            void add(sku!.id)
              .then(() => push({ title: t('toast.addedToCart', { title: product.title }) }))
              .catch((err: unknown) =>
                push({
                  title: err instanceof Error ? err.message : t('errors.generic'),
                  kind: 'error',
                }),
              )
          }
        >
          <SealCheck size={16} weight="duotone" />
          {t('catalog.addActive')}
        </Button>
        {sku && (
          <HeartButton
            active={ready && has(sku.id)}
            label={t('nav.wishlist')}
            className="h-11 w-11"
            onClick={() => {
              if (!token) {
                push({ title: t('toast.wishlistNeedsLogin'), kind: 'info' })
                return
              }
              void toggle(sku!.id).then((r) =>
                push({
                  title: t(r === 'added' ? 'toast.addedToWishlist' : 'toast.removedFromWishlist'),
                }),
              )
            }}
          />
        )}
      </div>
      <ButtonLink
        to="/$locale/catalog/$slug"
        params={{ locale, slug: product.slug }}
        variant="secondary"
        className="mt-2.5 w-full"
      >
        {t('product.detailsTitle')} →
      </ButtonLink>
    </div>
  )
}
