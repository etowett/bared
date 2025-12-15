import { FormEvent, useState } from 'react'
import { setAuth } from '../api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AlertCircle, HardDrive } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'

interface LoginProps {
  onLogin: () => void
}

export function Login({ onLogin }: LoginProps) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      setAuth(username, password)

      // Test authentication by calling health endpoint
      const response = await fetch('/api/dashboard', {
        headers: {
          Authorization: `Basic ${btoa(`${username}:${password}`)}`,
        },
      })

      if (response.ok) {
        onLogin()
      } else {
        setError('Invalid username or password')
        setUsername('')
        setPassword('')
      }
    } catch {
      setError('Failed to connect to server')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative flex items-center justify-center min-h-screen bg-background">
      {/* Theme Toggle in top-right corner */}
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>

      <div className="bg-card border p-8 rounded-lg shadow-lg w-full max-w-md">
        <div className="flex items-center gap-3 mb-2">
          <HardDrive className="h-8 w-8 text-primary" />
          <h1 className="text-4xl font-bold text-primary">BareD</h1>
        </div>
        <h2 className="text-xl text-muted-foreground mb-8">Backup Dashboard</h2>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
              disabled={loading}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              disabled={loading}
            />
          </div>

          {error && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <Button type="submit" disabled={loading}>
            {loading ? 'Signing in...' : 'Sign In'}
          </Button>
        </form>
      </div>
    </div>
  )
}
