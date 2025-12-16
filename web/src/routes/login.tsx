import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { Login } from '@/components/Login'
import { isAuthenticated } from '@/api/client'
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

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <Login onLogin={handleLogin} />
    </div>
  )
}
