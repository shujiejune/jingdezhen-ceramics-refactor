import { createFileRoute } from '@tanstack/react-router'

import { StoryCard } from '~/components/cards'
import { WaveBand } from '~/components/ornaments'
import { SectionHeading } from '~/components/common/ui'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'

/** Heritage index — dynasty timeline (SSR). */
export const Route = createFileRoute('/$locale/ceramicstory/')({
  loader: async ({ params }) => api.getStories(params.locale),
  head: () => ({}),
  component: HeritagePage,
})

function HeritagePage() {
  const { t } = useI18n()
  const stories = Route.useLoaderData()

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      <SectionHeading eyebrow={t('story.timeline')} title={t('story.title')} sub={t('story.subtitle')} />

      <div className="relative">
        {/* timeline spine */}
        <div className="absolute top-2 bottom-2 left-[7px] w-px bg-gradient-to-b from-cobalt-200 via-cobalt-300 to-transparent sm:left-[calc(11rem+11px)]" />
        <ol className="flex flex-col gap-5">
          {stories.map((s) => (
            <li key={s.id} className="relative flex flex-col gap-1 sm:flex-row sm:gap-8">
              <div className="mb-2 flex items-center gap-3.5 sm:mb-0 sm:w-44 sm:justify-end">
                <span className="order-2 h-[15px] w-[15px] shrink-0 rounded-full border-[3px] border-cobalt-500 bg-white sm:order-1" />
                <span className="order-1 text-[0.95rem] font-semibold tracking-tight text-cobalt-700 tabular-nums sm:order-2">
                  {s.dynasty_start_year}
                </span>
              </div>
              <div className="sm:flex-1">
                <StoryCard story={s} />
              </div>
            </li>
          ))}
        </ol>
      </div>

      <div className="mt-16 flex justify-center">
        <WaveBand width={200} />
      </div>
    </div>
  )
}
