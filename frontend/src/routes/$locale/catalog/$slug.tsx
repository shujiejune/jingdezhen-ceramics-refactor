import { createFileRoute, Link, notFound, useNavigate } from '@tanstack/react-router'
import { ArrowRight, SealCheck } from '@phosphor-icons/react'
import { useState } from 'react'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { CertificateChip, ProductCard, WaveDivider } from '~/components/cards'
import { BrushRule, CornerFrame } from '~/components/ornaments'
import { Badge, Breadcrumbs, Button, HeartButton, QuantityStepper, SectionHeading } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { JsonLd } from '~/components/seo/JsonLd'
import { api, ApiError } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'
import { useWishlist } from '~/lib/wishlist'
import { cn, loaderCurrency } from '~/lib/utils'

/**
 * Product detail — SKUs, gallery, certificate, artist cross-link, SEO
 * meta + JSON-LD from the translation row (PRD §3.2.1 / §4.4).
 */
export const Route = createFileRoute('/$locale/catalog/$slug')({
  loader: async ({ params }) => {
    try {
      const currency = await loaderCurrency()
      const product = await api.getProduct(params.slug, params.locale, currency)
      const more = await api.getProducts({ locale: params.locale, currency, limit: 48 })
      return {
        product,
        more: more.data.filter((p) => p.artist_id === product.artist_id && p.id !== product.id).slice(0, 3),
      }
    } catch (e) {
      if (e instanceof ApiError && e.is('not_found')) throw notFound()
      throw e
    }
  },
  head: ({ loaderData }) => ({
    meta: loaderData
      ? [
          { title: loaderData.product.meta_title ?? loaderData.product.title },
          {
            name: 'description',
            content: loaderData.product.meta_description ?? loaderData.product.description?.slice(0, 155),
          },
          { property: 'og:title', content: loaderData.product.meta_title ?? loaderData.product.title },
          { property: 'og:type', content: 'product' },
        ]
      : [],
  }),
  notFoundComponent: () => <ProductNotFound />,
  component: ProductDetail,
})

function ProductNotFound() {
  const { t } = useI18n()
  return (
    <div className="mx-auto max-w-shell px-6 py-32 text-center">
      <h1 className="text-display-sm text-ink-900">{t('errors.not_found')}</h1>
    </div>
  )
}

/* ------------------------------------------------------------------ */

function ProductDetail() {
  const { t, locale, price } = useI18n()
  const { product, more } = Route.useLoaderData()
  const { add, busy } = useCart()
  const { toggle, has, ready } = useWishlist()
  const { token } = useAuth()
  const { push } = useToast()
  const navigate = useNavigate()

  const [activeMedia, setActiveMedia] = useState(0)
  const [skuIndex, setSkuIndex] = useState(0)
  const [qty, setQty] = useState(1)

  const skus = product.skus ?? []
  const sku = skus[Math.min(skuIndex, Math.max(0, skus.length - 1))]
  const gallery = product.gallery ?? []

  const editionLabel = (et: string | undefined) =>
    et === 'one_of_a_kind'
      ? t('catalog.edition.one_of_a_kind')
      : et === 'limited_edition'
        ? t('catalog.edition.limited_edition')
        : et === 'open_production'
          ? t('catalog.edition.open_production')
          : undefined

  const onAdd = async () => {
    if (!sku) return
    try {
      await add(sku.id, qty)
      push({ title: t('toast.addedToCart', { title: product.title }) })
    } catch (err) {
      push({ title: err instanceof Error ? err.message : t('errors.generic'), kind: 'error' })
    }
  }

  const onHeart = async () => {
    if (!token || !sku) {
      push({ title: t('toast.wishlistNeedsLogin'), kind: 'info' })
      return
    }
    const result = await toggle(sku.id)
    push({ title: t(result === 'added' ? 'toast.addedToWishlist' : 'toast.removedFromWishlist') })
  }

  const attrRows: Array<[string, string | number | undefined]> = [
    [t('product.attr.size'), sku?.attributes.size],
    [t('product.attr.technique'), sku?.attributes.technique],
    [t('product.attr.glaze'), sku?.attributes.glaze],
    [t('product.attr.edition_type'), editionLabel(sku?.attributes.edition_type)],
    [
      t('product.attr.edition_number'),
      sku?.attributes.edition_number && sku?.attributes.edition_type === 'limited_edition'
        ? sku.attributes.edition_number
        : undefined,
    ],
    [t('product.attr.year'), sku?.attributes.year],
    [t('product.attr.kiln'), sku?.attributes.kiln],
  ].filter((row): row is [string, string | number] => row[1] !== undefined)

  return (
    <div className="mx-auto max-w-shell px-4 pt-8 sm:px-6">
      <Breadcrumbs
        items={[
          { label: t('nav.gallery'), to: `/${locale}/catalog` },
          ...(product.artist_slug
            ? [{ label: product.artist_name ?? '', to: `/${locale}/artists/$slug`, params: { slug: product.artist_slug } }]
            : []),
          { label: product.title },
        ]}
      />

      <div className="grid gap-12 lg:grid-cols-[1fr_1fr] xl:grid-cols-[1.05fr_0.95fr]">
        {/* ------------------------------ gallery ------------------------------ */}
        <div>
          <div className="relative overflow-hidden rounded-2xl border border-cobalt-100 bg-gradient-to-b from-wash to-porcelain shadow-card">
            <CornerFrame inset={16} />
            <PorcelainFigure
              kind={product.figure_kind}
              seed={gallery[activeMedia]?.figure_seed ?? product.figure_seed}
              className="aspect-square w-full"
              label={product.title}
            />
          </div>
          {gallery.length > 1 && (
            <div className="mt-4 flex gap-3">
              {gallery.map((m, i) => (
                <button
                  key={m.media_id}
                  type="button"
                  onClick={() => setActiveMedia(i)}
                  aria-label={`${product.title} ${i + 1}`}
                  className={cn(
                    'h-20 w-20 overflow-hidden rounded-lg border bg-wash transition',
                    i === activeMedia ? 'border-cobalt-500 ring-2 ring-cobalt-200' : 'border-cobalt-100 hover:border-cobalt-300',
                  )}
                >
                  <PorcelainFigure kind={m.figure_kind} seed={m.figure_seed} className="h-full w-full" />
                </button>
              ))}
            </div>
          )}
        </div>

        {/* ------------------------------ buy panel ------------------------------ */}
        <div className="lg:pt-2">
          {product.artist_name && (
            <Link
              to="/$locale/artists/$slug"
              params={{ locale, slug: product.artist_slug ?? '' }}
              className="text-[0.82rem] font-medium text-cobalt-600 hover:underline"
            >
              {t('product.byArtist', { artist: product.artist_name })}
            </Link>
          )}
          <h1 className="mt-2 text-display-sm text-ink-900">{product.title}</h1>
          <BrushRule className="mt-4" />

          <div className="mt-5 flex flex-wrap items-center gap-2">
            {sku?.attributes.edition_type === 'one_of_a_kind' && (
              <Badge tone="gold">
                <SealCheck size={12} weight="fill" /> {t('product.oneOfAKind')}
              </Badge>
            )}
            {sku && sku.stock > 0 && sku.stock <= sku.low_stock_threshold && (
              <Badge tone="warning">{t('product.lowStock', { count: sku.stock })}</Badge>
            )}
            {sku?.stock === 0 && <Badge tone="danger">{t('product.outOfStock')}</Badge>}
          </div>

          <p className="mt-5 text-[2rem] leading-none font-semibold tracking-tight text-ink-900">
            {price(sku?.price ?? sku?.price_cny, sku?.price_currency ?? undefined)}
          </p>
          <p className="mt-1.5 text-[0.78rem] text-ink-300">{t('cart.fxNote')}</p>

          {/* SKU selector */}
          {skus.length > 0 && (
            <div className="mt-7">
              <h2 className="label-base">{skus.length > 1 ? t('product.selectVariant') : t('product.buyingOptions')}</h2>
              <div className="flex flex-col gap-2">
                {skus.map((s, i) => {
                  const label = [s.attributes.size, editionLabel(s.attributes.edition_type)]
                    .filter(Boolean)
                    .join(' · ')
                  return (
                    <button
                      key={s.id}
                      type="button"
                      disabled={s.stock === 0}
                      onClick={() => {
                        setSkuIndex(i)
                        setQty(1)
                      }}
                      className={cn(
                        'flex items-center justify-between rounded-xl border px-4 py-3 text-left transition disabled:opacity-45',
                        i === skuIndex
                          ? 'border-cobalt-500 bg-cobalt-50/60 ring-1 ring-cobalt-200'
                          : 'border-ink-300/40 bg-white hover:border-cobalt-300',
                      )}
                    >
                      <span>
                        <span className="block text-[0.9rem] font-medium text-ink-800">{label || s.sku_code}</span>
                        <span className="mt-0.5 block text-[0.75rem] text-ink-400">
                          {s.stock === 0
                            ? t('product.outOfStock')
                            : s.stock <= s.low_stock_threshold
                              ? t('product.lowStock', { count: s.stock })
                              : t('product.inStock')}
                        </span>
                      </span>
                      <span className="text-[0.92rem] font-semibold text-ink-900">
                        {price(s.price ?? s.price_cny, s.price_currency ?? undefined)}
                      </span>
                    </button>
                  )
                })}
              </div>
            </div>
          )}

          {/* qty + add */}
          <div className="mt-7 flex items-center gap-3">
            <QuantityStepper value={qty} max={Math.max(1, sku?.stock ?? 1)} onChange={setQty} disabled={sku?.stock === 0} />
            <Button size="lg" className="flex-1" onClick={onAdd} loading={busy} disabled={sku?.stock === 0}>
              <SealCheck size={17} weight="duotone" />
              {t('common.addToCart')}
            </Button>
            {sku && (
              <HeartButton
                active={ready && has(sku.id)}
                onClick={onHeart}
                label={t('nav.wishlist')}
                className="h-12 w-12"
              />
            )}
          </div>

          {/* certificate */}
          {product.cert_code && (
            <div className="mt-6">
              <CertificateChip
                onClick={() =>
                  void navigate({ to: '/$locale/certificates/$code', params: { locale, code: product.cert_code! } })
                }
              />
            </div>
          )}

          {/* details */}
          <div className="mt-8 rounded-xl border border-cobalt-100 bg-wash/60 p-5">
            <h2 className="text-[0.82rem] font-semibold tracking-wide text-ink-600 uppercase">{t('product.detailsTitle')}</h2>
            <dl className="mt-3 grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
              {attrRows.map(([k, v]) => (
                <div key={k} className="flex items-baseline justify-between gap-3 border-b border-cobalt-100/60 pb-1.5">
                  <dt className="text-[0.8rem] text-ink-400">{k}</dt>
                  <dd className="text-[0.84rem] font-medium text-ink-700">{v}</dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      </div>

      {/* description */}
      {product.description && (
        <section className="mx-auto mt-16 max-w-2xl">
          <WaveDivider className="mb-8" />
          <h2 className="text-center text-[1.3rem] font-semibold text-ink-800">{t('product.aboutTitle')}</h2>
          <p className="mt-5 leading-[1.85] text-ink-600">{product.description}</p>
        </section>
      )}

      {/* more from artist */}
      {more.length > 0 && (
        <section className="mt-20">
          <SectionHeading
            eyebrow={t('landing.artistsEyebrow')}
            title={t('product.moreFromArtist')}
            action={
              product.artist_slug ? (
                <Link
                  to="/$locale/artists/$slug"
                  params={{ locale, slug: product.artist_slug ?? '' }}
                  className="inline-flex items-center gap-1 text-[0.84rem] font-medium text-cobalt-600 hover:underline"
                >
                  {product.artist_name}
                  <ArrowRight size={13} weight="bold" />
                </Link>
              ) : undefined
            }
          />
          <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {more.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        </section>
      )}

      {/* SEO: Product + BreadcrumbList JSON-LD */}
      <JsonLd
        data={{
          '@context': 'https://schema.org',
          '@type': 'Product',
          name: product.title,
          description: product.meta_description ?? product.description,
          sku: sku?.sku_code,
          brand: { '@type': 'Brand', name: product.artist_name ?? 'Jingdezhen' },
          offers: sku
            ? {
                '@type': 'Offer',
                price: ((sku.price ?? sku.price_cny) / 100).toFixed(2),
                priceCurrency: sku.price_currency ?? 'CNY',
                availability: sku.stock > 0 ? 'https://schema.org/InStock' : 'https://schema.org/OutOfStock',
              }
            : undefined,
        }}
      />
      <JsonLd
        data={{
          '@context': 'https://schema.org',
          '@type': 'BreadcrumbList',
          itemListElement: [
            { '@type': 'ListItem', position: 1, name: 'Gallery', item: `/${locale}/catalog` },
            { '@type': 'ListItem', position: 2, name: product.title },
          ],
        }}
      />
    </div>
  )
}

