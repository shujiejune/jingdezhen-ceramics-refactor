/**
 * Realtime notification push over the /ws WebSocket hub (TDD §5.3).
 *
 * The backend currently authenticates /ws via the Authorization header, which
 * browser WebSocket cannot set — the documented blocker tracked in
 * frontend/TODO.md ("Backend dependencies"). Until the backend accepts a
 * query-token or subprotocol, live connections fail the upgrade and this layer
 * retries with capped exponential backoff. Consumers must treat "not open" as
 * "keep polling": the Header's 30s poll is the fallback and stays active
 * whenever the socket isn't open.
 *
 * Push payload = JSON-marshaled models.Notification (same shape as the REST
 * GET /notifications items).
 */
import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'

import { API_MODE } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import type { Notification } from '~/lib/types'

export type RealtimeStatus = 'idle' | 'connecting' | 'open' | 'backing-off'

/** Capped exponential backoff: 1s, 2s, 4s … max 30s. Pure — unit-tested. */
export function backoffDelay(attempt: number): number {
  return Math.min(1000 * 2 ** Math.max(0, attempt), 30_000)
}

/** Builds the /ws URL with the JWT as a query param (pending backend support). */
export function wsUrl(origin: string, token: string): string {
  const base = origin.replace(/^http/, 'ws')
  return `${base}/ws?token=${encodeURIComponent(token)}`
}

/**
 * Parses a pushed frame into a Notification; returns null for anything that
 * isn't a notification-shaped object (the hub may add frame types later).
 * Pure — unit-tested.
 */
export function parseNotification(raw: string): Notification | null {
  try {
    const obj = JSON.parse(raw) as Record<string, unknown>
    if (
      typeof obj.notification_id === 'number' &&
      typeof obj.notification_type === 'string' &&
      typeof obj.message === 'string'
    ) {
      return obj as unknown as Notification
    }
    return null
  } catch {
    return null
  }
}

type Listener = (n: Notification) => void
type ListenerSubscribe = (cb: Listener) => () => void

interface RealtimeValue {
  status: RealtimeStatus
  /** Subscribe to pushed notifications (fires only while a socket is live). */
  subscribe: (cb: Listener) => () => void
}

const RealtimeContext = createContext<RealtimeValue | null>(null)

export function RealtimeProvider({ children }: { children: React.ReactNode }) {
  const { token, ready } = useAuth()
  const [status, setStatus] = useState<RealtimeStatus>('idle')
  const listeners = useRef(new Set<Listener>())

  const subscribe = useMemo<ListenerSubscribe>(
    () => (cb) => {
      listeners.current.add(cb)
      return () => listeners.current.delete(cb)
    },
    [],
  )

  useEffect(() => {
    // Mock mode has no WS hub; consumers fall back to the 30s poll.
    if (API_MODE !== 'live') return
    if (!ready || !token || typeof window === 'undefined') return

    let attempt = 0
    let socket: WebSocket | null = null
    let reconnectTimer: number | undefined
    let closed = false

    const connect = () => {
      if (closed) return
      setStatus('connecting')
      socket = new WebSocket(wsUrl(window.location.origin, token))

      socket.onopen = () => {
        attempt = 0
        setStatus('open')
      }
      socket.onmessage = (event) => {
        const n = typeof event.data === 'string' ? parseNotification(event.data) : null
        if (n) for (const l of listeners.current) l(n)
      }
      socket.onclose = () => {
        if (closed) return
        setStatus('backing-off')
        reconnectTimer = window.setTimeout(connect, backoffDelay(attempt++))
      }
      socket.onerror = () => {
        socket?.close()
      }
    }

    connect()
    return () => {
      closed = true
      window.clearTimeout(reconnectTimer)
      socket?.close()
      setStatus('idle')
    }
  }, [ready, token])

  const value = useMemo(() => ({ status, subscribe }), [status, subscribe])
  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>
}

export function useRealtime(): RealtimeValue {
  const ctx = useContext(RealtimeContext)
  if (!ctx) throw new Error('useRealtime must be used inside RealtimeProvider')
  return ctx
}
