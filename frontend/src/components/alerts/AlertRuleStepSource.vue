<template>
  <div>
    <div class="mb-3">
      <label
        for="alert-source-name"
        class="form-label required"
      >Nom</label>
      <input
        id="alert-source-name"
        v-model="nameModel"
        type="text"
        class="form-control"
        placeholder="Ex: CPU élevé sur serveur web"
      >
    </div>

    <div class="mb-3">
      <div class="form-label required">
        Source des données
      </div>
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
      <label
        for="alert-source-host-id"
        class="form-label"
      >Hôte cible</label>
      <select
        id="alert-source-host-id"
        v-model="hostIdModel"
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
        <label
          for="alert-source-proxmox-scope-mode"
          class="form-label"
        >Scope Proxmox</label>
        <select
          id="alert-source-proxmox-scope-mode"
          v-model="proxmoxScopeModeModel"
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
        <label
          for="alert-source-proxmox-connection"
          class="form-label"
        >Connexion</label>
        <select
          id="alert-source-proxmox-connection"
          v-model="proxmoxConnectionIdModel"
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
        <label
          for="alert-source-proxmox-node"
          class="form-label"
        >Nœud</label>
        <select
          id="alert-source-proxmox-node"
          v-model="proxmoxNodeIdModel"
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
        <label
          for="alert-source-proxmox-guest"
          class="form-label"
        >VM/LXC</label>
        <select
          id="alert-source-proxmox-guest"
          v-model="proxmoxGuestIdModel"
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
        <label
          for="alert-source-proxmox-storage"
          class="form-label"
        >Stockage</label>
        <select
          id="alert-source-proxmox-storage"
          v-model="proxmoxStorageIdModel"
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
        <label
          for="alert-source-proxmox-disk"
          class="form-label"
        >Disque physique</label>
        <select
          id="alert-source-proxmox-disk"
          v-model="proxmoxDiskIdModel"
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
        <label
          for="alert-source-docker-host"
          class="form-label required"
        >Hôte</label>
        <select
          id="alert-source-docker-host"
          :value="form.docker_scope.host_id"
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
        <label
          for="alert-source-docker-scope-mode"
          class="form-label"
        >Scope</label>
        <select
          id="alert-source-docker-scope-mode"
          :value="form.docker_scope.scope_mode"
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
        class="col-md-8"
      >
        <div class="form-label required">
          Container(s)
        </div>
        <div
          v-if="(selectedDockerHost?.containers || []).length === 0"
          class="form-hint"
        >
          Aucun container sur cet hôte.
        </div>
        <div
          v-else
          class="border rounded p-2 d-flex flex-wrap gap-2 docker-container-checklist"
        >
          <label
            v-for="c in selectedDockerHost?.containers || []"
            :key="c.id"
            class="form-check form-check-inline mb-0"
          >
            <input
              type="checkbox"
              class="form-check-input"
              :checked="form.docker_scope.container_ids.includes(c.id)"
              @change="toggleContainer(c.id, ($event.target as HTMLInputElement).checked)"
            >
            <span class="form-check-label">
              {{ c.name }} <template v-if="c.state !== 'running'">
                ({{ c.state }})
              </template>
            </span>
          </label>
        </div>
        <div
          v-if="form.docker_scope.container_ids.length === 0"
          class="text-warning small mt-1"
        >
          Sélectionnez au moins un container.
        </div>
      </div>
      <div
        v-if="form.metric === 'docker_compose_degraded_services' && form.docker_scope.host_id"
        class="col-md-4"
      >
        <label
          for="alert-source-docker-project"
          class="form-label required"
        >Projet Compose</label>
        <select
          id="alert-source-docker-project"
          v-model="dockerProjectNameModel"
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
import type { AlertRuleFormData, DockerScope } from '../../composables/useAlertRuleForm'
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
  (e: 'update:form', value: AlertRuleFormData): void
}>()

// ── Shared form-field emit helpers ───────────────────────────────────
// The `form` prop is owned by the parent (AlertRuleModal, via
// useAlertRuleForm). This component never mutates it in place — every
// field write emits a whole-object replacement for the parent to apply
// (bound as `v-model:form` there), which also keeps sibling reads (e.g.
// AlertRuleStepConditions) consistent.

function updateForm<K extends keyof AlertRuleFormData>(key: K, value: AlertRuleFormData[K]): void {
  emit('update:form', { ...props.form, [key]: value })
}

function updateProxmoxScope<K extends keyof AlertRuleFormData['proxmox_scope']>(
  key: K,
  value: AlertRuleFormData['proxmox_scope'][K],
): void {
  emit('update:form', { ...props.form, proxmox_scope: { ...props.form.proxmox_scope, [key]: value } })
}

function updateDockerScope<K extends keyof DockerScope>(key: K, value: DockerScope[K]): void {
  emit('update:form', { ...props.form, docker_scope: { ...props.form.docker_scope, [key]: value } })
}

function fieldModel<K extends keyof AlertRuleFormData>(key: K) {
  return computed<AlertRuleFormData[K]>({
    get: () => props.form[key],
    set: (value) => updateForm(key, value),
  })
}

function proxmoxScopeModel<K extends keyof AlertRuleFormData['proxmox_scope']>(key: K) {
  return computed<AlertRuleFormData['proxmox_scope'][K]>({
    get: () => props.form.proxmox_scope[key],
    set: (value) => updateProxmoxScope(key, value),
  })
}

function dockerScopeModel<K extends keyof DockerScope>(key: K) {
  return computed<DockerScope[K]>({
    get: () => props.form.docker_scope[key],
    set: (value) => updateDockerScope(key, value),
  })
}

const nameModel = fieldModel('name')
const hostIdModel = fieldModel('host_id')
const proxmoxScopeModeModel = proxmoxScopeModel('scope_mode')
const proxmoxConnectionIdModel = proxmoxScopeModel('connection_id')
const proxmoxNodeIdModel = proxmoxScopeModel('node_id')
const proxmoxGuestIdModel = proxmoxScopeModel('guest_id')
const proxmoxStorageIdModel = proxmoxScopeModel('storage_id')
const proxmoxDiskIdModel = proxmoxScopeModel('disk_id')
const dockerProjectNameModel = dockerScopeModel('project_name')

function isProxmoxMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'proxmox'
}

function isDockerMetric(metric: string): boolean {
  return getAlertMetricMeta(metric).category === 'docker'
}

const selectedDockerHost = computed(() =>
  props.dockerHosts.find(h => h.host_id === props.form.docker_scope?.host_id) ?? null
)

// Changing the host or the scope mode resets the container/project
// selection in the same atomic update as the field itself — reading
// `event.target.value` directly (rather than the prop, which may not have
// propagated back down yet at this point in the native change handler)
// keeps the whole docker_scope patch consistent in one emit.
function onDockerHostChange(event: Event): void {
  const hostId = (event.target as HTMLSelectElement).value
  emit('update:form', {
    ...props.form,
    docker_scope: {
      ...props.form.docker_scope,
      host_id: hostId,
      container_id: '',
      container_ids: [],
      project_name: '',
    },
  })
}

function onDockerScopeModeChange(event: Event): void {
  const scopeMode = (event.target as HTMLSelectElement).value
  emit('update:form', {
    ...props.form,
    docker_scope: {
      ...props.form.docker_scope,
      scope_mode: scopeMode,
      container_id: '',
      container_ids: [],
      project_name: '',
    },
  })
}

function toggleContainer(containerId: string, checked: boolean): void {
  const current = props.form.docker_scope.container_ids
  const next = checked
    ? (current.includes(containerId) ? current : [...current, containerId])
    : current.filter((id) => id !== containerId)
  updateDockerScope('container_ids', next)
}
</script>

<style scoped>
.metric-grid {
  display: grid;
  gap: 0.8rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

/* This app is dark-only (data-bs-theme="dark" fixed in index.html), so a
   single un-scoped rule set is the actual styling — no [data-bs-theme='dark']
   selector needed (a light/dark split here left the two states out of sync:
   the "light" values below were dead, always shadowed by a
   higher-specificity dark override with different colors — same pattern
   already cleaned up in AlertRuleModal's step chips). */
.metric-card {
  align-items: center;
  background: var(--ss-chip-idle-bg);
  border: 1px solid var(--ss-chip-idle-border);
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
  background: linear-gradient(160deg, rgba(33, 118, 210, 0.34) 0%, rgba(18, 79, 150, 0.2) 100%);
  border-color: var(--ss-accent-blue);
  box-shadow: inset 0 0 0 1px var(--ss-accent-blue);
}

.metric-icon {
  font-size: 1.2rem;
  line-height: 1;
}

.docker-container-checklist {
  max-height: 12rem;
  overflow-y: auto;
}

.metric-label {
  color: var(--tblr-body-color);
  font-size: 0.92rem;
  font-weight: 600;
}

@media (max-width: 768px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
