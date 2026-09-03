import { createFileRoute } from '@tanstack/react-router'

import { PolicyPage, policyHead } from '~/components/seo/PolicyPage'

export const Route = createFileRoute('/$locale/cookies')({
  head: policyHead('cookiesPolicy.title', 'cookiesPolicy.body', '/cookies'),
  component: () => <PolicyPage titleKey="cookiesPolicy.title" bodyKey="cookiesPolicy.body" />,
})
