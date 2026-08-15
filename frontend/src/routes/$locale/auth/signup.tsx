import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { SealMark, PetalScatter } from '~/components/ornaments'
import { Button, FieldError } from '~/components/common/ui'
import { errorKey, useAuth } from '~/lib/auth'
import { useCart } from '~/lib/cart'
import { useI18n } from '~/lib/i18n'

/** Sign-up — mandatory Privacy/ToS consent checkboxes (PRD §4.3). */
export const Route = createFileRoute('/$locale/auth/signup')({
  component: SignupPage,
})

function SignupPage() {
  const { t, locale } = useI18n()
  const { signup } = useAuth()
  const { refresh } = useCart()
  const navigate = useNavigate()

  const [form, setForm] = useState({ nickname: '', email: '', password: '', confirm: '' })
  const [agreeToS, setAgreeToS] = useState(false)
  const [agreePrivacy, setAgreePrivacy] = useState(false)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))

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
      await signup(form.email, form.password, form.nickname || form.email.split('@')[0])
      await refresh()
      await navigate({ to: `/${locale}` })
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="relative overflow-hidden">
      <div className="qinghua-watermark absolute inset-x-0 top-0 h-72 opacity-70" />
      <PetalScatter seed={55} className="pointer-events-none absolute top-24 left-[12%] opacity-60" />
      <div className="relative mx-auto flex max-w-md flex-col px-4 pt-16 pb-20 sm:px-6">
        <div className="flex justify-center">
          <SealMark size={48} />
        </div>

        <div className="card-surface mt-7 p-8">
          <h1 className="text-center text-[1.45rem] font-semibold tracking-tight text-ink-900">
            {t('auth.signupTitle')}
          </h1>
          <p className="mt-2 text-center text-[0.88rem] text-ink-500">{t('auth.signupSub')}</p>

          <form onSubmit={submit} className="mt-7 flex flex-col gap-4">
            <div>
              <label className="label-base">
                {t('auth.nickname')} <span className="text-ink-300">({t('common.optional')})</span>
              </label>
              <input className="input-base" value={form.nickname} onChange={set('nickname')} />
            </div>
            <div>
              <label className="label-base" htmlFor="su-email">{t('auth.email')}</label>
              <input id="su-email" type="email" required autoComplete="email" className="input-base" value={form.email} onChange={set('email')} />
            </div>
            <div>
              <label className="label-base" htmlFor="su-password">{t('auth.password')}</label>
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
              <label className="label-base" htmlFor="su-confirm">{t('auth.confirmPassword')}</label>
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
              <input type="checkbox" checked={agreeToS} onChange={(e) => setAgreeToS(e.target.checked)} className="mt-0.5 h-4 w-4 accent-[var(--cobalt-600)]" />
              {t('auth.tosAgree')}
            </label>
            <label className="-mt-2 flex cursor-pointer items-start gap-2.5 text-[0.82rem] leading-snug text-ink-600">
              <input type="checkbox" checked={agreePrivacy} onChange={(e) => setAgreePrivacy(e.target.checked)} className="mt-0.5 h-4 w-4 accent-[var(--cobalt-600)]" />
              {t('auth.privacyAgree')}
            </label>

            {error && <FieldError>{error}</FieldError>}
            <Button type="submit" size="lg" loading={busy}>
              {t('auth.createAccount')}
            </Button>
          </form>

          <p className="mt-6 text-center text-[0.84rem] text-ink-400">
            {t('auth.haveAccount')}{' '}
            <a
              href="#"
              className="link-quiet"
              onClick={(e) => {
                e.preventDefault()
                void navigate({ to: '/$locale/auth/login', params: { locale } })
              }}
            >
              {t('auth.loginLink')}
            </a>
          </p>
        </div>
      </div>
    </div>
  )
}
