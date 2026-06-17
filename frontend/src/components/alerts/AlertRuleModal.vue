<template>
  <div v-if="visible">
    <div
      ref="modalRef"
      class="modal modal-blur fade show"
      style="display: block"
      tabindex="-1"
      role="dialog"
      aria-modal="true"
    >
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">
              {{ rule ? 'Modifier l\'alerte' : 'Nouvelle alerte' }}
            </h5>
            <button
              type="button"
              class="btn-close"
              @click="close"
            />
          </div>
          <div class="modal-body">
            <div class="alert-steps mb-4">
              <div
                class="step-chip"
                :class="{ active: step === 1, done: step > 1 }"
              >
                <span class="step-chip-index">1</span>
                <span>Quoi surveiller</span>
              </div>
              <div
                class="step-chip"
                :class="{ active: step === 2, done: step > 2 }"
              >
                <span class="step-chip-index">2</span>
                <span>Conditions</span>
              </div>
              <div
                class="step-chip"
                :class="{ active: step === 3 }"
              >
                <span class="step-chip-index">3</span>
                <span>Notification</span>
              </div>
            </div>

            <div v-if="step === 1">
              <AlertRuleStepSource
                :form="form"
                :rule="rule"
                :hosts="(hosts as any)"
                :capabilities="(capabilities as any)"
                :capabilities-loading="capabilitiesLoading"
                :capabilities-error="capabilitiesError"
                :host-metrics="(hostMetrics as any)"
                :host-metrics-loading="hostMetricsLoading"
                :host-metrics-error="hostMetricsError"
                :metric-cards="metricCards"
                :metric-supports-host-filter="metricSupportsHostFilter"
                :metric-allows-guest-scope="metricAllowsGuestScope"
                :metric-allows-storage-scope="metricAllowsStorageScope"
                :metric-allows-disk-scope="metricAllowsDiskScope"
                :proxmox-connections="(proxmoxConnections as any)"
                :proxmox-nodes="(proxmoxNodes as any)"
                :proxmox-storages="(proxmoxStorages as any)"
                :proxmox-guests="(proxmoxGuests as any)"
                :proxmox-disks="(proxmoxDisks as any)"
                :docker-hosts="(dockerHosts as any)"
                :docker-capabilities-loading="dockerCapabilitiesLoading"
                @select-metric="selectMetric"
                @set-source-type="setSourceType"
              />
            </div>

            <div v-if="step === 2">
              <AlertRuleStepConditions
                :form="form"
                :rule="(rule as any)"
                :test-results="(testResults as any)"
                :has-no-data-results="hasNoDataResults"
                :can-download-test-logs="canDownloadTestLogs"
                :downloading-logs="downloadingLogs"
                :metric-unit="currentMetricUnit"
                @download-logs="downloadTestLogs"
              />
            </div>

            <div v-if="step === 3">
              <AlertRuleStepNotifications
                v-model:channel-smtp="channelSmtp"
                v-model:channel-ntfy="channelNtfy"
                v-model:channel-browser="channelBrowser"
                v-model:command-trigger-enabled="commandTriggerEnabled"
                :form="form"
                :rule="(rule as any)"
                :test-results="(testResults as any)"
                :test-error="testError"
                :browser-permission="browserPermission"
              />
            </div>
          </div>
          <div
            v-if="error"
            class="alert alert-danger mx-3 mb-0 mt-0 py-2 small"
            role="alert"
          >
            {{ error }}
          </div>
          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-secondary"
              @click="close"
            >
              Annuler
            </button>
            <button
              v-if="step > 1"
              type="button"
              class="btn btn-outline-secondary"
              :disabled="saving"
              @click="step -= 1"
            >
              ← Précédent
            </button>
            <button
              v-if="step < 3"
              type="button"
              class="btn btn-primary"
              :disabled="saving || !canProceedStep"
              @click="goNextStep"
            >
              Suivant →
            </button>
            <button
              v-if="step === 3"
              type="button"
              class="btn btn-outline-secondary"
              :disabled="testing || saving"
              @click="testAlert"
            >
              <span
                v-if="testing"
                class="spinner-border spinner-border-sm me-2"
              />
              <svg
                v-else
                class="icon me-1"
                width="16"
                height="16"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              {{ testing ? 'Test en cours...' : 'Tester' }}
            </button>
            <button
              v-if="step === 3"
              type="button"
              class="btn btn-primary"
              :disabled="saving"
              @click="submit"
            >
              <span
                v-if="saving"
                class="spinner-border spinner-border-sm me-2"
              />
              {{ rule ? 'Mettre à jour' : 'Créer' }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div
      v-if="visible"
      class="modal-backdrop fade show"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import apiClient from '../../api'
import AlertRuleStepSource from './AlertRuleStepSource.vue'
import AlertRuleStepConditions from './AlertRuleStepConditions.vue'
import AlertRuleStepNotifications from './AlertRuleStepNotifications.vue'
import { useAlertRuleForm, type AlertRuleInput } from '../../composables/useAlertRuleForm'
import type { AlertRulePayload as ApiAlertRulePayload } from '../../types/alert'
import { useModalFocusTrap } from '../../composables/useModalFocusTrap'
import { ALERT_METRIC_ORDER, getAlertMetricMeta } from '../../utils/alertMetrics'
import { getApiErrorMessage } from '../../api/client'

interface Host {
  id: string
  name?: string
}

interface MetricMeta {
  metric: string
  label: string
  icon?: string
  unit?: string
  supports_host_filter?: boolean
}

interface ProxmoxScopeItem {
  id: string | number
  name?: string
  [key: string]: unknown
}

interface Capabilities {
  metrics?: MetricMeta[]
  proxmox_scope?: {
    connections?: ProxmoxScopeItem[]
    nodes?: ProxmoxScopeItem[]
    storages?: ProxmoxScopeItem[]
    guests?: ProxmoxScopeItem[]
    disks?: ProxmoxScopeItem[]
  }
}

interface AlertRule {
  id?: string | number
  [key: string]: unknown
}

interface TestResult {
  has_data?: boolean
  [key: string]: unknown
}

interface TestResults {
  results?: TestResult[]
  [key: string]: unknown
}

interface HostMetrics {
  metrics?: MetricMeta[]
  [key: string]: unknown
}

const props = withDefaults(defineProps<{
  visible?: boolean
  rule?: AlertRule | null
  hosts?: Host[]
  capabilities?: Capabilities | null
  capabilitiesLoading?: boolean
  capabilitiesError?: string
  saving?: boolean
  error?: string
}>(), {
  visible: false,
  rule: null,
  hosts: () => [],
  capabilities: null,
  capabilitiesLoading: false,
  capabilitiesError: '',
  saving: false,
  error: '',
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: unknown): void
}>()

const modalRef = ref<HTMLElement | null>(null)
useModalFocusTrap(modalRef)

const browserPermission = ref<NotificationPermission | 'unsupported'>(
  typeof Notification !== 'undefined' ? Notification.permission : 'unsupported'
)
const step = ref(1)
const hostMetrics = ref<HostMetrics | null>(null)
const hostMetricsLoading = ref(false)
const hostMetricsError = ref('')

const metricCards = computed(() => {
  const metricSource = form.value.source_type || 'agent'

  // If a specific host is selected, use that host's filtered metrics
  if (metricSource === 'agent' && form.value.host_id && hostMetrics.value?.metrics) {
    return hostMetrics.value.metrics.map((metric) => ({
      value: metric.metric,
      label: metric.label,
      icon: metric.icon || getAlertMetricMeta(metric.metric).icon,
    }))
  }

  function matchesSource(metricName: string): boolean {
    const cat = getAlertMetricMeta(metricName).category
    if (metricSource === 'proxmox') return cat === 'proxmox'
    if (metricSource === 'synthetic') return cat === 'synthetic'
    if (metricSource === 'docker') return cat === 'docker'
    return cat === 'host'
  }

  // Otherwise, use global capabilities (all hosts)
  const fromCapabilities = props.capabilities?.metrics
  if (Array.isArray(fromCapabilities) && fromCapabilities.length > 0) {
    return fromCapabilities
      .filter((metric: MetricMeta) => matchesSource(metric.metric))
      .map((metric: MetricMeta) => ({
        value: metric.metric,
        label: metric.label,
        icon: metric.icon || getAlertMetricMeta(metric.metric).icon,
      }))
  }

  return ALERT_METRIC_ORDER
    .filter(matchesSource)
    .map((metric) => ({ value: metric, label: getAlertMetricMeta(metric).label, icon: getAlertMetricMeta(metric).icon }))
})

const proxmoxConnections = computed(() => props.capabilities?.proxmox_scope?.connections || [])
const proxmoxNodes = computed(() => props.capabilities?.proxmox_scope?.nodes || [])
const proxmoxStorages = computed(() => props.capabilities?.proxmox_scope?.storages || [])
const proxmoxGuests = computed(() => props.capabilities?.proxmox_scope?.guests || [])
const proxmoxDisks = computed(() => props.capabilities?.proxmox_scope?.disks || [])

interface DockerContainer { id: string; name: string; image: string; state: string }
interface DockerProject { name: string; services: string[] }
interface DockerHostOption { host_id: string; host_name: string; containers: DockerContainer[]; projects: DockerProject[] }

const dockerCapabilities = ref<{ hosts: DockerHostOption[] } | null>(null)
const dockerCapabilitiesLoading = ref(false)
const dockerHosts = computed<DockerHostOption[]>(() => dockerCapabilities.value?.hosts || [])

const metricMetaByKey = computed<Record<string, MetricMeta>>(() => {
  const items = props.capabilities?.metrics || []
  return Object.fromEntries(items.map((item: MetricMeta) => [item.metric, item]))
})

const metricAllowsStorageScope = computed(() => form.value.metric === 'proxmox_storage_percent')
const metricAllowsGuestScope = computed(() => form.value.metric === 'proxmox_guest_cpu_percent' || form.value.metric === 'proxmox_guest_memory_percent')
const metricAllowsDiskScope = computed(() => form.value.metric === 'proxmox_disk_failed_count' || form.value.metric === 'proxmox_disk_min_wearout_percent')

const metricSupportsHostFilter = computed(() => {
  const supports = metricMetaByKey.value?.[form.value.metric]?.supports_host_filter
  return supports !== false
})
const {
  form,
  channelSmtp,
  channelNtfy,
  channelBrowser,
  commandTriggerEnabled,
  hydrateFormFromRule,
  onMetricChange,
  buildPayload,
} = useAlertRuleForm()

const testing = ref(false)
const testResults = ref<TestResults | null>(null)
const testError = ref('')
const downloadingLogs = ref(false)

const hasNoDataResults = computed(() => testResults.value?.results?.some((result: TestResult) => !result.has_data) || false)
const canDownloadTestLogs = computed(
  () => !!testResults.value && form.value.metric === 'proxmox_auth_failures_recent'
)

const canProceedStep = computed(() => {
  if (step.value === 1) {
    const hasBase = !!form.value.metric && !!form.value.name?.trim()
    if (!hasBase) return false
    // "Tous les hôtes" is a valid selection for agent-based rules.
    if (form.value.source_type === 'agent') return true
    // Synthetic rules are global by construction — no scope to validate.
    if (form.value.source_type === 'synthetic') return true

    if (form.value.source_type === 'docker') {
      const ds = form.value.docker_scope
      if (!ds?.host_id) return false
      if (ds.scope_mode === 'container') return !!ds.container_id
      if (ds.scope_mode === 'compose_project') return !!ds.project_name
      return true
    }

    const scope = form.value.proxmox_scope || { scope_mode: 'global' }
    if (scope.scope_mode === 'connection') return !!scope.connection_id
    if (scope.scope_mode === 'node') return !!scope.node_id
    if (scope.scope_mode === 'guest') return !!scope.guest_id
    if (scope.scope_mode === 'storage') return !!scope.storage_id
    if (scope.scope_mode === 'disk') return !!scope.disk_id
    return true
  }
  if (step.value === 2) {
    if (form.value.metric === 'heartbeat_timeout') {
      return Number.isFinite(Number(form.value.threshold_crit))
    }

    return Number.isFinite(Number(form.value.threshold_warn)) && Number.isFinite(Number(form.value.threshold_crit))
  }
  return true
})

let autoTestTimer: ReturnType<typeof setTimeout> | null = null

watch(
  () => [props.visible, props.rule],
  () => {
    if (!props.visible) {
      if (autoTestTimer) clearTimeout(autoTestTimer)
      testResults.value = null
      testError.value = ''
      step.value = 1
      return
    }
    testResults.value = null
    testError.value = ''
    hydrateFormFromRule(props.rule as unknown as AlertRuleInput | null)
    step.value = 1
  },
  { immediate: true, deep: true }
)

watch(
  () => form.value.source_type,
  async (sourceType) => {
    if (sourceType !== 'docker' || dockerCapabilities.value) return
    dockerCapabilitiesLoading.value = true
    try {
      const response = await apiClient.getDockerAlertCapabilities()
      dockerCapabilities.value = response.data
    } catch {
      dockerCapabilities.value = { hosts: [] }
    } finally {
      dockerCapabilitiesLoading.value = false
    }
  }
)

watch(
  () => form.value.host_id,
  async (hostId) => {
    if (!hostId) {
      // "Tous les hôtes" selected — clear host-specific metrics
      hostMetrics.value = null
      hostMetricsLoading.value = false
      hostMetricsError.value = ''
      return
    }

    // Load metrics filtered for this specific host
    hostMetricsLoading.value = true
    hostMetricsError.value = ''
    try {
      const response = await apiClient.getHostCapabilities(hostId)
      hostMetrics.value = response.data
    } catch (_error) {
      hostMetricsError.value = 'Échec du chargement des métriques pour cet hôte'
      hostMetrics.value = null
    } finally {
      hostMetricsLoading.value = false
    }
  }
)

watch(
  () => [
    form.value.source_type,
    form.value.host_id,
    form.value.metric,
    form.value.operator,
    form.value.threshold_warn,
    form.value.threshold_crit,
    form.value.threshold_clear_warn,
    form.value.threshold_clear_crit,
    form.value.duration,
    form.value.proxmox_scope?.scope_mode,
    form.value.proxmox_scope?.connection_id,
    form.value.proxmox_scope?.node_id,
    form.value.proxmox_scope?.guest_id,
    form.value.proxmox_scope?.storage_id,
    form.value.proxmox_scope?.disk_id,
    form.value.docker_scope?.host_id,
    form.value.docker_scope?.scope_mode,
    form.value.docker_scope?.container_id,
    form.value.docker_scope?.project_name,
  ],
  () => {
    if (!props.visible) return
    if (step.value !== 2) return
    if (autoTestTimer) clearTimeout(autoTestTimer)
    autoTestTimer = setTimeout(testAlert, 600)
  }
)

watch(
  () => step.value,
  (currentStep) => {
    if (!props.visible) return
    if (currentStep !== 2) return
    if (autoTestTimer) clearTimeout(autoTestTimer)
    autoTestTimer = setTimeout(testAlert, 100)
  }
)

watch(
  () => metricSupportsHostFilter.value,
  (supportsHost) => {
    if (!supportsHost || form.value.source_type === 'proxmox') {
      form.value.host_id = null
    }
  }
)

watch(
  () => form.value.proxmox_scope?.scope_mode,
  (mode) => {
    const scope = form.value.proxmox_scope
    if (!scope) return
    if (mode !== 'connection') scope.connection_id = ''
    if (mode !== 'node') scope.node_id = ''
    if (mode !== 'guest' || !metricAllowsGuestScope.value) scope.guest_id = ''
    if (mode !== 'storage' || !metricAllowsStorageScope.value) scope.storage_id = ''
    if (mode !== 'disk' || !metricAllowsDiskScope.value) scope.disk_id = ''
  }
)

watch(
  () => props.visible,
  (visible) => {
    if (visible) {
      document.addEventListener('keydown', onKeyDown)
      return
    }
    document.removeEventListener('keydown', onKeyDown)
  },
  { immediate: true }
)

watch(
  () => metricCards.value,
  (cards) => {
    if (!Array.isArray(cards) || cards.length === 0) return
    const current = form.value.metric
    const exists = cards.some((item: { value: string }) => item.value === current)
    if (!exists && cards.length > 0) {
      // Selected metric is no longer available for this host
      form.value.metric = cards[0].value
      onMetricChange()
    }
  },
  { deep: true }
)

onUnmounted(() => {
  if (autoTestTimer) clearTimeout(autoTestTimer)
  document.removeEventListener('keydown', onKeyDown)
})

async function submit() {
  if (channelBrowser.value && typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
    browserPermission.value = await Notification.requestPermission()
  }
  emit('submit', buildPayload())
}

async function testAlert(): Promise<void> {
  if (!props.visible) return
  testing.value = true
  testResults.value = null
  testError.value = ''
  try {
    const response = await apiClient.testAlertRule(buildPayload() as unknown as ApiAlertRulePayload)
    testResults.value = response.data
  } catch (err: unknown) {
    testResults.value = null
    testError.value = getApiErrorMessage(err, 'Échec du test de la règle.')
  } finally {
    testing.value = false
  }
}

function formatDownloadTimestamp(date: Date): string {
  const iso = date.toISOString().replace(/\..+$/, '')
  return iso.replace(/[:T]/g, '-')
}

async function downloadTestLogs(): Promise<void> {
  if (downloadingLogs.value || !canDownloadTestLogs.value) return
  downloadingLogs.value = true
  try {
    const response = await apiClient.downloadAlertRuleTestLogs(buildPayload() as unknown as ApiAlertRulePayload)
    const blob = response.data instanceof Blob ? response.data : new Blob([response.data], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `proxmox-auth-failures-${formatDownloadTimestamp(new Date())}.log`
    link.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (err: unknown) {
    testError.value = getApiErrorMessage(err, 'Échec du téléchargement des logs.')
  } finally {
    downloadingLogs.value = false
  }
}

function close(): void {
  emit('close')
}

function onKeyDown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && props.visible) close()
}

function selectMetric(metric: string): void {
  form.value.metric = metric
  onMetricChange()
}

function setSourceType(sourceType: string): void {
  form.value.source_type = sourceType as 'agent' | 'proxmox' | 'synthetic' | 'docker'
  const currentCat = getAlertMetricMeta(form.value.metric).category
  const wantedCat = sourceType === 'proxmox' ? 'proxmox' : sourceType === 'synthetic' ? 'synthetic' : sourceType === 'docker' ? 'docker' : 'host'

  // Host filter only applies to agent rules.
  if (sourceType !== 'agent') {
    form.value.host_id = null
  }

  if (currentCat !== wantedCat) {
    const first = metricCards.value[0]
    if (first?.value) {
      form.value.metric = first.value
      onMetricChange()
    }
  }
}

function goNextStep(): void {
  if (!canProceedStep.value || step.value >= 3) return
  step.value += 1
}

function getMetricUnit(metric: string): string {
  return metricMetaByKey.value?.[metric]?.unit || getAlertMetricMeta(metric).unit
}

const currentMetricUnit = computed(() => getMetricUnit(form.value.metric))
</script>

<style scoped>
.alert-steps {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.step-chip {
  align-items: center;
  background: var(--tblr-bg-surface, #f3f6fa);
  border: 1px solid var(--tblr-border-color, #dbe3ec);
  border-radius: 0.6rem;
  color: var(--tblr-body-color, #4c5b6b);
  display: flex;
  font-weight: 600;
  gap: 0.5rem;
  justify-content: center;
  min-height: 44px;
  padding: 0.4rem 0.6rem;
}

.step-chip.active {
  background: var(--ss-accent-blue-bg);
  border-color: var(--ss-accent-blue);
  color: var(--ss-accent-blue-text);
}

.step-chip.done {
  background: var(--ss-success-bg);
  border-color: var(--ss-success-border);
  color: var(--ss-success-text);
}

.step-chip-index {
  background: rgba(255, 255, 255, 0.75);
  color: #1f2d3d;
  border-radius: 999px;
  display: inline-flex;
  font-size: 0.85rem;
  font-weight: 700;
  height: 24px;
  justify-content: center;
  width: 24px;
}


[data-bs-theme='dark'] .step-chip {
  background: var(--ss-chip-idle-bg);
  border-color: var(--ss-chip-idle-border);
  color: var(--ss-chip-idle-text);
}

[data-bs-theme='dark'] .step-chip.active {
  background: rgba(33, 118, 210, 0.28);
  border-color: var(--ss-accent-blue);
  color: #d2e6ff;
}

[data-bs-theme='dark'] .step-chip.done {
  background: rgba(56, 142, 99, 0.24);
  border-color: var(--ss-success-border);
  color: #c7f2da;
}

[data-bs-theme='dark'] .step-chip-index {
  background: rgba(255, 255, 255, 0.16);
  color: #d6e4fb;
}


@media (max-width: 768px) {
  .alert-steps {
    grid-template-columns: 1fr;
  }
}
</style>






