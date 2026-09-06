<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { api, fmtTime } from '../api'
import type { Eog, EogPlayer } from '../types'

const eog = ref<Eog | null>(null)
let timer: number | undefined
async function poll() {
  try {
    eog.value = await api.eog()
  } catch {
    /* transient */
  }
}
onMounted(() => {
  poll()
  timer = window.setInterval(poll, 5000)
})
onUnmounted(() => clearInterval(timer))

const n = (p: EogPlayer, k: string) => Number(p.stats?.[k] ?? 0)
const fmt = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(1)}k` : String(Math.round(v)))
const myTeam = () => eog.value?.teams?.find((t) => t.players?.some((p) => p.isLocalPlayer))
const me = () => myTeam()?.players?.find((p) => p.isLocalPlayer)
const share = (k: string) => {
  const t = myTeam()
  const m = me()
  if (!t || !m) return 0
  const total = (t.players ?? []).reduce((a, p) => a + n(p, k), 0)
  return total ? Math.round((100 * n(m, k)) / total) : 0
}
</script>

<template>
  <section class="panel span2" v-if="eog?.available">
    <h2>
      Post-game
      <span class="muted" style="margin-left: 10px">{{ fmtTime(eog.gameLength) }} · {{ eog.gameMode }}</span>
      <span class="result" :class="myTeam()?.isWinningTeam ? 'win' : 'loss'">{{ myTeam()?.isWinningTeam ? 'VICTORY' : 'DEFEAT' }}</span>
    </h2>
    <div v-if="me()" class="row" style="margin-bottom: 10px">
      <span class="badge riot">Your damage share {{ share('TOTAL_DAMAGE_DEALT_TO_CHAMPIONS') }}%</span>
      <span class="badge lcu">Kill participation {{ share('CHAMPIONS_KILLED') + share('ASSISTS') > 0 ? Math.round((100 * (n(me()!, 'CHAMPIONS_KILLED') + n(me()!, 'ASSISTS'))) / Math.max(1, (myTeam()?.players ?? []).reduce((a, p) => a + n(p, 'CHAMPIONS_KILLED'), 0))) : 0 }}%</span>
      <span class="badge">Damage taken share {{ share('TOTAL_DAMAGE_TAKEN') }}%</span>
      <span class="badge">Healing share {{ share('TOTAL_HEAL') }}%</span>
    </div>
    <div class="teams">
      <div v-for="t in eog.teams ?? []" :key="t.teamId">
        <h3>{{ t.isWinningTeam ? 'Winners' : 'Losers' }}</h3>
        <table class="score">
          <tr v-for="p in t.players ?? []" :key="p.championId" :class="{ me: p.isLocalPlayer }">
            <td class="champ"><img :src="p.champion.image" :alt="p.champion.name" /></td>
            <td class="who">{{ p.champion.name }}<div class="muted">{{ p.gameName || p.summonerName }}</div></td>
            <td class="kda">{{ n(p, 'CHAMPIONS_KILLED') }} / {{ n(p, 'NUM_DEATHS') }} / {{ n(p, 'ASSISTS') }}</td>
            <td class="num" title="Damage to champions">{{ fmt(n(p, 'TOTAL_DAMAGE_DEALT_TO_CHAMPIONS')) }} dmg</td>
            <td class="num" title="Gold earned">{{ fmt(n(p, 'GOLD_EARNED')) }} g</td>
            <td class="items"><div class="item-row"><img v-for="it in p.itemRefs ?? []" :key="it.id" :src="it.image" :title="it.name" /></div></td>
          </tr>
        </table>
      </div>
    </div>
  </section>
</template>

<style scoped>
.result { margin-left: auto; font-family: var(--display); font-size: 13px; letter-spacing: .2em; }
.result.win { color: var(--hextech-2); }
.result.loss { color: var(--red); }
.teams { display: grid; gap: 12px; }
.score { width: 100%; border-collapse: collapse; }
.score td { padding: 4px 6px; border-bottom: 1px solid rgba(200,170,110,.08); vertical-align: middle; }
.score tr.me td { background: rgba(3,151,171,.08); }
.score td.champ img { width: 32px; height: 32px; border: 1px solid var(--gold-dark); display: block; }
.who { font-weight: 600; font-size: 13px; }
.who .muted { font-weight: 400; font-size: 10px; }
.kda, .num { font-variant-numeric: tabular-nums; white-space: nowrap; font-size: 12px; }
.num { color: var(--muted); }
.item-row { display: flex; flex-wrap: nowrap; gap: 2px; }
.item-row img { width: 22px; height: 22px; border: 1px solid var(--gold-deep); display: block; }
</style>
