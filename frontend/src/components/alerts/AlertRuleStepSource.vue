<template>
  <div>
    <div class="mb-3">
      <label class="form-label required">Nom</label>
      <input
        v-model="form.name"
        type="text"
        class="form-control"
        placeholder="Ex: CPU élevé sur serveur web"
      >
    </div>

    <div class="mb-3">
      <label class="form-label required">Source des données</label>
      <div
        class="btn-group w-100"
        role="group"
        aria-label="Source type"
      >
        <button
          type="button"
          class="btn"
          :class="form.source_type === 'agent' ? 'btn-primary' : 'btn-outline-primary'"
          @click="emit('set-source-type', 'agent')"
        >
          Agent
        </button>
        <button
          type="button"
          class="btn"
          :class="form.source_type === 'proxmox' ? 'btn-primary' : 'btn-outline-primary'"
          @click="emit('set-source-type', 'proxmox')"
        >
          Proxmox
        </button>
        <button
          type="button"
          class="btn"
          :class="form.source_type === 'synthetic' ? 'btn-primary' : 'btn-outline-primary'"
          @click="emit('set-source-type', 'synthetic')"
        >
          Synthétique
        </button>
        <button
          type="button"
          class="btn"
          :class="form.source_type === 'docker' ? 'btn-primary' : 'btn-outline-primary'"
          @click="emit('set-source-type', 'docker')"
        >
          🐳 Docker
        </button>
      </div>
    </div>

    <div
      v-if="form.source_type === 'agent'"
      class="mb-3"
    >
      <label class="form-label">Hôte cible</label>
      <select
        v-model="form.host_id"
        class="form-select"
        :disabled="!metricSupportsHostFilter"
      >
        <option :value="null">
          Tous les hôtes
        </option>
        <option
          v-for="host in hosts"
          :key="host.id"
          :value="host.id"
        >
          {{ host.name }}
        </option>
      </select>
      <small
        v-if="!metricSupportsHostFilter"
        :id="`host-filter-hint-${rule?.id || 'new'}`"
        class="form-hint"
      >Cette métrique est globale et n'est pas liée à un hôte.</small>
    </div>


    <div class="mb-2 fw-semibold">
      Choisissez une métrique à surveiller
    </div>
    <div
      v-if="capabilitiesLoading"
      class="alert alert-info py-2 small mb-2"
    >
      Chargement des métriques...
    </div>
    <div
      v-else-if="capabilitiesError"
      class="alert alert-warning py-2 small mb-2"
    >
      {{ capabilitiesError }}
    </div>
    <div
      v-if="form.host_id && hostMetricsLoading"
      class="alert alert-info py-2 small mb-2"
    >
      Chargement des métriques pour cet hôte...
    </div>
    <div
      v-else-if="form.host_id && hostMetricsError"
      class="alert alert-warning py-2 small mb-2"
    >
      {{ hostMetricsError }}
    </div>
    <div
      v-else-if="form.host_id && hostMetrics?.metrics && hostMetrics.metrics.length < (capabilities?.metrics?.length || 0)"
      class="alert alert-info py-2 small mb-2"
    >
      ℹ️ Cet hôte dispose de {{ hostMetrics.metrics.length }} métrique(s) — certains collecteurs peuvent ne pas être actifs.
    </div>
    <div class="metric-grid">
      <button
        v-for="metric in metricCards"
        :key="metric.value"
        type="button"
        class="metric-card"
        :class="{ selected: form.metric === metric.value }"
        @click="emit('select-metric', metric.value)"
      >
        <span class="metric-icon">{{ metric.icon }}</span>
        <span class="metric-label">{{ metric.label }}</span>
      </button>
    </div>
    <div
      v-if="isProxmoxMetric(form.metric)"
      class="row g-2 mt-2"
    >
      <div class="col-md-4">
        <label class="form-label">Scope Proxmox</label>
        <select
          v-model="form.proxmox_scope.scope_mode"
          class="form-select"
        >
          <option value="global">
            Global
          </option>
          <option
            v-if="!metricAllowsGuestScope"
            value="connection"
          >
            Connexion
          </option>
          <option
            v-if="!metricAllowsGuestScope"
            value="node"
          >
            Nœud
          </option>
          <option
            v-if="metricAllowsGuestScope"
            value="guest"
          >
            VM/LXC
          </option>
          <option
            v-if="metricAllowsStorageScope"
            value="storage"
          >
            Stockage
          </option>
          <option
            v-if="metricAllowsDiskScope"
            value="disk"
          >
            Disque physique
          </option>
        </select>
      </div>
      <div
        v-if="!metricAllowsGuestScope && form.proxmox_scope.scope_mode === 'connection'"
        class="col-md-8"
      >
        <label class="form-label">Connexion</label>
        <select
          v-model="form.proxmox_scope.connection_id"
          class="form-select"
        >
          <option value="">
            Sélectionner...
          </option>
          <option
            v-for="opt in proxmoxConnections"
            :key="opt.id"
            :value="opt.id"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>
      <div
        v-if="!metricAllowsGuestScope && form.proxmox_scope.scope_mode === 'node'"
        class="col-md-8"
      >
        <label class="form-label">Nœud</label>
        <select
          v-model="form.proxmox_scope.node_id"
          class="form-select"
        >
          <option value="">
            Sélectionner...
          </option>
          <option
            v-for="opt in proxmoxNodes"
            :key="opt.id"
            :value="opt.id"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>
      <div
        v-if="metricAllowsGuestScope && form.proxmox_scope.scope_mode === 'guest'"
        class="col-md-8"
      >
        <label class="form-label">VM/LXC</label>
        <select
          v-model="form.proxmox_scope.guest_id"
          class="form-select"
        >
          <option value="">
            Sélectionner...
          </option>
          <option
            v-for="opt in proxmoxGuests"
            :key="opt.id"
            :value="opt.id"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>
      <div
        v-if="metricAllowsStorageScope && form.proxmox_scope.scope_mode === 'storage'"
        class="col-md-8"
      >
        <label class="form-label">Stockage</label>
        <select
          v-model="form.proxmox_scope.storage_id"
          class="form-select"
        >
          <option value="">
            Sélectionner...
          </option>
          <option
            v-for="opt in proxmoxStorages"
            :key="opt.id"
            :value="opt.id"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>
      <div
        v-if="metricAllowsDiskScope && form.proxmox_scope.scope_mode === 'disk'"
        class="col-md-8"
      >
        <label class="form-label">Disque physique</label>
        <select
          v-model="form.proxmox_scope.disk_id"
          class="form-select"
        >
          <option value="">
            Sélectionner...
          </option>
          <option
            v-for="opt in proxmoxDisks"
            :key="opt.id"
            :value="opt.id"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>
      <div class="col-12">
        <small
          :id="`proxmox-scope-hint-${rule?.id || 'new'}`"
          class="form-hint d-block"
        >
          Connexion = toute l'instance Proxmox liée. Nœud = un hôte Proxmox précis à l'intérieur de cette connexion.
        </small>
      </div>
    </div>
    <div
      v-if="isDockerMetric(form.metric)"
      class="row g-2 mt-2"
    >
      <div class="col-md-4">
        <label class="form-label required">Hôte</label>
        <select
          v-model="form.docker_scope.host_id"
          class="form-select"
          @change="onDockerHostChange"
        >
          <option value="">
            Sélectionner un hôte...
          </option>
          <option
            v-for="h in dockerHosts"
            :key="h.host_id"
            :value="h.host_id"
          >
            {{ h.host_name }}
          </option>
        </select>
        <div
          v-if="dockerCapabilitiesLoading"
          class="form-hint"
        >
          Chargement...
        </div>
      </div>
      <!-- Scope selector: shown for docker_container_state, hidden for docker_compose_degraded_services (forced compose_project) -->
      <div
        v-if="form.metric !== 'docker_compose_degraded_services'"
        class="col-md-4"
      >
        <label class="form-label">Scope</label>
        <select
          v-model="form.docker_scope.scope_mode"
          class="form-select"
          @change="onDockerScopeModeChange"
        >
          <option value="host">
            Tous les containers
          </option>
          <option value="container">
            Container spécifique
          </option>
        </select>
      </div>
      <div
        v-if="form.docker_scope.scope_mode === 'container' && form.docker_scope.host_id"
        class="col-md-4"
      >
        <label class="form-label required">Container</label>
        <select
          v-model="form.docker_scope.container_id"
          class="form-select"
        >
          <option value="">
            Sélectionner...
          </option>
          <option
            v-for="c in selectedDockerHost?.containers || []"
            :key="c.id"
            :value="c.id"
          >
            {{ c.name }} <template v-if="c.state !== 'running'">({{ c.state }})</template>
          </option>
        </select>
      </div>
      <div
        v-if="form.metric === 'docker_compose_degraded_services' && form.docker_scope.host_id"
        class="col-md-4"
      >
        <label class="form-label required">Projet Compose</label>
        <select
          v-model="form.docker_scope.project_name"
          class="form-select"
        >
          <option value="">
            Sélectionner...
          </option>
          <option
            v-for="p in selectedDockerHost?.projects || []"
            :key="p.name"
            :value="p.name"
          >
            {{ p.name }} ({{ p.services.length }} service{{ p.services.length > 1 ? 's' : '' }})
          </option>
        </select>
      </div>
      <div
        v-if="form.metric === 'docker_container_state' && form.docker_scope.scope_mode === 'host'"
        class="col-12"
      >
        <small class="form-hint">Un incident sera créé par container dont l'état correspond à la condition définie à l'étape suivante.</small>
      </div>
      <div
        v-if="form.metric === 'docker_compose_degraded_services'"
        class="col-12"
      >
        <small class="form-hint">Compare les services déclarés dans le compose.yml au nombre de services avec au moins un container running. La valeur est le nombre de services dégradés.</small>
      </div>
    </div>

    <div
      v-if="form.metric === 'proxmox_storage_percent'"
      class="text-secondary small mt-2"
    >
      Cette métrique est globale Proxmox: elle surveille le stockage le plus rempli parmi les stockages actifs.
    </div>
    <div
      v-else-if="form.metric === 'disk_smart_status'"
      class="text-secondary small mt-2"
    >
      Utilisez typiquement un seuil > 0.5 pour déclencher quand au moins un disque est en état SMART FAILED.
    </div>
    <div
      v-else-if="form.metric === 'docker_container_not_running'"
      class="text-secondary small mt-2"
    >
      Valeur 1 = container non running, 0 = running. Utilisez &gt; 0.5 comme seuil d'alerte.
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AlertRuleFormData } from '../../composables/useAlertRuleForm'
import { getAlertMetricMeta } from '../../utils/alertMetrics'

interface ScopeOption { id: string; label: string }
interface MetricCard { value: string; label: string; icon: string }
interface HostOption { id: string; name: string }
interface HostMetrics { metrics?: Array<{ metric: string; label: string; icon?: string }> }
interface Capabilities { metrics?: Array<{ metric: string }> }

interface DockerContainer { id: string; name: string; image: string; state: string }
interface DockerProject { name: string; services: string[] }
interface DockerHostOption {
  host_id: string
  host_name: string
  containers: DockerContainer[]
  projects: DockerProject[]
}

const props = defineProps<{
  form: AlertRuleFormData
  rule?: { id?: number | string } | null
  hosts: HostOption[]
  capabilities?: Capabilities | null
  capabilitiesLoading?: boolean
  capabilitiesError?: string
  hostMetrics?: HostMetrics | null
  hostMetricsLoading?: boolean
  hostMetricsError?: string
  metricCards: MetricCard[]
  metricSupportsHostFilter: boolean
  metricAllowsGuestScope: boolean
  metricAllowsStorageScope: boolean
  metricAllowsDiskScope: boolean
  proxmoxConnections: ScopeOption[]
  proxmoxNodes: ScopeOption[]
  proxmoxStorages: ScopeOption[]
  proxmoxGuests: ScopeOption[]
  proxmoxDisks: ScopeOption[]
  dockerHosts: DockerHostOption[]
  dockerCapabilitiesLoading?: boolean
}>()

const emit = defineEmits<{
  (e: 'select-metric', value: string): void
  (e: 'set-source-type', value: 'agent' | 'proxmox' | 'synthetic' | 'docker'): void
}>()

function isProxmoxMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'proxmox'
}

function isDockerMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'docker'
}

const selectedDockerHost = computed(() =>
  props.dockerHosts.find(h => h.host_id === props.form.docker_scope?.host_id) ?? null
)

function onDockerHostChange(): void {
  props.form.docker_scope.container_id = ''
  props.form.docker_scope.project_name = ''
}

function onDockerScopeModeChange(): void {
  props.form.docker_scope.container_id = ''
  props.form.docker_scope.project_name = ''
}
</script>

<style scoped>
.metric-grid {
  display: grid;
  gap: 0.8rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.metric-card {
  align-items: center;
  background: var(--tblr-bg-surface, #ffffff);
  border: 1px solid var(--tblr-border-color, #d9e2ee);
  border-radius: 0.8rem;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  justify-content: center;
  min-height: 90px;
  padding: 0.8rem;
  transition: all 0.15s ease;
}

.metric-card:hover {
  border-color: var(--ss-accent-blue-soft);
  box-shadow: 0 2px 10px rgba(66, 132, 245, 0.18);
}

.metric-card.selected {
  background: linear-gradient(160deg, rgba(45, 140, 255, 0.14) 0%, rgba(45, 140, 255, 0.06) 100%);
  border-color: var(--ss-accent-blue);
  box-shadow: inset 0 0 0 1px var(--ss-accent-blue);
}

.metric-icon {
  font-size: 1.2rem;
  line-height: 1;
}

.metric-label {
  color: var(--tblr-body-color, #1f2d3d);
  font-size: 0.92rem;
  font-weight: 600;
}

[data-bs-theme='dark'] .metric-card {
  background: var(--ss-chip-idle-bg);
  border-color: var(--ss-chip-idle-border);
}

[data-bs-theme='dark'] .metric-card.selected {
  background: linear-gradient(160deg, rgba(33, 118, 210, 0.34) 0%, rgba(18, 79, 150, 0.2) 100%);
}

@media (max-width: 768px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
