<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { CATALOG, DASH_KEY, DashboardGrid, type ModeKey } from './dashboard'
import GhostWidget from './components/GhostWidget.vue'
import { api, isElectron, subscribe } from './api'
import * as sound from './sound'
import type { Champion, LogEntry, Status } from './types'
import QueuePanel from './components/QueuePanel.vue'
import BuildPanel from './components/BuildPanel.vue'
import ChampionBar from './components/ChampionBar.vue'
import CompilePanel from './components/CompilePanel.vue'
import LiveGame from './components/LiveGame.vue'
import ChampSelect from './components/ChampSelect.vue'
import EndOfGame from './components/EndOfGame.vue'
import SettingsModal from './components/SettingsModal.vue'
import Toasts from './components/Toasts.vue'
import ThemeMenu from './components/ThemeMenu.vue'
import type { ThemeOption } from './components/ThemeMenu.vue'
import { toast } from './toast'
import type { UpdateInfo } from './types'

const params = new URLSearchParams(location.search)
const status = ref<Status | null>(null)
const logs = ref<LogEntry[]>([])
const champions = ref<Champion[]>([])
const selected = ref<Champion | null>(null)
const followPick = ref(true)
const soundOn = ref(sound.enabled())
const showSettings = ref(false)
const THEMES: ThemeOption[] = [
  { id: 'hextech', label: 'Hextech', swatch: ['#010a13', '#c8aa6e', '#0ac8b9'] },
  { id: 'linear', label: 'Linear dark', swatch: ['#0b0c10', '#141519', '#7c7cff'] },
  { id: 'neon', label: 'Neon', swatch: ['#05060d', '#ff2bd6', '#00f0ff'] },
  { id: 'synthwave', label: 'Synthwave', swatch: ['#170a2b', '#ff2d95', '#ffd166'] },
  { id: 'terminal', label: 'Terminal', swatch: ['#000000', '#1f3a1f', '#39ff14'] },
  { id: 'sketch', label: 'Sketch', swatch: ['#f6f1e7', '#2b2b2b', '#ffe066'] },
]
const themeParam = params.get('theme')
const theme = ref<string>(
  THEMES.some((t) => t.id === themeParam) ? (themeParam as string) : THEMES.some((t) => t.id === localStorage.getItem('ezlol.theme')) ? (localStorage.getItem('ezlol.theme') as string) : 'hextech',
)
if (params.get('open') === 'settings') setTimeout(() => (showSettings.value = true), 800)
watch(
  theme,
  (t) => {
    document.documentElement.dataset.theme = t
    localStorage.setItem('ezlol.theme', t)
  },
  { immediate: true },
)
const upd = ref<UpdateInfo | null>(null)
const electronUpdate = ref<{ status: string; version?: string; error?: string; progress?: number } | null>(null)
const installUpdate = () => window.ezlol?.installUpdate?.()
let announcedUpdate = ''
async function checkUpdateSoon() {
  try {
    upd.value = await api.update()
    if (!upd.value.checkedAt || upd.value.checkedAt.startsWith('0001')) upd.value = await api.checkUpdate()
    const u = upd.value
    if (u.hasUpdate && announcedUpdate !== u.latest && electronUpdate.value?.status !== 'downloading' && electronUpdate.value?.status !== 'downloaded') {
      announcedUpdate = u.latest
      toast({ key: 'update', kind: 'info', sticky: true, title: `ezlol ${u.latest} available`, body: `You have ${u.current}.`, actions: [{ label: 'Download', primary: true, run: () => window.open(u.url, '_blank') }] })
    }
  } catch {
    /* offline */
  }
}
watch(electronUpdate, (st) => {
  if (!st) return
  if (st.status === 'downloaded') toast({ key: 'update', kind: 'success', sticky: true, title: `ezlol ${st.version} is ready`, body: 'Downloaded and verified. Installing swaps the app and relaunches it.', actions: [{ label: 'Install & restart', primary: true, run: installUpdate }] })
  else if (st.status === 'downloading') toast({ key: 'update', kind: 'info', sticky: true, title: `Downloading ${st.version}…`, body: st.progress ? `${Math.round(st.progress * 100)}%` : undefined })
  else if (st.status === 'error') toast({ key: 'update', kind: 'warn', title: 'Auto-update failed', body: `${st.error ?? ''} — use Download instead.`, actions: upd.value?.url ? [{ label: 'Download', run: () => window.open(upd.value!.url, '_blank') }] : undefined })
})
const electron = isElectron()
// Page mode drives the layout: idle (lobby, queue, post-game) shows the side
// column; select and game modes drop it and spread the panels out.
const forcedMode = params.get('mode') // dev: ?mode=game|select|idle
const mode = computed(() => {
  if (forcedMode === 'game' || forcedMode === 'select' || forcedMode === 'idle') return forcedMode
  const p = status.value?.phase
  if (p === 'InProgress') return 'game'
  if (p === 'ChampSelect') return 'select'
  return 'idle'
})
const error = ref('')
watch(error, (e) => e && toast({ key: 'app-error', kind: 'error', title: 'ezlol', body: e }))

let unsub: (() => void) | null = null

function onStatus(s: Status) {
  const prev = status.value
  status.value = s
  if (followPick.value && s.pickedChampion) {
    const c = champions.value.find((x) => x.id === s.pickedChampion)
    if (c && selected.value?.id !== c.id) selected.value = c
  }
  if (prev && prev.phase !== s.phase) {
    if (s.phase === 'ReadyCheck') {
      sound.queuePop()
      sound.notify('ezlol', s.autoAccept ? 'Queue popped — accepting' : 'Queue popped!')
      toast({ key: 'phase', kind: s.autoAccept ? 'success' : 'warn', title: 'Queue popped', body: s.autoAccept ? 'Accepting…' : 'Auto-accept is off — accept in the client.', ttl: 12000 })
    } else if (s.phase === 'ChampSelect') {
      sound.alert()
      sound.notify('ezlol', 'Champ select')
      toast({ key: 'phase', kind: 'info', title: 'Champ select', ttl: 5000 })
    }
  }
}

function onLog(l: LogEntry) {
  logs.value = [...logs.value.slice(-199), l]
  if (l.level === 'accept') {
    sound.accepted()
    toast({ key: 'phase', kind: 'success', title: 'Accepted', body: l.message, ttl: 6000 })
  }
}

onMounted(async () => {
  try {
    const [st, ch] = await Promise.all([api.status(), api.champions()])
    champions.value = ch.champions
    logs.value = st.logs ?? []
    onStatus(st)
    const saved = localStorage.getItem('ezlol.champion')
    if (!selected.value && saved) selected.value = champions.value.find((c) => c.key === saved) ?? null
  } catch (e) {
    error.value = (e as Error).message
  }
  unsub = subscribe(onStatus, onLog)
  if (!params.get('quiet')) {
    setTimeout(checkUpdateSoon, 4000)
    setInterval(checkUpdateSoon, 6 * 3600 * 1000)
  }
  window.ezlol?.onUpdate?.((st) => (electronUpdate.value = st))
  sound.notify('', '') // triggers the permission prompt once
})
onUnmounted(() => unsub?.())

// Widget dashboard: one grid per screen mode; widgets register on mount.
const ARAM_QUEUES = new Set([450, 2400, 100])
const ARENA_QUEUES = new Set([1700, 1710, 1750])
// In-game screens differ by mode, so each gets its own default layout and saved arrangement.
const modeKey = computed<ModeKey>(() => {
  if (mode.value !== 'game') return mode.value
  const q = status.value?.queueId ?? 0
  const map = status.value?.mapId ?? 0
  if (ARENA_QUEUES.has(q) || map === 30) return 'game-arena'
  if (ARAM_QUEUES.has(q) || map === 12) return 'game-aram'
  return 'game-rift'
})
const dash = new DashboardGrid(() => modeKey.value)
provide(DASH_KEY, dash)
const mainEl = ref<HTMLElement | null>(null)
const editing = ref(false)
// Ghost slots: every catalogued box for this screen that is not mounted right now.
const ghosts = computed(() => (editing.value ? CATALOG[modeKey.value].filter((w) => !dash.mounted.has(w.id)) : []))
const layoutVersion = ref(0)
function attachGrid() {
  if (mainEl.value) dash.attach(mainEl.value)
}
onMounted(attachGrid)
watch([modeKey, layoutVersion], async () => {
  await nextTick()
  attachGrid()
})
watch(editing, (v) => dash.setEditing(v))
function resetLayout() {
  const hadPreset = dash.hasPreset()
  dash.reset()
  layoutVersion.value++
  toast({ key: 'layout', kind: 'info', title: hadPreset ? 'Layout restored' : 'Layout reset', body: hadPreset ? 'Back to your saved arrangement for this screen.' : 'Back to the built-in arrangement (no saved layout for this screen).', ttl: 4000 })
}
const hasPreset = ref(false)
function saveLayout() {
  dash.savePreset()
  hasPreset.value = true
  toast({ key: 'layout', kind: 'success', title: 'Layout saved', body: 'Reset now returns to this arrangement on this screen.', ttl: 4000 })
}
function forgetLayout() {
  dash.clearPreset()
  hasPreset.value = false
  toast({ key: 'layout', kind: 'info', title: 'Saved layout forgotten', body: 'Reset will use the built-in arrangement again.', ttl: 4000 })
}
watch([editing, modeKey], () => (hasPreset.value = dash.hasPreset()))
watch(soundOn, (v) => sound.setEnabled(v))

function select(c: Champion) {
  selected.value = c
  localStorage.setItem('ezlol.champion', c.key)
}

async function toggleAuto() {
  if (!status.value) return
  status.value = await api.setAutoAccept(!status.value.autoAccept)
}
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">ez<span>lol</span></div>
      <span v-if="status" class="pill" :class="status.connected ? 'ok' : 'bad'">
        <i class="dot" />{{ status.connected ? `${status.summoner || 'connected'}` : 'Client not running' }}
      </span>
      <span v-if="status?.patch" class="pill"><i class="dot" />Patch {{ status.patch }}</span>
      <span v-if="status?.queueName" class="pill warn"><i class="dot" />{{ status.queueName }}</span>
      <div class="grow" />
      <label class="toggle" :class="{ on: editing }" title="Drag and resize the boxes; layout is saved per screen" @click="editing = !editing">
        <span class="track" /><span>Edit layout</span>
      </label>
      <button v-if="editing" class="primary" title="Remember this arrangement for this screen" @click="saveLayout">Save layout</button>
      <button v-if="editing" :title="hasPreset ? 'Back to your saved arrangement' : 'Back to the built-in arrangement'" @click="resetLayout">Reset</button>
      <button v-if="editing && hasPreset" class="danger" title="Forget the saved arrangement for this screen" @click="forgetLayout">Forget saved</button>
      <label class="toggle" :class="{ on: soundOn }" @click="soundOn = !soundOn">
        <span class="track" /><span>Sound</span>
      </label>
      <label v-if="status" class="toggle" :class="{ on: status.autoAccept }" @click="toggleAuto">
        <span class="track" />
        <span>Auto-accept {{ status.autoAccept ? 'ON' : 'OFF' }}</span>
      </label>
      <ThemeMenu v-model="theme" :options="THEMES" />
      <button class="gear" title="Settings" aria-label="Settings" @click="showSettings = true">
        <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
      </button>
    </header>

    <Toasts />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />

    <!-- Champion card: fixed bar with the champion picker as a dropdown, never part of the grid below -->
    <ChampionBar :champion="selected" :champions="champions" :status="status" :screen="mode" :compact="mode === 'game'" v-model:follow="followPick" @select="select" />

    <div class="layout" :class="mode">
      <div ref="mainEl" :key="modeKey + ':' + layoutVersion" class="main grid-stack" :class="[mode, { editing }]">
        <QueuePanel v-if="mode === 'idle'" :status="status" :logs="logs" @select="select" />
        <EndOfGame v-if="mode === 'idle'" />
        <ChampSelect v-if="mode === 'select'" @preview="select" />
        <!-- In game: live boxes first, build boxes flow in after them -->
        <LiveGame v-if="mode === 'game'" @me="(c) => followPick && select(c)" />
        <BuildPanel :status="status" :compact="mode === 'game'" />
        <CompilePanel v-if="mode === 'idle'" />
        <GhostWidget v-for="g in ghosts" :key="'ghost-' + g.id" :spec="g" />
      </div>
    </div>
  </div>
</template>
