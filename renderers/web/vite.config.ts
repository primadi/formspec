import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

/**
 * Dev-only plugin: exposes `/_dev/hmr-reload` endpoint on the Vite dev server.
 * When the Forma backend detects a YAML spec change, it hits this endpoint,
 * and Vite broadcasts a custom 'forma:spec-reloaded' event to all connected
 * browsers via the existing HMR WebSocket — no polling needed.
 */
function formaHMRPlugin(): Plugin {
  return {
    name: 'forma-hmr-reload',
    configureServer(server) {
      server.middlewares.use('/_dev/hmr-reload', (_req, res) => {
        server.ws.send({ type: 'custom', event: 'forma:spec-reloaded' })
        res.end('ok')
      })
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [formaHMRPlugin(), react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    host: true,
    proxy: {
      // Proxy API calls to the Forma backend
      '/default/api/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true, // forward WebSocket upgrade (realtime /_ui/_ws) in dev
      },
      // Proxy _ui/* (meta API, entity CRUD) — matches what Go's viteSPAProxy intercepts.
      // ws: true is REQUIRED for realtime: the browser connects to /_ui/_ws here,
      // and without it the WebSocket upgrade hangs and realtime never receives events.
      '/default/_ui/': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          // Vendor chunk: all node_modules dependencies
          if (id.includes('node_modules')) {
            // Split react vendor into its own chunk (react, react-dom, react-router-dom)
            if (id.includes('react') || id.includes('react-dom') || id.includes('react-router')) {
              return 'vendor-react'
            }
            // Split icon library into its own chunk
            if (id.includes('lucide-react')) {
              return 'vendor-icons'
            }
            // Other vendor libs
            return 'vendor'
          }
        },
      },
    },
    // Warn when a chunk exceeds 1 MB (relaxed from default 500 kB for this SPA)
    chunkSizeWarningLimit: 1000,
  },
})
