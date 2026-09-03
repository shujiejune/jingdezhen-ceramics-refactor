import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { z } from 'zod'

import { api } from '~/lib/api'
import { persistSession } from '~/lib/auth'

/**
 * Google OAuth receiver — the backend 307s here with ?token=<jwt>.
 * Outside the $locale tree by contract (CLIENT_ORIGIN + /login/…), so
 * it writes the session directly and bounces into the default locale.
 */
const searchSchema = z.object({ token: z.string().optional() })

export const Route = createFileRoute('/login/success')({
  validateSearch: searchSchema,
  component: OAuthSuccess,
})

export function OAuthSuccess() {
  const search = useSearch({ from: '/login/success' })
  const navigate = useNavigate()
  const ran = useRef(false)

  useEffect(() => {
    if (ran.current) return
    ran.current = true
    const token = search.token
    if (!token) {
      void navigate({
        to: '/$locale/auth/login',
        params: { locale: 'en-US' },
        search: { oauthError: 1 },
      })
      return
    }
    void api
      .getProfile(token)
      .then((user) => {
        persistSession(token, user)
        void navigate({ to: '/$locale', params: { locale: 'en-US' } })
      })
      .catch(() => {
        void navigate({
          to: '/$locale/auth/login',
          params: { locale: 'en-US' },
          search: { oauthError: 1 },
        })
      })
  }, [search.token, navigate])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <svg className="h-7 w-7 animate-spin text-cobalt-400" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity="0.25" strokeWidth="3" />
        <path
          d="M21 12a9 9 0 0 0-9-9"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
    </div>
  )
}
