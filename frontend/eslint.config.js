// Flat ESLint config — type-aware-free recommended set (fast, CI-friendly).
// Route files legitimately mix exports (Route + component), so
// react-refresh/only-export-components stays off.
import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'

export default tseslint.config(
  { ignores: ['dist', '.output', '.tanstack', '.nitro', 'node_modules', 'src/routeTree.gen.ts'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': 'off',
      // fetch-on-mount + localStorage hydration are deliberate here (the
      // canonical external-system sync pattern); TanStack Query replaces
      // them in M-F1 — revisit then.
      'react-hooks/set-state-in-effect': 'off',
      // useLoopScroller returns ref objects by design; assigning them via
      // the ref prop is flagged by the compiler-oriented refs rule.
      'react-hooks/refs': 'off',
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
    },
  },
)
