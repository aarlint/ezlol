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

## Next
- [x] Trade buttons verified in ARAM champ select (AVAILABLE + ids)
- [ ] Verify Auto runes + reroll hint in next champ select
- [ ] Verify post-game share badges at next game end
- [ ] Riot key -> run ranked compile (ARAM now covered by op.gg without a key)
- [x] ezlol.app repackaged with augment build view (2026-09-05 22:47)
- [ ] Windows lockfile path only covers default install
