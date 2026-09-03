import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'

/** OAuth receiver: failure → the login page shows the error. */
export const Route = createFileRoute('/login/error')({
  component: OAuthError,
})

export function OAuthError() {
  const navigate = useNavigate()
  const ran = useRef(false)

  useEffect(() => {
    if (ran.current) return
    ran.current = true
    void navigate({
      to: '/$locale/auth/login',
      params: { locale: 'en-US' },
      search: { oauthError: 1 },
      replace: true,
    })
  }, [navigate])

  return null
}
