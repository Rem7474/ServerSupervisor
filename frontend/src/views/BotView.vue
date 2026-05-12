<template>
  <div>
    <div class="threats-topbar mb-3">
      <div class="d-flex align-items-center gap-2">
        <span class="fw-semibold">Menaces web</span>
      </div>
      <div class="d-flex align-items-center gap-2 flex-wrap">
        <span class="small text-secondary">Période :</span>
        <button
          v-for="p in periodOptions"
          :key="p.value"
          class="btn btn-sm"
          :class="period === p.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="setPeriod(p.value)"
        >
          {{ p.label }}
        </button>
      </div>
    </div>

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
          <span>Menaces web</span>
        </div>
        <h2 class="page-title">
          Menaces web
        </h2>
        <div class="text-secondary">
          IPs suspectes, chemins scannés, corrélation multi-hôtes et chronologie détaillée
        </div>
      </div>
    </div>

    <div class="card mb-4">
      <div class="card-body d-flex flex-wrap gap-2 align-items-end threats-filters">
        <div class="threats-filter-field">
          <label class="form-label mb-1">Source</label>
          <select
            v-model="source"
            class="form-select form-select-sm"
            style="min-width: 9rem;"
          >
            <option value="">
              Toutes
            </option>
            <option value="npm">
              npm
            </option>
            <option value="nginx">
              nginx
            </option>
            <option value="apache">
              apache
            </option>
            <option value="caddy">
              caddy
            </option>
          </select>
        </div>
        <div class="threats-filter-field">
          <label class="form-label mb-1">Hôte technique (ID)</label>
          <input
            v-model.trim="hostId"
            class="form-control form-control-sm"
            placeholder="(optionnel)"
            style="min-width: 14rem;"
          >
        </div>
        <button
          class="btn btn-primary btn-sm threats-refresh-btn"
          :disabled="loading"
          @click="loadThreats"
        >
          <span
            v-if="loading"
            class="spinner-border spinner-border-sm me-1"
          />
          Rafraîchir
        </button>
      </div>
    </div>

    <!-- Toast feedback ban/unban -->
    <div
      v-if="actionFeedback"
      class="alert alert-dismissible mb-3"
      :class="actionFeedback.type === 'success' ? 'alert-success' : 'alert-danger'"
      role="alert"
    >
      {{ actionFeedback.message }}
      <button
        type="button"
        class="btn-close"
        aria-label="Fermer"
        @click="actionFeedback = null"
      />
    </div>

    <!-- Squelette chargement -->
    <template v-if="loading">
      <LoadingSkeleton variant="kpi" :lines="4" class="mb-4" />
      <div class="row row-cards">
        <div class="col-lg-7">
          <div class="card h-100">
            <div class="card-body">
              <LoadingSkeleton variant="table" :lines="6" />
            </div>
          </div>
        </div>
        <div class="col-lg-5">
          <div class="card h-100">
            <div class="card-body">
              <LoadingSkeleton variant="table" :lines="6" />
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Contenu réel -->
    <template v-else>

    <div class="row row-cards mb-4">
      <div class="col-12 col-sm-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Requêtes suspectes
            </div>
            <div class="h2 mb-0 text-orange">
              {{ threats.suspicious_requests || 0 }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-12 col-sm-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              IPs suspectes
            </div>
            <div class="h2 mb-0">
              {{ threats.suspicious_ips || 0 }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-12 col-sm-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              Domaines ciblés
            </div>
            <div class="h2 mb-0">
              {{ threats.targeted_hosts || 0 }}
            </div>
          </div>
        </div>
      </div>
      <div class="col-12 col-sm-3">
        <div class="card card-sm h-100">
          <div class="card-body text-center">
            <div class="text-secondary small mb-1">
              IPs bloquées
            </div>
            <div class="h2 mb-0 text-success">
              {{ threats.blocked_ips || 0 }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards">
      <div class="col-lg-7">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title mb-0">
              IPs suspectes
            </h3>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th class="text-end">
                    Hits
                  </th>
                  <th class="text-end">
                    Chemins
                  </th>
                  <th class="text-end">
                    Domaines
                  </th>
                  <th>Niveau</th>
                  <th>Blocage</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr v-if="!topIPs.length">
                  <td
                    colspan="7"
                    class="text-center text-secondary py-4"
                  >
                    Aucune IP suspecte sur la période.
                  </td>
                </tr>
                <tr
                  v-for="ip in topIPs"
                  :key="ip.ip"
                >
                  <td class="font-monospace small">
                    {{ ip.ip }}
                  </td>
                  <td class="text-end">
                    {{ ip.hits || 0 }}
                  </td>
                  <td class="text-end">
                    {{ ip.unique_paths || 0 }}
                  </td>
                  <td class="text-end">
                    {{ ip.host_count || 0 }}
                  </td>
                  <td>
                    <span
                      class="badge"
                      :class="levelClass(ip.level)"
                    >{{ ip.level || 'LOW' }}</span>
                  </td>
                  <td>
                    <template v-if="ip.blocked || ip.blocked_type">
                      <span
                        class="badge"
                        :class="decisionBadgeClass(ip.blocked_type)"
                        :title="formatBlockedUntil(ip.blocked_until)"
                      >
                        {{ decisionLabel(ip.blocked_type) }}
                      </span>
                    </template>
                    <span
                      v-else
                      class="text-secondary small"
                    >
                      —
                    </span>
                  </td>
                  <td class="text-end">
                    <button
                      class="btn btn-sm btn-outline-primary"
                      @click="openTimeline(ip.ip)"
                    >
                      Timeline
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <div class="col-lg-5">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title mb-0">
              Top chemins scannés
            </h3>
          </div>
          <div class="card-body p-0">
            <div
              v-if="!topPaths.length"
              class="text-center py-4 text-secondary small"
            >
              Aucun chemin suspect.
            </div>
            <div
              v-for="p in topPaths"
              v-else
              :key="`${p.path}-${p.category}`"
              class="d-flex justify-content-between border-bottom px-3 py-2 top-path-row"
            >
              <div>
                <div class="font-monospace small">
                  {{ p.path }}
                </div>
                <div class="small text-secondary">
                  {{ p.category || 'Unknown' }}
                </div>
              </div>
              <span class="badge bg-yellow-lt text-yellow">{{ p.hits }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="row row-cards mt-4">
      <div class="col-lg-6">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title mb-0">
              Domaines les plus ciblés
            </h3>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>Domaine cible</th>
                  <th class="text-end">
                    Hits
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!mostTargetedHosts.length">
                  <td
                    colspan="2"
                    class="text-center text-secondary py-4"
                  >
                    Aucun domaine ciblé
                  </td>
                </tr>
                <tr
                  v-for="h in mostTargetedHosts"
                  :key="h.host_id"
                >
                  <td>{{ h.host_name || h.host_id }}</td>
                  <td class="text-end">
                    {{ h.hits || 0 }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div class="col-lg-6">
        <div class="card h-100">
          <div class="card-header">
            <h3 class="card-title mb-0">
              IP × Domaines (scan coordonné)
            </h3>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th class="text-end">
                    Domaines
                  </th>
                  <th class="text-end">
                    Hits
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!ipHostMatrix.length">
                  <td
                    colspan="3"
                    class="text-center text-secondary py-4"
                  >
                    Pas de scan coordonné détecté
                  </td>
                </tr>
                <tr
                  v-for="m in ipHostMatrix"
                  :key="m.ip"
                >
                  <td class="font-monospace small">
                    {{ m.ip }}
                  </td>
                  <td class="text-end">
                    {{ m.host_count || 0 }}
                  </td>
                  <td class="text-end">
                    {{ m.hits || 0 }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="crowdSecIPs.length"
      class="row row-cards mt-4"
    >
      <div class="col-12">
        <div class="card">
          <div class="card-header d-flex align-items-center justify-content-between gap-2 flex-wrap">
            <h3 class="card-title mb-0">
              IPs bloquées par CrowdSec
            </h3>
            <span class="badge bg-success-lt text-success fs-4">
              {{ crowdSecTotal.toLocaleString() }} décisions actives
            </span>
          </div>
          <div class="table-responsive">
            <table class="table table-vcenter card-table">
              <thead>
                <tr>
                  <th>IP</th>
                  <th>Action</th>
                  <th>Scénario</th>
                  <th>Origine</th>
                  <th>Pays</th>
                  <th>AS / Opérateur</th>
                  <th>Expiration</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="entry in crowdSecIPs"
                  :key="entry.ip"
                >
                  <td class="font-monospace small">
                    {{ entry.ip }}
                  </td>
                  <td>
                    <span
                      class="badge"
                      :class="decisionBadgeClass(entry.type)"
                    >{{ decisionLabel(entry.type) }}</span>
                  </td>
                  <td class="small text-secondary">
                    {{ entry.reason || '—' }}
                  </td>
                  <td>
                    <span
                      class="badge"
                      :class="entry.origin === 'CAPI' ? 'bg-azure-lt text-azure' : 'bg-purple-lt text-purple'"
                    >{{ entry.origin || '—' }}</span>
                  </td>
                  <td class="small">
                    {{ entry.country || '—' }}
                  </td>
                  <td
                    class="small text-secondary"
                    :title="entry.as_name"
                  >
                    {{ entry.as_name ? truncate(entry.as_name, 28) : '—' }}
                  </td>
                  <td class="small">
                    {{ formatBlockedUntil(entry.blocked_until) }}
                  </td>
                  <td class="text-end">
                    <div class="d-flex gap-1 justify-content-end">
                      <button
                        class="btn btn-sm"
                        :class="rowState[entry.ip] === 'error' ? 'btn-danger' : 'btn-outline-success'"
                        :disabled="rowState[entry.ip] === 'loading'"
                        @click="unblockCrowdSecEntry(entry.ip)"
                      >
                        <span v-if="rowState[entry.ip] === 'loading'" class="spinner-border spinner-border-sm me-1" />
                        <span v-if="rowState[entry.ip] === 'loading'">Déblocage…</span>
                        <span v-else-if="rowState[entry.ip] === 'error'">Erreur — Réessayer</span>
                        <span v-else>Débloquer</span>
                      </button>
                      <button
                        class="btn btn-sm btn-outline-primary"
                        @click="openTimeline(entry.ip)"
                      >
                        Timeline
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div
            v-if="crowdSecTotal > crowdSecIPs.length"
            class="card-footer text-secondary small"
          >
            Affichage des {{ crowdSecIPs.length }} premières entrées sur {{ crowdSecTotal.toLocaleString() }} IPs bloquées
          </div>
        </div>
      </div>
    </div>

    </template><!-- fin v-else contenu réel -->

    <IPTimelineModal
      :show="showTimeline"
      :ip="selectedIP"
      :timeline="timeline"
      :loading="timelineLoading"
      :blocked="isSelectedIPBlocked"
      :ban-loading="banState === 'loading'"
      :ban-error="banState === 'error'"
      :host-id="effectiveHostId"
      @close="closeTimeline"
      @ban="handleBanFromModal"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import apiClient, { getApiErrorMessage } from '../api'
import { useToast } from '../composables/useToast'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import IPTimelineModal from '../components/IPTimelineModal.vue'

type AnyRecord = Record<string, any>

const period = ref('24h')
const periodOptions = [
  { value: '1h', label: '1h' },
  { value: '24h', label: '24h' },
  { value: '168h', label: '7j' },
  { value: '720h', label: '30j' },
]
const source = ref('')
const hostId = ref('')

const loading = ref(false)
const summary = ref<AnyRecord>({ threats: {} })

const showTimeline = ref(false)
const timelineLoading = ref(false)
const banState = ref<'idle' | 'loading' | 'error'>('idle')
const selectedIP = ref('')
const timelineHostId = ref('')
const timeline = ref<AnyRecord[]>([])

const { value: actionFeedback, showToast: showActionFeedback } = useToast<{ message: string; type: 'success' | 'error' } | null>(null)

const threats = computed(() => summary.value.threats || {})
const topPaths = computed(() => threats.value.top_paths || [])
const mostTargetedHosts = computed(() => threats.value.most_targeted_hosts || [])
const ipHostMatrix = computed(() => threats.value.ip_host_matrix || [])
const unblockedIPs = ref(new Set<string>())
const rowState = ref<Record<string, 'loading' | 'error'>>({})
const optimisticBans = ref<AnyRecord[]>([])

const crowdSecIPs = computed(() => {
  const fromSnapshot = (threats.value.crowdsec_top_blocked || [] as AnyRecord[]).filter(
    (e: AnyRecord) => !unblockedIPs.value.has(e.ip as string),
  )
  const snapshotIPs = new Set(fromSnapshot.map((e: AnyRecord) => e.ip as string))
  const extra = optimisticBans.value.filter((e) => !snapshotIPs.has(e.ip as string) && !unblockedIPs.value.has(e.ip as string))
  return [...extra, ...fromSnapshot]
})
const crowdSecTotal = computed(() => Number(threats.value.crowdsec_blocked_ips) || 0)

// topIPs enriched: merge CrowdSec decision type from the active decisions list so the
// "Blocage" column reflects the current state even for IPs with no recent blocked requests.
const topIPs = computed(() => {
  const decisionMap = new Map(crowdSecIPs.value.map((e: AnyRecord) => [e.ip as string, e]))
  return (threats.value.top_ips || [] as AnyRecord[]).map((ip: AnyRecord) => {
    const decision = decisionMap.get(ip.ip as string)
    if (!decision) return ip
    if (ip.blocked && ip.blocked_type) return ip  // already enriched by agent, trust it
    const decType = ((decision.type as string) || 'ban').toLowerCase()
    const isBan = decType === 'ban'
    return {
      ...ip,
      blocked: isBan || Boolean(decision.blocked_until),
      blocked_source: 'crowdsec',
      blocked_type: decType || 'ban',
      blocked_until: ip.blocked_until || decision.blocked_until,
    }
  })
})
// host_id du snapshot CrowdSec renvoyé par l'API (présent même sans filtre hôte)
const crowdSecHostId = computed(() => (threats.value.crowdsec_host_id as string) || '')
const isSelectedIPBlocked = computed(() =>
  crowdSecIPs.value.some((e: AnyRecord) => e.ip === selectedIP.value),
)
// host_id effectif : filtre manuel > host_id du snapshot CrowdSec > déduit des lignes de la timeline
const effectiveHostId = computed(() => hostId.value || crowdSecHostId.value || timelineHostId.value)


function levelClass(level: string): string {
  switch (level) {
    case 'CRITICAL': return 'bg-red-lt text-red'
    case 'HIGH': return 'bg-orange-lt text-orange'
    case 'MEDIUM': return 'bg-yellow-lt text-yellow'
    default: return 'bg-azure-lt text-azure'
  }
}

function decisionLabel(type: string): string {
  if (!type) return '—'
  switch (type.toLowerCase()) {
    case 'ban': return 'Ban'
    case 'captcha': return 'Captcha'
    case 'audit': return 'Audit'
    default: return type
  }
}

function decisionBadgeClass(type: string): string {
  if (!type) return 'bg-secondary-lt text-secondary'
  switch (type.toLowerCase()) {
    case 'ban': return 'bg-red-lt text-red'
    case 'captcha': return 'bg-yellow-lt text-yellow'
    case 'audit': return 'bg-azure-lt text-azure'
    default: return 'bg-secondary-lt text-secondary'
  }
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + '…' : s
}

function formatBlockedUntil(blockedUntil?: string): string {
  if (!blockedUntil) return 'Bloquée'
  const d = new Date(blockedUntil)
  if (Number.isNaN(d.getTime())) return 'Bloquée'
  const now = new Date()
  if (d <= now) return 'Bloquée (permanent)'
  const diff = d.getTime() - now.getTime()
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(hours / 24)
  if (days > 0) return `Bloquée jusqu'à ${d.toLocaleString()}`
  if (hours > 0) return `Bloquée ${hours}h`
  return 'Bloquée (moins d\'une heure)'
}

async function loadThreats() {
  loading.value = true
  try {
    const res = await apiClient.getWebLogsSummary(period.value, hostId.value || undefined, source.value || undefined)
    summary.value = { threats: res.data?.threats || {} }
    // Purger les bans optimistes dont le snapshot réel prend le relais
    const snapshotIPs = new Set((res.data?.threats?.crowdsec_top_blocked || []).map((e: AnyRecord) => e.ip as string))
    optimisticBans.value = optimisticBans.value.filter((e) => !snapshotIPs.has(e.ip as string))
  } catch (err) {
    console.error('Failed to load threats summary', err)
  } finally {
    loading.value = false
  }
}

function setPeriod(value: string) {
  if (period.value === value) return
  period.value = value
  void loadThreats()
}

async function openTimeline(ip: string) {
  selectedIP.value = ip
  timelineHostId.value = ''
  banState.value = 'idle'
  showTimeline.value = true
  timelineLoading.value = true
  try {
    const res = await apiClient.getIPTimeline(ip, hostId.value || undefined, period.value, 500)
    timeline.value = res.data?.requests || []
    const rows: AnyRecord[] = timeline.value
    if (rows.length > 0) {
      const first = rows[0].host_id as string
      if (first && rows.every((r) => r.host_id === first)) {
        timelineHostId.value = first
      }
    }
  } catch (err) {
    console.error('Failed to load IP timeline', err)
    timeline.value = []
  } finally {
    timelineLoading.value = false
  }
}

function closeTimeline() {
  showTimeline.value = false
  timeline.value = []
  selectedIP.value = ''
  timelineHostId.value = ''
}

async function handleBanFromModal(duration: string) {
  banState.value = 'loading'
  const ip = selectedIP.value
  try {
    const res = await apiClient.blockCrowdSecIP(ip, effectiveHostId.value, duration)
    const commandId: string = res.data?.command_id
    const ms = duration.endsWith('h') ? parseInt(duration) * 3600000 : parseInt(duration) * 60000
    optimisticBans.value = [
      ...optimisticBans.value.filter((e) => e.ip !== ip),
      { ip, type: 'ban', reason: 'manual', origin: 'cscli', blocked_until: new Date(Date.now() + ms).toISOString() },
    ]
    banState.value = 'idle'
    showActionFeedback({ message: `IP ${ip} bloquée par CrowdSec (${duration})`, type: 'success' })
    closeTimeline()
    const { status, output } = await pollCommand(commandId)
    if (status === 'failed') {
      optimisticBans.value = optimisticBans.value.filter((e) => e.ip !== ip)
      showActionFeedback({ message: `Échec blocage ${ip} : ${output}`, type: 'error' })
    }
  } catch (error) {
    banState.value = 'error'
    showActionFeedback({ message: `Impossible de bloquer l'IP : ${getApiErrorMessage(error)}`, type: 'error' })
  }
}

async function pollCommand(commandId: string): Promise<{ status: string; output: string }> {
  for (let i = 0; i < 40; i++) {
    await new Promise((r) => setTimeout(r, 1500))
    try {
      const res = await apiClient.getCommand(commandId)
      const { status, output } = res.data ?? {}
      if (status === 'completed' || status === 'failed') return { status, output: output ?? '' }
    } catch {
      // ignore transient poll errors
    }
  }
  // Timeout après 60s (2× le cycle agent) — la commande est probablement passée
  return { status: 'timeout', output: '' }
}

async function unblockCrowdSecEntry(ip: string) {
  const matchedEntry = crowdSecIPs.value.find((entry: AnyRecord) => entry.ip === ip)
  const targetHost = hostId.value || (matchedEntry?.host_id as string) || crowdSecHostId.value
  if (!targetHost) {
    showActionFeedback({ message: 'Impossible de déterminer l\'hôte cible — renseigne le filtre Hôte', type: 'error' })
    return
  }
  rowState.value = { ...rowState.value, [ip]: 'loading' }
  try {
    const res = await apiClient.unblockCrowdSecIP(ip, targetHost)
    const commandId: string = res.data?.command_id
    const { status, output } = await pollCommand(commandId)
    if (status === 'completed' || status === 'timeout') {
      const next = new Set(unblockedIPs.value)
      next.add(ip)
      unblockedIPs.value = next
      const { [ip]: _, ...rest } = rowState.value
      rowState.value = rest
      showActionFeedback({ message: `IP ${ip} débloquée`, type: 'success' })
    } else {
      rowState.value = { ...rowState.value, [ip]: 'error' }
      showActionFeedback({ message: `Échec déblocage ${ip} : ${output}`, type: 'error' })
    }
  } catch (error) {
    rowState.value = { ...rowState.value, [ip]: 'error' }
    showActionFeedback({ message: `Impossible de débloquer l'IP : ${getApiErrorMessage(error)}`, type: 'error' })
  }
}

onMounted(loadThreats)
</script>

<style scoped>
.threats-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

@media (max-width: 992px) {
  .threats-filters {
    align-items: stretch !important;
  }

  .threats-filter-field {
    flex: 1 1 220px;
  }

  .threats-filter-field .form-select,
  .threats-filter-field .form-control {
    min-width: 0 !important;
    width: 100%;
  }

  .threats-refresh-btn {
    width: 100%;
  }

  .top-path-row {
    gap: 0.5rem;
    align-items: flex-start;
  }

  .top-path-row .font-monospace {
    overflow-wrap: anywhere;
  }
}
</style>
