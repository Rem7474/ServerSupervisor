<template>
  <div
    v-if="aptStatus"
    class="card"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        APT - Mises à jour système
      </h3>
      <div
        v-if="canRunApt"
        class="btn-group btn-group-sm"
      >
        <button
          type="button"
          class="btn btn-outline-secondary"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'update')"
        >
          <span
            v-if="aptCmdLoading === 'update'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt update
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'upgrade')"
        >
          <span
            v-if="aptCmdLoading === 'upgrade'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt upgrade
        </button>
        <button
          type="button"
          class="btn btn-outline-danger"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'dist-upgrade')"
        >
          <span
            v-if="aptCmdLoading === 'dist-upgrade'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt dist-upgrade
        </button>
      </div>
      <span
        v-else
        class="text-secondary small"
      >Mode lecture seule</span>
    </div>
    <div class="card-body">
      <div class="row row-cards">
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div
                class="h2 mb-0"
                :class="(aptStatus.pending_packages ?? 0) > 0 ? 'text-yellow' : 'text-green'"
              >
                {{ aptStatus.pending_packages }}
              </div>
              <div class="text-secondary small">
                Paquets en attente
              </div>
            </div>
          </div>
        </div>
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="h2 mb-0 text-red">
                {{ aptStatus.security_updates }}
              </div>
              <div class="text-secondary small">
                Mises à jour sécurité
              </div>
            </div>
          </div>
        </div>
        <div class="col-md-4">
          <div class="card card-sm">
            <div class="card-body text-center">
              <div class="text-secondary small">
                Dernier apt update
              </div>
              <div class="fw-semibold">
                {{ formatDate(aptStatus.last_update) }}
              </div>
              <div class="text-secondary small mt-2">
                Dernier upgrade
              </div>
              <div class="fw-semibold">
                {{ formatDate(lastUpgradeDate) }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="aptStatus.cve_list"
        class="mt-3"
      >
        <CVEList
          :cve-list="(aptStatus.cve_list as any)"
          :show-max-severity="true"
          :always-expanded="true"
        />
      </div>
    </div>
  </div>
  <div
    v-else
    class="card"
  >
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title">
        APT - Mises à jour système
      </h3>
      <div
        v-if="canRunApt"
        class="btn-group btn-group-sm"
      >
        <button
          type="button"
          class="btn btn-outline-secondary"
          :disabled="!!aptCmdLoading"
          @click="$emit('run-apt-command', 'update')"
        >
          <span
            v-if="aptCmdLoading === 'update'"
            class="spinner-border spinner-border-sm me-1"
          />
          apt update
        </button>
      </div>
      <span
        v-else
        class="text-secondary small"
      >Mode lecture seule</span>
    </div>
    <div class="card-body text-secondary small">
      Aucune donnée APT disponible. Lancez <strong>apt update</strong> pour initialiser.
    </div>
  </div>

  <!-- Unattended-upgrades card -->
  <div class="card mt-3">
    <div class="card-header d-flex align-items-center justify-content-between">
      <h3 class="card-title mb-0">
        Mises à jour automatiques
      </h3>
      <div
        v-if="uuStatus"
        class="d-flex align-items-center gap-2"
      >
        <span
          v-if="!uuStatus.installed"
          class="badge bg-secondary-lt text-secondary"
        >Non installé</span>
        <template v-else>
          <span
            class="badge"
            :class="uuStatus.enabled ? 'bg-green-lt text-green' : 'bg-secondary-lt text-secondary'"
          >{{ uuStatus.enabled ? 'Activé' : 'Désactivé' }}</span>
          <span
            v-if="uuStatus.reboot_required"
            class="badge bg-orange-lt text-orange"
          >Redémarrage requis</span>
        </template>
      </div>
    </div>
    <div class="card-body">
      <!-- Not installed -->
      <div
        v-if="uuStatus && !uuStatus.installed"
        class="d-flex align-items-center gap-3"
      >
        <span class="text-secondary">unattended-upgrades n'est pas installé sur cet hôte.</span>
        <button
          v-if="canRunApt"
          type="button"
          class="btn btn-sm btn-primary"
          :disabled="uuLoading === 'install'"
          @click="$emit('uu-install')"
        >
          <span
            v-if="uuLoading === 'install'"
            class="spinner-border spinner-border-sm me-1"
          />
          Installer
        </button>
      </div>

      <!-- Installed -->
      <div v-else-if="uuStatus && uuStatus.installed">
        <!-- Last run info -->
        <div
          v-if="uuStatus.last_run_at"
          class="mb-3 text-secondary small"
        >
          Dernière exécution : <strong>{{ formatDate(uuStatus.last_run_at) }}</strong>
          — {{ uuStatus.last_run_packages }} paquet(s) installé(s)
        </div>
        <div
          v-else
          class="mb-3 text-secondary small"
        >
          Aucune exécution enregistrée.
        </div>

        <!-- Config form -->
        <div
          v-if="canRunApt && uuForm"
          class="row g-3 mb-3"
        >
          <!-- Enable toggle -->
          <div class="col-12">
            <label class="form-check form-switch">
              <input
                v-model="uuForm.enabled"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label fw-semibold">Activé</span>
            </label>
          </div>
          <!-- Config options (only meaningful when enabled) -->
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="uuForm.config.security_only"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label">Sécurité uniquement</span>
            </label>
          </div>
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="uuForm.config.remove_unused"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label">Supprimer les dépendances inutilisées</span>
            </label>
          </div>
          <div class="col-md-6">
            <label class="form-check">
              <input
                v-model="uuForm.config.auto_reboot"
                class="form-check-input"
                type="checkbox"
              >
              <span class="form-check-label">Redémarrage automatique</span>
            </label>
          </div>
          <div
            v-if="uuForm.config.auto_reboot"
            class="col-md-6"
          >
            <label class="form-label small mb-1">Heure de redémarrage</label>
            <input
              v-model="uuForm.config.auto_reboot_time"
              type="time"
              class="form-control form-control-sm"
              style="max-width:120px"
            >
          </div>
          <!-- Actions -->
          <div class="col-12 d-flex gap-2">
            <button
              type="button"
              class="btn btn-sm btn-primary"
              :disabled="uuLoading === 'configure'"
              @click="$emit('uu-configure', uuForm)"
            >
              <span
                v-if="uuLoading === 'configure'"
                class="spinner-border spinner-border-sm me-1"
              />
              Enregistrer
            </button>
            <button
              type="button"
              class="btn btn-sm btn-outline-secondary"
              :disabled="!!uuLoading"
              @click="$emit('uu-run-now')"
            >
              <span
                v-if="uuLoading === 'run'"
                class="spinner-border spinner-border-sm me-1"
              />
              Lancer maintenant
            </button>
          </div>
        </div>

        <!-- Run history -->
        <div v-if="uuRuns && uuRuns.length > 0">
          <div class="fw-semibold small mb-2">
            Historique des upgrades automatiques
          </div>
          <div class="table-responsive">
            <table class="table table-sm table-vcenter">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Paquets</th>
                  <th>Statut</th>
                  <th>Logs</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="run in uuRuns"
                  :key="run.run_at"
                >
                  <td class="text-nowrap small">
                    {{ formatDate(run.run_at) }}
                  </td>
                  <td class="small">
                    <span
                      v-if="run.packages && run.packages.length"
                      :title="run.packages.join(', ')"
                    >
                      {{ run.packages.slice(0, 3).join(', ') }}
                      <span v-if="run.packages.length > 3">… (+{{ run.packages.length - 3 }})</span>
                    </span>
                    <span
                      v-else
                      class="text-secondary"
                    >Aucun</span>
                  </td>
                  <td>
                    <span
                      class="badge"
                      :class="run.had_error ? 'bg-red-lt text-red' : 'bg-green-lt text-green'"
                    >{{ run.had_error ? 'Erreur' : 'OK' }}</span>
                  </td>
                  <td>
                    <button
                      type="button"
                      class="btn btn-sm btn-ghost-secondary"
                      title="Voir les logs"
                      :disabled="!run.log_snippet"
                      @click="$emit('uu-log', run)"
                    >
                      <IconList
                        :size="16"
                        class="icon icon-sm"
                      />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div
          v-else-if="uuRuns"
          class="text-secondary small"
        >
          Aucun upgrade automatique enregistré.
        </div>
      </div>

      <!-- No data yet -->
      <div
        v-else
        class="d-flex align-items-center gap-3 text-secondary small"
      >
        <span>En attente des données de l'agent…</span>
        <button
          v-if="canRunApt"
          type="button"
          class="btn btn-sm btn-outline-primary"
          :disabled="uuLoading === 'install'"
          @click="$emit('uu-install')"
        >
          <span
            v-if="uuLoading === 'install'"
            class="spinner-border spinner-border-sm me-1"
          />
          Installer
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { IconList } from '@tabler/icons-vue'
import dayjs from '../../utils/dayjs'
import CVEList from '../apt/CVEList.vue'

interface AptStatus {
  last_upgrade?: string
  pending_packages?: number
  last_update?: string
  cve_list?: string | unknown[]
  [key: string]: unknown
}

interface UUStatus {
  last_run_at?: string
  [key: string]: unknown
}

interface UURun {
  run_at: string
  packages?: string[]
  had_error?: boolean
  log_snippet?: string
  [key: string]: unknown
}

interface UUConfig {
  security_only?: boolean
  remove_unused?: boolean
  auto_reboot?: boolean
  auto_reboot_time?: string
}

interface UUForm {
  config: UUConfig
  [key: string]: unknown
}

defineEmits<{
  (e: 'run-apt-command', action: string): void
  (e: 'uu-install'): void
  (e: 'uu-configure', form: UUForm): void
  (e: 'uu-run-now'): void
  (e: 'uu-log', run: UURun): void
}>()

const props = withDefaults(defineProps<{
  aptStatus?: AptStatus | null
  canRunApt?: boolean
  aptCmdLoading?: string
  uuStatus?: UUStatus | null
  uuRuns?: UURun[] | null
  uuForm?: UUForm | null
  uuLoading?: string
}>(), {
  aptStatus: null,
  canRunApt: false,
  aptCmdLoading: '',
  uuStatus: null,
  uuRuns: null,
  uuForm: null,
  uuLoading: '',
})

const lastUpgradeDate = computed(() => {
  const aptUpgrade = props.aptStatus?.last_upgrade
  if (aptUpgrade && aptUpgrade !== '0001-01-01T00:00:00Z') return aptUpgrade
  const uuUpgrade = props.uuStatus?.last_run_at
  if (uuUpgrade && uuUpgrade !== '0001-01-01T00:00:00Z') return uuUpgrade
  return null
})

function formatDate(date: string | null | undefined): string {
  if (!date || date === '0001-01-01T00:00:00Z') return 'Jamais'
  return dayjs.utc(date).local().fromNow()
}
</script>

