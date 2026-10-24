import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { tanstackRouter } from "@tanstack/router-plugin/vite"
import { defineConfig } from "vite"

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
  server: {
    port: 5173,
    // Same origin in dev, so the refresh cookie (path /api/v1/auth) just works.
    proxy: { "/api": process.env.API_PROXY_TARGET ?? "http://localhost:3000" },
  },
})
