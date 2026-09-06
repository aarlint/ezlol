import { reactive } from 'vue'
import type { Augment } from './types'

export type ToastKind = 'info' | 'success' | 'warn' | 'error' | 'pick'

export interface ToastAction {
  label: string
  primary?: boolean
  run: () => void
}

export interface Toast {
  id: number
  key?: string
  kind: ToastKind
  title: string
  body?: string
  actions?: ToastAction[]
  cards?: Augment[] // augment offer cards, best first
  sticky?: boolean
  until?: number
}

export const toasts = reactive<Toast[]>([])
let seq = 1

/** Show a toast. A toast with the same `key` replaces the existing one in place. */
export function toast(t: Omit<Toast, 'id'> & { ttl?: number }): number {
  const ttl = t.sticky ? 0 : (t.ttl ?? 8000)
  const existing = t.key ? toasts.find((x) => x.key === t.key) : undefined
  if (existing) {
    Object.assign(existing, t, { until: ttl ? Date.now() + ttl : undefined })
    return existing.id
  }
  const id = seq++
  toasts.push({ ...t, id, until: ttl ? Date.now() + ttl : undefined })
  return id
}

export function dismiss(id: number) {
  const i = toasts.findIndex((x) => x.id === id)
  if (i >= 0) toasts.splice(i, 1)
}

export function dismissKey(key: string) {
  const i = toasts.findIndex((x) => x.key === key)
  if (i >= 0) toasts.splice(i, 1)
}

// Expiry sweep.
setInterval(() => {
  const now = Date.now()
  for (let i = toasts.length - 1; i >= 0; i--) {
    const t = toasts[i]
    if (t.until && t.until <= now) toasts.splice(i, 1)
  }
}, 500)
