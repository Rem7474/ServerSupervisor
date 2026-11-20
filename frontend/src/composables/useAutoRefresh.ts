import { onMounted, onUnmounted, ref, type Ref } from 'vue'

interface AutoRefreshOptions {
  intervalSec: number
  /** An externally owned toggle (a parent page sharing one refresh bar). */
  enabled?: Ref<boolean>
  /** Skips a tick while true, e.g. during a mutation that reloads on its own. */
  skip?: () => boolean
}

/**
 * Periodic reload behind a page's PageRefreshBar: calls `tick` every
 * `intervalSec` while `autoRefresh` is on, from mount to unmount. The initial
 * load stays with the caller. `lastUpdatedAt` is for the caller to set when a
 * load succeeds; the refresh bar displays it.
 */
export function useAutoRefresh(tick: () => unknown, options: AutoRefreshOptions) {
  const autoRefresh = options.enabled ?? ref(true)
  const lastUpdatedAt = ref<Date | null>(null)
  let timer: ReturnType<typeof setInterval> | undefined

  onMounted(() => {
    timer = setInterval(() => {
      if (autoRefresh.value && !options.skip?.()) void tick()
    }, options.intervalSec * 1000)
  })
  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { autoRefresh, lastUpdatedAt }
}
