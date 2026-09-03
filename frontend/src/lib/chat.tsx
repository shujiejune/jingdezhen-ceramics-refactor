/**
 * Chat client (TDD §5.3). The live transport is the /ws WebSocket speaking the
 * frame protocol in types.ts — but the backend chat endpoints are not
 * implemented yet (tracked in frontend/TODO.md "Backend dependencies"), so the
 * widget + agent console run against this in-memory MockChatTransport. The
 * frame shapes are the integration seam: swap the mock for a WebSocket and the
 * UI code doesn't change.
 *
 * Offline fallback (PRD §3.3.1): in live mode (backend chat not yet wired) the
 * widget shows a leave-a-message state instead of a dead input.
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

import { API_MODE } from '~/lib/api'
import { useAuth } from '~/lib/auth'
import type { ChatMessage, ChatSender, ChatServerFrame, ChatSession } from '~/lib/types'

/* --------------------------- mock transport core --------------------------- */
/*
 * Registry persists to localStorage (`jdz.mockChat`, matching the mock
 * transport's persistence convention) so the customer widget and the agent
 * console share sessions across tabs; the `storage` event is the cross-tab
 * "push". Same-tab updates go through the local listener set directly.
 */

const STORE_KEY = 'jdz.mockChat'

type Listener = (frame: ChatServerFrame) => void
const listeners = new Set<Listener>()

function loadSessions(): ChatSession[] {
  if (typeof window === 'undefined') return []
  try {
    return JSON.parse(window.localStorage.getItem(STORE_KEY) ?? '[]') as ChatSession[]
  } catch {
    return []
  }
}

function saveSessions(sessions: ChatSession[]) {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(STORE_KEY, JSON.stringify(sessions))
}

if (typeof window !== 'undefined') {
  // Cross-tab "push": another tab mutated the registry. Re-emit one status
  // frame per session so each consumer refreshes the sessions it cares about.
  window.addEventListener('storage', (e) => {
    if (e.key !== STORE_KEY) return
    for (const s of loadSessions()) {
      emit({ type: 'chat.status', session_id: s.id, status: s.status })
    }
  })
}

function emit(frame: ChatServerFrame) {
  for (const l of listeners) l(frame)
}

function touch(session: ChatSession) {
  session.updated_at = new Date().toISOString()
}

function addMessage(session: ChatSession, sender: ChatSender, body: string): ChatMessage {
  const msg: ChatMessage = {
    id: Date.now() + Math.floor(Math.random() * 1000),
    session_id: session.id,
    sender,
    body,
    created_at: new Date().toISOString(),
  }
  session.messages.push(msg)
  touch(session)
  return msg
}

function commit(sessions: ChatSession[]) {
  saveSessions(sessions)
}

export const mockChat = {
  subscribe(cb: Listener): () => void {
    listeners.add(cb)
    return () => listeners.delete(cb)
  },
  sessions(): ChatSession[] {
    return loadSessions()
  },
  /** Start (or resume) the current user's session; called by the widget. */
  open(locale: string, userEmail?: string): ChatSession {
    const sessions = loadSessions()
    let session = sessions.find((s) => s.user_email === userEmail && s.status !== 'closed')
    if (!session) {
      session = {
        id: Date.now(),
        user_id: undefined,
        user_email: userEmail,
        locale,
        status: 'bot',
        messages: [],
        updated_at: new Date().toISOString(),
      }
      sessions.unshift(session)
      const hello =
        locale === 'zh-CN'
          ? '您好！我是景德镇陶瓷平台的智能助手，很高兴为您服务。请问有什么可以帮您？'
          : 'Hello! I’m the Jingdezhen Ceramics assistant. How can I help you today?'
      addMessage(session, 'bot', hello)
      commit(sessions)
      emit({ type: 'chat.message', sender: 'bot', message: session.messages[0]! })
    }
    return session
  },
  /** Customer sent a message → bot replies (canned), unless an agent owns the session. */
  send(sessionId: number, body: string): ChatMessage | null {
    const sessions = loadSessions()
    const session = sessions.find((s) => s.id === sessionId)
    if (!session) return null
    const msg = addMessage(session, 'user', body)
    if (session.status === 'bot') {
      const reply = addMessage(session, 'bot', botReply(body, session.locale))
      emit({ type: 'chat.message', sender: 'bot', message: reply })
    }
    commit(sessions)
    emit({ type: 'chat.message', sender: 'user', message: msg })
    return msg
  },
  /** Escalate to a human (bot → waiting_agent). */
  requestAgent(sessionId: number): void {
    const sessions = loadSessions()
    const session = sessions.find((s) => s.id === sessionId)
    if (!session || session.status !== 'bot') return
    session.status = 'waiting_agent'
    touch(session)
    commit(sessions)
    emit({ type: 'chat.status', session_id: sessionId, status: 'waiting_agent' })
  },
  /** Agent console: claim a waiting session. */
  claim(sessionId: number): void {
    const sessions = loadSessions()
    const session = sessions.find((s) => s.id === sessionId)
    if (!session || session.status !== 'waiting_agent') return
    session.status = 'with_agent'
    touch(session)
    commit(sessions)
    emit({ type: 'chat.status', session_id: sessionId, status: 'with_agent' })
  },
  /** Agent console reply. */
  agentReply(sessionId: number, body: string): void {
    const sessions = loadSessions()
    const session = sessions.find((s) => s.id === sessionId)
    if (!session) return
    const msg = addMessage(session, 'agent', body)
    commit(sessions)
    emit({ type: 'chat.message', sender: 'agent', message: msg })
  },
  close(sessionId: number): void {
    const sessions = loadSessions()
    const session = sessions.find((s) => s.id === sessionId)
    if (!session) return
    session.status = 'closed'
    touch(session)
    commit(sessions)
    emit({ type: 'chat.status', session_id: sessionId, status: 'closed' })
  },
}

/** Canned bot answers — keyword match, bilingual. Real bot = Qwen adapter server-side. */
function botReply(body: string, locale: string): string {
  const zh = locale === 'zh-CN'
  const b = body.toLowerCase()
  if (/ship|deliver|track|运|邮|物流/.test(b)) {
    return zh
      ? '我们的国际配送按重量分区计费，发货后您可以在订单页输入承运单号跟踪物流。一般 7–14 个工作日送达。'
      : 'International shipping is charged by weight tier per country; once your order ships you can track it from your order page with the carrier number. Typical delivery is 7–14 business days.'
  }
  if (/certificate|authentic|证书|鉴定/.test(b)) {
    return zh
      ? '每件作品都附有数字鉴定证书：扫描作品页面的二维码即可查看真伪与流转记录。'
      : 'Every work ships with a digital certificate of authenticity — scan the QR code on the product page to verify provenance and ownership history.'
  }
  if (/visit|travel|studio|tour|行程|旅行|工坊/.test(b)) {
    return zh
      ? '我们可以为您定制景德镇陶瓷之旅——需要我来安排一位行程规划师与您联系吗？您也可以直接填写“定制行程”表单。'
      : 'We arrange custom ceramics travel in Jingdezhen. Would you like me to connect you with a travel planner? You can also fill in the custom itinerary form directly.'
  }
  if (/price|cost|pay|refund|价|付款|退款/.test(b)) {
    return zh
      ? '网站以美元/欧元/英镑显示价格，结算以人民币完成；支持全额退款。如需人工咨询请点击“转人工客服”。'
      : 'Prices are shown in USD/EUR/GBP and settled in CNY; refunds are full-amount only. For anything else, tap “Human agent”.'
  }
  return zh
    ? '收到！如果需要更具体的帮助，可以点击“转人工客服”，我们的客服团队会尽快与您联系。'
    : 'Got it! If you need more specific help, tap “Human agent” and our team will pick this up shortly.'
}

/* ------------------------------- provider ------------------------------- */

interface ChatValue {
  /** Live backend chat is wired (always false until backend M3 lands). */
  available: boolean
  session: ChatSession | null
  open: boolean
  setOpen: (v: boolean) => void
  unread: number
  /** Mark the visible thread read (resets the bubble badge). */
  markRead: () => void
  send: (body: string) => void
  requestAgent: () => void
}

const ChatContext = createContext<ChatValue | null>(null)

export function ChatProvider({ children }: { children: React.ReactNode }) {
  const { user, token } = useAuth()
  const [session, setSession] = useState<ChatSession | null>(null)
  const [open, setOpen] = useState(false)
  const [unread, setUnread] = useState(0)

  // Mock transport only; in live mode the widget renders the offline fallback.
  const available = API_MODE === 'mock'

  // Start a session once a signed-in user opens chat (PRD: chat is signed-in).
  useEffect(() => {
    if (!available || !token || !user) return
    const s = mockChat.open(user.preferred_locale ?? 'en-US', user.email)
    setSession({ ...s, messages: [...s.messages] })
  }, [available, token, user])

  // Frames from the shared registry (agent console replies, status changes).
  // Any frame for our session refreshes state; new non-user messages while the
  // panel is closed bump the bubble badge (covers cross-tab agent replies).
  const sessionId = session?.id
  useEffect(() => {
    if (sessionId == null) return
    return mockChat.subscribe((frame) => {
      const isMessage = frame.type === 'chat.message' && frame.message.session_id === sessionId
      const isStatus = frame.type === 'chat.status' && frame.session_id === sessionId
      if (!isMessage && !isStatus) return
      const current = mockChat.sessions().find((s) => s.id === sessionId)
      if (!current) return
      setSession((prev) => {
        if (prev && !open) {
          const incoming = current.messages.filter((m) => m.sender !== 'user').length
          const prevCount = prev.messages.filter((m) => m.sender !== 'user').length
          if (incoming > prevCount) setUnread((n) => n + (incoming - prevCount))
        }
        return { ...current, messages: [...current.messages] }
      })
    })
  }, [sessionId, open])

  const send = useCallback(
    (body: string) => {
      if (!session || !available) return
      mockChat.send(session.id, body)
      const current = mockChat.sessions().find((s) => s.id === session.id)
      if (current) setSession({ ...current, messages: [...current.messages] })
    },
    [session, available],
  )

  const requestAgent = useCallback(() => {
    if (!session || !available) return
    mockChat.requestAgent(session.id)
    const current = mockChat.sessions().find((s) => s.id === session.id)
    if (current) setSession({ ...current, messages: [...current.messages] })
  }, [session, available])

  const markRead = useCallback(() => setUnread(0), [])

  const value = useMemo(
    () => ({ available, session, open, setOpen, unread, markRead, send, requestAgent }),
    [available, session, open, unread, markRead, send, requestAgent],
  )

  return <ChatContext.Provider value={value}>{children}</ChatContext.Provider>
}

export function useChat(): ChatValue {
  const ctx = useContext(ChatContext)
  if (!ctx) throw new Error('useChat must be used inside ChatProvider')
  return ctx
}
