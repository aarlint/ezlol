<script setup lang="ts">
// Item-gold lead over the game: your team above the zero line, the enemy below.
// One diverging series (ally hue vs enemy hue, neutral zero line); position
// carries the side as well as colour, and the hover tooltip carries the numbers.
import { computed, onUnmounted, ref, watch } from 'vue'
import { fmtTime } from '../api'
import type { Momentum, MomentumMark, MomentumPoint } from '../types'

const props = defineProps<{ momentum: Momentum; gameTime: number }>()

const box = ref<HTMLElement | null>(null)
// The graph fills whatever room the box gives it (the SVG is absolutely placed
// inside .gg-box, so its size never feeds back into the layout).
const width = ref(320)
const height = ref(120)
const pad = { l: 34, r: 8, t: 8, b: 18 }
const ro =
  typeof ResizeObserver === 'undefined'
    ? null
    : new ResizeObserver(([e]) => {
        width.value = Math.max(160, e.contentRect.width)
        height.value = Math.max(90, e.contentRect.height)
      })
watch(box, (el, old) => {
  if (old) ro?.unobserve(old)
  if (el) ro?.observe(el)
})
onUnmounted(() => ro?.disconnect())

const pts = computed<MomentumPoint[]>(() => props.momentum.points ?? [])
const diff = (p: MomentumPoint) => p.ally - p.enemy
const tMax = computed(() => Math.max(60, props.gameTime, pts.value[pts.value.length - 1]?.t ?? 0))
// Symmetric scale, rounded to a whole 1k step (at least ±1k).
const yMax = computed(() => {
  const m = Math.max(1000, ...pts.value.map((p) => Math.abs(diff(p))))
  const step = m > 8000 ? 4000 : m > 4000 ? 2000 : 1000
  return Math.ceil(m / step) * step
})
const plotW = computed(() => width.value - pad.l - pad.r)
const plotH = computed(() => height.value - pad.t - pad.b)
const x = (t: number) => pad.l + (t / tMax.value) * plotW.value
const y = (v: number) => pad.t + plotH.value / 2 - (v / yMax.value) * (plotH.value / 2)
const zeroY = computed(() => y(0))

const linePath = computed(() => pts.value.map((p, i) => `${i ? 'L' : 'M'}${x(p.t).toFixed(1)},${y(diff(p)).toFixed(1)}`).join(''))
const areaPath = computed(() => {
  const p = pts.value
  if (p.length < 2) return ''
  return `${linePath.value}L${x(p[p.length - 1].t).toFixed(1)},${zeroY.value}L${x(p[0].t).toFixed(1)},${zeroY.value}Z`
})

// Value of the series at time t (linear between samples), for placing marks.
function at(t: number): number {
  const p = pts.value
  if (!p.length) return 0
  if (t <= p[0].t) return diff(p[0])
  for (let i = 1; i < p.length; i++) {
    if (p[i].t >= t) {
      const a = p[i - 1]
      const b = p[i]
      const f = b.t === a.t ? 1 : (t - a.t) / (b.t - a.t)
      return diff(a) + f * (diff(b) - diff(a))
    }
  }
  return diff(p[p.length - 1])
}
const marks = computed(() => (props.momentum.marks ?? []).filter((m) => m.t <= tMax.value).map((m) => ({ ...m, cx: x(m.t), cy: y(at(m.t)) })))

const xTicks = computed(() => {
  const step = tMax.value > 1800 ? 600 : tMax.value > 600 ? 300 : 120
  const out: number[] = []
  for (let t = step; t < tMax.value - step * 0.3; t += step) out.push(t)
  return out
})
const k = (v: number) => `${v < 0 ? '−' : v > 0 ? '+' : ''}${(Math.abs(v) / 1000).toFixed(1)}k`
const last = computed(() => pts.value[pts.value.length - 1])
const lead = computed(() => (last.value ? diff(last.value) : 0))

// Hover: nearest sample to the pointer, plus any marks within a few pixels.
const hover = ref<{ p: MomentumPoint; px: number; marks: MomentumMark[] } | null>(null)
function onMove(e: PointerEvent) {
  const svg = e.currentTarget as SVGSVGElement
  const r = svg.getBoundingClientRect()
  const px = e.clientX - r.left
  if (!pts.value.length || px < pad.l - 4) return (hover.value = null)
  const t = ((px - pad.l) / plotW.value) * tMax.value
  let best = pts.value[0]
  for (const p of pts.value) if (Math.abs(p.t - t) < Math.abs(best.t - t)) best = p
  const near = marks.value.filter((m) => Math.abs(m.cx - px) <= 8)
  hover.value = { p: best, px: x(best.t), marks: near }
}
const tipLeft = computed(() => (hover.value ? Math.min(Math.max(hover.value.px - 80, 0), width.value - 170) : 0))
const uid = Math.random().toString(36).slice(2, 8)
</script>

<template>
  <div class="gg">
    <div class="gg-head">
      <div class="gg-lead">
        <b :class="lead > 0 ? 'ally' : lead < 0 ? 'enemy' : ''">{{ k(lead) }}</b>
        <span class="muted">{{ lead > 0 ? 'your team ahead' : lead < 0 ? 'enemy ahead' : 'even' }} in item gold</span>
      </div>
      <div v-if="last" class="gg-kills muted">kills {{ last.ak }}–{{ last.ek }}</div>
    </div>
    <div ref="box" class="gg-box">
      <svg
        :width="width"
        :height="height"
        :viewBox="`0 0 ${width} ${height}`"
        role="img"
        :aria-label="`Item gold lead over time. Now ${k(lead)} at ${fmtTime(last?.t ?? 0)}.`"
        @pointermove="onMove"
        @pointerleave="hover = null"
      >
        <defs>
          <clipPath :id="`up-${uid}`"><rect :x="0" :y="0" :width="width" :height="zeroY" /></clipPath>
          <clipPath :id="`dn-${uid}`"><rect :x="0" :y="zeroY" :width="width" :height="height - zeroY" /></clipPath>
        </defs>
        <!-- axes: hairline grid at ±max, zero line, time ticks -->
        <line :x1="pad.l" :x2="width - pad.r" :y1="y(yMax)" :y2="y(yMax)" class="grid" />
        <line :x1="pad.l" :x2="width - pad.r" :y1="y(-yMax)" :y2="y(-yMax)" class="grid" />
        <line v-for="t in xTicks" :key="t" :x1="x(t)" :x2="x(t)" :y1="pad.t" :y2="height - pad.b" class="grid" />
        <text :x="pad.l - 4" :y="y(yMax) + 4" class="ax" text-anchor="end">{{ k(yMax) }}</text>
        <text :x="pad.l - 4" :y="zeroY + 4" class="ax" text-anchor="end">0</text>
        <text :x="pad.l - 4" :y="y(-yMax) + 4" class="ax" text-anchor="end">{{ k(-yMax) }}</text>
        <text v-for="t in xTicks" :key="'l' + t" :x="x(t)" :y="height - 4" class="ax" text-anchor="middle">{{ fmtTime(t) }}</text>
        <text :x="pad.l + 4" :y="pad.t + 11" class="side">YOU</text>
        <text :x="pad.l + 4" :y="height - pad.b - 4" class="side">ENEMY</text>
        <!-- the series: wash + 2px line, ally hue above zero, enemy hue below -->
        <path :d="areaPath" class="area ally" :clip-path="`url(#up-${uid})`" />
        <path :d="areaPath" class="area enemy" :clip-path="`url(#dn-${uid})`" />
        <line :x1="pad.l" :x2="width - pad.r" :y1="zeroY" :y2="zeroY" class="zero" />
        <path :d="linePath" class="line ally" :clip-path="`url(#up-${uid})`" />
        <path :d="linePath" class="line enemy" :clip-path="`url(#dn-${uid})`" />
        <!-- swing moments -->
        <circle v-for="m in marks" :key="m.kind + m.t" :cx="m.cx" :cy="m.cy" :r="m.kind === 'ace' || m.kind === 'baron' ? 5 : 4" class="mark" :class="m.ally ? 'ally' : 'enemy'">
          <title>{{ fmtTime(m.t) }} {{ m.label }}</title>
        </circle>
        <!-- hover crosshair -->
        <template v-if="hover">
          <line :x1="hover.px" :x2="hover.px" :y1="pad.t" :y2="height - pad.b" class="cross" />
          <circle :cx="hover.px" :cy="y(diff(hover.p))" r="4" class="dot" :class="diff(hover.p) >= 0 ? 'ally' : 'enemy'" />
        </template>
      </svg>
      <div v-if="hover" class="gg-tip" :style="{ left: tipLeft + 'px' }">
        <div class="tt">{{ fmtTime(hover.p.t) }} · lead <b>{{ k(diff(hover.p)) }}</b></div>
        <div><i class="sw ally" />You {{ (hover.p.ally / 1000).toFixed(1) }}k · {{ hover.p.ak }} kills</div>
        <div><i class="sw enemy" />Enemy {{ (hover.p.enemy / 1000).toFixed(1) }}k · {{ hover.p.ek }} kills</div>
        <div v-for="m in hover.marks" :key="m.kind + m.t" class="tm">{{ fmtTime(m.t) }} {{ m.label }}</div>
      </div>
    </div>
    <!-- table view for screen readers -->
    <table class="sr-only">
      <caption>Item gold by team over time</caption>
      <tr><th>Time</th><th>Your team</th><th>Enemy</th><th>Lead</th></tr>
      <tr v-for="p in pts" :key="p.t"><td>{{ fmtTime(p.t) }}</td><td>{{ p.ally }}</td><td>{{ p.enemy }}</td><td>{{ diff(p) }}</td></tr>
    </table>
  </div>
</template>

<style scoped>
.gg { --gg-ally: var(--ally, var(--hextech-2)); --gg-enemy: var(--enemy, var(--red)); flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; }
.gg-head { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; margin-bottom: 2px; }
.gg-lead b { font-family: var(--display); font-size: 22px; font-weight: 700; font-variant-numeric: tabular-nums; margin-right: 6px; color: var(--text, inherit); }
.gg-lead .muted, .gg-kills { font-size: 11px; }
.gg-box { position: relative; flex: 1 1 auto; min-height: 110px; }
svg { position: absolute; inset: 0; display: block; overflow: visible; touch-action: none; }
.grid { stroke: var(--line); stroke-width: 1; }
.zero { stroke: var(--dim); stroke-width: 1; }
.ax { fill: var(--dim); font-size: 9px; font-variant-numeric: tabular-nums; }
.side { fill: var(--dim); font-size: 9px; letter-spacing: .12em; }
.area { stroke: none; }
.area.ally { fill: var(--gg-ally); fill-opacity: .12; }
.area.enemy { fill: var(--gg-enemy); fill-opacity: .12; }
.line { fill: none; stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.line.ally { stroke: var(--gg-ally); }
.line.enemy { stroke: var(--gg-enemy); }
.mark { stroke: var(--bg); stroke-width: 2; }
.mark.ally, .dot.ally, .sw.ally { fill: var(--gg-ally); background: var(--gg-ally); }
.mark.enemy, .dot.enemy, .sw.enemy { fill: var(--gg-enemy); background: var(--gg-enemy); }
.dot { stroke: var(--bg); stroke-width: 2; }
.cross { stroke: var(--muted); stroke-width: 1; }
.gg-tip { position: absolute; top: 0; width: 170px; pointer-events: none; font-size: 11px; line-height: 1.45; padding: 6px 8px; background: var(--surface-input, #000); border: 1px solid var(--gold-deep, var(--line)); box-shadow: 0 4px 14px rgba(0, 0, 0, .4); z-index: 2; }
.gg-tip .tt { color: var(--muted); }
.gg-tip .tm { color: var(--muted); border-top: 1px solid var(--line); margin-top: 3px; padding-top: 3px; }
.sw { display: inline-block; width: 8px; height: 8px; border-radius: 2px; margin-right: 5px; vertical-align: 0; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
</style>
