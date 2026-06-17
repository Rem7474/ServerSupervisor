import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import pluginTypeScript from '@typescript-eslint/eslint-plugin'
import parserTypeScript from '@typescript-eslint/parser'
import vueParser from 'vue-eslint-parser'

export default [
  {
    ignores: ['dist/**', 'node_modules/**', '.git/**', '.eslintrc.cjs', 'src/types/generated.ts'],
  },
  {
    files: ['**/*.{js,mjs,cjs,jsx,ts,tsx,vue}'],
    languageOptions: {
      parser: parserTypeScript,
      parserOptions: {
        ecmaVersion: 2020,
        sourceType: 'module',
        extraFileExtensions: ['.vue'],
      },
      globals: {
        console: 'readonly',
        window: 'readonly',
        document: 'readonly',
        navigator: 'readonly',
        setTimeout: 'readonly',
        clearTimeout: 'readonly',
        setInterval: 'readonly',
        clearInterval: 'readonly',
        fetch: 'readonly',
        localStorage: 'readonly',
        sessionStorage: 'readonly',
        Notification: 'readonly',
        PushManager: 'readonly',
        ServiceWorkerContainer: 'readonly',
        process: 'readonly',
        self: 'readonly',
        caches: 'readonly',
        clients: 'readonly',
        Response: 'readonly',
        URL: 'readonly',
        Blob: 'readonly',
        AbortSignal: 'readonly',
        AbortController: 'readonly',
        WebSocket: 'readonly',
        MessageEvent: 'readonly',
        CloseEvent: 'readonly',
        ResizeObserver: 'readonly',
        HTMLElement: 'readonly',
        PushSubscriptionJSON: 'readonly',
        atob: 'readonly',
        __APP_VERSION__: 'readonly',
      },
    },
    linterOptions: {
      reportUnusedDisableDirectives: true,
    },
  },
  js.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  {
    files: ['**/*.vue'],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        parser: parserTypeScript,
        sourceType: 'module',
      },
    },
  },
  {
    // Apply the TS plugin to .vue files too: their <script setup lang="ts"> is
    // parsed by the TS parser (configured above), so no-explicit-any etc. work
    // without type information. Without this, the bulk of the code (views +
    // components) escaped the any check entirely.
    files: ['**/*.{ts,tsx,vue}'],
    plugins: {
      '@typescript-eslint': pluginTypeScript,
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': ['error', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
      }],
      '@typescript-eslint/strict-boolean-expressions': 'off', // Too strict for Vue
    },
  },
  {
    files: ['**/*.vue'],
    rules: {
      'vue/multi-word-component-names': 'off',
      'vue/no-unused-components': 'warn',
      'vue/no-mutating-props': 'off',
    },
  },
  {
    // TEMP: these large views/components are scheduled for decomposition in the
    // Phase 7 split (see the frontend audit plan). Their display-layer `any`
    // will be eliminated as each is broken into typed sub-components/composables,
    // rather than typing the monolith and re-touching every line during the
    // split. Remove these entries (and the residual any) as each file is split.
    files: [
      '**/views/ProxmoxNodeView.vue',
      '**/views/TrafficView.vue',
      '**/views/AuditLogsView.vue',
      '**/views/GlobalScheduledTasksView.vue',
      '**/views/MonitoringView.vue',
      '**/components/docker/DockerContainersTab.vue',
      // Child of the ProxmoxNodeView monolith; guest/link/network rows carry
      // runtime fields beyond the generated ProxmoxGuest model. Typed when the
      // Proxmox node view is decomposed in Phase 7.
      '**/components/proxmox/ProxmoxNodeGuestsTab.vue',
      '**/components/proxmox/ProxmoxNodeSecurityTab.vue',
    ],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
  {
    rules: {
      'no-unused-vars': 'off', // Handled by TS
      'no-undef': 'off',
      'no-control-regex': 'off',
      'no-console': process.env.NODE_ENV === 'production' ? 'warn' : 'off',
    },
  },
]
