<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import type { SettingsView, UpdateInfo } from '../types'
import type { AppSettings } from '../api'
import * as sound from '../sound'

const emit = defineEmits<{ close: [] }>()

const s = ref<SettingsView | null>(null)
const upd = ref<UpdateInfo | null>(null)
const key = ref('')
const msg = ref('')
const msgKind = ref<'ok' | 'err'>('ok')
const busy = ref(false)
const loadError = ref('')
const soundOn = ref(sound.enabled())
const autoRunes = ref(localStorage.getItem('ezlol.autoRunes') === 'on')
const dialog = ref<HTMLElement | null>(null)
// Desktop-only settings live in the Electron shell; absent in a plain browser.
const desktop = !!window.ezlol?.getAppSettings
const appS = ref<AppSettings | null>(null)
async function saveApp(patch: Partial<Pick<AppSettings, 'launchAtLogin' | 'startInTray'>>) {
  if (!window.ezlol?.setAppSettings) return
  busy.value = true
  try {
    appS.value = await window.ezlol.setAppSettings(patch)
    flash('Saved', 'ok')
  } catch (e) {
    flash(errText(e), 'err')
  } finally {
    busy.value = false
  }
}
if (desktop) window.ezlol?.onAppSettings?.((v) => (appS.value = v))
let msgTimer: ReturnType<typeof setTimeout> | undefined
let pressOnBackdrop = false

/** Badge, description and tint for the Version row, derived together so they never disagree. */
const updateState = computed(() => {
  const u = upd.value
  if (!u) return { badge: 'checking…', cls: '', desc: 'Checking release status…', descCls: '' }
  if (u.hasUpdate) return { badge: `${u.latest} available`, cls: 'riot', desc: `You have ${u.current}; ${u.latest} is ready to download.`, descCls: 'ok' }
  if (u.error) return { badge: 'check failed', cls: '', desc: `Check failed: ${u.error}`, descCls: 'warn' }
  return { badge: 'up to date', cls: 'lcu', desc: `You have ${u.current}, the latest release.`, descCls: '' }
})

function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function flash(text: string, kind: 'ok' | 'err') {
  msg.value = text
  msgKind.value = kind
  clearTimeout(msgTimer)
  msgTimer = setTimeout(() => (msg.value = ''), 2500)
}

async function load() {
  loadError.value = ''
  let view: SettingsView
  try {
    view = await api.settings()
  } catch (e) {
    loadError.value = errText(e)
    return
  }
  s.value = view
  if (desktop) window.ezlol?.getAppSettings?.().then((v) => (appS.value = v)).catch(() => (appS.value = null))
  try {
    upd.value = await api.update()
  } catch (e) {
    // Surface the failure on the Version row rather than leaving it on "checking…".
    upd.value = { current: view.version, latest: '', hasUpdate: false, url: '', notes: '', checkedAt: '', error: errText(e) }
  }
}

/** Persist a partial settings patch; resolves true on success. */
async function save(patch: Record<string, unknown>): Promise<boolean> {
  busy.value = true
  try {
    s.value = await api.saveSettings(patch)
    flash('Saved', 'ok')
    return true
  } catch (e) {
    flash(errText(e), 'err')
    return false
  } finally {
    busy.value = false
  }
}

async function saveKey() {
  if (busy.value || !key.value) return
  if (await save({ riotApiKey: key.value })) key.value = ''
}

function clearKey() {
  return save({ riotApiKey: '' })
}

async function check() {
  busy.value = true
  try {
    window.ezlol?.checkUpdate?.()
    upd.value = await api.checkUpdate()
  } catch (e) {
    flash(errText(e), 'err')
  } finally {
    busy.value = false
  }
}

function download() {
  if (upd.value?.url) window.open(upd.value.url, '_blank', 'noopener,noreferrer')
}

function toggleSound() {
  soundOn.value = !soundOn.value
  sound.setEnabled(soundOn.value)
}

function toggleAutoRunes() {
  autoRunes.value = !autoRunes.value
  localStorage.setItem('ezlol.autoRunes', autoRunes.value ? 'on' : 'off')
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    emit('close')
  }
}

// Close only when the press started on the backdrop too, so a text drag that
// ends outside the dialog (selecting in the key input) does not dismiss it.
function onBackdropDown(e: PointerEvent) {
  pressOnBackdrop = e.target === e.currentTarget
}
function onBackdropClick(e: MouseEvent) {
  if (pressOnBackdrop && e.target === e.currentTarget) emit('close')
}

onMounted(() => {
  document.addEventListener('keydown', onKey)
  dialog.value?.focus()
  void load()
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  clearTimeout(msgTimer)
})
</script>

<template>
  <div class="backdrop" @pointerdown="onBackdropDown" @click="onBackdropClick">
    <section ref="dialog" class="panel modal" role="dialog" aria-modal="true" aria-labelledby="set-title" tabindex="-1">
      <h2>
        <span id="set-title">Settings</span>
        <span class="ver">ezlol {{ s?.version ?? '…' }}</span>
        <span class="status" :class="msgKind" aria-live="polite" :title="msg || undefined">{{ msg }}</span>
        <button type="button" class="x" aria-label="Close settings" @click="emit('close')">×</button>
      </h2>

      <div class="body">
        <template v-if="s">
          <!-- Riot API -->
          <h3>Riot API</h3>
          <p class="muted lead">
            Only needed for Summoner's Rift builds, which are compiled from ranked matches; ARAM builds, augments and runes work without it.
            Get a key at <a href="https://developer.riotgames.com" target="_blank" rel="noreferrer">developer.riotgames.com</a>.
            Stored locally in <code>settings.json</code> (mode 0600), never logged or sent anywhere but Riot.
          </p>
          <dl class="rows">
            <div class="setting wide">
              <dt>
                <span class="lbl">
                  <label for="set-key">API key</label>
                  <span class="badge" :class="{ riot: s.riotApiKeySet }">
                    <template v-if="s.riotApiKeySet">key set <span class="hint">{{ s.riotApiKeyHint }}</span></template>
                    <template v-else>no key</template>
                  </span>
                </span>
                <span class="desc muted">Dev keys expire daily; paste a new one here.</span>
              </dt>
              <dd>
                <input id="set-key" v-model="key" type="password" class="key" placeholder="RGAPI-…" autocomplete="off" spellcheck="false" @keydown.enter.prevent="saveKey" />
                <button type="button" class="primary" :disabled="busy || !key" @click="saveKey">Save key</button>
                <button type="button" class="danger" :disabled="busy || !s.riotApiKeySet" @click="clearKey">Clear</button>
              </dd>
            </div>

            <div class="setting">
              <dt>
                <span class="lbl"><label for="set-platform">Platform</label></span>
                <span class="desc muted">Riot platform routing used for match history: na1, euw1, kr…</span>
              </dt>
              <dd>
                <input id="set-platform" v-model.trim="s.platform" class="short" maxlength="5" spellcheck="false" autocapitalize="off" autocomplete="off" @change="save({ platform: s.platform })" />
              </dd>
            </div>

            <div class="setting">
              <dt>
                <span class="lbl"><label for="set-matches">Matches per compile</label></span>
                <span class="desc muted">Ranked matches sampled on each compile run (10 – 5000).</span>
              </dt>
              <dd>
                <input id="set-matches" v-model.number="s.matchesPerRun" type="number" class="short" min="10" max="5000" step="10" @change="save({ matchesPerRun: s.matchesPerRun })" />
              </dd>
            </div>

            <div class="setting">
              <dt>
                <span class="lbl"><label id="set-lbl-compile" for="set-compile">Compile on start</label></span>
                <span class="desc muted">Refresh Summoner's Rift builds when the app launches.</span>
              </dt>
              <dd>
                <button id="set-compile" type="button" role="switch" class="toggle" :class="{ on: s.autoCompile }" :aria-checked="s.autoCompile" aria-labelledby="set-lbl-compile" @click="save({ autoCompile: !s.autoCompile })">
                  <span class="track" aria-hidden="true" /><span class="state">{{ s.autoCompile ? 'On' : 'Off' }}</span>
                </button>
              </dd>
            </div>
          </dl>

          <!-- Queue -->
          <h3>Queue</h3>
          <dl class="rows">
            <div class="setting">
              <dt>
                <span class="lbl"><label id="set-lbl-accept" for="set-accept">Auto-accept queue</label></span>
                <span class="desc muted">Accept the ready check the moment it pops.</span>
              </dt>
              <dd>
                <button id="set-accept" type="button" role="switch" class="toggle" :class="{ on: s.autoAccept }" :aria-checked="s.autoAccept" aria-labelledby="set-lbl-accept" @click="save({ autoAccept: !s.autoAccept })">
                  <span class="track" aria-hidden="true" /><span class="state">{{ s.autoAccept ? 'On' : 'Off' }}</span>
                </button>
              </dd>
            </div>

            <div class="setting">
              <dt>
                <span class="lbl"><label id="set-lbl-runes" for="set-runes">Auto-apply runes</label></span>
                <span class="desc muted">Push the recommended rune page to the client during champion select.</span>
              </dt>
              <dd>
                <button id="set-runes" type="button" role="switch" class="toggle" :class="{ on: autoRunes }" :aria-checked="autoRunes" aria-labelledby="set-lbl-runes" @click="toggleAutoRunes">
                  <span class="track" aria-hidden="true" /><span class="state">{{ autoRunes ? 'On' : 'Off' }}</span>
                </button>
              </dd>
            </div>
          </dl>

          <!-- In-game -->
          <h3>In-game</h3>
          <dl class="rows">
            <div class="setting">
              <dt>
                <span class="lbl"><label id="set-lbl-ocr" for="set-ocr">Augment pick detection</label></span>
                <span v-if="s.ocrAvailable" class="desc muted">Read augment offers off the screen (OCR) and rank them live.</span>
                <span v-else class="desc warn">The OCR helper is not available on this platform.</span>
              </dt>
              <dd>
                <button id="set-ocr" type="button" role="switch" class="toggle" :class="{ on: s.ocr && s.ocrAvailable }" :aria-checked="s.ocr && s.ocrAvailable" aria-labelledby="set-lbl-ocr" :disabled="!s.ocrAvailable" :title="s.ocrAvailable ? undefined : 'OCR helper not available on this platform'" @click="save({ ocr: !s.ocr })">
                  <span class="track" aria-hidden="true" /><span class="state">{{ s.ocr && s.ocrAvailable ? 'On' : 'Off' }}</span>
                </button>
              </dd>
            </div>
          </dl>

          <!-- App -->
          <h3>App</h3>
          <dl class="rows">
            <div class="setting">
              <dt>
                <span class="lbl">Version <span class="badge" :class="updateState.cls">{{ updateState.badge }}</span></span>
                <span class="desc muted" :class="updateState.descCls">{{ updateState.desc }}</span>
              </dt>
              <dd>
                <button type="button" :disabled="busy" @click="check">Check now</button>
                <button v-if="upd?.hasUpdate" type="button" class="primary" @click="download">Download</button>
              </dd>
            </div>

            <div class="setting">
              <dt>
                <span class="lbl"><label id="set-lbl-updates" for="set-updates">Check for updates</label></span>
                <span class="desc muted">Look for a new release each time the app starts.</span>
              </dt>
              <dd>
                <button id="set-updates" type="button" role="switch" class="toggle" :class="{ on: s.updateCheck }" :aria-checked="s.updateCheck" aria-labelledby="set-lbl-updates" @click="save({ updateCheck: !s.updateCheck })">
                  <span class="track" aria-hidden="true" /><span class="state">{{ s.updateCheck ? 'On' : 'Off' }}</span>
                </button>
              </dd>
            </div>

            <div class="setting">
              <dt>
                <span class="lbl"><label id="set-lbl-sound" for="set-sound">Sounds</label></span>
                <span class="desc muted">Play cues for queue pop and accept.</span>
              </dt>
              <dd>
                <button id="set-sound" type="button" role="switch" class="toggle" :class="{ on: soundOn }" :aria-checked="soundOn" aria-labelledby="set-lbl-sound" @click="toggleSound">
                  <span class="track" aria-hidden="true" /><span class="state">{{ soundOn ? 'On' : 'Off' }}</span>
                </button>
              </dd>
            </div>
          </dl>

          <!-- Desktop app (Electron only) -->
          <template v-if="desktop && appS">
            <h3>Desktop</h3>
            <dl class="rows">
              <div class="setting">
                <dt>
                  <span class="lbl"><label id="set-lbl-login" for="set-login">Launch at login</label></span>
                  <span v-if="appS.loginItemSupported" class="desc muted">Start ezlol when you sign in, so queue pops are accepted before you even open it.</span>
                  <span v-else class="desc warn">Available in the installed app only.</span>
                </dt>
                <dd>
                  <button id="set-login" type="button" role="switch" class="toggle" :class="{ on: appS.launchAtLogin }" :aria-checked="appS.launchAtLogin" aria-labelledby="set-lbl-login" :disabled="!appS.loginItemSupported || busy" @click="saveApp({ launchAtLogin: !appS.launchAtLogin })">
                    <span class="track" aria-hidden="true" /><span class="state">{{ appS.launchAtLogin ? 'On' : 'Off' }}</span>
                  </button>
                </dd>
              </div>

              <div class="setting">
                <dt>
                  <span class="lbl"><label id="set-lbl-tray" for="set-tray">{{ appS.platform === 'darwin' ? 'Start in the menu bar' : 'Start in the tray' }}</label></span>
                  <span class="desc muted">Start hidden behind a {{ appS.platform === 'darwin' ? 'menu bar' : 'tray' }} icon; closing the window keeps ezlol running there.</span>
                </dt>
                <dd>
                  <button id="set-tray" type="button" role="switch" class="toggle" :class="{ on: appS.startInTray }" :aria-checked="appS.startInTray" aria-labelledby="set-lbl-tray" :disabled="busy" @click="saveApp({ startInTray: !appS.startInTray })">
                    <span class="track" aria-hidden="true" /><span class="state">{{ appS.startInTray ? 'On' : 'Off' }}</span>
                  </button>
                </dd>
              </div>
            </dl>
          </template>
        </template>

        <div v-else-if="loadError" class="empty">
          <span class="desc warn">Could not load settings: {{ loadError }}</span>
          <button type="button" @click="load">Retry</button>
        </div>
        <p v-else class="muted lead">Loading settings…</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
/*
 * Row layout only. Colours, fonts, borders and control looks all come from the
 * global .panel / .toggle / .badge / button / input rules in style.css.
 * One spacing scale (--sp-*) and one control height (--ctl-h) drive everything.
 */
.backdrop { position: fixed; inset: 0; z-index: 100; display: grid; place-items: center; background: rgba(0, 0, 0, .6); }

.modal {
  --sp-1: 4px;
  --sp-2: 8px;
  --sp-3: 12px;
  --sp-4: 16px;
  --ctl-h: 36px;
  --pad: 18px; /* matches .panel padding so the body can bleed to the frame edge */
  display: flex;
  flex-direction: column;
  width: min(720px, 94vw);
  max-height: 85vh;
  outline: none;
}

/* Header: title · version · transient status · close, on one line */
.modal h2 { display: flex; align-items: center; gap: var(--sp-3); flex: none; }
.modal h2 .ver,
.modal h2 .status { font-family: var(--serif); font-size: 12px; font-weight: 400; letter-spacing: 0; text-transform: none; color: var(--muted); }
.modal h2 .status { margin-left: auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.modal h2 .status.ok { color: var(--hextech-2); }
.modal h2 .status.err { color: var(--red); }
.modal h2 .x { flex: none; width: var(--ctl-h); height: var(--ctl-h); padding: 0; display: inline-grid; place-items: center; font-size: 18px; line-height: 1; }

/* Body scrolls inside the frame, bleeding to the side and bottom edges so the scrollbar runs the full panel height */
.body { flex: 1 1 auto; min-height: 0; overflow-y: auto; margin: 0 calc(var(--pad) * -1) calc(var(--pad) * -1); padding: 0 var(--pad) var(--pad); }
.body h3 { margin: var(--sp-4) 0 var(--sp-1); }
.body h3:first-child { margin-top: var(--sp-1); }
.lead { margin: 0 0 var(--sp-1); line-height: 1.45; }
.empty { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-4); padding: var(--sp-2) 0; }

/* Definition-list rows: [label + description | control] with hairline dividers */
.rows { margin: 0; }
.setting {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--sp-4);
  padding: var(--sp-2) 0;
  border-top: 1px solid var(--line);
}
.setting:first-child { border-top: 0; }
/* The key row is the one with a real text input: give its controls a fluid share instead of a fixed width */
.setting.wide { grid-template-columns: minmax(0, 1fr) minmax(0, 55%); }
.setting dt { min-width: 0; display: grid; gap: 2px; }
.setting dd { margin: 0; min-width: 0; display: flex; align-items: center; justify-content: flex-end; gap: var(--sp-2); }
.lbl { display: inline-flex; align-items: center; gap: var(--sp-2); font-size: 13px; font-weight: 600; color: var(--text); white-space: nowrap; }
.lbl label { cursor: pointer; }
.lbl .hint { font-family: ui-monospace, Menlo, monospace; text-transform: none; letter-spacing: 0; }
.desc { display: block; font-size: 12px; line-height: 1.4; overflow-wrap: anywhere; }
.desc.ok { color: var(--hextech-2); }
.desc.warn { color: var(--amber); }

/* Every control shares one height so each row reads as a single line */
.setting input { height: var(--ctl-h); padding: 0 var(--sp-3); }
.setting .short { flex: none; width: 96px; }
.setting .key { flex: 1 1 120px; min-width: 0; width: auto; }
.body button { flex: none; height: var(--ctl-h); padding: 0 var(--sp-3); display: inline-flex; align-items: center; white-space: nowrap; }
.body button:focus-visible,
.modal h2 .x:focus-visible { outline: 1px solid var(--gold); outline-offset: 1px; }

/* Switch: a real button (tab-focusable, Space/Enter, native :disabled) wearing the global .toggle track.
   The generic button chrome is stripped so only the .toggle look remains. */
.setting .toggle { padding: 0 var(--sp-2); min-height: 0; background: none; box-shadow: none; border-color: transparent; color: var(--muted); }
.setting .toggle.on { color: var(--gold-bright); }
.setting .toggle:hover:not(:disabled) { border-color: var(--gold-deep); background: var(--gold-soft); box-shadow: none; }
.setting .toggle:disabled { cursor: not-allowed; }
.setting .toggle .state { min-width: 2.6em; text-align: left; }

/* Narrow windows: stack the control under its label rather than squeeze either */
@media (max-width: 640px) {
  .setting,
  .setting.wide { grid-template-columns: 1fr; }
  .setting dd { justify-content: flex-start; }
}
</style>
