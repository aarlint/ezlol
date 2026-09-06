import type { Build, BuildsStatus, RunePage, ChampSelect, Champion, ChampionDetail, ChampionInfo, Eog, Live, LogEntry, Mastery, PlayRecord, Session, Status } from './types'

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { headers: { 'Content-Type': 'application/json' }, ...init })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(body.error ?? `${res.status} ${res.statusText}`)
  return body as T
}

export const api = {
  status: () => req<Status & { logs: LogEntry[]; ddVersion: string }>('/api/status'),
  setAutoAccept: (enabled: boolean) =>
    req<Status>('/api/auto-accept', { method: 'POST', body: JSON.stringify({ enabled }) }),
  accept: () => req<Status>('/api/accept', { method: 'POST' }),
  champions: () => req<{ version: string; champions: Champion[] }>('/api/champions'),
  build: (key: string, role?: string, mode?: string) => {
    const q = new URLSearchParams()
    if (role) q.set('role', role)
    if (mode) q.set('mode', mode)
    const qs = q.toString()
    return req<Build>(`/api/champions/${encodeURIComponent(key)}/build${qs ? `?${qs}` : ''}`)
  },
  buildsStatus: () => req<BuildsStatus>('/api/builds/status'),
  compile: (queue: 'ranked' | 'aram') => req<unknown>(`/api/builds/compile?queue=${queue}`, { method: 'POST' }),
  compileStop: () => req<unknown>('/api/builds/compile/stop', { method: 'POST' }),
  live: () => req<Live>('/api/live'),
  champSelect: () => req<ChampSelect>('/api/champselect'),
  swapBench: (id: number) => req<unknown>(`/api/champselect/swap/${id}`, { method: 'POST' }),
  reroll: () => req<unknown>('/api/champselect/reroll', { method: 'POST' }),
  trade: (id: number, accept = false) => req<unknown>(`/api/champselect/trade/${id}${accept ? '?accept=1' : ''}`, { method: 'POST' }),
  info: (key: string) => req<{ champion: Champion; info?: ChampionInfo }>(`/api/champions/${encodeURIComponent(key)}/info`),
  eog: () => req<Eog>('/api/eog'),
  session: () => req<Session>('/api/session'),
  mastery: () => req<Record<string, Mastery>>('/api/mastery'),
  applyRunes: (championId: number, page: RunePage) =>
    req<{ ok: boolean }>('/api/runes/apply', { method: 'POST', body: JSON.stringify({ championId, page }) }),
  spells: (key: string) => req<ChampionDetail>(`/api/champions/${encodeURIComponent(key)}/spells`),
  me: (championId: number, mode?: string) =>
    req<{ mastery: Mastery | null; record: PlayRecord | null; mode: string }>(`/api/me/${championId}${mode ? `?mode=${mode}` : ''}`),
}

export function subscribe(onStatus: (s: Status) => void, onLog: (l: LogEntry) => void): () => void {
  const es = new EventSource('/api/events')
  es.addEventListener('status', (e) => onStatus(JSON.parse((e as MessageEvent).data)))
  es.addEventListener('log', (e) => onLog(JSON.parse((e as MessageEvent).data)))
  return () => es.close()
}

export const ROLES = ['TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY'] as const
export const ROLE_LABEL: Record<string, string> = {
  TOP: 'Top',
  JUNGLE: 'Jungle',
  MIDDLE: 'Mid',
  BOTTOM: 'ADC',
  UTILITY: 'Support',
  UNKNOWN: '?',
}

export function winrate(c: { games: number; wins: number }): string {
  return c.games ? `${Math.round((c.wins / c.games) * 100)}%` : '–'
}

export function fmtTime(sec: number): string {
  const s = Math.max(0, Math.floor(sec))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

declare global {
  interface Window {
    ezlol?: { electron: boolean; platform: string; setOverlay?: (on: boolean) => Promise<boolean>; onToggleOverlay?: (fn: () => void) => void }
  }
}
export const isElectron = () => !!window.ezlol?.electron

export function fmtPoints(n: number): string {
  return n >= 1000 ? `${Math.round(n / 1000)}k` : String(n)
}

/** Rough ability rank at a champion level assuming a standard 5/5/5/3 path. */
export function estRank(level: number, key: string): number {
  if (key === 'R') return level >= 16 ? 3 : level >= 11 ? 2 : level >= 6 ? 1 : 0
  const r = level >= 16 ? 3 : level >= 11 ? 2 : level >= 6 ? 1 : 0
  return Math.max(1, Math.min(5, Math.round((level - r) / 3)))
}
