import { createFileRoute } from '@tanstack/react-router'
import { MagnifyingGlass, SlidersHorizontal, X } from '@phosphor-icons/react'
import { useState } from 'react'
import { z } from 'zod'

import { ProductCard } from '~/components/cards'
import { Button, ButtonLink, EmptyState } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'
import { cn, loaderCurrency } from '~/lib/utils'

/**
 * Gallery / catalog list. Filters live in type-safe search params
 * (validateSearch + zod, TDD §6); the loader re-runs on param change.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).optional(),
  tag: z.string().optional(),
  artist: z.string().optional(),
  edition: z.enum(['one_of_a_kind', 'limited_edition', 'open_production']).optional(),
  priceBand: z.enum(['low', 'mid', 'high']).optional(),
  sort: z.enum(['featured', 'price_asc', 'price_desc', 'newest']).optional(),
  q: z.string().optional(),
})

export const Route = createFileRoute('/$locale/catalog/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search: { page, tag, artist, edition, priceBand, sort, q } }) => ({
    page,
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
        page: deps.page ?? 1,
        limit: 9,
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
  const { t, locale, currency } = useI18n()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const data = Route.useLoaderData()
  const [filtersOpen, setFiltersOpen] = useState(false)

  const setParam = (patch: Record<string, string | undefined>) => {
    void navigate({
      search: (prev: z.infer<typeof searchSchema>) => ({ ...prev, ...patch, page: undefined }),
      replace: true,
    })
  }

  const hasFilters = Boolean(search.tag || search.artist || search.edition || search.priceBand || search.q)
  const page = search.page ?? 1
  const total = data.products.total
  const totalPages = data.products.total_pages

  const editionOptions = [
    { key: 'one_of_a_kind', label: t('catalog.edition.one_of_a_kind') },
    { key: 'limited_edition', label: t('catalog.edition.limited_edition') },
    { key: 'open_production', label: t('catalog.edition.open_production') },
  ] as const

  const priceBands = [
    { key: 'low', label: t('catalog.priceUnder', { amount: '$500' }) },
    { key: 'mid', label: '$500 – $1,500' },
    { key: 'high', label: t('catalog.priceOver', { amount: '$1,500' }) },
  ] as const

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      {/* page head */}
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
          <MagnifyingGlass size={16} className="absolute top-1/2 left-3.5 -translate-y-1/2 text-ink-300" />
          <input
            name="q"
            defaultValue={search.q ?? ''}
            placeholder={t('common.search')}
            className="input-base h-10 w-56 pl-10"
            aria-label={t('common.search')}
          />
        </form>
      </div>

      <div className="mt-8 grid gap-10 lg:grid-cols-[15rem_1fr]">
        {/* ------------------------------ filters ------------------------------ */}
        <aside className={cn('lg:block', filtersOpen ? 'block' : 'hidden')} aria-label={t('catalog.filters')}>
          <div className="card-surface sticky top-24 p-5">
            <div className="flex items-center justify-between">
              <h2 className="flex items-center gap-2 text-[0.82rem] font-semibold tracking-wide text-ink-700 uppercase">
                <SlidersHorizontal size={15} className="text-cobalt-500" />
                {t('catalog.filters')}
              </h2>
              {hasFilters && (
                <button
                  type="button"
                  className="text-[0.78rem] font-medium text-cobalt-600 hover:underline"
                  onClick={() =>
                    setParam({ tag: undefined, artist: undefined, edition: undefined, priceBand: undefined, q: undefined })
                  }
                >
                  {t('catalog.clearFilters')}
                </button>
              )}
            </div>

            {/* tags */}
            <FilterGroup title={t('catalog.filterTags')}>
              <div className="flex flex-wrap gap-1.5">
                {data.tags.map((tag) => (
                  <button
                    key={tag.key}
                    type="button"
                    onClick={() => setParam({ tag: search.tag === tag.key ? undefined : tag.key })}
                    className={cn(
                      'rounded-full border px-2.5 py-1 text-[0.76rem] font-medium transition',
                      search.tag === tag.key
                        ? 'border-cobalt-600 bg-cobalt-600 text-white'
                        : 'border-cobalt-100 bg-white text-ink-500 hover:border-cobalt-300 hover:text-cobalt-700',
                    )}
                  >
                    {tag.name}
                  </button>
                ))}
              </div>
            </FilterGroup>

            {/* artists */}
            <FilterGroup title={t('catalog.filterArtist')}>
              <div className="flex flex-col gap-1">
                {data.artists.map((a) => (
                  <button
                    key={a.id}
                    type="button"
                    onClick={() => setParam({ artist: search.artist === a.slug ? undefined : a.slug })}
                    className={cn(
                      'flex items-center justify-between rounded-md px-2.5 py-1.5 text-[0.84rem] transition',
                      search.artist === a.slug
                        ? 'bg-cobalt-50 font-semibold text-cobalt-700'
                        : 'text-ink-600 hover:bg-mist',
                    )}
                  >
                    {a.name}
                  </button>
                ))}
              </div>
            </FilterGroup>

            {/* edition */}
            <FilterGroup title={t('catalog.filterEdition')}>
              <div className="flex flex-col gap-1">
                {editionOptions.map((opt) => (
                  <button
                    key={opt.key}
                    type="button"
                    onClick={() => setParam({ edition: search.edition === opt.key ? undefined : opt.key })}
                    className={cn(
                      'flex items-center gap-2 rounded-md px-2.5 py-1.5 text-[0.84rem] transition',
                      search.edition === opt.key
                        ? 'bg-cobalt-50 font-semibold text-cobalt-700'
                        : 'text-ink-600 hover:bg-mist',
                    )}
                  >
                    <span
                      className={cn(
                        'h-3.5 w-3.5 rounded-full border',
                        search.edition === opt.key ? 'border-[4.5px] border-cobalt-600' : 'border-ink-300',
                      )}
                    />
                    {opt.label}
                  </button>
                ))}
              </div>
            </FilterGroup>

            {/* price */}
            <FilterGroup title={t('catalog.filterPrice')}>
              <div className="flex flex-col gap-1">
                {priceBands.map((band) => (
                  <button
                    key={band.key}
                    type="button"
                    onClick={() => setParam({ priceBand: search.priceBand === band.key ? undefined : band.key })}
                    className={cn(
                      'flex items-center gap-2 rounded-md px-2.5 py-1.5 text-[0.84rem] transition',
                      search.priceBand === band.key
                        ? 'bg-cobalt-50 font-semibold text-cobalt-700'
                        : 'text-ink-600 hover:bg-mist',
                    )}
                  >
                    <span
                      className={cn(
                        'h-3.5 w-3.5 rounded-full border',
                        search.priceBand === band.key ? 'border-[4.5px] border-cobalt-600' : 'border-ink-300',
                      )}
                    />
                    {band.label}
                  </button>
                ))}
              </div>
            </FilterGroup>

            <p className="mt-4 border-t border-cobalt-50 pt-3 text-[0.72rem] leading-relaxed text-ink-300">
              {t('cart.fxNote')}
            </p>
          </div>
        </aside>

        {/* ------------------------------ results ------------------------------ */}
        <div>
          <div className="mb-5 flex items-center justify-between gap-3">
            <button
              type="button"
              className="flex items-center gap-2 rounded-lg border border-cobalt-100 bg-white px-3 py-2 text-[0.82rem] font-medium text-ink-600 lg:hidden"
              onClick={() => setFiltersOpen((v) => !v)}
            >
              {filtersOpen ? <X size={15} /> : <SlidersHorizontal size={15} />}
              {t('catalog.filters')}
            </button>
            <p className="hidden text-[0.84rem] text-ink-400 sm:block">
              {t('catalog.results', { count: total })}
            </p>
            <label className="ml-auto flex items-center gap-2 text-[0.82rem] text-ink-400">
              {t('catalog.sort')}
              <select
                value={search.sort ?? 'featured'}
                onChange={(e) => setParam({ sort: e.target.value as 'featured' })}
                className="rounded-lg border border-ink-300/50 bg-white px-2.5 py-1.5 text-[0.82rem] text-ink-700"
              >
                <option value="featured">{t('catalog.sortFeatured')}</option>
                <option value="price_asc">{t('catalog.sortPriceAsc')}</option>
                <option value="price_desc">{t('catalog.sortPriceDesc')}</option>
                <option value="newest">{t('catalog.sortNewest')}</option>
              </select>
            </label>
          </div>

          {search.q && (
            <div className="mb-5 flex items-center gap-2 text-sm text-ink-500">
              <span>
                “{search.q}” — {t('catalog.results', { count: total })}
              </span>
              <button type="button" onClick={() => setParam({ q: undefined })} className="text-cobalt-600 hover:underline">
                <X size={14} weight="bold" />
              </button>
            </div>
          )}

          {total === 0 ? (
            <EmptyState
              title={t('catalog.noResults')}
              action={
                <Button variant="secondary" onClick={() => setParam({ tag: undefined, artist: undefined, edition: undefined, priceBand: undefined, q: undefined })}>
                  {t('catalog.noResultsCta')}
                </Button>
              }
            />
          ) : (
            <>
              <div className="grid gap-6 sm:grid-cols-2 xl:grid-cols-3">
                {data.products.data.map((p) => (
                  <ProductCard key={p.id} product={p} />
                ))}
              </div>

              {totalPages > 1 && (
                <nav className="mt-12 flex items-center justify-center gap-1.5" aria-label="pagination">
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={page <= 1}
                    onClick={() => void navigate({ search: { ...search, page: page - 1 } })}
                  >
                    ←
                  </Button>
                  {Array.from({ length: totalPages }, (_, i) => i + 1).map((n) => (
                    <button
                      key={n}
                      type="button"
                      onClick={() => void navigate({ search: { ...search, page: n } })}
                      className={cn(
                        'h-8 w-8 rounded-lg text-[0.82rem] font-medium transition',
                        n === page ? 'bg-cobalt-600 text-white' : 'text-ink-500 hover:bg-mist',
                      )}
                    >
                      {n}
                    </button>
                  ))}
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={page >= totalPages}
                    onClick={() => void navigate({ search: { ...search, page: page + 1 } })}
                  >
                    →
                  </Button>
                </nav>
              )}
            </>
          )}

          <div className="mt-14 border-t border-cobalt-50 pt-6">
            <ButtonLink to={`/${locale}`} variant="ghost" size="sm">
              ← {t('landing.ctaGallery')}
            </ButtonLink>
          </div>
        </div>
      </div>

      {/* currency hint (presentment refetch happens on navigation) */}
      <span className="hidden">{currency}</span>
    </div>
  )
}

function FilterGroup({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mt-5 border-t border-cobalt-50 pt-4 first-of-type:border-0">
      <h3 className="mb-2.5 text-[0.78rem] font-semibold tracking-wide text-ink-600 uppercase">{title}</h3>
      {children}
    </div>
  )
}
