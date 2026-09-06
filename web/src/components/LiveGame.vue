<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api, estRank, fmtTime } from '../api'
import * as sound from '../sound'
import type { Champion, ChampionDetail, Live, LivePlayer } from '../types'
const pctf = (x: number) => `${Math.round(x * 100)}%`
const tierLabel = (t: number) => ['', 'S', 'A', 'B', 'C', 'D'][t] ?? '?'

const emit = defineEmits<{ me: [c: Champion] }>()

const live = ref<Live | null>(null)
let timer: number | undefined
// Expanded rows showing ability cooldowns.
const expanded = ref<Record<string, boolean>>({})
const details = ref<Record<string, ChampionDetail>>({})
async function toggleRow(p: LivePlayer) {
  const k = p.champion.key
  expanded.value[p.name] = !expanded.value[p.name]
  if (expanded.value[p.name] && k && !details.value[k]) {
    try {
      details.value[k] = await api.spells(k)
    } catch {
      /* ignore */
    }
  }
}
const cdAt = (cds: number[], rank: number) => (rank <= 0 ? null : cds[Math.min(cds.length, rank) - 1])
const strip = (t: string) => t.replace(/<[^>]+>/g, '').replace(/\{\{[^}]*\}\}/g, '?')
// Enemy spell tracking: key -> game time when the spell is back up. Kept in
// sessionStorage so a reload mid-game does not lose the timers.
const tracked = ref<Record<string, number>>(load())
function load(): Record<string, number> {
  try {
    return JSON.parse(sessionStorage.getItem('ezlol.tracked') ?? '{}')
  } catch {
    return {}
  }
}
function persist() {
  try {
    sessionStorage.setItem('ezlol.tracked', JSON.stringify(tracked.value))
  } catch {
    /* ignore */
  }
}

let announced = 0
let respawnPinged = false
const myself = computed(() => live.value?.players?.find((p) => p.isMe))
// Item purchase feed: diff each player's inventory between polls.
interface Buy { t: number; who: string; champ: string; item: string; image: string; enemy: boolean }
const buys = ref<Buy[]>(loadBuys())
const lastItems: Record<string, Set<number>> = {}
function loadBuys(): Buy[] {
  try {
    return JSON.parse(sessionStorage.getItem('ezlol.buys') ?? '[]')
  } catch {
    return []
  }
}
function persistBuys() {
  try {
    sessionStorage.setItem('ezlol.buys', JSON.stringify(buys.value))
  } catch {
    /* ignore */
  }
}
function diffItems(prev: Live | null, next: Live) {
  for (const p of next.players ?? []) {
    const cur = new Set((p.items ?? []).filter((i) => i.image).map((i) => i.id))
    const before = lastItems[p.name]
    if (before && prev) {
      for (const it of p.items ?? []) {
        if (it.image && !before.has(it.id) && !it.name.toLowerCase().includes('potion') && it.count <= 1) {
          buys.value = [...buys.value, { t: next.gameTime, who: p.name, champ: p.champion.name, item: it.name, image: it.image, enemy: p.team !== next.myTeam }].slice(-30)
          persistBuys()
        }
      }
    }
    lastItems[p.name] = cur
  }
}
let lastGT = 0
let lastEvent = -1
let offerPinged = false
async function poll() {
  try {
    const prev = live.value
    live.value = await api.live()
    if (live.value.inGame) diffItems(prev, live.value)
    const t = live.value.gameTime ?? 0
    if (t < lastGT - 30) {
      tracked.value = {}
      buys.value = []
      persist()
      persistBuys()
    }
    lastGT = t
    const m = myself.value
    if (m?.isDead && m.respawnTimer <= 3.2 && !respawnPinged) {
      respawnPinged = true
      sound.accepted()
    }
    if (!m?.isDead) respawnPinged = false
    // Chime once when an augment offer is detected on screen.
    const off = live.value.offer
    if (off?.active && !offerPinged) {
      offerPinged = true
      sound.alert()
    }
    if (!off?.active) offerPinged = false
    // Celebrate your own multikills.
    const evs = live.value.events ?? []
    const last = evs[evs.length - 1]
    if (last && last.EventID !== lastEvent && last.EventName === 'Multikill' && last.KillerName === m?.name) sound.queuePop()
    if (last) lastEvent = last.EventID
    const me = live.value.players?.find((p) => p.isMe)
    if (me?.champion.id && me.champion.id !== announced) {
      announced = me.champion.id
      emit('me', me.champion)
    }
  } catch {
    /* transient */
  }
}
onMounted(() => {
  poll()
  timer = window.setInterval(poll, 1500)
})
onUnmounted(() => clearInterval(timer))

const myTeam = computed(() => (live.value?.players ?? []).filter((p) => p.team === live.value?.myTeam))
const enemies = computed(() => (live.value?.players ?? []).filter((p) => p.team !== live.value?.myTeam))
const teamKills = (ps: LivePlayer[]) => ps.reduce((a, p) => a + p.kills, 0)
const gt = () => live.value?.gameTime ?? 0

function spellKey(p: LivePlayer, i: number) {
  return `${p.name}:${i}`
}
function toggleSpell(p: LivePlayer, i: number) {
  const k = spellKey(p, i)
  const cd = p.spells?.[i]?.cooldown ?? 0
  if (tracked.value[k] && tracked.value[k] > gt()) delete tracked.value[k]
  else if (cd > 0) tracked.value[k] = gt() + cd
  persist()
}
function remaining(p: LivePlayer, i: number): number {
  const until = tracked.value[spellKey(p, i)]
  return until ? Math.max(0, until - gt()) : 0
}
// Numbers advantage: enemies dead vs allies dead.
const advantage = computed(() => {
  const ed = enemies.value.filter((p) => p.isDead).length
  const ad = myTeam.value.filter((p) => p.isDead).length
  return { ed, ad, diff: ed - ad }
})
const enemyBack = computed(() => enemies.value.filter((p) => p.isDead).sort((a, b) => a.respawnTimer - b.respawnTimer))
// Enemy spells currently tracked as down, soonest back first.
const downSpells = computed(() =>
  enemies.value
    .flatMap((p) => (p.spells ?? []).map((s, i) => ({ p, s, left: remaining(p, i) })))
    .filter((x) => x.left > 0)
    .sort((a, b) => a.left - b.left),
)
const events = computed(() =>
  (live.value?.events ?? [])
    .filter((e) => ['ChampionKill', 'Multikill', 'Ace', 'FirstBlood', 'TurretKilled', 'InhibKilled', 'GameEnd'].includes(e.EventName))
    .slice(-30),
)
function describe(e: Live['events'] extends (infer T)[] | null ? T : never): string {
  switch (e.EventName) {
    case 'ChampionKill':
      return `${e.KillerName} killed ${e.VictimName}${e.Assisters?.length ? ` (+${e.Assisters.length})` : ''}`
    case 'Multikill':
      return `${e.KillerName} ${['', '', 'double', 'triple', 'quadra', 'PENTA'][e.KillStreak ?? 0] ?? e.KillStreak} kill`
    case 'Ace':
      return `ACE by ${e.AcingTeam}`
    case 'FirstBlood':
      return 'First blood'
    case 'TurretKilled':
      return `Turret destroyed by ${e.KillerName}`
    case 'InhibKilled':
      return `Inhibitor destroyed by ${e.KillerName}`
    case 'GameEnd':
      return 'Game over'
    default:
      return e.EventName
  }
}
const stat = (k: string) => Math.round(Number(live.value?.me?.championStats?.[k] ?? 0))
</script>

<template>
  <template v-if="live?.inGame">
    <!-- Augment pick -->
    <section v-if="live.offer?.active" class="panel span-all offer-box">
      <h2>Augment pick · level {{ live.offer.level }} · <b class="take">take {{ live.offer.best }}</b> <span class="muted" style="text-transform: none; letter-spacing: 0">{{ live.offer.why }}</span></h2>
      <div class="offer-cards">
        <div v-for="(a, i) in live.offer.offered ?? []" :key="a.id" class="offer-card" :class="[a.rarity, { best: i === 0 }]" :title="a.desc">
          <img v-if="a.icon" :src="a.icon" :alt="a.name" />
          <div class="oc-body">
            <div class="oc-name"><span class="tier-pip" :class="'t' + a.tier">{{ tierLabel(a.tier) }}</span>{{ a.name }}</div>
            <div class="oc-stats">{{ a.games ? `${pctf(a.winRate)} win · ${a.games} games · ${pctf(a.pickRate)} pick` : 'no data' }}</div>
            <div class="oc-desc">{{ a.desc }}</div>
          </div>
        </div>
      </div>
    </section>

    <!-- Tactical: score, status, hints -->
    <section class="panel">
      <h2>Live <span class="clock">{{ fmtTime(live.gameTime) }}</span><span class="muted" style="margin-left: 8px">{{ live.gameMode }}</span></h2>
      <div class="score"><span class="ally">{{ teamKills(myTeam) }}</span><span class="vs">vs</span><span class="enemy">{{ teamKills(enemies) }}</span></div>
      <div v-if="myself?.isDead" class="respawn-banner">RESPAWN IN {{ Math.ceil(myself.respawnTimer) }}</div>
      <div v-if="live.offer?.pending && !live.offer.active" class="offer-pending">Augment pick due (lvl {{ live.offer.level }}) — shows when dead / at base. {{ live.ocr !== 'available' ? `Scan: ${live.ocr}` : 'Scanning…' }}</div>
      <div v-if="advantage.diff >= 2" class="adv go">NUMBERS +{{ advantage.diff }} — go!</div>
      <div v-else-if="advantage.diff <= -2" class="adv back">OUTNUMBERED {{ advantage.diff }} — play safe</div>
      <div v-if="live.focus" class="hintline"><b style="color: var(--red)">Focus {{ live.focus }}</b> — {{ live.focusWhy }}</div>
      <div v-if="live.hint" class="hintline">Build: {{ live.hint }}</div>
      <div v-if="enemyBack.length" class="down"><span class="muted">Enemy back:</span><span v-for="p in enemyBack" :key="p.name" class="pill"><img :src="p.champion.image" />{{ p.champion.name }} {{ Math.ceil(p.respawnTimer) }}s</span></div>
      <div v-if="downSpells.length" class="down"><span class="muted">Down:</span><span v-for="x in downSpells" :key="x.p.name + x.s.name" class="pill"><img :src="x.s.image" />{{ x.p.champion.name }} {{ x.s.name }} {{ fmtTime(x.left) }}</span></div>
    </section>

    <!-- Enemies -->
    <section class="panel span2 live">
      <h2>Enemies <span class="muted" style="text-transform: none; letter-spacing: 0; margin-left: 8px">click a spell when used · click portrait for cooldowns</span></h2>
        <div v-for="p in enemies" :key="p.name" class="prow" :class="{ dead: p.isDead }">
          <div class="portrait" @click="toggleRow(p)" title="Click: ability cooldowns">
            <img :src="p.champion.image" :alt="p.champion.name" />
            <span v-if="p.isDead" class="respawn">{{ Math.ceil(p.respawnTimer) }}</span>
            <span class="lvl">{{ p.level }}</span>
          </div>
          <div class="info">
            <div class="pname">{{ p.champion.name }} <span class="muted">{{ p.name }}</span></div>
            <div class="kda">{{ p.kills }} / {{ p.deaths }} / {{ p.assists }} <span class="istats"><b v-if="p.itemAD" class="ad">{{ p.itemAD }} AD</b><b v-if="p.itemAP" class="ap">{{ p.itemAP }} AP</b><b v-if="p.itemArmor" class="ar">{{ p.itemArmor }} AR</b><b v-if="p.itemMR" class="mr">{{ p.itemMR }} MR</b></span></div>
          </div>
          <div class="items"><img v-for="it in (p.items ?? []).filter((i) => i.image)" :key="it.id" :src="it.image" :title="it.name" /></div>
          <div class="spells">
            <button v-for="(s, i) in p.spells ?? []" :key="i" class="spell" :class="{ down: remaining(p, i) > 0 }" :title="`${s.name} (${s.cooldown}s) — click when used`" @click="toggleSpell(p, i)">
              <img :src="s.image" :alt="s.name" />
              <span v-if="remaining(p, i) > 0" class="cd">{{ fmtTime(remaining(p, i)) }}</span>
            </button>
          </div>
          <div v-if="expanded[p.name] && details[p.champion.key]" class="abilities">
            <span v-for="sp in details[p.champion.key].spells" :key="sp.key" class="ab" :title="`${sp.name}: ${strip(sp.tooltip).slice(0, 220)} — CDs ${sp.cooldowns.join('/')}`">
              <img :src="sp.image" :alt="sp.name" /><b>{{ sp.key }}</b>
              <span>{{ cdAt(sp.cooldowns, estRank(p.level, sp.key)) === null ? '—' : `≈${cdAt(sp.cooldowns, estRank(p.level, sp.key))}s` }}</span>
            </span>
          </div>
        </div>
    </section>

    <!-- Your team -->
    <section class="panel span2 live">
      <h2>Your team</h2>
        <div v-for="p in myTeam" :key="p.name" class="prow" :class="{ me: p.isMe, dead: p.isDead }">
          <div class="portrait" @click="toggleRow(p)" title="Click: ability cooldowns">
            <img :src="p.champion.image" :alt="p.champion.name" />
            <span v-if="p.isDead" class="respawn">{{ Math.ceil(p.respawnTimer) }}</span>
            <span class="lvl">{{ p.level }}</span>
          </div>
          <div class="info">
            <div class="pname">{{ p.champion.name }} <span class="muted">{{ p.name }}</span></div>
            <div class="kda">{{ p.kills }} / {{ p.deaths }} / {{ p.assists }} <span class="istats"><b v-if="p.itemAD" class="ad">{{ p.itemAD }} AD</b><b v-if="p.itemAP" class="ap">{{ p.itemAP }} AP</b><b v-if="p.itemArmor" class="ar">{{ p.itemArmor }} AR</b><b v-if="p.itemMR" class="mr">{{ p.itemMR }} MR</b></span></div>
          </div>
          <div class="items"><img v-for="it in (p.items ?? []).filter((i) => i.image)" :key="it.id" :src="it.image" :title="it.name" /></div>
          <div class="spells"><img v-for="(s, i) in p.spells ?? []" :key="i" :src="s.image" :title="s.name" /></div>
          <div v-if="expanded[p.name] && details[p.champion.key]" class="abilities">
            <span v-for="sp in details[p.champion.key].spells" :key="sp.key" class="ab" :title="`${sp.name}: ${strip(sp.tooltip).slice(0, 220)} — CDs ${sp.cooldowns.join('/')}`">
              <img :src="sp.image" :alt="sp.name" /><b>{{ sp.key }}</b>
              <span>{{ cdAt(sp.cooldowns, estRank(p.level, sp.key)) === null ? '—' : `≈${cdAt(sp.cooldowns, estRank(p.level, sp.key))}s` }}</span>
            </span>
          </div>
        </div>
    </section>

    <!-- You -->
    <section v-if="live.me" class="panel">
      <h2>You</h2>
      <div class="kv">
        <span>Gold</span><b>{{ Math.round(live.me.currentGold) }}</b>
        <span>Level</span><b>{{ live.me.level }}</b>
        <span>Abilities</span><b class="abil"><span v-for="k in ['Q', 'W', 'E', 'R']" :key="k">{{ k }}{{ live.me.abilities?.[k]?.abilityLevel ?? 0 }}</span></b>
        <span>AD / AP</span><b>{{ stat('attackDamage') }} / {{ stat('abilityPower') }}</b>
        <span>Armor / MR</span><b>{{ stat('armor') }} / {{ stat('magicResist') }}</b>
        <span>Haste</span><b>{{ stat('abilityHaste') }}</b>
        <span>Move speed</span><b>{{ stat('moveSpeed') }}</b>
        <span>Max HP</span><b>{{ stat('maxHealth') }}</b>
        <span>Crit / AS</span><b>{{ Math.round(Number(live.me.championStats?.critChance ?? 0) * 100) }}% / {{ Number(live.me.championStats?.attackSpeed ?? 0).toFixed(2) }}</b>
        <span>Lifesteal / Omni</span><b>{{ Math.round(Number(live.me.championStats?.lifeSteal ?? 0) * 100) }}% / {{ Math.round(Number(live.me.championStats?.omnivamp ?? 0) * 100) }}%</b>
      </div>
    </section>

    <!-- Shopping -->
    <section class="panel">
      <h2>Shopping</h2>
      <div class="log" v-autoscroll>
        <div v-for="(b, i) in buys" :key="i" class="row" :class="{ error: b.enemy }">
          <span class="t">{{ fmtTime(b.t) }}</span>
          <span><img :src="b.image" class="buy-img" /> {{ b.champ }} bought {{ b.item }}</span>
        </div>
        <div v-if="!buys.length" class="muted">No purchases seen yet.</div>
      </div>
    </section>

    <!-- Kill feed -->
    <section class="panel">
      <h2>Kill feed</h2>
      <div class="log" v-autoscroll>
        <div v-for="e in events" :key="e.EventID" class="row" :class="{ accept: e.EventName === 'Ace' || e.EventName === 'Multikill' || e.KillerName === myself?.name, error: e.VictimName === myself?.name }">
          <span class="t">{{ fmtTime(e.EventTime) }}</span><span>{{ describe(e) }}</span>
        </div>
        <div v-if="!events.length" class="muted">Quiet so far.</div>
      </div>
    </section>
  </template>
</template>


<style scoped>
.offer-box { border-color: var(--hextech-2); box-shadow: inset 0 0 0 1px #000, 0 0 22px rgba(10,200,185,.35); }
.offer-box h2 .take { color: var(--hextech-2); }
.hintline { font-size: 12px; color: var(--muted); margin: 4px 0; }
.offer-cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
@media (max-width: 900px) { .offer-cards { grid-template-columns: 1fr; } }
.offer-card { display: grid; grid-template-columns: 44px 1fr; gap: 8px; padding: 8px; border: 1px solid var(--gold-deep); background: rgba(255,255,255,.02); }
.offer-card.best { border-color: var(--hextech-2); box-shadow: inset 0 0 0 1px rgba(10,200,185,.4); }
.offer-card.prismatic { border-left: 3px solid #3fb4d8; }
.offer-card.gold { border-left: 3px solid var(--gold); }
.offer-card.silver { border-left: 3px solid #8b9bb0; }
.offer-card img { width: 44px; height: 44px; border: 1px solid var(--gold-deep); background: #000; }
.oc-name { font-weight: 700; display: flex; align-items: center; gap: 6px; }
.oc-stats { font-size: 11px; color: var(--hextech-2); font-family: var(--display); }
.oc-desc { font-size: 11px; color: var(--muted); }
.tier-pip { font-family: var(--display); font-size: 10px; width: 16px; height: 16px; display: grid; place-items: center; border: 1px solid var(--gold-deep); color: var(--muted); }
.tier-pip.t1 { color: #ff8a3d; border-color: #ff8a3d; } .tier-pip.t2 { color: var(--gold); border-color: var(--gold); } .tier-pip.t3 { color: var(--hextech-2); border-color: var(--hextech-dim); }
.offer-pending { font-size: 12px; color: var(--muted); border: 1px dashed var(--gold-deep); padding: 5px 8px; margin-bottom: 8px; }
.respawn-banner { text-align: center; font-family: var(--display); font-size: 26px; font-weight: 700; letter-spacing: .1em; color: var(--red); text-shadow: 0 0 18px rgba(232,64,87,.6); border: 1px solid #6b1e2b; background: rgba(232,64,87,.08); padding: 6px; margin-bottom: 6px; animation: pulse 1s infinite; }
.adv { text-align: center; font-family: var(--display); font-size: 16px; letter-spacing: .12em; padding: 6px; margin: 8px 0; border: 1px solid; }
.adv.go { color: var(--hextech-2); border-color: var(--hextech-dim); background: rgba(10,200,185,.08); text-shadow: 0 0 12px rgba(10,200,185,.5); }
.adv.back { color: var(--amber); border-color: #7a5a2a; background: rgba(240,178,50,.08); }
.down { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; margin: 8px 0; }
.down .pill img { width: 16px; height: 16px; }
.clock { color: var(--gold-bright); margin-left: 10px; font-variant-numeric: tabular-nums; }
.score { display: flex; justify-content: center; gap: 16px; align-items: baseline; font-family: var(--display); font-size: 28px; font-weight: 700; margin: 4px 0 8px; }
.score .ally { color: var(--hextech-2); }
.score .enemy { color: var(--red); }
.score .vs { font-size: 12px; color: var(--muted); letter-spacing: .2em; }
.prow { display: grid; grid-template-columns: 44px 1fr auto auto; gap: 10px; align-items: center; padding: 6px; border: 1px solid rgba(200,170,110,.08); background: rgba(255,255,255,.02); margin-bottom: 6px; }
.prow.me { border-color: var(--hextech-dim); background: rgba(3,151,171,.08); }
.prow.dead .portrait img { filter: grayscale(1) brightness(.5); }
.portrait { position: relative; width: 44px; height: 44px; cursor: pointer; }
.abilities { grid-column: 1 / -1; display: flex; gap: 10px; flex-wrap: wrap; padding: 4px 0 2px; border-top: 1px dashed var(--gold-deep); }
.ab { display: inline-flex; align-items: center; gap: 4px; font-size: 11px; color: var(--muted); }
.ab img { width: 22px; height: 22px; border: 1px solid var(--gold-deep); }
.ab b { color: var(--gold); font-family: var(--display); }
.portrait img { width: 44px; height: 44px; border: 1px solid var(--gold-dark); }
.portrait .lvl { position: absolute; right: -4px; bottom: -4px; background: #010a13; border: 1px solid var(--gold-dark); font-size: 10px; padding: 0 4px; color: var(--gold); }
.portrait .respawn { position: absolute; inset: 0; display: grid; place-items: center; font-family: var(--display); font-size: 18px; font-weight: 700; color: var(--red); text-shadow: 0 0 6px #000; }
.info { min-width: 0; }
.pname { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pname .muted { font-weight: 400; font-size: 11px; }
.kda { font-variant-numeric: tabular-nums; color: var(--muted); font-size: 12px; }
.items { display: flex; gap: 2px; }
.items img { width: 24px; height: 24px; border: 1px solid var(--gold-deep); }
.spells { display: flex; gap: 3px; }
.spells img { width: 26px; height: 26px; border: 1px solid var(--gold-deep); display: block; }
.spell { position: relative; padding: 0; border: 0; background: none; box-shadow: none; }
.spell.down img { filter: grayscale(1) brightness(.4); }
.spell .cd { position: absolute; inset: 0; display: grid; place-items: center; font-family: var(--display); font-size: 10px; font-weight: 700; color: var(--amber); text-shadow: 0 0 4px #000; }
.buy-img { width: 16px; height: 16px; vertical-align: -3px; border: 1px solid var(--gold-deep); }
.abil { display: flex; gap: 8px; font-family: var(--display); }
.istats { margin-left: 6px; display: inline-flex; gap: 5px; font-size: 10px; }
.istats b { font-weight: 600; }
.istats .ad { color: #e8a33d; }
.istats .ap { color: #7fb3ff; }
.istats .ar { color: #d9c27a; }
.istats .mr { color: #b58cff; }
</style>
