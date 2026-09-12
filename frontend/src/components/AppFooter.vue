<template>
  <footer class="footer footer-transparent d-print-none">
    <div class="container-xl">
      <div class="row text-center align-items-center flex-row-reverse g-2">
        <div class="col-lg-auto ms-lg-auto">
          <ul class="list-inline list-inline-dots mb-0">
            <li class="list-inline-item">
              <a
                href="https://github.com/Rem7474/ServerSupervisor"
                target="_blank"
                rel="noopener noreferrer"
                class="link-secondary"
              >
                <IconBrandGithub
                  :size="16"
                  class="icon me-1"
                />
                GitHub
              </a>
            </li>
            <li class="list-inline-item">
              <a
                href="https://github.com/Rem7474/ServerSupervisor/releases"
                target="_blank"
                rel="noopener noreferrer"
                class="link-secondary"
              >
                v{{ appVersion }}
              </a>
            </li>
            <li
              v-if="wsStatus"
              class="list-inline-item"
            >
              <span
                class="status-dot me-1"
                :class="wsDotClass"
              />
              <span class="text-secondary">{{ wsStatusLabel }}</span>
            </li>
          </ul>
        </div>
        <div class="col-12 col-lg-auto mt-0">
          <p class="mb-0">
            <span class="text-muted">
              Copyright &copy; {{ year }}
              <a
                href="https://github.com/Rem7474"
                target="_blank"
                rel="noopener noreferrer"
                class="link-secondary"
              >ServerSupervisor</a>.
              {{ t('common.footerAllRightsReserved') }}
            </span>
          </p>
        </div>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { IconBrandGithub } from '@tabler/icons-vue'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  wsStatus?: string | null
}>(), {
  wsStatus: null,
})

declare const __APP_VERSION__: string
const appVersion = __APP_VERSION__
const year = new Date().getFullYear()

const wsDotClass = computed(() => {
  switch (props.wsStatus) {
    case 'connected':    return 'status-dot-animated bg-success'
    case 'connecting':   return 'status-dot-animated bg-warning'
    case 'reconnecting': return 'status-dot-animated bg-warning'
    case 'error':        return 'bg-danger'
    case 'disconnected': return 'bg-secondary'
    default:             return 'bg-secondary'
  }
})

const wsStatusLabel = computed(() => {
  switch (props.wsStatus) {
    case 'connected':    return t('common.footerWsConnectedLabel')
    case 'connecting':   return t('common.footerWsConnectingLabel')
    case 'reconnecting': return t('common.footerWsReconnectingLabel')
    case 'error':        return t('common.footerWsErrorLabel')
    case 'disconnected': return t('common.footerWsDisconnectedLabel')
    default:             return ''
  }
})
</script>
