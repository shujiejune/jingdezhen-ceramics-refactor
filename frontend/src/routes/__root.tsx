import {
  HeadContent,
  Link,
  Scripts,
  createRootRouteWithContext,
  useRouterState,
  type ErrorComponentProps,
} from '@tanstack/react-router'

import appCss from '~/styles/tokens.css?url'
import type { RouterContext } from '~/router'
import { SITE_NAME } from '~/lib/seo'

export const Route = createRootRouteWithContext<RouterContext>()({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      {
        name: 'viewport',
        content: 'width=device-width, initial-scale=1',
      },
      { title: 'Jingdezhen Ceramics Platform' },
      {
        name: 'description',
        content: 'Centuries of porcelain craft from Jingdezhen — discover, collect, and visit.',
      },
      { property: 'og:site_name', content: SITE_NAME },
      { name: 'twitter:card', content: 'summary' },
    ],
    links: [
      { rel: 'stylesheet', href: appCss },
      { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' },
    ],
  }),
  shellComponent: RootDocument,
  notFoundComponent: RootNotFound,
  errorComponent: RootError,
  pendingComponent: RootPending,
})

function RootError({ error, reset }: ErrorComponentProps) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-6 text-center">
      <p className="eyebrow">500</p>
      <h1 className="text-display-sm text-ink-900">Something went wrong</h1>
      <p className="max-w-md text-[0.92rem] leading-relaxed text-ink-500">
        An unexpected error interrupted the page. The issue has been noted — try again.
      </p>
      {import.meta.env.DEV && (
        <pre className="max-w-xl overflow-auto rounded-md border border-cobalt-100 bg-mist px-4 py-3 text-left text-[0.75rem] text-ink-500">
          {error instanceof Error ? error.message : String(error)}
        </pre>
      )}
      <button
        type="button"
        onClick={reset}
        className="mt-2 inline-flex h-11 items-center rounded-lg bg-cobalt-600 px-6 text-[0.92rem] font-medium text-white shadow-card hover:bg-cobalt-700"
      >
        Try again
      </button>
    </div>
  )
}

function RootPending() {
  return (
    <div className="flex min-h-screen items-center justify-center">
      <svg
        className="h-7 w-7 animate-spin text-cobalt-400"
        viewBox="0 0 24 24"
        fill="none"
        aria-label="Loading"
      >
        <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity="0.25" strokeWidth="3" />
        <path
          d="M21 12a9 9 0 0 0-9-9"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
    </div>
  )
}

function RootNotFound() {
  return (
    <div style={{ fontFamily: 'system-ui, sans-serif', padding: '8rem 2rem', textAlign: 'center' }}>
      <p style={{ letterSpacing: '0.2em', fontSize: '0.75rem', color: '#4872c4' }}>404</p>
      <h1 style={{ marginTop: '0.75rem', fontSize: '1.6rem' }}>Page not found</h1>
      <Link
        to={'/en-US' as never}
        style={{ color: '#3559ae', marginTop: '1.5rem', display: 'inline-block' }}
      >
        Continue to the site →
      </Link>
    </div>
  )
}

function RootDocument({ children }: { children: React.ReactNode }) {
  // derive <html lang> from the locale path segment (SSR + client)
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const firstSeg = pathname.split('/')[1]
  const lang = firstSeg === 'zh-CN' ? 'zh-CN' : firstSeg === 'en-US' ? 'en-US' : 'en'

  return (
    <html lang={lang}>
      <head>
        <HeadContent />
      </head>
      <body>
        {children}
        <Scripts />
      </body>
    </html>
  )
}
