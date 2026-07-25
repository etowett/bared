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

// Create a new router instance
const router = createRouter({ routeTree })

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
