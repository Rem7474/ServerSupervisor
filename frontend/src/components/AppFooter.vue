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
              Tous droits réservés.
            </span>
          </p>
        </div>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IconBrandGithub } from '@tabler/icons-vue'

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
    case 'connected':    return 'status-dot-animated bg-green'
    case 'connecting':   return 'status-dot-animated bg-yellow'
    case 'reconnecting': return 'status-dot-animated bg-yellow'
    case 'error':        return 'bg-red'
    case 'disconnected': return 'bg-secondary'
    default:             return 'bg-secondary'
  }
})

const wsStatusLabel = computed(() => {
  switch (props.wsStatus) {
    case 'connected':    return 'Connecté'
    case 'connecting':   return 'Connexion…'
    case 'reconnecting': return 'Reconnexion…'
    case 'error':        return 'Erreur WS'
    case 'disconnected': return 'Déconnecté'
    default:             return ''
  }
})
</script>
