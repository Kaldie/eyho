import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Where the dev server forwards /api. Outside Docker the backend is on localhost:8080; in
// docker compose the frontend container reaches it as http://backend:8080 (VITE_PROXY_TARGET is
// set there). The browser only ever talks to the dev server, so VITE_API_BASE stays empty and
// there is no cross-origin request.
const apiProxyTarget = process.env.VITE_PROXY_TARGET || 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': new URL('./src', import.meta.url).pathname,
    },
  },
  server: {
    // The e2e container opens http://frontend:5173 (compose service name); Vite 5.4.12+ rejects
    // unknown Host headers by default.
    allowedHosts: ['frontend'],
    proxy: {
      '/api': {
        target: apiProxyTarget,
        changeOrigin: true,
      },
    },
  },
})
