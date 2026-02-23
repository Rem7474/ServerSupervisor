<template>
  <div>
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <h2 class="page-title">Sécurité</h2>
        <div class="text-secondary">Gestion du MFA (TOTP)</div>
      </div>
    </div>

    <div class="card" style="max-width: 640px;">
      <div class="card-body">
        <div class="d-flex align-items-center justify-content-between mb-3">
          <div class="fw-semibold">Authentification multi-facteur</div>
          <span :class="mfaEnabled ? 'badge bg-green-lt text-green' : 'badge bg-secondary-lt text-secondary'">
            {{ mfaEnabled ? 'Activé' : 'Désactivée' }}
          </span>
        </div>

        <div v-if="!mfaEnabled">
          <p class="text-secondary">Activez le MFA pour renforcer la sécurité du compte.</p>
          <button class="btn btn-primary" @click="startSetup" :disabled="loading">
            {{ loading ? 'Chargement...' : 'Activer MFA' }}
          </button>
        </div>

        <div v-else>
          <p class="text-secondary">Le MFA est actif. Vous pouvez le désactiver si besoin.</p>
          <button class="btn btn-outline-danger" @click="showDisable = true">Désactiver le MFA</button>
        </div>

        <div v-if="setupVisible" class="mt-4">
          <div class="border rounded p-3">
            <div class="fw-semibold mb-2">Configuration MFA</div>
            <div class="text-secondary small mb-3">Scannez le QR code avec votre application d'authentification.</div>
            <div class="d-flex flex-column flex-md-row gap-3 align-items-center">
              <img :src="setup.qr_code" alt="QR Code" class="border rounded" style="width: 160px; height: 160px;" />
              <div class="flex-fill">
                <div class="text-secondary small mb-1">Cle secrete</div>
                <div class="bg-dark text-light rounded p-2 mb-3"><code>{{ setup.secret }}</code></div>
                <div class="mb-3">
                  <label class="form-label">Code TOTP</label>
                  <input v-model="verifyCode" type="text" class="form-control" placeholder="123456" inputmode="numeric" maxlength="6" />
                </div>
                <button class="btn btn-success" @click="verifySetup" :disabled="loading || !verifyCode">
                  {{ loading ? 'Vérification...' : 'Vérifier et activer' }}
                </button>
              </div>
            </div>

            <div v-if="setup.backup_codes?.length" class="mt-4">
              <div class="text-secondary small mb-1">Codes de secours</div>
              <pre class="bg-dark text-light rounded p-2 small">{{ setup.backup_codes.join('\n') }}</pre>
              <button class="btn btn-outline-light btn-sm" @click="copyBackupCodes">
                {{ copiedBackup ? 'Copie' : 'Copier les codes' }}
              </button>
            </div>
          </div>
        </div>

        <div v-if="showDisable" class="mt-4">
          <div class="border rounded p-3">
            <div class="fw-semibold mb-2">Désactiver le MFA</div>
            <div class="mb-3">
              <label class="form-label">Mot de passe</label>
              <input v-model="disablePassword" type="password" class="form-control" placeholder="••••••••" />
            </div>
            <button class="btn btn-danger" @click="disableMFA" :disabled="loading || !disablePassword">
              {{ loading ? 'Désactivation...' : 'Confirmer la désactivation' }}
            </button>
            <button class="btn btn-outline-secondary ms-2" @click="showDisable = false" :disabled="loading">Annuler</button>
          </div>
        </div>

        <div v-if="error" class="alert alert-danger mt-3" role="alert">
          {{ error }}
        </div>
        <div v-if="success" class="alert alert-success mt-3" role="alert">
          {{ success }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import apiClient from '../api'

const mfaEnabled = ref(false)
const setupVisible = ref(false)
const setup = ref({ secret: '', qr_code: '', backup_codes: [] })
const verifyCode = ref('')
const disablePassword = ref('')
const showDisable = ref(false)
const loading = ref(false)
const error = ref('')
const success = ref('')
const copiedBackup = ref(false)

async function loadStatus() {
  try {
    const res = await apiClient.getMFAStatus()
    mfaEnabled.value = !!res.data?.mfa_enabled
  } catch (e) {
    mfaEnabled.value = false
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
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la configuration MFA'
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
    await loadStatus()
  } catch (e) {
    error.value = e.response?.data?.error || 'Code invalide'
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
  } catch (e) {
    error.value = e.response?.data?.error || 'Erreur lors de la désactivation'
  } finally {
    loading.value = false
  }
}

async function copyBackupCodes() {
  if (!setup.value.backup_codes?.length) return
  await navigator.clipboard.writeText(setup.value.backup_codes.join('\n'))
  copiedBackup.value = true
  setTimeout(() => {
    copiedBackup.value = false
  }, 1500)
}

onMounted(loadStatus)
</script>
