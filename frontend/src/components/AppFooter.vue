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
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="icon me-1"
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844a9.59 9.59 0 012.504.337c1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
                </svg>
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
            <li v-if="wsStatus" class="list-inline-item">
              <span
                class="status-dot me-1"
                :class="wsDotClass"
              ></span>
              <span class="text-secondary">{{ wsStatusLabel }}</span>
            </li>
          </ul>
        </div>
        <div class="col-12 col-lg-auto mt-0">
          <p class="mb-0">
            <span class="text-muted">
              Copyright &copy; {{ year }}
              <a href="https://github.com/Rem7474" target="_blank" rel="noopener noreferrer" class="link-secondary">ServerSupervisor</a>.
              Tous droits réservés.
            </span>
          </p>
        </div>
      </div>
    </div>
  </footer>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  /** WebSocket global status: 'connected' | 'connecting' | 'reconnecting' | 'error' | 'disconnected' | null */
  wsStatus: {
    type: String,
    default: null,
  },
})

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
