/**
 * Chat agent console (PRD §3.3.1 human handoff; TDD §5.3 frames).
 * Lists mock sessions, lets an agent (permission `chat.handle`) claim a
 * waiting session and reply. When the backend chat endpoints land, this page
 * swaps the mock registry for the live session list + WS frames.
 */
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { Badge, Button, FieldError } from '~/components/common/ui'
import { mockChat } from '~/lib/chat'
import { errorKey, useAuth } from '~/lib/auth'
import { useI18n } from '~/lib/i18n'
import type { ChatSession } from '~/lib/types'
import { cn } from '~/lib/utils'

export const Route = createFileRoute('/$locale/admin/chat')({
  component: ChatConsolePage,
})

const statusTone: Record<ChatSession['status'], 'neutral' | 'warning' | 'success' | 'danger'> = {
  bot: 'neutral',
  waiting_agent: 'warning',
  with_agent: 'success',
  closed: 'danger',
}

function ChatConsolePage() {
  const { t } = useI18n()
  const { ready, token, hasPermission } = useAuth()
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [activeId, setActiveId] = useState<number | null>(null)
  const [reply, setReply] = useState('')
  const [err, setErr] = useState<string | null>(null)

  const canHandle = hasPermission('chat.handle')

  useEffect(() => {
    if (!ready || !token) return
    const sync = () => {
      const all = mockChat.sessions().map((s) => ({ ...s, messages: [...s.messages] }))
      setSessions(all)
    }
    sync()
    return mockChat.subscribe(sync)
  }, [ready, token])

  const active = sessions.find((s) => s.id === activeId) ?? null

  const claim = (id: number) => {
    try {
      mockChat.claim(id)
      setActiveId(id)
    } catch (e) {
      setErr(t(errorKey(e) as Parameters<typeof t>[0]))
    }
  }

  const sendReply = () => {
    const body = reply.trim()
    if (!active || !body) return
    if (active.status === 'waiting_agent') mockChat.claim(active.id)
    mockChat.agentReply(active.id, body)
    setReply('')
  }

  const close = () => {
    if (!active) return
    mockChat.close(active.id)
  }

  if (!ready || !token) return null

  return (
    <div>
      <h2 className="text-[1.1rem] font-semibold text-ink-900">{t('admin.chat.title')}</h2>
      <p className="mt-1 text-[0.78rem] text-ink-400">{t('admin.chat.mockNote')}</p>

      {err && <FieldError>{err}</FieldError>}

      <div className="mt-6 grid gap-4 lg:grid-cols-[18rem_1fr]">
        {/* session list */}
        <div className="card-surface overflow-hidden p-0">
          <div className="border-b border-cobalt-50 px-4 py-2.5 text-[0.8rem] font-semibold text-ink-600">
            {t('admin.chat.sessions')} ({sessions.length})
          </div>
          {sessions.length === 0 ? (
            <p className="px-4 py-8 text-center text-[0.82rem] text-ink-400">
              {t('admin.common.empty')}
            </p>
          ) : (
            <ul className="max-h-[28rem] overflow-y-auto">
              {sessions.map((s) => (
                <li key={s.id}>
                  <button
                    type="button"
                    onClick={() => setActiveId(s.id)}
                    className={cn(
                      'flex w-full flex-col gap-1 border-b border-cobalt-50/60 px-4 py-2.5 text-left transition hover:bg-wash/40',
                      activeId === s.id && 'bg-cobalt-50/60',
                    )}
                  >
                    <span className="flex items-center justify-between">
                      <span className="text-[0.82rem] font-medium text-ink-700">
                        {s.user_email ?? t('admin.chat.guest')}
                      </span>
                      <Badge tone={statusTone[s.status]}>
                        {t(`admin.chat.status.${s.status}` as Parameters<typeof t>[0])}
                      </Badge>
                    </span>
                    <span className="truncate text-[0.74rem] text-ink-400">
                      {s.messages.at(-1)?.body ?? '—'}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* active thread */}
        <div className="card-surface flex min-h-[24rem] flex-col p-0">
          {!active ? (
            <div className="flex flex-1 items-center justify-center text-[0.84rem] text-ink-400">
              {t('admin.chat.pickSession')}
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between border-b border-cobalt-50 px-4 py-2.5">
                <span className="text-[0.84rem] font-semibold text-ink-700">
                  #{active.id} · {active.user_email ?? t('admin.chat.guest')} · {active.locale}
                </span>
                {canHandle && active.status !== 'closed' && (
                  <Button variant="danger" size="sm" onClick={close}>
                    {t('admin.chat.closeSession')}
                  </Button>
                )}
              </div>

              <div className="flex-1 overflow-y-auto px-4 py-3">
                <div className="flex flex-col gap-2">
                  {active.messages.map((m) => (
                    <div
                      key={m.id}
                      className={cn(
                        'max-w-[80%] rounded-xl px-3 py-2 text-[0.82rem]',
                        m.sender === 'agent'
                          ? 'self-end rounded-br-sm bg-cobalt-600 text-white'
                          : 'self-start rounded-bl-sm bg-wash text-ink-700',
                      )}
                    >
                      {m.sender === 'bot' && (
                        <span className="mb-0.5 block text-[0.64rem] font-semibold text-ink-400">
                          {t('admin.chat.botName')}
                        </span>
                      )}
                      {m.body}
                    </div>
                  ))}
                </div>
              </div>

              {canHandle && active.status !== 'closed' ? (
                active.status === 'waiting_agent' ? (
                  <div className="border-t border-cobalt-50 p-3">
                    <Button variant="secondary" onClick={() => claim(active.id)}>
                      {t('admin.chat.claim')}
                    </Button>
                  </div>
                ) : active.status === 'with_agent' ? (
                  <div className="flex gap-2 border-t border-cobalt-50 p-3">
                    <input
                      className="input-base flex-1 text-[0.82rem]"
                      value={reply}
                      placeholder={t('admin.chat.replyPlaceholder')}
                      onChange={(e) => setReply(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') sendReply()
                      }}
                    />
                    <Button onClick={sendReply} disabled={!reply.trim()}>
                      {t('admin.chat.reply')}
                    </Button>
                  </div>
                ) : (
                  <div className="border-t border-cobalt-50 p-3 text-center text-[0.78rem] text-ink-400">
                    {t('admin.chat.botHandling')}
                  </div>
                )
              ) : null}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
