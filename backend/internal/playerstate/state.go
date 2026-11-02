package playerstate

import (
	"errors"
	"fmt"

	"dungeon-rivals/backend/internal/items"
)

const BackpackCapacity = 12

var (
	ErrItemNotFound    = errors.New("ITEM_NOT_FOUND")
	ErrItemNotUsable   = errors.New("ITEM_NOT_USABLE")
	ErrInventoryFull   = errors.New("INVENTORY_FULL")
	ErrInvalidSlot     = errors.New("INVALID_SLOT")
	ErrWrongPhase      = errors.New("INVALID_PHASE")
	ErrInvalidQuantity = errors.New("INVALID_QUANTITY")
)

type Stack struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

type Equipment struct {
	Weapon      string    `json:"weapon"`
	Armor       string    `json:"armor"`
	Helmet      string    `json:"helmet"`
	Accessories [2]string `json:"accessories"`
}

// State is mutable server-owned player progression for a match.
type State struct {
	HP        int
	Gold      int
	Score     int
	Deaths    int
	Inventory []Stack
	Equipment Equipment
}

type StatsSnapshot struct {
	HP      int `json:"hp"`
	MaxHP   int `json:"max_hp"`
	BaseATK int `json:"base_atk"`
	BaseDEF int `json:"base_def"`
	ATK     int `json:"atk"`
	DEF     int `json:"def"`
}

type PrivateSnapshot struct {
	Stats     StatsSnapshot `json:"stats"`
	Gold      int           `json:"gold"`
	Score     int           `json:"score"`
	Deaths    int           `json:"deaths"`
	Inventory []Stack       `json:"inventory"`
	Equipment Equipment     `json:"equipment"`
}

// NewDefault creates the starting player state specified by the MVP plan.
func NewDefault(catalog *items.Catalog) (*State, error) {
	for _, required := range []struct {
		id       string
		itemType items.Type
		slot     string
	}{
		{"training_sword", items.Weapon, "weapon"},
		{"cloth_armor", items.Armor, "armor"},
		{"health_potion", items.Consumable, ""},
	} {
		item, ok := catalog.Get(required.id)
		if !ok || item.Type != required.itemType || item.Slot != required.slot {
			return nil, fmt.Errorf("required starting item %q is missing or invalid", required.id)
		}
	}
	return &State{HP: 100, Gold: 20, Score: 0, Inventory: []Stack{{ItemID: "health_potion", Quantity: 1}}, Equipment: Equipment{Weapon: "training_sword", Armor: "cloth_armor"}}, nil
}

func (s *State) Snapshot(catalog *items.Catalog) PrivateSnapshot {
	return PrivateSnapshot{Stats: s.derived(catalog), Gold: s.Gold, Score: s.Score, Deaths: s.Deaths, Inventory: append([]Stack(nil), s.Inventory...), Equipment: s.Equipment}
}

// Stats returns server-derived combat stats for the current equipment.
func (s *State) Stats(catalog *items.Catalog) StatsSnapshot { return s.derived(catalog) }

// Damage reduces HP without allowing it to fall below zero.
func (s *State) Damage(amount int) {
	if amount < 0 {
		return
	}
	s.HP = max(0, s.HP-amount)
}

// RestoreHP sets HP to a value within the current maximum.
func (s *State) RestoreHP(amount int, catalog *items.Catalog) {
	maximum := s.derived(catalog).MaxHP
	s.HP = min(maximum, max(0, amount))
}

func (s *State) derived(catalog *items.Catalog) StatsSnapshot {
	stats := StatsSnapshot{HP: s.HP, MaxHP: 100, BaseATK: 5, BaseDEF: 0, ATK: 5, DEF: 0}
	for _, id := range s.equippedIDs() {
		if id == "" {
			continue
		}
		item, ok := catalog.Get(id)
		if !ok {
			continue
		}
		stats.ATK += item.Stats.Attack
		stats.DEF += item.Stats.Defense
		stats.MaxHP += item.Stats.MaxHP
	}
	if stats.HP > stats.MaxHP {
		stats.HP = stats.MaxHP
	}
	return stats
}

func (s *State) Equip(itemID, slot string, catalog *items.Catalog) error {
	item, ok := catalog.Get(itemID)
	if !ok {
		return ErrItemNotFound
	}
	if item.Type == items.Consumable || item.Slot != slot {
		return ErrInvalidSlot
	}
	working := append([]Stack(nil), s.Inventory...)
	if err := remove(&working, itemID, 1); err != nil {
		return err
	}
	var old string
	switch items.Slot(slot) {
	case items.SlotWeapon:
		old = s.Equipment.Weapon
	case items.SlotArmor:
		old = s.Equipment.Armor
	case items.SlotHelmet:
		old = s.Equipment.Helmet
	case items.SlotAccessory:
		if s.Equipment.Accessories[0] == "" {
			s.Equipment.Accessories[0] = itemID
			s.Inventory = working
			s.clampHP(catalog)
			return nil
		}
		if s.Equipment.Accessories[1] == "" {
			s.Equipment.Accessories[1] = itemID
			s.Inventory = working
			s.clampHP(catalog)
			return nil
		}
		old = s.Equipment.Accessories[0]
	default:
		return ErrInvalidSlot
	}
	if old != "" {
		if err := add(&working, old, 1, catalog); err != nil {
			return err
		}
	}
	s.setSlot(slot, itemID, 0)
	s.Inventory = working
	s.clampHP(catalog)
	return nil
}

func (s *State) Unequip(slot string, index *int, catalog *items.Catalog) error {
	var itemID string
	if items.Slot(slot) == items.SlotAccessory {
		i := 0
		if index != nil {
			i = *index
		}
		if i < 0 || i >= len(s.Equipment.Accessories) {
			return ErrInvalidSlot
		}
		itemID = s.Equipment.Accessories[i]
	} else {
		if index != nil && *index != 0 {
			return ErrInvalidSlot
		}
		switch items.Slot(slot) {
		case items.SlotWeapon:
			itemID = s.Equipment.Weapon
		case items.SlotArmor:
			itemID = s.Equipment.Armor
		case items.SlotHelmet:
			itemID = s.Equipment.Helmet
		default:
			return ErrInvalidSlot
		}
	}
	if itemID == "" {
		return ErrInvalidSlot
	}
	working := append([]Stack(nil), s.Inventory...)
	if err := add(&working, itemID, 1, catalog); err != nil {
		return err
	}
	s.setSlot(slot, "", indexValue(index))
	s.Inventory = working
	s.clampHP(catalog)
	return nil
}

func (s *State) Use(itemID, phase string, catalog *items.Catalog) error {
	if phase != "dungeon" && phase != "preparation" && phase != "combat" && phase != "pvp" {
		return ErrWrongPhase
	}
	item, ok := catalog.Get(itemID)
	if !ok {
		return ErrItemNotFound
	}
	if item.Type != items.Consumable || item.Effect == nil || (phase == "combat" && !item.UsableInCombat) {
		return ErrItemNotUsable
	}
	if _, stack := findStack(s.Inventory, itemID); stack == nil {
		return ErrItemNotFound
	}
	stats := s.derived(catalog)
	if s.HP >= stats.MaxHP {
		return ErrItemNotUsable
	}
	working := append([]Stack(nil), s.Inventory...)
	if err := remove(&working, itemID, 1); err != nil {
		return err
	}
	s.HP += item.Effect.Value
	if s.HP > stats.MaxHP {
		s.HP = stats.MaxHP
	}
	s.Inventory = working
	return nil
}

func (s *State) Add(itemID string, quantity int, catalog *items.Catalog) error {
	working := append([]Stack(nil), s.Inventory...)
	if err := add(&working, itemID, quantity, catalog); err != nil {
		return err
	}
	s.Inventory = working
	return nil
}

// Remove deletes quantity items from the backpack using normal stack rules.
func (s *State) Remove(itemID string, quantity int) error {
	working := append([]Stack(nil), s.Inventory...)
	if err := remove(&working, itemID, quantity); err != nil {
		return err
	}
	s.Inventory = working
	return nil
}

func add(inventory *[]Stack, itemID string, quantity int, catalog *items.Catalog) error {
	if quantity < 1 {
		return ErrInvalidQuantity
	}
	item, ok := catalog.Get(itemID)
	if !ok {
		return ErrItemNotFound
	}
	if !item.Stackable && quantity > item.MaxStack {
		return ErrInvalidQuantity
	}
	remaining := quantity
	if item.Stackable {
		for i := range *inventory {
			stack := &(*inventory)[i]
			if stack.ItemID != itemID || stack.Quantity >= item.MaxStack {
				continue
			}
			added := min(remaining, item.MaxStack-stack.Quantity)
			stack.Quantity += added
			remaining -= added
			if remaining == 0 {
				return nil
			}
		}
	}
	for remaining > 0 {
		if len(*inventory) >= BackpackCapacity {
			return ErrInventoryFull
		}
		qty := 1
		if item.Stackable {
			qty = min(remaining, item.MaxStack)
		}
		*inventory = append(*inventory, Stack{ItemID: itemID, Quantity: qty})
		remaining -= qty
	}
	return nil
}

func remove(inventory *[]Stack, itemID string, quantity int) error {
	if quantity < 1 {
		return ErrInvalidQuantity
	}
	index, stack := findStack(*inventory, itemID)
	if stack == nil {
		return ErrItemNotFound
	}
	if stack.Quantity < quantity {
		return ErrItemNotFound
	}
	(*inventory)[index].Quantity -= quantity
	if (*inventory)[index].Quantity == 0 {
		*inventory = append((*inventory)[:index], (*inventory)[index+1:]...)
	}
	return nil
}

func findStack(inventory []Stack, itemID string) (int, *Stack) {
	for i := range inventory {
		if inventory[i].ItemID == itemID {
			return i, &inventory[i]
		}
	}
	return -1, nil
}
func (s *State) equippedIDs() []string {
	return []string{s.Equipment.Weapon, s.Equipment.Armor, s.Equipment.Helmet, s.Equipment.Accessories[0], s.Equipment.Accessories[1]}
}
func (s *State) clampHP(catalog *items.Catalog) {
	stats := s.derived(catalog)
	if s.HP > stats.MaxHP {
		s.HP = stats.MaxHP
	}
}
func (s *State) setSlot(slot, itemID string, index int) {
	switch items.Slot(slot) {
	case items.SlotWeapon:
		s.Equipment.Weapon = itemID
	case items.SlotArmor:
		s.Equipment.Armor = itemID
	case items.SlotHelmet:
		s.Equipment.Helmet = itemID
	case items.SlotAccessory:
		s.Equipment.Accessories[index] = itemID
	}
}
func indexValue(index *int) int {
	if index == nil {
		return 0
	}
	return *index
}
