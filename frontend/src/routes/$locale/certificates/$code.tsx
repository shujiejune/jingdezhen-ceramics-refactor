import { createFileRoute, notFound, useNavigate } from '@tanstack/react-router'
import { SealCheck, ArrowRight, DownloadSimple, FilePdf } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { PorcelainFigure } from '~/components/artwork/PorcelainFigure'
import { SealMark, WaveBand, PetalScatter } from '~/components/ornaments'
import { Badge, Button } from '~/components/common/ui'
import { JsonLd } from '~/components/seo/JsonLd'
import { api, ApiError } from '~/lib/api'
import { useI18n } from '~/lib/i18n'
import { buildSeoHead } from '~/lib/seo'
import { formatDate, seededRandom } from '~/lib/utils'
import type { Certificate } from '~/lib/types'

/**
 * Public certificate page — the QR target (PRD §3.2.2): certificate ID,
 * artwork + artist, provenance chain. No auth (router.go keeps this public).
 */
export const Route = createFileRoute('/$locale/certificates/$code')({
  loader: async ({ params }) => {
    try {
      return await api.getCertificate(params.code, params.locale)
    } catch (e) {
      if (e instanceof ApiError && e.is('not_found')) throw notFound()
      throw e
    }
  },
  head: ({ loaderData, params }) => {
    if (!loaderData) return { meta: [], links: [] }
    const title = `${loaderData.cert_code} — Certificate`
    const description = `Authenticity certificate ${loaderData.cert_code} for ${loaderData.product_title} by ${loaderData.artist_name}.`
    const { meta, links } = buildSeoHead({
      locale: params.locale,
      path: `/certificates/${params.code}`,
      title,
      description,
      ogType: 'article',
    })
    return {
      meta: [{ title }, { name: 'description', content: description }, ...meta],
      links,
    }
  },
  notFoundComponent: CertNotFound,
  component: CertificatePage,
})

function CertNotFound() {
  const { t, locale } = useI18n()
  const navigate = useNavigate()
  const [code, setCode] = useState('')
  return (
    <div className="mx-auto max-w-md px-4 pt-20 pb-16 text-center sm:px-6">
      <h1 className="text-[1.4rem] font-semibold text-ink-900">{t('cert.notFound')}</h1>
      <p className="mt-3 text-[0.88rem] text-ink-500">{t('cert.verifyHint')}</p>
      <form
        className="mt-6 flex gap-2.5"
        onSubmit={(e) => {
          e.preventDefault()
          if (code.trim())
            void navigate({
              to: '/$locale/certificates/$code',
              params: { locale, code: code.trim() },
            })
        }}
      >
        <input
          className="input-base"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder="JDZ-…"
        />
        <Button type="submit">{t('cert.verify')}</Button>
      </form>
    </div>
  )
}

/** Deterministic QR-style pattern for the prototype (real QR served by the API). */
function QrMotif({ seed, size = 108 }: { seed: number; size?: number }) {
  const cells: React.ReactNode[] = []
  const rand = seededRandom(seed)
  const n = 13
  for (let y = 0; y < n; y++) {
    for (let x = 0; x < n; x++) {
      const inFinder = (x < 4 && y < 4) || (x >= n - 4 && y < 4) || (x < 4 && y >= n - 4)
      if (inFinder) continue
      if (rand() > 0.52) cells.push(<rect key={`${x}-${y}`} x={x} y={y} width="1" height="1" />)
    }
  }
  const finder = (fx: number, fy: number) => (
    <g>
      <rect
        x={fx}
        y={fy}
        width="4"
        height="4"
        fill="none"
        stroke="var(--ink-900)"
        strokeWidth="0.9"
      />
      <rect x={fx + 1.2} y={fy + 1.2} width="1.6" height="1.6" fill="var(--ink-900)" />
    </g>
  )
  return (
    <svg
      width={size}
      height={size}
      viewBox={`-0.5 -0.5 ${n} ${n}`}
      fill="var(--ink-900)"
      aria-hidden="true"
    >
      {cells}
      {finder(0, 0)}
      {finder(n - 4, 0)}
      {finder(0, n - 4)}
    </svg>
  )
}

/**
 * Real QR code rendered client-side. Encodes the public certificate
 * verification URL so a scanner opens this page. Falls back to the
 * decorative `QrMotif` if generation fails (e.g. SSR or library error).
 */
function QrCodeImage({ cert, locale, seed }: { cert: Certificate; locale: string; seed: number }) {
  const { t } = useI18n()
  const [dataUrl, setDataUrl] = useState<string | null>(null)

  useEffect(() => {
    const verifyUrl = `${window.location.origin}/${locale}/certificates/${cert.cert_code}`
    QRCode.toDataURL(verifyUrl, {
      width: 216,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#1a1a2e', light: '#ffffff' },
    })
      .then((url) => setDataUrl(url))
      .catch(() => setDataUrl(null))
  }, [cert.cert_code, locale])

  return (
    <div
      className="flex flex-col items-center"
      role="img"
      aria-label={t('cert.viewQr')}
      title={t('cert.scanQr')}
    >
      {dataUrl ? (
        <img src={dataUrl} alt={t('cert.viewQr')} width={108} height={108} className="block" />
      ) : (
        <QrMotif seed={seed} />
      )}
      <p className="mt-2 text-center font-mono text-[0.72rem] text-ink-400">{cert.cert_code}</p>
    </div>
  )
}

/** PDF download control. Live backend serves a 302 to the storage URL (or
 *  404 if not generated); in mock mode there is no endpoint, so we render
 *  the "not yet generated" disabled state. */
function PdfDownload({ cert }: { cert: Certificate }) {
  const { t } = useI18n()
  const isLive = import.meta.env.VITE_API_MODE === 'live'
  const base =
    (import.meta.env.VITE_API_BROWSER_BASE as string | undefined) ??
    (import.meta.env.PROD ? '/api' : 'http://localhost:1323')
  const pdfUrl = `${base}/certificates/${cert.cert_code}/pdf?download=1`

  // Mock mode has no real PDF endpoint, and a missing pdf_key indicates the
  // PDF has not been generated yet — render the "not ready" state either way.
  if (!isLive || !cert.pdf_key) {
    return (
      <div className="flex items-center gap-2.5 text-[0.84rem] text-ink-400">
        <FilePdf size={15} weight="regular" />
        <span title={t('cert.pdfNotReady')}>{t('cert.pdfNotReady')}</span>
      </div>
    )
  }

  return (
    <a
      href={pdfUrl}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-2 rounded-lg border border-cobalt-200 bg-white px-3.5 py-2 text-[0.84rem] font-medium text-cobalt-700 shadow-card transition hover:border-cobalt-300 hover:bg-cobalt-50"
    >
      <DownloadSimple size={15} weight="regular" />
      {t('cert.downloadPdf')}
    </a>
  )
}

function CertificatePage() {
  const { t, locale } = useI18n()
  const cert = Route.useLoaderData() as Certificate

  const kindLabel = (k: string) => t(`cert.kind.${k}` as 'cert.kind.created')

  return (
    <div className="relative mx-auto max-w-3xl px-4 pt-12 pb-16 sm:px-6">
      <JsonLd
        data={{
          '@context': 'https://schema.org',
          '@type': 'Certificate',
          name: cert.cert_code,
          description: `Authenticity certificate for ${cert.product_title}`,
          issuanceDate: cert.issued_at,
          certifiedBy: { '@type': 'Organization', name: 'Jingdezhen Ceramics Platform' },
          about: {
            '@type': 'Product',
            name: cert.product_title,
            brand: { '@type': 'Brand', name: cert.artist_name },
          },
        }}
      />
      <PetalScatter
        seed={cert.figure_seed}
        className="pointer-events-none absolute top-8 right-6 opacity-40"
      />

      {/* certificate card */}
      <div className="card-surface relative overflow-hidden p-8 sm:p-10">
        <div className="qinghua-watermark absolute inset-x-0 top-0 h-40 opacity-50" />
        <div className="relative flex flex-wrap items-start justify-between gap-6">
          <div className="flex items-center gap-4">
            <SealMark size={52} />
            <div>
              <Badge tone="success">
                <SealCheck size={12} weight="fill" />
                {t('cert.verified')}
              </Badge>
              <h1 className="mt-2 text-[1.35rem] font-semibold tracking-tight text-ink-900">
                {t('cert.title')}
              </h1>
            </div>
          </div>
          <div className="rounded-xl border border-cobalt-100 bg-white p-3 shadow-card">
            <QrCodeImage cert={cert} locale={locale} seed={cert.figure_seed} />
          </div>
        </div>

        <WaveBand width={200} className="relative mx-auto my-8" />

        {/* artwork */}
        <div className="relative grid gap-8 sm:grid-cols-[10rem_1fr]">
          <div className="mx-auto w-40 overflow-hidden rounded-xl border border-cobalt-100 bg-gradient-to-b from-wash to-porcelain">
            <PorcelainFigure
              kind={cert.figure_kind}
              seed={cert.figure_seed}
              className="h-auto w-full"
            />
          </div>
          <div>
            <p className="eyebrow">{t('catalog.title')}</p>
            <h2 className="mt-1.5 text-[1.25rem] font-semibold text-ink-900">
              {cert.product_title}
            </h2>
            <p className="mt-1.5 text-[0.88rem] text-ink-500">
              {t('cert.artist')}:{' '}
              <span className="font-medium text-ink-700">{cert.artist_name}</span>
            </p>
            <dl className="mt-5 grid gap-x-8 gap-y-2 text-[0.85rem] sm:grid-cols-2">
              <div className="flex justify-between border-b border-cobalt-50 pb-1.5">
                <dt className="text-ink-400">{t('cert.code')}</dt>
                <dd className="font-mono font-medium text-ink-800">{cert.cert_code}</dd>
              </div>
              <div className="flex justify-between border-b border-cobalt-50 pb-1.5">
                <dt className="text-ink-400">{t('cert.issued')}</dt>
                <dd className="font-medium text-ink-800">{formatDate(cert.issued_at, locale)}</dd>
              </div>
              {cert.attributes?.technique && (
                <div className="flex justify-between border-b border-cobalt-50 pb-1.5">
                  <dt className="text-ink-400">{t('product.attr.technique')}</dt>
                  <dd className="font-medium text-ink-800">{cert.attributes.technique}</dd>
                </div>
              )}
              {cert.attributes?.year && (
                <div className="flex justify-between border-b border-cobalt-50 pb-1.5">
                  <dt className="text-ink-400">{t('product.attr.year')}</dt>
                  <dd className="font-medium text-ink-800">{cert.attributes.year}</dd>
                </div>
              )}
            </dl>
            <div className="mt-5 flex flex-wrap items-center gap-4">
              <a
                href={`/${locale}/catalog/${cert.product_slug}`}
                className="inline-flex items-center gap-1.5 text-[0.85rem] font-medium text-cobalt-600 hover:underline"
              >
                {t('cert.viewWork')}
                <ArrowRight size={13} weight="bold" />
              </a>
              <span className="h-4 w-px bg-cobalt-100" aria-hidden="true" />
              <PdfDownload cert={cert} />
            </div>
          </div>
        </div>
      </div>

      {/* provenance */}
      <section className="mt-10">
        <h2 className="text-[0.82rem] font-semibold tracking-[0.18em] text-ink-400 uppercase">
          {t('cert.provenance')}
        </h2>
        <ol className="relative mt-5 flex flex-col gap-6 border-l-2 border-cobalt-100 pl-6">
          {cert.provenance.map((p) => (
            <li key={p.id} className="relative">
              <span className="absolute top-1 -left-[31px] h-3.5 w-3.5 rounded-full border-[3px] border-cobalt-500 bg-white" />
              <p className="text-[0.9rem] font-semibold text-ink-800">{kindLabel(p.kind)}</p>
              <p className="mt-0.5 text-[0.84rem] text-ink-500">{p.detail}</p>
              <p className="mt-0.5 text-[0.75rem] text-ink-300">{formatDate(p.at, locale)}</p>
            </li>
          ))}
        </ol>
      </section>

      {/* verify another */}
      <div className="card-surface mt-10 flex flex-wrap items-center justify-between gap-4 p-6">
        <p className="text-[0.86rem] text-ink-500">{t('cert.verifyHint')}</p>
        <a
          href="#"
          onClick={(e) => e.preventDefault()}
          className="text-[0.84rem] font-medium text-cobalt-600 hover:underline"
        >
          {t('cert.checkOther')}
        </a>
      </div>
    </div>
  )
}
