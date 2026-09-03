import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, Plus, Trash } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Button, FieldError, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { AdminItineraryNote, ItineraryRequest, QuoteLineItemInput } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/itineraries/$id')({
  component: ItineraryDetailPage,
})

function ItineraryDetailPage() {
  const { id } = Route.useParams()
  const { t, locale } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [req, setReq] = useState<ItineraryRequest | null | undefined>(undefined)
  const [notes, setNotes] = useState<AdminItineraryNote[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [assigneeId, setAssigneeId] = useState('')
  const [noteText, setNoteText] = useState('')
  const [lineItems, setLineItems] = useState<QuoteLineItemInput[]>([
    { label: '', detail: '', amount_minor: 0 },
  ])
  const [payFull, setPayFull] = useState(false)

  useEffect(() => {
    if (!ready || !token) return
    Promise.all([
      api.adminGetItinerary(token, Number(id)),
      api.adminListItineraryNotes(token, Number(id)),
    ])
      .then(([r, n]) => {
        setReq(r)
        setNotes(n.data)
        setAssigneeId((r as unknown as { assignee_id?: string }).assignee_id ?? '')
      })
      .catch(() => setReq(null))
  }, [ready, token, id])

  const canWrite = hasPermission('itinerary.write')
  const canConfirm = hasPermission('itinerary.confirm')

  const assign = async () => {
    if (!token) return
    setLoading(true)
    setErr(null)
    try {
      const updated = await api.adminAssignItinerary(token, Number(id), {
        assignee_id: assigneeId,
      })
      setReq(updated)
      push({ title: t('admin.itin.assigned'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setLoading(false)
    }
  }

  const addNote = async () => {
    if (!token || !noteText.trim()) return
    setErr(null)
    try {
      const note = await api.adminAddItineraryNote(token, Number(id), {
        body: noteText.trim(),
      })
      setNotes((prev) => [...prev, note])
      setNoteText('')
      push({ title: t('admin.itin.noteAdded'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const sendQuote = async () => {
    if (!token) return
    setLoading(true)
    setErr(null)
    try {
      const quote = await api.adminSendQuote(token, Number(id), {
        line_items: lineItems.filter((li) => li.label.trim()),
        pay_full: payFull,
        currency: 'CNY',
      })
      push({ title: t('admin.itin.quoteSent'), kind: 'success' })
      setReq((prev) => (prev ? { ...prev, quote } : prev))
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setLoading(false)
    }
  }

  const confirm = async () => {
    if (!token) return
    setLoading(true)
    setErr(null)
    try {
      const updated = await api.adminConfirmItinerary(token, Number(id))
      setReq(updated)
      push({ title: t('admin.itin.confirmed'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setLoading(false)
    }
  }

  const refundDeposit = async () => {
    if (!token) return
    setLoading(true)
    setErr(null)
    try {
      const updated = await api.adminRefundDeposit(token, Number(id))
      setReq(updated)
      push({ title: t('admin.itin.depositRefunded'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setLoading(false)
    }
  }

  if (req === undefined) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-6 w-6 text-cobalt-400" />
      </div>
    )
  }
  if (req === null) {
    return (
      <div className="py-32 text-center text-[0.88rem] text-ink-400">
        <p>{t('admin.common.empty')}</p>
        <Link to={`/${locale}/admin/itineraries` as never} className="mt-4 link-quiet">
          {t('admin.common.back')}
        </Link>
      </div>
    )
  }

  return (
    <div>
      <Link
        to={`/${locale}/admin/itineraries` as never}
        className="inline-flex items-center gap-1.5 text-[0.84rem] text-ink-500 hover:text-cobalt-700"
      >
        <ArrowLeft size={14} /> {t('admin.nav.itineraries')}
      </Link>

      <div className="mt-4 flex items-center gap-3">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">#{req.id}</h2>
        <span className="text-[0.84rem] text-ink-500">{req.status}</span>
      </div>

      {err && <FieldError>{err}</FieldError>}

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        {/* Request info */}
        <div className="card-surface p-6">
          <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">Request</h3>
          <dl className="flex flex-col gap-2 text-[0.84rem]">
            <div className="flex justify-between">
              <dt className="text-ink-500">Arrival</dt>
              <dd className="text-ink-700">{new Date(req.arrival_date).toLocaleDateString()}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">Duration</dt>
              <dd className="text-ink-700">{req.duration_days} days</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">Adults</dt>
              <dd className="text-ink-700">{req.adults}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">Children</dt>
              <dd className="text-ink-700">{req.children}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">Pace</dt>
              <dd className="text-ink-700">{req.pace}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">Interests</dt>
              <dd className="text-ink-700">{req.interests.join(', ') || '—'}</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-ink-500">SLA</dt>
              <dd className="text-ink-700">{new Date(req.sla_deadline).toLocaleDateString()}</dd>
            </div>
          </dl>
        </div>

        {/* Assign + actions */}
        <div className="card-surface p-6">
          <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">
            {t('admin.itin.assign')}
          </h3>
          <div className="flex flex-col gap-3">
            <div>
              <label className="label-base">{t('admin.itin.assignee')}</label>
              <input
                className="input-base"
                value={assigneeId}
                onChange={(e) => setAssigneeId(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            {canWrite && (
              <Button variant="secondary" loading={loading} onClick={() => void assign()}>
                {t('admin.itin.assign')}
              </Button>
            )}

            {canConfirm && req.status === 'deposit_paid' && (
              <Button variant="secondary" loading={loading} onClick={() => void confirm()}>
                {t('admin.itin.confirm')}
              </Button>
            )}
            {canConfirm && (req.status === 'deposit_paid' || req.status === 'confirmed') && (
              <Button variant="danger" loading={loading} onClick={() => void refundDeposit()}>
                {t('admin.itin.refundDeposit')}
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Notes thread */}
      <div className="mt-6 card-surface p-6">
        <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">{t('admin.itin.notes')}</h3>
        {notes.length > 0 ? (
          <div className="mb-4 flex flex-col gap-3">
            {notes.map((note) => (
              <div
                key={note.id}
                className="rounded-lg border border-cobalt-50 bg-wash/20 p-3 text-[0.82rem]"
              >
                <div className="flex justify-between text-ink-500">
                  <span>{note.author_email}</span>
                  <span>{new Date(note.created_at).toLocaleString()}</span>
                </div>
                <p className="mt-1 text-ink-700">{note.body}</p>
              </div>
            ))}
          </div>
        ) : (
          <p className="mb-4 text-[0.84rem] text-ink-400">{t('admin.common.empty')}</p>
        )}
        {canWrite && (
          <div className="flex gap-2">
            <input
              className="input-base flex-1"
              value={noteText}
              onChange={(e) => setNoteText(e.target.value)}
              placeholder={t('admin.itin.addNote')}
            />
            <Button variant="secondary" onClick={() => void addNote()}>
              <Plus size={15} /> {t('admin.itin.addNote')}
            </Button>
          </div>
        )}
      </div>

      {/* Quote builder */}
      {canWrite && (
        <div className="mt-6 card-surface p-6">
          <h3 className="mb-4 text-[0.88rem] font-semibold text-ink-700">
            {t('admin.itin.quote')}
          </h3>
          <div className="flex flex-col gap-3">
            {lineItems.map((li, idx) => (
              <div key={idx} className="flex gap-2">
                <input
                  className="input-base flex-1"
                  placeholder="Label"
                  value={li.label}
                  onChange={(e) =>
                    setLineItems((prev) =>
                      prev.map((p, i) => (i === idx ? { ...p, label: e.target.value } : p)),
                    )
                  }
                />
                <input
                  className="input-base w-32"
                  placeholder="Amount (fen)"
                  type="number"
                  value={li.amount_minor}
                  onChange={(e) =>
                    setLineItems((prev) =>
                      prev.map((p, i) =>
                        i === idx ? { ...p, amount_minor: Number(e.target.value) || 0 } : p,
                      ),
                    )
                  }
                />
                <button
                  type="button"
                  onClick={() => setLineItems((prev) => prev.filter((_, i) => i !== idx))}
                  className="text-ink-400 transition hover:text-[color:var(--color-danger)]"
                >
                  <Trash size={14} />
                </button>
              </div>
            ))}
            <div className="flex items-center gap-2">
              <label className="flex items-center gap-2 text-[0.84rem] text-ink-600">
                <input
                  type="checkbox"
                  checked={payFull}
                  onChange={(e) => setPayFull(e.target.checked)}
                />
                Pay full
              </label>
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  setLineItems((prev) => [...prev, { label: '', detail: '', amount_minor: 0 }])
                }
              >
                <Plus size={14} /> Add line
              </Button>
            </div>
            <Button loading={loading} onClick={() => void sendQuote()}>
              {t('admin.itin.quote')}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
