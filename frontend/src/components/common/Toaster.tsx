/** Tiny hand-rolled toast layer (toast + inline + field = the 3 error layers, TDD §6). */
import { createContext, useCallback, useContext, useMemo, useState } from 'react'

import { cn } from '~/lib/utils'

export interface Toast {
  id: number
  title: string
  kind?: 'success' | 'error' | 'info'
}

interface ToastValue {
  push: (t: Omit<Toast, 'id'>) => void
}

const ToastContext = createContext<ToastValue | null>(null)

let seq = 1

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const push = useCallback((t: Omit<Toast, 'id'>) => {
    const id = seq++
    setToasts((prev) => [...prev, { ...t, id }])
    setTimeout(() => setToasts((prev) => prev.filter((x) => x.id !== id)), 3600)
  }, [])

  const value = useMemo(() => ({ push }), [push])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed right-4 bottom-4 z-[80] flex w-[22rem] flex-col gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            role="status"
            className={cn(
              'pointer-events-auto rounded-lg border px-4 py-3 text-sm shadow-lift backdrop-blur',
              t.kind === 'error'
                ? 'border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-bg)] text-[color:var(--color-danger)]'
                : t.kind === 'info'
                  ? 'border-cobalt-200 bg-white/95 text-ink-700'
                  : 'border-[color:var(--color-success)]/25 bg-[color:var(--color-success-bg)] text-[color:var(--color-success)]',
            )}
          >
            {t.title}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside ToastProvider')
  return ctx
}
