export interface ReadyCheck {
  state: string
  playerResponse: string
  timer: number
}

export interface Status {
  connected: boolean
  phase: string
  autoAccept: boolean
  summoner: string
  gameVersion: string
  patch: string
  readyCheck?: ReadyCheck
  lastAccept?: string
  acceptCount: number
  queueId: number
  queueName: string
  mapId: number
  gameMode: string
  pickedChampion: number
  pickedPosition: string
  error?: string
  updatedAt: string
}

export interface LogEntry {
  time: string
  level: string
  message: string
}

export interface Champion {
  id: number
  key: string
  name: string
  title: string
  tags: string[]
  image: string
  attack: number
  magic: number
}

export interface Count {
  games: number
  wins: number
}

export interface ItemRef {
  id: number
  name: string
  image: string
}
export interface ItemSet extends Count {
  items: ItemRef[]
}
export interface RuneRef {
  id: number
  name: string
  icon: string
  desc?: string
}
export interface StyleRef {
  id: number
  name: string
  icon: string
}
export interface RunePage extends Count {
  primary: StyleRef
  secondary: StyleRef
  perks: RuneRef[]
  shards: RuneRef[]
  source: string
}
export interface SpellRef {
  id: number
  name: string
  image: string
}
export interface SpellPair extends Count {
  spells: SpellRef[]
}
export interface Skills extends Count {
  order: string
}
export interface RoleCount extends Count {
  role: string
}

export interface Build {
  champion: Champion
  patch: string
  role: string
  mode: string
  roles: RoleCount[] | null
  source: string
  total: Count
  starting: ItemSet[] | null
  core: ItemSet[] | null
  boots: ItemSet[] | null
  late: ItemSet[] | null
  items: ItemSet[] | null
  runes: RunePage[] | null
  spells: SpellPair[] | null
  skillOrder: Skills[] | null
  skillStart: Skills[] | null
  skillPath?: string[]
  tier?: number
  rank?: number
  pickRate?: number
  augments?: Augment[]
  augScope?: string
  notes: string[] | null
}
export interface Augment {
  id: number
  name: string
  desc: string
  rarity: string
  icon: string
  tier: number
  winRate: number
  pickRate: number
  games: number
  scope: string
}

export interface Progress {
  running: boolean
  phase: string
  queue: number
  players: number
  matchesDone: number
  target: number
  requests: number
  errors: number
  lastError?: string
  startedAt?: string
  finishedAt?: string
}

export interface BuildsStatus {
  hasKey: boolean
  platform: string
  progress: Progress
  patches: { patch: string; matches: number; champions: number }[] | null
}

export interface LiveItem {
  id: number
  name: string
  image: string
  count: number
}
export interface LiveSpell {
  id: number
  name: string
  image: string
  cooldown: number
}
export interface LivePlayer {
  name: string
  champion: Champion
  team: string
  isDead: boolean
  respawnTimer: number
  level: number
  kills: number
  deaths: number
  assists: number
  cs: number
  items: LiveItem[] | null
  spells: LiveSpell[] | null
  keystone: string
  isMe: boolean
  itemAD: number
  itemAP: number
  itemArmor: number
  itemMR: number
  itemHP: number
}
export interface LiveEvent {
  EventID: number
  EventName: string
  EventTime: number
  KillerName?: string
  VictimName?: string
  Assisters?: string[]
  KillStreak?: number
  Acer?: string
  AcingTeam?: string
}
export interface ActivePlayer {
  abilities: Record<string, { abilityLevel: number; displayName: string }>
  championStats: Record<string, number | string>
  currentGold: number
  level: number
}
export interface Live {
  inGame: boolean
  gameTime: number
  gameMode: string
  mapId: number
  me?: ActivePlayer
  myTeam: string
  players: LivePlayer[] | null
  events: LiveEvent[] | null
  hint: string
  focus: string
  focusWhy: string
  offer?: AugmentOffer
  ocr: string
}
export interface AugmentOffer {
  active: boolean
  pending: boolean
  level: number
  offered: Augment[] | null
  best?: string
  why?: string
}

export interface Mastery {
  championId: number
  championLevel: number
  championPoints: number
  highestGrade: string
}
export interface PlayRecord {
  games: number
  wins: number
  kills: number
  deaths: number
  assists: number
}
export interface CsPlayer {
  name: string
  champion: Champion
  position: string
  isMe: boolean
  cellId: number
  tradeId?: number
  trade?: string
  mastery?: Mastery
  record?: PlayRecord
}
export interface CsBench {
  champion: Champion
  mastery?: Mastery
  record?: PlayRecord
  score: number
  why: string
}
export interface CompSummary {
  tanks: number
  ranged: number
  melee: number
  cc: number
  durability: number
  damage: number
  utility: number
  needs: string[] | null
}
export interface DamageProfile {
  attack: number
  magic: number
  hint: string
}
export interface ChampSelect {
  active: boolean
  benchEnabled: boolean
  rerollsRemaining: number
  timeLeft: number
  phase: string
  me?: CsPlayer
  myTeam: CsPlayer[] | null
  theirTeam: Champion[] | null
  bench: CsBench[] | null
  mode: string
  enemyProfile: DamageProfile
  allyProfile: DamageProfile
  allyComp: CompSummary
  enemyComp: CompSummary
}

export interface ChampionInfo {
  roles: string[]
  tacticalInfo: { style: number; difficulty: number; damageType: string; attackType: string }
  playstyleInfo: { damage: number; durability: number; crowdControl: number; mobility: number; utility: number }
}
export interface EogPlayer {
  championId: number
  gameName: string
  summonerName: string
  isLocalPlayer: boolean
  stats: Record<string, number | string | boolean>
  champion: Champion
  itemRefs: ItemRef[] | null
}
export interface Eog {
  available: boolean
  gameLength: number
  gameMode: string
  queueId: number
  teams: { isWinningTeam: boolean; teamId: number; players: EogPlayer[] | null }[] | null
}

export interface ChampionSpell {
  key: string
  name: string
  image: string
  cooldowns: number[]
  costs: number[]
  maxRank: number
  tooltip: string
}
export interface ChampionDetail {
  passive: { name: string; image: string; desc: string }
  spells: ChampionSpell[]
}

export interface SessionGame {
  championId: number
  win: boolean
  kills: number
  deaths: number
  assists: number
  queueId: number
  duration: number
  created: number
  champion: Champion
}
export interface Session {
  games: SessionGame[]
  wins: number
  losses: number
}
