import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { isAuthenticated } from './api/client'
import { Dashboard } from './components/Dashboard'
import { AppLayout } from './components/layout/AppLayout'
import { Login } from './components/Login'
import { ThemeProvider } from './contexts/ThemeContext'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: true,
    },
  },
})

export function App() {
  const [authenticated, setAuthenticated] = useState(isAuthenticated())

  useEffect(() => {
    // Check authentication status
    const checkAuth = () => {
      setAuthenticated(isAuthenticated())
    }

    // Recheck when window gains focus
    window.addEventListener('focus', checkAuth)
    return () => window.removeEventListener('focus', checkAuth)
  }, [])

  const handleLogin = () => {
    setAuthenticated(true)
  }

  const handleLogout = () => {
    setAuthenticated(false)
  }

  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        {authenticated ? (
          <AppLayout onLogout={handleLogout}>
            <Dashboard />
          </AppLayout>
        ) : (
          <Login onLogin={handleLogin} />
        )}
      </QueryClientProvider>
    </ThemeProvider>
  )
}
