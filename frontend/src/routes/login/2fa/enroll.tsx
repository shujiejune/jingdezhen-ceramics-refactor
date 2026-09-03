import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { z } from 'zod'

/** OAuth receiver: mandatory 2FA enrollment — hand off to the enroll page. */
const searchSchema = z.object({ pending_token: z.string().optional() })

export const Route = createFileRoute('/login/2fa/enroll')({
  validateSearch: searchSchema,
  component: OAuth2FAEnroll,
})

export function OAuth2FAEnroll() {
  const search = useSearch({ from: '/login/2fa/enroll' })
  const navigate = useNavigate()
  const pending = search.pending_token
  const ran = useRef(false)

  useEffect(() => {
    if (ran.current) return
    ran.current = true
    void navigate({
      to: '/$locale/auth/2fa-enroll',
      params: { locale: 'en-US' },
      search: pending ? { pending } : undefined,
      replace: true,
    })
  }, [navigate, pending])

  return null
}
