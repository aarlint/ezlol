// Tiny synthesized cues so no audio assets are needed. Gated by a user toggle.
let ctx: AudioContext | null = null
function ac(): AudioContext {
  if (!ctx) ctx = new AudioContext()
  return ctx
}
function tone(freq: number, start: number, dur: number, gain = 0.15, type: OscillatorType = 'sine') {
  const a = ac()
  const o = a.createOscillator()
  const g = a.createGain()
  o.type = type
  o.frequency.value = freq
  g.gain.setValueAtTime(0, a.currentTime + start)
  g.gain.linearRampToValueAtTime(gain, a.currentTime + start + 0.01)
  g.gain.exponentialRampToValueAtTime(0.0001, a.currentTime + start + dur)
  o.connect(g).connect(a.destination)
  o.start(a.currentTime + start)
  o.stop(a.currentTime + start + dur + 0.05)
}
export function enabled(): boolean {
  try {
    return localStorage.getItem('ezlol.sound') !== 'off'
  } catch {
    return true
  }
}
export function setEnabled(on: boolean) {
  try {
    localStorage.setItem('ezlol.sound', on ? 'on' : 'off')
  } catch {
    /* ignore */
  }
}
export function queuePop() {
  if (!enabled()) return
  tone(880, 0, 0.18, 0.2, 'triangle')
  tone(1174, 0.18, 0.18, 0.2, 'triangle')
  tone(1568, 0.36, 0.35, 0.22, 'triangle')
}
export function accepted() {
  if (!enabled()) return
  tone(1318, 0, 0.12, 0.15, 'sine')
  tone(1760, 0.12, 0.25, 0.15, 'sine')
}
export function alert() {
  if (!enabled()) return
  tone(440, 0, 0.1, 0.12, 'square')
  tone(440, 0.15, 0.1, 0.12, 'square')
}
export function notify(title: string, body: string) {
  const bridge = (window as unknown as { ezlol?: { notify?: (t: string, b: string) => Promise<boolean> } }).ezlol
  if (bridge?.notify) {
    if (title) bridge.notify(title, body)
    return
  }
  try {
    if (typeof Notification === 'undefined') return
    if (Notification.permission === 'granted') new Notification(title, { body, silent: true })
    else if (Notification.permission !== 'denied') Notification.requestPermission()
  } catch {
    /* ignore */
  }
}
