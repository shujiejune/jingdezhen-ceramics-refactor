/**
 * Site-wide floating chat widget (PRD §3.3.1 — smart support).
 * Bot → escalate → agent console; offline fallback = leave-a-message.
 */
import { ChatCircleDots, UserSwitch, X } from '@phosphor-icons/react'
import { useEffect, useRef, useState } from 'react'

import { Button } from '~/components/common/ui'
import { useChat } from '~/lib/chat'
import { useI18n } from '~/lib/i18n'

export function ChatWidget() {
  const { t } = useI18n()
  const { available, session, open, setOpen, unread, markRead, send, requestAgent } = useChat()
  const [text, setText] = useState('')
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (open) markRead()
  }, [open, markRead])

  useEffect(() => {
    if (open && listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight
  }, [open, session?.messages.length])

  if (!session && !available) return null

  const onSend = () => {
    const body = text.trim()
    if (!body) return
    send(body)
    setText('')
  }

  return (
    <>
      {/* bubble */}
      <button
        type="button"
        aria-label={t('chat.open')}
        onClick={() => setOpen(!open)}
        className="fixed right-4 bottom-4 z-[70] flex h-12 w-12 items-center justify-center rounded-full bg-cobalt-600 text-white shadow-lift transition hover:bg-cobalt-700"
      >
        {open ? <X size={20} /> : <ChatCircleDots size={22} />}
        {!open && unread > 0 && (
          <span className="absolute -top-1 -right-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-[color:var(--color-danger)] px-1 text-[0.68rem] font-semibold">
            {unread}
          </span>
        )}
      </button>

      {/* panel */}
      {open && (
        <div
          role="dialog"
          aria-label={t('chat.title')}
          className="fixed right-4 bottom-20 z-[70] flex h-[26rem] w-[21rem] flex-col overflow-hidden rounded-xl border border-cobalt-100 bg-white shadow-lift"
        >
          <div className="flex items-center justify-between border-b border-cobalt-50 bg-cobalt-600 px-4 py-2.5 text-white">
            <span className="flex items-center gap-2 text-[0.86rem] font-semibold">
              <ChatCircleDots size={16} /> {t('chat.title')}
            </span>
            <span className="text-[0.72rem] opacity-80">
              {session?.status === 'bot' && t('chat.statusBot')}
              {session?.status === 'waiting_agent' && t('chat.statusWaiting')}
              {session?.status === 'with_agent' && t('chat.statusAgent')}
              {session?.status === 'closed' && t('chat.statusClosed')}
            </span>
          </div>

          {!available ? (
            /* live mode before backend M3: leave-a-message fallback (PRD) */
            <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center">
              <p className="text-[0.84rem] text-ink-600">{t('chat.offline')}</p>
              <a
                href="mailto:support@jingdezhen.example"
                className="rounded-lg border border-cobalt-200 px-3 py-1.5 text-[0.8rem] text-cobalt-700 transition hover:border-cobalt-400"
              >
                {t('chat.leaveMessage')}
              </a>
            </div>
          ) : !session ? null : session.messages.length === 0 ? (
            <div className="flex flex-1 items-center justify-center text-[0.84rem] text-ink-400">
              {t('chat.empty')}
            </div>
          ) : (
            <div ref={listRef} className="flex-1 overflow-y-auto px-3 py-3">
              <div className="flex flex-col gap-2">
                {session.messages.map((m) => (
                  <div
                    key={m.id}
                    className={
                      m.sender === 'user'
                        ? 'self-end rounded-xl rounded-br-sm bg-cobalt-600 px-3 py-2 text-[0.82rem] text-white'
                        : 'self-start rounded-xl rounded-bl-sm bg-wash px-3 py-2 text-[0.82rem] text-ink-700'
                    }
                  >
                    {m.sender === 'agent' && (
                      <span className="mb-0.5 block text-[0.66rem] font-semibold text-cobalt-600">
                        {t('chat.agentName')}
                      </span>
                    )}
                    {m.body}
                  </div>
                ))}
              </div>
            </div>
          )}

          {available && session && session.status !== 'closed' && (
            <div className="border-t border-cobalt-50 p-2.5">
              {session.status === 'bot' && (
                <button
                  type="button"
                  onClick={requestAgent}
                  className="mb-2 flex w-full items-center justify-center gap-1.5 rounded-lg border border-dashed border-cobalt-200 py-1.5 text-[0.76rem] text-cobalt-600 transition hover:border-cobalt-400"
                >
                  <UserSwitch size={13} /> {t('chat.requestAgent')}
                </button>
              )}
              <div className="flex gap-2">
                <label htmlFor="chat-input" className="sr-only">
                  {t('chat.placeholder')}
                </label>
                <input
                  id="chat-input"
                  className="input-base flex-1 text-[0.82rem]"
                  value={text}
                  placeholder={t('chat.placeholder')}
                  onChange={(e) => setText(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') onSend()
                  }}
                />
                <Button size="sm" onClick={onSend} disabled={!text.trim()}>
                  {t('chat.send')}
                </Button>
              </div>
            </div>
          )}
        </div>
      )}
    </>
  )
}
