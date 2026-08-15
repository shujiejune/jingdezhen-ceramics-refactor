import { createFileRoute } from '@tanstack/react-router'

import { ArtistCard } from '~/components/cards'
import { SectionHeading } from '~/components/common/ui'
import { WaveDivider } from '~/components/cards'
import { api } from '~/lib/api'
import { useI18n } from '~/lib/i18n'

export const Route = createFileRoute('/$locale/artists/')({
  loader: async ({ params }) => {
    const [artists, catalog] = await Promise.all([
      api.getArtists(params.locale),
      api.getProducts({ locale: params.locale, limit: 48 }),
    ])
    return { artists, worksByArtist: catalog.data }
  },
  component: ArtistsPage,
})

function ArtistsPage() {
  const { t } = useI18n()
  const { artists, worksByArtist } = Route.useLoaderData()

  return (
    <div className="mx-auto max-w-shell px-4 pt-10 sm:px-6">
      <SectionHeading eyebrow={t('landing.artistsEyebrow')} title={t('nav.artists')} sub={t('engage.subtitle')} />
      <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {artists.map((a) => (
          <ArtistCard key={a.id} artist={a} works={worksByArtist.filter((p) => p.artist_id === a.id).length} />
        ))}
      </div>
      <WaveDivider className="mt-16" />
    </div>
  )
}
