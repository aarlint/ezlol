package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/aarlint/ezlol/internal/builds"
	"github.com/aarlint/ezlol/internal/ddragon"
	"github.com/aarlint/ezlol/internal/lcu"
	"github.com/aarlint/ezlol/internal/live"
)

// livePlayer is a scoreboard row with Data Dragon images resolved.
type livePlayer struct {
	Name         string           `json:"name"`
	Champion     ddragon.Champion `json:"champion"`
	Team         string           `json:"team"`
	Position     string           `json:"position"`
	IsDead       bool             `json:"isDead"`
	RespawnTimer float64          `json:"respawnTimer"`
	Level        int              `json:"level"`
	Kills        int              `json:"kills"`
	Deaths       int              `json:"deaths"`
	Assists      int              `json:"assists"`
	CS           int              `json:"cs"`
	Items        []liveItem       `json:"items"`
	Spells       []liveSpell      `json:"spells"`
	Keystone     string           `json:"keystone"`
	IsMe         bool             `json:"isMe"`
	// Rough item-derived stats (Data Dragon flat stats; percent/passives ignored).
	ItemAD    int `json:"itemAD"`
	ItemAP    int `json:"itemAP"`
	ItemArmor int `json:"itemArmor"`
	ItemMR    int `json:"itemMR"`
	ItemHP    int `json:"itemHP"`
}

type liveItem struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Count int      `json:"count"`
	Stats []string `json:"stats,omitempty"` // "+40 AD", "+300 HP", … in display order
}

// Stat bumps an item grants, as short labels, in the order players read them.
// Data Dragon has no ability haste, so haste items show only their other stats.
var statLabels = []struct {
	key     string
	label   string
	percent bool
}{
	{"FlatPhysicalDamageMod", "AD", false},
	{"FlatMagicDamageMod", "AP", false},
	{"FlatHPPoolMod", "HP", false},
	{"FlatMPPoolMod", "Mana", false},
	{"FlatArmorMod", "Armor", false},
	{"FlatSpellBlockMod", "MR", false},
	{"PercentAttackSpeedMod", "AS", true},
	{"FlatCritChanceMod", "Crit", true},
	{"PercentLifeStealMod", "Lifesteal", true},
	{"FlatMovementSpeedMod", "MS", false},
	{"PercentMovementSpeedMod", "MS", true},
	{"FlatHPRegenMod", "HP regen", false},
	{"FlatMPRegenMod", "Mana regen", false},
}

func itemStats(stats map[string]float64) []string {
	var out []string
	for _, sl := range statLabels {
		v, ok := stats[sl.key]
		if !ok || v == 0 {
			continue
		}
		if sl.percent {
			out = append(out, fmt.Sprintf("%+d%% %s", int(math.Round(v*100)), sl.label))
		} else {
			out = append(out, fmt.Sprintf("%+d %s", int(math.Round(v)), sl.label))
		}
	}
	return out
}

type liveSpell struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Image    string  `json:"image"`
	Cooldown float64 `json:"cooldown"`
}

type liveResponse struct {
	InGame     bool               `json:"inGame"`
	GameTime   float64            `json:"gameTime"`
	GameMode   string             `json:"gameMode"`
	MapID      int                `json:"mapId"`
	Me         *live.ActivePlayer `json:"me,omitempty"`
	MyTeam     string             `json:"myTeam"`
	Players    []livePlayer       `json:"players"`
	Hint       string             `json:"hint"`
	Focus      string             `json:"focus"`
	FocusWhy   string             `json:"focusWhy"`
	Offer      *augmentOffer      `json:"offer,omitempty"`
	OCR        string             `json:"ocr"` // available | unavailable | error text
	Objectives *objectives        `json:"objectives,omitempty"`
	Opponent   string             `json:"opponent,omitempty"` // lane opponent champion name
	Arena      *arenaInfo         `json:"arena,omitempty"`
	Events     []live.Event       `json:"events"`
}

func (s *Server) liveGame(w http.ResponseWriter, r *http.Request) {
	gd, err := s.live.Get(r.Context())
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	if gd == nil {
		writeJSON(w, 200, liveResponse{InGame: false})
		return
	}
	d := s.dd.Data()
	resp := liveResponse{InGame: true, GameTime: gd.GameData.GameTime, GameMode: gd.GameData.GameMode, MapID: gd.GameData.MapNumber, Me: &gd.ActivePlayer}
	meName := gd.ActivePlayer.RiotIDName
	if meName == "" {
		meName = gd.ActivePlayer.SummonerName
	}
	for _, p := range gd.AllPlayers {
		lp := livePlayer{
			Name: firstNonEmpty(p.RiotIDName, p.SummonerName), Team: p.Team, IsDead: p.IsDead, RespawnTimer: p.RespawnTimer,
			Position: p.Position, Level: p.Level, Kills: p.Scores.Kills, Deaths: p.Scores.Deaths, Assists: p.Scores.Assists, CS: p.Scores.CreepScore,
			Keystone: p.Runes.Keystone.DisplayName,
		}
		lp.IsMe = lp.Name == meName
		if lp.IsMe {
			resp.MyTeam = p.Team
		}
		lp.Champion = resolveChampion(d, p)
		for _, it := range p.Items {
			li := liveItem{ID: it.ID, Name: it.DisplayName, Count: it.Count}
			if d != nil {
				if dd, ok := d.Items[it.ID]; ok {
					li.Image = dd.Image
					li.Stats = itemStats(dd.Stats)
					n := it.Count
					if n < 1 {
						n = 1
					}
					lp.ItemAD += int(dd.Stats["FlatPhysicalDamageMod"]) * n
					lp.ItemAP += int(dd.Stats["FlatMagicDamageMod"]) * n
					lp.ItemArmor += int(dd.Stats["FlatArmorMod"]) * n
					lp.ItemMR += int(dd.Stats["FlatSpellBlockMod"]) * n
					lp.ItemHP += int(dd.Stats["FlatHPPoolMod"]) * n
				}
			}
			lp.Items = append(lp.Items, li)
		}
		for _, sp := range []live.Spell{p.SummonerSpells.One, p.SummonerSpells.Two} {
			ls := liveSpell{Name: sp.DisplayName}
			if d != nil {
				if dd, ok := d.SpellByName(sp.DisplayName); ok {
					ls.ID, ls.Image, ls.Cooldown = dd.ID, dd.Image, dd.Cooldown
				}
			}
			lp.Spells = append(lp.Spells, ls)
		}
		resp.Players = append(resp.Players, lp)
	}
	if gd.GameData.MapNumber == 30 {
		s.applyArena(r.Context(), &resp, gd)
	}
	resp.Hint = threatHint(resp.Players, resp.MyTeam)
	resp.Focus, resp.FocusWhy = s.focusTarget(r.Context(), resp.Players, resp.MyTeam)
	resp.Offer, resp.OCR = s.detectOffer(r.Context(), &resp)
	if gd.GameData.MapNumber == 11 {
		resp.Objectives = summarizeObjectives(gd, resp.Players, resp.MyTeam)
		resp.Opponent = laneOpponent(resp.Players)
	}
	evs := gd.Events.Events
	if len(evs) > 12 {
		evs = evs[len(evs)-12:]
	}
	resp.Events = evs
	writeJSON(w, 200, resp)
}

// resolveChampion maps the live "rawChampionName" (game_character_displayname_KaiSa)
// onto a Data Dragon champion for images.
func resolveChampion(d *ddragon.Data, p live.Player) ddragon.Champion {
	c := ddragon.Champion{Name: p.ChampionName}
	if d == nil {
		return c
	}
	raw := p.RawChampionName
	if i := strings.LastIndex(raw, "_"); i >= 0 {
		raw = raw[i+1:]
	}
	if found, ok := d.ChampionByKey(raw); ok {
		return found
	}
	// fall back to display-name match (strip punctuation: Kai'Sa -> KaiSa)
	norm := strings.NewReplacer("'", "", " ", "", ".", "", "&", "").Replace(p.ChampionName)
	if found, ok := d.ChampionByKey(norm); ok {
		return found
	}
	for _, ch := range d.Champions {
		if strings.EqualFold(ch.Name, p.ChampionName) {
			return ch
		}
	}
	return c
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type champSelectResponse struct {
	Active           bool               `json:"active"`
	BenchEnabled     bool               `json:"benchEnabled"`
	RerollsRemaining int                `json:"rerollsRemaining"`
	TimeLeft         int                `json:"timeLeft"`
	Phase            string             `json:"phase"`
	Me               *csPlayer          `json:"me,omitempty"`
	MyTeam           []csPlayer         `json:"myTeam"`
	TheirTeam        []ddragon.Champion `json:"theirTeam"`
	Bench            []csBench          `json:"bench"`
	Mode             string             `json:"mode"`
	EnemyProfile     damageProfile      `json:"enemyProfile"`
	AllyProfile      damageProfile      `json:"allyProfile"`
	AllyComp         compSummary        `json:"allyComp"`
	EnemyComp        compSummary        `json:"enemyComp"`
}

type csPlayer struct {
	Name     string           `json:"name"`
	Champion ddragon.Champion `json:"champion"`
	Position string           `json:"position"`
	IsMe     bool             `json:"isMe"`
	CellID   int              `json:"cellId"`
	TradeID  int              `json:"tradeId,omitempty"`
	Trade    string           `json:"trade,omitempty"` // AVAILABLE, SENT, RECEIVED, ...
	Mastery  *lcu.Mastery     `json:"mastery,omitempty"`
	Record   *Record          `json:"record,omitempty"`
}

// csBench is a bench champion with your history on it.
type csBench struct {
	Champion ddragon.Champion `json:"champion"`
	Mastery  *lcu.Mastery     `json:"mastery,omitempty"`
	Record   *Record          `json:"record,omitempty"`
	Score    int              `json:"score"`
	Why      string           `json:"why"`
}

// damageProfile summarises a team's AD vs AP leaning from Riot's champion info.
type damageProfile struct {
	Attack int    `json:"attack"`
	Magic  int    `json:"magic"`
	Hint   string `json:"hint"`
}

func (s *Server) champSelect(w http.ResponseWriter, r *http.Request) {
	c := s.watcher.Client()
	if c == nil {
		writeJSON(w, 200, champSelectResponse{})
		return
	}
	cs, err := c.ChampSelectSession(r.Context())
	if err != nil || cs == nil {
		writeJSON(w, 200, champSelectResponse{})
		return
	}
	d := s.dd.Data()
	champ := func(id int) ddragon.Champion {
		if d != nil {
			if ch, ok := d.Champions[id]; ok {
				return ch
			}
		}
		return ddragon.Champion{ID: id}
	}
	tag := builds.QueueTag(s.watcher.Status().QueueID)
	s.player.refresh(r.Context(), c)
	trades := map[int]lcu.Trade{}
	for _, t := range cs.Trades {
		trades[t.CellID] = t
	}
	resp := champSelectResponse{Active: true, BenchEnabled: cs.BenchEnabled, RerollsRemaining: cs.RerollsRemaining,
		TimeLeft: cs.Timer.AdjustedTimeLeftInPhase / 1000, Phase: cs.Timer.Phase, Mode: tag}
	var allies []ddragon.Champion
	for _, m := range cs.MyTeam {
		id := m.ChampionID
		if id == 0 {
			id = m.ChampionPickIntent
		}
		p := csPlayer{Name: m.GameName, Champion: champ(id), Position: strings.ToUpper(m.AssignedPosition), IsMe: m.CellID == cs.LocalPlayerCellID, CellID: m.CellID}
		if t, ok := trades[m.CellID]; ok {
			p.TradeID, p.Trade = t.ID, t.State
		}
		if p.IsMe {
			p.Mastery, p.Record = s.player.lookup(tag, id)
			me := p
			resp.Me = &me
		}
		if id != 0 {
			allies = append(allies, p.Champion)
		}
		resp.MyTeam = append(resp.MyTeam, p)
	}
	for _, t := range cs.TheirTeam {
		if t.ChampionID != 0 {
			resp.TheirTeam = append(resp.TheirTeam, champ(t.ChampionID))
		}
	}
	for _, b := range cs.BenchChampions {
		row := csBench{Champion: champ(b.ChampionID)}
		row.Mastery, row.Record = s.player.lookup(tag, b.ChampionID)
		resp.Bench = append(resp.Bench, row)
	}
	resp.EnemyProfile = s.profileWithInfo(r.Context(), resp.TheirTeam)
	resp.AllyProfile = s.profileWithInfo(r.Context(), allies)
	resp.AllyComp = s.summarize(r.Context(), allies)
	resp.EnemyComp = s.summarize(r.Context(), resp.TheirTeam)
	s.rankBench(r.Context(), resp.Bench, resp.AllyComp)
	writeJSON(w, 200, resp)
}

func (s *Server) champSelectSwap(w http.ResponseWriter, r *http.Request) {
	c := s.watcher.Client()
	id, err := strconv.Atoi(r.PathValue("id"))
	if c == nil || err != nil {
		writeErr(w, 400, "not connected or bad id")
		return
	}
	if err := c.SwapWithBench(r.Context(), id); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) champSelectReroll(w http.ResponseWriter, r *http.Request) {
	c := s.watcher.Client()
	if c == nil {
		writeErr(w, 400, "not connected")
		return
	}
	if err := c.Reroll(r.Context()); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// threatHint summarises what the enemy team is building so you know which
// resistance or penetration to buy next.
func threatHint(players []livePlayer, myTeam string) string {
	var ad, ap, armor, mr int
	for _, p := range players {
		if p.Team == myTeam {
			continue
		}
		ad += p.ItemAD
		ap += p.ItemAP
		armor += p.ItemArmor
		mr += p.ItemMR
	}
	if ad+ap+armor+mr == 0 {
		return ""
	}
	var parts []string
	switch {
	case ap > 0 && ap >= ad*2:
		parts = append(parts, "enemy items lean AP — magic resist")
	case ad > 0 && ad >= ap*2:
		parts = append(parts, "enemy items lean AD — armor")
	case ad > 0 || ap > 0:
		parts = append(parts, "enemy damage is mixed")
	}
	switch {
	case armor >= 150 && armor >= mr*2:
		parts = append(parts, "they are stacking armor — armor pen / % pen")
	case mr >= 150 && mr >= armor*2:
		parts = append(parts, "they are stacking MR — magic pen")
	}
	return strings.Join(parts, " · ")
}

func (s *Server) tradeRequest(w http.ResponseWriter, r *http.Request) {
	c := s.watcher.Client()
	id, err := strconv.Atoi(r.PathValue("id"))
	if c == nil || err != nil {
		writeErr(w, 400, "not connected or bad id")
		return
	}
	var call func(context.Context, int) error = c.RequestTrade
	if r.URL.Query().Get("accept") == "1" {
		call = c.AcceptTrade
	}
	if err := call(r.Context(), id); err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// focusTarget picks the squishiest high-damage enemy: the one to kill first in a fight.
func (s *Server) focusTarget(ctx context.Context, players []livePlayer, myTeam string) (string, string) {
	return s.focus(ctx, players, myTeam, true)
}

// focusTargetNoInfo is focusTarget without client tactical info (tests).
func (s *Server) focusTargetNoInfo(players []livePlayer, myTeam string) (string, string) {
	return s.focus(context.Background(), players, myTeam, false)
}

func (s *Server) focus(ctx context.Context, players []livePlayer, myTeam string, useInfo bool) (string, string) {
	var c *lcu.Client
	if useInfo && s.watcher != nil {
		c = s.watcher.Client()
	}
	best, bestScore := "", -1e9
	why := ""
	for _, p := range players {
		if p.Team == myTeam || p.Champion.ID == 0 {
			continue
		}
		var info *lcu.ChampionInfo
		if c != nil {
			info = s.info.get(ctx, c, p.Champion.ID)
		}
		dmg := float64(p.ItemAD)/40 + float64(p.ItemAP)/60
		tough := float64(p.ItemArmor)/40 + float64(p.ItemMR)/40 + float64(p.ItemHP)/300
		if info != nil {
			dmg += float64(info.PlaystyleInfo.Damage) * 1.5
			tough += float64(info.PlaystyleInfo.Durability) * 1.5
		}
		score := dmg - tough + float64(p.Kills)*0.3
		if score > bestScore {
			bestScore, best = score, p.Champion.Name
			switch {
			case p.Kills >= 5 && tough < dmg:
				why = "fed and squishy"
			case tough < 2:
				why = "no defensive items"
			default:
				why = "highest damage-to-toughness"
			}
		}
	}
	return best, why
}
