<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            Dashboard
          </router-link>
          <span class="text-muted mx-1">/</span>
          <router-link
            to="/account"
            class="text-decoration-none"
          >
            Mon compte
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>Sécurité du compte</span>
        </div>
        <h2 class="page-title">
          Authentification MFA
        </h2>
        <div class="text-secondary">
          Configuration de la sécurité utilisateur
        </div>
      </div>
    </div>

    <!-- MFA card -->
    <div
      class="card mb-4"
      style="max-width: 640px;"
    >
      <div class="card-body">
        <div class="d-flex align-items-center justify-content-between mb-3">
          <div class="fw-semibold">
            Authentification multi-facteur
          </div>
          <span :class="mfaEnabled ? 'badge bg-green-lt text-green' : 'badge bg-orange-lt text-orange'">
            {{ mfaEnabled ? 'Activé' : 'Désactivé' }}
          </span>
        </div>

        <div v-if="!mfaEnabled">
          <p class="text-secondary">
            Activez le MFA pour renforcer la sécurité du compte.
          </p>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="loading"
            @click="startSetup"
          >
            {{ loading ? 'Chargement...' : 'Activer MFA' }}
          </button>
        </div>

        <div v-else>
          <p class="text-secondary">
            Le MFA est actif. Vous pouvez le désactiver si besoin.
          </p>
          <button
            type="button"
            class="btn btn-outline-danger"
            @click="showDisable = true"
          >
            Désactiver le MFA
          </button>
        </div>

        <!-- Setup panel -->
        <div
          v-if="setupVisible"
          class="mt-4"
        >
          <div class="border rounded p-3">
            <div class="fw-semibold mb-2">
              Configuration MFA
            </div>

            <!-- Countdown bar -->
            <div class="d-flex align-items-center gap-2 mb-3">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="14"
                height="14"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                viewBox="0 0 24 24"
                :class="setupSecondsLeft < 120 ? 'text-danger' : 'text-secondary'"
              >
                <circle
                  cx="12"
                  cy="12"
                  r="10"
                /><polyline points="12 6 12 12 16 14" />
              </svg>
              <span
                class="small fw-semibold"
                :class="setupSecondsLeft < 120 ? 'text-danger' : 'text-secondary'"
              >
                Expire dans {{ formatCountdown(setupSecondsLeft) }}
              </span>
              <div
                class="progress flex-fill"
                style="height: 4px;"
              >
                <div
                  class="progress-bar"
                  :class="setupSecondsLeft < 120 ? 'bg-danger' : 'bg-azure'"
                  :style="{ width: setupProgressPct + '%', transition: 'width 1s linear' }"
                />
              </div>
            </div>

            <div class="text-secondary small mb-3">
              Scannez le QR code avec votre application d'authentification, puis saisissez le code généré pour confirmer.
            </div>
            <div class="d-flex flex-column flex-md-row gap-3 align-items-center">
              <img
                :src="setup.qr_code"
                alt="QR Code"
                class="border rounded"
                style="width: 160px; height: 160px;"
              >
              <div class="flex-fill">
                <div class="text-secondary small mb-1">
                  Clé secrète
                </div>
                <div class="bg-dark text-light rounded p-2 mb-3 d-flex align-items-center justify-content-between gap-2">
                  <code class="small">{{ setup.secret }}</code>
                  <button
                    type="button"
                    class="btn btn-sm btn-ghost-light py-0"
                    title="Copier"
                    @click="copySecret"
                  >
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="14"
                      height="14"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                      viewBox="0 0 24 24"
                    >
                      <rect
                        x="9"
                        y="9"
                        width="13"
                        height="13"
                        rx="2"
                        ry="2"
                      /><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" />
                    </svg>
                    {{ copiedSecret ? '✓' : '' }}
                  </button>
                </div>
                <div class="mb-3">
                  <label class="form-label">Code TOTP</label>
                  <input
                    v-model="verifyCode"
                    type="text"
                    class="form-control"
                    placeholder="123456"
                    inputmode="numeric"
                    maxlength="6"
                    autocomplete="one-time-code"
                  >
                </div>
                <button
                  type="button"
                  class="btn btn-success"
                  :disabled="loading || verifyCode.length !== 6"
                  @click="verifySetup"
                >
                  {{ loading ? 'Vérification...' : 'Vérifier et activer' }}
                </button>
              </div>
            </div>

            <div
              v-if="setup.backup_codes?.length"
              class="mt-4"
            >
              <div class="text-secondary small mb-1">
                Codes de secours — conservez-les dans un endroit sûr
              </div>
              <pre class="bg-dark text-light rounded p-2 small">{{ setup.backup_codes.join('\n') }}</pre>
              <button
                type="button"
                class="btn btn-outline-light btn-sm"
                @click="copyBackupCodes"
              >
                {{ copiedBackup ? 'Copié ✓' : 'Copier les codes' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Disable panel -->
        <div
          v-if="showDisable"
          class="mt-4"
        >
          <div class="border rounded p-3">
            <div class="fw-semibold mb-2">
              Désactiver le MFA
            </div>
            <div class="mb-3">
              <label class="form-label">Mot de passe</label>
              <input
                v-model="disablePassword"
                type="password"
                class="form-control"
                placeholder="••••••••"
              >
            </div>
            <button
              type="button"
              class="btn btn-danger"
              :disabled="loading || !disablePassword"
              @click="disableMFA"
            >
              {{ loading ? 'Désactivation...' : 'Confirmer la désactivation' }}
            </button>
            <button
              type="button"
              class="btn btn-outline-secondary ms-2"
              :disabled="loading"
              @click="showDisable = false"
            >
              Annuler
            </button>
          </div>
        </div>

        <div
          v-if="error"
          class="alert alert-danger mt-3"
          role="alert"
        >
          {{ error }}
        </div>
        <div
          v-if="success"
          class="alert alert-success mt-3"
          role="alert"
        >
          {{ success }}
        </div>
      </div>
    </div>

    <!-- Sessions actives -->
    <div
      class="card"
      style="max-width: 640px;"
    >
      <div class="card-header d-flex align-items-center justify-content-between">
        <h3 class="card-title mb-0">
          <svg
            class="icon me-2"
            width="18"
            height="18"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
            />
          </svg>
          Sessions actives
        </h3>
        <button
          v-if="auth.isAuthenticated"
          type="button"
          class="btn btn-sm btn-outline-danger"
          :disabled="revokeLoading"
          @click="revokeOtherSessions"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="14"
            height="14"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            viewBox="0 0 24 24"
            class="me-1"
          >
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
          {{ revokeLoading ? 'Révocation...' : 'Révoquer les autres sessions' }}
        </button>
      </div>
      <div class="card-body pb-0">
        <p class="text-secondary small mb-3">
          Connexions récentes associées à votre compte. Le bouton ci-dessus déconnecte tous les autres appareils immédiatement.
        </p>
      </div>
      <div class="table-responsive">
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>IP</th>
              <th>Navigateur</th>
              <th>OS</th>
              <th>Statut</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="sessionsLoading">
              <td
                colspan="5"
                class="text-center text-secondary py-3"
              >
                Chargement...
              </td>
            </tr>
            <tr v-else-if="!loginEvents.length">
              <td
                colspan="5"
                class="text-center text-secondary py-3"
              >
                Aucune connexion enregistrée
              </td>
            </tr>
            <tr
              v-for="ev in loginEvents"
              :key="ev.id"
            >
              <td class="text-secondary small">
                {{ formatDateTime(ev.created_at) }}
              </td>
              <td class="text-secondary small font-monospace">
                {{ ev.ip_address }}
              </td>
              <td class="text-secondary small">
                {{ parseUA(ev.user_agent).browser }}
              </td>
              <td class="text-secondary small">
                {{ parseUA(ev.user_agent).os }}
              </td>
              <td>
                <span
                  class="badge"
                  :class="ev.success ? 'bg-green-lt text-green' : 'bg-red-lt text-red'"
                >
                  {{ ev.success ? 'Succès' : 'Échec' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="revokeError"
        class="card-body pt-0"
      >
        <div
          class="alert alert-danger mb-0"
          role="alert"
        >
          {{ revokeError }}
        </div>
      </div>
      <div
        v-if="revokeSuccess"
        class="card-body pt-0"
      >
        <div
          class="alert alert-success mb-0"
          role="alert"
        >
          {{ revokeSuccess }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import apiClient from '../api'
import { formatDateTime } from '../utils/formatters'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()

const mfaEnabled = ref(false)
const setupVisible = ref(false)
const setup = ref<{ secret: string; qr_code: string; backup_codes: string[] }>({ secret: '', qr_code: '', backup_codes: [] })
const verifyCode = ref('')
const disablePassword = ref('')
const showDisable = ref(false)
const loading = ref(false)
const error = ref('')
const success = ref('')
const copiedBackup = ref(false)
const copiedSecret = ref(false)

// TOTP countdown
const SETUP_TIMEOUT = 600
const setupSecondsLeft = ref(0)
let setupTimer: ReturnType<typeof setInterval> | undefined

const setupProgressPct = computed(() =>
  Math.round((setupSecondsLeft.value / SETUP_TIMEOUT) * 100)
)

function formatCountdown(secs: number): string {
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

function startSetupTimer(): void {
  if (setupTimer) clearInterval(setupTimer)
  setupSecondsLeft.value = SETUP_TIMEOUT
  setupTimer = setInterval(() => {
    setupSecondsLeft.value = Math.max(0, setupSecondsLeft.value - 1)
    if (setupSecondsLeft.value === 0) {
      if (setupTimer) clearInterval(setupTimer)
      setupVisible.value = false
      error.value = 'Le délai de configuration a expiré. Veuillez cliquer sur "Activer MFA" pour recommencer.'
    }
  }, 1000)
}

function stopSetupTimer(): void {
  if (setupTimer) clearInterval(setupTimer)
  setupSecondsLeft.value = 0
}

const loginEvents = ref<any[]>([])
const sessionsLoading = ref(false)
const revokeLoading = ref(false)
const revokeError = ref('')
const revokeSuccess = ref('')

async function loadStatus() {
  try {
    const res = await apiClient.getMFAStatus()
    mfaEnabled.value = !!res.data?.mfa_enabled
  } catch {
    mfaEnabled.value = false
  }
}

async function loadLoginEvents() {
  sessionsLoading.value = true
  try {
    const res = await apiClient.getLoginEvents()
    loginEvents.value = (res.data?.events || []).slice(0, 15)
  } catch {
    loginEvents.value = []
  } finally {
    sessionsLoading.value = false
  }
}

async function startSetup() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await apiClient.setupMFA()
    setup.value = res.data
    setupVisible.value = true
    startSetupTimer()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur lors de la configuration MFA'
  } finally {
    loading.value = false
  }
}

async function verifySetup() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    await apiClient.verifyMFA(setup.value.secret, verifyCode.value, setup.value.backup_codes)
    success.value = 'MFA activé avec succès.'
    setupVisible.value = false
    verifyCode.value = ''
    stopSetupTimer()
    await loadStatus()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Code invalide'
  } finally {
    loading.value = false
  }
}

async function disableMFA() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    await apiClient.disableMFA(disablePassword.value)
    success.value = 'MFA désactivé.'
    showDisable.value = false
    disablePassword.value = ''
    await loadStatus()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Erreur lors de la désactivation'
  } finally {
    loading.value = false
  }
}

async function revokeOtherSessions() {
  if (!auth.isAuthenticated) return
  revokeLoading.value = true
  revokeError.value = ''
  revokeSuccess.value = ''
  try {
    await apiClient.revokeAllSessions()
    revokeSuccess.value = 'Toutes les autres sessions ont été révoquées.'
    await loadLoginEvents()
  } catch (e: any) {
    revokeError.value = e?.response?.data?.error || 'Erreur lors de la révocation des sessions.'
  } finally {
    revokeLoading.value = false
  }
}

async function copySecret() {
  await navigator.clipboard.writeText(setup.value.secret)
  copiedSecret.value = true
  setTimeout(() => { copiedSecret.value = false }, 1500)
}

async function copyBackupCodes() {
  if (!setup.value.backup_codes?.length) return
  await navigator.clipboard.writeText(setup.value.backup_codes.join('\n'))
  copiedBackup.value = true
  setTimeout(() => { copiedBackup.value = false }, 1500)
}

function parseUA(ua: string | undefined): { browser: string; os: string } {
  if (!ua) return { browser: '—', os: '—' }
  const browser = ua.includes('Firefox/') ? 'Firefox'
    : ua.includes('Edg/') ? 'Edge'
    : ua.includes('Chrome/') ? 'Chrome'
    : ua.includes('Safari/') ? 'Safari' : 'Other'
  const os = ua.includes('Windows') ? 'Windows'
    : ua.includes('Mac OS X') ? 'macOS'
    : ua.includes('Android') ? 'Android'
    : (ua.includes('iPhone') || ua.includes('iPad')) ? 'iOS'
    : ua.includes('Linux') ? 'Linux' : 'Other'
  return { browser, os }
}

onMounted(() => {
  loadStatus()
  loadLoginEvents()
})

onUnmounted(() => { stopSetupTimer() })
</script>
