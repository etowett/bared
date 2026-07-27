/* eslint-disable react-refresh/only-export-components */
import { TooltipProvider } from '@/components/ui/tooltip'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  RouterContextProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { render, RenderOptions } from '@testing-library/react'
import React, { ReactElement } from 'react'
import { ConfirmProvider } from '../contexts/ConfirmContext'
import { ThemeProvider } from '../contexts/ThemeContext'

// Create a fresh QueryClient for each test
const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

// A component under test may render a `<Link>`, and `useRouter()` throws
// without a router in scope. `RouterContextProvider` supplies one without
// taking over rendering the way `RouterProvider` does, so the component still
// renders exactly what the test passed it. Route paths are declared (rather
// than reusing the generated tree) to keep this wrapper cheap and free of the
// route modules' own imports; add a path here when a test needs to link to it.
const stubRootRoute = createRootRoute()
const stubRouteTree = stubRootRoute.addChildren(
  ['/', '/config', '/config/import', '/jobs', '/backup/jobs', '/restore/jobs'].map((path) =>
    createRoute({ getParentRoute: () => stubRootRoute, path })
  )
)

const createTestRouter = () =>
  createRouter({
    routeTree: stubRouteTree,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })

interface CustomRenderOptions extends Omit<RenderOptions, 'wrapper'> {
  queryClient?: QueryClient
}

export function renderWithQuery(
  ui: ReactElement,
  { queryClient = createTestQueryClient(), ...renderOptions }: CustomRenderOptions = {}
) {
  // Mirrors the provider stack `routes/__root.tsx` mounts, so a component under
  // test gets the same confirmation dialog and tooltip context it gets in the
  // real app.
  const router = createTestRouter()
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any -- the stub tree is not the registered route tree */}
        <RouterContextProvider router={router as any}>
          <TooltipProvider delayDuration={0}>
            <ConfirmProvider>{children}</ConfirmProvider>
          </TooltipProvider>
        </RouterContextProvider>
      </QueryClientProvider>
    </ThemeProvider>
  )
  return {
    ...render(ui, { wrapper: Wrapper, ...renderOptions }),
    queryClient,
  }
}

export * from '@testing-library/react'
export { renderWithQuery as render }
