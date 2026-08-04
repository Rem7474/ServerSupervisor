import { ref, computed, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import apiClient from '../api'
import { getApiErrorMessage } from '../api/client'
import { parseTagsInput } from '../utils/tags'
import { buildInstallCommand, buildAgentConfig } from '../utils/agentInstall'
import type { NetworkProxmoxGuestIP } from '../types/network'

interface HostResult {
  id?: string
  api_key?: string
}

interface GuestIPOption {
  guest: NetworkProxmoxGuestIP
  ip: string
}

const AGENT_POLL_INTERVAL_MS = 3000
const AGENT_POLL_TIMEOUT_MS = 120_000

export function useAddHost() {
  const router = useRouter()

  const form = ref({ name: '', ip_address: '', tags: '' })
  const error = ref('')
  const loading = ref(false)
  const touched = ref({ name: false, ip_address: false })

  const IP_RE = /^(\d{1,3}\.){3}\d{1,3}$/
  const isValidIp = computed<boolean | null>(() => {
    const v = form.value.ip_address.trim()
    if (!v) return null
    if (!IP_RE.test(v)) return false
    return v.split('.').every((n) => Number(n) <= 255)
  })
  const ipFeedback = computed(() => {
    if (!touched.value.ip_address || isValidIp.value === null) return ''
    return isValidIp.value ? '' : 'Adresse IPv4 invalide (ex: 192.168.1.100)'
  })
  const showGuestPicker = ref(false)
  const guestsLoading = ref(false)
  const guestsError = ref('')
  const guests = ref<NetworkProxmoxGuestIP[]>([])
  let guestsLoaded = false
  const guestSearch = ref('')

  // Flattened one row per guest IP (a guest can report several interfaces),
  // excluding guests already linked to an existing Host — picking one of
  // those would just create a duplicate host for the same machine.
  const guestIPOptions = computed<GuestIPOption[]>(() =>
    guests.value
      .filter((g) => !g.host_id)
      .flatMap((guest) => guest.ip_addresses.map((ip) => ({ guest, ip })))
  )

  const filteredGuestIPOptions = computed<GuestIPOption[]>(() => {
    const q = guestSearch.value.trim().toLowerCase()
    if (!q) return guestIPOptions.value
    return guestIPOptions.value.filter(
      ({ guest, ip }) =>
        guest.name.toLowerCase().includes(q) ||
        guest.node.toLowerCase().includes(q) ||
        ip.includes(q)
    )
  })

  async function toggleGuestPicker(): Promise<void> {
    showGuestPicker.value = !showGuestPicker.value
    if (!showGuestPicker.value || guestsLoaded) return
    guestsLoading.value = true
    guestsError.value = ''
    try {
      const res = await apiClient.getIPInventory()
      guests.value = res.data?.proxmox_guests ?? []
      guestsLoaded = true
    } catch (e: unknown) {
      guestsError.value = getApiErrorMessage(e, 'Erreur lors du chargement des hôtes Proxmox')
    } finally {
      guestsLoading.value = false
    }
  }

  function pickGuestIP(option: GuestIPOption): void {
    form.value.ip_address = option.ip
    if (!form.value.name.trim()) form.value.name = option.guest.name
    touched.value.ip_address = true
    showGuestPicker.value = false
    guestSearch.value = ''
  }

  const result = ref<HostResult | null>(null)
  const copiedApiKey = ref(false)
  const copiedConfig = ref(false)
  const copiedInstall = ref(false)
  const agentConnected = ref(false)

  let agentPollTimer: ReturnType<typeof setInterval> | null = null
  let agentPollStarted: number | null = null

  function startAgentPolling(hostId: string): void {
    agentPollStarted = Date.now()
    agentPollTimer = setInterval(async () => {
      if (agentPollStarted !== null && Date.now() - agentPollStarted > AGENT_POLL_TIMEOUT_MS) {
        stopAgentPolling()
        return
      }
      try {
        const res = await apiClient.getHost(hostId)
        if (res.data?.status === 'online' || res.data?.status === 'warning') {
          agentConnected.value = true
          stopAgentPolling()
        }
      } catch {
        // ignore transient errors, keep polling
      }
    }, AGENT_POLL_INTERVAL_MS)
  }

  function stopAgentPolling(): void {
    if (agentPollTimer) {
      clearInterval(agentPollTimer)
      agentPollTimer = null
    }
  }

  onUnmounted(stopAgentPolling)

  const installCmd = computed(() => {
    if (!result.value?.api_key) return ''
    return buildInstallCommand(result.value.api_key)
  })

  const agentConfig = computed(() => {
    if (!result.value?.api_key) return ''
    return buildAgentConfig(result.value.api_key)
  })

  async function handleSubmit(): Promise<void> {
    touched.value.name = true
    touched.value.ip_address = true
    if (!form.value.name.trim() || !isValidIp.value) return
    loading.value = true
    error.value = ''
    try {
      const res = await apiClient.registerHost({
        name: form.value.name,
        ip_address: form.value.ip_address,
        tags: parseTagsInput(form.value.tags),
      })
      result.value = res.data
      if (res.data?.id) startAgentPolling(res.data.id)
    } catch (e: unknown) {
      error.value = getApiErrorMessage(e, 'Erreur lors de l\'enregistrement')
    } finally {
      loading.value = false
    }
  }

  async function copyApiKey(): Promise<void> {
    if (!result.value?.api_key) return
    await navigator.clipboard.writeText(result.value.api_key)
    copiedApiKey.value = true
    setTimeout(() => { copiedApiKey.value = false }, 1500)
  }

  async function copyAgentConfig(): Promise<void> {
    if (!agentConfig.value) return
    await navigator.clipboard.writeText(agentConfig.value)
    copiedConfig.value = true
    setTimeout(() => { copiedConfig.value = false }, 1500)
  }

  async function copyInstallCmd(): Promise<void> {
    if (!installCmd.value) return
    await navigator.clipboard.writeText(installCmd.value)
    copiedInstall.value = true
    setTimeout(() => { copiedInstall.value = false }, 1500)
  }

  function finishAdd(): void {
    if (result.value?.id) {
      router.push(`/hosts/${result.value.id}`)
    } else {
      router.push('/')
    }
  }

  return {
    form,
    error,
    loading,
    touched,
    isValidIp,
    ipFeedback,
    showGuestPicker,
    guestsLoading,
    guestsError,
    guestSearch,
    filteredGuestIPOptions,
    toggleGuestPicker,
    pickGuestIP,
    result,
    copiedApiKey,
    copiedConfig,
    copiedInstall,
    agentConnected,
    installCmd,
    agentConfig,
    handleSubmit,
    copyApiKey,
    copyAgentConfig,
    copyInstallCmd,
    finishAdd,
  }
}
