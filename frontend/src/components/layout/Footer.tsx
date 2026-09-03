/** Site footer — sitemap links, policies, contact, wave ornament. */
import { Link } from '@tanstack/react-router'
import { Envelope, Phone } from '@phosphor-icons/react'

import { PetalScatter, SealMark, WaveBand } from '~/components/ornaments'
import { CONTACT } from '~/mocks/data'
import { useConsent } from '~/lib/consent'
import { useI18n } from '~/lib/i18n'

export function Footer() {
  const { t, locale } = useI18n()
  const { reopen } = useConsent()
  const base = `/${locale}`

  const cols = [
    {
      title: t('footer.explore'),
      links: [
        { label: t('nav.gallery'), to: `${base}/catalog` },
        { label: t('nav.artists'), to: `${base}/artists` },
        { label: t('nav.heritage'), to: `${base}/ceramicstory` },
        { label: t('nav.visit'), to: `${base}/engage` },
        { label: t('nav.travel'), to: `${base}/itinerary` },
      ],
    },
    {
      title: t('footer.support'),
      links: [
        { label: t('footer.contact'), to: `${base}/engage` },
        { label: t('footer.faq'), to: `${base}/ceramicstory` },
        { label: t('footer.shipping'), to: `${base}/catalog` },
        { label: t('footer.refunds'), to: `${base}/orders` },
      ],
    },
    {
      title: t('footer.legal'),
      links: [
        { label: t('footer.privacy'), to: `${base}/account` },
        { label: t('footer.terms'), to: `${base}/account` },
      ],
    },
  ]

  return (
    <footer className="relative mt-24 border-t border-cobalt-100/70 bg-wash">
      <div className="pointer-events-none absolute top-6 right-8 opacity-60">
        <PetalScatter seed={42} count={7} width={180} height={70} />
      </div>
      <div className="relative mx-auto max-w-shell px-4 pt-16 pb-10 sm:px-6">
        <div className="grid gap-12 md:grid-cols-[1.4fr_repeat(3,1fr)]">
          <div>
            <div className="flex items-center gap-2.5">
              <SealMark size={34} />
              <span className="flex flex-col leading-none">
                <span className="text-[0.95rem] font-semibold text-ink-900">
                  {t('common.brand')}
                </span>
                <span className="mt-1 text-[0.62rem] font-medium tracking-[0.16em] text-cobalt-600 uppercase">
                  {t('common.brandSub')}
                </span>
              </span>
            </div>
            <p className="mt-4 max-w-xs text-sm leading-relaxed text-ink-500">
              {t('footer.tagline')}
            </p>
            <div className="mt-5 flex flex-col gap-1.5 text-sm text-ink-500">
              <a
                href={`mailto:${CONTACT.email}`}
                className="flex items-center gap-2 hover:text-cobalt-600"
              >
                <Envelope size={15} className="text-cobalt-400" />
                {CONTACT.email}
              </a>
              <a
                href={`tel:${CONTACT.phone.replace(/\s/g, '')}`}
                className="flex items-center gap-2 hover:text-cobalt-600"
              >
                <Phone size={15} className="text-cobalt-400" />
                {CONTACT.phone}
              </a>
            </div>
          </div>
          {cols.map((col) => (
            <nav key={col.title} aria-label={col.title}>
              <h3 className="text-[0.72rem] font-semibold tracking-[0.18em] text-ink-400 uppercase">
                {col.title}
              </h3>
              <ul className="mt-4 flex flex-col gap-2.5">
                {col.links.map((l) => (
                  <li key={l.label}>
                    <Link
                      to={l.to as never}
                      className="text-sm text-ink-600 transition hover:text-cobalt-700"
                    >
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </nav>
          ))}
          <nav aria-label={t('footer.legal')}>
            <h3 className="text-[0.72rem] font-semibold tracking-[0.18em] text-ink-400 uppercase">
              {t('footer.legal')}
            </h3>
            <ul className="mt-4 flex flex-col gap-2.5">
              <li>
                <button
                  type="button"
                  onClick={reopen}
                  className="text-left text-sm text-ink-600 transition hover:text-cobalt-700"
                >
                  {t('footer.cookies')}
                </button>
              </li>
            </ul>
          </nav>
        </div>

        <div className="mt-14 flex flex-col items-center gap-4">
          <WaveBand width={220} />
          <p className="text-[0.78rem] text-ink-400">{t('common.prototypeNote')}</p>
          <div className="flex items-center gap-3 text-[0.78rem] text-ink-400">
            <span>{t('footer.rights')}</span>
            <span aria-hidden="true">·</span>
            <Link to={`/${locale}/sitemap` as never} className="transition hover:text-cobalt-700">
              {t('footer.sitemap')}
            </Link>
          </div>
        </div>
      </div>
    </footer>
  )
}
