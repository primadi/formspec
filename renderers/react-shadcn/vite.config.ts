import { defineConfig, type Plugin } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import path from "path"

/**
 * Dev-only plugin: exposes `/_dev/hmr-reload` endpoint on the Vite dev server.
 * When the FormSpec backend detects a YAML spec change, it hits this endpoint,
 * and Vite broadcasts a custom 'formspec:spec-reloaded' event to all connected
 * browsers via the existing HMR WebSocket — no polling needed.
 */
function formaHMRPlugin(): Plugin {
  return {
    name: "formspec-hmr-reload",
    configureServer(server) {
      server.middlewares.use("/_dev/hmr-reload", (_req, res) => {
        server.ws.send({ type: "custom", event: "formspec:spec-reloaded" })
        res.end("ok")
      })
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [formaHMRPlugin(), react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    host: true,
    proxy: {
      // Proxy API calls to the FormSpec backend
      "/default/api/v1": {
        target: "http://localhost:8080",
        changeOrigin: true,
        ws: true, // forward WebSocket upgrade (realtime /_ui/_ws) in dev
      },
      // Proxy _ui/* (meta API, entity CRUD) — matches what Go's viteSPAProxy intercepts.
      // ws: true is REQUIRED for realtime: the browser connects to /_ui/_ws here,
      // and without it the WebSocket upgrade hangs and realtime never receives events.
      "/default/_ui/": {
        target: "http://localhost:8080",
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          // Only split node_modules dependencies.
          if (!id.includes("node_modules")) return
          // Core React runtime — path-bounded match. A bare `id.includes('react')`
          // would also swallow every lib with "react" in its name (@radix-ui/
          // react-*, @base-ui/react, @tanstack/react-table, react-hook-form,
          // react-day-picker, …) collapsing them into one >1 MB mega-chunk.
          if (
            /node_modules[\\/](react|react-dom|react-router|react-router-dom|scheduler)[\\/]/.test(
              id,
            )
          ) {
            return "vendor-react"
          }
          // Icon library into its own chunk.
          if (id.includes("lucide-react")) {
            return "vendor-icons"
          }
          // UI primitives (Radix + Base UI) — largest group after React itself.
          if (id.includes("@radix-ui") || id.includes("@base-ui")) {
            return "vendor-ui"
          }
          // Form stack (react-hook-form + zod + resolvers).
          if (
            id.includes("react-hook-form") ||
            id.includes("@hookform") ||
            /node_modules[\\/]zod[\\/]/.test(id)
          ) {
            return "vendor-forms"
          }
          // Other vendor libs.
          return "vendor"
        },
      },
    },
    // Warn when a chunk exceeds 1 MB (relaxed from default 500 kB for this SPA)
    chunkSizeWarningLimit: 1000,
  },
})
