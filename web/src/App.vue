<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, isElectron, subscribe } from './api'
import * as sound from './sound'
import type { Champion, LogEntry, Status } from './types'
import QueuePanel from './components/QueuePanel.vue'
import ChampionPicker from './components/ChampionPicker.vue'
import BuildPanel from './components/BuildPanel.vue'
import CompilePanel from './components/CompilePanel.vue'
import LiveGame from './components/LiveGame.vue'
import ChampSelect from './components/ChampSelect.vue'
import EndOfGame from './components/EndOfGame.vue'

const status = ref<Status | null>(null)
const logs = ref<LogEntry[]>([])
const champions = ref<Champion[]>([])
const selected = ref<Champion | null>(null)
const followPick = ref(true)
const soundOn = ref(sound.enabled())
const overlay = ref(false)
const electron = isElectron()
const autoOverlay = ref(localStorage.getItem('ezlol.autoOverlay') === 'on')
watch(autoOverlay, (v) => localStorage.setItem('ezlol.autoOverlay', v ? 'on' : 'off'))
async function setOverlay(on: boolean) {
  if (overlay.value === on) return
  overlay.value = on
  await window.ezlol?.setOverlay?.(on)
  document.body.classList.toggle('overlay', on)
}
const toggleOverlay = () => setOverlay(!overlay.value)
// Page mode drives the layout: idle (lobby, queue, post-game) shows the side
// column; select and game modes drop it and spread the panels out.
const forcedMode = new URLSearchParams(location.search).get('mode') // dev: ?mode=game|select|idle
const mode = computed(() => {
  if (forcedMode === 'game' || forcedMode === 'select' || forcedMode === 'idle') return forcedMode
  const p = status.value?.phase
  if (p === 'InProgress') return 'game'
  if (p === 'ChampSelect') return 'select'
  return 'idle'
})
const error = ref('')

let unsub: (() => void) | null = null

function onStatus(s: Status) {
  const prev = status.value
  status.value = s
  if (followPick.value && s.pickedChampion) {
    const c = champions.value.find((x) => x.id === s.pickedChampion)
    if (c && selected.value?.id !== c.id) selected.value = c
  }
  if (prev && prev.phase !== s.phase && electron && autoOverlay.value) {
    if (s.phase === 'InProgress') setOverlay(true)
    else if (prev.phase === 'InProgress') setOverlay(false)
  }
  if (prev && prev.phase !== s.phase) {
    if (s.phase === 'ReadyCheck') {
      sound.queuePop()
      sound.notify('ezlol', s.autoAccept ? 'Queue popped — accepting' : 'Queue popped!')
    } else if (s.phase === 'ChampSelect') {
      sound.alert()
      sound.notify('ezlol', 'Champ select')
    }
  }
}

function onLog(l: LogEntry) {
  logs.value = [...logs.value.slice(-199), l]
  if (l.level === 'accept') sound.accepted()
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
  window.ezlol?.onToggleOverlay?.(() => toggleOverlay())
  sound.notify('', '') // triggers the permission prompt once
})
onUnmounted(() => unsub?.())

// Masonry packing: the dashboard grid uses tiny implicit rows and every panel
// spans as many as its rendered height needs, so boxes of different heights
// pack tightly instead of leaving row-height gaps.
const ROW = 8
const GAP = 10
const mainEl = ref<HTMLElement | null>(null)
let ro: ResizeObserver | null = null
let mo: MutationObserver | null = null
function pack(el: Element) {
  const p = el as HTMLElement
  // Measure content height with the span removed so growth and shrink both register.
  p.style.gridRowEnd = 'span 1'
  const h = p.scrollHeight
  p.style.gridRowEnd = `span ${Math.max(1, Math.ceil((h + GAP) / (ROW + GAP)))}`
}
let packing = false
let lastZoom = 1
function packAll() {
  if (!mainEl.value || packing) return
  packing = true
  const m = mainEl.value
  // Measure unscaled, then fit: if the packed grid is taller than the window,
  // scale the whole dashboard down so every box stays on screen (no scrolling in game).
  const z = (m.style as unknown as { zoom: string }).zoom
  ;(m.style as unknown as { zoom: string }).zoom = '1'
  for (const child of Array.from(m.children)) pack(child)
  const avail = window.innerHeight - m.getBoundingClientRect().top - 12
  const need = m.scrollHeight
  let zoom = need > avail ? Math.max(0.55, avail / need) : 1
  zoom = Math.round(zoom * 100) / 100
  if (Math.abs(zoom - lastZoom) < 0.02 && z) zoom = lastZoom
  lastZoom = zoom
  ;(m.style as unknown as { zoom: string }).zoom = String(zoom)
  packing = false
}
window.addEventListener('resize', () => requestAnimationFrame(packAll))
function observeAll() {
  if (!mainEl.value || !ro) return
  ro.disconnect()
  for (const child of Array.from(mainEl.value.children)) ro.observe(child)
  packAll()
}
onMounted(() => {
  ro = new ResizeObserver(() => requestAnimationFrame(packAll))
  mo = new MutationObserver(() => observeAll())
  if (mainEl.value) mo.observe(mainEl.value, { childList: true })
  observeAll()
})
onUnmounted(() => {
  ro?.disconnect()
  mo?.disconnect()
})
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
    <div v-if="electron" class="dragbar" />
    <header class="topbar">
      <div class="brand">ez<span>lol</span></div>
      <span v-if="status" class="pill" :class="status.connected ? 'ok' : 'bad'">
        <i class="dot" />{{ status.connected ? `${status.summoner || 'connected'}` : 'Client not running' }}
      </span>
      <span v-if="status?.patch" class="pill"><i class="dot" />Patch {{ status.patch }}</span>
      <span v-if="status?.queueName" class="pill warn"><i class="dot" />{{ status.queueName }}</span>
      <div class="grow" />
      <label v-if="electron" class="toggle" :class="{ on: overlay }" @click="toggleOverlay" title="Compact always-on-top window for in-game use">
        <span class="track" /><span>Overlay</span>
      </label>
      <label v-if="electron" class="toggle" :class="{ on: autoOverlay }" @click="autoOverlay = !autoOverlay" title="Switch to overlay when a game starts, back when it ends">
        <span class="track" /><span>Auto</span>
      </label>
      <label class="toggle" :class="{ on: soundOn }" @click="soundOn = !soundOn">
        <span class="track" /><span>Sound</span>
      </label>
      <label v-if="status" class="toggle" :class="{ on: status.autoAccept }" @click="toggleAuto">
        <span class="track" />
        <span>Auto-accept {{ status.autoAccept ? 'ON' : 'OFF' }}</span>
      </label>
    </header>

    <div v-if="error" class="note">{{ error }}</div>

    <div class="layout" :class="mode">
      <div ref="mainEl" class="main" :class="mode">
        <QueuePanel v-if="mode === 'idle'" :status="status" :logs="logs" @select="select" />
        <EndOfGame v-if="mode === 'idle'" />
        <ChampSelect v-if="mode === 'select'" @preview="select" />
        <!-- In game: live boxes first, build boxes flow in after them -->
        <LiveGame v-if="mode === 'game'" @me="(c) => followPick && select(c)" />
        <BuildPanel :champion="selected" :status="status" :compact="mode === 'game'" v-model:follow="followPick" />
        <ChampionPicker v-if="mode === 'idle'" :champions="champions" :selected="selected" @select="select" />
        <CompilePanel v-if="mode === 'idle'" />
      </div>
    </div>
  </div>
</template>
