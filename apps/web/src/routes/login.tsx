import { Login } from '@/components/Login'
import { useAuthStore } from '@/stores/auth'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'

export const Route = createFileRoute('/login')({
  component: LoginPage,
})

export function LoginPage() {
  const navigate = useNavigate()

  useEffect(() => {
    let cancelled = false

    // Whether a session is still live is a server question now, so this is
    // async where it used to be a storage read.
    void useAuthStore
      .getState()
      .check()
      .then((authenticated) => {
        if (authenticated && !cancelled) {
          navigate({ to: '/' })
        }
      })

    return () => {
      cancelled = true
    }
  }, [navigate])

  const handleLogin = () => {
    navigate({ to: '/' })
  }

  return <Login onLogin={handleLogin} />
}
