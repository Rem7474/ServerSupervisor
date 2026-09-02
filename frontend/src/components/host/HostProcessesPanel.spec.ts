import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { setLocale } from '../../i18n'

const { useHostProcesses } = vi.hoisted(() => ({ useHostProcesses: vi.fn() }))

vi.mock('../../composables/useHostProcesses', () => ({ useHostProcesses }))

import HostProcessesPanel from './HostProcessesPanel.vue'

describe('HostProcessesPanel', () => {
  let processes: ReturnType<typeof ref<unknown[]>>
  let loading: ReturnType<typeof ref<boolean>>
  let error: ReturnType<typeof ref<string>>
  let load: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('fr')
    processes = ref([])
    loading = ref(false)
    error = ref('')
    load = vi.fn().mockResolvedValue(undefined)
    useHostProcesses.mockReturnValue({ processes, loading, error, load })
  })

  it('renders nothing when canRun is false', () => {
    const wrapper = mount(HostProcessesPanel, { props: { hostId: 'h1', canRun: false } })
    expect(wrapper.find('.card').exists()).toBe(false)
  })

  it('shows the load-processes hint before anything has been loaded', () => {
    const wrapper = mount(HostProcessesPanel, { props: { hostId: 'h1', canRun: true } })
    expect(wrapper.text()).toContain('Charger les processus')
    expect(wrapper.text()).toContain('Cliquez sur "Charger les processus"')
  })

  it('shows a loading skeleton and the loading button label while loading', () => {
    loading.value = true
    const wrapper = mount(HostProcessesPanel, { props: { hostId: 'h1', canRun: true } })
    expect(wrapper.text()).toContain('Chargement...')
    expect(wrapper.findComponent({ name: 'LoadingSkeleton' }).exists()).toBe(true)
  })

  it('shows an error message', () => {
    error.value = 'Impossible de parser la liste des processus'
    const wrapper = mount(HostProcessesPanel, { props: { hostId: 'h1', canRun: true } })
    expect(wrapper.find('.alert-danger').text()).toBe('Impossible de parser la liste des processus')
  })

  it('renders the processes table and switches the button to "refresh" once loaded', () => {
    processes.value = [
      { pid: 1, name: 'systemd', user: 'root', cpu_pct: 0.1, mem_pct: 0.2, mem_rss_kb: 1024, state: 'S' },
    ]
    const wrapper = mount(HostProcessesPanel, { props: { hostId: 'h1', canRun: true } })
    expect(wrapper.findComponent({ name: 'ProcessesTable' }).exists()).toBe(true)
    expect(wrapper.text()).toContain('Actualiser les processus')
  })

  it('calls load() and emits history-changed when the button is clicked', async () => {
    const wrapper = mount(HostProcessesPanel, { props: { hostId: 'h1', canRun: true } })
    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(load).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('history-changed')).toBeTruthy()
  })
})
