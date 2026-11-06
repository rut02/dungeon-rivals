package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fasthttp/websocket"

	"dungeon-rivals/backend/internal/combat"
	"dungeon-rivals/backend/internal/config"
	"dungeon-rivals/backend/internal/dungeon"
	"dungeon-rivals/backend/internal/items"
	"dungeon-rivals/backend/internal/rooms"
)

type e2eEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

type e2eClient struct {
	conn     *websocket.Conn
	mu       sync.RWMutex
	latest   rooms.Snapshot
	updates  chan rooms.Snapshot
	accepted chan string
	errors   chan e2eCommandError
}

type e2eCommandError struct {
	RequestID string
	Code      string
}

var e2eRequestCounter atomic.Uint64

func TestThreeFloorMatchOverTwoWebSocketClients(t *testing.T) {
	itemCatalog, err := items.Load("../../data/items.json")
	if err != nil {
		t.Fatal(err)
	}
	dungeonConfig, err := dungeon.Load("../../data/dungeon.json")
	if err != nil {
		t.Fatal(err)
	}
	gameConfig, err := config.Load("../../config/game.json")
	if err != nil {
		t.Fatal(err)
	}
	monsters := &combat.Catalog{Monsters: []combat.Monster{
		{ID: "test_goblin", Name: "Test Goblin", BaseHP: 1, GoldMin: 1, GoldMax: 2, ScoreReward: 10, Tier: "normal"},
		{ID: "test_elite", Name: "Test Elite", BaseHP: 1, GoldMin: 2, GoldMax: 3, ScoreReward: 25, Tier: "elite"},
		{ID: "test_boss", Name: "Test Boss", BaseHP: 1, GoldMin: 3, GoldMax: 4, ScoreReward: 50, Tier: "boss", Floor: 1},
		{ID: "test_boss_two", Name: "Test Boss Two", BaseHP: 1, GoldMin: 3, GoldMax: 4, ScoreReward: 50, Tier: "boss", Floor: 2},
		{ID: "test_boss_three", Name: "Test Boss Three", BaseHP: 1, GoldMin: 3, GoldMax: 4, ScoreReward: 50, Tier: "boss", Floor: 3},
	}}
	manager := rooms.NewManagerWithAllGameData(gameConfig.Room.MinPlayers, gameConfig.Room.MaxPlayers, itemCatalog, dungeonConfig, monsters)
	app := newApp(manager, gameConfig, itemCatalog)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable in this environment: %v", err)
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- app.Listener(listener) }()
	defer func() {
		_ = app.Shutdown()
		_ = listener.Close()
		select {
		case <-serverErr:
		case <-time.After(time.Second):
		}
	}()
	baseURL := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 5 * time.Second}
	var host, guest rooms.SessionResponse
	postSession := func(path, name string, into *rooms.SessionResponse) {
		request, _ := http.NewRequest(http.MethodPost, baseURL+path, strings.NewReader(fmt.Sprintf(`{"player_name":%q}`, name)))
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("POST %s status=%d", path, response.StatusCode)
		}
		if err := json.NewDecoder(response.Body).Decode(into); err != nil {
			t.Fatal(err)
		}
	}
	postSession("/api/v1/rooms", "Host", &host)
	postSession("/api/v1/rooms/"+host.RoomCode+"/join", "Guest", &guest)
	hostClient := connectE2EClient(t, listener.Addr().String(), host)
	defer hostClient.conn.Close()
	guestClient := connectE2EClient(t, listener.Addr().String(), guest)
	defer guestClient.conn.Close()
	if err := hostClient.command("lobby.ready", map[string]any{"ready": true}); err != nil {
		t.Fatal(err)
	}
	if err := guestClient.command("lobby.ready", map[string]any{"ready": true}); err != nil {
		t.Fatal(err)
	}
	if err := hostClient.command("game.start", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if err := playE2EDungeon(hostClient); err != nil {
		t.Fatalf("host dungeon run: %v", err)
	}
	if err := hostClient.command("preparation.ready", map[string]any{"ready": true}); err != nil {
		t.Fatal(err)
	}
	if err := playE2EDungeon(guestClient); err != nil {
		t.Fatalf("guest dungeon run: %v", err)
	}
	if err := guestClient.command("preparation.ready", map[string]any{"ready": true}); err != nil {
		t.Fatal(err)
	}
	if phase := guestClient.snapshot().Room.Phase; phase != "pvp" {
		t.Fatalf("phase after everyone readied=%q, want pvp", phase)
	}
	for step := 0; step < 200; step++ {
		state := hostClient.snapshot()
		if guestState := guestClient.snapshot(); guestState.Version > state.Version {
			state = guestState
		}
		if state.Room.Phase == "results" {
			break
		}
		turnID := ""
		if state.Me.Private != nil {
			turnID = state.Me.Private.PvPCurrentTurnID
		}
		client := hostClient
		if turnID == guest.PlayerID {
			client = guestClient
		}
		if turnID == "" {
			t.Fatalf("PvP snapshot has no current turn: %+v", state.Me.Private)
		}
		if err := client.command("pvp.action", map[string]any{"action": "ATTACK"}); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			latest := hostClient.snapshot()
			if guestState := guestClient.snapshot(); guestState.Version > latest.Version {
				latest = guestState
			}
			if latest.Version > state.Version {
				break
			}
			time.Sleep(time.Millisecond)
		}
	}
	var results rooms.Snapshot
	for waitResult := 0; waitResult < 50; waitResult++ {
		results = hostClient.snapshot()
		if results.Room.Phase == "results" && len(results.Results) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if results.Room.Phase != "results" || len(results.Results) != 2 {
		t.Fatalf("final snapshot: phase=%s results=%+v", results.Room.Phase, results.Results)
	}
	csvResponse, err := client.Get(baseURL + "/api/v1/rooms/" + host.RoomCode + "/results.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer csvResponse.Body.Close()
	if csvResponse.StatusCode != http.StatusOK || !strings.HasPrefix(csvResponse.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("CSV response status=%d content-type=%q", csvResponse.StatusCode, csvResponse.Header.Get("Content-Type"))
	}
}

func connectE2EClient(t *testing.T, address string, session rooms.SessionResponse) *e2eClient {
	t.Helper()
	connection, _, err := websocket.DefaultDialer.Dial("ws://"+address+session.WSPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &e2eClient{conn: connection, updates: make(chan rooms.Snapshot, 2048), accepted: make(chan string, 64), errors: make(chan e2eCommandError, 64)}
	if err := connection.WriteJSON(map[string]any{"type": "session.authenticate", "request_id": "auth-" + session.PlayerID, "payload": map[string]any{"room_code": session.RoomCode, "player_id": session.PlayerID, "reconnect_token": session.ReconnectToken}}); err != nil {
		t.Fatal(err)
	}
	var authenticated e2eEnvelope
	_ = connection.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := connection.ReadJSON(&authenticated); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Time{})
	if authenticated.Type != "session.authenticated" {
		t.Fatalf("authentication response=%s", authenticated.Type)
	}
	go client.readLoop()
	client.waitInitial(t)
	return client
}

func (c *e2eClient) readLoop() {
	for {
		var message e2eEnvelope
		if err := c.conn.ReadJSON(&message); err != nil {
			return
		}
		switch message.Type {
		case "room.snapshot":
			var snapshot rooms.Snapshot
			if json.Unmarshal(message.Payload, &snapshot) == nil {
				c.mu.Lock()
				c.latest = snapshot
				c.mu.Unlock()
				select {
				case c.updates <- snapshot:
				default:
				}
			}
		case "command.error":
			var payload struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(message.Payload, &payload)
			select {
			case c.errors <- e2eCommandError{RequestID: message.RequestID, Code: payload.Code}:
			default:
			}
		case "command.accepted":
			select {
			case c.accepted <- message.RequestID:
			default:
			}
		}
	}
}
func (c *e2eClient) snapshot() rooms.Snapshot { c.mu.RLock(); defer c.mu.RUnlock(); return c.latest }
func (c *e2eClient) waitInitial(t *testing.T) {
	t.Helper()
	select {
	case <-c.updates:
	case <-time.After(5 * time.Second):
		t.Fatal("no initial room snapshot")
	}
}
func (c *e2eClient) command(kind string, payload map[string]any) error {
	requestID := fmt.Sprintf("e2e-%d", e2eRequestCounter.Add(1))
	if err := c.conn.WriteJSON(map[string]any{"type": kind, "request_id": requestID, "payload": payload}); err != nil {
		return err
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case id := <-c.accepted:
			if id == requestID {
				return nil
			}
		case response := <-c.errors:
			if response.RequestID == requestID {
				return fmt.Errorf("%s failed: %s", kind, response.Code)
			}
		case <-deadline.C:
			return fmt.Errorf("%s was not acknowledged", kind)
		}
	}
}

func playE2EDungeon(client *e2eClient) error {
	shopVisited := map[string]bool{}
	for steps := 0; steps < 400; steps++ {
		snapshot := client.snapshot()
		private := snapshot.Me.Private
		if private == nil || private.Dungeon == nil {
			return fmt.Errorf("player has no private dungeon snapshot")
		}
		if private.Dungeon.Completed {
			return nil
		}
		if private.Combat != nil && !private.Combat.Finished {
			if err := client.command("combat.action", map[string]any{"action": "ATTACK"}); err != nil {
				return err
			}
			continue
		}
		if private.Activity != nil {
			switch private.Activity.Type {
			case "SHOP":
				nodeID := private.Dungeon.ActiveNodeID
				bought := false
				if !shopVisited[nodeID] {
					shopVisited[nodeID] = true
					for _, stock := range private.Activity.Stock {
						if stock.Available && stock.Price <= private.Gold {
							if err := client.command("room.action", map[string]any{"action": "buy", "item_id": stock.ItemID}); err != nil {
								if strings.Contains(err.Error(), "INVENTORY_FULL") {
									break
								}
								return err
							}
							bought = true
							break
						}
					}
				}
				if bought {
					continue
				}
				if err := client.command("room.action", map[string]any{"action": "leave"}); err != nil {
					return err
				}
			case "TREASURE", "EVENT", "PUZZLE", "FORGE":
				if len(private.Activity.Options) == 0 {
					return fmt.Errorf("%s room has no choices", private.Activity.Type)
				}
				if err := client.command("room.action", map[string]any{"action": "claim", "choice_id": private.Activity.Options[0].ID}); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported room activity %q", private.Activity.Type)
			}
			continue
		}
		var availableID string
		for _, floor := range private.Dungeon.Floors {
			if floor.Number != private.Dungeon.CurrentFloor {
				continue
			}
			var connections []string
			for _, node := range floor.Nodes {
				if node.ID == private.Dungeon.CurrentNodeID {
					connections = node.Connections
					break
				}
			}
			for _, connectionID := range connections {
				for _, node := range floor.Nodes {
					if node.ID == connectionID && node.Status == "AVAILABLE" {
						availableID = node.ID
						break
					}
				}
				if availableID != "" {
					break
				}
			}
		}
		if availableID == "" {
			return fmt.Errorf("no available node on floor %d", private.Dungeon.CurrentFloor)
		}
		if err := client.command("dungeon.move", map[string]any{"node_id": availableID}); err != nil {
			return err
		}
	}
	return fmt.Errorf("dungeon run exceeded action limit")
}
