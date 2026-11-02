package items

import "testing"

func TestLoadCatalogAndRejectBrokenDefinitions(t *testing.T) {
	catalog, err := Load("../../data/items.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.Get("training_sword"); !ok {
		t.Fatal("training sword should be in the catalog")
	}
	if _, ok := catalog.Get("health_potion"); !ok {
		t.Fatal("health potion should be in the catalog")
	}
	if _, err := NewCatalog([]Definition{{ID: "broken", Name: "Broken", Type: Weapon, Rarity: "common", Slot: "armor", MaxStack: 1}}); err == nil {
		t.Fatal("expected mismatched equipment slot to be rejected")
	}
}
