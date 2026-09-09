# ezlol — todo

## Done
- [x] Go backend: LCU discovery, watcher loop, auto-accept, SSE, persisted settings
- [x] Data Dragon (champions w/ damage info, items w/ stats, runes, spells w/ cooldowns, per-champion abilities)
- [x] Build compiler from Riot match-v5 (ranked + ARAM queues, needs RIOT_API_KEY)
- [x] LCU recommended runes (Rift map 11 / ARAM map 12), Apply to client, Auto runes in champ select
- [x] Vue 3 frontend, hextech theme, Vite HMR dev mode
- [x] Electron shell: overlay mode, auto-overlay, Cmd+Shift+E, native notifications + dock bounce, own userData dir
- [x] Champ select: teams, damage profile, comp analysis, ranked bench + reroll hint, swap/reroll/trade buttons
- [x] Live game: scoreboard, item stats, threat tip, focus target, spell tracker (persisted), death timers, enemy-back list, numbers banner, ability CDs, item purchase feed, kill feed w/ own highlights, multikill sound
- [x] Post-game scoreboard with damage/KP/taken/heal shares
- [x] Mastery + recent record per champion; mastery badges + sort in picker; today's W/L + streak
- [x] Sound cues, trade/timer alerts, respawn ping

- [x] Community ARAM builds (op.gg JSON, no key) + Mayhem augments per champion (aramgg.com) with CDragon icons, blitz descriptions; one-page build view
- [x] Trades: client route is /session/champion-swaps; session carries "trades"

## Verified live (2026-09-05, ARAM Mayhem)
- auto-accept (2 pops, ~100 ms), live scoreboard, ARAM runes, rune apply, post-game block, session tally, shopping feed, focus target

- [x] Widget dashboard (gridstack, per-screen layouts, edit/reset), themes ×6, toasts, Settings dialog, updates (Win in-place, mac swap updater), Rift mode, screenshots — released v1.0.4–v1.0.6

- [x] ARAM augments by win rate, ghost slots, Save layout/Reset, symmetrical 3|6|3 defaults per screen (PRs #10–#13, unreleased on main)

- [x] Champion card is a fixed bar above the grid (ChampionBar + shared build store); screen change resets forced mode/lane; layout store keys v3; README screenshots retaken

- [x] Champion picker is a dropdown card on the champion bar (portrait/name opens it; Esc / outside click closes; Enter picks first match)

- [x] Save layout / Reset restore exact positions (double placement on remount fixed; gridstack child adoption off)

- [x] Desktop settings: Launch at login + Start in the tray / menu bar (tray menu, hide-to-tray on close, single instance); Settings button is a gear icon

- [x] Champ select is one fixed strip under the champion bar (team / enemies / bench), only during champion select; select grid holds the build only

- [x] Unsigned mac builds are ad-hoc signed by an afterSign hook (Gatekeeper "damaged" fix); installed 1.0.10 repaired in place

## Next
- [ ] Developer ID signing + notarization: create the Developer ID Application cert (Xcode → Settings → Accounts → Manage Certificates), export .p12, set the five repo secrets per docs/SIGNING.md
- [ ] Verify Launch at login on a packaged build (login item only registers when app.isPackaged)
- [x] v1.0.8, v1.0.9 (2026-09-07) and v1.0.10 (2026-09-08) released, all jobs green
- [x] Trade buttons verified in ARAM champ select (AVAILABLE + ids)
- [ ] Verify Auto runes + reroll hint in next champ select
- [ ] Verify post-game share badges at next game end
- [ ] Riot key -> run ranked compile (ARAM now covered by op.gg without a key)
- [x] ezlol.app repackaged with augment build view (2026-09-05 22:47)
- [ ] Windows lockfile path only covers default install
