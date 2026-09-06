<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
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

    <div class="layout" :class="{ ingame: status?.phase === 'InProgress' || status?.phase === 'ChampSelect' }">
      <div class="side" style="display: grid; gap: 16px">
        <QueuePanel :status="status" :logs="logs" @select="select" />
        <ChampionPicker :champions="champions" :selected="selected" @select="select" />
        <CompilePanel />
      </div>
      <div style="display: grid; gap: 16px">
        <ChampSelect v-if="status?.phase === 'ChampSelect'" @preview="select" />
        <EndOfGame v-if="!['ChampSelect', 'InProgress', 'ReadyCheck'].includes(status?.phase ?? '')" />
        <!-- In game: build (augments first) on top, scoreboard below -->
        <BuildPanel v-if="status?.phase === 'InProgress'" :champion="selected" :status="status" v-model:follow="followPick" />
        <LiveGame v-if="status?.phase === 'InProgress'" @me="(c) => followPick && select(c)" />
        <BuildPanel v-else :champion="selected" :status="status" v-model:follow="followPick" />
      </div>
    </div>
  </div>
</template>
