// Package combat contains server-owned PvE encounter rules.
package combat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
)

type Monster struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	BaseHP      int    `json:"base_hp"`
	BaseAttack  int    `json:"base_attack"`
	BaseDefense int    `json:"base_defense"`
	GoldMin     int    `json:"gold_min"`
	GoldMax     int    `json:"gold_max"`
	ScoreReward int    `json:"score_reward"`
	LootTable   string `json:"loot_table"`
	Tier        string `json:"tier"`
	Floor       int    `json:"floor,omitempty"`
}

type Catalog struct{ Monsters []Monster }
type State struct {
	MonsterID string   `json:"monster_id"`
	Name      string   `json:"name"`
	Tier      string   `json:"tier"`
	HP        int      `json:"hp"`
	MaxHP     int      `json:"max_hp"`
	Attack    int      `json:"attack"`
	Defense   int      `json:"defense"`
	Guarding  bool     `json:"guarding"`
	Finished  bool     `json:"finished"`
	Log       []string `json:"log"`
}

var (
	ErrInvalidCatalog = errors.New("INVALID_MONSTER_CATALOG")
	ErrNoMonsters     = errors.New("NO_MONSTERS_FOR_ROOM")
	ErrNoCombat       = errors.New("NO_ACTIVE_COMBAT")
	ErrInvalidAction  = errors.New("INVALID_COMBAT_ACTION")
	ErrBossEscape     = errors.New("BOSS_ESCAPE_NOT_ALLOWED")
)

func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var ms []Monster
	if err = d.Decode(&ms); err != nil {
		return nil, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON array")
	}
	if len(ms) == 0 {
		return nil, ErrInvalidCatalog
	}
	seen := map[string]bool{}
	for _, m := range ms {
		if m.ID == "" || m.Name == "" || m.BaseHP < 1 || m.BaseAttack < 0 || m.BaseDefense < 0 || m.GoldMin < 0 || m.GoldMax < m.GoldMin || m.ScoreReward < 0 || seen[m.ID] || (m.Tier != "normal" && m.Tier != "elite" && m.Tier != "boss") {
			return nil, fmt.Errorf("%w: invalid monster %q", ErrInvalidCatalog, m.ID)
		}
		seen[m.ID] = true
	}
	return &Catalog{Monsters: ms}, nil
}

func (c *Catalog) Create(roomType string, floor int, multiplier float64, seed int64) (*State, Monster, error) {
	if c == nil {
		return nil, Monster{}, ErrNoMonsters
	}
	tier := roomType
	if tier == "MONSTER" {
		tier = "normal"
	}
	if tier == "ELITE" {
		tier = "elite"
	}
	if tier == "BOSS" {
		tier = "boss"
	}
	pool := []Monster{}
	for _, m := range c.Monsters {
		if m.Tier == tier && (m.Floor == 0 || m.Floor == floor) {
			pool = append(pool, m)
		}
	}
	if len(pool) == 0 {
		return nil, Monster{}, ErrNoMonsters
	}
	rng := rand.New(rand.NewSource(seed))
	m := pool[rng.Intn(len(pool))]
	scale := func(v int) int { return int(math.Ceil(float64(v) * multiplier)) }
	copy := m
	copy.BaseHP = scale(m.BaseHP)
	copy.BaseAttack = scale(m.BaseAttack)
	copy.BaseDefense = scale(m.BaseDefense)
	return &State{MonsterID: m.ID, Name: m.Name, Tier: m.Tier, HP: copy.BaseHP, MaxHP: copy.BaseHP, Attack: copy.BaseAttack, Defense: copy.BaseDefense, Log: []string{m.Name + " appears."}}, copy, nil
}

// Damage applies the MVP minimum-one damage rule.
func Damage(attack, defense int) int { return max(1, attack-defense) }
