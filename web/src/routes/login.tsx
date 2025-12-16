import { isAuthenticated } from '@/api/client'
import { Login } from '@/components/Login'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'

export const Route = createFileRoute('/login')({
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()

  useEffect(() => {
    // Redirect to home if already authenticated
    if (isAuthenticated()) {
      navigate({ to: '/' })
    }
  }, [navigate])

  const handleLogin = () => {
    navigate({ to: '/' })
  }

  return <Login onLogin={handleLogin} />
}
