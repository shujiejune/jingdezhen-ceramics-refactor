import '@testing-library/jest-dom/vitest'

import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// explicit cleanup — RTL auto-cleanup needs vitest globals, which we don't enable
afterEach(cleanup)
