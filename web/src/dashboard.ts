import { GridStack, type GridStackWidget } from 'gridstack'
import { reactive, type InjectionKey } from 'vue'

export interface WidgetOpts {
  w?: number
  h?: number
}

export interface Dashboard {
  add(id: string, el: HTMLElement, opts: WidgetOpts): void
  remove(el: HTMLElement): void
  /** ids of widgets currently mounted (reactive) */
  mounted: Set<string>
}

/** Every box a screen can show, so edit mode can offer a ghost slot for the ones not on screen yet. */
export interface WidgetSpec {
  id: string
  title: string
  w: number
  h: number
  when?: string
}
const BUILD_WIDGETS: WidgetSpec[] = [
  { id: 'build-head', title: 'Champion', w: 3, h: 7 },
  { id: 'items', title: 'Items', w: 3, h: 12 },
  { id: 'boots', title: 'Boots & late items', w: 3, h: 12 },
  { id: 'runes', title: 'Runes', w: 3, h: 10 },
  { id: 'spells', title: 'Spells & skills', w: 3, h: 10 },
  { id: 'aug-prismatic', title: 'Prismatic augments', w: 3, h: 10, when: 'ARAM Mayhem / Arena' },
  { id: 'aug-gold', title: 'Gold augments', w: 3, h: 10, when: 'ARAM Mayhem / Arena' },
  { id: 'aug-silver', title: 'Silver augments', w: 3, h: 10, when: 'ARAM Mayhem / Arena' },
  { id: 'prismatic', title: 'Prismatic items', w: 3, h: 10, when: 'Arena' },
  { id: 'synergies', title: 'Best partners', w: 3, h: 10, when: 'Arena' },
]
export const CATALOG: Record<string, WidgetSpec[]> = {
  idle: [
    { id: 'queue', title: 'Queue watcher', w: 3, h: 10 },
    { id: 'postgame', title: 'Post-game', w: 6, h: 12, when: 'after a game' },
    ...BUILD_WIDGETS,
    { id: 'picker', title: 'Champion picker', w: 3, h: 9 },
    { id: 'compile', title: 'Build data', w: 3, h: 6 },
  ],
  select: [
    { id: 'cs-team', title: 'Champ select · your team', w: 4, h: 9 },
    { id: 'cs-enemies', title: 'Champ select · enemies', w: 4, h: 7, when: 'once enemy picks are visible' },
    { id: 'cs-bench', title: 'Bench', w: 4, h: 8, when: 'ARAM' },
    ...BUILD_WIDGETS,
  ],
  game: [
    { id: 'live-status', title: 'Live', w: 3, h: 6 },
    { id: 'live-enemies', title: 'Enemies', w: 6, h: 10, when: 'Rift / ARAM' },
    { id: 'live-allies', title: 'Your team', w: 6, h: 10, when: 'Rift / ARAM' },
    { id: 'arena-mine', title: 'Your team (Arena)', w: 6, h: 7, when: 'Arena' },
    { id: 'arena-teams', title: 'Enemy teams (Arena)', w: 6, h: 24, when: 'Arena' },
    { id: 'live-you', title: 'You', w: 3, h: 8 },
    { id: 'live-objectives', title: 'Objectives', w: 3, h: 9, when: 'Rift' },
    { id: 'live-matchup', title: 'Matchup', w: 3, h: 8, when: 'Rift' },
    { id: 'live-shopping', title: 'Shopping', w: 3, h: 8 },
    { id: 'live-killfeed', title: 'Kill feed', w: 3, h: 8 },
    ...BUILD_WIDGETS,
  ],
}

export const DASH_KEY: InjectionKey<Dashboard> = Symbol('dashboard')

const COLUMNS = 12

/**
 * Default arrangement: widgets without a saved position are laid out in this
 * priority order (top-left first), regardless of the order they mounted in.
 * Ids not listed come after, in mount order. W is the default width in columns.
 */
const ORDER = [
  'live-status', 'arena-mine', 'live-enemies', 'live-you', 'arena-teams', 'live-allies', 'live-objectives', 'live-matchup', 'live-shopping', 'live-killfeed',
  'cs-team', 'cs-enemies', 'cs-bench',
  'queue', 'postgame',
  'build-head', 'prismatic', 'synergies', 'items', 'boots', 'runes', 'spells', 'aug-prismatic', 'aug-gold', 'aug-silver',
  'picker', 'compile', 'build-empty', 'build-loading',
]
/** Default height cap (rows) for boxes whose content keeps growing (feeds, lists). */
const MAXH: Record<string, number> = { 'live-shopping': 8, 'live-killfeed': 8, queue: 10, picker: 9, boots: 14, 'aug-prismatic': 10, 'aug-gold': 10, 'aug-silver': 10, 'arena-teams': 30 }
const W: Record<string, number> = {
  'live-enemies': 6, 'live-allies': 6, 'arena-mine': 6, 'arena-teams': 6, 'cs-team': 4, 'cs-enemies': 4, 'cs-bench': 4, postgame: 6,
}
const CELL = 40
const MARGIN = 6
const STORE = 'ezlol.layout.v2'
const PRESET = 'ezlol.layout.preset.v1' // user-saved arrangements, restored by Reset

type Pos = { x: number; y: number; w: number; h: number }
type Layouts = Record<string, Record<string, Pos>>

/** Natural height of a widget's content (ignoring the box it currently sits in). */
function naturalHeight(el: HTMLElement): number {
  const section = el.firstElementChild as HTMLElement | null
  const inner = section?.querySelector<HTMLElement>('.wcontent')
  if (!section || !inner) return 0
  inner.classList.add('measuring')
  const cs = getComputedStyle(section)
  const h = inner.scrollHeight + parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom)
  inner.classList.remove('measuring')
  return h
}

/**
 * Grid rows needed for a content height. gridstack rows are CELL px tall with the
 * MARGIN taken from inside each item, so an h-row item shows h*CELL - 2*MARGIN px.
 */
function rowsFor(contentHeight: number): number {
  return Math.min(40, Math.max(2, Math.ceil((contentHeight + 2 * MARGIN + 2) / CELL)))
}

function loadJSON(key: string): Layouts {
  try {
    return JSON.parse(localStorage.getItem(key) ?? '{}')
  } catch {
    return {}
  }
}
function saveJSON(key: string, l: Layouts) {
  try {
    localStorage.setItem(key, JSON.stringify(l))
  } catch {
    /* ignore */
  }
}
const loadLayouts = () => loadJSON(STORE)
const saveLayouts = (l: Layouts) => saveJSON(STORE, l)

/**
 * One grid per screen mode (idle / select / game). Widgets register themselves
 * when they mount; positions come from the saved layout for the current mode or,
 * for a widget never placed by the user, from auto-placement with a size derived
 * from its content height. Layout changes are saved on every drag/resize.
 */
export class DashboardGrid implements Dashboard {
  private grid: GridStack | null = null
  private pending: { id: string; el: HTMLElement; opts: WidgetOpts }[] = []
  private layouts = loadLayouts()
  private presets = loadJSON(PRESET)
  private editing = false
  // Widgets without a saved position keep tracking their content height (data and
  // fonts arrive after mount) until the user has arranged the screen.
  private auto = new Map<HTMLElement, ResizeObserver>()
  readonly mounted = reactive(new Set<string>())

  constructor(private mode: () => string) {}

  private reloading = false

  attach(container: HTMLElement) {
    this.reloading = true
    this.detach()
    this.grid = GridStack.init(
      { column: COLUMNS, cellHeight: CELL, margin: MARGIN, float: true, animate: false, staticGrid: !this.editing, minRow: 1, resizable: { handles: 'se,e,s' } },
      container,
    )
    const g = this.grid
    if (!g) return
    g.on('change', () => {
      if (this.editing) this.persist()
    })
    // Place everything: widgets queued while there was no grid, plus any item the
    // new container already held (a remount mounts children before this runs).
    const queued = this.pending.splice(0)
    const seen = new Set<HTMLElement>()
    for (const p of queued) {
      seen.add(p.el)
      this.place(p.id, p.el, p.opts)
    }
    for (const child of Array.from(container.querySelectorAll<HTMLElement>(':scope > .grid-stack-item'))) {
      if (seen.has(child)) continue
      const id = child.getAttribute('gs-id')
      if (!id) continue
      let opts: WidgetOpts = {}
      try {
        opts = JSON.parse(child.dataset.gsOpts ?? '{}')
      } catch {
        /* ignore */
      }
      this.place(id, child, opts)
    }
    this.reloading = false
  }

  detach() {
    for (const ro of this.auto.values()) ro.disconnect()
    this.auto.clear()
    if (this.grid) {
      // Unhook first: destroy() fires change events that would persist the
      // outgoing grid's positions over a layout we just restored.
      this.grid.offAll()
      this.grid.destroy(false)
      this.grid = null
    }
  }

  setEditing(on: boolean) {
    this.editing = on
    this.grid?.setStatic(!on)
  }

  /** Snapshot the current arrangement as this screen's saved layout. */
  savePreset() {
    this.persist(true)
    const cur = this.layouts[this.mode()]
    if (!cur) return
    this.presets[this.mode()] = JSON.parse(JSON.stringify(cur))
    saveJSON(PRESET, this.presets)
  }

  hasPreset(): boolean {
    return !!this.presets[this.mode()]
  }

  /** Drop the saved layout for this screen (Reset then falls back to the built-in default). */
  clearPreset() {
    delete this.presets[this.mode()]
    saveJSON(PRESET, this.presets)
  }

  /** Restore this screen's saved layout, or the built-in default if none was saved. Caller remounts. */
  reset() {
    const p = this.presets[this.mode()]
    if (p) this.layouts[this.mode()] = JSON.parse(JSON.stringify(p))
    else delete this.layouts[this.mode()]
    saveLayouts(this.layouts)
  }

  add(id: string, el: HTMLElement, opts: WidgetOpts) {
    if (el.classList.contains('ghost')) {
      // Ghosts live under their own id in the grid so they never collide with the
      // real box (gridstack would rename a duplicate id to "<id>_1"); persist()
      // maps them back to the real id.
      el.setAttribute('gs-id', 'ghost:' + id)
      id = 'ghost:' + id
    } else {
      this.mounted.add(id)
    }
    el.dataset.gsOpts = JSON.stringify(opts)
    // No grid yet, or the grid belongs to a container being replaced (remount):
    // queue until attach() runs on the new container.
    if (!this.grid || !this.grid.el.contains(el)) {
      this.pending.push({ id, el, opts })
      return
    }
    this.place(id, el, opts)
  }

  remove(el: HTMLElement) {
    const id = el.getAttribute('gs-id')
    if (id && !el.classList.contains('ghost')) this.mounted.delete(id)
    this.auto.get(el)?.disconnect()
    this.auto.delete(el)
    this.pending = this.pending.filter((p) => p.el !== el)
    if (this.grid && el.classList.contains('grid-stack-item')) this.grid.removeWidget(el, false)
  }

  private place(id: string, el: HTMLElement, opts: WidgetOpts) {
    if (!this.grid) return
    // GridStack.init adopts any .grid-stack-item already in the container as a 1x1
    // node; drop that node so makeWidget can apply our real size and position.
    if ((el as HTMLElement & { gridstackNode?: unknown }).gridstackNode) this.grid.removeWidget(el, false, false)
    const realID = id.startsWith('ghost:') ? id.slice(6) : id
    const saved = this.layouts[this.mode()]?.[realID]
    const w = Math.min(COLUMNS, Math.max(2, saved?.w ?? W[realID] ?? opts.w ?? 3))
    const spec: GridStackWidget = { id, w }
    if (saved) {
      spec.x = saved.x
      spec.y = saved.y
      spec.h = saved.h
    } else {
      spec.h = Math.min(MAXH[realID] ?? 40, opts.h ?? this.measureRows(el, w))
      spec.autoPosition = true
    }
    this.grid.makeWidget(el, spec)
    if (el.classList.contains('ghost')) {
      if (this.editing) this.persist()
      return
    }
    if (!saved) {
      this.track(el, MAXH[realID] ?? 40)
      this.scheduleRelayout()
    }
  }

  private relayoutTimer: ReturnType<typeof setTimeout> | null = null
  private scheduleRelayout() {
    if (this.relayoutTimer) clearTimeout(this.relayoutTimer)
    this.relayoutTimer = setTimeout(() => this.relayout(), 60)
  }

  /** Re-place every unsaved widget in ORDER priority so late mounts still land where they belong. */
  private relayout() {
    const g = this.grid
    if (!g || this.editing) return
    const saved = this.layouts[this.mode()] ?? {}
    type Node = { el?: HTMLElement; id?: string; w?: number; h?: number }
    const nodes = (g.engine.nodes as Node[]).filter((n) => n.el && n.id && !saved[String(n.id)] && !n.el.classList.contains('ghost'))
    const rank = (id: string) => {
      const i = ORDER.indexOf(id)
      return i < 0 ? ORDER.length : i
    }
    const mountIndex = new Map(nodes.map((n, i) => [n, i]))
    nodes.sort((a, b) => rank(String(a.id)) - rank(String(b.id)) || (mountIndex.get(a) ?? 0) - (mountIndex.get(b) ?? 0))
    // (ghosts are excluded above, so ids here are real ids)
    g.batchUpdate()
    for (const n of nodes) g.removeWidget(n.el!, false, false)
    for (const n of nodes) g.makeWidget(n.el!, { id: n.id, w: n.w, h: n.h, autoPosition: true })
    g.batchUpdate(false)
  }

  /** Grow/shrink an auto-sized widget as its content settles; stops once the user edits. */
  private track(el: HTMLElement, maxRows: number) {
    if (!el.firstElementChild) return
    let last = 0
    const tick = () => {
      if (!this.grid || this.editing) return
      const rows = Math.min(maxRows, rowsFor(naturalHeight(el)))
      if (rows === last) return
      last = rows
      const node = (el as HTMLElement & { gridstackNode?: { h?: number } }).gridstackNode
      if (node && node.h !== rows) {
        this.grid.update(el, { h: rows })
        this.scheduleRelayout()
      }
    }
    // Content that was mid-load at mount (fonts, data) settles within a few seconds;
    // poll for that window, then leave the layout alone during play.
    const timer = setInterval(tick, 400)
    const ro = { disconnect: () => clearInterval(timer) } as ResizeObserver
    this.auto.set(el, ro)
    setTimeout(() => {
      clearInterval(timer)
      this.auto.delete(el)
    }, 8000)
  }

  /** Rows needed to show the widget's content without an inner scrollbar, at width w. */
  private measureRows(el: HTMLElement, w: number): number {
    const content = el.firstElementChild as HTMLElement | null
    if (!content || !this.grid) return 6
    const colWidth = this.grid.el.clientWidth / COLUMNS
    const prev = { position: el.style.position, width: el.style.width, height: el.style.height }
    el.style.position = 'relative'
    el.style.width = `${Math.floor(colWidth * w)}px`
    el.style.height = 'auto'
    const h = naturalHeight(el)
    el.style.position = prev.position
    el.style.width = prev.width
    el.style.height = prev.height
    return rowsFor(h)
  }

  private persist(force = false) {
    if (!this.grid || this.reloading) return
    if (!this.editing && !force) return
    const nodes = this.grid.save(false) as GridStackWidget[]
    const m: Record<string, Pos> = {}
    for (const n of nodes) {
      if (!n.id) continue
      let id = String(n.id)
      if (id.startsWith('ghost:')) id = id.slice(6)
      if (id === 'build-loading' || id === 'build-empty') continue // transient placeholders
      // A real box wins over its ghost if both are somehow present.
      if (m[id] && !String(n.id).startsWith('ghost:')) m[id] = { x: n.x ?? 0, y: n.y ?? 0, w: n.w ?? 3, h: n.h ?? 4 }
      else if (!m[id]) m[id] = { x: n.x ?? 0, y: n.y ?? 0, w: n.w ?? 3, h: n.h ?? 4 }
    }
    this.layouts[this.mode()] = m
    saveLayouts(this.layouts)
  }
}
