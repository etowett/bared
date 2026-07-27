import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import { TanStackRouterVite } from '@tanstack/router-vite-plugin'

export default defineConfig({
  plugins: [react(), TanStackRouterVite()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // A budget, not a cosmetic setting. The largest chunk after splitting is
    // vendor-react (~190 kB); the default 500 kB limit is now so far above
    // anything we emit that it would never catch a regression. 300 kB warns if
    // a chunk starts creeping back toward the monolith this replaced.
    chunkSizeWarningLimit: 300,
    // Routes are code-split via TanStack Router's `.lazy.tsx` convention, so the
    // entry chunk only carries the shell plus the two eager routes (`/` and
    // `/login`). On top of that, peel the big stable vendors into their own
    // chunks: they change far less often than app code, so a release only
    // invalidates the app chunks and browsers keep the vendor ones.
    //
    // Vite 8 bundles with Rolldown, where the object form of
    // `rollupOptions.output.manualChunks` has been removed and the function form
    // is deprecated — `codeSplitting.groups` is the supported replacement.
    // https://rolldown.rs/in-depth/manual-code-splitting
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: 'vendor-react',
              test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/,
              priority: 30,
            },
            {
              name: 'vendor-tanstack',
              test: /node_modules[\\/]@tanstack[\\/]/,
              priority: 20,
            },
            // Nothing else is grouped by hand. Measured on this app, every
            // further group (a `@radix-ui` catch-all, a `lucide-react` icon
            // chunk, a `node_modules` catch-all) made the initial download
            // *larger*, because it pulled route-only dependencies up into
            // chunks the shell already needs. Radix is the clearest case: only a
            // few primitives (slot, label, tooltip) are used by the shell, and
            // a catch-all `@radix-ui` group drags the heavy ones (select,
            // dialog) that only config routes need into the initial download.
            // Rolldown's automatic splitting already gives each primitive a
            // chunk shared by exactly the routes that use it.
          ],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        // The job log stream is a WebSocket; without this the dev server never
        // upgrades it.
        ws: true,
      },
    },
  },
})
