<template>
  <div>
    <!-- Forced password change banner -->
    <div
      v-if="auth.mustChangePassword"
      class="alert alert-warning alert-dismissible mb-4"
      role="alert"
    >
      <div class="d-flex align-items-center">
        <IconAlertTriangle
          :size="24"
          class="icon alert-icon me-2"
        />
        <strong>Changement de mot de passe requis.</strong>&nbsp;Pour des raisons de sécurité, veuillez définir un nouveau mot de passe avant de continuer.
      </div>
    </div>

    <div class="page-header mb-4">
      <div class="row align-items-center">
        <div class="col-auto">
          <div class="page-pretitle">
            <router-link
              to="/"
              class="text-decoration-none"
            >
              Dashboard
            </router-link>
            <span class="text-muted mx-1">/</span>
            <span>Mon compte</span>
          </div>
          <h2 class="page-title">
            Mon compte
          </h2>
          <div class="text-secondary">
            Gérez vos informations personnelles et la sécurité de votre compte
          </div>
        </div>
      </div>
    </div>

    <!-- Tabs nav -->
    <ul class="nav nav-tabs mb-4">
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: activeTab === 'profil' }"
          @click="activeTab = 'profil'"
        >
          Profil
        </button>
      </li>
      <li class="nav-item">
        <button
          type="button"
          class="nav-link"
          :class="{ active: activeTab === 'historique' }"
          @click="switchToHistorique"
        >
          Historique
          <span
            v-if="myCommands.length"
            class="badge bg-azure-lt text-azure ms-1"
          >{{ myCommands.length }}</span>
        </button>
      </li>
      <li class="nav-item">
        <router-link
          to="/account/security"
          class="nav-link"
        >
          Connexions
        </router-link>
      </li>
    </ul>

    <!-- ── Onglet Profil ── -->
    <div v-show="activeTab === 'profil'">
      <div class="row g-4">
        <!-- Profile info card -->
        <div class="col-12 col-lg-4">
          <div class="card">
            <div class="card-body text-center py-4">
              <div
                class="avatar avatar-xl mb-3"
                style="width:64px;height:64px;font-size:1.6rem;background:var(--tblr-azure-lt);color:var(--tblr-azure);border-radius:50%;display:flex;align-items:center;justify-content:center;margin:0 auto;"
              >
                {{ auth.username?.slice(0, 2).toUpperCase() }}
              </div>
              <div class="h3 mb-1">
                {{ profile?.username || auth.username }}
              </div>
              <div class="mb-3">
                <span
                  class="badge"
                  :class="roleBadgeClass"
                >{{ roleLabel }}</span>
              </div>
              <div
                v-if="profile?.created_at"
                class="text-secondary small"
              >
                Membre depuis {{ formatDate(profile.created_at) }}
              </div>
            </div>
            <div class="card-footer text-center py-3">
              <div class="row g-3">
                <div class="col-6 border-end">
                  <div class="text-secondary small">
                    MFA
                  </div>
                  <div
                    class="fw-bold"
                    :class="profile?.mfa_enabled ? 'text-success' : 'text-secondary'"
                  >
                    {{ profile?.mfa_enabled ? 'Activé' : 'Désactivé' }}
                  </div>
                </div>
                <div class="col-6">
                  <div class="text-secondary small">
                    Statut
                  </div>
                  <div class="fw-bold text-success">
                    Actif
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- MFA card -->
          <div class="card mt-4">
            <div class="card-header">
              <h3 class="card-title">
                <IconLock
                  :size="20"
                  class="icon me-2"
                />
                Authentification à deux facteurs
              </h3>
            </div>
            <div class="card-body">
              <div class="d-flex align-items-center justify-content-between mb-3">
                <div>
                  <div class="fw-bold">
                    TOTP (Authenticator)
                  </div>
                  <div class="text-secondary small">
                    Google Authenticator, Authy, etc.
                  </div>
                </div>
                <span
                  class="badge"
                  :class="profile?.mfa_enabled ? 'bg-success-lt text-success' : 'bg-warning-lt text-warning'"
                >
                  {{ profile?.mfa_enabled ? 'Actif' : 'Inactif' }}
                </span>
              </div>
              <router-link
                to="/account/security"
                class="btn btn-outline-secondary w-100"
              >
                Gérer le MFA du compte
              </router-link>
            </div>
          </div>
        </div>

        <!-- Change password -->
        <div class="col-12 col-lg-8">
          <div class="card">
            <div class="card-header">
              <h3 class="card-title">
                <IconKey
                  :size="20"
                  class="icon me-2"
                />
                Changer le mot de passe
              </h3>
            </div>
            <div class="card-body">
              <form @submit.prevent="submitChangePassword">
                <div class="mb-3">
                  <label class="form-label required">Mot de passe actuel</label>
                  <input
                    v-model="pwForm.current"
                    type="password"
                    class="form-control"
                    :class="{ 'is-invalid': pwErrors.current }"
                    placeholder="••••••••"
                    required
                  >
                  <div
                    v-if="pwErrors.current"
                    class="invalid-feedback"
                  >
                    {{ pwErrors.current }}
                  </div>
                </div>
                <div class="mb-3">
                  <label class="form-label required">Nouveau mot de passe</label>
                  <input
                    v-model="pwForm.next"
                    type="password"
                    class="form-control"
                    :class="{ 'is-invalid': pwErrors.next }"
                    placeholder="••••••••"
                    required
                  >
                  <div
                    v-if="pwErrors.next"
                    class="invalid-feedback"
                  >
                    {{ pwErrors.next }}
                  </div>
                  <div
                    v-if="pwStrengthMeta"
                    class="mt-2"
                  >
                    <div
                      class="progress"
                      style="height: 4px;"
                    >
                      <div
                        class="progress-bar"
                        :class="pwStrengthMeta.cls"
                        :style="{ width: pwStrengthMeta.width, transition: 'width 0.3s' }"
                      />
                    </div>
                    <div class="form-hint mt-1">
                      Force : <span :class="{ 'text-danger': pwStrength <= 1, 'text-warning': pwStrength === 2, 'text-success': pwStrength >= 4 }">{{ pwStrengthMeta.label }}</span>
                    </div>
                  </div>
                  <div
                    v-else
                    class="form-hint"
                  >
                    Au moins 8 caractères.
                  </div>
                </div>
                <div class="mb-4">
                  <label class="form-label required">Confirmer le nouveau mot de passe</label>
                  <input
                    v-model="pwForm.confirm"
                    type="password"
                    class="form-control"
                    :class="{ 'is-invalid': pwErrors.confirm }"
                    placeholder="••••••••"
                    required
                  >
                  <div
                    v-if="pwErrors.confirm"
                    class="invalid-feedback"
                  >
                    {{ pwErrors.confirm }}
                  </div>
                </div>

                <div
                  v-if="pwError"
                  class="alert alert-danger mb-3"
                  role="alert"
                >
                  {{ pwError }}
                </div>
                <div
                  v-if="pwSuccess"
                  class="alert alert-success mb-3"
                  role="alert"
                >
                  {{ pwSuccess }}
                </div>

                <div class="d-flex gap-2">
                  <button
                    type="submit"
                    class="btn btn-primary"
                    :disabled="pwLoading"
                  >
                    <span
                      v-if="pwLoading"
                      class="spinner-border spinner-border-sm me-2"
                    />
                    {{ pwLoading ? 'Enregistrement...' : 'Mettre à jour le mot de passe' }}
                  </button>
                  <button
                    v-if="!auth.mustChangePassword"
                    type="button"
                    class="btn btn-outline-secondary"
                    @click="resetPwForm"
                  >
                    Annuler
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- ── Onglet Historique ── -->
    <div
      v-show="activeTab === 'historique'"
      class="side-layout"
    >
      <!-- Table principale -->
      <div class="side-main">
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between">
            <h3 class="card-title mb-0">
              <IconClock
                :size="20"
                class="icon me-2"
              />
              Activité récente
            </h3>
            <span
              v-if="myCommands.length"
              class="badge bg-azure-lt text-azure"
            >{{ myCommands.length }}</span>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Hôte</th>
                  <th>Type</th>
                  <th>Commande</th>
                  <th>Statut</th>
                  <th>Durée</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-if="cmdsLoading">
                  <td
                    colspan="7"
                    class="py-2"
                  >
                    <LoadingSkeleton
                      variant="table"
                      :lines="4"
                    />
                  </td>
                </tr>
                <tr v-else-if="!myCommands.length">
                  <td colspan="7">
                    <EmptyState title="Aucune activité récente" />
                  </td>
                </tr>
                <tr
                  v-for="cmd in myCommands"
                  :key="cmd.id"
                  :class="{ 'table-active': selectedCmd?.id === cmd.id }"
                >
                  <td class="text-secondary small">
                    {{ formatDateTime(cmd.created_at) }}
                  </td>
                  <td>
                    <router-link
                      :to="`/hosts/${cmd.host_id}`"
                      class="text-decoration-none fw-semibold"
                    >
                      {{ cmd.host_name || cmd.host_id }}
                    </router-link>
                  </td>
                  <td><span :class="moduleClass(cmd.module)">{{ moduleLabel(cmd.module) }}</span></td>
                  <td><code class="small">{{ cmdLabel(cmd) }}</code></td>
                  <td><span :class="statusClass(cmd.status)">{{ commandStatusLabel(cmd.status) }}</span></td>
                  <td class="text-secondary small">
                    {{ formatDuration(cmd.started_at, cmd.ended_at) }}
                  </td>
                  <td>
                    <button
                      type="button"
                      class="btn btn-icon btn-sm btn-ghost-secondary"
                      :disabled="!cmd.output && cmd.status === 'pending'"
                      title="Voir les logs"
                      @click="openLogViewer(cmd)"
                    >
                      <IconFileText :size="14" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <CommandLogPanel
        :command="selectedCmd"
        :show="showConsole"
        wrapper-class="side-panel"
        title="Console"
        empty-text="Aucune console active"
        @close="closeLogViewer"
        @open="showConsole = true"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { IconAlertTriangle, IconClock, IconFileText, IconKey, IconLock } from '@tabler/icons-vue'
import CommandLogPanel from '../components/host/CommandLogPanel.vue'
import EmptyState from '../components/EmptyState.vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import { commandStatusLabel } from '../utils/commandStatus'
import { useAccount } from '../composables/useAccount'

const {
  auth,
  activeTab,
  showConsole,
  profile,
  pwForm,
  pwErrors,
  pwError,
  pwSuccess,
  pwLoading,
  pwStrength,
  pwStrengthMeta,
  cmdsLoading,
  myCommands,
  selectedCmd,
  roleBadgeClass,
  roleLabel,
  formatDate,
  formatDateTime,
  formatDuration,
  cmdLabel,
  statusClass,
  moduleLabel,
  moduleClass,
  openLogViewer,
  closeLogViewer,
  resetPwForm,
  submitChangePassword,
  switchToHistorique,
} = useAccount()
</script>
