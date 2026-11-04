package items

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

type Type string
type Slot string

const (
	Weapon        Type = "weapon"
	Armor         Type = "armor"
	Helmet        Type = "helmet"
	Accessory     Type = "accessory"
	Consumable    Type = "consumable"
	SlotWeapon    Slot = "weapon"
	SlotArmor     Slot = "armor"
	SlotHelmet    Slot = "helmet"
	SlotAccessory Slot = "accessory"
)

type Stats struct {
	Attack  int `json:"attack"`
	Defense int `json:"defense"`
	MaxHP   int `json:"max_hp"`
	MaxMP   int `json:"max_mp,omitempty"`
	Speed   int `json:"speed,omitempty"`
}

type Effect struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

type Definition struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Type           Type    `json:"type"`
	Rarity         string  `json:"rarity"`
	Slot           string  `json:"slot"`
	Stats          Stats   `json:"stats"`
	Effect         *Effect `json:"effect"`
	Price          int     `json:"price"`
	Stackable      bool    `json:"stackable"`
	MaxStack       int     `json:"max_stack"`
	UsableInCombat bool    `json:"usable_in_combat"`
	Archetype      string  `json:"archetype,omitempty"`
	SkillName      string  `json:"skill_name,omitempty"`
	SkillCost      int     `json:"skill_cost,omitempty"`
	SkillDesc      string  `json:"skill_desc,omitempty"`
	SkillTarget    string  `json:"skill_target,omitempty"`
}

type Catalog struct{ byID map[string]Definition }

// Load reads static item definitions and rejects invalid or incomplete data.
func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read item catalog: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var definitions []Definition
	if err := decoder.Decode(&definitions); err != nil {
		return nil, fmt.Errorf("parse item catalog: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("parse item catalog: expected one JSON array")
	}
	catalog, err := NewCatalog(definitions)
	if err != nil {
		return nil, err
	}
	if err := catalog.ValidateStartingItems(); err != nil {
		return nil, err
	}
	return catalog, nil
}

func (c *Catalog) ValidateStartingItems() error {
	for _, required := range []struct {
		id       string
		itemType Type
		slot     string
	}{
		{"training_sword", Weapon, "weapon"},
		{"iron_sword", Weapon, "weapon"},
		{"cloth_armor", Armor, "armor"},
		{"iron_armor", Armor, "armor"},
		{"health_potion", Consumable, ""},
		{"apprentice_staff", Weapon, "weapon"},
		{"mana_potion", Consumable, ""},
		{"iron_dagger", Weapon, "weapon"},
		{"hunting_bow", Weapon, "weapon"},
		{"wooden_shield", Accessory, "accessory"},
		{"apprentice_ring", Accessory, "accessory"},
		{"swift_boots", Accessory, "accessory"},
		{"quiver_charm", Accessory, "accessory"},
	} {
		item, ok := c.Get(required.id)
		if !ok || item.Type != required.itemType || item.Slot != required.slot {
			return fmt.Errorf("required starting item %q is missing or invalid", required.id)
		}
	}
	return nil
}

func NewCatalog(definitions []Definition) (*Catalog, error) {
	if len(definitions) == 0 {
		return nil, fmt.Errorf("item catalog must contain at least one item")
	}
	catalog := &Catalog{byID: make(map[string]Definition, len(definitions))}
	for i, item := range definitions {
		if err := validate(item); err != nil {
			return nil, fmt.Errorf("items[%d] (%q): %w", i, item.ID, err)
		}
		if _, exists := catalog.byID[item.ID]; exists {
			return nil, fmt.Errorf("duplicate item id %q", item.ID)
		}
		catalog.byID[item.ID] = item
	}
	return catalog, nil
}

func (c *Catalog) Get(id string) (Definition, bool) {
	if c == nil {
		return Definition{}, false
	}
	item, ok := c.byID[id]
	return item, ok
}

func (c *Catalog) List() []Definition {
	items := make([]Definition, 0, len(c.byID))
	for _, item := range c.byID {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func validate(item Definition) error {
	if item.ID == "" || item.Name == "" {
		return fmt.Errorf("id and name are required")
	}
	if item.Rarity != "common" && item.Rarity != "rare" && item.Rarity != "epic" && item.Rarity != "legendary" {
		return fmt.Errorf("invalid rarity %q", item.Rarity)
	}
	if item.Price < 0 || item.Stats.Attack < 0 || item.Stats.Defense < 0 || item.Stats.MaxHP < 0 || item.Stats.MaxMP < 0 {
		return fmt.Errorf("price and stats (atk, def, max_hp, max_mp) cannot be negative")
	}
	if item.Stackable && item.MaxStack < 2 {
		return fmt.Errorf("stackable item must have max_stack of at least 2")
	}
	if !item.Stackable && item.MaxStack != 1 {
		return fmt.Errorf("non-stackable item must have max_stack equal to 1")
	}
	switch item.Type {
	case Weapon:
		if item.Slot != "weapon" {
			return fmt.Errorf("weapon must use weapon slot")
		}
	case Armor:
		if item.Slot != "armor" {
			return fmt.Errorf("armor must use armor slot")
		}
	case Helmet:
		if item.Slot != "helmet" {
			return fmt.Errorf("helmet must use helmet slot")
		}
	case Accessory:
		if item.Slot != "accessory" {
			return fmt.Errorf("accessory must use accessory slot")
		}
	case Consumable:
		if item.Slot != "" {
			return fmt.Errorf("consumable must not have an equipment slot")
		}
		if item.Effect == nil {
			return fmt.Errorf("consumable must define an effect")
		}
		validEffects := map[string]bool{"heal": true, "heal_mp": true, "damage": true, "damage_aoe": true, "speed_buff": true, "smoke": true}
		if !validEffects[item.Effect.Type] || item.Effect.Value < 0 {
			return fmt.Errorf("unsupported or invalid consumable effect %q", item.Effect.Type)
		}
	default:
		return fmt.Errorf("invalid item type %q", item.Type)
	}
	if item.Type != Consumable && item.Effect != nil {
		return fmt.Errorf("equipment cannot define a consumable effect")
	}
	return nil
}
