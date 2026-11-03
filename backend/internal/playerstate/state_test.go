package playerstate

import (
	"testing"

	"dungeon-rivals/backend/internal/items"
)

func testCatalog(t *testing.T) *items.Catalog {
	t.Helper()
	catalog, err := items.Load("../../data/items.json")
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestStartingStateAndDerivedEquipmentStats(t *testing.T) {
	catalog := testCatalog(t)
	state, err := NewDefault(catalog)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := state.Snapshot(catalog)
	if snapshot.Stats.HP != 100 || snapshot.Stats.MaxHP != 100 || snapshot.Stats.ATK != 8 || snapshot.Stats.DEF != 1 || snapshot.Gold != 20 || snapshot.Score != 0 {
		t.Fatalf("unexpected starting snapshot: %+v", snapshot)
	}
	if err := state.Add("iron_sword", 1, catalog); err != nil {
		t.Fatal(err)
	}
	if err := state.Equip("iron_sword", "weapon", catalog); err != nil {
		t.Fatal(err)
	}
	snapshot = state.Snapshot(catalog)
	if snapshot.Stats.ATK != 10 || snapshot.Equipment.Weapon != "iron_sword" {
		t.Fatalf("equipment stats were not derived: %+v", snapshot)
	}
	if len(snapshot.Inventory) != 2 {
		t.Fatalf("old sword should return to backpack: %+v", snapshot.Inventory)
	}
	if err := state.Equip("health_potion", "weapon", catalog); err != ErrInvalidSlot {
		t.Fatalf("consumable equip result = %v, want %v", err, ErrInvalidSlot)
	}
}

func TestStackingAndPotionUse(t *testing.T) {
	catalog := testCatalog(t)
	state, err := NewDefault(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Add("health_potion", 8, catalog); err != nil {
		t.Fatal(err)
	}
	if got := state.Inventory[0].Quantity; got != 5 {
		t.Fatalf("first stack quantity = %d, want 5", got)
	}
	if got := state.Inventory[1].Quantity; got != 4 {
		t.Fatalf("second stack quantity = %d, want 4", got)
	}
	if err := state.Use("health_potion", "dungeon", catalog); err != ErrItemNotUsable {
		t.Fatalf("full HP potion result = %v, want %v", err, ErrItemNotUsable)
	}
	state.HP = 85
	if err := state.Use("health_potion", "dungeon", catalog); err != nil {
		t.Fatal(err)
	}
	if state.HP != 100 || state.Inventory[0].Quantity != 4 {
		t.Fatalf("potion did not heal and consume one: hp=%d inventory=%+v", state.HP, state.Inventory)
	}
	if err := state.Use("health_potion", "lobby", catalog); err != ErrWrongPhase {
		t.Fatalf("lobby use result = %v, want %v", err, ErrWrongPhase)
	}
}

func TestBackpackCapacityDoesNotPartiallyMutate(t *testing.T) {
	definitions := make([]items.Definition, BackpackCapacity+1)
	for i := range definitions {
		definitions[i] = items.Definition{ID: string(rune('a' + i)), Name: "Token", Type: items.Consumable, Rarity: "common", Stats: items.Stats{}, Effect: &items.Effect{Type: "heal", Value: 1}, Stackable: false, MaxStack: 1}
	}
	catalog, err := items.NewCatalog(definitions)
	if err != nil {
		t.Fatal(err)
	}
	state := &State{}
	for i := 0; i < BackpackCapacity; i++ {
		if err := state.Add(definitions[i].ID, 1, catalog); err != nil {
			t.Fatal(err)
		}
	}
	before := append([]Stack(nil), state.Inventory...)
	if err := state.Add(definitions[BackpackCapacity].ID, 1, catalog); err != ErrInventoryFull {
		t.Fatalf("capacity result = %v, want %v", err, ErrInventoryFull)
	}
	if len(state.Inventory) != len(before) {
		t.Fatal("failed add changed backpack size")
	}
	for i := range before {
		if state.Inventory[i] != before[i] {
			t.Fatal("failed add partially changed inventory")
		}
	}
}

func TestKitsAndLevelUp(t *testing.T) {
	catalog := testCatalog(t)
	for _, kit := range []string{"knight", "mage", "rogue", "ranger"} {
		state, err := NewWithKit(catalog, kit)
		if err != nil {
			t.Fatalf("failed to create kit %s: %v", kit, err)
		}
		snap := state.Snapshot(catalog)
		if snap.Stats.Level != 1 || snap.Stats.Archetype == "" {
			t.Fatalf("invalid kit snapshot: %+v", snap)
		}
	}
	mage, _ := NewWithKit(catalog, "mage")
	leveled := mage.GainEXP(35, catalog)
	if !leveled || mage.Level != 2 {
		t.Fatalf("expected level 2, got level %d", mage.Level)
	}
	snap := mage.Snapshot(catalog)
	if snap.Stats.Level != 2 || snap.Stats.BaseATK != 12 {
		t.Fatalf("stats did not grow on level up: %+v", snap)
	}
}
