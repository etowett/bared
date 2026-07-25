import js from '@eslint/js'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from '@typescript-eslint/eslint-plugin'
import tsparser from '@typescript-eslint/parser'

export default [
  {
    ignores: ['dist', 'node_modules', 'coverage'],
  },
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsparser,
      ecmaVersion: 2020,
      sourceType: 'module',
      globals: {
        window: 'readonly',
        document: 'readonly',
        console: 'readonly',
        setTimeout: 'readonly',
        clearTimeout: 'readonly',
        fetch: 'readonly',
        btoa: 'readonly',
        sessionStorage: 'readonly',
        localStorage: 'readonly',
        WebSocket: 'readonly',
        confirm: 'readonly',
        alert: 'readonly',
        RequestInit: 'readonly',
        URLSearchParams: 'readonly',
        HTMLElement: 'readonly',
        HTMLDivElement: 'readonly',
        HTMLInputElement: 'readonly',
        HTMLParagraphElement: 'readonly',
        HTMLHeadingElement: 'readonly',
        HTMLButtonElement: 'readonly',
        HTMLTableElement: 'readonly',
        HTMLTableSectionElement: 'readonly',
        HTMLTableRowElement: 'readonly',
        HTMLTableCellElement: 'readonly',
        HTMLTableCaptionElement: 'readonly',
        HTMLTextAreaElement: 'readonly',
        HTMLSelectElement: 'readonly',
        MouseEvent: 'readonly',
        Node: 'readonly',
        React: 'readonly',
        URL: 'readonly',
      },
    },
    plugins: {
      '@typescript-eslint': tseslint,
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...js.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': [
        'warn',
        // `Route` is TanStack Router's file-route export; the router handles its HMR,
        // so it is not expected to be a component. Route modules export their page
        // component alongside it so fast refresh keeps working.
        { allowConstantExport: true, allowExportNames: ['Route'] },
      ],
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      '@typescript-eslint/no-explicit-any': 'warn',
      'no-unused-vars': 'off', // Disable base rule as it's handled by @typescript-eslint
    },
  },
  {
    files: ['vite.config.ts', '*.config.ts'],
    languageOptions: {
      parser: tsparser,
      ecmaVersion: 2020,
      sourceType: 'module',
      globals: {
        __dirname: 'readonly',
        __filename: 'readonly',
        process: 'readonly',
      },
    },
    plugins: {
      '@typescript-eslint': tseslint,
    },
    rules: {
      ...js.configs.recommended.rules,
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      'no-unused-vars': 'off',
    },
  },
]
