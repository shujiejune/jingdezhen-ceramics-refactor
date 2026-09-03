/**
 * Auth context — single HS256 access token in localStorage (the CURRENT
 * backend reality per TDD §5.1: 30-day JWT, no refresh rotation yet).
 * The shape (login → optional pending-2FA → verify) is designed so a
 * refresh-rotation flow slots in later without redesign.
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

import { enUS } from '~/i18n/en-US'
import { api, ApiError } from '~/lib/api'
import type { Permission, StaffRole, User } from '~/lib/types'

const AUTH_KEY = 'jdz.auth'

/** Role → permissions map (mirrors backend `role_permissions` seed). */
const ROLE_PERMISSIONS: Record<StaffRole, Permission[]> = {
  super_admin: [
    'users.manage',
    'content.write',
    'content.publish',
    'product.read',
    'product.write',
    'product.publish',
    'certificate.manage',
    'order.read',
    'order.write',
    'order.refund',
    'itinerary.read',
    'itinerary.write',
    'itinerary.confirm',
    'chat.handle',
    'dashboard.view',
    'settings.manage',
  ],
  content_editor: ['content.write', 'product.read'],
  travel_planner: [
    'itinerary.read',
    'itinerary.write',
    'itinerary.confirm',
    'chat.handle',
    'order.read',
  ],
  ecommerce_operator: [
    'product.read',
    'product.write',
    'certificate.manage',
    'order.read',
    'order.write',
    'order.refund',
    'dashboard.view',
  ],
  customer_service: ['order.read', 'itinerary.read', 'chat.handle', 'dashboard.view'],
}

const STAFF_ROLES: StaffRole[] = [
  'super_admin',
  'content_editor',
  'travel_planner',
  'ecommerce_operator',
  'customer_service',
]

function userRoles(user: User | null): StaffRole[] {
  if (!user) return []
  if (user.roles && user.roles.length > 0) return user.roles
  if (user.role !== 'customer' && STAFF_ROLES.includes(user.role as StaffRole)) {
    return [user.role as StaffRole]
  }
  return []
}

function hasPermission(user: User | null, perm: Permission): boolean {
  const roles = userRoles(user)
  return roles.some((r) => ROLE_PERMISSIONS[r].includes(perm))
}

function isStaff(user: User | null): boolean {
  return userRoles(user).length > 0
}

interface StoredAuth {
  token: string
  user: User
}

export interface AuthValue {
  ready: boolean
  user: User | null
  token: string | null
  /** true if the user holds any staff role */
  isStaff: boolean
  /** check a specific permission (derived from role→permissions map) */
  hasPermission: (perm: Permission) => boolean
  /** check if the user holds a specific staff role */
  hasRole: (role: StaffRole) => boolean
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
  /**
   * Real contract: signup answers 201 with an EMPTY access_token — the
   * account needs email activation first. needsActivation reflects that;
   * activation_token is a mock-mode affordance for the dev shortcut UI.
   */
  signup: (
    email: string,
    password: string,
    nickname: string,
  ) => Promise<{ user: User; needsActivation: boolean; activationToken?: string }>
  logout: () => void
}

/** Persist a session from outside the provider (OAuth receiver routes). */
export function persistSession(token: string, user: User) {
  const payload: StoredAuth = { token, user }
  try {
    localStorage.setItem(AUTH_KEY, JSON.stringify(payload))
  } catch {
    /* storage blocked — session stays in-memory for this tab */
  }
  // other tabs re-read on their next 'storage' event; this tab's
  // provider picks it up on next mount
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
      if (!res.access_token) {
        return { user: res.user, needsActivation: true, activationToken: res.activation_token }
      }
      persist({ token: res.access_token, user: res.user })
      return { user: res.user, needsActivation: false }
    },
    [persist],
  )

  const logout = useCallback(() => persist(null), [persist])

  const value = useMemo<AuthValue>(
    () => ({
      ready,
      user: stored?.user ?? null,
      token: stored?.token ?? null,
      isStaff: isStaff(stored?.user ?? null),
      hasPermission: (perm: Permission) => hasPermission(stored?.user ?? null, perm),
      hasRole: (role: StaffRole) => userRoles(stored?.user ?? null).includes(role),
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
