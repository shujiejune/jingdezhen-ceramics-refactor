/**
 * Auth context — single HS256 access token in localStorage (the CURRENT
 * backend reality per TDD §5.1: 30-day JWT, no refresh rotation yet).
 * The shape (login → optional pending-2FA → verify) is designed so a
 * refresh-rotation flow slots in later without redesign.
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

import { enUS } from '~/i18n/en-US'
import { api, ApiError } from '~/lib/api'
import type { User } from '~/lib/types'

const AUTH_KEY = 'jdz.auth'

interface StoredAuth {
  token: string
  user: User
}

export interface AuthValue {
  ready: boolean
  user: User | null
  token: string | null
  /**
   * OK → { user }. A 2FA-enabled account returns
   * { pending2FA, enrollment? } — the backend answers 401 with
   * {"error":{code:"2fa_required"|"2fa_enrollment_required", pending_token}}.
   */
  login: (
    email: string,
    password: string,
  ) => Promise<{ pending2FA?: string; enrollment?: boolean; user?: User }>
  verify2FA: (pendingToken: string, code: string) => Promise<User>
  signup: (email: string, password: string, nickname: string) => Promise<User>
  logout: () => void
}

const AuthContext = createContext<AuthValue | null>(null)

function read(): StoredAuth | null {
  if (typeof localStorage === 'undefined') return null
  try {
    const raw = localStorage.getItem(AUTH_KEY)
    return raw ? (JSON.parse(raw) as StoredAuth) : null
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [stored, setStored] = useState<StoredAuth | null>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    setStored(read())
    setReady(true)
  }, [])

  const persist = useCallback((s: StoredAuth | null) => {
    setStored(s)
    if (typeof localStorage === 'undefined') return
    if (s) localStorage.setItem(AUTH_KEY, JSON.stringify(s))
    else localStorage.removeItem(AUTH_KEY)
  }, [])

  const login = useCallback<AuthValue['login']>(
    async (email, password) => {
      try {
        const res = await api.login(email, password)
        persist({ token: res.access_token, user: res.user })
        return { user: res.user }
      } catch (e) {
        if (
          e instanceof ApiError &&
          e.is('2fa_required', '2fa_enrollment_required') &&
          e.pendingToken
        ) {
          return { pending2FA: e.pendingToken, enrollment: e.code === '2fa_enrollment_required' }
        }
        throw e
      }
    },
    [persist],
  )

  const verify2FA = useCallback<AuthValue['verify2FA']>(
    async (pendingToken, code) => {
      const res = await api.verify2FA(pendingToken, code)
      persist({ token: res.access_token, user: res.user })
      return res.user
    },
    [persist],
  )

  const signup = useCallback<AuthValue['signup']>(
    async (email, password, nickname) => {
      const res = await api.signup({ email, password, nickname })
      persist({ token: res.access_token, user: res.user })
      return res.user
    },
    [persist],
  )

  const logout = useCallback(() => persist(null), [persist])

  const value = useMemo<AuthValue>(
    () => ({
      ready,
      user: stored?.user ?? null,
      token: stored?.token ?? null,
      login,
      verify2FA,
      signup,
      logout,
    }),
    [ready, stored, login, verify2FA, signup, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}

/** Turn an ApiError into a catalog error key ('errors.*'), falling back
 *  to errors.generic for codes without a translated string. */
export function errorKey(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === 'validation_failed' && e.details) {
      const first = Object.values(e.details)[0]
      if (first?.startsWith('errors.')) return first
    }
    const mapped = `errors.${e.code}`
    return mapped in enUS ? mapped : 'errors.generic'
  }
  return 'errors.network'
}
