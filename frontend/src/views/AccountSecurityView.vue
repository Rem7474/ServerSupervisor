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
              <IconClock
                :size="14"
                class="setupSecondsLeft < 120 ? 'text-danger' : 'text-secondary'"
              />
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
                    <IconCopy :size="14" />
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
          <IconDeviceDesktop
            :size="18"
            class="icon me-2"
          />
          Sessions actives
        </h3>
        <button
          v-if="auth.isAuthenticated"
          type="button"
          class="btn btn-sm btn-outline-danger"
          :disabled="revokeLoading"
          @click="revokeOtherSessions"
        >
          <IconX
            :size="14"
            class="me-1"
          />
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
import { IconClock, IconCopy, IconDeviceDesktop, IconX } from '@tabler/icons-vue'
import { useAccountSecurity } from '../composables/useAccountSecurity'

const {
  auth,
  mfaEnabled,
  setupVisible,
  setup,
  verifyCode,
  disablePassword,
  showDisable,
  loading,
  error,
  success,
  copiedBackup,
  copiedSecret,
  setupSecondsLeft,
  setupProgressPct,
  formatCountdown,
  loginEvents,
  sessionsLoading,
  revokeLoading,
  revokeError,
  revokeSuccess,
  formatDateTime,
  startSetup,
  verifySetup,
  disableMFA,
  revokeOtherSessions,
  copySecret,
  copyBackupCodes,
  parseUA,
} = useAccountSecurity()
</script>
