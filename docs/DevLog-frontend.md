## 1. SolidStart vs. TanStack Start with Solid?

 ┌─────────────────────┬────────────────────────────┬───────────────────────────────┐
 │                     │ SolidStart                 │ TanStack Start (Solid)        │
 ├─────────────────────┼────────────────────────────┼───────────────────────────────┤
 │ Maturity            │ 1.0+, official Solid       │ Newer; the Solid adapter lags │
 │                     │ meta-framework,            │ behind the React one and has  │
 │                     │ production-proven          │ been alpha/beta quality       │
 ├─────────────────────┼────────────────────────────┼───────────────────────────────┤
 │ Router              │ Solid Router (solid,       │ TanStack Router — fully       │
 │                     │ simpler)                   │ type-safe routes, params,     │
 │                     │                            │ search params, loaders        │
 │                     │                            │ (best-in-class)               │
 ├─────────────────────┼────────────────────────────┼───────────────────────────────┤
 │ SSR/SSG/streaming   │ ✅ Yes, incl. islands      │ ✅ Yes (Nitro-based)          │
 │                     │ experiments                │                               │
 ├─────────────────────┼────────────────────────────┼───────────────────────────────┤
 │ Data layer          │ createResource / query     │ First-class TanStack Query    │
 │                     │ primitives, or add         │ integration                   │
 │                     │ TanStack Solid Query       │                               │
 ├─────────────────────┼────────────────────────────┼───────────────────────────────┤
 │ Server functions    │ ✅ "use server"            │ ✅ createServerFn             │
 ├─────────────────────┼────────────────────────────┼───────────────────────────────┤
 │ Ecosystem/docs for  │ Larger, most Solid SSR     │ Most docs/examples are        │
 │ Solid               │ examples target SolidStart │ React-centric; Solid examples │
 │                     │                            │ sparse                        │
 ├─────────────────────┼────────────────────────────┼───────────────────────────────┤
 │ Community help      │ solidjs Discord, many      │ Fewer people running Solid    │
 │                     │ SolidStart apps in the     │ Start in production           │
 │                     │ wild                       │                               │
 └─────────────────────┴────────────────────────────┴───────────────────────────────┘

 SolidStart, for these reasons:

 1. Your app's risk profile. This is a client-facing, SEO-critical, payments-handling
    platform with a deadline — not a playground. SolidStart is the boring, proven
    choice in the Solid ecosystem; TanStack Start's Solid adapter is the least
    battle-tested combination you could pick (React Start is fine, but that's not
    what you'd be using).
 2. You can still use the TanStack libraries you actually want.
    @tanstack/solid-query, @tanstack/solid-table (great for the CMS admin tables),
    and @tanstack/solid-form all work perfectly inside SolidStart. You lose only
    TanStack Router, and Solid Router is adequate.
 3. SEO needs are core (§4.4 of the PRD): SSR, per-locale slugs, meta tags, sitemap.
    SolidStart handles all of this with more community examples to lean on.
 4. Backend is Go/Fiber anyway. A big selling point of TanStack Start is its server
    functions/RPC layer — but your API lives in Fiber, so the SSR layer is mostly a
    rendering + data-fetching shell. That diminishes TanStack Start's advantages
    further.
