import { createFileRoute } from '@tanstack/react-router'

import { PolicyPage, policyHead } from '~/components/seo/PolicyPage'

export const Route = createFileRoute('/$locale/privacy')({
  head: policyHead('privacyPolicy.title', 'privacyPolicy.body', '/privacy'),
  component: () => <PolicyPage titleKey="privacyPolicy.title" bodyKey="privacyPolicy.body" />,
})
