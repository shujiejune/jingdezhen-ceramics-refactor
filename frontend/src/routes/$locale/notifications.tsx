import { Link, createFileRoute } from '@tanstack/react-router'
import { BellSimple, CheckCircle } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import { Button, EmptyState, Spinner } from '~/components/common/ui'
import { useToast } from '~/components/common/Toaster'
import { api } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import { noindexHead } from '~/lib/seo'
import { formatDate } from '~/lib/utils'
import type { Notification } from '~/lib/types'

/** Notifications inbox — unread badge + mark-read (PRD §3.1.2). */
export const Route = createFileRoute('/$locale/notifications')({
  head: () => noindexHead('Notifications — Jingdezhen Ceramics'),
  component: NotificationsPage,
})

function NotificationsPage() {
  const { t, locale } = useI18n()
  const { ready, token } = useAuth()
  const { push } = useToast()
  const [list, setList] = useState<Notification[] | null>(null)
  const [unread, setUnread] = useState(0)
  const [marking, setMarking] = useState<number | null>(null)
  const [markingAll, setMarkingAll] = useState(false)

  async function refresh(tk: string) {
    const [listRes, countRes] = await Promise.all([
      api.listNotifications(tk),
      api.getUnreadNotificationCount(tk),
    ])
    setList(listRes.data)
    setUnread(countRes.count)
  }

  useEffect(() => {
    if (ready && token) {
      void refresh(token).catch(() => setList([]))
    } else if (ready) {
      setList([])
    }
  }, [ready, token])

  async function handleMarkAll() {
    if (!token || markingAll) return
    setMarkingAll(true)
    try {
      await api.markAllNotificationsRead(token)
      await refresh(token)
      push({ title: t('notif.markedRead'), kind: 'success' })
    } catch {
      push({ title: t('errors.generic'), kind: 'error' })
    } finally {
      setMarkingAll(false)
    }
  }

  async function handleMarkOne(id: number) {
    if (!token || marking) return
    setMarking(id)
    try {
      await api.markNotificationRead(token, id)
      await refresh(token)
    } catch {
      push({ title: t('errors.generic'), kind: 'error' })
    } finally {
      setMarking(null)
    }
  }

  if (!ready || (token && list === null)) {
    return (
      <div className="flex justify-center py-32">
        <Spinner className="h-7 w-7 text-cobalt-400" />
      </div>
    )
  }

  if (!token) {
    return (
      <div className="mx-auto max-w-md px-4 pt-20 pb-12 text-center sm:px-6">
        <h1 className="text-display-sm text-ink-900">{t('notif.title')}</h1>
        <p className="mt-3 text-ink-500">{t('checkout.signInBody')}</p>
        <Link
          to="/$locale/auth/login"
          params={{ locale }}
          search={{ returnTo: `/${locale}/notifications` }}
          className="mt-8 inline-flex h-12 items-center rounded-lg bg-cobalt-600 px-6 text-[0.95rem] font-medium text-white shadow-card hover:bg-cobalt-700"
        >
          {t('nav.login')}
        </Link>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-3xl px-4 pt-10 sm:px-6">
      <p className="eyebrow">{t('nav.account')}</p>
      <div className="mt-2 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-display-sm text-ink-900">{t('notif.title')}</h1>
          {unread > 0 && (
            <p className="mt-2 text-[0.92rem] text-ink-500">
              {t('notif.unread', { count: unread })}
            </p>
          )}
        </div>
        {unread > 0 && (
          <Button
            variant="secondary"
            size="sm"
            loading={markingAll}
            onClick={() => void handleMarkAll()}
          >
            <CheckCircle size={15} weight="duotone" />
            {t('notif.markAllRead')}
          </Button>
        )}
      </div>

      {list && list.length === 0 ? (
        <div className="mt-10">
          <EmptyState icon={<BellSimple size={40} weight="duotone" />} title={t('notif.empty')} />
        </div>
      ) : (
        <div className="mt-8 flex flex-col gap-3">
          {list?.map((n) => (
            <button
              key={n.notification_id}
              type="button"
              onClick={() => !n.is_read && void handleMarkOne(n.notification_id)}
              disabled={n.is_read || marking === n.notification_id}
              className={`card-surface flex items-start gap-3 p-4 text-left transition ${
                n.is_read ? 'opacity-70' : 'hover:shadow-lift'
              }`}
            >
              <div className="mt-0.5 flex shrink-0">
                {n.is_read ? (
                  <CheckCircle size={18} className="text-ink-300" />
                ) : (
                  <span className="mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full bg-cobalt-600" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-[0.92rem] font-medium text-ink-800">{n.message}</p>
                <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[0.78rem] text-ink-400">
                  <span className="font-medium text-cobalt-600">{n.notification_type}</span>
                  <span>{formatDate(n.created_at, locale)}</span>
                </div>
              </div>
              {!n.is_read && marking === n.notification_id && (
                <Spinner className="mt-1 h-4 w-4 shrink-0 text-cobalt-400" />
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
