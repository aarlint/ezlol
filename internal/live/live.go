// Package live reads the in-game Live Client Data API (https://127.0.0.1:2999),
// which the game client serves while a match is running. No credentials needed.
package live

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

const base = "https://127.0.0.1:2999/liveclientdata"

// Client fetches and briefly caches game data.
type Client struct {
	http  *http.Client
	mu    sync.Mutex
	cache *GameData
	at    time.Time
}

// New creates a client. The game presents a self-signed loopback cert.
func New() *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // loopback self-signed cert
		DialContext:     (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext,
	}
	return &Client{http: &http.Client{Transport: tr, Timeout: 2 * time.Second}}
}

// GameData is the trimmed allgamedata payload.
type GameData struct {
	ActivePlayer ActivePlayer `json:"activePlayer"`
	AllPlayers   []Player     `json:"allPlayers"`
	Events       struct {
		Events []Event `json:"Events"`
	} `json:"events"`
	GameData struct {
		GameMode  string  `json:"gameMode"`
		GameTime  float64 `json:"gameTime"`
		MapName   string  `json:"mapName"`
		MapNumber int     `json:"mapNumber"`
	} `json:"gameData"`
}

// ActivePlayer is you.
type ActivePlayer struct {
	Abilities    map[string]Ability `json:"abilities"`
	Stats        map[string]any     `json:"championStats"`
	CurrentGold  float64            `json:"currentGold"`
	Level        int                `json:"level"`
	RiotIDName   string             `json:"riotIdGameName"`
	SummonerName string             `json:"summonerName"`
}

// Ability is one of Passive/Q/W/E/R.
type Ability struct {
	Level       int    `json:"abilityLevel"`
	DisplayName string `json:"displayName"`
}

// Player is one of the ten participants.
type Player struct {
	ChampionName    string  `json:"championName"`
	RawChampionName string  `json:"rawChampionName"`
	IsBot           bool    `json:"isBot"`
	IsDead          bool    `json:"isDead"`
	Level           int     `json:"level"`
	Position        string  `json:"position"`
	RespawnTimer    float64 `json:"respawnTimer"`
	SkinID          int     `json:"skinID"`
	RiotIDName      string  `json:"riotIdGameName"`
	SummonerName    string  `json:"summonerName"`
	Team            string  `json:"team"` // ORDER | CHAOS
	Items           []Item  `json:"items"`
	Scores          struct {
		Kills      int     `json:"kills"`
		Deaths     int     `json:"deaths"`
		Assists    int     `json:"assists"`
		CreepScore int     `json:"creepScore"`
		WardScore  float64 `json:"wardScore"`
	} `json:"scores"`
	SummonerSpells struct {
		One Spell `json:"summonerSpellOne"`
		Two Spell `json:"summonerSpellTwo"`
	} `json:"summonerSpells"`
	Runes struct {
		Keystone struct {
			ID          int    `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"keystone"`
	} `json:"runes"`
}

// Item is an inventory slot.
type Item struct {
	ID          int    `json:"itemID"`
	DisplayName string `json:"displayName"`
	Count       int    `json:"count"`
	Slot        int    `json:"slot"`
	Consumable  bool   `json:"consumable"`
}

// Spell is a summoner spell.
type Spell struct {
	DisplayName    string `json:"displayName"`
	RawDisplayName string `json:"rawDisplayName"`
}

// Event is a game event (ChampionKill, Multikill, FirstBlood, TurretKilled, ...).
type Event struct {
	ID         int      `json:"EventID"`
	Name       string   `json:"EventName"`
	Time       float64  `json:"EventTime"`
	KillerName string   `json:"KillerName,omitempty"`
	VictimName string   `json:"VictimName,omitempty"`
	Assisters  []string `json:"Assisters,omitempty"`
	KillStreak int      `json:"KillStreak,omitempty"`
	Acer       string   `json:"Acer,omitempty"`
	AcingTeam  string   `json:"AcingTeam,omitempty"`
	DragonType string   `json:"DragonType,omitempty"`
	Stolen     string   `json:"Stolen,omitempty"`
}

// Get returns game data, cached for up to 700ms. Returns nil, nil when no game is running.
func (c *Client) Get(ctx context.Context) (*GameData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache != nil && time.Since(c.at) < 700*time.Millisecond {
		return c.cache, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/allgamedata", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.cache = nil
		return nil, nil // game not running
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		c.cache = nil
		return nil, nil // loading screen returns 404
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var gd GameData
	if err := json.Unmarshal(b, &gd); err != nil {
		return nil, fmt.Errorf("live: %w", err)
	}
	c.cache = &gd
	c.at = time.Now()
	return &gd, nil
}
