import { describe, expect, it } from 'vitest'

import { ApiError, classifyApiError } from '~/lib/api'
import { errorKey } from '~/lib/auth'

/**
 * Contract tests against the REAL wire format (fixtures captured from
 * the live Fiber backend, plus the documented 2FA challenge envelope).
 * The mock transport is aligned to these shapes so mock-mode exercises
 * the same code paths.
 */
describe('classifyApiError — real backend envelopes', () => {
  it('2FA login challenge → structured code + pendingToken', () => {
    const e = classifyApiError(
      401,
      '{"error":{"code":"2fa_required","message":"two-factor authentication code required","pending_token":"pending_u1_101"}}',
    )
    expect(e.code).toBe('2fa_required')
    expect(e.pendingToken).toBe('pending_u1_101')
    expect(e.status).toBe(401)
    expect(e.is('2fa_required', '2fa_enrollment_required')).toBe(true)
  })

  it('live bad login {"message":"Invalid email or password"} → invalid_credentials', () => {
    const e = classifyApiError(401, '{"message":"Invalid email or password"}')
    expect(e.code).toBe('invalid_credentials')
  })

  it('live unknown route → leaked JWT middleware {"message":"Invalid or expired JWT"} → unauthorized', () => {
    const e = classifyApiError(401, '{"message":"Invalid or expired JWT"}')
    expect(e.code).toBe('unauthorized')
  })

  it('live RBAC denial → forbidden', () => {
    const e = classifyApiError(403, '{"message":"Permission denied: order.read"}')
    expect(e.code).toBe('forbidden')
  })

  it('checkout blocks: cart empty / unshippable / overweight', () => {
    expect(classifyApiError(400, '{"message":"cart is empty; cannot check out"}').code).toBe(
      'cart_empty',
    )
    expect(classifyApiError(400, '{"message":"destination country is not shippable"}').code).toBe(
      'unshippable',
    )
    expect(
      classifyApiError(400, '{"message":"order exceeds the maximum shipping weight"}').code,
    ).toBe('overweight')
  })

  it('validator 422 carries the raw details string on details.form', () => {
    const e = classifyApiError(
      422,
      '{"message":"Validation failed: Password must be at least 8 characters","details":"Password must be at least 8 characters"}',
    )
    expect(e.code).toBe('validation_failed')
    expect(e.details?.form).toContain('at least 8')
  })

  it('429 → too_many_attempts; 409 → conflict', () => {
    expect(
      classifyApiError(429, '{"message":"too many failed attempts, try again later"}').code,
    ).toBe('too_many_attempts')
    expect(classifyApiError(409, '{"message":"resource conflict, item already exists"}').code).toBe(
      'conflict',
    )
  })

  it('Fiber plain-text bodies (405 etc.) classify by status', () => {
    const e = classifyApiError(405, 'Method Not Allowed\n')
    expect(e.code).toBe('bad_request')
    expect(e.message).toBe('Method Not Allowed')
  })

  it('JSON {"message":"requested resource not found"} 404 → not_found', () => {
    expect(classifyApiError(404, '{"message":"requested resource not found"}').code).toBe(
      'not_found',
    )
  })

  it('empty body still classifies', () => {
    expect(classifyApiError(500, '').code).toBe('internal')
  })
})

describe('errorKey with classifier codes', () => {
  it('live invalid-credentials message reaches the catalog key', () => {
    expect(errorKey(classifyApiError(401, '{"message":"Invalid email or password"}'))).toBe(
      'errors.invalid_credentials',
    )
  })

  it('unknown codes fall back to errors.generic', () => {
    expect(errorKey(new ApiError('mystery_code', 'x', 418))).toBe('errors.generic')
  })
})
