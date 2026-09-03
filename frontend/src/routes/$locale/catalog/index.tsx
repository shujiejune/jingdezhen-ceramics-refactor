import { createFileRoute } from '@tanstack/react-router'
import { ArrowLeft, ArrowRight, MagnifyingGlass, X } from '@phosphor-icons/react'
import { z } from 'zod'

import { ProductCard } from '~/components/cards'
import { Button, ButtonLink, EmptyState } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'
import { cn, loaderCurrency } from '~/lib/utils'

/**
 * Gallery — a plain, boring grid (user decision 2026-08-15: the
 * record-crate experiment was reverted). Sticky filter bar with family
 * tag chips + selects; type-safe search params via validateSearch/zod;
 * classic pagination. Prices come from the API list view (SKUs included).
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

/** family color for a tag chip dot */
const TAG_TONE: Record<string, string> = {
  qinghua: 'bg-cobalt-600',
  fencai: 'bg-rose-500',
  linglong: 'bg-celadon-500',
  yanseyou: 'bg-cinnabar-500',
  enamel: 'bg-imperial-400',
}

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
  loader: async ({ context, params, deps }) => {
    const locale = params.locale
    const currency = await loaderCurrency()
    const { queryClient } = context
    const query = {
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
    }
    const [products, tags, artists] = await Promise.all([
      queryClient.ensureQueryData({
        queryKey: ['products', locale, currency, query],
        queryFn: () => api.getProducts(query),
      }),
      queryClient.ensureQueryData({
        queryKey: ['tags', locale],
        queryFn: () => api.getTags(locale),
      }),
      queryClient.ensureQueryData({
        queryKey: ['artists', locale],
        queryFn: () => api.getArtists(locale),
      }),
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
  const total = data.products.total
  const totalPages = data.products.total_pages

  const setParam = (patch: Record<string, string | number | undefined>) => {
    void navigate({ search: (prev) => ({ ...prev, page: undefined, ...patch }), replace: true })
  }

  const hasFilters = Boolean(
    search.tag || search.artist || search.edition || search.priceBand || search.q,
  )
  const page = search.page ?? 1

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
                  aria-label={t('common.clearSearch')}
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

      {/* ------------------------------ grid ------------------------------ */}
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
        <>
          <div className="mt-10 grid gap-6 sm:grid-cols-2 xl:grid-cols-3">
            {data.products.data.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>

          {totalPages > 1 && (
            <nav
              className="mt-12 flex items-center justify-center gap-1.5"
              aria-label={t('catalog.pagination')}
            >
              <Button
                variant="secondary"
                size="sm"
                aria-label={t('catalog.prevPage')}
                disabled={page <= 1}
                onClick={() => void navigate({ search: { ...search, page: page - 1 } })}
              >
                <ArrowLeft size={15} weight="bold" />
              </Button>
              {Array.from({ length: totalPages }, (_, i) => i + 1).map((n) => (
                <button
                  key={n}
                  type="button"
                  aria-current={n === page ? 'page' : undefined}
                  aria-label={`${t('catalog.pagination')} ${n}`}
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
                aria-label={t('catalog.nextPage')}
                disabled={page >= totalPages}
                onClick={() => void navigate({ search: { ...search, page: page + 1 } })}
              >
                <ArrowRight size={15} weight="bold" />
              </Button>
            </nav>
          )}
        </>
      )}

      <div className="mt-14 border-t border-cobalt-50 pt-6">
        <ButtonLink to={`/${locale}`} variant="ghost" size="sm">
          ← {t('common.brand')}
        </ButtonLink>
      </div>
    </div>
  )
}
