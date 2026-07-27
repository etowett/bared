import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { onAuthFailure } from './api/client'
import { ThemeProvider } from './contexts/ThemeContext'
import { useAuthStore } from './stores/auth'

// Import the generated route tree
import { routeTree } from './routeTree.gen'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: true,
    },
  },
})

// Most routes are code-split (`*.lazy.tsx`), so navigating to one for the first
// time fetches its chunk. The router keeps the previous screen mounted while
// that happens; this only appears if the fetch is slow enough to be noticeable,
// and `defaultPendingMinMs` keeps it from flickering once shown.
function RoutePending() {
  return <div className="text-center py-12 text-muted-foreground">Loading...</div>
}

// Create a new router instance
const router = createRouter({
  routeTree,
  defaultPendingComponent: RoutePending,
  defaultPendingMs: 200,
  defaultPendingMinMs: 300,
})

// Route 401s to the login page through the router rather than reloading the
// document. The API client raises the signal here, where the router is in
// scope, instead of importing it and creating a cycle.
onAuthFailure(() => {
  useAuthStore.getState().markAnonymous()
  queryClient.clear()
  if (router.state.location.pathname !== '/login') {
    void router.navigate({ to: '/login' })
  }
})

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

export function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
        <Toaster position="top-right" richColors />
      </QueryClientProvider>
    </ThemeProvider>
  )
}
