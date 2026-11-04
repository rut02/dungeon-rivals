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
	Speed       int    `json:"speed,omitempty"`
	GoldMin     int    `json:"gold_min"`
	GoldMax     int    `json:"gold_max"`
	ScoreReward int    `json:"score_reward"`
	LootTable   string `json:"loot_table"`
	Tier        string `json:"tier"`
	Floor       int    `json:"floor,omitempty"`
}

type Catalog struct{ Monsters []Monster }

type Enemy struct {
	Index       int    `json:"index"`
	MonsterID   string `json:"monster_id"`
	Name        string `json:"name"`
	Tier        string `json:"tier"`
	HP          int    `json:"hp"`
	MaxHP       int    `json:"max_hp"`
	Attack      int    `json:"attack"`
	Defense     int    `json:"defense"`
	Speed       int    `json:"speed"`
	Intent      string `json:"intent"` // "ATTACK", "HEAVY", "BUFF"
	IntentValue int    `json:"intent_value"`
	Alive       bool   `json:"alive"`
}

type State struct {
	Enemies       []Enemy  `json:"enemies"`
	SelectedEnemy int      `json:"selected_enemy"`
	Turn          int      `json:"turn"`
	Finished      bool     `json:"finished"`
	Victory       bool     `json:"victory"`
	Guarding      bool     `json:"guarding"`
	FirstStrike   bool     `json:"first_strike"`
	SkillCooldown int      `json:"skill_cooldown"`
	EscapeSuccess bool     `json:"escape_success"`
	LootItemID    string   `json:"loot_item_id,omitempty"`
	Log           []string `json:"log"`

	// Backward compatibility fields for legacy clients
	MonsterID   string `json:"monster_id"`
	Name        string `json:"name"`
	Tier        string `json:"tier"`
	HP          int    `json:"hp"`
	MaxHP       int    `json:"max_hp"`
	Attack      int    `json:"attack"`
	Defense     int    `json:"defense"`
	Speed       int    `json:"speed"`
	Intent      string `json:"intent"`
	IntentValue int    `json:"intent_value"`
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

	scale := func(v int) int { return int(math.Ceil(float64(v) * multiplier)) }

	// Determine enemy count (Floor 1: 1 monster. Floor 2: 1-2 monsters. Floor 3: 1-3 monsters for weak enemies)
	count := 1
	if tier == "normal" {
		if floor == 1 {
			count = 1
		} else if floor == 2 {
			if rng.Intn(100) < 60 {
				count = 2
			} else {
				count = 1
			}
		} else {
			roll := rng.Intn(100)
			if roll < 35 {
				count = 3
			} else if roll < 75 {
				count = 2
			} else {
				count = 1
			}
		}
	}

	enemies := make([]Enemy, 0, count)
	log := make([]string, 0, 4)

	for i := 0; i < count; i++ {
		m := pool[rng.Intn(len(pool))]
		hp := scale(m.BaseHP)
		atk := scale(m.BaseAttack)
		def := scale(m.BaseDefense)
		if count == 2 {
			hp = max(10, int(math.Ceil(float64(hp)*0.75)))
			atk = max(3, int(math.Ceil(float64(atk)*0.80)))
		} else if count >= 3 {
			hp = max(10, int(math.Ceil(float64(hp)*0.65)))
			atk = max(3, int(math.Ceil(float64(atk)*0.70)))
		}
		spd := m.Speed
		if spd == 0 {
			spd = 4
		}
		intent := "ATTACK"
		intentVal := atk
		name := m.Name
		if count > 1 {
			name = fmt.Sprintf("%s #%d", m.Name, i+1)
		}
		enemies = append(enemies, Enemy{
			Index:       i,
			MonsterID:   m.ID,
			Name:        name,
			Tier:        m.Tier,
			HP:          hp,
			MaxHP:       hp,
			Attack:      atk,
			Defense:     def,
			Speed:       spd,
			Intent:      intent,
			IntentValue: intentVal,
			Alive:       true,
		})
		log = append(log, name+" appears!")
	}

	primaryMonster := pool[0]
	for _, m := range pool {
		if m.ID == enemies[0].MonsterID {
			primaryMonster = m
			break
		}
	}

	state := &State{
		Enemies:       enemies,
		SelectedEnemy: 0,
		Turn:          1,
		Finished:      false,
		Victory:       false,
		Guarding:      false,
		FirstStrike:   false,
		Log:           log,
	}
	state.SyncLegacyFields()

	return state, primaryMonster, nil
}

func (s *State) SyncLegacyFields() {
	if len(s.Enemies) == 0 {
		return
	}
	// Select first alive enemy or default to 0
	targetIndex := s.SelectedEnemy
	if targetIndex < 0 || targetIndex >= len(s.Enemies) || !s.Enemies[targetIndex].Alive {
		for i, e := range s.Enemies {
			if e.Alive {
				targetIndex = i
				s.SelectedEnemy = i
				break
			}
		}
	}
	target := s.Enemies[targetIndex]
	s.MonsterID = target.MonsterID
	s.Name = target.Name
	s.Tier = target.Tier
	s.HP = target.HP
	s.MaxHP = target.MaxHP
	s.Attack = target.Attack
	s.Defense = target.Defense
	s.Speed = target.Speed
	s.Intent = target.Intent
	s.IntentValue = target.IntentValue
}

func (s *State) AliveCount() int {
	count := 0
	for _, e := range s.Enemies {
		if e.Alive {
			count++
		}
	}
	return count
}

func (s *State) Selected() *Enemy {
	if len(s.Enemies) == 0 {
		return nil
	}
	if s.SelectedEnemy >= 0 && s.SelectedEnemy < len(s.Enemies) && s.Enemies[s.SelectedEnemy].Alive {
		return &s.Enemies[s.SelectedEnemy]
	}
	for i := range s.Enemies {
		if s.Enemies[i].Alive {
			s.SelectedEnemy = i
			return &s.Enemies[i]
		}
	}
	return nil
}

// Damage applies the MVP minimum-one damage rule.
func Damage(attack, defense int) int { return max(1, attack-defense) }
