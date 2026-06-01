<template>
  <div>
    <div class="page-header mb-3">
      <div class="d-flex flex-column flex-lg-row align-items-lg-center justify-content-between gap-3">
        <div>
          <h2 class="page-title">
            Certificats SSL/TLS
          </h2>
          <div class="text-muted">
            Surveillance des dates d'expiration. Vérifié toutes les 6 heures depuis le serveur.
          </div>
        </div>
        <button
          v-if="auth.role === 'admin'"
          class="btn btn-primary"
          @click="openCreate"
        >
          + Ajouter un certificat
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
      v-if="loading && !certs.length"
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
      v-else-if="!certs.length"
      title="Aucun certificat surveillé"
      subtitle="Ajoutez un domaine pour suivre l'expiration de son certificat TLS."
      :cta-label="auth.role === 'admin' ? 'Ajouter un certificat' : ''"
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
              <th>Nom</th>
              <th>Endpoint</th>
              <th>Émetteur</th>
              <th>Expiration</th>
              <th>Jours restants</th>
              <th>Dernière vérification</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="c in certs"
              :key="c.id"
            >
              <td class="fw-semibold">
                {{ c.name }}
                <span
                  v-if="!c.enabled"
                  class="badge bg-secondary-lt text-secondary ms-1"
                >désactivé</span>
              </td>
              <td class="text-secondary">
                <code>{{ c.host }}:{{ c.port }}</code>
              </td>
              <td class="text-secondary small">
                {{ shortIssuer(c.issuer) || '—' }}
              </td>
              <td class="text-secondary small">
                {{ c.valid_to ? formatDate(c.valid_to) : '—' }}
              </td>
              <td>
                <span :class="['badge', daysBadge(c.days_remaining)]">
                  {{ daysLabel(c.days_remaining) }}
                </span>
              </td>
              <td class="text-secondary small">
                <RelativeTime
                  v-if="c.last_checked_at"
                  :date="c.last_checked_at"
                />
                <span
                  v-else
                  class="text-secondary"
                >Jamais</span>
                <div
                  v-if="c.last_error"
                  class="text-danger small"
                  :title="c.last_error"
                >
                  {{ c.last_error.length > 40 ? c.last_error.slice(0, 40) + '...' : c.last_error }}
                </div>
              </td>
              <td class="text-end">
                <div class="btn-list">
                  <button
                    v-if="auth.role === 'admin'"
                    class="btn btn-sm btn-outline-secondary"
                    :disabled="checkingId === c.id"
                    @click="checkNow(c)"
                  >
                    {{ checkingId === c.id ? '...' : 'Vérifier' }}
                  </button>
                  <button
                    v-if="auth.role === 'admin'"
                    class="btn btn-sm btn-outline-secondary"
                    @click="openEdit(c)"
                  >
                    Modifier
                  </button>
                  <button
                    v-if="auth.role === 'admin'"
                    class="btn btn-sm btn-outline-danger"
                    @click="confirmDelete(c)"
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

    <!-- Modal -->
    <div
      v-if="modalOpen"
      class="modal modal-blur fade show"
      style="display:block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ form.id ? 'Modifier le certificat' : 'Nouveau certificat' }}
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
              <div class="mb-3">
                <label class="form-label required">Nom</label>
                <input
                  v-model="form.name"
                  type="text"
                  class="form-control"
                  placeholder="Ex: api.example.com"
                  required
                >
              </div>
              <div class="row g-3">
                <div class="col-md-8">
                  <label class="form-label required">Hôte</label>
                  <input
                    v-model="form.host"
                    type="text"
                    class="form-control"
                    placeholder="api.example.com"
                    required
                  >
                </div>
                <div class="col-md-4">
                  <label class="form-label required">Port</label>
                  <input
                    v-model.number="form.port"
                    type="number"
                    min="1"
                    max="65535"
                    class="form-control"
                  >
                </div>
                <div class="col-12">
                  <label class="form-label">SNI (override, optionnel)</label>
                  <input
                    v-model="form.server_name"
                    type="text"
                    class="form-control"
                    placeholder="Laisser vide pour utiliser l'hôte"
                  >
                </div>
                <div class="col-12">
                  <label class="form-check">
                    <input
                      v-model="form.enabled"
                      type="checkbox"
                      class="form-check-input"
                    >
                    <span class="form-check-label">Activé</span>
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
import dayjs from '../utils/dayjs'

interface SSLCert {
  id: string
  name: string
  host: string
  port: number
  server_name?: string
  enabled: boolean
  [key: string]: any
}

interface CertForm {
  id: string
  name: string
  host: string
  port: number
  server_name: string
  enabled: boolean
}

const auth = useAuthStore()
const dialog = useConfirmDialog()

const certs = ref<SSLCert[]>([])
const loading = ref(false)
const error = ref('')
const checkingId = ref('')

const modalOpen = ref(false)
const saving = ref(false)
const formError = ref('')
const form = ref<CertForm>(emptyForm())

function emptyForm(): CertForm {
  return { id: '', name: '', host: '', port: 443, server_name: '', enabled: true }
}

function formatDate(ts: string | undefined | null): string {
  return ts ? dayjs(ts).format('YYYY-MM-DD') : '—'
}

function shortIssuer(s: string | undefined): string {
  if (!s) return ''
  const cn = /CN=([^,]+)/.exec(s)
  return cn ? cn[1] : s.split(',')[0]
}

function daysLabel(d: number | null | undefined): string {
  if (d == null) return 'Inconnu'
  if (d < 0) return `Expiré (${Math.abs(d)}j)`
  return `${d}j`
}

function daysBadge(d: number | null | undefined): string {
  if (d == null) return 'bg-secondary-lt text-secondary'
  if (d < 0) return 'bg-red text-white'
  if (d <= 7) return 'bg-red-lt text-red'
  if (d <= 30) return 'bg-yellow-lt text-yellow'
  return 'bg-green-lt text-green'
}

async function fetchCerts(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const { data } = await api.getSSLCertificates()
    certs.value = data?.certificates || []
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Impossible de charger les certificats'
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  form.value = emptyForm()
  formError.value = ''
  modalOpen.value = true
}

function openEdit(c: SSLCert): void {
  form.value = {
    id: c.id,
    name: c.name,
    host: c.host,
    port: c.port,
    server_name: c.server_name || '',
    enabled: c.enabled,
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
      await api.updateSSLCertificate(form.value.id, body)
    } else {
      await api.createSSLCertificate(body)
    }
    closeModal()
    await fetchCerts()
  } catch (e: any) {
    formError.value = e?.response?.data?.error || e?.message || 'Erreur lors de l\'enregistrement'
  } finally {
    saving.value = false
  }
}

async function checkNow(c: SSLCert): Promise<void> {
  checkingId.value = c.id
  try {
    await api.checkSSLCertificateNow(c.id)
    await fetchCerts()
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Échec de la vérification'
  } finally {
    checkingId.value = ''
  }
}

async function confirmDelete(c: SSLCert): Promise<void> {
  const ok = await dialog.confirm({
    title: 'Supprimer le certificat ?',
    message: `Cette action supprimera "${c.name}" du suivi.`,
    okLabel: 'Supprimer',
    destructive: true,
  })
  if (!ok) return
  try {
    await api.deleteSSLCertificate(c.id)
    await fetchCerts()
  } catch (e: any) {
    error.value = e?.response?.data?.error || e?.message || 'Suppression impossible'
  }
}

let refreshTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  fetchCerts()
  refreshTimer = setInterval(fetchCerts, 60000)
})
onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>
