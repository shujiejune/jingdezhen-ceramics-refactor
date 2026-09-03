import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'

import { Button, FieldError } from '~/components/common/ui'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { noindexHead } from '~/lib/seo'
import { AuthShell } from './signup'

/** Forgot password — anti-enumeration: the API always answers 200. */
export const Route = createFileRoute('/$locale/auth/forgot')({
  head: () => noindexHead('Forgot password — Jingdezhen Ceramics'),
  component: ForgotPage,
})

function ForgotPage() {
  const { t, locale } = useI18n()
  void useAuth()
  const [email, setEmail] = useState('')
  const [busy, setBusy] = useState(false)
  const [sent, setSent] = useState(false)
  const [devToken, setDevToken] = useState<string | undefined>()
  const [error, setError] = useState<string | null>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const res = await api.requestPasswordReset(email)
      setDevToken(res.reset_token)
      setSent(true)
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell>
      <h1 className="text-center text-[1.35rem] font-semibold tracking-tight text-ink-900">
        {t('auth.forgotTitle')}
      </h1>
      <p className="mt-2 text-center text-[0.88rem] text-ink-500">{t('auth.forgotBody')}</p>

      {sent ? (
        <div className="mt-7 flex flex-col items-center gap-3 text-center">
          <p className="rounded-md bg-[color:var(--color-success-bg)] px-4 py-3 text-[0.88rem] text-[color:var(--color-success)]">
            {t('auth.forgotSent')}
          </p>
          {devToken && (
            <p className="text-[0.78rem] text-ink-400">
              {t('auth.devActivate')}{' '}
              <Link
                to="/$locale/auth/reset"
                params={{ locale }}
                search={{ token: devToken }}
                className="link-quiet"
              >
                /auth/reset?token={devToken.slice(0, 16)}…
              </Link>
            </p>
          )}
        </div>
      ) : (
        <form onSubmit={submit} className="mt-7 flex flex-col gap-4">
          <div>
            <label className="label-base" htmlFor="fp-email">
              {t('auth.email')}
            </label>
            <input
              id="fp-email"
              type="email"
              required
              autoComplete="email"
              className="input-base"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          {error && <FieldError>{error}</FieldError>}
          <Button type="submit" size="lg" loading={busy}>
            {t('auth.forgotCta')}
          </Button>
        </form>
      )}

      <p className="mt-6 text-center text-[0.84rem] text-ink-400">
        <Link to="/$locale/auth/login" params={{ locale }} className="link-quiet">
          ← {t('auth.backToLogin')}
        </Link>
      </p>
    </AuthShell>
  )
}
