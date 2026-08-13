import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Relative base so the same build works on any Cloudflare Pages project/path.
  base: "/",
  build: {
    outDir: "dist",
    sourcemap: false,
  },
})
