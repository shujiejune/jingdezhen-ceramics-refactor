import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle,
  ClockCountdown,
  FloppyDisk,
} from '@phosphor-icons/react'
import { useEffect, useMemo, useState } from 'react'
import { z } from 'zod'

import { PetalScatter, WaveBand } from '~/components/ornaments'
import { Badge, Button, FieldError, Spinner } from '~/components/common/ui'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { formatMinor } from '~/lib/money'
import { INTEREST_OPTIONS } from '~/mocks/data'
import { SUPPORTED_CURRENCIES, cn, formatDate } from '~/lib/utils'
import type { ItineraryRequest } from '~/lib/types'

/**
 * Custom Itinerary Builder — 4-step wizard + review (PRD §3.3.2).
 * The step lives in the URL (?step=) so refresh/share keeps position.
 * Draft persists to localStorage (prototype stand-in for the server-side
 * itinerary_drafts save-resume); submission requires sign-in and consent.
 */
const searchSchema = z.object({ step: z.number().int().min(1).max(5).optional() })

export const Route = createFileRoute('/$locale/itinerary')({
  validateSearch: searchSchema,
  component: ItineraryWizard,
})

/* --------------------------- form model --------------------------- */

interface WizardState {
  arrival_date: string
  duration_days: number
  flexible: boolean
  adults: number
  children: number
  interests: string[]
  budget_currency: 'USD' | 'EUR' | 'GBP'
  budget_min: number // major units per person
  budget_max: number
  pace: 'relaxed' | 'balanced' | 'packed'
  guide: 'none' | 'english' | 'other'
  hotel: boolean
  hotel_level: 'budget' | 'comfort' | 'luxury'
  pickup: boolean
  experience: boolean
  dietary: string
  channel: 'email' | 'whatsapp'
  whatsapp_number: string
  notes: string
  consent: boolean
}

const DRAFT_KEY = 'jdz.itineraryDraft'

const initialState: WizardState = {
  arrival_date: '',
  duration_days: 3,
  flexible: false,
  adults: 2,
  children: 0,
  interests: [],
  budget_currency: 'USD',
  budget_min: 1500,
  budget_max: 3000,
  pace: 'balanced',
  guide: 'english',
  hotel: true,
  hotel_level: 'comfort',
  pickup: true,
  experience: true,
  dietary: '',
  channel: 'email',
  whatsapp_number: '',
  notes: '',
  consent: false,
}

function ItineraryWizard() {
  const { t, locale } = useI18n()
  const { ready, token, user } = useAuth()
  const search = Route.useSearch()
  const navigate = useNavigate()
  const step = search.step ?? 1

  const [form, setForm] = useState<WizardState>(initialState)
  const [draftSaved, setDraftSaved] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitted, setSubmitted] = useState<ItineraryRequest | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)

  /* load draft */
  useEffect(() => {
    if (typeof localStorage === 'undefined') return
    const raw = localStorage.getItem(DRAFT_KEY)
    if (raw) {
      try {
        setForm({ ...initialState, ...(JSON.parse(raw) as Partial<WizardState>) })
      } catch {
        /* corrupted draft — start fresh */
      }
    }
  }, [])

  /* autosave draft */
  useEffect(() => {
    if (typeof localStorage === 'undefined') return
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ ...form, consent: false }))
    setDraftSaved(true)
    const tmo = setTimeout(() => setDraftSaved(false), 1600)
    return () => clearTimeout(tmo)
  }, [form])

  const set = <K extends keyof WizardState>(key: K, value: WizardState[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  const goStep = (n: number) => void navigate({ to: '.', search: { step: n }, replace: true })

  /* --------------------------- validation --------------------------- */

  const validateStep = (n: number): boolean => {
    const errs: Record<string, string> = {}
    if (n === 1) {
      if (!form.arrival_date) errs.arrival_date = t('errors.required')
      if (form.duration_days < 1) errs.duration_days = t('errors.required')
      if (form.adults < 1) errs.adults = t('errors.required')
    }
    if (n === 2 && form.budget_min > form.budget_max) {
      errs.budget = t('errors.validation_failed')
    }
    if (n === 4 && form.channel === 'whatsapp' && !form.whatsapp_number) {
      errs.whatsapp_number = t('errors.required')
    }
    setErrors(errs)
    return Object.keys(errs).length === 0
  }

  const next = () => {
    if (!validateStep(step)) return
    goStep(Math.min(5, step + 1))
  }

  const submit = async () => {
    if (!token) {
      void navigate({
        to: '/$locale/auth/login',
        params: { locale },
        search: { returnTo: `/${locale}/itinerary` },
      })
      return
    }
    if (!form.consent) {
      setSubmitError(t('errors.consent_required'))
      return
    }
    setSubmitting(true)
    setSubmitError(null)
    try {
      const req = await api.submitItinerary(token, {
        arrival_date: form.arrival_date,
        duration_days: form.duration_days,
        flexible: form.flexible,
        adults: form.adults,
        children: form.children,
        interests: form.interests,
        budget: {
          currency: form.budget_currency,
          min_minor: Math.round(form.budget_min * 100),
          max_minor: Math.round(form.budget_max * 100),
        },
        pace: form.pace,
        services: {
          guide: form.guide,
          hotel: form.hotel,
          hotel_level: form.hotel ? form.hotel_level : undefined,
          pickup: form.pickup,
          experience: form.experience,
          dietary_accessibility: form.dietary || undefined,
        },
        contact: {
          channel: form.channel,
          whatsapp_number: form.channel === 'whatsapp' ? form.whatsapp_number : undefined,
          notes: form.notes || undefined,
        },
        consent: true,
      })
      setSubmitted(req)
      if (typeof localStorage !== 'undefined') localStorage.removeItem(DRAFT_KEY)
    } catch (e) {
      setSubmitError(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setSubmitting(false)
    }
  }

  /* --------------------------- submitted screen --------------------------- */

  const departure = useMemo(() => {
    if (!form.arrival_date) return ''
    const d = new Date(form.arrival_date)
    d.setDate(d.getDate() + form.duration_days)
    return d.toISOString().slice(0, 10)
  }, [form.arrival_date, form.duration_days])

  if (submitted) {
    return (
      <div className="mx-auto max-w-lg px-4 pt-16 pb-10 text-center sm:px-6">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-[color:var(--color-success-bg)]">
          <CheckCircle size={32} weight="duotone" className="text-[color:var(--color-success)]" />
        </div>
        <h1 className="mt-6 text-display-sm text-ink-900">{t('itin.submittedTitle')}</h1>
        <p className="mt-3 leading-relaxed text-ink-500">{t('itin.submittedBody')}</p>
        <div className="mt-6 flex items-center justify-center gap-3">
          <Badge tone="cobalt">
            <ClockCountdown size={13} /> {t('itin.slaBadge')}
          </Badge>
          <Badge tone="neutral">{t('itin.requestN', { id: submitted.id })}</Badge>
        </div>
        <div className="mt-8 flex justify-center gap-3">
          <Link
            to="/$locale/itineraries"
            params={{ locale }}
            className="inline-flex h-11 items-center rounded-lg bg-cobalt-600 px-5 text-[0.95rem] font-medium text-white shadow-card hover:bg-cobalt-700"
          >
            {t('itin.viewMyJourneys')}
          </Link>
        </div>
        <div className="mt-10 flex justify-center">
          <WaveBand width={160} />
        </div>
      </div>
    )
  }

  if (!ready) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  const stepTitles = [
    t('itin.step1'),
    t('itin.step2'),
    t('itin.step3'),
    t('itin.step4'),
    t('itin.review'),
  ]

  return (
    <div className="mx-auto max-w-3xl px-4 pt-10 sm:px-6">
      {/* head */}
      <div className="relative">
        <PetalScatter
          seed={31}
          count={6}
          className="pointer-events-none absolute -top-4 right-0 opacity-50"
        />
        <p className="eyebrow">{t('landing.travelEyebrow')}</p>
        <h1 className="mt-2 text-display-sm text-ink-900">{t('itin.title')}</h1>
        <p className="mt-2 text-[0.92rem] text-ink-500">{t('itin.subtitle')}</p>
        {!token && user === null && (
          <p className="mt-3 text-[0.82rem] text-cobalt-600">
            {t('itin.signInFirst')} —{' '}
            <Link
              to="/$locale/auth/login"
              params={{ locale }}
              search={{ returnTo: `/${locale}/itinerary` }}
              className="link-quiet"
            >
              {t('nav.login')}
            </Link>
          </p>
        )}
      </div>

      {/* progress */}
      <div className="mt-8">
        <div className="flex items-center justify-between">
          {stepTitles.map((title, i) => {
            const n = i + 1
            const done = step > n
            const active = step === n
            return (
              <div key={title} className="flex flex-1 items-center">
                <button
                  type="button"
                  onClick={() => n <= step && goStep(n)}
                  className={cn(
                    'flex h-8 w-8 shrink-0 items-center justify-center rounded-full border text-[0.78rem] font-bold transition',
                    done && 'border-cobalt-600 bg-cobalt-600 text-white',
                    active && 'border-cobalt-600 bg-white text-cobalt-700 ring-2 ring-cobalt-200',
                    !done && !active && 'border-ink-300 bg-white text-ink-300',
                  )}
                >
                  {done ? <CheckCircle size={14} weight="fill" /> : n}
                </button>
                {n < stepTitles.length && (
                  <div
                    className={cn('mx-2 h-px flex-1', step > n ? 'bg-cobalt-500' : 'bg-cobalt-100')}
                  />
                )}
              </div>
            )
          })}
        </div>
        <div className="mt-3 flex items-center justify-between">
          <p className="text-[0.84rem] font-medium text-ink-700">{stepTitles[step - 1]}</p>
          <p className="flex items-center gap-1.5 text-[0.74rem] text-ink-300">
            {draftSaved && (
              <>
                <FloppyDisk size={13} /> {t('itin.draftSaved')}
              </>
            )}
            {t('itin.autosaveNote')}
          </p>
        </div>
      </div>

      {/* --------------------------- steps --------------------------- */}
      <div className="card-surface mt-6 p-6 sm:p-8">
        {step === 1 && (
          <div className="grid gap-6 sm:grid-cols-2">
            <div>
              <label className="label-base">{t('itin.arrivalDate')}</label>
              <input
                type="date"
                className="input-base"
                value={form.arrival_date}
                min={new Date().toISOString().slice(0, 10)}
                onChange={(e) => set('arrival_date', e.target.value)}
              />
              <FieldError>{errors.arrival_date}</FieldError>
              {departure && (
                <p className="mt-1.5 text-[0.76rem] text-ink-300">
                  {t('itin.departure')}: {formatDate(departure, locale)}
                </p>
              )}
            </div>
            <div>
              <label className="label-base">{t('itin.duration')}</label>
              <div className="flex items-center gap-3">
                <input
                  type="number"
                  min={1}
                  max={30}
                  className="input-base w-24"
                  value={form.duration_days}
                  onChange={(e) => set('duration_days', Math.max(1, Number(e.target.value) || 1))}
                />
                <span className="text-[0.85rem] text-ink-400">{t('itin.days')}</span>
              </div>
            </div>
            <div>
              <label className="label-base">{t('itin.adults')}</label>
              <Counter value={form.adults} min={1} onChange={(v) => set('adults', v)} />
            </div>
            <div>
              <label className="label-base">{t('itin.children')}</label>
              <Counter value={form.children} min={0} onChange={(v) => set('children', v)} />
            </div>
            <label className="flex cursor-pointer items-center gap-2.5 text-[0.86rem] text-ink-600 sm:col-span-2">
              <input
                type="checkbox"
                checked={form.flexible}
                onChange={(e) => set('flexible', e.target.checked)}
                className="h-4 w-4 accent-[var(--cobalt-600)]"
              />
              {t('itin.flexible')}
            </label>
          </div>
        )}

        {step === 2 && (
          <div className="flex flex-col gap-8">
            <div>
              <label className="label-base">{t('itin.interests')}</label>
              <p className="-mt-1 mb-3 text-[0.78rem] text-ink-300">{t('itin.interestsSub')}</p>
              <div className="flex flex-wrap gap-2">
                {INTEREST_OPTIONS.map((opt) => {
                  const key = opt.key
                  const label = locale === 'zh-CN' ? opt.translations.zhCN : opt.translations.enUS
                  const active = form.interests.includes(key)
                  return (
                    <button
                      key={key}
                      type="button"
                      onClick={() =>
                        set(
                          'interests',
                          active
                            ? form.interests.filter((i) => i !== key)
                            : [...form.interests, key],
                        )
                      }
                      className={cn(
                        'rounded-full border px-3.5 py-1.5 text-[0.84rem] font-medium transition',
                        active
                          ? 'border-cobalt-600 bg-cobalt-600 text-white'
                          : 'border-cobalt-100 bg-white text-ink-500 hover:border-cobalt-300 hover:text-cobalt-700',
                      )}
                    >
                      {label}
                    </button>
                  )
                })}
              </div>
            </div>

            <div>
              <label className="label-base">{t('itin.budget')}</label>
              <div className="grid grid-cols-3 gap-3">
                <select
                  className="input-base"
                  value={form.budget_currency}
                  onChange={(e) => set('budget_currency', e.target.value as 'USD')}
                >
                  {SUPPORTED_CURRENCIES.map((c) => (
                    <option key={c}>{c}</option>
                  ))}
                </select>
                <input
                  type="number"
                  min={0}
                  className="input-base"
                  value={form.budget_min}
                  onChange={(e) => set('budget_min', Number(e.target.value) || 0)}
                  placeholder={t('itin.budgetMin')}
                />
                <input
                  type="number"
                  min={0}
                  className="input-base"
                  value={form.budget_max}
                  onChange={(e) => set('budget_max', Number(e.target.value) || 0)}
                  placeholder={t('itin.budgetMax')}
                />
              </div>
              <FieldError>{errors.budget}</FieldError>
            </div>

            <div>
              <label className="label-base">{t('itin.pace')}</label>
              <div className="grid gap-3 sm:grid-cols-3">
                {(['relaxed', 'balanced', 'packed'] as const).map((p) => (
                  <button
                    key={p}
                    type="button"
                    onClick={() => set('pace', p)}
                    className={cn(
                      'rounded-xl border px-4 py-3.5 text-[0.88rem] font-medium transition',
                      form.pace === p
                        ? 'border-cobalt-500 bg-cobalt-50/60 text-cobalt-700 ring-1 ring-cobalt-200'
                        : 'border-ink-300/40 text-ink-600 hover:border-cobalt-300',
                    )}
                  >
                    {t(`itin.pace.${p}` as Parameters<typeof t>[0])}
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}

        {step === 3 && (
          <div className="flex flex-col gap-6">
            <div>
              <label className="label-base">{t('itin.guide')}</label>
              <div className="grid gap-3 sm:grid-cols-3">
                {(['none', 'english', 'other'] as const).map((g) => (
                  <button
                    key={g}
                    type="button"
                    onClick={() => set('guide', g)}
                    className={cn(
                      'rounded-xl border px-4 py-3 text-[0.86rem] font-medium transition',
                      form.guide === g
                        ? 'border-cobalt-500 bg-cobalt-50/60 text-cobalt-700 ring-1 ring-cobalt-200'
                        : 'border-ink-300/40 text-ink-600 hover:border-cobalt-300',
                    )}
                  >
                    {t(`itin.guide.${g}` as Parameters<typeof t>[0])}
                  </button>
                ))}
              </div>
            </div>

            <ToggleRow
              label={t('itin.hotel')}
              checked={form.hotel}
              onChange={(v) => set('hotel', v)}
            >
              {form.hotel && (
                <div className="mt-3 flex gap-2.5">
                  {(['budget', 'comfort', 'luxury'] as const).map((l) => (
                    <button
                      key={l}
                      type="button"
                      onClick={() => set('hotel_level', l)}
                      className={cn(
                        'rounded-full border px-3.5 py-1.5 text-[0.8rem] font-medium transition',
                        form.hotel_level === l
                          ? 'border-cobalt-600 bg-cobalt-600 text-white'
                          : 'border-cobalt-100 bg-white text-ink-500 hover:border-cobalt-300',
                      )}
                    >
                      {t(`itin.hotelLevel.${l}` as Parameters<typeof t>[0])}
                    </button>
                  ))}
                </div>
              )}
            </ToggleRow>

            <ToggleRow
              label={t('itin.pickup')}
              checked={form.pickup}
              onChange={(v) => set('pickup', v)}
            />
            <ToggleRow
              label={t('itin.experience')}
              checked={form.experience}
              onChange={(v) => set('experience', v)}
            />

            <div>
              <label className="label-base">
                {t('itin.dietary')} <span className="text-ink-300">({t('common.optional')})</span>
              </label>
              <textarea
                className="input-base min-h-20"
                value={form.dietary}
                onChange={(e) => set('dietary', e.target.value)}
                placeholder={t('itin.dietaryPlaceholder')}
              />
            </div>
          </div>
        )}

        {step === 4 && (
          <div className="flex flex-col gap-6">
            <div>
              <label className="label-base">{t('itin.channel')}</label>
              <div className="grid gap-3 sm:grid-cols-2">
                {(['email', 'whatsapp'] as const).map((c) => (
                  <button
                    key={c}
                    type="button"
                    onClick={() => set('channel', c)}
                    className={cn(
                      'rounded-xl border px-4 py-3.5 text-left transition',
                      form.channel === c
                        ? 'border-cobalt-500 bg-cobalt-50/60 ring-1 ring-cobalt-200'
                        : 'border-ink-300/40 hover:border-cobalt-300',
                    )}
                  >
                    <span className="block text-[0.88rem] font-medium text-ink-800">
                      {t(`itin.channel.${c}` as Parameters<typeof t>[0])}
                    </span>
                    {c === 'email' && user && (
                      <span className="mt-0.5 block text-[0.76rem] text-ink-400">{user.email}</span>
                    )}
                  </button>
                ))}
              </div>
            </div>

            {form.channel === 'whatsapp' && (
              <div>
                <label className="label-base">{t('itin.whatsappNumber')}</label>
                <input
                  className="input-base"
                  value={form.whatsapp_number}
                  onChange={(e) => set('whatsapp_number', e.target.value)}
                  placeholder="+1 555 000 0000"
                />
                <FieldError>{errors.whatsapp_number}</FieldError>
              </div>
            )}

            <div>
              <label className="label-base">
                {t('itin.notes')} <span className="text-ink-300">({t('common.optional')})</span>
              </label>
              <textarea
                className="input-base min-h-24"
                value={form.notes}
                onChange={(e) => set('notes', e.target.value)}
                placeholder={t('itin.notesPlaceholder')}
              />
            </div>

            <label className="flex cursor-pointer items-start gap-2.5 text-[0.84rem] leading-snug text-ink-600">
              <input
                type="checkbox"
                checked={form.consent}
                onChange={(e) => set('consent', e.target.checked)}
                className="mt-0.5 h-4 w-4 accent-[var(--cobalt-600)]"
              />
              {t('itin.consent')}
            </label>
          </div>
        )}

        {step === 5 && (
          <div className="flex flex-col gap-6">
            <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('itin.reviewTitle')}</h2>

            <ReviewSection onEdit={() => goStep(1)} title={t('itin.step1')}>
              <p>
                {t('itin.arrivalDate')}: <strong>{form.arrival_date}</strong> · {form.duration_days}{' '}
                {t('itin.days')}
                {form.flexible && ` · ${t('itin.flexible')}`}
              </p>
              <p className="mt-1">
                {t('itin.travelers')}: {form.adults} + {form.children} 🧒
              </p>
            </ReviewSection>

            <ReviewSection onEdit={() => goStep(2)} title={t('itin.step2')}>
              <p>{form.interests.map((k) => interestLabel(k, locale)).join(' · ')}</p>
              <p className="mt-1">
                {t('itin.budget')}:{' '}
                {formatMinor(form.budget_min * 100, form.budget_currency, locale)} –{' '}
                {formatMinor(form.budget_max * 100, form.budget_currency, locale)} ·{' '}
                {t(`itin.pace.${form.pace}` as Parameters<typeof t>[0])}
              </p>
            </ReviewSection>

            <ReviewSection onEdit={() => goStep(3)} title={t('itin.step3')}>
              <p>
                {t(`itin.guide.${form.guide}` as Parameters<typeof t>[0])}
                {form.hotel &&
                  ` · ${t('itin.hotel')} (${t(`itin.hotelLevel.${form.hotel_level}` as Parameters<typeof t>[0])})`}
                {form.pickup && ` · ${t('itin.pickup')}`}
                {form.experience && ` · ${t('itin.experience')}`}
              </p>
              {form.dietary && <p className="mt-1 text-ink-400">{form.dietary}</p>}
            </ReviewSection>

            <ReviewSection onEdit={() => goStep(4)} title={t('itin.step4')}>
              <p>
                {t(`itin.channel.${form.channel}` as Parameters<typeof t>[0])}
                {form.channel === 'whatsapp' && ` · ${form.whatsapp_number}`}
              </p>
              {form.notes && <p className="mt-1 text-ink-400">“{form.notes}”</p>}
            </ReviewSection>

            {!token && (
              <p className="rounded-lg border border-[color:var(--color-warning)]/30 bg-[color:var(--color-warning-bg)] px-4 py-3 text-[0.84rem] text-ink-600">
                {t('itin.signInBody')}{' '}
                <Link
                  to="/$locale/auth/login"
                  params={{ locale }}
                  search={{ returnTo: `/${locale}/itinerary` }}
                  className="link-quiet"
                >
                  {t('nav.login')}
                </Link>
              </p>
            )}
            {submitError && <FieldError>{submitError}</FieldError>}
          </div>
        )}

        {/* nav */}
        <div className="mt-8 flex items-center justify-between border-t border-cobalt-50 pt-6">
          {step > 1 ? (
            <Button variant="ghost" onClick={() => goStep(step - 1)}>
              <ArrowLeft size={15} />
              {t('common.back')}
            </Button>
          ) : (
            <span />
          )}
          {step < 5 ? (
            <Button onClick={next}>
              {t('common.continue')}
              <ArrowRight size={15} weight="bold" />
            </Button>
          ) : (
            <Button size="lg" loading={submitting} onClick={() => void submit()}>
              <CheckCircle size={16} weight="duotone" />
              {t('itin.submit')}
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

/* --------------------------- small parts --------------------------- */

function interestLabel(key: string, locale: string): string {
  const opt = INTEREST_OPTIONS.find((o) => o.key === key)
  if (!opt) return key
  return locale === 'zh-CN' ? opt.translations.zhCN : opt.translations.enUS
}

function Counter({
  value,
  min,
  onChange,
}: {
  value: number
  min: number
  onChange: (v: number) => void
}) {
  return (
    <div className="inline-flex h-11 items-center rounded-lg border border-ink-300/50">
      <button
        type="button"
        className="h-full w-10 text-ink-500 hover:text-cobalt-600 disabled:opacity-30"
        disabled={value <= min}
        onClick={() => onChange(value - 1)}
      >
        −
      </button>
      <span className="w-10 text-center font-medium text-ink-800">{value}</span>
      <button
        type="button"
        className="h-full w-10 text-ink-500 hover:text-cobalt-600"
        onClick={() => onChange(value + 1)}
      >
        +
      </button>
    </div>
  )
}

function ToggleRow({
  label,
  checked,
  onChange,
  children,
}: {
  label: string
  checked: boolean
  onChange: (v: boolean) => void
  children?: React.ReactNode
}) {
  return (
    <div
      className={cn(
        'rounded-xl border p-4 transition',
        checked ? 'border-cobalt-200 bg-cobalt-50/40' : 'border-ink-300/30',
      )}
    >
      <label className="flex cursor-pointer items-center justify-between">
        <span className="text-[0.88rem] font-medium text-ink-700">{label}</span>
        <input
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          className="h-4 w-4 accent-[var(--cobalt-600)]"
        />
      </label>
      {children}
    </div>
  )
}

function ReviewSection({
  title,
  onEdit,
  children,
}: {
  title: string
  onEdit: () => void
  children: React.ReactNode
}) {
  const { t } = useI18n()
  return (
    <div className="rounded-xl border border-cobalt-100 bg-wash/50 p-4">
      <div className="flex items-center justify-between">
        <h3 className="text-[0.8rem] font-semibold tracking-wide text-cobalt-600 uppercase">
          {title}
        </h3>
        <button
          type="button"
          onClick={onEdit}
          className="text-[0.78rem] font-medium text-cobalt-600 hover:underline"
        >
          {t('common.back')}
        </button>
      </div>
      <div className="mt-2 text-[0.88rem] leading-relaxed text-ink-600">{children}</div>
    </div>
  )
}
