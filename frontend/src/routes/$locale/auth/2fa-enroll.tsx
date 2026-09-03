import { createFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { CheckCircle, CopySimple, ShieldCheck } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { z } from 'zod'

import { Button, FieldError } from '~/components/common/ui'
import { api } from '~/lib/api'
import { errorKey, persistSession } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { AuthShell } from './signup'

/**
 * Mandatory 2FA enrollment for super_admin (PRD §3.4.1): the login
 * challenge answers 2fa_enrollment_required with a pending token; this
 * page confirms the password (pending-enroll → otpauth QR + secret),
 * verifies a code (pending-confirm), and shows the backup codes ONCE.
 */
const searchSchema = z.object({ pending: z.string().optional() })

export const Route = createFileRoute('/$locale/auth/2fa-enroll')({
  validateSearch: searchSchema,
  component: EnrollPage,
})

function EnrollPage() {
  const { t, locale } = useI18n()
  const search = useSearch({ from: '/$locale/auth/2fa-enroll' })
  const navigate = useNavigate()

  const [step, setStep] = useState<'password' | 'scan' | 'backup' | 'done'>('password')
  const [password, setPassword] = useState('')
  const [secret, setSecret] = useState('')
  const [otpauth, setOtpauth] = useState('')
  const [qrData, setQrData] = useState('')
  const [code, setCode] = useState('')
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (otpauth) {
      void QRCode.toDataURL(otpauth, { margin: 1, width: 220, color: { dark: '#182e6c' } }).then(
        setQrData,
      )
    }
  }, [otpauth])

  const withPending = (fn: (pending: string) => Promise<void>) => {
    if (!search.pending) {
      setError(t('auth.activateFailed'))
      return
    }
    setBusy(true)
    setError(null)
    fn(search.pending)
      .catch((err: unknown) => {
        setError(t(errorKey(err) as Parameters<typeof t>[0]))
      })
      .finally(() => setBusy(false))
  }

  const startEnroll = () =>
    withPending(async (pending) => {
      const res = await api.pending2FAEnroll(pending, password)
      setSecret(res.secret)
      setOtpauth(res.otpauth_url)
      setStep('scan')
    })

  const confirmEnroll = () =>
    withPending(async (pending) => {
      const res = await api.pending2FAConfirm(pending, code)
      persistSession(res.access_token, res.user)
      setBackupCodes(res.backup_codes ?? [])
      setStep('backup')
    })

  if (step === 'backup') {
    return (
      <AuthShell>
        <div className="flex flex-col items-center gap-3 text-center">
          <ShieldCheck size={32} weight="duotone" className="text-cobalt-600" />
          <h1 className="text-[1.3rem] font-semibold tracking-tight text-ink-900">
            {t('auth.backupTitle')}
          </h1>
          <p className="max-w-sm text-[0.85rem] leading-relaxed text-ink-500">
            {t('auth.backupBody')}
          </p>
          <div className="mt-2 grid w-full grid-cols-2 gap-2">
            {backupCodes.map((c) => (
              <code
                key={c}
                className="rounded border border-cobalt-100 bg-mist px-3 py-2 font-mono text-[0.9rem] text-ink-800"
              >
                {c}
              </code>
            ))}
          </div>
          <Button
            className="mt-5"
            onClick={() => {
              setStep('done')
              void navigate({ to: `/${locale}` })
            }}
          >
            <CheckCircle size={16} weight="duotone" />
            {t('auth.backupDone')}
          </Button>
        </div>
      </AuthShell>
    )
  }

  return (
    <AuthShell>
      <h1 className="text-center text-[1.35rem] font-semibold tracking-tight text-ink-900">
        {t('auth.enrollTitle')}
      </h1>
      <p className="mt-2 text-center text-[0.86rem] leading-relaxed text-ink-500">
        {t('auth.enrollBody')}
      </p>

      {step === 'password' && (
        <div className="mt-7 flex flex-col gap-4">
          <div>
            <label className="label-base" htmlFor="en-password">
              {t('auth.enrollPassword')}
            </label>
            <input
              id="en-password"
              type="password"
              required
              autoComplete="current-password"
              className="input-base"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          {error && <FieldError>{error}</FieldError>}
          <Button size="lg" loading={busy} onClick={() => void startEnroll()}>
            {t('auth.enrollStart')}
          </Button>
        </div>
      )}

      {step === 'scan' && (
        <div className="mt-6 flex flex-col items-center gap-4">
          <h2 className="text-[1.05rem] font-semibold text-ink-900">{t('auth.enrollScanTitle')}</h2>
          {qrData && (
            <img
              src={qrData}
              alt="2FA enrollment QR code"
              className="rounded-md border border-cobalt-100 bg-white p-2 shadow-card"
              width={220}
              height={220}
            />
          )}
          <p className="text-[0.8rem] text-ink-500">{t('auth.enrollScanBody')}</p>
          <button
            type="button"
            className="flex items-center gap-1.5 rounded border border-cobalt-100 bg-mist px-3 py-1.5 font-mono text-[0.8rem] text-ink-700"
            onClick={() => {
              void navigator.clipboard?.writeText(secret).then(() => {
                setCopied(true)
                setTimeout(() => setCopied(false), 1500)
              })
            }}
          >
            {copied ? <CheckCircle size={13} /> : <CopySimple size={13} />}
            {secret}
          </button>
          <div className="mt-2 w-full">
            <label className="label-base" htmlFor="en-code">
              {t('auth.enrollCodeLabel')}
            </label>
            <input
              id="en-code"
              required
              inputMode="numeric"
              maxLength={6}
              className="input-base text-center text-lg tracking-[0.4em]"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="••••••"
            />
          </div>
          {error && <FieldError>{error}</FieldError>}
          <Button size="lg" className="w-full" loading={busy} onClick={() => void confirmEnroll()}>
            {t('auth.enrollConfirm')}
          </Button>
        </div>
      )}
    </AuthShell>
  )
}
