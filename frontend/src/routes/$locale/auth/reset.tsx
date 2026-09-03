import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { CheckCircle } from '@phosphor-icons/react'
import { useState } from 'react'
import { z } from 'zod'

import { Button, FieldError } from '~/components/common/ui'
import { api } from '~/lib/api'
import { errorKey, persistSession } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { AuthShell } from './signup'

/** Reset password — email link lands with ?token=. */
const searchSchema = z.object({ token: z.string().optional() })

export const Route = createFileRoute('/$locale/auth/reset')({
  validateSearch: searchSchema,
  component: ResetPage,
})

function ResetPage() {
  const { t, locale } = useI18n()
  const search = useSearch({ from: '/$locale/auth/reset' })
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fieldError, setFieldError] = useState<string | null>(null)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setFieldError(null)
    if (password.length < 8) {
      setFieldError(t('errors.passwordShort'))
      return
    }
    if (password !== confirm) {
      setFieldError(t('errors.passwordMismatch'))
      return
    }
    if (!search.token) {
      setError(t('auth.activateFailed'))
      return
    }
    setBusy(true)
    try {
      const res = await api.resetPassword(search.token, password)
      persistSession(res.access_token, res.user)
      setDone(true)
      setTimeout(() => void navigate({ to: `/${locale}` }), 1400)
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell>
      {done ? (
        <div className="flex flex-col items-center gap-3 text-center">
          <CheckCircle size={32} weight="duotone" className="text-[color:var(--color-success)]" />
          <h1 className="text-[1.2rem] font-semibold text-ink-900">{t('auth.resetSuccess')}</h1>
        </div>
      ) : (
        <>
          <h1 className="text-center text-[1.35rem] font-semibold tracking-tight text-ink-900">
            {t('auth.resetTitle')}
          </h1>
          <form onSubmit={submit} className="mt-7 flex flex-col gap-4">
            <div>
              <label className="label-base" htmlFor="rs-password">
                {t('auth.password')}
              </label>
              <input
                id="rs-password"
                type="password"
                required
                autoComplete="new-password"
                className="input-base"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <div>
              <label className="label-base" htmlFor="rs-confirm">
                {t('auth.confirmPassword')}
              </label>
              <input
                id="rs-confirm"
                type="password"
                required
                autoComplete="new-password"
                className="input-base"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
              />
              <FieldError>{fieldError}</FieldError>
            </div>
            {error && <FieldError>{error}</FieldError>}
            <Button type="submit" size="lg" loading={busy}>
              {t('auth.resetCta')}
            </Button>
          </form>
        </>
      )}
    </AuthShell>
  )
}
