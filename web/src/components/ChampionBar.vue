<script setup lang="ts">
// Fixed champion bar above the widget grid: portrait, mode / lane buttons,
// source and tier, your mastery and record, the Follow switch. It drives the
// shared build state that the build widgets read; it is never part of the grid.
import { computed, watch } from 'vue'
import { fmtPoints, ROLES, ROLE_LABEL, winrate } from '../api'
import { buildState as s, loadBuild, pickMode, pickRole } from '../build'
import type { Champion, Status } from '../types'

const props = defineProps<{ champion: Champion | null; status: Status | null; screen: 'idle' | 'select' | 'game'; compact?: boolean }>()
const follow = defineModel<boolean>('follow', { default: true })

const build = computed(() => s.build)
const info = computed(() => s.info)
const me = computed(() => s.me)
const DMG: Record<string, string> = { kMagic: 'AP', kPhysical: 'AD', kMixed: 'Mixed' }
const ATK: Record<string, string> = { kRanged: 'ranged', kMelee: 'melee' }
const PIPS = ['damage', 'durability', 'crowdControl', 'mobility', 'utility'] as const

watch(
  () => props.champion?.id,
  () => {
    s.role = ''
    void loadBuild(props.champion)
  },
  { immediate: true },
)
// A forced mode or lane is for browsing in the lobby; a new screen (champ
// select, game, back to lobby) goes back to following the client's queue.
watch(
  () => props.screen,
  () => {
    s.mode = ''
    s.role = ''
    void loadBuild()
  },
)
// The assigned lane usually arrives before the pick, so watch both together.
watch(
  () => [props.status?.pickedChampion, props.status?.pickedPosition] as const,
  ([c, p]) => {
    if (follow.value && p && c === props.champion?.id && p !== s.role) {
      s.role = p
      void loadBuild()
    }
  },
)
watch(
  () => props.status?.connected,
  (c, prev) => {
    if (c && !prev) void loadBuild()
  },
)
watch(
  () => props.status?.queueId,
  () => {
    if (!s.mode) void loadBuild()
  },
)

const pctf = (x: number) => `${Math.round(x * 100)}%`
const tierLabel = (t?: number) => (t ? ['', 'S', 'A', 'B', 'C', 'D'][t] ?? String(t) : '')
const patchOf = (p: string) => p.replace('-aram', '').replace('-arena', '')
const games = (r: string) => build.value?.roles?.find((x) => x.role === r)?.games
</script>

<template>
  <!-- Loading modal: fixed overlay, never affects the layout -->
  <Teleport to="body">
    <div v-if="s.loading && s.loadingChamp" class="load-backdrop" aria-live="polite" aria-busy="true">
      <div class="load-card">
        <img :src="s.loadingChamp.image" :alt="s.loadingChamp.name" />
        <div class="load-name">{{ s.loadingChamp.name }}</div>
        <div class="load-sub">Loading build…</div>
        <div class="spinner" />
      </div>
    </div>
  </Teleport>

  <section class="panel champbar" :class="{ compact, empty: !build }" aria-label="Champion">
    <template v-if="build">
      <img class="portrait" :src="build.champion.image" :alt="build.champion.name" />
      <div class="rows">
        <div class="row1">
          <div class="who">
            <div class="name">{{ build.champion.name }}</div>
            <div class="title">
              {{ build.champion.title }} · patch {{ patchOf(build.patch) }}<span v-if="build.mode === 'aram'"> · Howling Abyss</span><span v-else-if="build.mode === 'arena'"> · Arena</span>
            </div>
          </div>
          <div class="roles">
            <button :class="{ active: build.mode === 'sr' }" @click="pickMode('sr')">Rift</button>
            <button :class="{ active: build.mode === 'aram' }" @click="pickMode('aram')">ARAM</button>
            <button :class="{ active: build.mode === 'arena' }" @click="pickMode('arena')">Arena</button>
            <template v-if="build.mode !== 'aram' && build.mode !== 'arena'">
              <span class="sep" />
              <button v-for="r in ROLES" :key="r" class="sm" :class="{ active: s.role === r }" @click="pickRole(r)">
                {{ ROLE_LABEL[r] }}<span v-if="games(r)" class="muted"> {{ games(r) }}</span>
              </button>
            </template>
          </div>
        </div>
        <div class="row2">
          <div class="facts">
            <span class="badge" :class="build.source">
              {{ build.source === 'opgg' ? (build.mode === 'arena' ? 'op.gg Arena stats' : build.mode === 'aram' ? 'op.gg ARAM stats' : 'op.gg ranked stats') : build.source === 'riot' ? 'Compiled from ranked matches' : build.source === 'lcu' ? 'Riot in-client recommendations' : 'No data' }}
            </span>
            <span v-if="build.tier" class="badge tier" :class="'t' + build.tier">Tier {{ tierLabel(build.tier) }} · #{{ build.rank }}<template v-if="build.pickRate"> · {{ pctf(build.pickRate) }} pick</template></span>
            <span v-if="build.total.games && build.mode !== 'arena'" class="muted">{{ build.total.games }} games · {{ winrate(build.total) }} win rate</span>
            <span v-else-if="build.total.games" class="muted">{{ build.total.games }} games · avg place {{ (build.avgPlace ?? 0).toFixed(2) }} · {{ pctf(build.top1 ?? 0) }} first</span>
            <span v-if="me?.mastery" class="badge lcu" :title="`Highest grade ${me.mastery.highestGrade}`">You: M{{ me.mastery.championLevel }} · {{ fmtPoints(me.mastery.championPoints) }}</span>
            <span v-if="me?.record" class="badge riot">
              Recent {{ build.mode === 'aram' ? 'ARAM' : 'Rift' }}: {{ me.record.wins }}W {{ me.record.games - me.record.wins }}L ·
              {{ ((me.record.kills + me.record.assists) / Math.max(1, me.record.deaths)).toFixed(1) }} KDA
            </span>
          </div>
          <div v-if="info && !compact" class="playstyle">
            <span class="badge" :class="info.tacticalInfo.damageType === 'kMagic' ? 'ap' : info.tacticalInfo.damageType === 'kPhysical' ? 'ad' : ''">
              {{ DMG[info.tacticalInfo.damageType] ?? '?' }} · {{ ATK[info.tacticalInfo.attackType] ?? info.tacticalInfo.attackType }} · {{ (info.roles ?? []).join(', ') }}
            </span>
            <div v-for="k in PIPS" :key="k" class="ps">
              <span class="lbl">{{ k === 'crowdControl' ? 'CC' : k }}</span>
              <span class="pips"><i v-for="n in 3" :key="n" :class="{ on: n <= info.playstyleInfo[k] }" /></span>
            </div>
          </div>
        </div>
        <div v-if="build.notes?.length" class="notes">
          <span v-for="n in build.notes" :key="n" class="note">{{ n }}</span>
        </div>
      </div>
      <button type="button" role="switch" class="toggle follow" :class="{ on: follow }" :aria-checked="follow" title="Switch the build to whatever you lock in or play" @click="follow = !follow">
        <span class="track" aria-hidden="true" /><span class="muted">Follow</span>
      </button>
    </template>
    <template v-else-if="s.loading"><div class="muted">Loading build…</div></template>
    <template v-else-if="champion">
      <img class="portrait" :src="champion.image" :alt="champion.name" />
      <div class="who">
        <div class="name">{{ champion.name }}</div>
        <div class="title">{{ s.error || 'No build data' }}</div>
      </div>
      <button @click="loadBuild(champion)">Retry</button>
    </template>
    <div v-else class="muted">Pick a champion. When you lock in during champ select the build shows here automatically.</div>
  </section>
</template>

<style scoped>
.champbar { display: flex; align-items: center; gap: 14px; padding: 6px 130px 6px 14px; min-height: 76px; box-sizing: border-box; }
.champbar.empty { min-height: 48px; padding-right: 14px; }
/* Pinned top-right so it never decides where the rest of the bar wraps */
.follow { position: absolute; right: 10px; top: 50%; transform: translateY(-50%); padding: 0 8px; min-height: 0; background: none; box-shadow: none; border-color: transparent; }
.follow.on { color: var(--gold-bright); }
.portrait { width: 56px; height: 56px; flex: none; border: 2px solid var(--gold); border-radius: var(--radius-sm); box-shadow: var(--shadow-inset), 0 0 16px var(--gold-glow); }
/* Row 1: who + mode / lane buttons. Row 2: stats badges + playstyle. Each wraps on its own. */
.rows { display: flex; flex-direction: column; gap: 6px; min-width: 0; flex: 1; }
.row1, .row2 { display: flex; align-items: center; gap: 8px 18px; flex-wrap: wrap; }
.who { min-width: 0; }
.name { font-family: var(--display); font-size: 22px; font-weight: 700; letter-spacing: .04em; line-height: 1.1; color: var(--gold-bright); white-space: nowrap; }
.title { color: var(--muted); font-style: italic; font-size: 12px; white-space: nowrap; }
.roles { justify-content: flex-start; align-items: center; }
.roles .sep { width: 1px; height: 22px; background: var(--gold-deep); margin: 0 4px; }
.facts { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.playstyle { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.ps { display: inline-flex; align-items: center; gap: 6px; font-size: 11px; color: var(--muted); text-transform: capitalize; }
.pips { display: inline-flex; gap: 2px; }
.pips i { width: 10px; height: 10px; border: 1px solid var(--gold-deep); background: var(--surface-input); }
.pips i.on { background: var(--gold); box-shadow: 0 0 6px var(--gold-glow); }
.notes { display: flex; gap: 8px; flex-wrap: wrap; }
.notes .note { margin: 0; padding: 4px 10px; font-size: 12px; }
/* In game: one line when it fits, the rows flow next to each other */
.compact { min-height: 56px; }
.compact .rows { flex-direction: row; flex-wrap: wrap; gap: 4px 18px; align-items: center; }
.compact .portrait { width: 40px; height: 40px; }
.compact .name { font-size: 17px; }
button.sm { padding: 4px 8px; font-size: 10px; }
.badge.opgg { color: var(--hextech-2); border-color: var(--hextech-dim); }
.badge.tier { color: var(--gold-bright); }
.badge.tier.t1 { color: #ff8a3d; border-color: #ff8a3d; }
.badge.tier.t2 { color: var(--gold); border-color: var(--gold); }
.badge.ap { color: #7fb3ff; border-color: #2a4a7a; }
.badge.ad { color: #e8a33d; border-color: #7a5a2a; }
.load-backdrop { position: fixed; inset: 0; z-index: 150; background: rgba(0, 0, 0, 0.55); backdrop-filter: blur(3px); display: grid; place-items: center; animation: fadein .15s ease; }
@keyframes fadein { from { opacity: 0; } to { opacity: 1; } }
.load-card { display: grid; justify-items: center; gap: 8px; padding: 22px 34px; background: var(--panel-2); border: 1px solid var(--gold-dark); border-radius: var(--radius); box-shadow: var(--shadow-panel); }
.load-card img { width: 96px; height: 96px; border: 2px solid var(--gold); border-radius: var(--radius-sm); box-shadow: 0 0 20px var(--gold-glow); }
.load-name { font-family: var(--display); font-size: 20px; font-weight: 700; color: var(--gold-bright); letter-spacing: .04em; }
.load-sub { color: var(--muted); font-size: 13px; }
.spinner { width: 28px; height: 28px; border: 3px solid var(--gold-deep); border-top-color: var(--accent); border-radius: 50%; animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
