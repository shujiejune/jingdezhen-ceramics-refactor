import { defineConfig } from 'vite'

import { tanstackStart } from '@tanstack/react-start/plugin/vite'

import viteReact from '@vitejs/plugin-react'
import { nitro } from 'nitro/vite'

const API_TARGET = process.env.VITE_API_BASE_URL ?? 'http://localhost:1323'

const config = defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [nitro({ rollupConfig: { external: [] } }), tanstackStart(), viteReact()],
  server: {
    proxy: {
      // browser same-origin calls hit /api/* → Fiber (prefix stripped;
      // mirrors the production reverse proxy). SSR loaders call the API
      // directly via VITE_API_BASE_URL (see lib/api.ts liveBase).
      '/api': {
        target: API_TARGET,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      // local-dev storage mode serves media from the API's /media mount
      '/media': { target: API_TARGET, changeOrigin: true },
      // WebSocket push (/ws — notification hub; needs query-token auth on the
      // backend before live mode can connect, see frontend/TODO.md)
      '/ws': { target: API_TARGET.replace(/^http/, 'ws'), ws: true },
    },
  },
})

export default config
