import { createFileRoute, redirect } from '@tanstack/react-router'

/**
 * `/` → default locale segment (TDD §6: locale path prefix, default en-US).
 */
export const Route = createFileRoute('/')({
  beforeLoad: () => {
    throw redirect({ to: '/$locale', params: { locale: 'en-US' } })
  },
})
