import { createFileRoute, notFound } from '@tanstack/react-router'

import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { ContentBlocks } from '~/components/content/ContentBlocks'
import { CornerFrame, PetalScatter } from '~/components/ornaments'
import { Breadcrumbs } from '~/components/common/ui'
import { JsonLd } from '~/components/seo/JsonLd'
import { api, ApiError } from '~/lib/api'
import { useI18n } from '~/lib/i18n'

export const Route = createFileRoute('/$locale/ceramicstory/$slug')({
  loader: async ({ params }) => {
    try {
      return await api.getStory(params.slug, params.locale)
    } catch (e) {
      if (e instanceof ApiError && e.is('not_found')) throw notFound()
      throw e
    }
  },
  head: ({ loaderData }) => ({
    meta: loaderData ? [{ title: loaderData.title }, { name: 'description', content: loaderData.summary }] : [],
  }),
  notFoundComponent: () => {
    const { t } = useI18n()
    return (
      <div className="mx-auto max-w-shell px-6 py-32 text-center">
        <h1 className="text-display-sm text-ink-900">{t('errors.not_found')}</h1>
      </div>
    )
  },
  component: StoryPage,
})

function StoryPage() {
  const { t, locale } = useI18n()
  const story = Route.useLoaderData()

  return (
    <article className="mx-auto max-w-shell px-4 sm:px-6">
      <Breadcrumbs items={[{ label: t('story.title'), to: `/${locale}/ceramicstory` }, { label: story.title }]} />

      {/* editorial hero */}
      <header className="relative mt-2 overflow-hidden rounded-2xl border border-cobalt-100 bg-gradient-to-b from-porcelain/70 to-wash">
        <PetalScatter seed={story.figure_seed} className="pointer-events-none absolute top-6 right-10 opacity-50" />
        <div className="relative mx-auto grid max-w-4xl items-center gap-8 px-6 py-12 sm:grid-cols-[1fr_13rem] sm:px-10">
          <div>
            <p className="eyebrow">{String(story.dynasty_start_year)}</p>
            <h1 className="mt-3 text-display-sm text-ink-900">{story.title}</h1>
            <p className="mt-4 max-w-xl leading-relaxed text-ink-500">{story.summary}</p>
          </div>
          <div className="relative mx-auto w-48 overflow-hidden rounded-xl border border-cobalt-100 bg-white shadow-card">
            <PorcelainFigure kind="vase" seed={story.figure_seed} className="h-auto w-full" />
            <CornerFrame inset={8} />
          </div>
        </div>
      </header>

      {/* body */}
      <div className="mx-auto mt-14 max-w-[42rem] pb-8">
        <ContentBlocks blocks={story.content} />

        <JsonLd
          data={{
            '@context': 'https://schema.org',
            '@type': 'Article',
            headline: story.title,
            description: story.summary,
            inLanguage: locale,
          }}
        />
      </div>
    </article>
  )
}
