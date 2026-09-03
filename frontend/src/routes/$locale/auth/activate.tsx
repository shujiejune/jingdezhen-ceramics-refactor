import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { CheckCircle, SpinnerGap } from '@phosphor-icons/react'
import { useEffect, useRef, useState } from 'react'
import { z } from 'zod'

import { Button } from '~/components/common/ui'
import { api } from '~/lib/api'
import { errorKey, persistSession } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { AuthShell } from './signup'

/**
 * Activation — the email link lands here with ?token=; the page POSTs
 * {token} to /auth/activate and signs in with the returned session.
 */
const searchSchema = z.object({ token: z.string().optional() })

export const Route = createFileRoute('/$locale/auth/activate')({
  validateSearch: searchSchema,
  component: ActivatePage,
})

function ActivatePage() {
  const { t, locale } = useI18n()
  const search = useSearch({ from: '/$locale/auth/activate' })
  const navigate = useNavigate()
  const [state, setState] = useState<'working' | 'ok' | 'failed'>('working')
  const [message, setMessage] = useState('')
  const ran = useRef(false)

  useEffect(() => {
    if (ran.current) return
    ran.current = true
    const token = search.token
    if (!token) {
      setState('failed')
      return
    }
    void api
      .activate(token)
      .then((res) => {
        persistSession(res.access_token, res.user)
        setState('ok')
        setTimeout(() => void navigate({ to: `/${locale}` }), 1200)
      })
      .catch((e: unknown) => {
        setMessage(e instanceof Error ? e.message : '')
        setState('failed')
      })
  }, [search.token, locale, navigate])

  void errorKey
  return (
    <AuthShell>
      <div className="flex flex-col items-center gap-3 text-center">
        {state === 'working' && (
          <>
            <SpinnerGap size={32} className="animate-spin text-cobalt-500" />
            <h1 className="text-[1.2rem] font-semibold text-ink-900">{t('auth.activateTitle')}</h1>
          </>
        )}
        {state === 'ok' && (
          <>
            <CheckCircle size={32} weight="duotone" className="text-[color:var(--color-success)]" />
            <h1 className="text-[1.2rem] font-semibold text-ink-900">
              {t('auth.activateSuccess')}
            </h1>
          </>
        )}
        {state === 'failed' && (
          <>
            <h1 className="text-[1.2rem] font-semibold text-ink-900">{t('auth.activateFailed')}</h1>
            <Button
              variant="secondary"
              className="mt-3"
              onClick={() => void navigate({ to: '/$locale/auth/login', params: { locale } })}
            >
              {t('auth.loginLink')}
            </Button>
          </>
        )}
        {state === 'failed' && message && <p className="text-[0.78rem] text-ink-300">{message}</p>}
      </div>
    </AuthShell>
  )
}
