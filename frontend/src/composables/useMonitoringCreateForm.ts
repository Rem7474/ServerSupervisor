import { computed, ref, watch, type Ref } from 'vue'
import type { ProbeForm } from './useUptimeProbes'
import type { CertForm } from './useSslCertificates'

/**
 * Derives the host/port an SSL cert check should use from a probe target,
 * whatever shape the probe type expects: a full URL (http), `host:port` (tcp)
 * or a bare host (icmp). Defaults to port 443 when none is given.
 */
export function parseCertTarget(raw: string): { host: string; port: number } {
  let host = raw
  let port = 443
  if (/^https?:\/\//i.test(raw)) {
    try {
      const u = new URL(raw)
      host = u.hostname
      port = u.port ? Number(u.port) : (u.protocol === 'https:' ? 443 : 80)
    } catch {
      // Not a parseable URL — keep host as the raw string (already assigned above).
    }
  } else if (raw.includes(':')) {
    const [h, p] = raw.split(':')
    host = h
    if (p && !Number.isNaN(Number(p))) port = Number(p)
  } else {
    host = raw.split('/')[0]
  }
  return { host, port }
}

interface CreateFormDeps {
  probeForm: Ref<ProbeForm>
  certForm: Ref<CertForm>
  // Getters, not refs: read right after an await, they must see the live state
  // rather than a prop snapshot that only refreshes on the next render.
  isProbeModalOpen: () => boolean
  isCertModalOpen: () => boolean
  saveProbe: () => Promise<void>
  saveCert: () => Promise<void>
}

// Create-mode state of the shared "new tracker" modal: which resource(s) to
// create together, and the shared "Host / Target" this exists for — a probe
// and a cert on the same target used to mean typing the same domain twice.
// Editing never goes through this (it is always exactly one resource).
export function useMonitoringCreateForm(deps: CreateFormDeps) {
  const { probeForm, certForm, isProbeModalOpen, isCertModalOpen, saveProbe, saveCert } = deps

  // Initialised from whichever modal the entry point opened (openCreateProbe
  // → probe only), so the toggle buttons start in the right state.
  const createIncludeProbe = ref(isProbeModalOpen())
  const createIncludeCert = ref(isCertModalOpen())

  function toggleIncludeProbe(): void {
    if (createIncludeProbe.value && !createIncludeCert.value) return // keep at least one selected
    createIncludeProbe.value = !createIncludeProbe.value
  }

  function toggleIncludeCert(): void {
    if (createIncludeCert.value && !createIncludeProbe.value) return
    createIncludeCert.value = !createIncludeCert.value
  }

  const sharedName = computed<string>({
    get: () => (createIncludeProbe.value ? probeForm.value.name : certForm.value.name),
    set: (v: string) => {
      if (createIncludeProbe.value) probeForm.value.name = v
      if (createIncludeCert.value) certForm.value.name = v
    },
  })

  // Keeps certForm.host/port in lockstep with the probe's own target field
  // while both are selected, so the cert form has nothing left to fill in.
  watch([() => probeForm.value.target, createIncludeProbe, createIncludeCert], () => {
    if (!createIncludeProbe.value || !createIncludeCert.value) return
    const raw = probeForm.value.target.trim()
    if (!raw) {
      certForm.value.host = ''
      return
    }
    const { host, port } = parseCertTarget(raw)
    certForm.value.host = host
    certForm.value.port = port
  })

  async function submitCreateModal(): Promise<void> {
    if (createIncludeProbe.value) {
      await saveProbe()
      if (isProbeModalOpen()) return // save failed (error shown) — don't also attempt the cert
    }
    if (createIncludeCert.value) {
      await saveCert()
    }
  }

  return { createIncludeProbe, createIncludeCert, toggleIncludeProbe, toggleIncludeCert, sharedName, submitCreateModal }
}
