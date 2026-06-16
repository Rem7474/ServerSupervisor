<template>
  <div>
    <div class="card-header d-flex align-items-center gap-2 flex-wrap">
      <select
        v-model="securityService"
        class="form-select proxmox-security-service-select"
      >
        <option value="rotate">
          Rotation (3 services)
        </option>
        <option value="pveproxy">
          pveproxy
        </option>
        <option value="sshd">
          sshd
        </option>
        <option value="pvedaemon">
          pvedaemon
        </option>
        <option value="">
          Tous les services
        </option>
      </select>
      <input
        v-model="securitySearch"
        type="text"
        class="form-control proxmox-security-search"
        placeholder="Filtre syslog (ex: failed, denied, apparmor)"
      >
      <button
        type="button"
        class="btn btn-sm btn-outline-secondary"
        :disabled="loading"
        @click="loadEvents"
      >
        <span
          v-if="loading"
          class="spinner-border spinner-border-sm me-1"
        />
        Rechercher
      </button>
    </div>
    <div
      v-if="error"
      class="card-body pb-0"
    >
      <div class="alert alert-danger mb-0">
        {{ error }}
      </div>
    </div>
    <div
      v-if="loading"
      class="card-body text-muted small"
    >
      <span class="spinner-border spinner-border-sm me-1" />Chargement des événements de sécurité…
    </div>
    <div
      v-else-if="events.length"
      class="table-responsive"
    >
      <table class="table table-vcenter card-table">
        <thead>
          <tr>
            <th>Date</th>
            <th>Niveau</th>
            <th>Tag</th>
            <th>Message</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, idx) in events"
            :key="item.id || `${item.parsedTimeMs || item.time || 't'}-${idx}`"
          >
            <td class="text-muted small">
              {{ formatSyslogTime(item) }}
            </td>
            <td>
              <span
                class="badge text-uppercase"
                :class="syslogLevelBadgeClass(item)"
              >{{ item.parsedLevel || item.pri || item.level || '—' }}</span>
            </td>
            <td class="font-monospace small">
              {{ item.parsedTag || item.tag || item.ident || '—' }}
            </td>
            <td class="small">
              {{ item.parsedMsg || item.msg || item.t || '—' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div
      v-else
      class="card-body text-center text-muted py-4"
    >
      Aucun événement de sécurité trouvé pour ce filtre.
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import api from '../../api'
import { getApiErrorMessage } from '../../api/client'

type SyslogItem = Record<string, any>

const props = defineProps<{
  nodeId: string
  active?: boolean
}>()

const emit = defineEmits<{ (e: 'count', value: number): void }>()

const events = ref<SyslogItem[]>([])
const loading = ref(false)
const error = ref('')
const securitySearch = ref('')
const securityService = ref('rotate')
let loadedOnce = false

const SYSLOG_MONTHS: Record<string, number> = {
  Jan: 0, Feb: 1, Mar: 2, Apr: 3, May: 4, Jun: 5,
  Jul: 6, Aug: 7, Sep: 8, Oct: 9, Nov: 10, Dec: 11,
}

function guessLevel(text: string): string {
  const v = String(text || '').toLowerCase()
  if (!v) return ''
  if (
    v.includes('successful auth') ||
    v.includes('authentication success') ||
    v.includes('authentication succeeded') ||
    v.includes('login successful')
  ) return 'success'
  if (
    v.includes('authentication failure') ||
    v.includes('failed password') ||
    v.includes('invalid user') ||
    v.includes('too many authentication failures') ||
    v.includes('maximum authentication attempts exceeded') ||
    v.includes('user root@pam msg=authentication failure')
  ) return 'critical'
  if (v.includes('critical') || v.includes('panic') || v.includes('fatal')) return 'critical'
  if (v.includes('error') || v.includes('failed') || v.includes('denied')) return 'error'
  if (v.includes('failure')) return 'error'
  if (v.includes('warn')) return 'warning'
  if (v.includes('info')) return 'info'
  return ''
}

function parseHeaderDate(prefix: string): Date | null {
  const m = /^([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{2}):(\d{2}):(\d{2})$/.exec(String(prefix || '').trim())
  if (!m) return null
  const month = SYSLOG_MONTHS[m[1]]
  if (month == null) return null
  const now = new Date()
  let year = now.getFullYear()
  let d = new Date(year, month, Number(m[2]), Number(m[3]), Number(m[4]), Number(m[5]))
  if (d.getTime() > now.getTime() + 86_400_000) {
    year -= 1
    d = new Date(year, month, Number(m[2]), Number(m[3]), Number(m[4]), Number(m[5]))
  }
  return d
}

function normalizeSyslogEntry(item: SyslogItem): SyslogItem {
  const out: SyslogItem = { ...(item || {}) }
  const raw = String(out.t || '')
  if (raw) {
    const m = /^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+([^\s:]+)(?:\[(\d+)\])?:\s*(.*)$/.exec(raw)
    if (m) {
      const parsedDate = parseHeaderDate(m[1])
      if (parsedDate) out.parsedTimeMs = parsedDate.getTime()
      if (!out.parsedTag) out.parsedTag = m[3]
      const pidSuffix = m[4] ? `[${m[4]}]` : ''
      const message = (m[5] || '').trim()
      out.parsedMsg = message || `${m[2]} ${m[3]}${pidSuffix}`
      out.parsedLevel = out.level || guessLevel(out.parsedMsg)
    } else {
      out.parsedMsg = out.msg || raw
      out.parsedLevel = out.level || guessLevel(out.parsedMsg)
      out.parsedTag = out.tag || out.ident || ''
    }
  } else {
    out.parsedMsg = out.msg || ''
    out.parsedLevel = out.level || guessLevel(out.parsedMsg)
    out.parsedTag = out.tag || out.ident || ''
  }

  if (!out.parsedTimeMs && out.time) {
    const rawTime = out.time
    const ms = typeof rawTime === 'number'
      ? (rawTime < 1_000_000_000_000 ? rawTime * 1000 : rawTime)
      : Date.parse(rawTime)
    if (Number.isFinite(ms)) out.parsedTimeMs = ms
  }

  return out
}

function mergeAndRankSyslogLines(groups: SyslogItem[][], maxLines = 200): SyslogItem[] {
  const flat = groups.flatMap(g => Array.isArray(g) ? g : []).map(normalizeSyslogEntry)
  const uniq = new Map<string, SyslogItem>()
  for (const item of flat) {
    const key = `${item.parsedTimeMs ?? item.time ?? ''}|${item.parsedTag ?? item.tag ?? ''}|${item.parsedMsg ?? item.msg ?? item.t ?? ''}`
    if (!uniq.has(key)) uniq.set(key, item)
  }
  const ranked = [...uniq.values()].sort((a, b) => {
    const ta = Number(a?.parsedTimeMs ?? a?.time ?? 0)
    const tb = Number(b?.parsedTimeMs ?? b?.time ?? 0)
    if (ta !== tb) return tb - ta
    return Number(b?.n ?? 0) - Number(a?.n ?? 0)
  })
  return ranked.slice(0, maxLines)
}

function formatSyslogTime(item: SyslogItem): string {
  const raw = item?.parsedTimeMs ?? item?.time
  if (!raw) return '—'
  const ms = typeof raw === 'number' ? (raw < 1_000_000_000_000 ? raw * 1000 : raw) : Date.parse(raw)
  if (!Number.isFinite(ms)) return '—'
  return new Date(ms).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'medium' })
}

function syslogLevelBadgeClass(item: SyslogItem): string {
  const raw = String(item?.parsedLevel || item?.pri || item?.level || '').toLowerCase()
  if (raw.includes('critical') || raw.includes('fatal') || raw.includes('panic')) return 'bg-danger-lt text-danger'
  if (raw.includes('error') || raw.includes('err')) return 'bg-danger-lt text-danger'
  if (raw.includes('warning') || raw.includes('warn')) return 'bg-orange-lt text-orange'
  if (raw.includes('success') || raw.includes('ok')) return 'bg-success-lt text-success'
  if (raw.includes('info') || raw.includes('notice')) return 'bg-azure-lt text-azure'
  return 'bg-secondary-lt text-secondary'
}

async function loadEvents() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    if (securityService.value === 'rotate') {
      const services = ['pveproxy', 'sshd', 'pvedaemon']
      const calls = services.map(service =>
        api.getProxmoxNodeSyslog(props.nodeId, {
          limit: 120,
          search: securitySearch.value,
          service,
        })
      )
      const results = await Promise.allSettled(calls)
      const groups = results
        .filter((r): r is PromiseFulfilledResult<any> => r.status === 'fulfilled')
        .map(r => Array.isArray(r.value?.data) ? r.value.data : [])
      if (!groups.length) {
        throw new Error('Aucun service syslog accessible (pveproxy, sshd, pvedaemon).')
      }
      events.value = mergeAndRankSyslogLines(groups, 200)
    } else {
      const res = await api.getProxmoxNodeSyslog(props.nodeId, {
        limit: 200,
        search: securitySearch.value,
        service: securityService.value,
      })
      events.value = mergeAndRankSyslogLines([Array.isArray(res.data) ? res.data : []], 200)
    }
  } catch (e: unknown) {
    error.value = getApiErrorMessage(e, 'Erreur lors du chargement des événements de sécurité.')
    events.value = []
  } finally {
    loading.value = false
  }
}

watch(events, (list) => emit('count', list.length))

// Lazy-load on first activation (mirrors the original tab-click fetch).
watch(
  () => props.active,
  (isActive) => {
    if (isActive && !loadedOnce) {
      loadedOnce = true
      void loadEvents()
    }
  },
  { immediate: true }
)
</script>

<style scoped>
.proxmox-security-service-select {
  max-width: 11rem;
}

.proxmox-security-search {
  max-width: 18rem;
}

@media (max-width: 992px) {
  .proxmox-security-service-select,
  .proxmox-security-search {
    max-width: 100%;
    width: 100%;
  }
}
</style>
