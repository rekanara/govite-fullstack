import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { tanstackRouter } from "@tanstack/router-plugin/vite"
import { defineConfig } from "vite"

const proxy = { "/api": process.env.API_PROXY_TARGET ?? "http://localhost:3000" }

export default defineConfig({
  // tanstackRouter must precede react(): it generates src/routeTree.gen.ts.
  plugins: [
    tanstackRouter({ target: "react", autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  // Same origin in dev and preview, so the refresh cookie (path /api/v1/auth) just works.
  server: { port: 5173, proxy },
  preview: { port: 4173, proxy },
})
