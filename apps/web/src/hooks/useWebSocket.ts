import { useCallback, useEffect, useRef, useState } from 'react'
import { getAuthHeader } from '../api/client'
import type { LogEntry } from '../types'

interface UseWebSocketOptions {
  enabled?: boolean
  maxReconnectDelay?: number
  initialReconnectDelay?: number
}

// WebSocket hook with auto-reconnect
export function useWebSocket(jobId: string, options: UseWebSocketOptions = {}) {
  const { enabled = true, maxReconnectDelay = 30000, initialReconnectDelay = 1000 } = options

  const [messages, setMessages] = useState<LogEntry[]>([])
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<number | undefined>(undefined)
  const reconnectDelayRef = useRef(initialReconnectDelay)
  const mountedRef = useRef(true)
  // Holds the latest `connect` so the reconnect timer can call it without
  // referencing the callback it lives inside.
  const connectRef = useRef<(() => void) | null>(null)

  const connect = useCallback(() => {
    if (!enabled || !jobId || !mountedRef.current) return

    try {
      // Build WebSocket URL with auth
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const host = window.location.host
      const auth = getAuthHeader()

      // For WebSocket, we can't set Authorization header directly in browser
      // We'll need to pass auth via query parameter or use a different approach
      const wsUrl = `${protocol}//${host}/api/jobs/${jobId}/logs/stream`

      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        if (!mountedRef.current) {
          ws.close()
          return
        }

        setConnected(true)
        setError(null)
        reconnectDelayRef.current = initialReconnectDelay

        // Send auth as first message if needed
        if (auth) {
          ws.send(JSON.stringify({ type: 'auth', token: auth }))
        }
      }

      ws.onmessage = (event) => {
        if (!mountedRef.current) return

        try {
          const message = JSON.parse(event.data) as LogEntry
          setMessages((prev) => [...prev, message])
        } catch (err) {
          console.error('Failed to parse WebSocket message:', err)
        }
      }

      ws.onerror = (event) => {
        if (!mountedRef.current) return
        setError('WebSocket error occurred')
        console.error('WebSocket error:', event)
      }

      ws.onclose = () => {
        if (!mountedRef.current) return

        setConnected(false)
        wsRef.current = null

        // Exponential backoff reconnection
        if (enabled) {
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectDelayRef.current = Math.min(reconnectDelayRef.current * 2, maxReconnectDelay)
            connectRef.current?.()
          }, reconnectDelayRef.current)
        }
      }
    } catch (err) {
      // `connect` runs synchronously from the mount effect, so this path must not
      // touch React state (see react-hooks/set-state-in-effect). Constructing the
      // socket only throws on a malformed URL, which is a programming error —
      // logging it is enough. Runtime failures arrive via onerror/onclose below,
      // which do update `error`.
      console.error('Failed to create WebSocket:', err)
    }
  }, [enabled, jobId, initialReconnectDelay, maxReconnectDelay])

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setConnected(false)
  }, [])

  const clearMessages = useCallback(() => {
    setMessages([])
  }, [])

  // Connect on mount or when jobId/enabled changes
  useEffect(() => {
    mountedRef.current = true
    connectRef.current = connect
    connect()

    return () => {
      mountedRef.current = false
      connectRef.current = null
      disconnect()
    }
  }, [connect, disconnect])

  return {
    messages,
    connected,
    error,
    disconnect,
    reconnect: connect,
    clearMessages,
  }
}
