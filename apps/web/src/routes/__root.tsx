import { isAuthenticated, logout } from '@/api/client'
import { AppLayout } from '@/components/layout/AppLayout'
import { createRootRoute, Outlet, redirect, useRouterState } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/router-devtools'

export const Route = createRootRoute({
  beforeLoad: async ({ location }) => {
    // Skip auth check for login page
    if (location.pathname === '/login') {
      return
    }

    if (!isAuthenticated()) {
      throw redirect({
        to: '/login',
      })
    }
  },
  component: RootComponent,
})

function RootComponent() {
  const handleLogout = () => {
    logout()
    window.location.href = '/login'
  }

  const routerState = useRouterState()
  const isLoginPage = routerState.location.pathname === '/login'

  return (
    <>
      {isLoginPage ? (
        <Outlet />
      ) : (
        <AppLayout onLogout={handleLogout}>
          <Outlet />
        </AppLayout>
      )}
      {import.meta.env.DEV && <TanStackRouterDevtools />}
    </>
  )
}
