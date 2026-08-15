import { Link, HeadContent, Scripts, useRouterState, createRootRoute } from '@tanstack/react-router'

import appCss from '~/styles/tokens.css?url'

export const Route = createRootRoute({
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
        content:
          'Centuries of porcelain craft from Jingdezhen — discover, collect, and visit.',
      },
    ],
    links: [
      { rel: 'stylesheet', href: appCss },
      { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' },
    ],
  }),
  shellComponent: RootDocument,
  notFoundComponent: RootNotFound,
})

function RootNotFound() {
  return (
    <div style={{ fontFamily: 'system-ui, sans-serif', padding: '8rem 2rem', textAlign: 'center' }}>
      <p style={{ letterSpacing: '0.2em', fontSize: '0.75rem', color: '#4872c4' }}>404</p>
      <h1 style={{ marginTop: '0.75rem', fontSize: '1.6rem' }}>Page not found</h1>
      <Link to={'/en-US' as never} style={{ color: '#3559ae', marginTop: '1.5rem', display: 'inline-block' }}>
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
