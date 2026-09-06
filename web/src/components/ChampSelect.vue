<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { api, fmtPoints } from '../api'
import * as sound from '../sound'
import { toast } from '../toast'
import type { ChampSelect, Champion, CompSummary, DamageProfile, Mastery, PlayRecord } from '../types'

const comp = (c: CompSummary) => `${c.tanks} tank · ${c.ranged} ranged · ${c.melee} melee · CC ${c.cc} · tough ${c.durability}`

const emit = defineEmits<{ preview: [c: Champion] }>()
// Auto-apply Riot's recommended rune page for whatever champion you end up with.
const autoRunes = ref(localStorage.getItem('ezlol.autoRunes') === 'on')
watch(autoRunes, (v) => localStorage.setItem('ezlol.autoRunes', v ? 'on' : 'off'))
let appliedFor = 0
const runeMsg = ref('')
async function maybeApplyRunes(c: ChampSelect) {
  const id = c.me?.champion.id
  if (!autoRunes.value || !id || id === appliedFor) return
  appliedFor = id
  try {
    const b = await api.build(String(id), undefined, c.mode === 'aram' ? 'aram' : 'sr')
    const page = b.runes?.[0]
    if (!page) return
    await api.applyRunes(id, page)
    toast({ key: 'runes', kind: 'success', title: 'Runes set', body: `${b.champion.name}: ${page.primary.name} + ${page.secondary.name}` })
    sound.accepted()
  } catch (e) {
    toast({ key: 'runes', kind: 'error', title: 'Auto runes failed', body: (e as Error).message })
  }
}
const cs = ref<ChampSelect | null>(null)
const busy = ref(false)
const err = ref('')
let timer: number | undefined

let tradePinged = false
let timerPinged = false
async function poll() {
  try {
    cs.value = await api.champSelect()
    const c = cs.value
    if (c.active) maybeApplyRunes(c)
    else appliedFor = 0
    const received = c.myTeam?.some((p) => p.trade === 'RECEIVED') ?? false
    if (received && !tradePinged) {
      tradePinged = true
      sound.alert()
      sound.notify('ezlol', 'Trade request received')
      toast({ key: 'trade', kind: 'warn', title: 'Trade request', body: 'A teammate wants to swap — Accept is in the champ select box.', ttl: 15000 })
    }
    if (!received) tradePinged = false
    if (c.active && c.timeLeft > 0 && c.timeLeft <= 10 && !timerPinged) {
      timerPinged = true
      sound.alert()
    }
    if (c.timeLeft > 10) timerPinged = false
  } catch {
    /* transient */
  }
}
onMounted(() => {
  poll()
  timer = window.setInterval(poll, 1000)
})
onUnmounted(() => clearInterval(timer))

async function act(fn: () => Promise<unknown>) {
  busy.value = true
  err.value = ''
  try {
    await fn()
    await poll()
  } catch (e) {
    err.value = (e as Error).message
    toast({ key: 'cs', kind: 'error', title: 'Champ select action failed', body: err.value })
  } finally {
    busy.value = false
  }
}
// Suggest a reroll when nothing on the bench beats what you have and rerolls remain.
const rerollHint = (c: ChampSelect) => {
  if (!c.benchEnabled || c.rerollsRemaining <= 0) return ''
  const mine = c.me?.mastery?.championLevel ?? 0
  const best = c.bench?.[0]
  if (best && best.score >= 25) return `Swap to ${best.champion.name} (${best.why})`
  if (mine < 5) return 'Nothing strong on the bench and low mastery on your pick: reroll'
  return ''
}
const pct = (p: DamageProfile) => (p.attack + p.magic ? Math.round((100 * p.attack) / (p.attack + p.magic)) : 50)
const rec = (r?: PlayRecord) => (r ? `${r.wins}W ${r.games - r.wins}L` : '')
const mast = (m?: Mastery) => (m ? `M${m.championLevel} · ${fmtPoints(m.championPoints)}` : 'new')
</script>

<template>
  <template v-if="cs?.active">
    <section class="panel">
      <h2>Champ select <span class="muted" style="margin-left: 8px">{{ cs.mode === 'aram' ? 'ARAM' : cs.phase }} · {{ cs.timeLeft }}s</span>
        <label class="toggle" :class="{ on: autoRunes }" style="margin-left: auto" title="Set Riot's recommended rune page automatically for the champion you get" @click="autoRunes = !autoRunes"><span class="track" /><span>Auto runes</span></label>
      </h2>
      <h3>Your team</h3>
      <div class="team">
        <div v-for="p in cs.myTeam ?? []" :key="p.name + p.champion.id" class="cs-player" :class="{ me: p.isMe }" @click="p.champion.id && emit('preview', p.champion)">
          <img v-if="p.champion.image" :src="p.champion.image" :alt="p.champion.name" />
          <div v-else class="empty" />
          <div class="cname">{{ p.champion.name || '…' }}</div>
          <div class="muted">{{ p.isMe ? mast(p.mastery) : p.name }}</div>
          <div v-if="p.isMe && p.record" class="rec">{{ rec(p.record) }}</div>
          <button v-if="!p.isMe && p.tradeId && p.trade === 'AVAILABLE'" :disabled="busy" @click.stop="act(() => api.trade(p.tradeId!))">Trade</button>
          <button v-else-if="!p.isMe && p.tradeId && p.trade === 'RECEIVED'" class="primary" :disabled="busy" @click.stop="act(() => api.trade(p.tradeId!, true))">Accept</button>
          <div v-else-if="!p.isMe && p.trade === 'SENT'" class="rec">trade sent</div>
        </div>
      </div>
      <div v-if="cs.allyProfile.hint" class="profile">
        <div class="bar"><div :style="{ width: pct(cs.allyProfile) + '%' }" /></div>
        <span class="muted">AD {{ pct(cs.allyProfile) }}% · AP {{ 100 - pct(cs.allyProfile) }}% · {{ comp(cs.allyComp) }}</span>
        <div v-if="cs.allyComp.needs?.length" class="hint">Comp is missing: {{ cs.allyComp.needs.join(', ') }}</div>
      </div>
    </section>

    <section v-if="cs.theirTeam?.length" class="panel">
      <h2>Enemies</h2>
      <div class="team">
        <div v-for="c in cs.theirTeam" :key="c.id" class="cs-player" @click="emit('preview', c)">
          <img :src="c.image" :alt="c.name" />
          <div class="cname">{{ c.name }}</div>
        </div>
      </div>
      <div class="profile">
        <div class="bar"><div :style="{ width: pct(cs.enemyProfile) + '%' }" /></div>
        <span class="muted">AD {{ pct(cs.enemyProfile) }}% · AP {{ 100 - pct(cs.enemyProfile) }}% · {{ comp(cs.enemyComp) }}</span>
        <div class="hint">{{ cs.enemyProfile.hint }}</div>
      </div>
    </section>

    <section v-if="cs.benchEnabled" class="panel span2">
      <h2>Bench
        <button class="primary" style="margin-left: auto" :disabled="busy || cs.rerollsRemaining <= 0" @click="act(api.reroll)">Reroll ({{ cs.rerollsRemaining }})</button>
      </h2>
      <div class="team" v-if="cs.bench?.length">
        <div v-for="(b, i) in cs.bench" :key="b.champion.id" class="cs-player bench" :class="{ hot: i === 0 && b.score >= 20 }" :title="b.why">
          <img :src="b.champion.image" :alt="b.champion.name" @click="emit('preview', b.champion)" />
          <div class="cname">{{ b.champion.name }}</div>
          <div class="muted">{{ b.why || mast(b.mastery) }}</div>
          <div v-if="b.record" class="rec">{{ rec(b.record) }}</div>
          <button :disabled="busy" @click="act(() => api.swapBench(b.champion.id))">Swap</button>
        </div>
      </div>
      <div v-else class="muted">Bench is empty. Reroll or wait for teammates to trade.</div>
      <div v-if="rerollHint(cs)" class="hint" style="margin-top: 6px">{{ rerollHint(cs) }}</div>
    </section>
  </template>
</template>


<style scoped>
.team { display: flex; gap: 8px; flex-wrap: wrap; }
.cs-player { width: 96px; text-align: center; cursor: pointer; padding: 6px; border: 1px solid var(--line); background: var(--surface-raised); }
.cs-player.me { border-color: var(--hextech-dim); background: var(--accent-soft); }
.cs-player.hot { border-color: var(--gold); box-shadow: 0 0 10px var(--gold-glow); }
.cs-player img, .cs-player .empty { width: 56px; height: 56px; border: 1px solid var(--gold-dark); display: block; margin: 0 auto 4px; }
.cs-player .empty { background: var(--surface-input); }
.cs-player:hover img { border-color: var(--gold); }
.cname { font-size: 12px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cs-player .muted { font-size: 10px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rec { font-size: 10px; color: var(--hextech-2); font-family: var(--display); }
.cs-player button { margin-top: 4px; padding: 3px 8px; font-size: 10px; width: 100%; }
.profile { margin-top: 8px; }
.bar { height: 6px; background: #3b2fbf; border: 1px solid var(--gold-deep); overflow: hidden; }
.bar div { height: 100%; background: #c8452a; }
.hint { color: var(--gold); font-size: 12px; margin-top: 2px; }
</style>
