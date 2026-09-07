<script setup lang="ts">
import Widget from './Widget.vue'
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../api'
import type { BuildsStatus } from '../types'

const st = ref<BuildsStatus | null>(null)
const err = ref('')
let timer: number | undefined

async function refresh() {
  try {
    st.value = await api.buildsStatus()
  } catch (e) {
    err.value = (e as Error).message
  }
}
async function start(queue: 'ranked' | 'aram') {
  err.value = ''
  try {
    await api.compile(queue)
    await refresh()
  } catch (e) {
    err.value = (e as Error).message
  }
}
async function stop() {
  await api.compileStop()
  await refresh()
}
onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 3000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <Widget id="compile" :w="3">
    <h2>Build data</h2>
    <template v-if="st">
      <div class="kv">
        <span>Riot API key</span><b>{{ st.hasKey ? 'configured' : 'not set' }}</b>
        <span>Platform</span><b>{{ st.platform }}</b>
        <template v-for="p in st.patches ?? []" :key="p.patch">
          <span>Patch {{ p.patch }}</span><b>{{ p.matches }} matches · {{ p.champions }} champs</b>
        </template>
      </div>
      <div v-if="st.progress.running" style="margin-top: 10px">
        <div class="muted">{{ st.progress.queue === 450 ? 'ARAM' : 'Ranked' }} · {{ st.progress.phase }} · {{ st.progress.requests }} requests · {{ st.progress.errors }} errors</div>
        <div class="progress"><div :style="{ width: `${(100 * st.progress.matchesDone) / Math.max(1, st.progress.target)}%` }" /></div>
        <div class="row">
          <span class="muted">{{ st.progress.matchesDone }} / {{ st.progress.target }} matches</span>
          <button class="danger" @click="stop">Stop</button>
        </div>
      </div>
      <div v-else class="row" style="margin-top: 10px">
        <button class="primary" :disabled="!st.hasKey" @click="start('ranked')">Compile ranked</button>
        <button class="primary" :disabled="!st.hasKey" @click="start('aram')">Compile ARAM</button>
        <span v-if="!st.hasKey" class="muted">Set <code>RIOT_API_KEY</code> and restart to enable.</span>
      </div>
      <div v-if="st.progress.lastError" class="note">{{ st.progress.lastError }}</div>
    </template>
    <div v-if="err" class="note">{{ err }}</div>
  </Widget>
</template>
