import { createRootRoute, Outlet } from "@tanstack/solid-router";
import { TanStackRouterDevtools } from "@tanstack/solid-router-devtools";
import { MetaProvider, Title } from "@solidjs/meta";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import { Suspense } from "solid-js";
import { VerticalNavbar } from "~/components/layout/VerticalNavbar";

export const Route = createRootRoute({
  component: () => {
    const queryClient = new QueryClient();

    return (
      <MetaProvider>
        <Title>Jingdezhen Ceramics Platform</Title>
        <QueryClientProvider client={queryClient}>
          <div class="flex h-screen bg-gray-100 font-sans">
            {/* 1. The persistent vertical navbar */}
            <VerticalNavbar />

            {/* 2. The main content area that will change on navigation */}
            <main class="flex-1 overflow-y-auto">
              <Suspense>
                {/* 3. The placeholder for your page components */}
                <Outlet />
              </Suspense>
            </main>
          </div>

          {/* 4. The helpful devtools (only appears in development) */}
          <TanStackRouterDevtools />
        </QueryClientProvider>
      </MetaProvider>
    );
  },
});
