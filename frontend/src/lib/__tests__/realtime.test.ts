import { describe, expect, it } from 'vitest'

import { backoffDelay, parseNotification, wsUrl } from '~/lib/realtime'

describe('backoffDelay', () => {
  it('doubles from 1s', () => {
    expect(backoffDelay(0)).toBe(1000)
    expect(backoffDelay(1)).toBe(2000)
    expect(backoffDelay(2)).toBe(4000)
  })

  it('caps at 30s', () => {
    expect(backoffDelay(5)).toBe(30_000)
    expect(backoffDelay(20)).toBe(30_000)
  })

  it('treats negative attempts as the first', () => {
    expect(backoffDelay(-3)).toBe(1000)
  })
})

describe('wsUrl', () => {
  it('swaps http(s) for ws(s) and carries the token', () => {
    expect(wsUrl('http://localhost:3000', 'abc.def')).toBe('ws://localhost:3000/ws?token=abc.def')
    expect(wsUrl('https://jdz.example', 'a b')).toBe('wss://jdz.example/ws?token=a%20b')
  })
})

describe('parseNotification', () => {
  it('parses a models.Notification push', () => {
    const n = parseNotification(
      JSON.stringify({
        notification_id: 7,
        recipient_user_id: 'u1',
        notification_type: 'order_shipped',
        message: 'Your order has shipped',
        is_read: false,
        created_at: '2026-09-03T00:00:00Z',
      }),
    )
    expect(n?.notification_id).toBe(7)
    expect(n?.message).toBe('Your order has shipped')
  })

  it('rejects non-notification frames and bad JSON', () => {
    expect(parseNotification('{"type":"chat.message"}')).toBeNull()
    expect(parseNotification('not json')).toBeNull()
    expect(parseNotification('42')).toBeNull()
  })
})
