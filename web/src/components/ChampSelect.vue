<script setup lang="ts">
// Champ select bar: one fixed strip under the champion bar, only while the
// client is in champion select. Your team (trades), enemies, bench (swap /
// reroll) and the comp read-outs sit side by side; never part of the grid.
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { api, fmtPoints } from '../api'
import * as sound from '../sound'
import { toast } from '../toast'
import type { ArenaTier, ChampSelect, Champion, CompSummary, DamageProfile, Mastery, PlayRecord } from '../types'

const comp = (c: CompSummary) => `${c.tanks} tank · ${c.ranged} ranged · ${c.melee} melee · CC ${c.cc} · tough ${c.durability}`

const emit = defineEmits<{ preview: [c: Champion] }>()
// Auto-apply Riot's recommended rune page for whatever champion you end up with.
const autoRunes = ref(localStorage.getItem('ezlol.autoRunes') === 'on')
watch(autoRunes, (v) => localStorage.setItem('ezlol.autoRunes', v ? 'on' : 'off'))
let appliedFor = 0
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
const tiers = ref<Record<string, ArenaTier>>({})
let tiersLoaded = false
const tierLetter = (id: number) => {
  const t = tiers.value[String(id)]
  return t ? `${['', 'S', 'A', 'B', 'C', 'D'][t.tier] ?? t.tier} · #${t.avgPlace.toFixed(2)}` : ''
}
const busy = ref(false)
let timer: number | undefined

let tradePinged = false
let timerPinged = false
async function poll() {
  try {
    cs.value = await api.champSelect()
    const c = cs.value
    if (c.mode === 'arena' && !tiersLoaded) {
      tiersLoaded = true
      api.arenaTiers().then((t) => (tiers.value = t)).catch(() => {})
    }
    if (c.active) maybeApplyRunes(c)
    else appliedFor = 0
    const received = c.myTeam?.some((p) => p.trade === 'RECEIVED') ?? false
    if (received && !tradePinged) {
      tradePinged = true
      sound.alert()
      sound.notify('ezlol', 'Trade request received')
      toast({ key: 'trade', kind: 'warn', title: 'Trade request', body: 'A teammate wants to swap — Accept is in the champ select bar.', ttl: 15000 })
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
  try {
    await fn()
    await poll()
  } catch (e) {
    toast({ key: 'cs', kind: 'error', title: 'Champ select action failed', body: (e as Error).message })
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
  <section v-if="cs?.active" class="panel csbar" aria-label="Champ select">
    <div class="cs-head">
      <div class="cs-title">Champ select</div>
      <div class="cs-sub">{{ cs.mode === 'aram' ? 'ARAM' : cs.mode === 'arena' ? 'Arena' : cs.phase }}</div>
      <div class="cs-timer" :class="{ urgent: cs.timeLeft > 0 && cs.timeLeft <= 10 }">{{ cs.timeLeft }}s</div>
      <button type="button" role="switch" class="toggle" :class="{ on: autoRunes }" :aria-checked="autoRunes" title="Set Riot's recommended rune page automatically for the champion you get" @click="autoRunes = !autoRunes">
        <span class="track" aria-hidden="true" /><span>Auto runes</span>
      </button>
    </div>

    <div class="group">
      <div class="g-label">Your team</div>
      <div class="team">
        <div v-for="p in cs.myTeam ?? []" :key="p.name + p.champion.id" class="cs-player" :class="{ me: p.isMe }" :title="p.name" @click="p.champion.id && emit('preview', p.champion)">
          <img v-if="p.champion.image" :src="p.champion.image" :alt="p.champion.name" />
          <div v-else class="empty" />
          <div class="cname">{{ p.champion.name || '…' }}</div>
          <div v-if="cs.mode === 'arena' && tierLetter(p.champion.id)" class="rec">{{ tierLetter(p.champion.id) }}</div>
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
        <span v-if="cs.allyComp.needs?.length" class="hint">missing: {{ cs.allyComp.needs.join(', ') }}</span>
      </div>
    </div>

    <div v-if="cs.theirTeam?.length" class="group">
      <div class="g-label">Enemies</div>
      <div class="team">
        <div v-for="c in cs.theirTeam" :key="c.id" class="cs-player" @click="emit('preview', c)">
          <img :src="c.image" :alt="c.name" />
          <div class="cname">{{ c.name }}</div>
        </div>
      </div>
      <div class="profile">
        <div class="bar"><div :style="{ width: pct(cs.enemyProfile) + '%' }" /></div>
        <span class="muted">AD {{ pct(cs.enemyProfile) }}% · AP {{ 100 - pct(cs.enemyProfile) }}% · {{ comp(cs.enemyComp) }}</span>
        <span class="hint">{{ cs.enemyProfile.hint }}</span>
      </div>
    </div>

    <div v-if="cs.benchEnabled" class="group">
      <div class="g-label">
        Bench
        <button class="primary sm" :disabled="busy || cs.rerollsRemaining <= 0" @click="act(api.reroll)">Reroll ({{ cs.rerollsRemaining }})</button>
      </div>
      <div v-if="cs.bench?.length" class="team">
        <div v-for="(b, i) in cs.bench" :key="b.champion.id" class="cs-player bench" :class="{ hot: i === 0 && b.score >= 20 }" :title="b.why">
          <img :src="b.champion.image" :alt="b.champion.name" @click="emit('preview', b.champion)" />
          <div class="cname">{{ b.champion.name }}</div>
          <div class="muted">{{ b.why || mast(b.mastery) }}</div>
          <div v-if="b.record" class="rec">{{ rec(b.record) }}</div>
          <button :disabled="busy" @click="act(() => api.swapBench(b.champion.id))">Swap</button>
        </div>
      </div>
      <div v-else class="muted">Bench is empty. Reroll or wait for teammates to trade.</div>
      <div v-if="rerollHint(cs)" class="hint">{{ rerollHint(cs) }}</div>
    </div>
  </section>
</template>

<style scoped>
.csbar { display: flex; align-items: flex-start; gap: 18px; flex-wrap: wrap; padding: 8px 14px; }
.cs-head { display: flex; flex-direction: column; gap: 2px; min-width: 120px; align-self: stretch; justify-content: center; }
.cs-title { font-family: var(--display); font-size: 13px; letter-spacing: var(--label-spacing); text-transform: var(--label-transform); color: var(--h2-color); }
.cs-sub { font-size: 11px; color: var(--muted); }
.cs-timer { font-family: var(--display); font-size: 22px; font-weight: 700; color: var(--gold-bright); line-height: 1.1; }
.cs-timer.urgent { color: var(--red); }
.cs-head .toggle { padding: 0 6px 0 0; min-height: 0; background: none; box-shadow: none; border-color: transparent; margin-top: 4px; justify-content: flex-start; }
.group { display: flex; flex-direction: column; gap: 6px; padding-left: 16px; border-left: 1px solid var(--gold-deep); min-width: 0; }
.g-label { font-family: var(--display); font-size: 11px; letter-spacing: var(--label-spacing); text-transform: var(--label-transform); color: var(--h3-color); display: flex; align-items: center; gap: 10px; min-height: 22px; }
.g-label button.sm { padding: 2px 8px; font-size: 10px; min-height: 0; }
.team { display: flex; gap: 6px; flex-wrap: wrap; }
.cs-player { width: 82px; text-align: center; cursor: pointer; padding: 4px; border: 1px solid var(--line); background: var(--surface-raised); }
.cs-player.me { border-color: var(--hextech-dim); background: var(--accent-soft); }
.cs-player.hot { border-color: var(--gold); box-shadow: 0 0 10px var(--gold-glow); }
.cs-player img, .cs-player .empty { width: 44px; height: 44px; border: 1px solid var(--gold-dark); display: block; margin: 0 auto 3px; }
.cs-player .empty { background: var(--surface-input); }
.cs-player:hover img { border-color: var(--gold); }
.cname { font-size: 11px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cs-player .muted { font-size: 10px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rec { font-size: 10px; color: var(--hextech-2); font-family: var(--display); }
.cs-player button { margin-top: 3px; padding: 2px 6px; font-size: 10px; width: 100%; min-height: 0; }
/* width: 0 + min-width: 100%: the read-out wraps under the portraits instead of
   widening the group (a long hint would otherwise push the next group onto a new row). */
.profile { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 11px; width: 0; min-width: 100%; }
.profile .muted { font-size: 11px; }
.bar { width: 110px; height: 6px; background: #3b2fbf; border: 1px solid var(--gold-deep); overflow: hidden; flex: none; }
.bar div { height: 100%; background: #c8452a; }
.hint { color: var(--gold); font-size: 11px; }
</style>
