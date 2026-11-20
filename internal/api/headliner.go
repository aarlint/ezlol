package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/aarlint/ezlol/internal/screen"
)

// ARAM: Mayhem Season 3 (patch 26.21 on) gives every game one Headliner: a band
// that changes one rule for the match. Nothing in the client or Live Client
// Data APIs names it (gameModeMutators only carries the map skin), so ezlol
// learns it three ways, strongest first:
//
//   - manual: the player picks it in the Headliner box;
//   - inferred: hard evidence in the game (Pentakill's exclusive HEAVY/POWER/
//     THRASH pick on screen, a four-card offer for Maestro, a level above the
//     normal cap for Superfans);
//   - screen: the band's name read on screen early in the game, matched strictly
//     (the whole line, or with "stage"/"headliner" beside it) so a kill
//     announcement, an item name or a skin line does not count.

type headliner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Rule  string `json:"rule"`
	Tip   string `json:"tip"`
	Added string `json:"added"` // patch the band joined Mayhem
	// aliases are normalized spellings matched against OCR lines.
	aliases []string
	// strict names also occur elsewhere on screen (the PENTAKILL announcement,
	// the Heartsteel item): only accept them next to "stage"/"headliner".
	strict bool
	// extra lists additional augment pick levels the band grants.
	extra []int
}

var headliners = []headliner{
	{ID: "heartsteel", Name: "HEARTSTEEL", Added: "26.21", strict: true, aliases: []string{"heartsteel"},
		Rule: "Band members put stickers on some augment offers. Taking a stickered augment adds that member's bonus: Aphelios 2 Stat Anvils, Ezreal bonus AD and AP, Kayn 5% omnivamp, K'Sante +1 level (cap 19), Sett bonus health, Yone attack speed and 20 ability haste.",
		Tip:  "A sticker can settle a close pick. It doesn't make a weak augment worth taking."},
	{ID: "kda", Name: "K/DA", Added: "26.21", aliases: []string{"kda"},
		Rule: "Photocards of Ahri, Akali, Evelynn and Kai'Sa drop every 12 s from 0:30. Collect all four for an extra augment, offered the next time you die.",
		Tip:  "Pick up cards on the way to fights. ezlol checks every death for the bonus pick."},
	{ID: "truedamage", Name: "True Damage", Added: "26.21", aliases: []string{"truedamage"},
		Rule: "Takedowns and destroyed turrets can drop Stat Anvils of a random tier that either team can pick up. At most one Prismatic anvil per team per game.",
		Tip:  "Walk over anvils after a fight before the enemy does."},
	{ID: "pentakill", Name: "Pentakill", Added: "26.21", strict: true, aliases: []string{"pentakill"}, extra: []int{18},
		Rule: "An exclusive pick at game start empowers Q (HEAVY), W (POWER) or E (THRASH): +20 ability haste, +10% damage, +10% healing and shielding. At level 18 a second pick offers the other two plus DOOM (R).",
		Tip:  "Empower the ability you max first. ezlol ranks HEAVY / POWER / THRASH by your skill order."},
	{ID: "cafecuties", Name: "Cafe Cuties", Added: "26.21", aliases: []string{"cafecuties", "cafécuties"},
		Rule: "Health relics heal 20% more missing health and give 25% move speed that decays over 6 s. Each base gets an extra relic.",
		Tip:  "Fight near relics and keep track of when they come back."},
	{ID: "commandline", Name: "Command Line", Added: "26.21", aliases: []string{"commandline"},
		Rule: "Augment offers are Silver or Gold only. Rerolls become Hacked Rerolls, which upgrade one offer a tier (Silver to Gold, Gold to Prismatic).",
		Tip:  "Use the hack on a Gold you would take anyway. It turns into a Prismatic."},
	{ID: "spacegroove", Name: "Space Groove", Added: "26.21", aliases: []string{"spacegroove", "disco"},
		Rule: "A dance floor in the middle of the lane grows a disco ball. When it bursts, every champion on both teams gets up to 1,000 gold.",
		Tip:  "Both teams get the gold. Dance only when nobody can punish it."},
	{ID: "beeband", Name: "The Bee Band", Added: "26.21", aliases: []string{"thebeeband", "beeband"},
		Rule: "Bees and honey fly across the map every few seconds. Bees deal true damage based on max health; honey heals and gives gold.",
		Tip:  "Step into the honey and out of the way of the bees."},
	{ID: "superfans", Name: "Superfans", Added: "26.22", aliases: []string{"superfans"}, extra: []int{21},
		Rule: "Level cap 21 and 60% more experience. An extra augment is offered at level 21, the next time you die.",
		Tip:  "Ultimates and scaling champions get more out of the extra levels. ezlol arms a pick at 21."},
	{ID: "arcade", Name: "Arcade", Added: "26.23", aliases: []string{"arcade", "bitrush"},
		Rule: "A takedown within 3 s of damaging an enemy champion gives a Token (up to 4). With 4 Tokens, dying turns your summoner spells into Revive.",
		Tip:  "Bank all 4 tokens before you take a risky fight."},
	{ID: "debonair", Name: "Debonair", Added: "26.23", aliases: []string{"debonair", "debonairzed", "thedebonairs"},
		Rule: "Every augment offer also grants gold (200 to 700) and Cappa Juice hats.",
		Tip:  "Pick on the augment itself. You get the gold whichever one you take."},
	{ID: "lailusion", Name: "La Ilusión", Added: "26.23", aliases: []string{"lailusión", "lailusion"},
		Rule: "When an ally dies a candle drops near the nearest ally for 10 s. Picking it up shields you (250 + 35% max health) and allies nearby.",
		Tip:  "Grab the candle right after a teammate falls."},
	{ID: "maestro", Name: "Maestro", Added: "26.23", aliases: []string{"maestro"},
		Rule: "Augment picks show four offers instead of three.",
		Tip:  "ezlol reads and ranks all four cards."},
}

func headlinerByID(id string) *headliner {
	for i := range headliners {
		if headliners[i].ID == id {
			return &headliners[i]
		}
	}
	return nil
}

// headlinerInfo is the current game's Headliner as sent to the UI.
type headlinerInfo struct {
	headliner
	Source string `json:"source"` // manual | inferred | screen
}

type headlinerState struct {
	mu     sync.Mutex
	id     string
	source string
	lastGT float64
}

// rank orders evidence: a stronger source may replace a weaker one, never the reverse.
func sourceRank(s string) int {
	switch s {
	case "manual":
		return 3
	case "inferred":
		return 2
	case "screen":
		return 1
	}
	return 0
}

// tick resets the state when a new game starts (game time jumps back).
func (h *headlinerState) tick(gameTime float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if gameTime < h.lastGT-30 {
		h.id, h.source = "", ""
	}
	h.lastGT = gameTime
}

// reset forgets the Headliner (called when no game is running).
func (h *headlinerState) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.id, h.source, h.lastGT = "", "", 0
}

func (h *headlinerState) set(id, source string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if source != "manual" && sourceRank(source) <= sourceRank(h.source) {
		return false
	}
	h.id, h.source = id, source
	if id == "" {
		h.source = ""
	}
	return true
}

func (h *headlinerState) current() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.id
}

func (h *headlinerState) info() *headlinerInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	hl := headlinerByID(h.id)
	if hl == nil {
		return nil
	}
	return &headlinerInfo{headliner: *hl, Source: h.source}
}

// observe looks for the band's name in OCR lines during the first minutes.
func (h *headlinerState) observe(lines []screen.Line, gameTime float64) {
	if gameTime > 180 {
		return
	}
	if id := matchHeadliner(lines); id != "" {
		h.set(id, "screen")
	}
}

// matchHeadliner returns the id of the band named on screen, or "".
func matchHeadliner(lines []screen.Line) string {
	for _, l := range lines {
		if l.Conf < 0.5 {
			continue
		}
		t := screen.Normalize(l.Text)
		if len(t) < 3 || len(t) > 40 {
			continue
		}
		context := strings.Contains(t, "stage") || strings.Contains(t, "headliner")
		for _, hl := range headliners {
			for _, a := range hl.aliases {
				if (t == a && !hl.strict) || (context && strings.Contains(t, a)) {
					return hl.ID
				}
			}
		}
	}
	return ""
}

// inferFromOffer records the Headliner an augment offer proves.
func (h *headlinerState) inferFromOffer(off *augmentOffer) {
	if off == nil || !off.Active {
		return
	}
	for _, a := range off.Offered {
		if a.Special && a.Skill != "" {
			h.set("pentakill", "inferred")
			return
		}
	}
	if len(off.Offered) >= 4 {
		h.set("maestro", "inferred")
	}
}

// inferFromLevels: only Superfans lifts the cap past 19 (HEARTSTEEL's K'Sante sticker gives 19).
func (h *headlinerState) inferFromLevels(players []livePlayer) {
	for _, p := range players {
		if p.Level >= 20 {
			h.set("superfans", "inferred")
			return
		}
	}
}

// extraPickLevels are the pick levels the current Headliner adds.
func (h *headlinerState) extraPickLevels() []int {
	if hl := headlinerByID(h.current()); hl != nil {
		return hl.extra
	}
	return nil
}

func (s *Server) listHeadliners(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"headliners": headliners, "current": s.headliner.info()})
}

// setHeadliner lets the player name the Headliner ({"id": ""} clears it).
func (s *Server) setHeadliner(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if body.ID != "" && headlinerByID(body.ID) == nil {
		writeErr(w, 400, "unknown headliner")
		return
	}
	s.headliner.set(body.ID, "manual")
	writeJSON(w, 200, map[string]any{"current": s.headliner.info()})
}
