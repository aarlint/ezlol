import type { Directive } from 'vue'

// v-autoscroll: keeps a chronological list pinned to its newest entry (bottom)
// whenever content changes, unless the user has deliberately scrolled up.
const STICK = 40
const observers = new WeakMap<HTMLElement, MutationObserver>()

export const autoscroll: Directive<HTMLElement> = {
  mounted(el) {
    let stick = true
    el.addEventListener('scroll', () => {
      stick = el.scrollHeight - el.scrollTop - el.clientHeight < STICK
    })
    const mo = new MutationObserver(() => {
      if (stick) requestAnimationFrame(() => (el.scrollTop = el.scrollHeight))
    })
    mo.observe(el, { childList: true, subtree: true, characterData: true })
    observers.set(el, mo)
    requestAnimationFrame(() => (el.scrollTop = el.scrollHeight))
  },
  unmounted(el) {
    observers.get(el)?.disconnect()
    observers.delete(el)
  },
}
