import { onUnmounted } from 'vue'

interface ManualAbortApi {
  getSignal: () => AbortSignal
  abort: () => void
}

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

/**
 * Returns an abort() function and a signal for manual control.
 * Useful when you need to cancel a request before component unmount
 * (e.g. when starting a new search while a previous one is in flight).
 *
 *   const { signal, abort } = useManualAbort()
 *   function search(q) {
 *     abort()                           // cancel previous
 *     api.get('/v1/hosts', { signal })  // new request with fresh signal
 *   }
 *
 * Note: after abort() the signal is spent — call abort() again on a new
 * controller if you need to issue a subsequent cancellable request.
 */
export function useManualAbort(): ManualAbortApi {
  let controller = new AbortController()

  function abort(): void {
    controller.abort()
    controller = new AbortController()
  }

  function getSignal(): AbortSignal {
    return controller.signal
  }

  onUnmounted(() => controller.abort())

  return { getSignal, abort }
}
