package dungeon

import (
	"testing"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	cfg, err := Load("../../data/dungeon.json")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestGeneratedMapHasThreeConnectedFloorsAndRoomGuarantee(t *testing.T) {
	cfg := testConfig(t)
	state, err := Generate(12345, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Floors) != 3 {
		t.Fatalf("floor count = %d, want 3", len(state.Floors))
	}
	lastDepth := len(cfg.Depths) - 1
	for _, floor := range state.Floors {
		byID := map[string]Node{}
		for _, node := range floor.Nodes {
			byID[node.ID] = node
		}
		layers := make([][]Node, len(cfg.Depths))
		for _, node := range floor.Nodes {
			if node.Depth < 0 || node.Depth > lastDepth {
				t.Fatalf("invalid depth: %+v", node)
			}
			layers[node.Depth] = append(layers[node.Depth], node)
		}
		if len(layers[0]) != 1 || layers[0][0].Type != StartRoom {
			t.Fatalf("floor %d has invalid start layer", floor.Number)
		}
		if len(layers[lastDepth]) != 1 || layers[lastDepth][0].Type != BossRoom {
			t.Fatalf("floor %d must end with one boss", floor.Number)
		}
		hasRestOrTreasure := false
		for depth := 0; depth < lastDepth; depth++ {
			incoming := map[string]int{}
			for _, node := range layers[depth] {
				if len(node.Connections) == 0 {
					t.Fatalf("node %s has no next-depth connection", node.ID)
				}
				for _, targetID := range node.Connections {
					target, ok := byID[targetID]
					if !ok {
						t.Fatalf("node %s connects outside its floor", node.ID)
					}
					if target.Depth != depth+1 {
						t.Fatalf("connection skipped depth: %+v -> %+v", node, target)
					}
					incoming[targetID]++
				}
			}
			for _, node := range layers[depth+1] {
				if incoming[node.ID] == 0 {
					t.Fatalf("node %s has no incoming edge", node.ID)
				}
			}
		}
		for _, node := range layers[1:lastDepth] {
			for _, candidate := range node {
				if candidate.Type == RestRoom || candidate.Type == TreasureRoom {
					hasRestOrTreasure = true
				}
			}
		}
		if !hasRestOrTreasure {
			t.Fatalf("floor %d is missing guaranteed rest/treasure", floor.Number)
		}
	}
	first := state.Floors[0]
	if first.Nodes[0].Status != Cleared {
		t.Fatal("start node must be cleared")
	}
	for _, node := range state.Floors[1].Nodes {
		if node.Status != Locked {
			t.Fatal("future floors must start locked")
		}
	}
}

func TestMoveRequiresAnAvailableConnectedNode(t *testing.T) {
	state, err := Generate(90, testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	floor := state.currentFloor()
	startIndex := findNodeIndex(floor, state.CurrentNodeID)
	targetID := floor.Nodes[startIndex].Connections[0]
	if err := state.Move("not-a-node"); err != ErrInvalidNode {
		t.Fatalf("unknown node result = %v", err)
	}
	if err := state.Move(targetID); err != nil {
		t.Fatal(err)
	}
	if state.ActiveNodeID != targetID {
		t.Fatalf("active node = %s, want %s", state.ActiveNodeID, targetID)
	}
	if err := state.Move(targetID); err != ErrEncounterActive {
		t.Fatalf("double move result = %v, want %v", err, ErrEncounterActive)
	}
}
