import { QueryClient, dehydrate, hydrate } from '@tanstack/react-query'
import { createRouter as createTanStackRouter } from '@tanstack/react-router'
import { routeTree } from './routeTree.gen'

export interface RouterContext {
  queryClient: QueryClient
}

export function getRouter() {
  // a fresh client per server request / browser session — loaders fill it
  // via ensureQueryData and the router-level dehydrate/hydrate pair ships
  // the cache to the browser (TanStack Start + Query, TDD §6)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { staleTime: 60_000, refetchOnWindowFocus: false, retry: 1 },
    },
  })

  const router = createTanStackRouter({
    routeTree,
    context: { queryClient },
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    defaultNotFoundComponent: () => null, // rendered inside the locale layout
  })

  // Ship the Query cache with the router state (SSR → browser). Attached
  // post-construction: the options' serializer registry typing is stricter
  // than runtime (DehydratedState is plain JSON), and inline casts poison
  // the router generics that loader typing flows from.
  router.options.dehydrate = () => ({
    dehydratedState: dehydrate(queryClient),
  })
  router.options.hydrate = (dehydrated) => {
    hydrate(queryClient, dehydrated.dehydratedState)
  }

  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
