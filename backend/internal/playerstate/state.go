package playerstate

import (
	"errors"

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
	ErrNotEnoughMP     = errors.New("NOT_ENOUGH_MANA")
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
	MaxHP     int
	MP        int
	MaxMP     int
	Level     int
	EXP       int
	MaxEXP    int
	BaseATK   int
	BaseDEF   int
	BaseSPD   int
	Gold      int
	Score     int
	Deaths    int
	Kit       string
	Inventory []Stack
	Equipment Equipment
}

type StatsSnapshot struct {
	HP                int    `json:"hp"`
	MaxHP             int    `json:"max_hp"`
	MP                int    `json:"mp"`
	MaxMP             int    `json:"max_mp"`
	Level             int    `json:"level"`
	EXP               int    `json:"exp"`
	MaxEXP            int    `json:"max_exp"`
	BaseATK           int    `json:"base_atk"`
	BaseDEF           int    `json:"base_def"`
	BaseSPD           int    `json:"base_spd"`
	ATK               int    `json:"atk"`
	DEF               int    `json:"def"`
	SPD               int    `json:"spd"`
	Archetype         string `json:"archetype"`
	WeaponSkill       string `json:"weapon_skill,omitempty"`
	WeaponSkillCost   int    `json:"weapon_skill_cost,omitempty"`
	WeaponSkillDesc   string `json:"weapon_skill_desc,omitempty"`
	WeaponSkillTarget string `json:"weapon_skill_target,omitempty"`
}

type PrivateSnapshot struct {
	Stats     StatsSnapshot `json:"stats"`
	Gold      int           `json:"gold"`
	Score     int           `json:"score"`
	Deaths    int           `json:"deaths"`
	Kit       string        `json:"kit"`
	Inventory []Stack       `json:"inventory"`
	Equipment Equipment     `json:"equipment"`
}

// NewDefault creates the starting player state with default items.
func NewDefault(catalog *items.Catalog) (*State, error) {
	return &State{
		HP:        100,
		MaxHP:     100,
		MP:        20,
		MaxMP:     20,
		Level:     1,
		EXP:       0,
		MaxEXP:    30,
		BaseATK:   5,
		BaseDEF:   0,
		BaseSPD:   5,
		Gold:      20,
		Score:     0,
		Kit:       "default",
		Inventory: []Stack{{ItemID: "health_potion", Quantity: 1}},
		Equipment: Equipment{Weapon: "training_sword", Armor: "cloth_armor"},
	}, nil
}

// NewWithKit creates player state customized for the chosen starting loadout.
// Starting loadout consists strictly of the starting weapon + 1x Health Potion + 1x Mana Potion.
// All armors, shields, and accessories must be discovered and farmed in the dungeon.
func NewWithKit(catalog *items.Catalog, kit string) (*State, error) {
	if kit == "" {
		kit = "knight"
	}
	var s *State
	switch kit {
	case "mage":
		s = &State{
			HP:        90,
			MaxHP:     90,
			MP:        35,
			MaxMP:     35,
			Level:     1,
			EXP:       0,
			MaxEXP:    30,
			BaseATK:   10,
			BaseDEF:   2,
			BaseSPD:   6,
			Gold:      25,
			Score:     0,
			Kit:       "mage",
			Inventory: []Stack{{ItemID: "health_potion", Quantity: 1}, {ItemID: "mana_potion", Quantity: 1}},
			Equipment: Equipment{Weapon: "apprentice_staff", Armor: "", Accessories: [2]string{"", ""}},
		}
	case "rogue":
		s = &State{
			HP:        100,
			MaxHP:     100,
			MP:        30,
			MaxMP:     30,
			Level:     1,
			EXP:       0,
			MaxEXP:    30,
			BaseATK:   8,
			BaseDEF:   2,
			BaseSPD:   9,
			Gold:      25,
			Score:     0,
			Kit:       "rogue",
			Inventory: []Stack{{ItemID: "health_potion", Quantity: 1}, {ItemID: "mana_potion", Quantity: 1}},
			Equipment: Equipment{Weapon: "iron_dagger", Armor: "", Accessories: [2]string{"", ""}},
		}
	case "ranger":
		s = &State{
			HP:        100,
			MaxHP:     100,
			MP:        30,
			MaxMP:     30,
			Level:     1,
			EXP:       0,
			MaxEXP:    30,
			BaseATK:   8,
			BaseDEF:   2,
			BaseSPD:   8,
			Gold:      25,
			Score:     0,
			Kit:       "ranger",
			Inventory: []Stack{{ItemID: "health_potion", Quantity: 1}, {ItemID: "mana_potion", Quantity: 1}},
			Equipment: Equipment{Weapon: "hunting_bow", Armor: "", Accessories: [2]string{"", ""}},
		}
	default: // knight / warrior
		s = &State{
			HP:        110,
			MaxHP:     110,
			MP:        25,
			MaxMP:     25,
			Level:     1,
			EXP:       0,
			MaxEXP:    30,
			BaseATK:   8,
			BaseDEF:   4,
			BaseSPD:   6,
			Gold:      25,
			Score:     0,
			Kit:       "knight",
			Inventory: []Stack{{ItemID: "health_potion", Quantity: 1}, {ItemID: "mana_potion", Quantity: 1}},
			Equipment: Equipment{Weapon: "iron_sword", Armor: "", Accessories: [2]string{"", ""}},
		}
	}
	if catalog != nil {
		derived := s.derived(catalog)
		s.HP = derived.MaxHP
		s.MP = derived.MaxMP
	}
	return s, nil
}

func (s *State) Snapshot(catalog *items.Catalog) PrivateSnapshot {
	return PrivateSnapshot{
		Stats:     s.derived(catalog),
		Gold:      s.Gold,
		Score:     s.Score,
		Deaths:    s.Deaths,
		Kit:       s.Kit,
		Inventory: append([]Stack(nil), s.Inventory...),
		Equipment: s.Equipment,
	}
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

// RestoreMP recovers MP up to maximum.
func (s *State) RestoreMP(amount int, catalog *items.Catalog) {
	maximum := s.derived(catalog).MaxMP
	s.MP = min(maximum, max(0, s.MP+amount))
}

// ConsumeMP spends MP if available.
func (s *State) ConsumeMP(amount int) bool {
	if s.MP < amount {
		return false
	}
	s.MP -= amount
	return true
}

// GainEXP awards EXP and performs level up with stat growth when threshold is reached.
func (s *State) GainEXP(amount int, catalog *items.Catalog) bool {
	if amount <= 0 {
		return false
	}
	s.EXP += amount
	leveledUp := false
	for s.EXP >= s.MaxEXP {
		s.EXP -= s.MaxEXP
		s.Level++
		s.MaxEXP = int(float64(s.MaxEXP) * 1.5)
		s.MaxHP += 15
		s.MaxMP += 5
		s.BaseATK += 2
		s.BaseDEF += 1
		s.BaseSPD += 1
		// Level up heal bonus
		maxStats := s.derived(catalog)
		s.HP = maxStats.MaxHP
		s.MP = maxStats.MaxMP
		leveledUp = true
	}
	return leveledUp
}

func (s *State) derived(catalog *items.Catalog) StatsSnapshot {
	baseMaxHP := s.MaxHP
	if baseMaxHP == 0 {
		baseMaxHP = 100
	}
	baseMaxMP := s.MaxMP
	if baseMaxMP == 0 {
		baseMaxMP = 20
	}
	baseATK := s.BaseATK
	if baseATK == 0 {
		baseATK = 5
	}
	baseSPD := s.BaseSPD
	if baseSPD == 0 {
		baseSPD = 5
	}

	stats := StatsSnapshot{
		HP:        s.HP,
		MaxHP:     baseMaxHP,
		MP:        s.MP,
		MaxMP:     baseMaxMP,
		Level:     s.Level,
		EXP:       s.EXP,
		MaxEXP:    s.MaxEXP,
		BaseATK:   baseATK,
		BaseDEF:   s.BaseDEF,
		BaseSPD:   baseSPD,
		ATK:       baseATK,
		DEF:       s.BaseDEF,
		SPD:       baseSPD,
		Archetype: "sword",
	}

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
		stats.MaxMP += item.Stats.MaxMP
		stats.SPD += item.Stats.Speed
		if item.Type == items.Weapon {
			if item.Archetype != "" {
				stats.Archetype = item.Archetype
			}
			stats.WeaponSkill = item.SkillName
			stats.WeaponSkillCost = item.SkillCost
			stats.WeaponSkillDesc = item.SkillDesc
			stats.WeaponSkillTarget = item.SkillTarget
		}
	}
	if stats.HP > stats.MaxHP {
		stats.HP = stats.MaxHP
	}
	if stats.MP > stats.MaxMP {
		stats.MP = stats.MaxMP
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
	if items.Slot(slot) == items.SlotAccessory {
		s.Equipment.Accessories[indexValue(index)] = ""
	} else {
		s.setSlot(slot, "", 0)
	}
	s.Inventory = working
	s.clampHP(catalog)
	return nil
}

func (s *State) Use(itemID, phase string, catalog *items.Catalog) error {
	item, ok := catalog.Get(itemID)
	if !ok {
		return ErrItemNotFound
	}
	if item.Type != items.Consumable {
		return ErrItemNotUsable
	}
	if phase == "lobby" {
		return ErrWrongPhase
	}
	if phase == "combat" && !item.UsableInCombat {
		return ErrWrongPhase
	}
	stats := s.derived(catalog)
	if item.Effect != nil {
		if (item.Effect.Type == "damage" || item.Effect.Type == "damage_aoe" || item.Effect.Type == "smoke") && phase != "combat" && phase != "pvp" {
			return ErrWrongPhase
		}
		if item.Effect.Type == "heal" && s.HP >= stats.MaxHP {
			return ErrItemNotUsable
		}
		if item.Effect.Type == "heal_mp" && s.MP >= stats.MaxMP {
			return ErrItemNotUsable
		}
	}
	working := append([]Stack(nil), s.Inventory...)
	if err := remove(&working, itemID, 1); err != nil {
		return err
	}
	if item.Effect != nil {
		switch item.Effect.Type {
		case "heal":
			s.HP = min(stats.MaxHP, s.HP+item.Effect.Value)
		case "heal_mp":
			s.MP = min(stats.MaxMP, s.MP+item.Effect.Value)
		case "damage", "damage_aoe", "smoke", "speed_buff":
			// Processed during combat resolution
		}
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

// Count returns the total quantity of itemID in the inventory.
func (s *State) Count(itemID string) int {
	for _, stack := range s.Inventory {
		if stack.ItemID == itemID {
			return stack.Quantity
		}
	}
	return 0
}
