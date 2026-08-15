/** Shared UI primitives — headless-free basics styled with our tokens. */
import { Link, type LinkProps } from '@tanstack/react-router'
import { HeartStraight, Minus, Plus } from '@phosphor-icons/react'

import { cn } from '~/lib/utils'

/* ------------------------------ Button ------------------------------ */

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
type ButtonSize = 'sm' | 'md' | 'lg'

const buttonBase =
  'inline-flex select-none items-center justify-center gap-2 rounded-lg font-medium transition focus-visible:ring-2 focus-visible:ring-cobalt-400/60 disabled:pointer-events-none disabled:opacity-50'

const buttonVariants: Record<ButtonVariant, string> = {
  primary:
    'bg-cobalt-600 text-white shadow-card hover:bg-cobalt-700 hover:shadow-lift active:bg-cobalt-800',
  secondary:
    'border border-cobalt-200 bg-white text-cobalt-700 shadow-card hover:border-cobalt-300 hover:bg-cobalt-50',
  ghost: 'text-cobalt-600 hover:bg-cobalt-50 hover:text-cobalt-700',
  danger: 'border border-[color:var(--color-danger)]/30 bg-white text-[color:var(--color-danger)] hover:bg-[color:var(--color-danger-bg)]',
}

const buttonSizes: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-[0.82rem]',
  md: 'h-10 px-4 text-sm',
  lg: 'h-12 px-6 text-[0.95rem]',
}

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
}

export function Button({ variant = 'primary', size = 'md', loading, className, children, disabled, ...rest }: ButtonProps) {
  return (
    <button
      className={cn(buttonBase, buttonVariants[variant], buttonSizes[size], className)}
      disabled={disabled || loading}
      {...rest}
    >
      {loading && <Spinner className="h-4 w-4" />}
      {children}
    </button>
  )
}

export function ButtonLink({
  to,
  params,
  search,
  variant = 'primary',
  size = 'md',
  className,
  children,
}: {
  /** route id ("/$locale/catalog") or resolved path ("/en-US/catalog") */
  to: LinkProps['to'] | string
  params?: Record<string, string>
  search?: Record<string, unknown>
  variant?: ButtonVariant
  size?: ButtonSize
  className?: string
  children: React.ReactNode
}) {
  return (
    <Link
      to={to as never}
      params={params as never}
      search={search as never}
      className={cn(buttonBase, buttonVariants[variant], buttonSizes[size], className)}
    >
      {children}
    </Link>
  )
}

export function Spinner({ className }: { className?: string }) {
  return (
    <svg className={cn('animate-spin', className)} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity="0.25" strokeWidth="3" />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  )
}

/* ------------------------------ Badges ------------------------------ */

export function Badge({
  tone = 'cobalt',
  className,
  children,
}: {
  tone?: 'cobalt' | 'success' | 'warning' | 'danger' | 'neutral' | 'gold'
  className?: string
  children: React.ReactNode
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[0.72rem] font-semibold tracking-wide',
        tone === 'cobalt' && 'bg-cobalt-50 text-cobalt-700',
        tone === 'success' && 'bg-[color:var(--color-success-bg)] text-[color:var(--color-success)]',
        tone === 'warning' && 'bg-[color:var(--color-warning-bg)] text-[color:var(--color-warning)]',
        tone === 'danger' && 'bg-[color:var(--color-danger-bg)] text-[color:var(--color-danger)]',
        tone === 'neutral' && 'bg-mist text-ink-500',
        tone === 'gold' && 'bg-gold-400/15 text-gold-600',
        className,
      )}
    >
      {children}
    </span>
  )
}

/* --------------------------- Section heading --------------------------- */

export function SectionHeading({
  eyebrow,
  title,
  sub,
  align = 'left',
  action,
}: {
  eyebrow?: string
  title: string
  sub?: string
  align?: 'left' | 'center'
  action?: React.ReactNode
}) {
  return (
    <div
      className={cn(
        'mb-10 flex flex-wrap items-end justify-between gap-4',
        align === 'center' && 'flex-col items-center text-center',
      )}
    >
      <div className={cn(align === 'center' && 'flex flex-col items-center')}>
        {eyebrow && <p className="eyebrow mb-2.5">{eyebrow}</p>}
        <h2 className="text-display-sm text-ink-900">{title}</h2>
        {sub && <p className="mt-3 max-w-2xl text-[0.95rem] leading-relaxed text-ink-500">{sub}</p>}
      </div>
      {action}
    </div>
  )
}

/* ----------------------------- Empty state ----------------------------- */

export function EmptyState({
  icon,
  title,
  body,
  action,
}: {
  icon?: React.ReactNode
  title: string
  body?: string
  action?: React.ReactNode
}) {
  return (
    <div className="card-surface flex flex-col items-center gap-3 px-6 py-16 text-center">
      {icon && <div className="text-cobalt-300">{icon}</div>}
      <h3 className="text-lg font-semibold text-ink-800">{title}</h3>
      {body && <p className="max-w-sm text-sm text-ink-500">{body}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}

/* ---------------------------- Qty stepper ---------------------------- */

export function QuantityStepper({
  value,
  min = 1,
  max = 99,
  onChange,
  size = 'md',
  disabled,
}: {
  value: number
  min?: number
  max?: number
  onChange: (v: number) => void
  size?: 'sm' | 'md'
  disabled?: boolean
}) {
  const btn = cn(
    'flex items-center justify-center text-ink-500 transition hover:text-cobalt-600 disabled:opacity-30',
    size === 'sm' ? 'h-7 w-7' : 'h-9 w-9',
  )
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-lg border border-ink-300/50 bg-white',
        size === 'sm' ? 'h-7' : 'h-9',
      )}
    >
      <button
        type="button"
        className={btn}
        disabled={disabled || value <= min}
        onClick={() => onChange(value - 1)}
        aria-label="decrease"
      >
        <Minus size={size === 'sm' ? 12 : 14} weight="bold" />
      </button>
      <span className={cn('min-w-6 text-center font-medium text-ink-800', size === 'sm' ? 'text-xs' : 'text-sm')}>
        {value}
      </span>
      <button
        type="button"
        className={btn}
        disabled={disabled || value >= max}
        onClick={() => onChange(value + 1)}
        aria-label="increase"
      >
        <Plus size={size === 'sm' ? 12 : 14} weight="bold" />
      </button>
    </div>
  )
}

/* ---------------------------- Wishlist heart ---------------------------- */

export function HeartButton({
  active,
  onClick,
  className,
  label,
}: {
  active: boolean
  onClick: (e: React.MouseEvent) => void
  className?: string
  label: string
}) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        'flex h-9 w-9 items-center justify-center rounded-full border bg-white/95 backdrop-blur transition',
        active
          ? 'border-cobalt-200 text-cobalt-600'
          : 'border-cobalt-100/80 text-ink-400 hover:text-cobalt-600',
        className,
      )}
    >
      <HeartStraight size={17} weight={active ? 'fill' : 'regular'} />
    </button>
  )
}

/* ------------------------------ Breadcrumbs ------------------------------ */

export function Breadcrumbs({
  items,
}: {
  items: Array<{ label: string; to?: LinkProps['to'] | string; params?: Record<string, string> }>
}) {
  return (
    <nav aria-label="breadcrumb" className="mb-6 flex flex-wrap items-center gap-1.5 text-[0.82rem] text-ink-400">
      {items.map((item, i) => (
        <span key={i} className="flex items-center gap-1.5">
          {i > 0 && <span aria-hidden="true" className="text-ink-300">/</span>}
          {item.to ? (
            <Link to={item.to as never} params={item.params as never} className="transition hover:text-cobalt-600">
              {item.label}
            </Link>
          ) : (
            <span className="text-ink-600">{item.label}</span>
          )}
        </span>
      ))}
    </nav>
  )
}

/* ------------------------------ Field error ------------------------------ */

export function FieldError({ children }: { children?: React.ReactNode }) {
  if (!children) return null
  return <p className="mt-1.5 text-[0.8rem] text-[color:var(--color-danger)]">{children}</p>
}
