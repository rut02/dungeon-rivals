package rooms

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"dungeon-rivals/backend/internal/combat"
	"dungeon-rivals/backend/internal/dungeon"
	"dungeon-rivals/backend/internal/items"
	"dungeon-rivals/backend/internal/playerstate"
)

func managerWithTestItems(t *testing.T) *Manager {
	t.Helper()
	catalog, err := items.Load("../../data/items.json")
	if err != nil {
		t.Fatal(err)
	}
	dungeonConfig, err := dungeon.Load("../../data/dungeon.json")
	if err != nil {
		t.Fatal(err)
	}
	return NewManagerWithGameData(2, 4, catalog, dungeonConfig)
}

func managerWithCombat(t *testing.T) *Manager {
	t.Helper()
	m := managerWithTestItems(t)
	monsters, err := combat.Load("../../data/monsters.json")
	if err != nil {
		t.Fatal(err)
	}
	m.monsters = monsters
	return m
}

func TestRoomLifecycle(t *testing.T) {
	m := managerWithTestItems(t)
	host, err := m.Create(" Host ")
	if err != nil {
		t.Fatal(err)
	}
	if host.RoomCode == "" || !host.IsHost || host.PlayerID == "" || host.ReconnectToken == "" {
		t.Fatalf("incomplete host session: %+v", host)
	}
	if err := m.start(m.rooms[host.RoomCode], host.PlayerID); err != ErrNotReady {
		t.Fatalf("start with one player: got %v, want %v", err, ErrNotReady)
	}
	guest, err := m.Join(host.RoomCode, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	if guest.IsHost {
		t.Fatal("joined player must not be host")
	}
	r := m.rooms[host.RoomCode]
	if err := m.start(r, guest.PlayerID); err != ErrNotHost {
		t.Fatalf("guest start: got %v, want %v", err, ErrNotHost)
	}
	if err := m.setReady(r, host.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	if err := m.start(r, host.PlayerID); err != ErrNotReady {
		t.Fatalf("start with guest unready: got %v, want %v", err, ErrNotReady)
	}
	if err := m.setReady(r, guest.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	if r.Phase != "dungeon" {
		t.Fatalf("phase = %q, want dungeon", r.Phase)
	}
}

func TestRoomLimitAndTokenValidation(t *testing.T) {
	m := NewManager()
	host, _ := m.Create("A")
	_, _ = m.Join(host.RoomCode, "B")
	_, _ = m.Join(host.RoomCode, "C")
	_, _ = m.Join(host.RoomCode, "D")
	if _, err := m.Join(host.RoomCode, "E"); err != ErrRoomFull {
		t.Fatalf("join full room: got %v, want %v", err, ErrRoomFull)
	}
	if _, _, err := m.authenticate(host.RoomCode, host.PlayerID, "wrong", nil); err != ErrInvalidToken {
		t.Fatalf("bad token: got %v, want %v", err, ErrInvalidToken)
	}
}

func TestPlayerNameIsRequired(t *testing.T) {
	if _, err := NewManager().Create("  "); err != ErrInvalidPlayerName {
		t.Fatalf("empty name: got %v", err)
	}
}

func TestRequestIDValidationAndDuplicateProtection(t *testing.T) {
	m := NewManager()
	session, err := m.Create("Player")
	if err != nil {
		t.Fatal(err)
	}
	r := m.rooms[session.RoomCode]
	if err := m.ClaimRequestID(r, session.PlayerID, ""); err != ErrInvalidRequestID {
		t.Fatalf("empty ID: got %v, want %v", err, ErrInvalidRequestID)
	}
	if err := m.ClaimRequestID(r, session.PlayerID, "ready-1"); err != nil {
		t.Fatal(err)
	}
	if err := m.ClaimRequestID(r, session.PlayerID, "ready-1"); err != ErrDuplicateRequest {
		t.Fatalf("duplicate ID: got %v, want %v", err, ErrDuplicateRequest)
	}
	for i := 0; i < maxRequestHistory; i++ {
		if err := m.ClaimRequestID(r, session.PlayerID, fmt.Sprintf("bounded-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if len(findPlayer(r, session.PlayerID).RequestIDs) != maxRequestHistory {
		t.Fatalf("request history grew beyond bound: %d", len(findPlayer(r, session.PlayerID).RequestIDs))
	}
	if err := m.ClaimRequestID(r, session.PlayerID, "bounded-4095"); err != ErrDuplicateRequest {
		t.Fatalf("recent duplicate after eviction window: %v", err)
	}
}

func TestStartedRoomSnapshotsContainOnlyRecipientInventory(t *testing.T) {
	m := managerWithTestItems(t)
	host, err := m.Create("Host")
	if err != nil {
		t.Fatal(err)
	}
	guest, err := m.Join(host.RoomCode, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	r := m.rooms[host.RoomCode]
	if err = m.setReady(r, host.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	if err = m.setReady(r, guest.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	if err = m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	hostSnapshot := m.snapshotLocked(r, host.PlayerID)
	guestSnapshot := m.snapshotLocked(r, guest.PlayerID)
	if hostSnapshot.Me.Private == nil || guestSnapshot.Me.Private == nil {
		t.Fatal("each player must receive their own private state")
	}
	if hostSnapshot.Me.Private.Stats.ATK != 8 || hostSnapshot.Me.Private.Stats.DEF != 1 || hostSnapshot.Me.Private.Gold != 20 {
		t.Fatalf("unexpected starting stats: %+v", hostSnapshot.Me.Private)
	}
	if len(hostSnapshot.Me.Private.Inventory) != 1 || hostSnapshot.Me.Private.Inventory[0] != (playerstate.Stack{ItemID: "health_potion", Quantity: 1}) {
		t.Fatalf("unexpected starting inventory: %+v", hostSnapshot.Me.Private.Inventory)
	}
	if len(guestSnapshot.Me.Private.Inventory) != 1 || guestSnapshot.Me.Private.Inventory[0].ItemID != "health_potion" {
		t.Fatalf("guest private inventory is wrong: %+v", guestSnapshot.Me.Private.Inventory)
	}
	if hostSnapshot.Me.Private.Dungeon == nil || guestSnapshot.Me.Private.Dungeon == nil {
		t.Fatal("dungeon map must be included in each player's private snapshot")
	}
	if findPlayer(r, host.PlayerID).Dungeon.Seed == findPlayer(r, guest.PlayerID).Dungeon.Seed {
		t.Fatal("each player must receive a different server-generated map seed")
	}
}

func TestDungeonMoveIsIndependentPerPlayer(t *testing.T) {
	m := managerWithTestItems(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	hostDungeon := findPlayer(r, host.PlayerID).Dungeon
	start := hostDungeon.Floors[0].Nodes[0]
	if err := m.ServeCommand(r, host.PlayerID, nil, "dungeon.move", map[string]any{"node_id": start.Connections[0]}); err != nil {
		t.Fatal(err)
	}
	if hostDungeon.ActiveNodeID == "" {
		t.Fatal("host move did not enter an encounter node")
	}
	if findPlayer(r, guest.PlayerID).Dungeon.ActiveNodeID != "" {
		t.Fatal("host move changed the guest dungeon state")
	}
	guestSnapshot := m.snapshotLocked(r, guest.PlayerID)
	if guestSnapshot.Me.Private.Dungeon.ActiveNodeID != "" {
		t.Fatal("guest private snapshot contains the host's current node")
	}
}

func TestCombatRewardsAndRespawnRules(t *testing.T) {
	m := managerWithCombat(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	p := findPlayer(r, host.PlayerID)
	floor := &p.Dungeon.Floors[0]
	start := floor.Nodes[0]
	targetID := start.Connections[0]
	for i := range floor.Nodes {
		if floor.Nodes[i].ID == targetID {
			floor.Nodes[i].Type = dungeon.MonsterRoom
		}
	}
	if err := m.moveDungeon(r, host.PlayerID, targetID); err != nil {
		t.Fatal(err)
	}
	if p.Combat == nil {
		t.Fatal("combat encounter did not start")
	}
	p.Combat.HP = 1
	for i := 0; i < 1; i++ {
		if err := m.combatAction(r, host.PlayerID, "ATTACK", ""); err != nil {
			t.Fatal(err)
		}
	}
	if !p.Combat.Finished || p.Dungeon.ActiveNodeID != "" || p.State.Score != 10 || p.State.Gold < 24 {
		t.Fatalf("victory not resolved: combat=%+v score=%d gold=%d map=%+v", p.Combat, p.State.Score, p.State.Gold, p.Dungeon)
	}
	if floor.Nodes[1].Status != dungeon.Cleared && floor.Nodes[2].Status != dungeon.Cleared { // identify exact target below
		found := false
		for _, node := range floor.Nodes {
			if node.ID == targetID && node.Status == dungeon.Cleared {
				found = true
			}
		}
		if !found {
			t.Fatalf("victory node %s not cleared", targetID)
		}
	}
}

func TestPvEDeathRespawnsWithoutRewardAndReopensNode(t *testing.T) {
	m := managerWithCombat(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	p := findPlayer(r, host.PlayerID)
	floor := &p.Dungeon.Floors[0]
	targetID := floor.Nodes[0].Connections[0]
	for i := range floor.Nodes {
		if floor.Nodes[i].ID == targetID {
			floor.Nodes[i].Type = dungeon.MonsterRoom
		}
	}
	if err := m.moveDungeon(r, host.PlayerID, targetID); err != nil {
		t.Fatal(err)
	}
	p.Combat.Attack = 200
	p.State.HP = 1
	if err := m.combatAction(r, host.PlayerID, "ATTACK", ""); err != nil {
		t.Fatal(err)
	}
	if p.State.Deaths != 1 || p.State.HP != 50 || p.State.Gold != 16 || p.State.Score != 0 || p.Dungeon.ActiveNodeID != "" || p.Dungeon.CurrentNodeID != p.Dungeon.CheckpointNodeID {
		t.Fatalf("respawn state invalid: player=%+v dungeon=%+v", p.State, p.Dungeon)
	}
	index := 0
	for i, node := range floor.Nodes {
		if node.ID == targetID {
			index = i
		}
	}
	if floor.Nodes[index].Status != dungeon.Available {
		t.Fatalf("defeated node status=%s", floor.Nodes[index].Status)
	}
}

func TestTreasureEventShopAndRestRoomsResolve(t *testing.T) {
	m := managerWithCombat(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	p := findPlayer(r, host.PlayerID)
	enter := func(kind dungeon.RoomType) string {
		floor := &p.Dungeon.Floors[p.Dungeon.CurrentFloor-1]
		current := 0
		for i, n := range floor.Nodes {
			if n.ID == p.Dungeon.CurrentNodeID {
				current = i
			}
		}
		id := floor.Nodes[current].Connections[0]
		for i := range floor.Nodes {
			if floor.Nodes[i].ID == id {
				floor.Nodes[i].Type = kind
			}
		}
		if err := m.moveDungeon(r, host.PlayerID, id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	treasureID := enter(dungeon.TreasureRoom)
	if err := m.roomAction(r, host.PlayerID, "claim", "health_potion", ""); err != nil {
		t.Fatal(err)
	}
	if p.State.Score != 5 || p.Dungeon.ActiveNodeID != "" {
		t.Fatalf("treasure did not resolve: score=%d map=%+v", p.State.Score, p.Dungeon)
	}
	_ = treasureID
	enter(dungeon.EventRoom)
	if err := m.roomAction(r, host.PlayerID, "claim", "pay", ""); err != nil {
		t.Fatal(err)
	}
	if p.State.Score != 15 || p.State.Gold != 12 {
		t.Fatalf("event did not apply: score=%d gold=%d", p.State.Score, p.State.Gold)
	}
	enter(dungeon.ShopRoom)
	p.State.Gold = 100
	p.State.Inventory = make([]playerstate.Stack, playerstate.BackpackCapacity)
	for i := range p.State.Inventory {
		p.State.Inventory[i] = playerstate.Stack{ItemID: "training_sword", Quantity: 1}
	}
	if err := m.roomAction(r, host.PlayerID, "buy", "", "bronze_ring"); err != playerstate.ErrInventoryFull || p.State.Gold != 100 {
		t.Fatalf("full backpack purchase was not atomic: err=%v gold=%d", err, p.State.Gold)
	}
	p.State.Inventory = nil
	if err := m.roomAction(r, host.PlayerID, "buy", "", "bronze_ring"); err != nil {
		t.Fatal(err)
	}
	if p.State.Gold != 82 || len(p.State.Inventory) != 1 {
		t.Fatalf("shop purchase mismatch: gold=%d inv=%+v", p.State.Gold, p.State.Inventory)
	}
	if err := m.roomAction(r, host.PlayerID, "leave", "", ""); err != nil {
		t.Fatal(err)
	}
	enter(dungeon.RestRoom)
	if p.Dungeon.ActiveNodeID != "" || p.Activity != nil {
		t.Fatal("rest room must resolve immediately")
	}
}

func TestPreparationWaitsForDungeonCompletionAndAllReady(t *testing.T) {
	m := managerWithCombat(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	h := findPlayer(r, host.PlayerID)
	g := findPlayer(r, guest.PlayerID)
	h.Dungeon.Completed = true
	h.State.HP = 10
	if err := m.setPreparationReady(r, host.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	if r.Phase != "dungeon" {
		t.Fatalf("early preparation changed room phase: %s", r.Phase)
	}
	g.Dungeon.Completed = true
	m.maybePreparation(r)
	if r.Phase != "preparation" {
		t.Fatalf("phase=%s want preparation", r.Phase)
	}
	if err := m.setPreparationReady(r, guest.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	if r.Phase != "pvp" {
		t.Fatalf("phase=%s want pvp", r.Phase)
	}
	if h.State.HP != h.State.Stats(m.catalog).MaxHP {
		t.Fatalf("HP not restored for PvP: %d", h.State.HP)
	}
}

func TestArenaBudgetRespawnScoreAndRanking(t *testing.T) {
	m := managerWithCombat(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	h := findPlayer(r, host.PlayerID)
	g := findPlayer(r, guest.PlayerID)
	h.Dungeon.Completed = true
	g.Dungeon.Completed = true
	h.State.Score = 10
	g.State.Score = 5
	h.State.Gold = 20
	g.State.Gold = 20
	if err := m.setPreparationReady(r, host.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	m.maybePreparation(r)
	if err := m.setPreparationReady(r, guest.PlayerID, true); err != nil {
		t.Fatal(err)
	}
	if r.Phase != "pvp" {
		t.Fatalf("phase=%s want pvp", r.Phase)
	}
	_ = h.State.Add("fire_bomb", 1, m.catalog)
	if err := m.pvpAction(r, host.PlayerID, "USE_ITEM", guest.PlayerID, "fire_bomb"); err != nil {
		t.Fatal(err)
	}
	if err := m.pvpAction(r, host.PlayerID, "SKILL", guest.PlayerID, ""); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 20 && r.Phase == "pvp"; step++ {
		if err := m.pvpAction(r, host.PlayerID, "ATTACK", guest.PlayerID, ""); err != nil {
			t.Fatal(err)
		}
	}
	if h.PvPKnockouts != 1 || h.PvPWins != 1 || g.PvPDeaths != 1 || g.PvPLosses != 1 || g.PvPPenalty != 10 {
		t.Fatalf("PvP state mismatch: host=%+v guest=%+v", h, g)
	}
	if r.Phase != "results" || len(r.Results) != 2 {
		t.Fatalf("results missing: phase=%s results=%+v", r.Phase, r.Results)
	}
	if r.Results[0].PlayerID != host.PlayerID || r.Results[0].Rank != 1 || r.Results[1].Rank != 2 {
		t.Fatalf("wrong ranking/score breakdown: %+v", r.Results)
	}
	csvData, err := m.ResultsCSV(host.RoomCode)
	if err != nil || !strings.Contains(string(csvData), "total_score") || !strings.Contains(string(csvData), "Host") {
		t.Fatalf("CSV export missing scoreboard: %q, %v", csvData, err)
	}
}

func TestHostTransfersAndCanRemoveDisconnectedPlayer(t *testing.T) {
	m := managerWithTestItems(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	findPlayer(r, guest.PlayerID).Connected = true
	m.disconnect(r, host.PlayerID, nil, false)
	if r.HostID != guest.PlayerID {
		t.Fatalf("host did not transfer to connected player: %s", r.HostID)
	}
	if err := m.kickDisconnected(r, guest.PlayerID, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	if len(r.Players) != 1 || r.Players[0].ID != guest.PlayerID {
		t.Fatalf("disconnected player was not removed: %+v", r.Players)
	}
}

func TestConcurrentDuplicateRequestIDIsClaimedOnce(t *testing.T) {
	m := NewManager()
	session, _ := m.Create("Player")
	r := m.rooms[session.RoomCode]
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.ClaimRequestID(r, session.PlayerID, "one-request"); err == nil {
				accepted.Add(1)
			} else if err != ErrDuplicateRequest {
				t.Errorf("claim returned unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("request ID accepted %d times", accepted.Load())
	}
}

func TestServerValidatesEquipmentAndUseCommands(t *testing.T) {
	m := managerWithTestItems(t)
	host, _ := m.Create("Host")
	guest, _ := m.Join(host.RoomCode, "Guest")
	r := m.rooms[host.RoomCode]
	_ = m.setReady(r, host.PlayerID, true)
	_ = m.setReady(r, guest.PlayerID, true)
	if err := m.start(r, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	state := findPlayer(r, host.PlayerID).State
	if err := state.Add("iron_sword", 1, m.catalog); err != nil {
		t.Fatal(err)
	}
	if err := m.ServeCommand(r, host.PlayerID, nil, "inventory.equip", map[string]any{"item_id": "iron_sword", "slot": "weapon"}); err != nil {
		t.Fatal(err)
	}
	if got := state.Snapshot(m.catalog).Stats.ATK; got != 10 {
		t.Fatalf("ATK after equip = %d, want 10", got)
	}
	if err := m.ServeCommand(r, host.PlayerID, nil, "inventory.equip", map[string]any{"item_id": "fake_sword", "slot": "weapon"}); err != playerstate.ErrItemNotFound {
		t.Fatalf("fake item result = %v, want %v", err, playerstate.ErrItemNotFound)
	}
	state.HP = 85
	if err := m.ServeCommand(r, host.PlayerID, nil, "inventory.use", map[string]any{"item_id": "health_potion"}); err != nil {
		t.Fatal(err)
	}
	if state.HP != 100 || len(state.Inventory) != 1 || state.Inventory[0].ItemID != "training_sword" {
		t.Fatalf("potion resolution mismatch: hp=%d inventory=%+v", state.HP, state.Inventory)
	}
}


func TestBotLobbyDungeonAndPvPFlow(t *testing.T) {
	m := managerWithTestItems(t)
	host, err := m.Create("SoloHero")
	if err != nil {
		t.Fatal(err)
	}
	r := m.rooms[host.RoomCode]

	// 1. Add Bot with difficulty and kit
	if err := m.ServeCommand(r, host.PlayerID, nil, "lobby.add_bot", map[string]any{"difficulty": "hard", "kit": "knight"}); err != nil {
		t.Fatalf("failed to add bot: %v", err)
	}
	if len(r.Players) != 2 {
		t.Fatalf("player count want 2, got %d", len(r.Players))
	}
	bot := r.Players[1]
	if !bot.IsBot || bot.BotDifficulty != "hard" || !bot.Ready {
		t.Fatalf("bot fields mismatch: is_bot=%v diff=%s ready=%v", bot.IsBot, bot.BotDifficulty, bot.Ready)
	}

	// 2. Remove bot test
	if err := m.ServeCommand(r, host.PlayerID, nil, "lobby.remove_bot", map[string]any{"bot_player_id": bot.ID}); err != nil {
		t.Fatalf("failed to remove bot: %v", err)
	}
	if len(r.Players) != 1 {
		t.Fatalf("player count after remove want 1, got %d", len(r.Players))
	}

	// 3. Re-add bot and start game
	if err := m.ServeCommand(r, host.PlayerID, nil, "lobby.add_bot", map[string]any{"difficulty": "nightmare", "kit": "mage"}); err != nil {
		t.Fatalf("failed to add bot: %v", err)
	}
	bot = r.Players[1]
	_ = m.Ready(r, host.PlayerID, true)
	if err := m.Start(r, host.PlayerID); err != nil {
		t.Fatalf("failed to start match with bot: %v", err)
	}
	if r.Phase != "dungeon" {
		t.Fatalf("phase want dungeon, got %s", r.Phase)
	}
	if bot.State == nil || bot.Dungeon == nil {
		t.Fatal("bot state or dungeon was not initialized")
	}

	// 4. Test simulateBotsProgress
	m.simulateBotsProgress(r)
	if bot.Dungeon.Progress == 0 {
		t.Fatal("bot progress did not advance")
	}

	// 5. Test player leaving mid-game doesn't cause INVALID_PHASE and cleans up room
	if err := m.ServeCommand(r, host.PlayerID, nil, "room.leave", nil); err != nil {
		t.Fatalf("leave during dungeon returned unexpected error: %v", err)
	}
	// Since host was the only human, room should be deleted
	if _, exists := m.rooms[host.RoomCode]; exists {
		t.Fatal("room should be cleaned up when all human players leave")
	}
}
