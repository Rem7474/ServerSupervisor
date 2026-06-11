import { reactive } from 'vue'

export interface GlobalToast {
  id: number
  type: 'success' | 'error' | 'info' | 'warning'
  message: string
}

let _nextId = 0
const _toasts = reactive<GlobalToast[]>([])

export function addToast(message: string, type: GlobalToast['type'] = 'info', durationMs = 5000): void {
  const id = ++_nextId
  _toasts.push({ id, type, message })
  if (durationMs > 0) {
    setTimeout(() => removeToast(id), durationMs)
  }
}

export function removeToast(id: number): void {
  const idx = _toasts.findIndex((t) => t.id === id)
  if (idx >= 0) _toasts.splice(idx, 1)
}

export function useGlobalToast() {
  return { toasts: _toasts, addToast, removeToast }
}
