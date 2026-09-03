import { Link } from '@tanstack/react-router'

import { PetalScatter, SealMark, WaveBand } from '~/components/ornaments'
import { useI18n } from '~/lib/i18n'
import { buildSeoHead } from '~/lib/seo'

/**
 * Shared layout for legal/policy placeholder pages (PRD §4.4).
 * The content is i18n-catalog text for now — the legal-reviewed version
 * ships in the may-trail (MLS).
 */
export function PolicyPage({
  titleKey,
  bodyKey,
}: {
  titleKey: 'privacyPolicy.title' | 'termsPolicy.title' | 'cookiesPolicy.title'
  bodyKey: 'privacyPolicy.body' | 'termsPolicy.body' | 'cookiesPolicy.body'
}) {
  const { t, locale } = useI18n()
  const title = t(titleKey)
  const body = t(bodyKey)

  return (
    <div className="relative mx-auto max-w-2xl px-4 pt-16 pb-16 sm:px-6">
      <PetalScatter
        seed={17}
        count={5}
        className="pointer-events-none absolute top-8 right-6 opacity-40"
      />

      <div className="flex items-center gap-3">
        <SealMark size={28} />
        <h1 className="text-display-sm text-ink-900">{title}</h1>
      </div>

      <div className="mt-8 text-[0.92rem] leading-relaxed text-ink-600">
        {body.split('\n').map((para, i) => (
          <p key={i} className={i > 0 ? 'mt-4' : ''}>
            {para}
          </p>
        ))}
      </div>

      <div className="mt-12 flex flex-col items-center gap-4">
        <WaveBand width={180} />
        <Link
          to={`/${locale}` as never}
          className="text-[0.84rem] font-medium text-cobalt-600 transition hover:text-cobalt-700"
        >
          {t('landing.ctaGallery')}
        </Link>
      </div>
    </div>
  )
}

/** Build the head() return for a policy page. */
export function policyHead(
  titleKey: 'privacyPolicy.title' | 'termsPolicy.title' | 'cookiesPolicy.title',
  _bodyKey: 'privacyPolicy.body' | 'termsPolicy.body' | 'cookiesPolicy.body',
  path: string,
) {
  return ({ params }: { params: { locale: string } }) => {
    // i18n isn't available in head() — use static English fallbacks;
    // the real content is rendered by the component.
    const titles: Record<string, string> = {
      'privacyPolicy.title': 'Privacy policy',
      'termsPolicy.title': 'Terms of service',
      'cookiesPolicy.title': 'Cookie policy',
    }
    const title = titles[titleKey]
    const description = 'Legal policy page for the Jingdezhen Ceramics Platform.'
    const { meta, links } = buildSeoHead({
      locale: params.locale,
      path,
      title,
      description,
      ogType: 'website',
    })
    return {
      meta: [{ title }, { name: 'description', content: description }, ...meta],
      links,
    }
  }
}
