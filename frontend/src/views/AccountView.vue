<template>
  <div>
    <!-- Forced password change banner -->
    <div v-if="auth.mustChangePassword" class="alert alert-warning alert-dismissible mb-4" role="alert">
      <div class="d-flex align-items-center">
        <svg class="icon alert-icon me-2" width="24" height="24" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
        </svg>
        <strong>Changement de mot de passe requis.</strong>&nbsp;Pour des raisons de sécurité, veuillez définir un nouveau mot de passe avant de continuer.
      </div>
    </div>

    <div class="page-header mb-4">
      <div class="row align-items-center">
        <div class="col-auto">
          <h2 class="page-title">Mon compte</h2>
          <div class="text-secondary">Gérez vos informations personnelles et la sécurité de votre compte</div>
        </div>
      </div>
    </div>

    <div class="row g-4">
      <!-- Profile info card -->
      <div class="col-12 col-lg-4">
        <div class="card">
          <div class="card-body text-center py-4">
            <div class="avatar avatar-xl mb-3" style="width:64px;height:64px;font-size:1.6rem;background:var(--tblr-azure-lt);color:var(--tblr-azure);border-radius:50%;display:flex;align-items:center;justify-content:center;margin:0 auto;">
              {{ auth.username?.slice(0, 2).toUpperCase() }}
            </div>
            <div class="h3 mb-1">{{ profile?.username || auth.username }}</div>
            <div class="mb-3">
              <span class="badge" :class="roleBadgeClass">{{ roleLabel }}</span>
            </div>
            <div class="text-secondary small" v-if="profile?.created_at">
              Membre depuis {{ formatDate(profile.created_at) }}
            </div>
          </div>
          <div class="card-footer text-center py-3">
            <div class="row g-3">
              <div class="col-6 border-end">
                <div class="text-secondary small">MFA</div>
                <div class="fw-bold" :class="profile?.mfa_enabled ? 'text-success' : 'text-secondary'">
                  {{ profile?.mfa_enabled ? 'Activé' : 'Désactivé' }}
                </div>
              </div>
              <div class="col-6">
                <div class="text-secondary small">Statut</div>
                <div class="fw-bold text-success">Actif</div>
              </div>
            </div>
          </div>
        </div>

        <!-- MFA card -->
        <div class="card mt-4">
          <div class="card-header">
            <h3 class="card-title">
              <svg class="icon me-2" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
              </svg>
              Authentification à deux facteurs
            </h3>
          </div>
          <div class="card-body">
            <div class="d-flex align-items-center justify-content-between mb-3">
              <div>
                <div class="fw-bold">TOTP (Authenticator)</div>
                <div class="text-secondary small">Google Authenticator, Authy, etc.</div>
              </div>
              <span class="badge" :class="profile?.mfa_enabled ? 'bg-success-lt text-success' : 'bg-secondary-lt text-secondary'">
                {{ profile?.mfa_enabled ? 'Actif' : 'Inactif' }}
              </span>
            </div>
            <router-link to="/security" class="btn btn-outline-secondary w-100">
              Gérer le MFA
            </router-link>
          </div>
        </div>
      </div>

      <!-- Change password card -->
      <div class="col-12 col-lg-8">
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">
              <svg class="icon me-2" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/>
              </svg>
              Changer le mot de passe
            </h3>
          </div>
          <div class="card-body">
            <form @submit.prevent="submitChangePassword">
              <div class="mb-3">
                <label class="form-label required">Mot de passe actuel</label>
                <input v-model="pwForm.current" type="password" class="form-control" :class="{ 'is-invalid': pwErrors.current }" placeholder="••••••••" required />
                <div v-if="pwErrors.current" class="invalid-feedback">{{ pwErrors.current }}</div>
              </div>
              <div class="mb-3">
                <label class="form-label required">Nouveau mot de passe</label>
                <input v-model="pwForm.next" type="password" class="form-control" :class="{ 'is-invalid': pwErrors.next }" placeholder="••••••••" required />
                <div v-if="pwErrors.next" class="invalid-feedback">{{ pwErrors.next }}</div>
                <div class="form-hint">Au moins 8 caractères.</div>
              </div>
              <div class="mb-4">
                <label class="form-label required">Confirmer le nouveau mot de passe</label>
                <input v-model="pwForm.confirm" type="password" class="form-control" :class="{ 'is-invalid': pwErrors.confirm }" placeholder="••••••••" required />
                <div v-if="pwErrors.confirm" class="invalid-feedback">{{ pwErrors.confirm }}</div>
              </div>

              <div v-if="pwError" class="alert alert-danger mb-3" role="alert">{{ pwError }}</div>
              <div v-if="pwSuccess" class="alert alert-success mb-3" role="alert">{{ pwSuccess }}</div>

              <div class="d-flex gap-2">
                <button type="submit" class="btn btn-primary" :disabled="pwLoading">
                  <span v-if="pwLoading" class="spinner-border spinner-border-sm me-2"></span>
                  {{ pwLoading ? 'Enregistrement...' : 'Mettre à jour le mot de passe' }}
                </button>
                <button v-if="!auth.mustChangePassword" type="button" class="btn btn-outline-secondary" @click="resetPwForm">
                  Annuler
                </button>
              </div>
            </form>
          </div>
        </div>

        <!-- Recent activity (audit logs for this user) -->
        <div class="card mt-4">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title mb-0">
              <svg class="icon me-2" width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
              </svg>
              Activité récente
            </h3>
            <span class="badge bg-azure-lt text-azure">{{ auditLogs.length }}</span>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Action</th>
                  <th>Ressource</th>
                  <th>Date</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="auditLoading">
                  <td colspan="3" class="text-center text-secondary py-3">Chargement...</td>
                </tr>
                <tr v-else-if="!auditLogs.length">
                  <td colspan="3" class="text-center text-secondary py-3">Aucune activité récente</td>
                </tr>
                <tr v-for="log in auditLogs" :key="log.id">
                  <td>
                    <span class="badge" :class="actionBadgeClass(log.action)">{{ log.action }}</span>
                  </td>
                  <td class="text-secondary small">{{ log.resource_type }} {{ log.resource_id ? `#${log.resource_id}` : '' }}</td>
                  <td class="text-secondary small">{{ formatDateTime(log.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import apiClient from '../api'
import { formatDateLong as formatDate, formatDateTime } from '../utils/formatters'

const auth = useAuthStore()
const router = useRouter()

const profile = ref(null)
const auditLogs = ref([])
const auditLoading = ref(false)

const pwForm = ref({ current: '', next: '', confirm: '' })
const pwErrors = ref({ current: '', next: '', confirm: '' })
const pwError = ref('')
const pwSuccess = ref('')
const pwLoading = ref(false)

const roleBadgeClass = computed(() => {
  const map = { admin: 'bg-danger-lt text-danger', operator: 'bg-warning-lt text-warning', viewer: 'bg-secondary-lt text-secondary' }
  return map[profile.value?.role] || 'bg-secondary-lt text-secondary'
})

const roleLabel = computed(() => {
  const map = { admin: 'Administrateur', operator: 'Opérateur', viewer: 'Lecteur' }
  return map[profile.value?.role] || profile.value?.role || auth.role
})


function actionBadgeClass(action) {
  if (!action) return 'bg-secondary-lt text-secondary'
  const a = action.toLowerCase()
  if (a.includes('delete') || a.includes('remove')) return 'bg-danger-lt text-danger'
  if (a.includes('create') || a.includes('add')) return 'bg-success-lt text-success'
  if (a.includes('update') || a.includes('change') || a.includes('rotate')) return 'bg-azure-lt text-azure'
  return 'bg-secondary-lt text-secondary'
}

function resetPwForm() {
  pwForm.value = { current: '', next: '', confirm: '' }
  pwErrors.value = { current: '', next: '', confirm: '' }
  pwError.value = ''
  pwSuccess.value = ''
}

async function submitChangePassword() {
  pwErrors.value = { current: '', next: '', confirm: '' }
  pwError.value = ''
  pwSuccess.value = ''

  let valid = true
  if (!pwForm.value.current) {
    pwErrors.value.current = 'Le mot de passe actuel est requis.'
    valid = false
  }
  if (pwForm.value.next.length < 8) {
    pwErrors.value.next = 'Le nouveau mot de passe doit faire au moins 8 caractères.'
    valid = false
  }
  if (pwForm.value.next !== pwForm.value.confirm) {
    pwErrors.value.confirm = 'La confirmation ne correspond pas.'
    valid = false
  }
  if (!valid) return

  pwLoading.value = true
  try {
    await apiClient.changePassword(pwForm.value.current, pwForm.value.next)
    pwSuccess.value = 'Mot de passe mis à jour avec succès.'
    pwForm.value = { current: '', next: '', confirm: '' }
    // Clear the forced change flag
    auth.clearMustChangePassword()
  } catch (e) {
    pwError.value = e.response?.data?.error || 'Erreur lors de la mise à jour du mot de passe.'
  } finally {
    pwLoading.value = false
  }
}

async function loadProfile() {
  try {
    const { data } = await apiClient.getProfile()
    profile.value = data
  } catch {
    // fallback to store data
  }
}

async function loadAuditLogs() {
  auditLoading.value = true
  try {
    const { data } = await apiClient.getMyAuditLogs(10)
    auditLogs.value = data?.logs || data || []
  } catch {
    auditLogs.value = []
  } finally {
    auditLoading.value = false
  }
}

onMounted(() => {
  loadProfile()
  loadAuditLogs()
})
</script>
