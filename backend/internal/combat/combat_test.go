package combat

import "testing"

func TestDamageIsMinimumOne(t *testing.T) {
	if Damage(10, 3) != 7 || Damage(3, 10) != 1 {
		t.Fatal("damage formula must subtract defense and deal at least one")
	}
}

func TestLoadMonsterDataAndScaleEncounterOnce(t *testing.T) {
	catalog, err := Load("../../data/monsters.json")
	if err != nil {
		t.Fatal(err)
	}
	state, monster, err := catalog.Create("BOSS", 2, 1.25, 123)
	if err != nil {
		t.Fatal(err)
	}
	if monster.Floor != 2 || monster.ID != "frost_colossus" {
		t.Fatalf("floor 2 boss = %+v", monster)
	}
	if state.MaxHP != 157 || state.Attack != 22 || state.Defense != 8 {
		t.Fatalf("scaled state = %+v", state)
	}
}
