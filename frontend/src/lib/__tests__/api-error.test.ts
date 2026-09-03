import { describe, expect, it } from 'vitest'

import { ApiError } from '~/lib/api'
import { errorKey } from '~/lib/auth'

describe('ApiError', () => {
  it('carries code, status and details', () => {
    const e = new ApiError('overweight', 'too heavy', 422, { cart: 'errors.required' })
    expect(e.code).toBe('overweight')
    expect(e.status).toBe(422)
    expect(e.details).toEqual({ cart: 'errors.required' })
    expect(e.is('overweight', 'unshippable')).toBe(true)
    expect(e.is('not_found')).toBe(false)
  })

  it('is a plain Error for catch-blocks', () => {
    expect(new ApiError('x', 'boom')).toBeInstanceOf(Error)
    expect(new ApiError('x', 'boom').message).toBe('boom')
  })
})

describe('errorKey — domain code → catalog key', () => {
  it('maps known codes', () => {
    expect(errorKey(new ApiError('invalid_credentials', 'x', 401))).toBe(
      'errors.invalid_credentials',
    )
    expect(errorKey(new ApiError('overweight', 'x', 422))).toBe('errors.overweight')
    // 2FA challenges are intercepted by code before errorKey; no catalog key exists
    expect(errorKey(new ApiError('2fa_required', 'x', 401))).toBe('errors.generic')
  })

  it('surfaces validation field errors when present', () => {
    const e = new ApiError('validation_failed', 'x', 422, { password: 'errors.passwordShort' })
    expect(errorKey(e)).toBe('errors.passwordShort')
  })

  it('falls back to network for non-ApiError', () => {
    expect(errorKey(new Error('boom'))).toBe('errors.network')
  })
})
