<template>
  <!-- Side panel -->
  <div v-show="show" :class="wrapperClass">
    <div class="card d-flex flex-column h-100">
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title">
          <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler me-1" width="24" height="24"
            viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"
            stroke-linecap="round" stroke-linejoin="round">
            <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
            <path d="M8 9l3 3l-3 3" />
            <path d="M13 15l3 0" />
            <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
          </svg>
          {{ title }}
        </h3>
        <div class="d-flex gap-1">
          <!-- Copy -->
          <button
            class="btn btn-sm btn-ghost-secondary"
            :title="copied ? 'Copié !' : 'Copier la sortie'"
            :disabled="!command"
            @click="copy"
          >
            <svg v-if="!copied" xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18"
              viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"
              stroke-linecap="round" stroke-linejoin="round">
              <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
              <path d="M8 8m0 2a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-8a2 2 0 0 1 -2 -2z" />
              <path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2" />
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="icon text-success" width="18" height="18"
              viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"
              stroke-linecap="round" stroke-linejoin="round">
              <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
              <path d="M5 12l5 5l10 -10" />
            </svg>
          </button>
          <!-- Download -->
          <button
            class="btn btn-sm btn-ghost-secondary"
            title="Télécharger (.txt)"
            :disabled="!command"
            @click="download"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="18" height="18"
              viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"
              stroke-linecap="round" stroke-linejoin="round">
              <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
              <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2" />
              <path d="M7 11l5 5l5 -5" />
              <path d="M12 4l0 12" />
            </svg>
          </button>
          <!-- Close -->
          <button
            class="btn btn-sm btn-ghost-secondary"
            title="Fermer"
            @click="$emit('close')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="icon" width="24" height="24"
              viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"
              stroke-linecap="round" stroke-linejoin="round">
              <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
              <path d="M18 6l-12 12" />
              <path d="M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div class="card-body d-flex flex-column flex-fill p-0" style="min-height: 0;">
        <!-- Empty state -->
        <div
          v-if="!command"
          class="d-flex align-items-center justify-content-center flex-fill text-secondary"
          style="background: #1e293b; border-radius: 0 0 0.5rem 0.5rem;"
        >
          <div class="text-center p-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler mb-2 opacity-50" width="48" height="48"
              viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" fill="none"
              stroke-linecap="round" stroke-linejoin="round">
              <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
              <path d="M8 9l3 3l-3 3" />
              <path d="M13 15l3 0" />
              <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
            </svg>
            <div class="opacity-75">{{ emptyText }}</div>
            <div class="small mt-1 opacity-50">Cliquez sur "Logs" pour afficher la sortie d'une commande</div>
          </div>
        </div>

        <!-- Active viewer -->
        <div v-else class="d-flex flex-column h-100">
          <div class="px-3 pt-3 pb-2" style="background: #1e293b; border-bottom: 1px solid rgba(255,255,255,0.1);">
            <div class="d-flex align-items-start justify-content-between mb-2">
              <div class="flex-fill" style="min-width: 0;">
                <div class="fw-semibold text-light" style="font-size: 0.95rem;">
                  {{ command.host_name || command.host_id }}
                </div>
                <div class="d-flex align-items-center gap-2 mt-1 flex-wrap">
                  <span :class="moduleClass(command.module)">{{ moduleLabel(command.module) }}</span>
                  <code style="background: rgba(0,0,0,0.3); padding: 0.15rem 0.4rem; border-radius: 0.25rem; color: #94a3b8;">
                    {{ cmdLabel(command) }}
                  </code>
                </div>
              </div>
              <span :class="statusClass(command.status)" class="ms-2">{{ command.status }}</span>
            </div>
          </div>
          <pre
            ref="outputEl"
            class="mb-0 flex-fill"
            style="
              background: #0f172a;
              color: #e2e8f0;
              padding: 1rem;
              margin: 0;
              overflow-y: auto;
              font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
              font-size: 0.813rem;
              line-height: 1.5;
              border-radius: 0 0 0.5rem 0.5rem;
            "
            v-html="colorizedOutput || '<span style=\'opacity:0.5\'>Aucune sortie disponible.</span>'"
          ></pre>
        </div>
      </div>
    </div>
  </div>

  <!-- Floating reopen button -->
  <button
    v-show="!show"
    class="btn btn-primary"
    style="position: fixed; bottom: 1.5rem; right: 1.5rem; z-index: 100;"
    @click="$emit('open')"
  >
    <svg xmlns="http://www.w3.org/2000/svg" class="icon me-1" width="24" height="24"
      viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none"
      stroke-linecap="round" stroke-linejoin="round">
      <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
      <path d="M8 9l3 3l-3 3" />
      <path d="M13 15l3 0" />
      <path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z" />
    </svg>
    {{ title }}
  </button>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { colorizeConsoleOutput, copyConsoleOutput, downloadConsoleOutput } from '../utils/consoleOutput'
import { useStatusBadge } from '../composables/useStatusBadge'

const props = defineProps({
  /** The command record: { host_name?, host_id?, module, action, target?, status, output? } */
  command: {
    type: Object,
    default: null,
  },
  show: {
    type: Boolean,
    default: false,
  },
  title: {
    type: String,
    default: 'Logs',
  },
  emptyText: {
    type: String,
    default: 'Aucun log sélectionné',
  },
  wrapperClass: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['close', 'open'])

const { getStatusBadgeClass } = useStatusBadge()

const outputEl = ref(null)
const copied = ref(false)

const MODULE_META = {
  apt:       { label: 'APT',       cls: 'badge bg-azure-lt text-azure' },
  docker:    { label: 'Docker',    cls: 'badge bg-blue-lt text-blue' },
  systemd:   { label: 'Systemd',   cls: 'badge bg-green-lt text-green' },
  journal:   { label: 'Journal',   cls: 'badge bg-purple-lt text-purple' },
  processes: { label: 'Processus', cls: 'badge bg-orange-lt text-orange' },
  custom:    { label: 'Custom',    cls: 'badge bg-teal-lt text-teal' },
}

function moduleLabel(module) {
  return MODULE_META[module]?.label ?? module
}

function moduleClass(module) {
  return MODULE_META[module]?.cls ?? 'badge bg-secondary-lt text-secondary'
}

function cmdLabel(cmd) {
  return [cmd.action, cmd.target].filter(Boolean).join(' ')
}

function statusClass(status) {
  return getStatusBadgeClass(status, 'badge bg-yellow-lt text-yellow')
}

const colorizedOutput = computed(() => colorizeConsoleOutput(props.command?.output || ''))

// Scroll to bottom whenever output changes
watch(colorizedOutput, () => {
  nextTick(() => {
    if (outputEl.value) outputEl.value.scrollTop = outputEl.value.scrollHeight
  })
})

async function copy() {
  await copyConsoleOutput(props.command?.output || '')
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

function download() {
  const name = [props.command?.module, props.command?.action, props.command?.target]
    .filter(Boolean).join('-')
  downloadConsoleOutput(props.command?.output || '', `log-${name || 'command'}.txt`)
}
</script>
