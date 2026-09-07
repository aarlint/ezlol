<script setup lang="ts">
import Widget from './Widget.vue'
import { computed, onMounted, ref } from 'vue'
import { api, fmtPoints } from '../api'
import type { Champion, Mastery } from '../types'

const props = defineProps<{ champions: Champion[]; selected: Champion | null }>()
const emit = defineEmits<{ select: [c: Champion] }>()
const q = ref('')
const sortBy = ref<'name' | 'mastery'>((localStorage.getItem('ezlol.sort') as 'name' | 'mastery') || 'name')
const mastery = ref<Record<string, Mastery>>({})
onMounted(async () => {
  try {
    mastery.value = await api.mastery()
  } catch {
    /* client not running */
  }
})
function setSort(v: 'name' | 'mastery') {
  sortBy.value = v
  localStorage.setItem('ezlol.sort', v)
}
const m = (c: Champion) => mastery.value[String(c.id)]

const filtered = computed(() => {
  const s = q.value.trim().toLowerCase()
  let list = s ? props.champions.filter((c) => c.name.toLowerCase().includes(s) || c.key.toLowerCase().includes(s)) : [...props.champions]
  if (sortBy.value === 'mastery') list = list.sort((a, b) => (m(b)?.championPoints ?? 0) - (m(a)?.championPoints ?? 0))
  return list
})

function onEnter() {
  if (filtered.value.length) emit('select', filtered.value[0])
}
</script>

<template>
  <Widget id="picker" :w="3">
    <h2>Champion</h2>
    <div class="row">
      <input v-model="q" placeholder="Search champion…" @keydown.enter="onEnter" style="flex: 1; width: auto" />
      <button class="sm" :class="{ active: sortBy === 'name' }" @click="setSort('name')" title="Sort by name">A–Z</button>
      <button class="sm" :class="{ active: sortBy === 'mastery' }" @click="setSort('mastery')" title="Sort by your mastery">M</button>
    </div>
    <div class="champ-grid">
      <div v-for="c in filtered" :key="c.id" class="cell" :class="{ active: selected?.id === c.id }" :title="`${c.name}${m(c) ? ` · M${m(c).championLevel} · ${fmtPoints(m(c).championPoints)}` : ''}`" @click="emit('select', c)">
        <img :src="c.image" :alt="c.name" loading="lazy" />
        <span v-if="m(c)?.championLevel" class="m">{{ m(c).championLevel }}</span>
      </div>
    </div>
  </Widget>
</template>

<style scoped>
.row { flex-wrap: nowrap; }
button.sm { padding: 6px 8px; font-size: 10px; }
.cell { position: relative; cursor: pointer; }
.cell img { width: 100%; aspect-ratio: 1; border: 1px solid var(--gold-deep); display: block; filter: saturate(.85); transition: all .12s; }
.cell:hover img { border-color: var(--gold); filter: saturate(1.1); transform: scale(1.06); }
.cell.active img { border-color: var(--hextech-2); box-shadow: 0 0 10px var(--accent-glow); filter: saturate(1.1); }
.m { position: absolute; right: 1px; bottom: 1px; font-size: 9px; font-family: var(--display); color: var(--gold); background: var(--pill-bg); padding: 0 3px; border: 1px solid var(--gold-deep); }
</style>
