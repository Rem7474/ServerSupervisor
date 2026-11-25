import { computed, defineAsyncComponent, ref, watch, type Ref } from 'vue'
import type { HostRecord } from '../types/hostDetail'

interface Options {
  isAdmin: () => boolean
  proxmoxLink: Ref<HostRecord | null>
  activeTab: Ref<string>
  showConsole: Ref<boolean>
  closeConsoleAndStream: () => void
}

/**
 * State machine of HostDetailView's single side panel, which holds either the
 * live command log or the PVE LXC terminal (see `consoleKind`), plus the VNC
 * dialog of a host linked to a QEMU VM (a modal, independent of the panel).
 */
export function useHostLxcConsole(opts: Options) {
  // The PVE console (termproxy) exists for LXC only, needs an admin session, and
  // is addressed by the Proxmox guest a confirmed link points at.
  const hasLxcConsole = computed(() => {
    const link = opts.proxmoxLink.value
    return opts.isAdmin() && !!link && link.status === 'confirmed' && link.guest_type === 'lxc'
  })
  // A host linked to a QEMU VM gets the graphical VNC console instead: same
  // admin + confirmed-link conditions, different PVE endpoint and UI.
  const hasVncConsole = computed(() => {
    const link = opts.proxmoxLink.value
    return opts.isAdmin() && !!link && link.status === 'confirmed' && link.guest_type === 'vm'
  })
  const hasGuestConsole = computed(() => hasLxcConsole.value || hasVncConsole.value)
  const ProxmoxConsoleLazy = defineAsyncComponent(() => import('../components/proxmox/ProxmoxConsole.vue'))
  const ProxmoxVncConsoleLazy = defineAsyncComponent(() => import('../components/proxmox/ProxmoxVncConsole.vue'))
  const showVncConsole = ref(false)
  const hasOpenedVncConsole = ref(false)

  function openVncConsole(): void {
    hasOpenedVncConsole.value = true
    showVncConsole.value = true
  }
  const showLxcConsole = ref(false)
  const hasOpenedLxcConsole = ref(false)
  // A single panel holds either console; `consoleKind` is which one it shows
  // (and which one the reopen button brings back). Opening one closes the other.
  const consoleKind = ref<'live' | 'lxc'>('live')
  watch(opts.showConsole, (open) => {
    if (!open) return
    consoleKind.value = 'live'
    showLxcConsole.value = false
  })

  function openLxcConsole(): void {
    hasOpenedLxcConsole.value = true
    consoleKind.value = 'lxc'
    showLxcConsole.value = true
    opts.showConsole.value = false
  }

  function reopenConsole(): void {
    if (consoleKind.value === 'lxc') showLxcConsole.value = true
    else opts.showConsole.value = true
  }

  function closeActiveConsole(): void {
    if (consoleKind.value === 'lxc') showLxcConsole.value = false
    else opts.closeConsoleAndStream()
  }

  watch(opts.activeTab, (tab) => {
    if (tab === 'console' && hasLxcConsole.value && !hasOpenedLxcConsole.value) openLxcConsole()
  })

  return {
    hasLxcConsole,
    hasVncConsole,
    hasGuestConsole,
    ProxmoxConsoleLazy,
    ProxmoxVncConsoleLazy,
    showVncConsole,
    hasOpenedVncConsole,
    openVncConsole,
    showLxcConsole,
    hasOpenedLxcConsole,
    consoleKind,
    openLxcConsole,
    reopenConsole,
    closeActiveConsole,
  }
}
