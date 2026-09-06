<script setup lang="ts">
import { ref, watch } from 'vue'
import { api, fmtPoints, ROLES, ROLE_LABEL, winrate } from '../api'
import type { Build, Champion, ChampionInfo, ItemSet, Mastery, PlayRecord, RunePage, Status } from '../types'

const props = defineProps<{ champion: Champion | null; status: Status | null; compact?: boolean }>()
const expandAugs = ref(false)
const follow = defineModel<boolean>('follow', { default: true })

const build = ref<Build | null>(null)
const role = ref('')
// '' = follow the client's current queue; 'sr' | 'aram' = forced
const mode = ref<'' | 'sr' | 'aram'>('')
const loading = ref(false)
const err = ref('')
const me = ref<{ mastery: Mastery | null; record: PlayRecord | null } | null>(null)
const info = ref<ChampionInfo | null>(null)
const DMG: Record<string, string> = { kMagic: 'AP', kPhysical: 'AD', kMixed: 'Mixed' }

async function load() {
  if (!props.champion) return
  loading.value = true
  err.value = ''
  try {
    build.value = await api.build(props.champion.key, role.value || undefined, mode.value || undefined)
    role.value = build.value.role
    api.me(props.champion.id, build.value.mode).then((m) => (me.value = m)).catch(() => (me.value = null))
    api.info(props.champion.key).then((r) => (info.value = r.info ?? null)).catch(() => (info.value = null))
  } catch (e) {
    err.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

watch(
  () => props.champion?.id,
  () => {
    role.value = ''
    load()
  },
  { immediate: true },
)
watch(
  () => props.status?.pickedPosition,
  (p) => {
    if (follow.value && p && p !== role.value && props.status?.pickedChampion === props.champion?.id) {
      role.value = p
      load()
    }
  },
)
watch(
  () => props.status?.connected,
  (c, prev) => {
    if (c && !prev) load()
  },
)

function pickRole(r: string) {
  role.value = r
  load()
}
function pickMode(m: '' | 'sr' | 'aram') {
  mode.value = m
  load()
}
watch(
  () => props.status?.queueId,
  () => {
    if (!mode.value) load()
  },
)

const applying = ref(false)
const applied = ref('')
async function applyRunes(p: RunePage) {
  if (!build.value) return
  applying.value = true
  applied.value = ''
  try {
    await api.applyRunes(build.value.champion.id, p)
    applied.value = 'Rune page set in client'
  } catch (e) {
    applied.value = (e as Error).message
  } finally {
    applying.value = false
    setTimeout(() => (applied.value = ''), 4000)
  }
}
const RARITIES = ['prismatic', 'gold', 'silver'] as const
const augBy = (b: Build, r: string) => (b.augments ?? []).filter((a) => a.rarity === r).slice(0, props.compact && !expandAugs.value ? 6 : 12)
const pctf = (x: number) => `${Math.round(x * 100)}%`
const tierLabel = (t?: number) => (t ? ['', 'S', 'A', 'B', 'C', 'D'][t] ?? String(t) : '')
const pct = (s: ItemSet, total: number) => (total ? `${Math.round((100 * s.games) / total)}% pick` : '')
</script>

<template>
  <template v-if="!champion">
    <section class="panel"><div class="muted">Pick a champion. When you lock in during champ select the build shows here automatically.</div></section>
  </template>
  <template v-else-if="build">
    <!-- Header box -->
    <section class="panel box-head" :class="{ compact }">
      <div class="build-head">
        <img :src="build.champion.image" :alt="build.champion.name" />
        <div>
          <div class="name">{{ build.champion.name }}</div>
          <div class="title">{{ build.champion.title }} · patch {{ build.patch.replace('-aram', '') }}<span v-if="build.mode === 'aram'"> · Howling Abyss</span></div>
        </div>
        <div class="grow" />
        <div class="roles">
          <button :class="{ active: build.mode === 'sr' }" @click="pickMode('sr')">Rift</button>
          <button :class="{ active: build.mode === 'aram' }" @click="pickMode('aram')">ARAM</button>
        </div>
      </div>
      <div class="roles" v-if="build.mode !== 'aram'" style="margin-top: 8px">
        <button v-for="r in ROLES" :key="r" :class="{ active: role === r }" @click="pickRole(r)">
          {{ ROLE_LABEL[r] }}
          <span v-if="build.roles?.find((x) => x.role === r)" class="muted"> {{ build.roles?.find((x) => x.role === r)?.games }}</span>
        </button>
      </div>
      <div class="row" style="margin-top: 10px">
        <span class="badge" :class="build.source">
          {{ build.source === 'opgg' ? 'op.gg ARAM stats' : build.source === 'riot' ? 'Compiled from ranked matches' : build.source === 'lcu' ? 'Riot in-client recommendations' : 'No data' }}
        </span>
        <span v-if="build.tier" class="badge tier" :class="'t' + build.tier">Tier {{ tierLabel(build.tier) }} · #{{ build.rank }}<template v-if="build.pickRate"> · {{ pctf(build.pickRate) }} pick</template></span>
        <span v-if="build.total.games" class="muted">{{ build.total.games }} games · {{ winrate(build.total) }} win rate</span>
      </div>
      <div class="row" style="margin-top: 6px">
        <span v-if="me?.mastery" class="badge lcu" :title="`Highest grade ${me.mastery.highestGrade}`">You: M{{ me.mastery.championLevel }} · {{ fmtPoints(me.mastery.championPoints) }}</span>
        <span v-if="me?.record" class="badge riot">
          Recent {{ build.mode === 'aram' ? 'ARAM' : 'Rift' }}: {{ me.record.wins }}W {{ me.record.games - me.record.wins }}L ·
          {{ ((me.record.kills + me.record.assists) / Math.max(1, me.record.deaths)).toFixed(1) }} KDA
        </span>
        <label class="toggle" :class="{ on: follow }" style="margin-left: auto" @click="follow = !follow">
          <span class="track" /><span class="muted">Follow</span>
        </label>
      </div>
      <div v-if="info" class="playstyle">
        <span class="badge" :class="info.tacticalInfo.damageType === 'kMagic' ? 'ap' : info.tacticalInfo.damageType === 'kPhysical' ? 'ad' : ''">
          {{ DMG[info.tacticalInfo.damageType] ?? '?' }} · {{ info.tacticalInfo.attackType }} · {{ (info.roles ?? []).join(', ') }}
        </span>
        <div v-for="k in (['damage', 'durability', 'crowdControl', 'mobility', 'utility'] as const)" :key="k" class="ps">
          <span class="lbl">{{ k === 'crowdControl' ? 'CC' : k }}</span>
          <span class="pips"><i v-for="n in 3" :key="n" :class="{ on: n <= info.playstyleInfo[k] }" /></span>
        </div>
      </div>
      <div v-for="n in build.notes ?? []" :key="n" class="note">{{ n }}</div>
      <div v-if="err" class="note">{{ err }}</div>
    </section>

    <!-- Augments: one box per rarity -->
    <section v-for="r in RARITIES" :key="r" class="panel aug-col" :class="[r, { compact }]" v-show="build.augments?.length">
      <h2>{{ r }} augments <span class="muted" style="text-transform: none; letter-spacing: 0; margin-left: 8px">{{ build.augScope === 'champion' ? build.champion.name : 'global' }} · aramgg</span></h2>
      <div v-for="a in augBy(build, r)" :key="a.id" class="aug" :title="a.desc">
        <img v-if="a.icon" :src="a.icon" :alt="a.name" />
        <div class="aug-body">
          <div class="aug-name"><span class="tier-pip" :class="'t' + a.tier">{{ tierLabel(a.tier) || '?' }}</span>{{ a.name }}</div>
          <div class="aug-desc">{{ a.desc }}</div>
        </div>
        <div class="stat"><b>{{ pctf(a.winRate) }}</b>{{ a.games }}g · {{ pctf(a.pickRate) }}</div>
      </div>
      <div v-if="!augBy(build, r).length" class="muted">no data</div>
      <button v-if="compact && (build.augments ?? []).filter((a) => a.rarity === r).length > 6" class="sm" style="margin-top: 6px" @click="expandAugs = !expandAugs">{{ expandAugs ? 'Top 6' : 'Show all' }}</button>
    </section>

    <!-- Items -->
    <section v-if="build.starting?.length || build.core?.length" class="panel">
      <h2>Items</h2>
      <template v-if="build.starting?.length">
        <h3>Starting</h3>
        <div class="sets">
          <div v-for="(s, i) in build.starting" :key="i" class="set">
            <div class="items"><img v-for="(it, j) in s.items" :key="j" :src="it.image" :title="it.name" /></div>
            <div class="stat"><b>{{ winrate(s) }}</b>{{ pct(s, build.total.games) }}</div>
          </div>
        </div>
      </template>
      <template v-if="build.core?.length">
        <h3>Core build</h3>
        <div class="sets">
          <div v-for="(s, i) in build.core" :key="i" class="set">
            <div class="items">
              <template v-for="(it, j) in s.items" :key="j">
                <span v-if="j" class="arrow">›</span><img :src="it.image" :title="it.name" />
              </template>
            </div>
            <div class="stat"><b>{{ winrate(s) }}</b>{{ s.games }} games</div>
          </div>
        </div>
      </template>
    </section>

    <section v-if="build.boots?.length || build.late?.length" class="panel">
      <h2>Boots &amp; late items</h2>
      <template v-if="build.boots?.length">
        <h3>Boots</h3>
        <div class="sets">
          <div v-for="(s, i) in build.boots" :key="i" class="set">
            <div class="items"><img v-for="(it, j) in s.items" :key="j" :src="it.image" :title="it.name" /></div>
            <span>{{ s.items[0]?.name }}</span>
            <div class="stat"><b>{{ winrate(s) }}</b>{{ pct(s, build.total.games) }}</div>
          </div>
        </div>
      </template>
      <template v-if="build.late?.length">
        <h3>4th–6th options</h3>
        <div class="sets">
          <div v-for="(s, i) in build.late" :key="i" class="set">
            <div class="items"><img v-for="(it, j) in s.items" :key="j" :src="it.image" :title="it.name" /></div>
            <span>{{ s.items[0]?.name }}</span>
            <div class="stat"><b>{{ winrate(s) }}</b>{{ s.games }} games</div>
          </div>
        </div>
      </template>
    </section>

    <!-- Runes -->
    <section v-if="build.runes?.length" class="panel">
      <h2>Runes <span v-if="applied" class="muted" style="text-transform: none; letter-spacing: 0; margin-left: 8px">{{ applied }}</span></h2>
      <div class="runes">
        <div v-for="(p, i) in build.runes" :key="i" class="rune-page">
          <div class="trees">
            <img :src="p.primary.icon" :alt="p.primary.name" /> {{ p.primary.name }}
            <span>+</span>
            <img :src="p.secondary.icon" :alt="p.secondary.name" /> {{ p.secondary.name }}
            <span class="badge" :class="p.source" style="margin-left: auto">{{ p.source === 'lcu' ? 'Riot rec.' : `${winrate(p)} · ${p.games}g` }}</span>
            <button :disabled="applying || !status?.connected" title="Create this page in the League client and select it" @click="applyRunes(p)">Apply</button>
          </div>
          <div class="perks">
            <img v-for="(r, j) in p.perks" :key="j" :src="r.icon" :title="`${r.name}${r.desc ? ' — ' + r.desc.replace(/<[^>]+>/g, '') : ''}`" :class="{ keystone: j === 0 }" />
            <span class="sep" />
            <img v-for="(r, j) in p.shards" :key="'s' + j" :src="r.icon" :title="r.name" class="shard" />
          </div>
        </div>
      </div>
    </section>

    <!-- Spells + skills -->
    <section v-if="build.spells?.length || build.skillOrder?.length" class="panel">
      <h2>Spells &amp; skills</h2>
      <template v-if="build.spells?.length">
        <div class="sets">
          <div v-for="(s, i) in (build.spells ?? []).slice(0, 2)" :key="i" class="set spells">
            <img v-for="(sp, j) in s.spells" :key="j" :src="sp.image" :title="sp.name" />
            <span>{{ s.spells.map((x) => x.name).join(' + ') }}</span>
            <div v-if="s.games" class="stat"><b>{{ winrate(s) }}</b>{{ s.games }} games</div>
          </div>
        </div>
      </template>
      <template v-if="build.skillOrder?.length">
        <h3>Skill max order</h3>
        <div class="sets">
          <div v-for="(s, i) in build.skillOrder" :key="i" class="set">
            <span class="skills">{{ s.order }}</span>
            <span v-if="build.skillStart?.[i]" class="muted">start {{ build.skillStart[i].order }}</span>
            <div class="stat"><b>{{ winrate(s) }}</b>{{ s.games }} games</div>
          </div>
        </div>
        <div v-if="build.skillPath?.length" class="path">
          <span v-for="(k, i) in build.skillPath" :key="i" class="lv" :class="k.toLowerCase()"><i>{{ i + 1 }}</i>{{ k }}</span>
        </div>
      </template>
    </section>
  </template>
  <section v-else-if="loading" class="panel"><div class="muted">Loading…</div></section>
</template>


<style scoped>
.compact .build-head img { width: 44px; height: 44px; }
.compact .build-head .name { font-size: 18px; }
.compact .aug-desc { display: none; }
.compact .aug { padding: 3px 6px; grid-template-columns: 28px 1fr auto; }
.compact .aug img { width: 28px; height: 28px; }
.compact .playstyle { display: none; }
.badge.opgg { color: var(--hextech-2); border-color: var(--hextech-dim); }
.badge.tier { color: var(--gold-bright); }
.badge.tier.t1 { color: #ff8a3d; border-color: #ff8a3d; }
.badge.tier.t2 { color: var(--gold); border-color: var(--gold); }
.path { display: flex; gap: 3px; flex-wrap: wrap; margin-top: 8px; }
.lv { width: 26px; height: 32px; display: grid; place-items: center; font-family: var(--display); font-weight: 700; font-size: 12px; border: 1px solid var(--gold-deep); background: #010a13; position: relative; }
.lv i { position: absolute; top: 1px; left: 3px; font-size: 8px; font-style: normal; color: var(--dim); }
.lv.q { color: #7fb3ff; } .lv.w { color: #7fe0a0; } .lv.e { color: #f0b232; } .lv.r { color: #e84057; }
.aug-col.prismatic h2 { color: #c7f3ff; background: linear-gradient(90deg, rgba(63,180,216,.22), transparent); border-color: #3fb4d8; }
.aug-col.gold h2 { color: var(--gold-bright); background: linear-gradient(90deg, rgba(200,170,110,.22), transparent); }
.aug-col.silver h2 { color: #d8dde3; background: linear-gradient(90deg, rgba(139,155,176,.22), transparent); border-color: #8b9bb0; }
button.sm { padding: 4px 8px; font-size: 10px; }
.aug { display: grid; grid-template-columns: 36px 1fr auto; gap: 8px; align-items: center; padding: 5px 6px; margin-bottom: 4px; border: 1px solid rgba(200,170,110,.08); background: rgba(255,255,255,.02); }
.aug img { width: 36px; height: 36px; border: 1px solid var(--gold-deep); background: #000; }
.aug-body { min-width: 0; }
.aug-name { font-weight: 600; font-size: 13px; display: flex; align-items: center; gap: 6px; }
.aug-desc { font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.tier-pip { font-family: var(--display); font-size: 10px; width: 16px; height: 16px; display: grid; place-items: center; border: 1px solid var(--gold-deep); color: var(--muted); }
.tier-pip.t1 { color: #ff8a3d; border-color: #ff8a3d; } .tier-pip.t2 { color: var(--gold); border-color: var(--gold); } .tier-pip.t3 { color: var(--hextech-2); border-color: var(--hextech-dim); }
.playstyle { display: flex; gap: 14px; align-items: center; flex-wrap: wrap; margin-top: 10px; }
.ps { display: inline-flex; align-items: center; gap: 6px; font-size: 11px; color: var(--muted); text-transform: capitalize; }
.pips { display: inline-flex; gap: 2px; }
.pips i { width: 10px; height: 10px; border: 1px solid var(--gold-deep); background: #010a13; }
.pips i.on { background: var(--gold); box-shadow: 0 0 6px rgba(200,170,110,.5); }
.badge.ap { color: #7fb3ff; border-color: #2a4a7a; }
.badge.ad { color: #e8a33d; border-color: #7a5a2a; }
</style>
