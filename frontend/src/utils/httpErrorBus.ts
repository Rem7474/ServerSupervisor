export interface HttpErrorEvent {
  status: number | null
  message: string
  timestamp: number
}

type HttpErrorListener = (event: HttpErrorEvent) => void

const listeners = new Set<HttpErrorListener>()

export function subscribeHttpErrors(listener: HttpErrorListener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function emitHttpError(status: number | null, message: string): void {
  const event: HttpErrorEvent = {
    status,
    message,
    timestamp: Date.now(),
  }

  for (const listener of listeners) {
    listener(event)
  }
}

// Connectivity recovery channel: emitted on any successful API response so the
// "server unreachable" banner can auto-clear once the backend answers again.
type NetworkOkListener = () => void
const networkOkListeners = new Set<NetworkOkListener>()

export function subscribeNetworkOk(listener: NetworkOkListener): () => void {
  networkOkListeners.add(listener)
  return () => {
    networkOkListeners.delete(listener)
  }
}

export function emitNetworkOk(): void {
  for (const listener of networkOkListeners) {
    listener()
  }
}