import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft, CheckCircle, XCircle, Eye, PencilSimple } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { StatusBadge } from '~/components/admin/ContentTable'
import { Badge, Button, FieldError, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { Activity, ContentStatus } from '~/lib/types'

export const Route = createFileRoute('/$locale/admin/content/activities/$id')({
  component: ActivityDetailPage,
})

function ActivityDetailPage() {
  const { id } = Route.useParams()
  const { t, locale } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const { push } = useToast()
  const [item, setItem] = useState<Activity | null | undefined>(undefined)
  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')
  const [summary, setSummary] = useState('')
  const [metaTitle, setMetaTitle] = useState('')
  const [metaDescription, setMetaDescription] = useState('')
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    if (!ready || !token) return
    api
      .adminGetActivity(token, id)
      .then((s) => {
        setItem(s)
        setTitle(s.title ?? '')
        setSlug(s.slug ?? '')
        setSummary(s.summary ?? '')
        setMetaTitle(s.meta_title ?? '')
        setMetaDescription(s.meta_description ?? '')
      })
      .catch(() => setItem(null))
  }, [ready, token, id])

  const canWrite = hasPermission('content.write')
  const canPublish = hasPermission('content.publish')

  const save = async () => {
    if (!token) return
    setSaving(true)
    setErr(null)
    try {
      const updated = await api.adminUpdateActivity(token, Number(id), {
        title,
        slug,
        summary,
        meta_title: metaTitle,
        meta_description: metaDescription,
      })
      setItem(updated)
      push({ title: t('admin.common.saved'), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    } finally {
      setSaving(false)
    }
  }

  const workflow = async (action: 'submit' | 'approve' | 'reject' | 'unpublish') => {
    if (!token) return
    try {
      const fn = {
        submit: api.adminSubmitActivity,
        approve: api.adminApproveActivity,
        reject: api.adminRejectActivity,
        unpublish: api.adminUnpublishActivity,
      }[action]
      const updated = await fn(token, Number(id))
      setItem(updated)
      const msg = {
        submit: 'admin.workflow.submitted',
        approve: 'admin.workflow.approved',
        reject: 'admin.workflow.rejected',
        unpublish: 'admin.workflow.unpublished',
      }[action]
      push({ title: t(msg as Parameters<typeof t>[0]), kind: 'success' })
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  if (item === undefined) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-6 w-6 text-cobalt-400" />
      </div>
    )
  }
  if (item === null) {
    return (
      <div className="py-32 text-center text-[0.88rem] text-ink-400">
        <p>{t('admin.common.empty')}</p>
        <Link to={`/${locale}/admin/content/activities` as never} className="mt-4 link-quiet">
          {t('admin.common.back')}
        </Link>
      </div>
    )
  }

  const status = item.status as ContentStatus

  return (
    <div>
      <Link
        to={`/${locale}/admin/content/activities` as never}
        className="inline-flex items-center gap-1.5 text-[0.84rem] text-ink-500 hover:text-cobalt-700"
      >
        <ArrowLeft size={14} /> {t('admin.content.activities')}
      </Link>

      <div className="mt-4 flex items-center gap-3">
        <h2 className="text-[1.1rem] font-semibold text-ink-900">{item.title}</h2>
        <StatusBadge status={status} />
      </div>

      {err && <FieldError>{err}</FieldError>}

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <div className="card-surface p-6">
          <div className="flex flex-col gap-3">
            <div>
              <label className="label-base">{t('admin.common.title')}</label>
              <input
                className="input-base"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">{t('admin.common.slug')}</label>
              <input
                className="input-base"
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">Summary</label>
              <textarea
                className="input-base min-h-20"
                value={summary}
                onChange={(e) => setSummary(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">Meta title</label>
              <input
                className="input-base"
                value={metaTitle}
                onChange={(e) => setMetaTitle(e.target.value)}
                disabled={!canWrite}
              />
            </div>
            <div>
              <label className="label-base">Meta description</label>
              <textarea
                className="input-base min-h-16"
                value={metaDescription}
                onChange={(e) => setMetaDescription(e.target.value)}
                disabled={!canWrite}
              />
            </div>
          </div>
        </div>

        <div className="card-surface p-6">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-[0.88rem] font-semibold text-ink-700">
              {t('admin.common.status')}
            </h3>
            <Badge tone="neutral">{status}</Badge>
          </div>
          <div className="flex flex-col gap-2.5">
            {canWrite && status === 'draft' && (
              <Button variant="secondary" onClick={() => void workflow('submit')}>
                <CheckCircle size={15} /> {t('admin.workflow.submit')}
              </Button>
            )}
            {canPublish && status === 'in_review' && (
              <>
                <Button variant="secondary" onClick={() => void workflow('approve')}>
                  <CheckCircle size={15} /> {t('admin.workflow.approve')}
                </Button>
                <Button variant="danger" onClick={() => void workflow('reject')}>
                  <XCircle size={15} /> {t('admin.workflow.reject')}
                </Button>
              </>
            )}
            {canPublish && status === 'published' && (
              <Button variant="secondary" onClick={() => void workflow('unpublish')}>
                <Eye size={15} /> {t('admin.workflow.unpublish')}
              </Button>
            )}
            {canWrite && (
              <Button onClick={() => void save()} loading={saving}>
                <PencilSimple size={15} /> {t('admin.common.save')}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
