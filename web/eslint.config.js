// ESLint flat config —— 修原版零静态检查, 未定义变量/遗留引用直接抛错。
import js from '@eslint/js'
import ts from '@typescript-eslint/eslint-plugin'
import tsParser from '@typescript-eslint/parser'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'

export default [
  { ignores: ['dist'] },
  {
    files: ['**/*.{ts,tsx}'],
    ...js.configs.recommended,
    languageOptions: {
      parser: tsParser,
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: { window: 'readonly', document: 'readonly', console: 'readonly', fetch: 'readonly', EventSource: 'readonly', HTMLDivElement: 'readonly', CanvasRenderingContext2D: 'readonly', setInterval: 'readonly', clearInterval: 'readonly', setTimeout: 'readonly', clearTimeout: 'readonly' },
    },
    plugins: {
      '@typescript-eslint': ts,
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...js.configs.recommended.rules,
      ...ts.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      'no-console': 'off',
    },
  },
]
