import { AppLayout } from '@/components/layout/AppLayout'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ConfirmProvider } from '@/contexts/ConfirmContext'
import { useAuthStore } from '@/stores/auth'
import { useQueryClient } from '@tanstack/react-query'
import {
  createRootRoute,
  Outlet,
  redirect,
  useNavigate,
  useRouterState,
} from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/router-devtools'

export const Route = createRootRoute({
  beforeLoad: async ({ location }) => {
    // Skip auth check for login page
    if (location.pathname === '/login') {
      return
    }

    // The session cookie is httpOnly, so only the server can confirm it. The
    // store caches the answer, so this costs one request per session rather
    // than one per navigation.
    const authenticated = await useAuthStore.getState().check()
    if (!authenticated) {
      throw redirect({
        to: '/login',
      })
    }
  },
  component: RootComponent,
})

export function RootComponent() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const signOut = useAuthStore((state) => state.signOut)

  const handleLogout = async () => {
    await signOut()
    // Drop cached server state so the next user never sees the previous one's
    // dashboard behind a loading spinner.
    queryClient.clear()
    navigate({ to: '/login' })
  }

  const routerState = useRouterState()
  const isLoginPage = routerState.location.pathname === '/login'

  return (
    // Mounted once, here, so no page can forget to render the confirmation
    // dialog or a tooltip's provider — see `contexts/ConfirmContext.tsx`.
    <TooltipProvider delayDuration={200}>
      <ConfirmProvider>
        {isLoginPage ? (
          <Outlet />
        ) : (
          <AppLayout onLogout={handleLogout}>
            <Outlet />
          </AppLayout>
        )}
        {import.meta.env.DEV && <TanStackRouterDevtools />}
      </ConfirmProvider>
    </TooltipProvider>
  )
}
