import { GridStack, type GridStackWidget } from 'gridstack'
import type { InjectionKey } from 'vue'

export interface WidgetOpts {
  w?: number
  h?: number
}

export interface Dashboard {
  add(id: string, el: HTMLElement, opts: WidgetOpts): void
  remove(el: HTMLElement): void
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

function loadLayouts(): Layouts {
  try {
    return JSON.parse(localStorage.getItem(STORE) ?? '{}')
  } catch {
    return {}
  }
}
function saveLayouts(l: Layouts) {
  try {
    localStorage.setItem(STORE, JSON.stringify(l))
  } catch {
    /* ignore */
  }
}

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
  private editing = false
  // Widgets without a saved position keep tracking their content height (data and
  // fonts arrive after mount) until the user has arranged the screen.
  private auto = new Map<HTMLElement, ResizeObserver>()

  constructor(private mode: () => string) {}

  attach(container: HTMLElement) {
    this.detach()
    this.grid = GridStack.init(
      { column: COLUMNS, cellHeight: CELL, margin: MARGIN, float: false, animate: false, staticGrid: !this.editing, minRow: 1, resizable: { handles: 'se,e,s' } },
      container,
    )
    const g = this.grid
    if (!g) return
    g.on('change', () => {
      if (this.editing) this.persist()
    })
    for (const p of this.pending.splice(0)) this.place(p.id, p.el, p.opts)
  }

  detach() {
    for (const ro of this.auto.values()) ro.disconnect()
    this.auto.clear()
    if (this.grid) {
      this.grid.destroy(false)
      this.grid = null
    }
  }

  setEditing(on: boolean) {
    this.editing = on
    this.grid?.setStatic(!on)
  }

  /** Forget the saved layout for the current mode; caller remounts the widgets. */
  reset() {
    delete this.layouts[this.mode()]
    saveLayouts(this.layouts)
  }

  add(id: string, el: HTMLElement, opts: WidgetOpts) {
    if (!this.grid) {
      this.pending.push({ id, el, opts })
      return
    }
    this.place(id, el, opts)
  }

  remove(el: HTMLElement) {
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
    const saved = this.layouts[this.mode()]?.[id]
    const w = Math.min(COLUMNS, Math.max(2, saved?.w ?? W[id] ?? opts.w ?? 3))
    const spec: GridStackWidget = { id, w }
    if (saved) {
      spec.x = saved.x
      spec.y = saved.y
      spec.h = saved.h
    } else {
      spec.h = Math.min(MAXH[id] ?? 40, opts.h ?? this.measureRows(el, w))
      spec.autoPosition = true
    }
    this.grid.makeWidget(el, spec)
    if (!saved) {
      this.track(el, MAXH[id] ?? 40)
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
    const nodes = (g.engine.nodes as Node[]).filter((n) => n.el && n.id && !saved[String(n.id)])
    const rank = (id: string) => {
      const i = ORDER.indexOf(id)
      return i < 0 ? ORDER.length : i
    }
    const mountIndex = new Map(nodes.map((n, i) => [n, i]))
    nodes.sort((a, b) => rank(String(a.id)) - rank(String(b.id)) || (mountIndex.get(a) ?? 0) - (mountIndex.get(b) ?? 0))
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

  private persist() {
    if (!this.grid) return
    const nodes = this.grid.save(false) as GridStackWidget[]
    const m: Record<string, Pos> = {}
    for (const n of nodes) if (n.id) m[String(n.id)] = { x: n.x ?? 0, y: n.y ?? 0, w: n.w ?? 3, h: n.h ?? 4 }
    this.layouts[this.mode()] = m
    saveLayouts(this.layouts)
  }
}
