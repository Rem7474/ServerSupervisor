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