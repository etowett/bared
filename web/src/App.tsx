import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState, useEffect } from 'react'
import { isAuthenticated } from './api/client'
import { Dashboard } from './components/Dashboard'
import { Login } from './components/Login'

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
    <QueryClientProvider client={queryClient}>
      <div className="app">
        {authenticated ? (
          <Dashboard onLogout={handleLogout} />
        ) : (
          <Login onLogin={handleLogin} />
        )}
      </div>
    </QueryClientProvider>
  )
}
