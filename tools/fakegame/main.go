// Command fakegame serves a simulated League match on the Live Client Data API
// (https://127.0.0.1:2999/liveclientdata/allgamedata) so the live screen can be
// built and tested without a real game: ten players, kills, deaths, multikills,
// aces, item purchases with real Data Dragon recipes, objectives on the Rift.
//
//	go run ./tools/fakegame -speed 8            # ARAM: Mayhem, 8 game seconds per second
//	go run ./tools/fakegame -mode rift -champ Ahri
//
// Run ezlol with -no-open and open http://127.0.0.1:7331/?mode=game. With ezlol
// running, your champion follows the build ezlol shows (-ezlol), so the Next buy
// box credits the right components.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aarlint/ezlol/internal/ddragon"
)

type player struct {
	name, champ, key, team, position, role string
	path                                   []int
	spells                                 [2]string
}

type event struct {
	ID         int      `json:"EventID"`
	Name       string   `json:"EventName"`
	Time       float64  `json:"EventTime"`
	KillerName string   `json:"KillerName,omitempty"`
	VictimName string   `json:"VictimName,omitempty"`
	Assisters  []string `json:"Assisters,omitempty"`
	KillStreak int      `json:"KillStreak,omitempty"`
	AcingTeam  string   `json:"AcingTeam,omitempty"`
	DragonType string   `json:"DragonType,omitempty"`
	Stolen     string   `json:"Stolen,omitempty"`
	Recipient  string   `json:"Recipient,omitempty"`
}

type game struct {
	d       *ddragon.Data
	players []*player
	events  []event
	aram    bool
	length  float64
	start   time.Time
	speed   float64
	offset  float64
}

var (
	ally  = []string{"Jinx", "Lux", "Malphite", "Yasuo", "Nami"}
	enemy = []string{"Zed", "Ahri", "Darius", "Ezreal", "Leona"}
	// Completed items per role; ids missing from the current patch are skipped
	// and the path is topped up from Data Dragon by the role's main stat.
	paths = map[string][]int{
		"adc":     {3006, 3031, 3046, 3094, 3072, 3036},
		"mage":    {3020, 6655, 3089, 3135, 3157, 4645},
		"tank":    {3047, 3068, 3075, 3065, 3143, 3742},
		"bruiser": {3158, 6692, 3071, 6694, 3814, 3026},
		"support": {3009, 3504, 6617, 3222, 3107, 2065},
	}
	roleStat = map[string]string{"adc": "FlatCritChanceMod", "mage": "FlatMagicDamageMod", "tank": "FlatHPPoolMod", "bruiser": "FlatPhysicalDamageMod", "support": "FlatMPRegenMod"}
	rift     = []string{"BOTTOM", "MIDDLE", "TOP", "JUNGLE", "UTILITY"}
)

func roleOf(c ddragon.Champion) string {
	for _, t := range c.Tags {
		switch t {
		case "Marksman":
			return "adc"
		case "Mage":
			return "mage"
		case "Tank":
			return "tank"
		case "Support":
			return "support"
		}
	}
	return "bruiser"
}

func main() {
	addr := flag.String("addr", "127.0.0.1:2999", "listen address")
	mode := flag.String("mode", "aram", "aram (ARAM: Mayhem) or rift")
	speed := flag.Float64("speed", 6, "game seconds per real second")
	from := flag.Float64("start", 0, "start at this game time (seconds)")
	length := flag.Float64("length", 1500, "game length in seconds; the clock stops there")
	me := flag.String("champ", "Jinx", "your champion (Data Dragon key)")
	ez := flag.String("ezlol", "http://127.0.0.1:7331", "running ezlol, to follow its build for your champion ('' to skip)")
	seed := flag.Uint64("seed", 7, "random seed")
	flag.Parse()

	cache, _ := os.UserCacheDir()
	store := ddragon.New(filepath.Join(cache, "ezlol-fakegame", "ddragon"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := store.Load(ctx); err != nil {
		log.Fatalf("data dragon: %v", err)
	}
	g := &game{d: store.Data(), aram: *mode != "rift", length: *length, start: time.Now(), speed: *speed, offset: *from}
	rng := mrand.New(mrand.NewPCG(*seed, *seed^0x5eed))
	allies := append([]string{*me}, ally...)
	for i, names := range [][]string{allies, enemy} {
		team := []string{"ORDER", "CHAOS"}[i]
		count := 0
		for _, key := range names {
			if count == 5 || (i == 0 && count > 0 && key == *me) {
				continue
			}
			c, ok := g.d.ChampionByKey(key)
			if !ok {
				log.Fatalf("unknown champion %q", key)
			}
			p := &player{champ: c.Name, key: c.Key, team: team, role: roleOf(c), spells: [2]string{"Flash", "Mark"}}
			p.name = fmt.Sprintf("%s Fan", c.Name)
			if i == 0 && count == 0 {
				p.name = "Tester"
			}
			if !g.aram {
				p.position = rift[count]
				p.spells[1] = map[string]string{"JUNGLE": "Smite", "UTILITY": "Exhaust", "TOP": "Teleport", "MIDDLE": "Ignite", "BOTTOM": "Heal"}[p.position]
			}
			p.path = g.rolePath(p.role)
			if p.name == "Tester" && *ez != "" {
				if bp := buildPath(*ez, c.Key, map[bool]string{true: "aram", false: "sr"}[g.aram]); len(bp) > 0 {
					p.path = bp
					log.Printf("your %s follows ezlol's build: %v", c.Name, bp)
				}
			}
			g.players = append(g.players, p)
			count++
		}
	}
	g.script(rng)

	srv := &http.Server{Addr: *addr, TLSConfig: &tls.Config{Certificates: []tls.Certificate{selfSigned()}}}
	http.HandleFunc("/liveclientdata/allgamedata", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(g.snapshot(g.now()))
	})
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("fake %s game on https://%s/liveclientdata/allgamedata (%.0fx)", *mode, *addr, *speed)
	log.Fatal(srv.ServeTLS(ln, "", ""))
}

func (g *game) now() float64 {
	return math.Min(g.length, g.offset+time.Since(g.start).Seconds()*g.speed)
}

// rolePath keeps the role's items that exist this patch and tops it up to six.
func (g *game) rolePath(role string) []int {
	var out []int
	seen := map[int]bool{}
	for _, id := range paths[role] {
		if _, ok := g.d.Items[id]; ok {
			out = append(out, id)
			seen[id] = true
		}
	}
	var pool []ddragon.Item
	for id, it := range g.d.Items {
		if g.d.IsCompleted(id) && it.Maps["11"] && it.Stats[roleStat[role]] > 0 && !seen[id] {
			pool = append(pool, it)
		}
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].ID < pool[j].ID })
	for _, it := range pool {
		if len(out) >= 6 {
			break
		}
		out = append(out, it.ID)
	}
	return out
}

// buildPath asks a running ezlol for the build it shows (core, boots, late).
func buildPath(base, key, mode string) []int {
	c := &http.Client{Timeout: 20 * time.Second}
	res, err := c.Get(fmt.Sprintf("%s/api/champions/%s/build?mode=%s", strings.TrimRight(base, "/"), key, mode))
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	var b struct {
		Core, Boots, Late []struct {
			Items []struct {
				ID int `json:"id"`
			} `json:"items"`
		}
	}
	if json.NewDecoder(res.Body).Decode(&b) != nil {
		return nil
	}
	var out []int
	add := func(id int) {
		for _, x := range out {
			if x == id {
				return
			}
		}
		out = append(out, id)
	}
	if len(b.Core) > 0 {
		for _, it := range b.Core[0].Items {
			add(it.ID)
		}
	}
	if len(b.Boots) > 0 && len(b.Boots[0].Items) > 0 {
		add(b.Boots[0].Items[0].ID)
	}
	for i, s := range b.Late {
		if i == 3 {
			break
		}
		if len(s.Items) > 0 {
			add(s.Items[0].ID)
		}
	}
	return out
}

func (g *game) respawn(t float64) float64 {
	return 8 + float64(g.level(t))*1.6
}

func (g *game) level(t float64) int {
	if g.aram {
		return min(18, 3+int(t/80))
	}
	return min(18, 1+int(t/75))
}

// script pre-rolls the whole game: kills with swings between the teams, your
// deaths every couple of minutes (so the shop prompts fire), multikills, aces,
// and Rift objectives.
func (g *game) script(rng *mrand.Rand) {
	deadUntil := map[string]float64{}
	streak := map[string][2]float64{} // killer -> {count, last time}
	byTeam := map[string][]*player{}
	for _, p := range g.players {
		byTeam[p.team] = append(byTeam[p.team], p)
	}
	id := 0
	add := func(e event) {
		e.ID = id
		id++
		g.events = append(g.events, e)
	}
	add(event{Name: "GameStart", Time: 0})
	alive := func(team string, t float64) []*player {
		var out []*player
		for _, p := range byTeam[team] {
			if deadUntil[p.name] <= t {
				out = append(out, p)
			}
		}
		return out
	}
	nextMine := 150.0
	first := true
	gap := map[bool][2]float64{true: {8, 22}, false: {20, 70}}[g.aram]
	for t := 50.0; t < g.length; t += gap[0] + rng.Float64()*(gap[1]-gap[0]) {
		// Momentum swings: allies ahead early, enemies come back mid game.
		pAlly := 0.5 + 0.25*math.Sin(t/260)
		kt, vt := "ORDER", "CHAOS"
		if rng.Float64() > pAlly {
			kt, vt = vt, kt
		}
		killers, victims := alive(kt, t), alive(vt, t)
		var victim *player
		if t >= nextMine && deadUntil["Tester"] <= t {
			kt, vt = "CHAOS", "ORDER"
			killers = alive(kt, t)
			victim = byTeam["ORDER"][0]
			nextMine = t + 140 + rng.Float64()*60
		} else if len(victims) > 0 {
			victim = victims[rng.IntN(len(victims))]
		}
		if victim == nil || len(killers) == 0 {
			continue
		}
		killer := killers[rng.IntN(len(killers))]
		var assists []string
		for _, p := range killers {
			if p != killer && rng.Float64() < 0.45 {
				assists = append(assists, p.name)
			}
		}
		if first {
			add(event{Name: "FirstBlood", Time: t, Recipient: killer.name})
			first = false
		}
		add(event{Name: "ChampionKill", Time: t, KillerName: killer.name, VictimName: victim.name, Assisters: assists})
		deadUntil[victim.name] = t + g.respawn(t)
		s := streak[killer.name]
		if t-s[1] <= 10 {
			s[0]++
		} else {
			s[0] = 1
		}
		s[1] = t
		streak[killer.name] = s
		if s[0] >= 2 {
			add(event{Name: "Multikill", Time: t, KillerName: killer.name, KillStreak: int(s[0])})
		}
		if len(alive(vt, t)) == 0 {
			add(event{Name: "Ace", Time: t, AcingTeam: kt})
		}
		// Fights come in bursts: often a second kill right after.
		if rng.Float64() < 0.35 {
			if more := alive(vt, t); len(more) > 0 {
				v2 := more[rng.IntN(len(more))]
				t2 := t + 2 + rng.Float64()*4
				add(event{Name: "ChampionKill", Time: t2, KillerName: killer.name, VictimName: v2.name})
				deadUntil[v2.name] = t2 + g.respawn(t2)
				s[0]++
				s[1] = t2
				streak[killer.name] = s
				add(event{Name: "Multikill", Time: t2, KillerName: killer.name, KillStreak: int(s[0])})
				if len(alive(vt, t2)) == 0 {
					add(event{Name: "Ace", Time: t2, AcingTeam: kt})
				}
			}
		}
	}
	if !g.aram {
		drakes := []string{"Fire", "Water", "Earth", "Air", "Hextech", "Chemtech"}
		for i, t := 0, 300.0; t < g.length; i, t = i+1, t+330 {
			team := []string{"ORDER", "CHAOS"}[rng.IntN(2)]
			add(event{Name: "DragonKill", Time: t, KillerName: byTeam[team][3].name, DragonType: drakes[i%len(drakes)], Stolen: "False"})
		}
		add(event{Name: "HeraldKill", Time: 870, KillerName: byTeam["ORDER"][3].name, Stolen: "False"})
		add(event{Name: "BaronKill", Time: 1290, KillerName: byTeam["CHAOS"][3].name, Stolen: "False"})
	}
	sort.SliceStable(g.events, func(i, j int) bool { return g.events[i].Time < g.events[j].Time })
	for i := range g.events {
		g.events[i].ID = i
	}
}

// shop spends gold along the path: finish the next item if it is affordable,
// else buy the biggest components that fit and stop.
func (g *game) shop(inv []int, gold int, path []int) ([]int, int) {
	for _, target := range path {
		if has(inv, target) {
			continue
		}
		left, used := g.cost(target, inv)
		if gold >= left && (len(used) > 0 || len(inv) < 6) {
			inv = append(remove(inv, used), target)
			gold -= left
			continue
		}
		comps := g.parts(target)
		sort.Slice(comps, func(i, j int) bool { return g.d.Items[comps[i]].Gold > g.d.Items[comps[j]].Gold })
		for _, c := range comps {
			if price := g.d.Items[c].Gold; price <= gold && !has(inv, c) && len(inv) < 6 {
				inv = append(inv, c)
				gold -= price
			}
		}
		break
	}
	return inv, gold
}

func (g *game) parts(id int) []int {
	var out []int
	for _, f := range g.d.Items[id].From {
		if c, err := strconv.Atoi(f); err == nil {
			if _, ok := g.d.Items[c]; ok {
				out = append(out, c)
			}
		}
	}
	return out
}

// cost is what finishing id costs given inv, and which inventory slots it uses.
func (g *game) cost(id int, inv []int) (int, []int) {
	left := g.d.Items[id].Gold
	var used []int
	taken := map[int]bool{}
	var walk func(id int, depth int)
	walk = func(id int, depth int) {
		for _, c := range g.parts(id) {
			slot := -1
			for i, x := range inv {
				if x == c && !taken[i] {
					slot = i
					break
				}
			}
			if slot >= 0 {
				taken[slot] = true
				used = append(used, slot)
				left -= g.d.Items[c].Gold
			} else if depth < 4 {
				walk(c, depth+1)
			}
		}
	}
	walk(id, 0)
	return max(0, left), used
}

func has(inv []int, id int) bool {
	for _, x := range inv {
		if x == id {
			return true
		}
	}
	return false
}

func remove(inv []int, slots []int) []int {
	drop := map[int]bool{}
	for _, s := range slots {
		drop[s] = true
	}
	var out []int
	for i, x := range inv {
		if !drop[i] {
			out = append(out, x)
		}
	}
	return out
}

func (g *game) snapshot(t float64) map[string]any {
	type score struct{ k, d, a int }
	scores := map[string]*score{}
	deaths := map[string][]float64{}
	for _, p := range g.players {
		scores[p.name] = &score{}
	}
	var evs []event
	for _, e := range g.events {
		if e.Time > t {
			break
		}
		evs = append(evs, e)
		if e.Name != "ChampionKill" {
			continue
		}
		scores[e.KillerName].k++
		scores[e.VictimName].d++
		deaths[e.VictimName] = append(deaths[e.VictimName], e.Time)
		for _, a := range e.Assisters {
			scores[a].a++
		}
	}
	startGold, rate := 1400.0, 9.0
	if !g.aram {
		startGold, rate = 500, 6.5
	}
	var all []map[string]any
	var myGold float64
	for i, p := range g.players {
		sc := scores[p.name]
		earned := startGold + rate*t + 300*float64(sc.k) + 140*float64(sc.a)
		// Shop at the start, every death, and (everyone but you) every 75 s.
		moments := []float64{0}
		if p.name == "Tester" {
			// You shop while dead and leave as you respawn, so gold stays
			// unspent during the death timer (that is when the shop prompt shows).
			for _, d := range deaths[p.name] {
				moments = append(moments, d+g.respawn(d)-0.5)
			}
		} else {
			moments = append(moments, deaths[p.name]...)
			for m := 75.0; m <= t; m += 75 {
				moments = append(moments, m)
			}
		}
		sort.Float64s(moments)
		var inv []int
		spent := 0
		for _, m := range moments {
			if m > t {
				break
			}
			// Gold earned up to that moment (kills counted at the end: close enough).
			budget := int(startGold+rate*m) - spent
			if m == t || m == moments[len(moments)-1] {
				budget = int(earned) - spent
			}
			var left int
			inv, left = g.shop(inv, budget, p.path)
			spent += budget - left
		}
		if p.name == "Tester" {
			myGold = earned - float64(spent)
		}
		dead, timer := false, 0.0
		if ds := deaths[p.name]; len(ds) > 0 {
			if until := ds[len(ds)-1] + g.respawn(ds[len(ds)-1]); until > t {
				dead, timer = true, until-t
			}
		}
		var items []map[string]any
		for slot, id := range inv {
			items = append(items, map[string]any{"itemID": id, "displayName": g.d.Items[id].Name, "count": 1, "slot": slot, "consumable": false})
		}
		all = append(all, map[string]any{
			"championName": p.champ, "rawChampionName": "game_character_displayname_" + p.key, "isBot": false,
			"isDead": dead, "respawnTimer": timer, "level": g.level(t), "position": p.position, "skinID": 0,
			"riotIdGameName": p.name, "summonerName": p.name, "team": p.team, "items": items,
			"scores":         map[string]any{"kills": sc.k, "deaths": sc.d, "assists": sc.a, "creepScore": int(t / 60 * float64(4+i%5)), "wardScore": 0},
			"summonerSpells": map[string]any{"summonerSpellOne": map[string]string{"displayName": p.spells[0]}, "summonerSpellTwo": map[string]string{"displayName": p.spells[1]}},
			"runes":          map[string]any{"keystone": map[string]any{"id": 8005, "displayName": "Press the Attack"}},
		})
	}
	lvl := g.level(t)
	ab := func(n int) map[string]any { return map[string]any{"abilityLevel": n, "displayName": ""} }
	mode, mapID := "KIWI", 12
	if !g.aram {
		mode, mapID = "CLASSIC", 11
	}
	return map[string]any{
		"activePlayer": map[string]any{
			"riotIdGameName": "Tester", "summonerName": "Tester", "level": lvl, "currentGold": math.Max(0, myGold),
			"abilities": map[string]any{"Q": ab(min(5, (lvl+2)/3)), "W": ab(min(5, (lvl+1)/4)), "E": ab(min(5, lvl/4)), "R": ab(min(3, lvl/6))},
			"championStats": map[string]any{"attackDamage": 60 + 4*lvl, "abilityPower": 0, "armor": 28 + 4*lvl, "magicResist": 30 + lvl,
				"abilityHaste": 10, "moveSpeed": 335, "maxHealth": 600 + 95*lvl, "critChance": 0.25, "attackSpeed": 0.9, "lifeSteal": 0.08, "omnivamp": 0},
		},
		"allPlayers": all,
		"events":     map[string]any{"Events": evs},
		"gameData":   map[string]any{"gameMode": mode, "gameTime": t, "mapName": fmt.Sprintf("Map%d", mapID), "mapNumber": mapID},
	}
}

func selfSigned() tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fakegame"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
