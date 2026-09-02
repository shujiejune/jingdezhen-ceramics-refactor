import { Link, createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { SealCheck } from '@phosphor-icons/react'
import { useState } from 'react'
import { z } from 'zod'

import { SealMark, CloudScroll } from '~/components/ornaments'
import { Button, FieldError } from '~/components/common/ui'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { PetalScatter } from '~/components/ornaments'

/**
 * Sign-in — email + password, with the 2FA verify step when the account
 * has TOTP enabled (contract: login → pending token → /auth/2fa/verify).
 */
const searchSchema = z.object({ returnTo: z.string().optional() })

export const Route = createFileRoute('/$locale/auth/login')({
  validateSearch: searchSchema,
  component: LoginPage,
})

function LoginPage() {
  const { t, locale } = useI18n()
  const { login, verify2FA } = useAuth()
  const search = useSearch({ from: '/$locale/auth/login' })
  const navigate = useNavigate()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [pending2FA, setPending2FA] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const finish = async () => {
    // cart merge/reload is handled by CartProvider's token-change effect
    await navigate({ to: search.returnTo ?? `/${locale}` })
  }

  const onLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const res = await login(email, password)
      if (res.pending2FA) {
        setPending2FA(res.pending2FA)
      } else {
        await finish()
      }
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setBusy(false)
    }
  }

  const onVerify = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      await verify2FA(pending2FA!, code)
      await finish()
    } catch (err) {
      setError(t(errorKey(err) as Parameters<typeof t>[0]))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="relative overflow-hidden">
      <div className="qinghua-watermark absolute inset-x-0 top-0 h-72 opacity-70" />
      <PetalScatter
        seed={77}
        className="pointer-events-none absolute top-24 right-[12%] opacity-60"
      />
      <div className="relative mx-auto flex max-w-md flex-col px-4 pt-16 pb-20 sm:px-6">
        <div className="flex justify-center">
          <SealMark size={48} />
        </div>

        {!pending2FA ? (
          <div className="card-surface mt-7 p-8">
            <h1 className="text-center text-[1.45rem] font-semibold tracking-tight text-ink-900">
              {t('auth.loginTitle')}
            </h1>
            <p className="mt-2 text-center text-[0.88rem] text-ink-500">{t('auth.loginSub')}</p>

            <form onSubmit={onLogin} className="mt-7 flex flex-col gap-4">
              <div>
                <label className="label-base">{t('auth.email')}</label>
                <input
                  type="email"
                  required
                  autoComplete="email"
                  className="input-base"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@example.com"
                />
              </div>
              <div>
                <label className="label-base">{t('auth.password')}</label>
                <input
                  type="password"
                  required
                  autoComplete="current-password"
                  className="input-base"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
              {error && <FieldError>{error}</FieldError>}
              <Button type="submit" size="lg" loading={busy}>
                {t('auth.login')}
              </Button>
            </form>

            <div className="my-6 flex items-center gap-4">
              <span className="h-px flex-1 bg-cobalt-100" />
              <span className="text-[0.75rem] text-ink-300">{t('auth.or')}</span>
              <span className="h-px flex-1 bg-cobalt-100" />
            </div>

            <button
              type="button"
              disabled
              title="Google OAuth — post-MVP at launch (PRD §7)"
              className="flex h-11 w-full items-center justify-center gap-2.5 rounded-lg border border-ink-300/40 bg-white text-[0.88rem] font-medium text-ink-600 opacity-60"
            >
              <svg width="17" height="17" viewBox="0 0 24 24" aria-hidden="true">
                <path
                  fill="#4285F4"
                  d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.27-4.74 3.27-8.1Z"
                />
                <path
                  fill="#34A853"
                  d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23Z"
                />
                <path
                  fill="#FBBC05"
                  d="M5.84 14.1a6.6 6.6 0 0 1 0-4.2V7.06H2.18a11 11 0 0 0 0 9.88l3.66-2.84Z"
                />
                <path
                  fill="#EA4335"
                  d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15A11 11 0 0 0 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52Z"
                />
              </svg>
              {t('auth.google')}
            </button>

            <p className="mt-6 text-center text-[0.84rem] text-ink-400">
              {t('auth.noAccount')}{' '}
              <Link
                to="/$locale/auth/signup"
                params={{ locale }}
                search={search.returnTo ? { returnTo: search.returnTo } : undefined}
                className="link-quiet"
              >
                {t('auth.signupLink')}
              </Link>
            </p>
          </div>
        ) : (
          <div className="card-surface mt-7 p-8">
            <div className="flex justify-center">
              <CloudScroll size={34} />
            </div>
            <h1 className="mt-4 text-center text-[1.35rem] font-semibold tracking-tight text-ink-900">
              {t('auth.twoFactorTitle')}
            </h1>
            <p className="mt-2 text-center text-[0.88rem] text-ink-500">{t('auth.twoFactorSub')}</p>

            <form onSubmit={onVerify} className="mt-7 flex flex-col gap-4">
              <div>
                <label className="label-base" htmlFor="login-2fa">
                  {t('auth.codeLabel')}
                </label>
                <input
                  id="login-2fa"
                  required
                  inputMode="numeric"
                  pattern="[0-9]*"
                  maxLength={6}
                  autoComplete="one-time-code"
                  className="input-base text-center text-lg tracking-[0.5em]"
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder="••••••"
                />
              </div>
              {error && <FieldError>{error}</FieldError>}
              <Button type="submit" size="lg" loading={busy}>
                <SealCheck size={16} weight="duotone" />
                {t('auth.verify')}
              </Button>
              <button
                type="button"
                className="text-[0.82rem] text-ink-400 hover:text-cobalt-600"
                onClick={() => {
                  setPending2FA(null)
                  setError(null)
                }}
              >
                ← {t('auth.backToLogin')}
              </button>
            </form>
          </div>
        )}

        {/* demo credentials */}
        <div className="mt-6 rounded-xl border border-dashed border-cobalt-200 bg-wash p-4">
          <p className="text-[0.72rem] font-semibold tracking-[0.16em] text-cobalt-500 uppercase">
            {t('auth.demoTitle')}
          </p>
          <ul className="mt-2 flex flex-col gap-1 text-[0.8rem] text-ink-500">
            <li>{t('auth.demoCustomer')}</li>
            <li>{t('auth.demoAdmin')}</li>
            <li className="text-ink-400">{t('auth.demoPassword')}</li>
          </ul>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setEmail('emily@demo.dev')
                setPassword('porcelain123')
                setPending2FA(null)
              }}
            >
              emily
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setEmail('admin@demo.dev')
                setPassword('porcelain123')
                setPending2FA(null)
              }}
            >
              admin (2FA)
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
