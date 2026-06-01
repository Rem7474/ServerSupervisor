<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            Uptime / Sondes synthétiques
          </h2>
          <div class="text-muted">
            Sondes HTTP et TCP exécutées depuis le serveur. Vérifient la disponibilité et la latence à intervalle régulier.
          </div>
        </div>
        <button
          v-if="auth.role === 'admin'"
          class="btn btn-primary"
          @click="openCreate"
        >
          + Nouvelle sonde
        </button>
      </div>
    </div>

    <div
      v-if="error"
      class="alert alert-danger mb-3"
    >
      {{ error }}
    </div>

    <div
      v-if="loading && !probes.length"
      class="row row-cards"
    >
      <div class="col-12">
        <LoadingSkeleton
          variant="table"
          :lines="5"
        />
      </div>
    </div>

    <EmptyState
      v-else-if="!probes.length"
      title="Aucune sonde configurée"
      subtitle="Créez votre première sonde HTTP ou TCP pour surveiller un service."
      :cta-label="auth.role === 'admin' ? 'Nouvelle sonde' : ''"
      @cta="openCreate"
    />

    <div
      v-else
      class="card"
    >
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Sonde</th>
              <th>Cible</th>
              <th>Statut</th>
              <th>Latence</th>
              <th>Dernière vérification</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="p in probes"
              :key="p.id"
            >
              <td>
                <router-link
                  :to="`/uptime/probes/${p.id}`"
                  class="fw-semibold text-decoration-none"
                >
                  {{ p.name }}
                </router-link>
                <div class="text-secondary small">
                  {{ p.type.toUpperCase() }} · interval {{ p.interval_sec }}s
                </div>
              </td>
              <td class="text-secondary">
                <code>{{ p.target }}</code>
              </td>
              <td>
                <span :class="['badge', statusBadge(p)]">
                  {{ statusLabel(p) }}
                </span>
                <span
                  v-if="!p.enabled"
                  class="badge bg-secondary-lt text-secondary ms-1"
                >désactivée</span>
              </td>
              <td>
                <template v-if="p.last_latency_ms != null && p.last_status === 'up'">
                  {{ p.last_latency_ms }} ms
                </template>
                <span
                  v-else
                  class="text-secondary"
                >—</span>
              </td>
              <td class="text-secondary small">
                <RelativeTime
                  v-if="p.last_checked_at"
                  :date="p.last_checked_at"
                />
                <span
                  v-else
                  class="text-secondary"
                >Jamais</span>
              </td>
              <td class="text-end">
                <div class="btn-list">
                  <button
                    v-if="auth.role === 'admin'"
                    class="btn btn-sm btn-outline-secondary"
                    :disabled="checkingId === p.id"
                    @click="checkNow(p)"
                  >
                    {{ checkingId === p.id ? '...' : 'Vérifier' }}
                  </button>
                  <button
                    v-if="auth.role === 'admin'"
                    class="btn btn-sm btn-outline-secondary"
                    @click="openEdit(p)"
                  >
                    Modifier
                  </button>
                  <button
                    v-if="auth.role === 'admin'"
                    class="btn btn-sm btn-outline-danger"
                    @click="confirmDelete(p)"
                  >
                    Supprimer
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Modal create/edit -->
    <div
      v-if="modalOpen"
      class="modal modal-blur fade show"
      style="display:block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ form.id ? 'Modifier la sonde' : 'Nouvelle sonde' }}
            </h5>
            <button
              type="button"
              class="btn-close"
              :disabled="saving"
              @click="closeModal"
            />
          </div>
          <form @submit.prevent="save">
            <div class="modal-body">
              <div
                v-if="formError"
                class="alert alert-danger"
              >
                {{ formError }}
              </div>
              <div class="row g-3">
                <div class="col-md-7">
                  <label class="form-label required">Nom</label>
                  <input
                    v-model="form.name"
                    type="text"
                    class="form-control"
                    placeholder="Ex: API prod"
                    required
                  >
                </div>
                <div class="col-md-5">
                  <label class="form-label required">Type</label>
                  <select
                    v-model="form.type"
                    class="form-select"
                  >
                    <option value="http">
                      HTTP/HTTPS
                    </option>
                    <option value="tcp">
                      TCP
                    </option>
                  </select>
                </div>
                <div class="col-12">
                  <label class="form-label required">{{ form.type === 'http' ? 'URL' : 'host:port' }}</label>
                  <input
                    v-model="form.target"
                    type="text"
                    class="form-control"
                    :placeholder="form.type === 'http' ? 'https://example.com/health' : 'example.com:443'"
                    required
                  >
                </div>
                <div class="col-md-4">
                  <label class="form-label">Intervalle (sec)</label>
                  <input
                    v-model.number="form.interval_sec"
                    type="number"
                    min="10"
                    class="form-control"
                  >
                </div>
                <div class="col-md-4">
                  <label class="form-label">Timeout (sec)</label>
                  <input
                    v-model.number="form.timeout_sec"
                    type="number"
                    min="1"
                    max="60"
                    class="form-control"
                  >
                </div>
                <template v-if="form.type === 'http'">
                  <div class="col-md-4">
                    <label class="form-label">Statut HTTP attendu</label>
                    <input
                      v-model.number="form.expected_status"
                      type="number"
                      min="100"
                      max="599"
                      class="form-control"
                    >
                  </div>
                  <div class="col-12">
                    <label class="form-label">Regex corps attendu (optionnel)</label>
                    <input
                      v-model="form.expected_body_regex"
                      type="text"
                      class="form-control"
                      placeholder="Ex: &quot;status&quot;:\s*&quot;ok&quot;"
                    >
                  </div>
                  <div class="col-md-6">
                    <label class="form-check">
                      <input
                        v-model="form.follow_redirects"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Suivre les redirections</span>
                    </label>
                  </div>
                  <div class="col-md-6">
                    <label class="form-check">
                      <input
                        v-model="form.verify_tls"
                        type="checkbox"
                        class="form-check-input"
                      >
                      <span class="form-check-label">Vérifier le certificat TLS</span>
                    </label>
                  </div>
                </template>
                <div class="col-12">
                  <label class="form-check">
                    <input
                      v-model="form.enabled"
                      type="checkbox"
                      class="form-check-input"
                    >
                    <span class="form-check-label">Activée</span>
                  </label>
                </div>
              </div>
            </div>
            <div class="modal-footer">
              <button
                type="button"
                class="btn link-secondary"
                :disabled="saving"
                @click="closeModal"
              >
                Annuler
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="saving"
              >
                {{ saving ? 'Enregistrement...' : 'Enregistrer' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
    <div
      v-if="modalOpen"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import api from '../api'
import { useAuthStore } from '../stores/auth'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import RelativeTime from '../components/RelativeTime.vue'

interface Probe {
  id: string
  name: string
  type: string
  target: string
  interval_sec: number
  timeout_sec: number
  expected_status: number
  expected_body_regex?: string
  follow_redirects: boolean
  verify_tls: boolean
  enabled: boolean
  last_status?: string
  [key: string]: any
}

interface ProbeForm {
  id: string
  name: string
  type: string
  target: string
  interval_sec: number
  timeout_sec: number
  expected_status: number
  expected_body_regex: string
  follow_redirects: boolean
  verify_tls: boolean
  enabled: boolean
}

const auth = useAuthStore()
const dialog = useConfirmDialog()

const probes = ref<Probe[]>([])
const loading = ref(false)
const error = ref('')
const checkingId = ref('')

const modalOpen = ref(false)
const saving = ref(false)
const formError = ref('')
const form = ref<ProbeForm>(emptyForm())

function emptyForm(): ProbeForm {
  return {
    id: '',
    name: '',
    type: 'http',
    target: '',
    interval_sec: 60,
    timeout_sec: 10,
    expected_status: 200,
    expected_body_regex: '',
    follow_redirects: true,
    verify_tls: true,
    enabled: true,
  }
}

function statusBadge(p: Probe): string {
  if (!p.enabled) return 'bg-secondary-lt text-secondary'
  if (p.last_status === 'up') return 'bg-green-lt text-green'
  if (p.last_status === 'down') return 'bg-red-lt text-red'
  return 'bg-secondary-lt text-secondary'
}

function statusLabel(p: Probe): string {
  if (p.last_status === 'up') return 'UP'
  if (p.last_status === 'down') return 'DOWN'
  return 'Inconnue'
}

async function fetchProbes(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.getUptimeProbes()
    probes.value = data?.probes || []
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Impossible de charger les sondes'
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  form.value = emptyForm()
  formError.value = ''
  modalOpen.value = true
}

function openEdit(p: Probe): void {
  form.value = {
    id: p.id,
    name: p.name,
    type: p.type,
    target: p.target,
    interval_sec: p.interval_sec,
    timeout_sec: p.timeout_sec,
    expected_status: p.expected_status,
    expected_body_regex: p.expected_body_regex || '',
    follow_redirects: p.follow_redirects,
    verify_tls: p.verify_tls,
    enabled: p.enabled,
  }
  formError.value = ''
  modalOpen.value = true
}

function closeModal(): void {
  modalOpen.value = false
  saving.value = false
}

async function save(): Promise<void> {
  saving.value = true
  formError.value = ''
  try {
    const { id: _id, ...body } = form.value
    if (form.value.id) {
      await api.updateUptimeProbe(form.value.id, body)
    } else {
      await api.createUptimeProbe(body)
    }
    closeModal()
    await fetchProbes()
  } catch (e: any) {
    formError.value = e?.response?.data?.error || e?.message || 'Erreur lors de l\'enregistrement'
  } finally {
    saving.value = false
  }
}

async function checkNow(p: Probe): Promise<void> {
  checkingId.value = p.id
  try {
    await api.checkUptimeProbeNow(p.id)
    await fetchProbes()
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Échec de la vérification'
  } finally {
    checkingId.value = ''
  }
}

async function confirmDelete(p: Probe): Promise<void> {
  const ok = await dialog.confirm({
    title: 'Supprimer la sonde ?',
    message: `Cette action supprimera "${p.name}" et tout son historique.`,
    okLabel: 'Supprimer',
    destructive: true,
  })
  if (!ok) return
  try {
    await api.deleteUptimeProbe(p.id)
    await fetchProbes()
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Suppression impossible'
  }
}

let refreshTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  fetchProbes()
  refreshTimer = setInterval(fetchProbes, 15000)
})
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>
