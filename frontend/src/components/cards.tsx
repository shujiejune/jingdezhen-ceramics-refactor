/** Entity cards — gallery, artists, heritage stories, engage activities. */
import { Link } from '@tanstack/react-router'
import { ArrowRight, SealCheck } from '@phosphor-icons/react'

import { PorcelainFigure, PorcelainLandscape, ArtistMedallion } from '~/components/artwork/PorcelainFigure'
import { WaveBand } from '~/components/ornaments'
import { Badge, HeartButton } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { useWishlist } from '~/lib/wishlist'
import type { Activity, Artist, CeramicStory, Product, SKU } from '~/lib/types'
import { cn } from '~/lib/utils'

/* ------------------------------ Product ------------------------------ */

function cheapestInStock(skus: SKU[]): SKU | undefined {
  const active = skus.filter((s) => s.is_active)
  const pool = active.length ? active : skus
  return pool.reduce<SKU | undefined>(
    (min, s) => (!min || priceOf(s) < priceOf(min) ? s : min),
    undefined,
  )
}

function priceOf(sku: SKU): number {
  return sku.price ?? sku.price_cny
}

export function ProductCard({ product, priority }: { product: Product; priority?: boolean }) {
  const { t, locale, price } = useI18n()
  const { toggle, has, ready } = useWishlist()
  const { token } = useAuth()
  const { push } = useToast()

  const skus = product.skus ?? []
  const sku = cheapestInStock(skus)
  const edition = sku?.attributes.edition_type
  const soldOut = skus.length > 0 && skus.every((s) => s.stock <= 0)

  const onHeart = async (e: React.MouseEvent) => {
    e.preventDefault()
    if (!token || !sku) {
      push({ title: t('toast.wishlistNeedsLogin'), kind: 'info' })
      return
    }
    const result = await toggle(sku.id)
    push({ title: t(result === 'added' ? 'toast.addedToWishlist' : 'toast.removedFromWishlist') })
  }

  return (
    <article className="group card-surface relative overflow-hidden transition duration-300 hover:-translate-y-1 hover:shadow-lift">
      <Link
        to="/$locale/catalog/$slug"
        params={{ locale, slug: product.slug }}
        className="block"
        aria-label={product.title}
      >
        <div className="relative aspect-[4/4.4] overflow-hidden bg-gradient-to-b from-wash to-porcelain">
          <div className="qinghua-watermark absolute inset-0 opacity-70" />
          <PorcelainFigure
            kind={product.figure_kind}
            seed={product.figure_seed}
            className={cn(
              'relative h-full w-full transition duration-500 group-hover:scale-[1.04]',
              priority && 'animate-[fadein_0.4s_ease]',
            )}
          />
          {edition === 'one_of_a_kind' && (
            <Badge tone="gold" className="absolute top-3 left-3 shadow-card">
              {t('catalog.edition.one_of_a_kind')}
            </Badge>
          )}
          {soldOut && (
            <Badge tone="neutral" className="absolute top-3 left-3 shadow-card">
              {t('product.outOfStock')}
            </Badge>
          )}
        </div>
      </Link>

      {sku && (
        <HeartButton
          active={ready && has(sku.id)}
          onClick={onHeart}
          label={t('nav.wishlist')}
          className="absolute top-3 right-3 shadow-card"
        />
      )}

      <div className="flex items-start justify-between gap-3 px-5 py-4">
        <div className="min-w-0">
          <Link to="/$locale/catalog/$slug" params={{ locale, slug: product.slug }}>
            <h3 className="truncate text-[0.92rem] font-semibold text-ink-800 transition group-hover:text-cobalt-700">
              {product.title}
            </h3>
          </Link>
          <p className="mt-0.5 truncate text-[0.8rem] text-ink-400">{product.artist_name}</p>
        </div>
        {sku && (
          <p className="shrink-0 text-[0.92rem] font-semibold whitespace-nowrap text-ink-900">
            {price(sku.price ?? sku.price_cny, sku.price_currency ?? undefined)}
          </p>
        )}
      </div>
    </article>
  )
}

/* ------------------------------ Artist ------------------------------ */

export function ArtistCard({ artist, works }: { artist: Artist; works?: number }) {
  const { t, locale } = useI18n()
  return (
    <Link
      to="/$locale/artists/$slug"
      params={{ locale, slug: artist.slug }}
      className="card-surface group flex flex-col items-center px-6 pt-8 pb-6 text-center transition duration-300 hover:-translate-y-1 hover:shadow-lift"
    >
      <ArtistMedallion glyph={artist.glyph} seed={artist.id} size={76} />
      <h3 className="mt-4 text-[0.95rem] font-semibold text-ink-800 transition group-hover:text-cobalt-700">
        {artist.name}
      </h3>
      {works !== undefined && (
        <p className="mt-1 text-[0.78rem] tracking-wide text-cobalt-500">
          {works} {works === 1 ? 'work' : 'works'}
        </p>
      )}
      <p className="mt-3 line-clamp-3 text-[0.82rem] leading-relaxed text-ink-500">{artist.bio}</p>
      <span className="mt-4 inline-flex items-center gap-1 text-[0.8rem] font-medium text-cobalt-600">
        {t('common.learnMore')}
        <ArrowRight size={12} className="transition group-hover:translate-x-0.5" />
      </span>
    </Link>
  )
}

/* ------------------------------ Story ------------------------------ */

export function StoryCard({ story }: { story: CeramicStory }) {
  const { t, locale } = useI18n()
  return (
    <Link
      to="/$locale/ceramicstory/$slug"
      params={{ locale, slug: story.slug }}
      className="card-surface group relative flex items-stretch gap-5 overflow-hidden p-5 transition duration-300 hover:-translate-y-0.5 hover:shadow-lift"
    >
      <div className="w-24 shrink-0 overflow-hidden rounded-lg bg-gradient-to-b from-wash to-porcelain sm:w-32">
        <PorcelainFigure kind="vase" seed={story.figure_seed} className="h-full w-full" />
      </div>
      <div className="min-w-0 py-1">
        <p className="eyebrow">{String(story.dynasty_start_year)}</p>
        <h3 className="mt-1.5 text-[1.02rem] leading-snug font-semibold text-ink-800 transition group-hover:text-cobalt-700">
          {story.title}
        </h3>
        <p className="mt-2 line-clamp-2 text-[0.84rem] leading-relaxed text-ink-500">{story.summary}</p>
        <span className="mt-3 inline-flex items-center gap-1 text-[0.8rem] font-medium text-cobalt-600">
          {t('story.read')}
          <ArrowRight size={12} className="transition group-hover:translate-x-0.5" />
        </span>
      </div>
    </Link>
  )
}

/* ------------------------------ Activity ------------------------------ */

export function ActivityCard({ activity }: { activity: Activity }) {
  const { t, locale } = useI18n()
  return (
    <Link
      to="/$locale/engage/$slug"
      params={{ locale, slug: activity.slug }}
      className="card-surface group flex flex-col overflow-hidden transition duration-300 hover:-translate-y-1 hover:shadow-lift"
    >
      <div className="relative aspect-[4/3] overflow-hidden">
        <PorcelainLandscape
          seed={activity.figure_seed}
          className="h-full w-full transition duration-500 group-hover:scale-[1.05]"
        />
        <Badge tone={activity.type === 'destination' ? 'cobalt' : 'neutral'} className="absolute top-3 left-3 shadow-card">
          {activity.type === 'destination' ? t('engage.destinations') : t('engage.lifestyle')}
        </Badge>
      </div>
      <div className="flex flex-1 flex-col px-5 py-4">
        <h3 className="text-[0.98rem] font-semibold text-ink-800 transition group-hover:text-cobalt-700">
          {activity.title}
        </h3>
        <p className="mt-1.5 line-clamp-2 flex-1 text-[0.84rem] leading-relaxed text-ink-500">{activity.summary}</p>
        <span className="mt-3 inline-flex items-center gap-1 text-[0.8rem] font-medium text-cobalt-600">
          {t('engage.readMore')}
          <ArrowRight size={12} className="transition group-hover:translate-x-0.5" />
        </span>
      </div>
    </Link>
  )
}

/* --------------------------- Certificate chip --------------------------- */

export function CertificateChip({ onClick }: { onClick: () => void }) {
  const { t } = useI18n()
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-2 rounded-lg border border-gold-400/30 bg-gold-400/8 px-3.5 py-2.5 text-left transition hover:bg-gold-400/15"
    >
      <SealCheck size={22} className="text-gold-500" weight="duotone" />
      <span>
        <span className="block text-[0.82rem] font-semibold text-ink-800">{t('product.certificateTitle')}</span>
        <span className="block text-[0.75rem] text-cobalt-600 underline decoration-cobalt-200 underline-offset-2">
          {t('product.viewCertificate')}
        </span>
      </span>
    </button>
  )
}

/* ------------------------- Decorative divider ------------------------- */

export function WaveDivider({ className }: { className?: string }) {
  return (
    <div className={cn('flex items-center justify-center gap-4', className)}>
      <span className="h-px w-16 bg-gradient-to-r from-transparent to-cobalt-200" />
      <WaveBand width={72} opacity={0.65} />
      <span className="h-px w-16 bg-gradient-to-l from-transparent to-cobalt-200" />
    </div>
  )
}
