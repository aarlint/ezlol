<script setup lang="ts">
// ⌘K / Ctrl+K command palette: jump to any champion's build ("jinx", "ahri mid",
// "kaisa aram") or run an app action (theme, auto-accept, layout, settings).
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import type { Champion } from '../types'

export interface PaletteAction {
  id: string
  label: string
  group: string
  hint?: string
  keywords?: string
  run: () => void
}
/** A champion pick with an optional build mode / lane ("ahri mid"). */
export interface PalettePick {
  champion: Champion
  mode?: 'sr' | 'aram' | 'arena'
  role?: string
}

const props = defineProps<{ champions: Champion[]; actions: PaletteAction[] }>()
const emit = defineEmits<{ pick: [p: PalettePick] }>()

const open = ref(false)
const q = ref('')
const active = ref(0)
const input = ref<HTMLInputElement | null>(null)
const list = ref<HTMLElement | null>(null)
const isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)
const shortcut = isMac ? '⌘K' : 'Ctrl+K'
defineExpose({ show: () => (open.value = true), shortcut })

// Trailing words that pick a mode or lane for the champion.
const MODE_WORDS: Record<string, { mode: PalettePick['mode']; role?: string; label: string }> = {
  aram: { mode: 'aram', label: 'ARAM' },
  mayhem: { mode: 'aram', label: 'ARAM' },
  arena: { mode: 'arena', label: 'Arena' },
  sr: { mode: 'sr', label: "Summoner's Rift" },
  rift: { mode: 'sr', label: "Summoner's Rift" },
  top: { mode: 'sr', role: 'TOP', label: 'Top' },
  jg: { mode: 'sr', role: 'JUNGLE', label: 'Jungle' },
  jungle: { mode: 'sr', role: 'JUNGLE', label: 'Jungle' },
  mid: { mode: 'sr', role: 'MIDDLE', label: 'Mid' },
  middle: { mode: 'sr', role: 'MIDDLE', label: 'Mid' },
  bot: { mode: 'sr', role: 'BOTTOM', label: 'Bot' },
  adc: { mode: 'sr', role: 'BOTTOM', label: 'Bot' },
  sup: { mode: 'sr', role: 'UTILITY', label: 'Support' },
  support: { mode: 'sr', role: 'UTILITY', label: 'Support' },
}

const RECENT = 'ezlol.palette.recent'
function recent(): string[] {
  try {
    return JSON.parse(localStorage.getItem(RECENT) ?? '[]')
  } catch {
    return []
  }
}
function remember(key: string) {
  try {
    localStorage.setItem(RECENT, JSON.stringify([key, ...recent().filter((k) => k !== key)].slice(0, 6)))
  } catch {
    /* ignore */
  }
}

const norm = (s: string) => s.toLowerCase().replace(/[^a-z0-9]/g, '')
// Prefix beats word-start beats substring beats in-order letters (0 = no match).
function score(text: string, query: string): number {
  if (!query) return 1
  const t = text.toLowerCase()
  const nq = norm(query)
  const nt = norm(text)
  if (nt.startsWith(nq)) return 100 - nt.length / 10
  if (t.split(/[\s:'.&-]+/).some((w) => norm(w).startsWith(nq))) return 70
  if (nt.includes(nq)) return 50
  // Letters in order ("mf" → Miss Fortune), starting at a word start.
  if (!t.split(/[\s:'.&-]+/).some((w) => norm(w)[0] === nq[0])) return 0
  let i = 0
  for (const ch of nt) if (ch === nq[i]) i++
  return i === nq.length ? 20 : 0
}

// Keywords count only on a real word/substring match, never on scattered letters.
const kw = (s: number) => (s >= 50 ? s - 5 : 0)

interface Row {
  key: string
  kind: 'champion' | 'action'
  label: string
  sub?: string
  image?: string
  group: string
  run: () => void
}

const rows = computed<Row[]>(() => {
  const raw = q.value.trim()
  const words = raw.split(/\s+/).filter(Boolean)
  const last = words[words.length - 1]?.toLowerCase()
  const mod = words.length > 1 && last ? MODE_WORDS[last] : undefined
  const champQuery = mod ? words.slice(0, -1).join(' ') : raw
  const pick = (c: Champion): Row => ({
    key: `c:${c.id}`,
    kind: 'champion',
    label: c.name,
    sub: mod ? `${mod.label} build` : c.title,
    image: c.image,
    group: 'Champions',
    run: () => {
      remember(c.key)
      emit('pick', { champion: c, mode: mod?.mode, role: mod?.role })
    },
  })
  let champs: Row[]
  if (!raw) {
    champs = recent()
      .map((k) => props.champions.find((c) => c.key === k))
      .filter((c): c is Champion => !!c)
      .map((c) => ({ ...pick(c), group: 'Recent' }))
  } else {
    champs = props.champions
      .map((c) => ({ c, s: Math.max(score(c.name, champQuery), score(c.key, champQuery) - 1) }))
      .filter((x) => x.s > 0)
      .sort((a, b) => b.s - a.s || a.c.name.localeCompare(b.c.name))
      .slice(0, mod ? 6 : 8)
      .map((x) => pick(x.c))
  }
  const acts = props.actions
    .map((a) => ({ a, s: raw ? Math.max(score(a.label, raw), kw(score(a.keywords ?? '', raw)), score(`${a.group} ${a.label}`, raw) - 10) : 1 }))
    .filter((x) => x.s > 0 && !(mod && raw))
    .sort((a, b) => (raw ? b.s - a.s : 0))
    .map(({ a }): Row => ({ key: `a:${a.id}`, kind: 'action', label: a.label, sub: a.hint, group: raw ? 'Actions' : a.group, run: a.run }))
  // A query that names an action more precisely than any champion lists actions first.
  const topChamp = champs.length ? score(champs[0].label, champQuery) : 0
  const topAct = acts.length && raw ? score(acts[0].label, raw) : 0
  return topAct > topChamp ? [...acts, ...champs] : [...champs, ...acts]
})

// Group headers, in row order.
const grouped = computed(() => {
  const out: { group: string; rows: { row: Row; i: number }[] }[] = []
  rows.value.forEach((row, i) => {
    let g = out[out.length - 1]
    if (!g || g.group !== row.group) out.push((g = { group: row.group, rows: [] }))
    g.rows.push({ row, i })
  })
  return out
})

watch(q, () => (active.value = 0))
watch(open, async (o) => {
  if (!o) return
  q.value = ''
  active.value = 0
  await nextTick()
  input.value?.focus()
})
watch(active, async () => {
  await nextTick()
  list.value?.querySelector<HTMLElement>('.prow.on')?.scrollIntoView({ block: 'nearest' })
})

function run(i: number) {
  const r = rows.value[i]
  if (!r) return
  open.value = false
  r.run()
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = Math.min(rows.value.length - 1, active.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = Math.max(0, active.value - 1)
  } else if (e.key === 'Enter') {
    e.preventDefault()
    run(active.value)
  } else if (e.key === 'Escape') {
    e.preventDefault()
    open.value = false
  }
}
function onGlobal(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && !e.altKey && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    open.value = !open.value
    return
  }
  // "/" opens it too, unless you are typing somewhere.
  const el = e.target as HTMLElement | null
  if (e.key === '/' && !open.value && !(el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName)))) {
    e.preventDefault()
    open.value = true
  }
}
onMounted(() => document.addEventListener('keydown', onGlobal))
onUnmounted(() => document.removeEventListener('keydown', onGlobal))
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="palette-scrim" @mousedown.self="open = false">
      <div class="palette" role="dialog" aria-modal="true" aria-label="Command palette">
        <div class="pin">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
          <input
            ref="input"
            v-model="q"
            placeholder="Champion (try “ahri mid”, “jinx aram”) or an action…"
            role="combobox"
            aria-controls="palette-list"
            :aria-activedescendant="rows[active] ? `pal-${active}` : undefined"
            aria-expanded="true"
            @keydown="onKey"
          />
          <kbd>esc</kbd>
        </div>
        <div id="palette-list" ref="list" class="plist" role="listbox">
          <template v-for="g in grouped" :key="g.group + g.rows[0].i">
            <div class="pgroup">{{ g.group }}</div>
            <div
              v-for="{ row, i } in g.rows"
              :id="`pal-${i}`"
              :key="row.key"
              class="prow"
              :class="{ on: i === active }"
              role="option"
              :aria-selected="i === active"
              @mousemove="active = i"
              @click="run(i)"
            >
              <img v-if="row.image" :src="row.image" alt="" />
              <span v-else class="picon">›</span>
              <span class="plabel">{{ row.label }}</span>
              <span v-if="row.sub" class="psub">{{ row.sub }}</span>
            </div>
          </template>
          <div v-if="!rows.length" class="pempty">Nothing matches “{{ q }}”.</div>
        </div>
        <div class="pfoot"><span><kbd>↑</kbd><kbd>↓</kbd> move</span><span><kbd>↵</kbd> open</span><span><kbd>{{ shortcut }}</kbd> or <kbd>/</kbd> toggle</span></div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.palette-scrim { position: fixed; inset: 0; z-index: 1000; background: rgba(0, 0, 0, .45); display: flex; justify-content: center; align-items: flex-start; padding: 12vh 16px 16px; }
.palette { width: min(560px, 100%); max-height: 70vh; display: flex; flex-direction: column; background: var(--panel-2); border: 1px solid var(--gold-deep); border-radius: var(--radius-sm); box-shadow: var(--shadow-panel), 0 0 40px var(--accent-glow); overflow: hidden; }
.pin { display: flex; align-items: center; gap: 8px; padding: 10px 12px; border-bottom: 1px solid var(--line); color: var(--muted); }
.pin input { flex: 1; width: auto; border: 0; background: transparent; box-shadow: none; font-size: 15px; padding: 4px 0; color: var(--text); outline: none; }
.plist { overflow-y: auto; padding: 4px 0; min-height: 0; }
.pgroup { font-size: 10px; letter-spacing: .14em; text-transform: uppercase; color: var(--dim); padding: 8px 14px 4px; font-family: var(--display); }
.prow { display: flex; align-items: center; gap: 10px; padding: 6px 14px; cursor: pointer; border-left: 2px solid transparent; }
.prow.on { background: var(--accent-soft); border-left-color: var(--hextech-2); }
.prow img { width: 28px; height: 28px; border: 1px solid var(--gold-deep); }
.picon { width: 28px; text-align: center; color: var(--gold); font-family: var(--display); }
.plabel { font-weight: 600; white-space: nowrap; }
.psub { color: var(--muted); font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-left: auto; }
.pempty { padding: 14px; color: var(--muted); }
.pfoot { display: flex; gap: 14px; padding: 7px 12px; border-top: 1px solid var(--line); font-size: 11px; color: var(--dim); flex-wrap: wrap; }
kbd { font-family: var(--display); font-size: 10px; padding: 1px 5px; border: 1px solid var(--gold-deep); border-radius: 3px; color: var(--muted); background: var(--surface-input); margin-right: 3px; }
@media (max-width: 600px) { .psub { display: none; } }
</style>
