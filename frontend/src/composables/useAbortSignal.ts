import { onUnmounted } from 'vue'

/**
 * Returns an AbortSignal that is automatically aborted when the calling
 * component is unmounted. Pass the signal to any API call that accepts it:
 *
 *   const signal = useAbortSignal()
 *   api.get('/v1/hosts', { signal }).then(...)
 *
 * Axios natively supports AbortSignal — no CancelToken adapter required.
 * Aborted requests resolve as axios.isCancel(err) === true and should be
 * silently swallowed in catch blocks.
 */
export function useAbortSignal(): AbortSignal {
  const controller = new AbortController()
  onUnmounted(() => controller.abort())
  return controller.signal
}
