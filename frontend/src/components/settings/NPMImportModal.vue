<template>
  <div
    class="modal modal-blur fade show d-block"
    tabindex="-1"
    style="background: rgba(0,0,0,.4)"
    @click.self="$emit('close')"
  >
    <div class="modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">
            Importer des proxy hosts — {{ connectionName }}
          </h5>
          <button
            type="button"
            class="btn-close"
            @click="$emit('close')"
          />
        </div>

        <div class="modal-body p-0">
          <!-- Loading -->
          <div
            v-if="loading"
            class="p-4 text-center text-muted"
          >
            <div class="spinner-border spinner-border-sm me-2" />
            Récupération des proxy hosts depuis NPM…
          </div>

          <!-- Error -->
          <div
            v-else-if="loadError"
            class="p-4"
          >
            <div class="alert alert-danger mb-0">
              {{ loadError }}
            </div>
          </div>

          <!-- List -->
          <template v-else>
            <div class="px-3 py-2 border-bottom d-flex align-items-center gap-3">
              <label class="form-check mb-0">
                <input
                  v-model="allSelected"
                  class="form-check-input"
                  type="checkbox"
                  :indeterminate="isIndeterminate"
                >
                <span class="form-check-label text-muted small">Tout sélectionner</span>
              </label>
              <span class="text-muted small ms-auto">
                {{ selectedIds.length }} sélectionné(s) · {{ previews.length }} total
              </span>
            </div>

            <div class="list-group list-group-flush">
              <label
                v-for="h in previews"
                :key="h.npm_id"
                class="list-group-item list-group-item-action d-flex align-items-start gap-3 py-3"
                :class="{ 'opacity-50': h.already_imported }"
              >
                <input
                  class="form-check-input mt-1 flex-shrink-0"
                  type="checkbox"
                  :checked="selectedIds.includes(h.npm_id)"
                  :disabled="h.already_imported"
                  @change="toggle(h.npm_id)"
                >
                <div class="flex-grow-1 min-w-0">
                  <div class="d-flex align-items-center gap-2 flex-wrap">
                    <span class="fw-medium text-truncate">{{ h.domain_names[0] }}</span>
                    <span
                      v-for="d in h.domain_names.slice(1)"
                      :key="d"
                      class="badge bg-secondary-lt text-secondary small"
                    >{{ d }}</span>
                  </div>
                  <div class="text-muted small mt-1">
                    → {{ h.forward_host }}:{{ h.forward_port }}
                    <span
                      v-if="h.ssl_enabled"
                      class="badge bg-success-lt text-success ms-2"
                    >HTTPS</span>
                    <span
                      v-else
                      class="badge bg-secondary-lt text-secondary ms-2"
                    >HTTP</span>
                    <span
                      v-if="!h.npm_enabled"
                      class="badge bg-warning-lt text-warning ms-1"
                    >Désactivé dans NPM</span>
                  </div>
                </div>
                <div class="flex-shrink-0 text-end small">
                  <span
                    v-if="h.already_imported"
                    class="badge bg-blue-lt text-blue"
                  >Déjà importé</span>
                  <template v-else>
                    <span class="text-muted">Uptime + </span>
                    <span :class="h.ssl_enabled ? 'text-success' : 'text-muted'">SSL</span>
                  </template>
                </div>
              </label>

              <div
                v-if="previews.length === 0"
                class="list-group-item text-center text-muted py-4"
              >
                Aucun proxy host trouvé dans cette instance NPM.
              </div>
            </div>
          </template>
        </div>

        <div class="modal-footer">
          <span
            v-if="importMsg"
            :class="['me-auto small', importOk ? 'text-success' : 'text-danger']"
          >{{ importMsg }}</span>
          <button
            class="btn btn-secondary"
            @click="$emit('close')"
          >
            Annuler
          </button>
          <button
            class="btn btn-primary"
            :disabled="selectedIds.length === 0 || importing"
            @click="doImport"
          >
            <span
              v-if="importing"
              class="spinner-border spinner-border-sm me-1"
            />
            {{ importing ? 'Import…' : `Importer la sélection (${selectedIds.length})` }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { npmApi } from '../../api/npm'
import type { NPMProxyHostPreview } from '../../types/npm'

const props = defineProps<{
  connectionId: string
  connectionName: string
}>()

const emit = defineEmits<{
  close: []
  imported: [count: number]
}>()

const previews = ref<NPMProxyHostPreview[]>([])
const loading = ref(true)
const loadError = ref('')
const selectedIds = ref<number[]>([])
const importing = ref(false)
const importMsg = ref('')
const importOk = ref(false)

// ─── Select-all ───────────────────────────────────────────────────────────────

const selectableIds = computed(() => previews.value.filter(h => !h.already_imported).map(h => h.npm_id))

const allSelected = computed({
  get: () => selectableIds.value.length > 0 && selectableIds.value.every((id: number) => selectedIds.value.includes(id)),
  set: (v: boolean) => {
    selectedIds.value = v ? [...selectableIds.value] : []
  },
})

const isIndeterminate = computed(() =>
  selectedIds.value.length > 0 && !allSelected.value,
)

function toggle(id: number): void {
  const idx: number = selectedIds.value.indexOf(id)
  if (idx === -1) selectedIds.value.push(id)
  else selectedIds.value.splice(idx, 1)
}

// ─── Load preview ─────────────────────────────────────────────────────────────

async function loadPreview(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const res = await npmApi.previewProxyHosts(props.connectionId)
    previews.value = res.data.proxy_hosts ?? []
    // Pre-select all importable hosts.
    selectedIds.value = selectableIds.value.slice()
  } catch (e: any) {
    loadError.value = e?.response?.data?.error || 'Impossible de récupérer les proxy hosts depuis NPM.'
  } finally {
    loading.value = false
  }
}

// ─── Import ───────────────────────────────────────────────────────────────────

async function doImport(): Promise<void> {
  if (selectedIds.value.length === 0) return
  importing.value = true
  importMsg.value = ''
  try {
    const res = await npmApi.importSelected(props.connectionId, selectedIds.value)
    const n = res.data.imported
    importMsg.value = `${n} proxy host${n !== 1 ? 's' : ''} importé${n !== 1 ? 's' : ''}.`
    importOk.value = true
    emit('imported', n)
    // Reload preview so imported items are greyed out.
    await loadPreview()
  } catch (e: any) {
    importMsg.value = e?.response?.data?.error || 'Erreur lors de l\'import.'
    importOk.value = false
  } finally {
    importing.value = false
  }
}

onMounted(loadPreview)
</script>
