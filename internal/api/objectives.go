package api

import (
	"sort"
	"strings"

	"github.com/aarlint/ezlol/internal/live"
)

// objectives summarises Summoner's Rift neutral objectives from the event feed.
type objectives struct {
	AllyDragons  []string `json:"allyDragons"`
	EnemyDragons []string `json:"enemyDragons"`
	AllyGrubs    int      `json:"allyGrubs"`
	EnemyGrubs   int      `json:"enemyGrubs"`
	AllyHerald   bool     `json:"allyHerald"`
	EnemyHerald  bool     `json:"enemyHerald"`
	AllyBarons   int      `json:"allyBarons"`
	EnemyBarons  int      `json:"enemyBarons"`
	AllyTurrets  int      `json:"allyTurrets"` // turrets taken by allies
	EnemyTurrets int      `json:"enemyTurrets"`
	Next         []spawn  `json:"next"`
	Soul         string   `json:"soul,omitempty"` // "ally" | "enemy" when a soul is secured
	Notes        []string `json:"notes,omitempty"`
}

type spawn struct {
	Name string  `json:"name"`
	At   float64 `json:"at"` // game seconds
	In   float64 `json:"in"`
}

// Spawn rules (seconds). Dragon: first at 5:00, 5:00 after each kill (elder 6:00).
// Void grubs 8:00 (second wave 4:00 after the first three die, until 13:45 herald prep).
// Herald 16:00 (single). Baron 25:00, respawn 6:00. Atakhan 20:00 (single).
const (
	dragonFirst   = 300.0
	dragonRespawn = 300.0
	elderRespawn  = 360.0
	grubsFirst    = 480.0
	heraldAt      = 960.0
	baronFirst    = 1500.0
	baronRespawn  = 360.0
	atakhanAt     = 1200.0
	platesFall    = 840.0
)

func summarizeObjectives(gd *live.GameData, players []livePlayer, myTeam string) *objectives {
	teamOf := map[string]string{}
	for _, p := range players {
		teamOf[p.Name] = p.Team
	}
	ally := func(killer string) bool { return teamOf[killer] == myTeam }
	o := &objectives{}
	var lastDragon, lastBaron float64 = -1, -1
	var heraldDead, atakhanDead bool
	var grubsKilled int
	var lastGrub float64
	elder := false
	for _, e := range gd.Events.Events {
		switch e.Name {
		case "DragonKill":
			lastDragon = e.Time
			dt := strings.TrimSpace(e.DragonType)
			if strings.EqualFold(dt, "Elder") {
				elder = true
			}
			if ally(e.KillerName) {
				o.AllyDragons = append(o.AllyDragons, dt)
			} else {
				o.EnemyDragons = append(o.EnemyDragons, dt)
			}
		case "BaronKill":
			lastBaron = e.Time
			if ally(e.KillerName) {
				o.AllyBarons++
			} else {
				o.EnemyBarons++
			}
		case "HeraldKill":
			heraldDead = true
			if ally(e.KillerName) {
				o.AllyHerald = true
			} else {
				o.EnemyHerald = true
			}
		case "HordeKill":
			grubsKilled++
			lastGrub = e.Time
			if ally(e.KillerName) {
				o.AllyGrubs++
			} else {
				o.EnemyGrubs++
			}
		case "AtakhanKill":
			atakhanDead = true
		case "TurretKilled":
			if ally(e.KillerName) {
				o.AllyTurrets++
			} else {
				o.EnemyTurrets++
			}
		}
	}
	if len(o.AllyDragons) >= 4 {
		o.Soul = "ally"
	} else if len(o.EnemyDragons) >= 4 {
		o.Soul = "enemy"
	}
	t := gd.GameData.GameTime
	add := func(name string, at float64) {
		if at > t-1 {
			o.Next = append(o.Next, spawn{Name: name, At: at, In: at - t})
		}
	}
	// Dragon
	nextDragon := dragonFirst
	if lastDragon >= 0 {
		if elder || o.Soul != "" {
			nextDragon = lastDragon + elderRespawn
		} else {
			nextDragon = lastDragon + dragonRespawn
		}
	}
	name := "Dragon"
	if o.Soul != "" {
		name = "Elder"
	}
	add(name, nextDragon)
	// Grubs: two waves of three before herald time.
	if t < heraldAt-15 {
		switch {
		case grubsKilled == 0:
			add("Void grubs", grubsFirst)
		case grubsKilled == 3 && lastGrub+240 < heraldAt-15:
			add("Void grubs (2nd)", lastGrub+240)
		}
	}
	if !heraldDead && t < baronFirst {
		add("Herald", heraldAt)
	}
	if !atakhanDead {
		add("Atakhan", atakhanAt)
	}
	nextBaron := baronFirst
	if lastBaron >= 0 {
		nextBaron = lastBaron + baronRespawn
	}
	add("Baron", nextBaron)
	if t < platesFall {
		add("Plates fall", platesFall)
	}
	sort.Slice(o.Next, func(i, j int) bool { return o.Next[i].At < o.Next[j].At })
	if len(o.AllyDragons) == 3 {
		o.Notes = append(o.Notes, "Soul point: next dragon wins the soul")
	}
	if len(o.EnemyDragons) == 3 {
		o.Notes = append(o.Notes, "Enemy on soul point: contest or trade")
	}
	return o
}

// laneOpponent returns the enemy champion sharing your position (Rift only).
func laneOpponent(players []livePlayer) string {
	var me *livePlayer
	for i := range players {
		if players[i].IsMe {
			me = &players[i]
		}
	}
	if me == nil || me.Position == "" || me.Position == "NONE" {
		return ""
	}
	for _, p := range players {
		if p.Team != me.Team && p.Position == me.Position {
			return p.Champion.Name
		}
	}
	return ""
}
