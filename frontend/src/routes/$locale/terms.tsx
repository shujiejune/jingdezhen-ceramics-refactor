import { createFileRoute } from '@tanstack/react-router'

import { PolicyPage, policyHead } from '~/components/seo/PolicyPage'

export const Route = createFileRoute('/$locale/terms')({
  head: policyHead('termsPolicy.title', 'termsPolicy.body', '/terms'),
  component: () => <PolicyPage titleKey="termsPolicy.title" bodyKey="termsPolicy.body" />,
})
