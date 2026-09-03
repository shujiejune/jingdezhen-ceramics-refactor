import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { EnvelopeSimple } from '@phosphor-icons/react'
import { useState } from 'react'

import { SealMark, PetalScatter } from '~/components/ornaments'
import { Button, FieldError } from '~/components/common/ui'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'

/**
 * Sign-up — real contract: 201 with an EMPTY access_token, the account
 * activates via the email link (POST /auth/activate from the activate
 * page). Mock mode surfaces the activation token as a dev shortcut.
 */
export const Route = createFileRoute('/$locale/auth/signup')({
  component: SignupPage,
})

function SignupPage() {
  const { t, locale } = useI18n()
  const { signup } = useAuth()
  const navigate = useNavigate()

  const [form, setForm] = useState({ nickname: '', email: '', password: '', confirm: '' })
  const [agreeToS, setAgreeToS] = useState(false)
  const [agreePrivacy, setAgreePrivacy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [checkEmail, setCheckEmail] = useState<{ email: string; token?: string } | null>(null)

  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))

  if (checkEmail) {
    return (
      <AuthShell>
        <div className="flex flex-col items-center gap-3 text-center">
          <span className="flex h-14 w-14 items-center justify-center rounded-full bg-cobalt-50">
            <EnvelopeSimple size={26} className="text-cobalt-600" weight="duotone" />
          </span>
          <h1 className="text-[1.3rem] font-semibold tracking-tight text-ink-900">
            {t('auth.checkEmailTitle')}
          </h1>
          <p className="max-w-sm text-[0.88rem] leading-relaxed text-ink-500">
            {t('auth.checkEmailBody', { email: checkEmail.email })}
          </p>
          {checkEmail.token && (
            <p className="mt-2 text-[0.78rem] text-ink-400">
              {t('auth.devActivate')}{' '}
              <Link
                to="/$locale/auth/activate"
                params={{ locale }}
                search={{ token: checkEmail.token }}
                className="link-quiet"
              >
                /auth/activate?token={checkEmail.token.slice(0, 18)}…
              </Link>
            </p>
          )}
        </div>
      </AuthShell>
    )
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    const errs: Record<string, string> = {}
    if (form.password.length < 8) errs.password = t('errors.passwordShort')
    if (form.password !== form.confirm) errs.confirm = t('errors.passwordMismatch')
    if (!agreeToS || !agreePrivacy) {
      setError(t('errors.consent_required'))
      return
    }
    setFieldErrors(errs)
    if (Object.keys(errs).length > 0) return

    setBusy(true)
    try {
      const res = await signup(form.email, form.password, form.nickname || form.email.split('@')[0])
      if (res.needsActivation) {
        setCheckEmail({ email: form.email, token: res.activationToken })
      } else {
        await navigate({ to: `/${locale}` })
      }
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell>
      <h1 className="text-center text-[1.45rem] font-semibold tracking-tight text-ink-900">
        {t('auth.signupTitle')}
      </h1>
      <p className="mt-2 text-center text-[0.88rem] text-ink-500">{t('auth.signupSub')}</p>

      <form onSubmit={submit} className="mt-7 flex flex-col gap-4">
        <div>
          <label className="label-base" htmlFor="su-nickname">
            {t('auth.nickname')} <span className="text-ink-300">({t('common.optional')})</span>
          </label>
          <input
            id="su-nickname"
            className="input-base"
            value={form.nickname}
            onChange={set('nickname')}
          />
        </div>
        <div>
          <label className="label-base" htmlFor="su-email">
            {t('auth.email')}
          </label>
          <input
            id="su-email"
            type="email"
            required
            autoComplete="email"
            className="input-base"
            value={form.email}
            onChange={set('email')}
          />
        </div>
        <div>
          <label className="label-base" htmlFor="su-password">
            {t('auth.password')}
          </label>
          <input
            id="su-password"
            type="password"
            required
            autoComplete="new-password"
            className="input-base"
            value={form.password}
            onChange={set('password')}
          />
          <FieldError>{fieldErrors.password}</FieldError>
        </div>
        <div>
          <label className="label-base" htmlFor="su-confirm">
            {t('auth.confirmPassword')}
          </label>
          <input
            id="su-confirm"
            type="password"
            required
            autoComplete="new-password"
            className="input-base"
            value={form.confirm}
            onChange={set('confirm')}
          />
          <FieldError>{fieldErrors.confirm}</FieldError>
        </div>

        <label className="flex cursor-pointer items-start gap-2.5 text-[0.82rem] leading-snug text-ink-600">
          <input
            type="checkbox"
            checked={agreeToS}
            onChange={(e) => setAgreeToS(e.target.checked)}
            className="mt-0.5 h-4 w-4 accent-[var(--cobalt-600)]"
          />
          {t('auth.tosAgree')}
        </label>
        <label className="-mt-2 flex cursor-pointer items-start gap-2.5 text-[0.82rem] leading-snug text-ink-600">
          <input
            type="checkbox"
            checked={agreePrivacy}
            onChange={(e) => setAgreePrivacy(e.target.checked)}
            className="mt-0.5 h-4 w-4 accent-[var(--cobalt-600)]"
          />
          {t('auth.privacyAgree')}
        </label>

        {error && <FieldError>{error}</FieldError>}
        <Button type="submit" size="lg" loading={busy}>
          {t('auth.createAccount')}
        </Button>
      </form>

      <p className="mt-6 text-center text-[0.84rem] text-ink-400">
        {t('auth.haveAccount')}{' '}
        <Link to="/$locale/auth/login" params={{ locale }} className="link-quiet">
          {t('auth.loginLink')}
        </Link>
      </p>
    </AuthShell>
  )
}

/** shared auth card shell (seal + card + watermark) */
export function AuthShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative overflow-hidden">
      <div className="qinghua-watermark absolute inset-x-0 top-0 h-72 opacity-70" />
      <PetalScatter
        seed={55}
        className="pointer-events-none absolute top-24 left-[12%] opacity-60"
      />
      <div className="relative mx-auto flex max-w-md flex-col px-4 pt-16 pb-20 sm:px-6">
        <div className="flex justify-center">
          <SealMark size={48} />
        </div>
        <div className="card-surface mt-7 p-8">{children}</div>
      </div>
    </div>
  )
}
