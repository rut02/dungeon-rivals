package rooms

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand"
	"strings"
	"sync"

	"dungeon-rivals/backend/internal/combat"
	"dungeon-rivals/backend/internal/dungeon"
	"dungeon-rivals/backend/internal/items"
	"dungeon-rivals/backend/internal/playerstate"
)

const minPlayers = 2
const maxPlayers = 4
const roomAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const maxRequestHistory = 4096

var (
	ErrInvalidPlayerName        = errors.New("INVALID_PLAYER_NAME")
	ErrRoomNotFound             = errors.New("ROOM_NOT_FOUND")
	ErrRoomFull                 = errors.New("ROOM_FULL")
	ErrRoomStarted              = errors.New("ROOM_ALREADY_STARTED")
	ErrPlayerNotFound           = errors.New("PLAYER_NOT_FOUND")
	ErrInvalidToken             = errors.New("INVALID_RECONNECT_TOKEN")
	ErrNotHost                  = errors.New("NOT_ROOM_HOST")
	ErrNotReady                 = errors.New("NOT_READY")
	ErrInvalidPhase             = errors.New("INVALID_PHASE")
	ErrDuplicateRequest         = errors.New("DUPLICATE_REQUEST")
	ErrInvalidRequestID         = errors.New("INVALID_REQUEST_ID")
	ErrItemCatalogUnavailable   = errors.New("ITEM_CATALOG_UNAVAILABLE")
	ErrDungeonConfigUnavailable = errors.New("DUNGEON_CONFIG_UNAVAILABLE")
)

type PlayerNameRequest struct {
	PlayerName string `json:"player_name"`
}

type SessionResponse struct {
	RoomCode       string `json:"room_code"`
	PlayerID       string `json:"player_id"`
	ReconnectToken string `json:"reconnect_token"`
	IsHost         bool   `json:"is_host"`
	WSPath         string `json:"ws_path"`
}

type Preview struct {
	RoomCode    string `json:"room_code"`
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	Phase       string `json:"phase"`
	Status      string `json:"status"`
}

type PlayerSnapshot struct {
	PlayerID            string `json:"player_id"`
	Name                string `json:"name"`
	IsHost              bool   `json:"is_host"`
	Ready               bool   `json:"ready"`
	Connected           bool   `json:"connected"`
	Kit                 string `json:"kit"`
	Stage               string `json:"stage"`
	Floor               int    `json:"floor"`
	Progress            int    `json:"progress"`
	Score               int    `json:"score"`
	Completed           bool   `json:"completed"`
	HP                  int    `json:"hp"`
	MaxHP               int    `json:"max_hp"`
	PvPActionsRemaining int    `json:"pvp_actions_remaining"`
	PvPScore            int    `json:"pvp_score"`
	PvPDamage           int    `json:"pvp_damage"`
	PvPKnockouts        int    `json:"pvp_knockouts"`
}

type Snapshot struct {
	Version uint64           `json:"version"`
	Room    RoomView         `json:"room"`
	Players []PlayerSnapshot `json:"players"`
	Me      SelfSnapshot     `json:"me"`
	Results []ResultEntry    `json:"results,omitempty"`
}

type RoomView struct {
	RoomCode     string `json:"room_code"`
	Phase        string `json:"phase"`
	HostPlayerID string `json:"host_player_id"`
	PlayerCount  int    `json:"player_count"`
	MinPlayers   int    `json:"min_players"`
	MaxPlayers   int    `json:"max_players"`
}

type SelfSnapshot struct {
	PlayerID  string           `json:"player_id"`
	Name      string           `json:"name"`
	IsHost    bool             `json:"is_host"`
	Ready     bool             `json:"ready"`
	Connected bool             `json:"connected"`
	Kit       string           `json:"kit"`
	Private   *PrivateSnapshot `json:"private,omitempty"`
}

type PrivateSnapshot struct {
	playerstate.PrivateSnapshot
	Dungeon             *dungeon.PrivateView `json:"dungeon,omitempty"`
	Combat              *combat.State        `json:"combat,omitempty"`
	Activity            *RoomActivity        `json:"activity,omitempty"`
	PvPActionsUsed      int                  `json:"pvp_actions_used,omitempty"`
	PvPActionsRemaining int                  `json:"pvp_actions_remaining,omitempty"`
	PvPScore            int                  `json:"pvp_score,omitempty"`
	PvPDamage           int                  `json:"pvp_damage,omitempty"`
	PvPKnockouts        int                  `json:"pvp_knockouts,omitempty"`
}

type ResultEntry struct {
	PlayerID       string `json:"player_id"`
	Name           string `json:"name"`
	Rank           int    `json:"rank"`
	TotalScore     int    `json:"total_score"`
	PvEScore       int    `json:"pve_score"`
	PvPScore       int    `json:"pvp_score"`
	GoldScore      int    `json:"gold_score"`
	Bonus          int    `json:"bonus"`
	Penalties      int    `json:"penalties"`
	PvPDamage      int    `json:"pvp_damage"`
	PvPKnockouts   int    `json:"pvp_knockouts"`
	PvPDeaths      int    `json:"pvp_deaths"`
	BossesDefeated int    `json:"bosses_defeated"`
	Gold           int    `json:"gold"`
}

type ScoreEvent struct {
	ID       string `json:"id"`
	PlayerID string `json:"player_id"`
	Category string `json:"category"`
	Source   string `json:"source"`
	Points   int    `json:"points"`
}

type player struct {
	ID, Name, Token   string
	Ready, Connected  bool
	Kit               string
	Conn              interface{ WriteJSON(any) error }
	RequestIDs        map[string]struct{}
	RequestOrder      []string
	State             *playerstate.State
	Dungeon           *dungeon.State
	Combat            *combat.State
	Activity          *RoomActivity
	PvPActionsUsed    int
	PvPScore          int
	PvPScoreRemainder int
	PvPDamage         int
	PvPKnockouts      int
	PvPDeaths         int
	PvPPenalty        int
	PvPBonus          int
	PvPGuarding       bool
	PveScore          int
	BossesDefeated    int
}

type room struct {
	Code, HostID, Phase string
	Version             uint64
	Players             []*player
	ScoreEventIDs       map[string]struct{}
	Results             []ResultEntry
	ScoreEvents         []ScoreEvent
}

type Manager struct {
	mu                     sync.Mutex
	rooms                  map[string]*room
	minPlayers, maxPlayers int
	catalog                *items.Catalog
	dungeonConfig          dungeon.Config
	monsters               *combat.Catalog
}

func NewManager() *Manager { return NewManagerWithLimits(minPlayers, maxPlayers) }
func NewManagerWithLimits(minimum, maximum int) *Manager {
	return NewManagerWithItems(minimum, maximum, nil)
}
func NewManagerWithItems(minimum, maximum int, catalog *items.Catalog) *Manager {
	return &Manager{rooms: make(map[string]*room), minPlayers: minimum, maxPlayers: maximum, catalog: catalog}
}
func NewManagerWithGameData(minimum, maximum int, catalog *items.Catalog, dungeonConfig dungeon.Config) *Manager {
	manager := NewManagerWithItems(minimum, maximum, catalog)
	manager.dungeonConfig = dungeonConfig
	return manager
}
func NewManagerWithAllGameData(minimum, maximum int, catalog *items.Catalog, dungeonConfig dungeon.Config, monsters *combat.Catalog) *Manager {
	m := NewManagerWithGameData(minimum, maximum, catalog, dungeonConfig)
	m.monsters = monsters
	return m
}

func (m *Manager) Create(name string) (SessionResponse, error) {
	name = cleanName(name)
	if name == "" {
		return SessionResponse{}, ErrInvalidPlayerName
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	code, err := m.newRoomCodeLocked()
	if err != nil {
		return SessionResponse{}, err
	}
	p, err := newPlayer(name)
	if err != nil {
		return SessionResponse{}, err
	}
	r := &room{Code: code, HostID: p.ID, Phase: "lobby", Version: 1, Players: []*player{p}}
	m.rooms[code] = r
	return sessionFor(r, p), nil
}

func (m *Manager) Join(code, name string) (SessionResponse, error) {
	name = cleanName(name)
	if name == "" {
		return SessionResponse{}, ErrInvalidPlayerName
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[code]
	if !ok {
		return SessionResponse{}, ErrRoomNotFound
	}
	if r.Phase != "lobby" {
		return SessionResponse{}, ErrRoomStarted
	}
	if len(r.Players) >= m.maxPlayers {
		return SessionResponse{}, ErrRoomFull
	}
	p, err := newPlayer(name)
	if err != nil {
		return SessionResponse{}, err
	}
	r.Players = append(r.Players, p)
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return sessionFor(r, p), nil
}

func (m *Manager) Preview(code string) (Preview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[code]
	if !ok {
		return Preview{}, ErrRoomNotFound
	}
	status := "open"
	if r.Phase != "lobby" {
		status = "started"
	} else if len(r.Players) >= m.maxPlayers {
		status = "full"
	}
	return Preview{RoomCode: r.Code, PlayerCount: len(r.Players), MaxPlayers: m.maxPlayers, Phase: r.Phase, Status: status}, nil
}

func (m *Manager) authenticate(code, playerID, token string, conn interface{ WriteJSON(any) error }) (*room, *player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[code]
	if !ok {
		return nil, nil, ErrRoomNotFound
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return nil, nil, ErrPlayerNotFound
	}
	if subtle.ConstantTimeCompare([]byte(p.Token), []byte(token)) != 1 {
		return nil, nil, ErrInvalidToken
	}
	p.Conn = conn
	p.Connected = true
	r.Version++
	return r, p, nil
}

func (m *Manager) Ready(r *room, playerID string, ready bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "lobby" {
		return ErrInvalidPhase
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	p.Ready = ready
	r.Version++
	m.broadcastLocked(r, "room.player_ready", map[string]any{"player_id": playerID, "ready": ready})
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) setReady(r *room, playerID string, ready bool) error { return m.Ready(r, playerID, ready) }
func (m *Manager) start(r *room, playerID string) error { return m.Start(r, playerID) }

func (m *Manager) selectKit(r *room, playerID, kit string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "lobby" {
		return ErrInvalidPhase
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	p.Kit = kit
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) Start(r *room, playerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "lobby" {
		return ErrInvalidPhase
	}
	if r.HostID != playerID {
		return ErrNotHost
	}
	if len(r.Players) < m.minPlayers || len(r.Players) > m.maxPlayers {
		return ErrNotReady
	}
	for _, p := range r.Players {
		if !p.Ready {
			return ErrNotReady
		}
	}
	if m.catalog == nil {
		return ErrItemCatalogUnavailable
	}
	if err := m.dungeonConfig.Validate(); err != nil {
		return ErrDungeonConfigUnavailable
	}
	type initialPlayer struct {
		state   *playerstate.State
		dungeon *dungeon.State
	}
	initial := make(map[string]initialPlayer, len(r.Players))
	for _, p := range r.Players {
		kit := p.Kit
		var state *playerstate.State
		var err error
		if kit == "" || kit == "default" {
			state, err = playerstate.NewDefault(m.catalog)
		} else {
			state, err = playerstate.NewWithKit(m.catalog, kit)
		}
		if err != nil {
			return err
		}
		seed, err := randomSeed()
		if err != nil {
			return err
		}
		playerMap, err := dungeon.Generate(seed, m.dungeonConfig)
		if err != nil {
			return err
		}
		initial[p.ID] = initialPlayer{state: state, dungeon: playerMap}
	}
	for _, p := range r.Players {
		p.State = initial[p.ID].state
		p.Dungeon = initial[p.ID].dungeon
		p.Ready = false
	}
	r.Phase = "dungeon"
	r.ScoreEventIDs = make(map[string]struct{})
	r.ScoreEvents = nil
	r.Results = nil
	r.Version++
	m.broadcastLocked(r, "game.phase_changed", map[string]any{"phase": r.Phase})
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) disconnect(r *room, playerID string, conn interface{ WriteJSON(any) error }, leave bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return
	}
	for i, p := range r.Players {
		if p.ID != playerID {
			continue
		}
		if p.Conn != conn {
			return
		}
		p.Conn = nil
		p.Connected = false
		if r.HostID == playerID {
			for _, candidate := range r.Players {
				if candidate.ID != playerID && candidate.Connected {
					r.HostID = candidate.ID
					break
				}
			}
		}
		if r.Phase == "lobby" {
			p.Ready = false
		}
		if leave && r.Phase == "lobby" {
			r.Players = append(r.Players[:i], r.Players[i+1:]...)
			if len(r.Players) == 0 {
				delete(m.rooms, r.Code)
				return
			}
			if r.HostID == playerID {
				r.HostID = r.Players[0].ID
			}
			for _, remaining := range r.Players {
				remaining.Ready = false
			}
		}
		r.Version++
		m.broadcastLocked(r, "room.player_connection", map[string]any{"player_id": playerID, "connected": false})
		if leave {
			m.broadcastLocked(r, "room.player_left", map[string]any{"player_id": playerID})
		}
		m.broadcastSnapshotsLocked(r)
		return
	}
}

func (m *Manager) snapshotLocked(r *room, meID string) Snapshot {
	players := make([]PlayerSnapshot, 0, len(r.Players))
	var me SelfSnapshot
	for _, p := range r.Players {
		view := snapshotPlayer(p, p.ID == r.HostID)
		if p.State != nil {
			stats := p.State.Stats(m.catalog)
			view.HP = stats.HP
			view.MaxHP = stats.MaxHP
		}
		players = append(players, view)
		if p.ID == meID {
			me = SelfSnapshot{PlayerID: p.ID, Name: p.Name, Ready: p.Ready, Connected: p.Connected, Kit: p.Kit, IsHost: p.ID == r.HostID}
			if p.State != nil {
				private := PrivateSnapshot{PrivateSnapshot: p.State.Snapshot(m.catalog), PvPActionsUsed: p.PvPActionsUsed, PvPActionsRemaining: max(0, 5-p.PvPActionsUsed), PvPScore: p.PvPScore, PvPDamage: p.PvPDamage, PvPKnockouts: p.PvPKnockouts}
				if p.Dungeon != nil {
					view := p.Dungeon.PrivateSnapshot()
					private.Dungeon = &view
				}
				if p.Combat != nil {
					clone := *p.Combat
					clone.Log = append([]string(nil), p.Combat.Log...)
					clone.Enemies = append([]combat.Enemy(nil), p.Combat.Enemies...)
					private.Combat = &clone
				}
				if p.Activity != nil {
					clone := *p.Activity
					clone.Options = append([]RoomOption(nil), p.Activity.Options...)
					clone.Stock = append([]ShopStock(nil), p.Activity.Stock...)
					private.Activity = &clone
				}
				me.Private = &private
			}
		}
	}
	return Snapshot{Version: r.Version, Room: RoomView{RoomCode: r.Code, Phase: r.Phase, HostPlayerID: r.HostID, PlayerCount: len(r.Players), MinPlayers: m.minPlayers, MaxPlayers: m.maxPlayers}, Players: players, Me: me, Results: append([]ResultEntry(nil), r.Results...)}
}

func (m *Manager) broadcastSnapshotsLocked(r *room) {
	for _, p := range r.Players {
		if p.Conn != nil {
			_ = p.Conn.WriteJSON(envelope("room.snapshot", "", m.snapshotLocked(r, p.ID)))
		}
	}
}

func (m *Manager) broadcastLocked(r *room, event string, payload any) {
	for _, p := range r.Players {
		if p.Conn != nil {
			_ = p.Conn.WriteJSON(envelope(event, "", payload))
		}
	}
}

func snapshotPlayer(p *player, host bool) PlayerSnapshot {
	view := PlayerSnapshot{PlayerID: p.ID, Name: p.Name, IsHost: host, Ready: p.Ready, Connected: p.Connected, Kit: p.Kit, Stage: "lobby"}
	if p.Dungeon != nil {
		view.Stage = "exploring"
		view.Completed = p.Dungeon.Completed
		if p.Dungeon.Completed {
			view.Stage = "completed"
		}
		view.Floor = p.Dungeon.CurrentFloor
		view.Progress = p.Dungeon.Progress
	}
	if p.State != nil {
		view.Score = p.State.Score
		view.HP = p.State.HP
	}
	view.PvPActionsRemaining = max(0, 5-p.PvPActionsUsed)
	view.PvPScore = p.PvPScore
	view.PvPDamage = p.PvPDamage
	view.PvPKnockouts = p.PvPKnockouts
	if !p.Connected {
		view.Stage = "disconnected"
	}
	return view
}

func (m *Manager) ServeCommand(r *room, playerID string, conn interface{ WriteJSON(any) error }, commandType string, payload map[string]any) error {
	switch commandType {
	case "lobby.ready":
		ready, ok := payload["ready"].(bool)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		return m.Ready(r, playerID, ready)
	case "lobby.select_kit":
		kit, ok := payload["kit"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		return m.selectKit(r, playerID, kit)
	case "game.start":
		return m.Start(r, playerID)
	case "room.leave":
		return m.leave(r, playerID, conn)
	case "preparation.ready":
		ready, ok := payload["ready"].(bool)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		return m.setPreparationReady(r, playerID, ready)
	case "pvp.action":
		action, ok := payload["action"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		targetID, _ := payload["target_player_id"].(string)
		itemID, _ := payload["item_id"].(string)
		return m.pvpAction(r, playerID, action, targetID, itemID)
	case "pvp.force_finish":
		return m.forceFinishPvp(r, playerID)
	case "room.kick_disconnected":
		targetID, ok := payload["target_player_id"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		return m.kickDisconnected(r, playerID, targetID)
	case "room.replay", "lobby.restart":
		return m.restartLobby(r, playerID)
	case "inventory.equip":
		itemID, hasItem := payload["item_id"].(string)
		slot, hasSlot := payload["slot"].(string)
		if !hasItem || !hasSlot {
			return errors.New("INVALID_REQUEST")
		}
		return m.mutatePlayerState(r, playerID, func(state *playerstate.State) error {
			return state.Equip(itemID, slot, m.catalog)
		})
	case "inventory.unequip":
		slot, ok := payload["slot"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		var index *int
		if raw, exists := payload["index"]; exists {
			value, ok := raw.(float64)
			if !ok || value != float64(int(value)) {
				return playerstate.ErrInvalidSlot
			}
			parsed := int(value)
			index = &parsed
		}
		return m.mutatePlayerState(r, playerID, func(state *playerstate.State) error {
			return state.Unequip(slot, index, m.catalog)
		})
	case "inventory.use":
		itemID, ok := payload["item_id"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		return m.mutatePlayerState(r, playerID, func(state *playerstate.State) error {
			return state.Use(itemID, r.Phase, m.catalog)
		})
	case "dungeon.move":
		nodeID, ok := payload["node_id"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		return m.moveDungeon(r, playerID, nodeID)
	case "combat.dismiss":
		return m.dismissCombat(r, playerID)
	case "combat.select_target":
		targetEnemy := 0
		if val, ok := payload["target_enemy"].(float64); ok {
			targetEnemy = int(val)
		}
		return m.selectCombatTarget(r, playerID, targetEnemy)
	case "combat.action":
		action, ok := payload["action"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		itemID, _ := payload["item_id"].(string)
		targetEnemy := 0
		if val, ok := payload["target_enemy"].(float64); ok {
			targetEnemy = int(val)
		}
		skillID, _ := payload["skill_id"].(string)
		return m.combatAction(r, playerID, action, itemID, targetEnemy, skillID)
	case "room.action":
		action, ok := payload["action"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		choiceID, _ := payload["choice_id"].(string)
		itemID, _ := payload["item_id"].(string)
		return m.roomAction(r, playerID, action, choiceID, itemID)
	default:
		return errors.New("INVALID_ACTION")
	}
}

func (m *Manager) dismissCombat(r *room, playerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	if p.Combat != nil && p.Combat.Finished {
		p.Combat = nil
		r.Version++
		m.broadcastSnapshotsLocked(r)
	}
	return nil
}

func (m *Manager) selectCombatTarget(r *room, playerID string, targetEnemy int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	p := findPlayer(r, playerID)
	if p == nil || p.Combat == nil || p.Combat.Finished {
		return combat.ErrNoCombat
	}
	if targetEnemy >= 0 && targetEnemy < len(p.Combat.Enemies) && p.Combat.Enemies[targetEnemy].Alive {
		p.Combat.SelectedEnemy = targetEnemy
		p.Combat.SyncLegacyFields()
		r.Version++
		m.broadcastSnapshotsLocked(r)
	}
	return nil
}

func (m *Manager) restartLobby(r *room, playerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "results" {
		return ErrInvalidPhase
	}
	r.Phase = "lobby"
	r.Results = nil
	r.ScoreEvents = nil
	r.ScoreEventIDs = make(map[string]struct{})
	for _, p := range r.Players {
		p.Ready = false
		p.State = nil
		p.Dungeon = nil
		p.Combat = nil
		p.Activity = nil
		p.PvPActionsUsed = 0
		p.PvPScore = 0
		p.PvPDamage = 0
		p.PvPKnockouts = 0
		p.PvPDeaths = 0
		p.PvPPenalty = 0
		p.PvPBonus = 0
		p.PvPGuarding = false
		p.PveScore = 0
		p.BossesDefeated = 0
	}
	r.Version++
	m.broadcastLocked(r, "game.phase_changed", map[string]any{"phase": r.Phase})
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) moveDungeon(r *room, playerID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "dungeon" {
		return ErrInvalidPhase
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	if p.Dungeon == nil || p.State == nil {
		return ErrInvalidPhase
	}
	if p.Activity != nil && !p.Activity.Completed {
		return dungeon.ErrEncounterActive
	}
	if p.Combat != nil && !p.Combat.Finished {
		return combat.ErrNoCombat
	}
	if err := p.Dungeon.Move(nodeID); err != nil {
		return err
	}
	floor := p.Dungeon.Floors[p.Dungeon.CurrentFloor-1]
	var node dungeon.Node
	for _, candidate := range floor.Nodes {
		if candidate.ID == nodeID {
			node = candidate
			break
		}
	}
	if node.Type == dungeon.MonsterRoom || node.Type == dungeon.EliteRoom || node.Type == dungeon.BossRoom {
		p.Activity = nil
		multiplier := floor.DifficultyMultiplier
		seed := int64(p.Dungeon.Seed) + int64(node.Floor*1000+node.Depth*100+len(node.ID))
		if m.monsters != nil {
			state, _, err := m.monsters.Create(string(node.Type), node.Floor, multiplier, seed)
			if err != nil {
				_ = p.Dungeon.Respawn()
				return err
			}
			p.Combat = state
		}
	} else if err := m.beginActivity(p, node); err != nil {
		_ = p.Dungeon.Respawn()
		return err
	} else {
		p.Combat = nil
	}
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) mutatePlayerState(r *room, playerID string, action func(*playerstate.State) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "dungeon" && r.Phase != "preparation" && r.Phase != "combat" {
		return ErrInvalidPhase
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	if p.State == nil {
		return ErrInvalidPhase
	}
	if p.Combat != nil && !p.Combat.Finished {
		return combat.ErrNoCombat
	}
	if err := action(p.State); err != nil {
		return err
	}
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) combatAction(r *room, playerID, action, itemID string, extra ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "dungeon" {
		return ErrInvalidPhase
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	if p.State == nil || p.Dungeon == nil || p.Combat == nil || p.Combat.Finished {
		return combat.ErrNoCombat
	}
	enc := p.Combat
	if len(extra) > 0 {
		if val, ok := extra[0].(int); ok && val >= 0 && val < len(enc.Enemies) && enc.Enemies[val].Alive {
			enc.SelectedEnemy = val
		}
	}
	target := enc.Selected()
	if target == nil && enc.AliveCount() > 0 {
		for i := range enc.Enemies {
			if enc.Enemies[i].Alive {
				enc.SelectedEnemy = i
				target = &enc.Enemies[i]
				break
			}
		}
	}
	if target != nil && enc.HP > 0 && target.HP > enc.HP {
		target.HP = enc.HP
	}

	rng := mathrand.New(mathrand.NewSource(int64(p.Dungeon.Seed) ^ int64(r.Version+1)))
	finish := false
	skipped := false
	died := false
	monsterDef := m.monsterByID(enc.MonsterID)
	stats := p.State.Stats(m.catalog)

	// Ranger First Strike on Turn 1
	if stats.Archetype == "bow" && !enc.FirstStrike && target != nil {
		firstDmg := max(1, stats.ATK/2)
		target.HP = max(0, target.HP-firstDmg)
		enc.FirstStrike = true
		enc.Log = append(enc.Log, fmt.Sprintf("🏹 First Strike! You hit %s for %d damage before battle starts!", target.Name, firstDmg))
		if target.HP == 0 {
			target.Alive = false
			enc.Log = append(enc.Log, target.Name+" was slain by your opening volley!")
			target = enc.Selected()
		}
	}

	switch action {
	case "ATTACK":
		if target == nil {
			finish = true
			break
		}
		playerFirst := stats.SPD >= target.Speed
		dealDamage := func() {
			dmg := combat.Damage(stats.ATK, target.Defense)
			if stats.Archetype == "daggers" && rng.Intn(100) < 30 {
				dmg = int(float64(dmg) * 1.8)
				enc.Log = append(enc.Log, "💥 CRITICAL STRIKE!")
			} else if stats.Archetype == "staff" {
				dmg = combat.Damage(stats.ATK, int(float64(target.Defense)*0.6))
			}
			target.HP = max(0, target.HP-dmg)
			enc.Log = append(enc.Log, fmt.Sprintf("You dealt %d damage to %s.", dmg, target.Name))
			if target.HP == 0 {
				target.Alive = false
				enc.Log = append(enc.Log, target.Name+" was defeated!")
			}
		}

		if playerFirst {
			dealDamage()
			if enc.AliveCount() == 0 {
				finish = true
			}
		} else {
			m.enemiesAttack(p, enc, stats, rng)
			if p.State.HP == 0 {
				died = true
				finish = true
			} else {
				dealDamage()
				if enc.AliveCount() == 0 {
					finish = true
				}
			}
		}

	case "SKILL":
		cost := stats.WeaponSkillCost
		if cost <= 0 {
			cost = 4
		}
		if !p.State.ConsumeMP(cost) {
			return playerstate.ErrNotEnoughMP
		}
		enc.Log = append(enc.Log, fmt.Sprintf("⚡ You used %s! (-%d MP)", stats.WeaponSkill, cost))
		if stats.WeaponSkillTarget == "aoe" {
			for i := range enc.Enemies {
				e := &enc.Enemies[i]
				if !e.Alive {
					continue
				}
				dmg := combat.Damage(int(float64(stats.ATK)*1.25), e.Defense)
				e.HP = max(0, e.HP-dmg)
				enc.Log = append(enc.Log, fmt.Sprintf("Hit %s for %d damage.", e.Name, dmg))
				if e.HP == 0 {
					e.Alive = false
					enc.Log = append(enc.Log, e.Name+" was defeated!")
				}
			}
		} else {
			if target != nil {
				dmg := combat.Damage(int(float64(stats.ATK)*1.6)+stats.DEF, target.Defense)
				target.HP = max(0, target.HP-dmg)
				enc.Log = append(enc.Log, fmt.Sprintf("Dealt %d devastating damage to %s!", dmg, target.Name))
				if target.HP == 0 {
					target.Alive = false
					enc.Log = append(enc.Log, target.Name+" was defeated!")
				}
			}
		}
		if enc.AliveCount() == 0 {
			finish = true
		}

	case "DEFEND":
		enc.Guarding = true
		enc.Log = append(enc.Log, "You raise your guard for the incoming turn.")
		if stats.Archetype == "staff" {
			p.State.RestoreMP(8, m.catalog)
			enc.Log = append(enc.Log, "Mana Barrier restored 8 MP!")
		}

	case "USE_ITEM":
		if itemID == "" {
			return errors.New("INVALID_REQUEST")
		}
		itemDef, ok := m.catalog.Get(itemID)
		if !ok {
			return errors.New("ITEM_NOT_FOUND")
		}
		if itemDef.Effect != nil && itemDef.Effect.Type == "damage_aoe" {
			if err := p.State.Remove(itemID, 1); err != nil {
				return err
			}
			enc.Log = append(enc.Log, fmt.Sprintf("💣 You hurled a %s dealing %d direct damage to all foes!", itemDef.Name, itemDef.Effect.Value))
			for i := range enc.Enemies {
				if enc.Enemies[i].Alive {
					enc.Enemies[i].HP = max(0, enc.Enemies[i].HP-itemDef.Effect.Value)
					if enc.Enemies[i].HP == 0 {
						enc.Enemies[i].Alive = false
						enc.Log = append(enc.Log, enc.Enemies[i].Name+" was destroyed!")
					}
				}
			}
			if enc.AliveCount() == 0 {
				finish = true
			}
		} else if itemDef.Effect != nil && itemDef.Effect.Type == "smoke" {
			if enc.Tier == "boss" {
				return combat.ErrBossEscape
			}
			if err := p.State.Remove(itemID, 1); err != nil {
				return err
			}
			skipped = true
			finish = true
			enc.Log = append(enc.Log, "💨 Smoke screen deployed! Escaped the encounter.")
		} else {
			if err := p.State.Use(itemID, "combat", m.catalog); err != nil {
				return err
			}
			enc.Log = append(enc.Log, "You used "+itemDef.Name+".")
		}

	case "ESCAPE":
		if enc.Tier == "boss" {
			return combat.ErrBossEscape
		}
		if rng.Intn(2) == 0 {
			skipped = true
			enc.Log = append(enc.Log, "You escaped the encounter.")
			finish = true
		} else {
			enc.Log = append(enc.Log, "Escape failed!")
		}
	default:
		return combat.ErrInvalidAction
	}

	// Counter-attack if enemies are alive and player did not act second
	if !finish && !died && action != "ATTACK" {
		m.enemiesAttack(p, enc, stats, rng)
		if p.State.HP == 0 {
			died = true
			finish = true
		}
	} else if !finish && !died && action == "ATTACK" {
		playerFirst := target != nil && stats.SPD >= target.Speed
		if playerFirst {
			m.enemiesAttack(p, enc, stats, rng)
			if p.State.HP == 0 {
				died = true
				finish = true
			}
		}
	}

	if died {
		enc.Log = append(enc.Log, "💀 You were defeated and returned to checkpoint.")
		m.pveRespawn(p, rng)
		finish = true
	}

	if !finish {
		p.State.RestoreMP(3, m.catalog)
		enc.Guarding = false
		enc.Turn++
		for i := range enc.Enemies {
			e := &enc.Enemies[i]
			if !e.Alive {
				continue
			}
			if enc.Turn%3 == 0 {
				e.Intent = "HEAVY"
				e.IntentValue = int(float64(e.Attack) * 1.8)
			} else {
				e.Intent = "ATTACK"
				e.IntentValue = e.Attack
			}
		}
	}

	if finish {
		enc.Finished = true
		if !died {
			enc.Victory = true
			_ = p.Dungeon.ResolveActive(skipped)
			if !skipped {
				reward := rng.Intn(max(1, monsterDef.GoldMax-monsterDef.GoldMin+1)) + monsterDef.GoldMin
				p.State.Gold += reward
				source := "monster_kill"
				if monsterDef.Tier == "elite" {
					source = "elite_kill"
				}
				if monsterDef.Tier == "boss" {
					source = "boss_kill"
				}
				if m.recordScoreEvent(r, ScoreEvent{ID: "pve:monster:" + p.ID + ":" + p.Dungeon.CheckpointNodeID, PlayerID: p.ID, Category: "pve", Source: source, Points: monsterDef.ScoreReward}) {
					p.State.Score += monsterDef.ScoreReward
				}
				expReward := monsterDef.ScoreReward * 2
				leveled := p.State.GainEXP(expReward, m.catalog)
				enc.Log = append(enc.Log, fmt.Sprintf("🎉 Victory! +%d Gold, +%d Score, +%d EXP.", reward, monsterDef.ScoreReward, expReward))
				if leveled {
					enc.Log = append(enc.Log, fmt.Sprintf("🌟 LEVEL UP! You reached Level %d! All stats increased!", p.State.Level))
				}
				m.tryMonsterLoot(p, monsterDef, rng)
				if monsterDef.Tier == "boss" {
					completed, err := p.Dungeon.AdvanceFloor()
					if err == nil {
						p.Ready = false
						p.BossesDefeated++
						if m.recordScoreEvent(r, ScoreEvent{ID: fmt.Sprintf("pve:floor:%s:%d", p.ID, p.Dungeon.CurrentFloor), PlayerID: p.ID, Category: "pve", Source: "floor_clear", Points: 20}) {
							p.State.Score += 20
						}
						if completed {
							if m.recordScoreEvent(r, ScoreEvent{ID: "pve:dungeon:" + p.ID, PlayerID: p.ID, Category: "pve", Source: "dungeon_clear", Points: 40}) {
								p.State.Score += 40
							}
							enc.Log = append(enc.Log, "🏆 Dungeon fully cleared! +40 Score.")
						} else {
							enc.Log = append(enc.Log, "Floor cleared: +20 Score.")
						}
						m.maybePreparation(r)
					}
				}
			}
		}
	}
	enc.SyncLegacyFields()
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) enemiesAttack(p *player, enc *combat.State, stats playerstate.StatsSnapshot, rng *mathrand.Rand) {
	for i := range enc.Enemies {
		e := &enc.Enemies[i]
		if !e.Alive {
			continue
		}
		rawDmg := e.Attack
		if e.Intent == "HEAVY" {
			rawDmg = e.IntentValue
			enc.Log = append(enc.Log, fmt.Sprintf("⚠️ %s unleashes a HEAVY SMASH!", e.Name))
		}
		dmg := combat.Damage(rawDmg, stats.DEF)
		if enc.Guarding {
			if stats.Archetype == "daggers" && rng.Intn(100) < 50 {
				dmg = 0
				enc.Log = append(enc.Log, "💨 Shadow Dodge! You completely evaded the attack!")
			} else if stats.Archetype == "sword" {
				dmg = max(1, int(float64(dmg)*0.3))
				reflect := max(1, dmg)
				e.HP = max(0, e.HP-reflect)
				enc.Log = append(enc.Log, fmt.Sprintf("🛡️ Shield Block! Reduced damage to %d and reflected %d to %s!", dmg, reflect, e.Name))
			} else if stats.Archetype == "staff" {
				dmg = max(1, int(float64(dmg)*0.4))
				enc.Log = append(enc.Log, fmt.Sprintf("✨ Mana Barrier absorbed the hit! Took only %d damage.", dmg))
			} else {
				dmg = max(1, int(float64(dmg)*0.5))
				enc.Log = append(enc.Log, fmt.Sprintf("🛡️ Guard reduced damage to %d.", dmg))
			}
		}
		if dmg > 0 {
			p.State.Damage(dmg)
			enc.Log = append(enc.Log, fmt.Sprintf("%s dealt %d damage.", e.Name, dmg))
		}
		if p.State.HP == 0 {
			break
		}
	}
}

func (m *Manager) maybePreparation(r *room) {
	for _, player := range r.Players {
		if player.Dungeon == nil || !player.Dungeon.Completed {
			return
		}
	}
	r.Phase = "preparation"
	r.Version++
	m.broadcastLocked(r, "game.phase_changed", map[string]any{"phase": r.Phase})
	m.broadcastSnapshotsLocked(r)
}

func (m *Manager) setPreparationReady(r *room, playerID string, ready bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "dungeon" && r.Phase != "preparation" {
		return ErrInvalidPhase
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	if p.Dungeon == nil || !p.Dungeon.Completed {
		return ErrInvalidPhase
	}
	p.Ready = ready
	r.Version++
	if r.Phase == "preparation" {
		allReady := true
		for _, player := range r.Players {
			if !player.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			m.startPvPLocked(r)
			return nil
		}
	}
	m.broadcastLocked(r, "preparation.player_ready", map[string]any{"player_id": playerID, "ready": ready})
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) startPvPLocked(r *room) {
	for _, player := range r.Players {
		if player.State != nil {
			maxHP := player.State.Stats(m.catalog).MaxHP
			player.State.RestoreHP(maxHP, m.catalog)
			maxMP := player.State.Stats(m.catalog).MaxMP
			player.State.RestoreMP(maxMP, m.catalog)
			player.PveScore = player.State.Score
		}
		player.Ready = false
		player.PvPActionsUsed = 0
		player.PvPGuarding = false
	}
	r.Phase = "pvp"
	r.Version++
	m.broadcastLocked(r, "game.phase_changed", map[string]any{"phase": r.Phase})
	m.broadcastSnapshotsLocked(r)
}

func (m *Manager) monsterByID(id string) combat.Monster {
	if m.monsters == nil {
		return combat.Monster{}
	}
	for _, monster := range m.monsters.Monsters {
		if monster.ID == id {
			return monster
		}
	}
	return combat.Monster{}
}

func (m *Manager) pveRespawn(p *player, rng *mathrand.Rand) {
	p.State.Deaths++
	p.State.Gold -= p.State.Gold / 5
	total := 0
	for _, s := range p.State.Inventory {
		total += s.Quantity
	}
	if total > 0 {
		pick := rng.Intn(total)
		for _, s := range p.State.Inventory {
			if pick < s.Quantity {
				_ = playerstateRemoveOne(p.State, s.ItemID)
				break
			}
			pick -= s.Quantity
		}
	}
	_ = p.Dungeon.Respawn()
	maxHP := p.State.Stats(m.catalog).MaxHP
	p.State.RestoreHP((maxHP+1)/2, m.catalog)
}

func playerstateRemoveOne(s *playerstate.State, id string) error { return s.Remove(id, 1) }

func (m *Manager) tryMonsterLoot(p *player, monster combat.Monster, rng *mathrand.Rand) {
	if rng.Intn(100) >= 40 {
		return
	}
	pool := []string{
		"health_potion", "greater_health_potion", "mana_potion",
		"iron_sword", "crystal_wand", "shadowfang", "elven_recurve",
		"chainmail_hauberk", "boots_of_haste", "thiefs_coin", "fire_bomb",
	}
	id := pool[rng.Intn(len(pool))]
	if err := p.State.Add(id, 1, m.catalog); err == nil {
		p.Combat.Log = append(p.Combat.Log, "Found "+id+".")
	}
	_ = monster
}

func (m *Manager) leave(r *room, playerID string, conn interface{ WriteJSON(any) error }) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "lobby" {
		return ErrInvalidPhase
	}
	for index, p := range r.Players {
		if p.ID != playerID {
			continue
		}
		if p.Conn != conn {
			return ErrPlayerNotFound
		}
		r.Players = append(r.Players[:index], r.Players[index+1:]...)
		if len(r.Players) == 0 {
			delete(m.rooms, r.Code)
			return nil
		}
		if r.HostID == playerID {
			r.HostID = r.Players[0].ID
		}
		for _, remaining := range r.Players {
			remaining.Ready = false
		}
		r.Version++
		m.broadcastLocked(r, "room.player_left", map[string]any{"player_id": playerID})
		m.broadcastSnapshotsLocked(r)
		return nil
	}
	return ErrPlayerNotFound
}

func (m *Manager) Touch(r *room) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r != nil && m.rooms[r.Code] == r {
		r.Version++
		m.broadcastSnapshotsLocked(r)
	}
}

func (m *Manager) notifyConnected(r *room, playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r != nil && m.rooms[r.Code] == r {
		m.broadcastLocked(r, "room.player_connection", map[string]any{"player_id": playerID, "connected": true})
		m.broadcastSnapshotsLocked(r)
	}
}

func (m *Manager) ClaimRequestID(r *room, playerID, requestID string) error {
	if requestID == "" || len(requestID) > 128 {
		return ErrInvalidRequestID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	if p.RequestIDs == nil {
		p.RequestIDs = make(map[string]struct{})
	}
	if _, exists := p.RequestIDs[requestID]; exists {
		return ErrDuplicateRequest
	}
	p.RequestIDs[requestID] = struct{}{}
	p.RequestOrder = append(p.RequestOrder, requestID)
	if len(p.RequestOrder) > maxRequestHistory {
		oldest := p.RequestOrder[0]
		delete(p.RequestIDs, oldest)
		copy(p.RequestOrder, p.RequestOrder[1:])
		p.RequestOrder = p.RequestOrder[:len(p.RequestOrder)-1]
	}
	return nil
}

func (m *Manager) newRoomCodeLocked() (string, error) {
	for attempts := 0; attempts < 1000; attempts++ {
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		var codeBuilder strings.Builder
		for _, b := range bytes {
			codeBuilder.WriteByte(roomAlphabet[int(b)%len(roomAlphabet)])
		}
		code := codeBuilder.String()
		if _, exists := m.rooms[code]; !exists {
			return code, nil
		}
	}
	return "", errors.New("ROOM_CODE_GENERATION_FAILED")
}

func newPlayer(name string) (*player, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(bytes[:8])
	token := hex.EncodeToString(bytes[8:])
	return &player{ID: id, Name: name, Token: token, Ready: false, Connected: false, Kit: "default", RequestIDs: make(map[string]struct{})}, nil
}

func cleanName(name string) string {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) > 20 {
		return trimmed[:20]
	}
	return trimmed
}

func sessionFor(r *room, p *player) SessionResponse {
	return SessionResponse{RoomCode: r.Code, PlayerID: p.ID, ReconnectToken: p.Token, IsHost: r.HostID == p.ID, WSPath: "/ws"}
}

func findPlayer(r *room, playerID string) *player {
	for _, p := range r.Players {
		if p.ID == playerID {
			return p
		}
	}
	return nil
}

func randomSeed() (uint64, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(bytes[:]), nil
}
