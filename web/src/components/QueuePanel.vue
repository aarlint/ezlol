<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api } from '../api'
import type { Champion, LogEntry, Session, Status } from '../types'

const props = defineProps<{ status: Status | null; logs: LogEntry[] }>()
const emit = defineEmits<{ select: [c: Champion] }>()

const session = ref<Session | null>(null)
async function loadSession() {
  try {
    session.value = await api.session()
  } catch {
    /* ignore */
  }
}
let timer: number | undefined
onMounted(() => {
  loadSession()
  timer = window.setInterval(loadSession, 60000)
})
onUnmounted(() => clearInterval(timer))
// Refresh the tally when a game ends or the client connects.
watch(
  () => [props.status?.phase, props.status?.connected],
  ([p, c], [pp]) => {
    if ((p === 'EndOfGame' && pp !== 'EndOfGame') || (c && p === 'Lobby')) setTimeout(loadSession, 3000)
  },
)
// Current streak from today's games (newest first).
const streak = computed(() => {
  const g = session.value?.games ?? []
  if (!g.length) return { n: 0, win: false }
  let n = 0
  for (const x of g) {
    if (x.win !== g[0].win) break
    n++
  }
  return { n, win: g[0].win }
})
const phaseLabel = computed(() => {
  const p = props.status?.phase
  if (!props.status?.connected) return 'Waiting for League client…'
  switch (p) {
    case 'None':
    case '':
      return 'Idle'
    case 'Lobby':
      return 'In lobby'
    case 'Matchmaking':
      return 'In queue'
    case 'ReadyCheck':
      return 'QUEUE POPPED'
    case 'ChampSelect':
      return 'Champ select'
    case 'InProgress':
      return 'In game'
    case 'EndOfGame':
    case 'PreEndOfGame':
      return 'Post-game'
    case 'WaitingForStats':
      return 'Waiting for stats'
    default:
      return p ?? ''
  }
})

const time = (s: string) => new Date(s).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
async function acceptNow() {
  try {
    await api.accept()
  } catch {
    /* logged server-side */
  }
}
</script>

<template>
  <section class="panel">
    <h2>Queue watcher</h2>
    <div class="phase" :class="{ pop: status?.phase === 'ReadyCheck', queue: status?.phase === 'Matchmaking' }">{{ phaseLabel }}</div>
    <div v-if="status?.readyCheck" style="margin-bottom: 10px">
      <div class="progress"><div :style="{ width: `${Math.max(0, 100 - (100 * status.readyCheck.timer) / 12)}%` }" /></div>
      <div class="row">
        <span class="muted">{{ status.readyCheck.playerResponse === 'Accepted' ? 'Accepted — waiting for others' : `Respond within ${Math.max(0, Math.round(12 - status.readyCheck.timer))}s` }}</span>
        <button v-if="status.readyCheck.playerResponse === 'None'" class="primary" @click="acceptNow">Accept now</button>
      </div>
    </div>
    <div class="kv" v-if="status">
      <span>Accepted</span><b>{{ status.acceptCount }} {{ status.lastAccept ? `· last ${time(status.lastAccept)}` : '' }}</b>
      <span>Auto-accept</span><b>{{ status.autoAccept ? 'armed' : 'off' }}</b>
      <template v-if="status.error"><span>Note</span><b>{{ status.error }}</b></template>
    </div>
    <h3 v-if="session?.games.length">
      Today
      <span class="muted" style="text-transform: none; letter-spacing: 0"
        >{{ session.wins }}W {{ session.losses }}L<template v-if="streak.n >= 2">
          · {{ streak.n }} {{ streak.win ? 'win' : 'loss' }} streak</template
        ></span
      >
    </h3>
    <div v-if="streak.n >= 3 && !streak.win" class="note" style="margin: 0 0 8px">{{ streak.n }} losses in a row. Water break, then queue.</div>
    <div v-else-if="streak.n >= 3 && streak.win" class="note" style="margin: 0 0 8px; border-color: var(--hextech-2)">{{ streak.n }} in a row. Keep going.</div>
    <div v-if="session?.games.length" class="today">
      <div v-for="g in session.games" :key="g.created" class="tg" :class="g.win ? 'win' : 'loss'" :title="`${g.champion.name} ${g.kills}/${g.deaths}/${g.assists} — click for build`" @click="emit('select', g.champion)">
        <img :src="g.champion.image" :alt="g.champion.name" />
        <span>{{ g.kills }}/{{ g.deaths }}/{{ g.assists }}</span>
      </div>
    </div>
    <h3>Events</h3>
    <div class="log">
      <div v-for="(l, i) in [...logs].reverse()" :key="i" class="row" :class="l.level">
        <span class="t">{{ time(l.time) }}</span><span>{{ l.message }}</span>
      </div>
      <div v-if="!logs.length" class="muted">No events yet.</div>
    </div>
  </section>
</template>

<style scoped>
.today { display: flex; gap: 6px; flex-wrap: wrap; }
.tg { cursor: pointer; display: flex; flex-direction: column; align-items: center; font-size: 10px; color: var(--muted); width: 40px; }
.tg img { width: 34px; height: 34px; border: 2px solid var(--red); }
.tg.win img { border-color: var(--hextech-2); }
</style>
