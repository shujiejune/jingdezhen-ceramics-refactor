import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { z } from 'zod'

/** OAuth receiver: 2FA challenge pending — hand off to the login page. */
const searchSchema = z.object({ pending_token: z.string().optional() })

export const Route = createFileRoute('/login/2fa')({
  validateSearch: searchSchema,
  component: OAuth2FA,
})

export function OAuth2FA() {
  const search = useSearch({ from: '/login/2fa' })
  const navigate = useNavigate()
  const pending = search.pending_token
  const ran = useRef(false)

  useEffect(() => {
    if (ran.current) return
    ran.current = true
    void navigate({
      to: '/$locale/auth/login',
      params: { locale: 'en-US' },
      search: pending ? { pending2fa: pending } : { oauthError: 1 },
      replace: true,
    })
  }, [navigate, pending])

  return null
}
