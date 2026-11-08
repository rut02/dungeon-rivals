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
	PlayerID      string `json:"player_id"`
	Name          string `json:"name"`
	IsHost        bool   `json:"is_host"`
	Ready         bool   `json:"ready"`
	Connected     bool   `json:"connected"`
	Kit           string `json:"kit"`
	IsBot         bool   `json:"is_bot"`
	BotDifficulty string `json:"bot_difficulty,omitempty"`
	Stage         string `json:"stage"`
	Floor         int    `json:"floor"`
	Progress      int    `json:"progress"`
	Score         int    `json:"score"`
	Completed     bool   `json:"completed"`
	HP            int    `json:"hp"`
	MaxHP         int    `json:"max_hp"`
	MP            int    `json:"mp"`
	MaxMP         int    `json:"max_mp"`
	ATK           int    `json:"atk"`
	DEF           int    `json:"def"`
	SPD           int    `json:"spd"`
	WeaponSkill   string `json:"weapon_skill,omitempty"`
	PvPScore      int    `json:"pvp_score"`
	PvPDamage     int    `json:"pvp_damage"`
	PvPKnockouts  int    `json:"pvp_knockouts"`
	PvPWins       int    `json:"pvp_wins"`
	PvPLosses     int    `json:"pvp_losses"`
}

type PvPBoutView struct {
	BoutNumber  int    `json:"bout_number"`
	PlayerAID   string `json:"player_a_id"`
	PlayerAName string `json:"player_a_name"`
	PlayerBID   string `json:"player_b_id"`
	PlayerBName string `json:"player_b_name"`
	WinnerID    string `json:"winner_id,omitempty"`
	WinnerName  string `json:"winner_name,omitempty"`
	Finished    bool   `json:"finished"`
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
	IsBot     bool             `json:"is_bot,omitempty"`
	Private   *PrivateSnapshot `json:"private,omitempty"`
}

type PrivateSnapshot struct {
	playerstate.PrivateSnapshot
	Dungeon               *dungeon.PrivateView `json:"dungeon,omitempty"`
	Combat                *combat.State        `json:"combat,omitempty"`
	Activity              *RoomActivity        `json:"activity,omitempty"`
	LastAcquiredItemID    string               `json:"last_acquired_item_id,omitempty"`
	LastAcquiredSeq       int                  `json:"last_acquired_seq,omitempty"`
	PvPActionsUsed        int                  `json:"pvp_actions_used,omitempty"`
	PvPCurrentTurnID      string               `json:"pvp_current_turn_player_id,omitempty"`
	PvPYourTurn           bool                 `json:"pvp_your_turn,omitempty"`
	PvPScore              int                  `json:"pvp_score,omitempty"`
	PvPDamage             int                  `json:"pvp_damage,omitempty"`
	PvPKnockouts          int                  `json:"pvp_knockouts,omitempty"`
	PvPWins               int                  `json:"pvp_wins,omitempty"`
	PvPLosses             int                  `json:"pvp_losses,omitempty"`
	PvPCurrentOpponentID  string               `json:"pvp_current_opponent_id,omitempty"`
	PvPBoutNumber         int                  `json:"pvp_bout_number,omitempty"`
	PvPTotalBouts         int                  `json:"pvp_total_bouts,omitempty"`
	PvPLog                []string             `json:"pvp_log,omitempty"`
	PvPBouts              []PvPBoutView        `json:"pvp_bouts,omitempty"`
	PvPTempSPD            int                  `json:"pvp_temp_spd,omitempty"`
	PvPTempSPDTurns       int                  `json:"pvp_temp_spd_turns,omitempty"`
	PvPEquipmentCooldowns map[string]int       `json:"pvp_equipment_cooldowns,omitempty"`
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
	ID, Name, Token       string
	Ready, Connected      bool
	Kit                   string
	IsBot                 bool
	BotDifficulty         string
	Conn                  interface{ WriteJSON(any) error }
	RequestIDs            map[string]struct{}
	RequestOrder          []string
	State                 *playerstate.State
	Dungeon               *dungeon.State
	Combat                *combat.State
	Activity              *RoomActivity
	LastAcquiredItemID    string
	LastAcquiredSeq       int
	PvPActionsUsed        int
	PvPScore              int
	PvPScoreRemainder     int
	PvPDamage             int
	PvPKnockouts          int
	PvPDeaths             int
	PvPPenalty            int
	PvPBonus              int
	PvPGuarding           bool
	PvPSmoke              bool
	PvPFirstHitGuardUsed  bool
	PvPBarrier            int
	PvPLifeStealRemainder int
	PvPTempSPD            int
	PvPTempSPDTurns       int
	PvPEquipmentCooldowns map[string]int
	PvPWins               int
	PvPLosses             int
	PveScore              int
	BossesDefeated        int
}

func (p *player) recordAcquiredItem(itemID string) {
	if itemID == "" {
		return
	}
	p.LastAcquiredItemID = itemID
	p.LastAcquiredSeq++
}

type PvPBout struct {
	BoutNumber          int
	PlayerAID           string
	PlayerBID           string
	WinnerID            string
	Turns               int
	CurrentTurnPlayerID string
	Finished            bool
}

type room struct {
	Code, HostID, Phase string
	Version             uint64
	Players             []*player
	ScoreEventIDs       map[string]struct{}
	Results             []ResultEntry
	ScoreEvents         []ScoreEvent
	PvPBouts            []PvPBout
	CurrentBoutIdx      int
	PvPLog              []string
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

func (m *Manager) setReady(r *room, playerID string, ready bool) error {
	return m.Ready(r, playerID, ready)
}
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
		if p.IsBot {
			applyBotDifficulty(state, p.BotDifficulty, m.catalog)
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
		if leave {
			r.Players = append(r.Players[:i], r.Players[i+1:]...)
			humanCount := 0
			for _, remaining := range r.Players {
				if !remaining.IsBot {
					humanCount++
				}
			}
			if humanCount == 0 {
				delete(m.rooms, r.Code)
				return
			}
			if r.HostID == playerID {
				for _, remaining := range r.Players {
					if !remaining.IsBot {
						r.HostID = remaining.ID
						break
					}
				}
			}
			if r.Phase == "lobby" {
				for _, remaining := range r.Players {
					if !remaining.IsBot {
						remaining.Ready = false
					}
				}
			} else if r.Phase == "dungeon" || r.Phase == "preparation" {
				m.maybePreparation(r)
			} else if r.Phase == "pvp" {
				if m.allPvPBoutsFinished(r) {
					m.finishPvPLocked(r)
				}
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
			view.MP = stats.MP
			view.MaxMP = stats.MaxMP
			view.ATK = stats.ATK
			view.DEF = stats.DEF
			view.SPD = stats.SPD
			view.WeaponSkill = stats.WeaponSkill
		}
		players = append(players, view)
		if p.ID == meID {
			me = SelfSnapshot{PlayerID: p.ID, Name: p.Name, Ready: p.Ready, Connected: p.Connected, Kit: p.Kit, IsBot: p.IsBot, IsHost: p.ID == r.HostID}
			if p.State != nil {
				oppID, boutNum, totalBouts := m.pvpStatusForPlayer(r, p.ID)
				currentTurnID := ""
				if bout := m.currentBoutLocked(r); bout != nil {
					currentTurnID = bout.CurrentTurnPlayerID
				}
				private := PrivateSnapshot{
					PrivateSnapshot:       p.State.Snapshot(m.catalog),
					LastAcquiredItemID:    p.LastAcquiredItemID,
					LastAcquiredSeq:       p.LastAcquiredSeq,
					PvPActionsUsed:        p.PvPActionsUsed,
					PvPCurrentTurnID:      currentTurnID,
					PvPYourTurn:           currentTurnID == p.ID,
					PvPScore:              p.PvPScore,
					PvPDamage:             p.PvPDamage,
					PvPKnockouts:          p.PvPKnockouts,
					PvPWins:               p.PvPWins,
					PvPLosses:             p.PvPLosses,
					PvPCurrentOpponentID:  oppID,
					PvPBoutNumber:         boutNum,
					PvPTotalBouts:         totalBouts,
					PvPLog:                append([]string(nil), r.PvPLog...),
					PvPBouts:              m.pvpBoutViews(r),
					PvPTempSPD:            p.PvPTempSPD,
					PvPTempSPDTurns:       p.PvPTempSPDTurns,
					PvPEquipmentCooldowns: cloneIntMap(p.PvPEquipmentCooldowns),
				}
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
	view := PlayerSnapshot{PlayerID: p.ID, Name: p.Name, IsHost: host, Ready: p.Ready, Connected: p.Connected, Kit: p.Kit, IsBot: p.IsBot, BotDifficulty: p.BotDifficulty, Stage: "lobby"}
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
	view.PvPScore = p.PvPScore
	view.PvPDamage = p.PvPDamage
	view.PvPKnockouts = p.PvPKnockouts
	view.PvPWins = p.PvPWins
	view.PvPLosses = p.PvPLosses
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
	case "lobby.add_bot":
		diff, _ := payload["difficulty"].(string)
		kit, _ := payload["kit"].(string)
		return m.addBot(r, playerID, diff, kit)
	case "lobby.remove_bot":
		botID, ok := payload["bot_player_id"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		return m.removeBot(r, playerID, botID)
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
		targetEnemy := -1
		if val, ok := payload["target_enemy"].(float64); ok {
			targetEnemy = int(val)
		} else if val, ok := payload["target"].(float64); ok {
			targetEnemy = int(val)
		}
		return m.selectCombatTarget(r, playerID, targetEnemy)
	case "combat.action":
		action, ok := payload["action"].(string)
		if !ok {
			return errors.New("INVALID_REQUEST")
		}
		itemID, _ := payload["item_id"].(string)
		targetEnemy := -1
		if val, ok := payload["target_enemy"].(float64); ok {
			targetEnemy = int(val)
		} else if val, ok := payload["target"].(float64); ok {
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
		m.simulateBotsProgress(r)
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
	r.PvPBouts = nil
	r.CurrentBoutIdx = 0
	r.PvPLog = nil
	for _, p := range r.Players {
		p.Ready = false
		p.State = nil
		p.Dungeon = nil
		p.Combat = nil
		p.Activity = nil
		p.LastAcquiredItemID = ""
		p.LastAcquiredSeq = 0
		p.PvPActionsUsed = 0
		p.PvPScore = 0
		p.PvPDamage = 0
		p.PvPKnockouts = 0
		p.PvPDeaths = 0
		p.PvPPenalty = 0
		p.PvPBonus = 0
		p.PvPGuarding = false
		p.PvPSmoke = false
		p.PvPWins = 0
		p.PvPLosses = 0
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
		// Keep encounters threatening as EXP increases the player's whole stat line.
		// A small level-based bump prevents early-floor bosses from falling behind.
		levelBonus := float64(min(5, max(0, p.State.Level-1))) * 0.08
		multiplier := floor.DifficultyMultiplier + levelBonus
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
	m.simulateBotsProgress(r)
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
	if len(enc.Enemies) == 1 && target != nil {
		if enc.HP > 0 && target.HP > enc.HP {
			target.HP = enc.HP
		}
		if enc.Attack > 0 && enc.Attack > target.Attack {
			target.Attack = enc.Attack
		}
	}

	rng := mathrand.New(mathrand.NewSource(int64(p.Dungeon.Seed) ^ int64(r.Version+1)))
	finish := false
	skipped := false
	died := false
	monsterDef := m.monsterByID(enc.MonsterID)
	stats := p.State.Stats(m.catalog)
	stats = combatStatsWithBuffs(stats, enc)
	kit := p.State.Kit
	if kit == "" || kit == "default" || kit == "warrior" {
		kit = "knight"
	}
	playerFollowUpBonus := m.equipmentAbilityValue(p, "followup")
	if stats.Archetype == "bow" {
		playerFollowUpBonus += m.equipmentAbilityValue(p, "ranger_followup")
	}

	// Ranger First Strike on Turn 1
	if kit == "ranger" && !enc.FirstStrike && target != nil {
		firstDmg := max(1, stats.ATK/2)
		target.HP = max(0, target.HP-firstDmg)
		enc.FirstStrike = true
		enc.Log = append(enc.Log, fmt.Sprintf("🏹 First Strike! You hit %s for %d damage before battle starts!", target.Name, firstDmg))
		if !target.Marked {
			target.Marked = true
			enc.Log = append(enc.Log, fmt.Sprintf("🎯 [Ranger Passive] เล็งจุดตาย! ทำเครื่องหมายใส่ %s (เป้าหมายรับดาเมจเดี่ยว +20%%)", target.Name))
		}
		if target.HP == 0 {
			target.Alive = false
			enc.Log = append(enc.Log, target.Name+" was slain by your opening volley!")
			target = enc.Selected()
		}
	}

	playerFirst := target == nil || stats.SPD >= target.Speed

	switch action {
	case "ATTACK":
		if target == nil {
			finish = true
			break
		}
		dealDamage := func() {
			dmg := combat.Damage(stats.ATK, target.Defense)
			critChance := 30
			if stats.Archetype == "daggers" {
				critChance += m.equipmentAbilityValue(p, "rogue_crit")
			}
			if kit == "rogue" {
				critChance += 15 // Rogue passive bonus
			}
			if (stats.Archetype == "daggers" || kit == "rogue") && rng.Intn(100) < min(50, critChance) {
				dmg = int(float64(dmg) * 1.8)
				enc.Log = append(enc.Log, "💥 CRITICAL STRIKE!")
			} else if stats.Archetype == "staff" {
				dmg = combat.Damage(stats.ATK, int(float64(target.Defense)*0.5))
			}
			if target.Marked {
				dmg = int(float64(dmg) * 1.20)
				enc.Log = append(enc.Log, fmt.Sprintf("🎯 [Marked Target] รอยมาร์กจุดตายเพิ่มความเสียหาย +20%%! (%d ดาเมจ)", dmg))
			}
			hpBefore := target.HP
			target.HP = max(0, target.HP-dmg)
			m.healFromDamage(p, hpBefore, dmg, enc)
			enc.Log = append(enc.Log, fmt.Sprintf("You dealt %d damage to %s.", dmg, target.Name))
			if kit == "rogue" && target.HP > 0 {
				if rng.Intn(100) < 35 {
					target.Status = "bleed"
					target.StatusTurns = 2
					enc.Log = append(enc.Log, fmt.Sprintf("🩸 [Rogue Passive] คมอาวุธกรีดลึก! %s ติดสถานะเลือดไหล (Bleed 2 รอบ)", target.Name))
				}
			}
			if kit == "ranger" && target.HP > 0 {
				if !target.Marked {
					target.Marked = true
					enc.Log = append(enc.Log, fmt.Sprintf("🎯 [Ranger Passive] เล็งจุดตาย! ทำเครื่องหมายใส่ %s (เป้าหมายรับดาเมจเดี่ยว +20%%)", target.Name))
				}
			}
			if target.HP == 0 {
				target.Alive = false
				enc.Log = append(enc.Log, target.Name+" was defeated!")
				return
			}
			if combat.ExtraStrikeEligible(stats.SPD, target.Speed) && enc.PlayerStatus != "freeze" && target.Alive {
				followUp := max(1, int(float64(dmg)*0.40))
				hpBefore = target.HP
				target.HP = max(0, target.HP-followUp)
				m.healFromDamage(p, hpBefore, followUp, enc)
				enc.Log = append(enc.Log, fmt.Sprintf("⚡ SPD Advantage! (SPD %d vs %d) โจมตีซ้ำปกติเพิ่ม 1 ครั้ง ใส่ %s อีก %d ดาเมจ (40%%)", stats.SPD, target.Speed, target.Name, followUp))
				if kit == "rogue" && target.HP > 0 && target.Status == "" {
					if rng.Intn(100) < 18 {
						target.Status = "bleed"
						target.StatusTurns = 2
						enc.Log = append(enc.Log, fmt.Sprintf("🩸 [Rogue Passive] โจมตีซ้ำติดสถานะเลือดไหลใส่ %s!", target.Name))
					}
				}
				if target.HP == 0 {
					target.Alive = false
					enc.Log = append(enc.Log, target.Name+" was defeated!")
				}
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
			} else if enc.AliveCount() == 0 {
				finish = true
			} else {
				dealDamage()
				if enc.AliveCount() == 0 {
					finish = true
				}
			}
		}

	case "SKILL":
		if enc.SkillCooldown > 0 {
			return errors.New("SKILL_ON_COOLDOWN")
		}
		cost := stats.WeaponSkillCost
		if cost <= 0 {
			cost = 4
		}
		cost = max(1, cost-m.equipmentAbilityValue(p, "skill_cost"))
		if kit == "mage" && !enc.MageSkillUsed {
			enc.MageSkillUsed = true
			savedMP := max(3, cost/2)
			cost = max(1, cost-savedMP)
			enc.Log = append(enc.Log, fmt.Sprintf("✨ [Mage Passive] สมาธิจิตแห่งจอมเวท: ลดค่าร่าย MP สกิลแรกในการต่อสู้ลง %d หน่วย!", savedMP))
		}
		if !p.State.ConsumeMP(cost) {
			return playerstate.ErrNotEnoughMP
		}
		enc.SkillCooldown = 3 // 2 turns cooldown after turn ends
		if kit == "knight" {
			enc.ShieldBarrier += 5
			enc.Log = append(enc.Log, "🛡️ [Knight Passive] จิตวิญญาณอัศวินปลุกพลังบาเรียป้องกัน +5 หน่วย!")
		}
		skillATK := (stats.ATK*(100+m.equipmentAbilityValue(p, "skill_damage")) + 99) / 100
		castSkill := func() {
			enc.Log = append(enc.Log, fmt.Sprintf("⚡ You used %s! (-%d MP)", stats.WeaponSkill, cost))
			if stats.WeaponSkill == "Fourfold Spirit Pact" {
				if enc.Summon != nil {
					enc.Log = append(enc.Log, "สปิริตเดิมสลายก่อนเปิดทางให้สปิริตธาตุใหม่")
				}
				elements := []string{"fire", "earth", "water", "wind"}
				element := elements[rng.Intn(len(elements))]
				summon := &combat.Summon{Element: element, TurnsRemaining: 2}
				switch element {
				case "fire":
					summon.Name = "สปิริตเพลิง"
					if target != nil && target.Alive {
						dmg := combat.Damage(int(float64(skillATK)*1.25), target.Defense)
						hpBefore := target.HP
						target.HP = max(0, target.HP-dmg)
						m.healFromDamage(p, hpBefore, dmg, enc)
						enc.Log = append(enc.Log, fmt.Sprintf("🔥 สปิริตเพลิงพุ่งโจมตี %s สร้างความเสียหาย %d", target.Name, dmg))
						if target.HP == 0 {
							target.Alive = false
						}
					} else {
						summon.TurnsRemaining = 1
					}
				case "earth":
					summon.Name = "โกเลมศิลา"
					summon.MaxHP = max(18, stats.MaxHP/4)
					summon.HP = summon.MaxHP
					summon.TurnsRemaining = 2
					enc.Log = append(enc.Log, fmt.Sprintf("🪨 โกเลมศิลายั่วยุศัตรู! HP %d · รับการโจมตีเดี่ยวแทนคุณ 2 รอบ", summon.HP))
				case "water":
					summon.Name = "สปิริตวารี"
					heal := max(1, stats.MaxHP/10)
					before := p.State.HP
					p.State.RestoreHP(p.State.HP+heal, m.catalog)
					if enc.PlayerStatus != "" {
						enc.Log = append(enc.Log, "💧 สปิริตวารีล้างสถานะผิดปกติ!")
						enc.PlayerStatus = ""
					}
					enc.Log = append(enc.Log, fmt.Sprintf("💧 สปิริตวารีฟื้นฟู HP %d", p.State.HP-before))
				case "wind":
					summon.Name = "สปิริตวายุ"
					applyCombatBuff(enc, combat.StatBuff{Name: "พรแห่งวายุ", Stat: "spd", Value: 4, TurnsRemaining: 3})
					enc.Log = append(enc.Log, "🌪️ สปิริตวายุเร่งความเร็วชั่วคราว: SPD +4 เป็นเวลา 2 รอบ")
				}
				if summon.MaxHP == 0 {
					summon.MaxHP = 1
					summon.HP = 1
				}
				enc.Summon = summon
				return
			}
			if stats.WeaponSkillTarget == "adjacent" {
				start := max(0, enc.SelectedEnemy-1)
				end := min(len(enc.Enemies)-1, enc.SelectedEnemy+1)
				for i := start; i <= end; i++ {
					e := &enc.Enemies[i]
					if !e.Alive {
						continue
					}
					dmg := combat.Damage(int(float64(skillATK)*1.25), e.Defense)
					hpBefore := e.HP
					e.HP = max(0, e.HP-dmg)
					m.healFromDamage(p, hpBefore, dmg, enc)
					enc.Log = append(enc.Log, fmt.Sprintf("⚔️ Cleave กวาดโดน %s %d ดาเมจ", e.Name, dmg))
					if e.HP == 0 {
						e.Alive = false
					}
				}
				return
			}
			if stats.WeaponSkillTarget == "aoe" {
				for i := range enc.Enemies {
					e := &enc.Enemies[i]
					if !e.Alive {
						continue
					}
					def := int(float64(e.Defense) * 0.7)
					if stats.Archetype == "staff" {
						def = int(float64(e.Defense) * 0.4)
					}
					dmg := combat.Damage(int(float64(skillATK)*1.20), def)
					hpBefore := e.HP
					e.HP = max(0, e.HP-dmg)
					m.healFromDamage(p, hpBefore, dmg, enc)
					enc.Log = append(enc.Log, fmt.Sprintf("Hit %s for %d damage.", e.Name, dmg))
					if e.HP == 0 {
						e.Alive = false
						enc.Log = append(enc.Log, e.Name+" was defeated!")
					}
				}
			} else {
				if target != nil && target.Alive {
					var dmg int
					switch stats.Archetype {
					case "sword":
						dmg = combat.Damage(int(float64(skillATK)*1.35)+stats.DEF, target.Defense)
					case "daggers":
						dmg = combat.Damage(int(float64(skillATK)*1.65), target.Defense)
					case "bow", "staff":
						dmg = combat.Damage(int(float64(skillATK)*1.45), target.Defense/2)
					default:
						dmg = combat.Damage(int(float64(skillATK)*1.40)+stats.DEF, target.Defense)
					}
					if target.Marked {
						dmg = int(float64(dmg) * 1.20)
						enc.Log = append(enc.Log, fmt.Sprintf("🎯 [Marked Target] รอยมาร์กจุดตายเพิ่มความเสียหาย +20%%! (%d ดาเมจ)", dmg))
					}
					hpBefore := target.HP
					target.HP = max(0, target.HP-dmg)
					m.healFromDamage(p, hpBefore, dmg, enc)
					enc.Log = append(enc.Log, fmt.Sprintf("Dealt %d devastating damage to %s!", dmg, target.Name))
					if kit == "ranger" && target.HP > 0 && !target.Marked {
						target.Marked = true
						enc.Log = append(enc.Log, fmt.Sprintf("🎯 [Ranger Passive] เล็งจุดตาย! ทำเครื่องหมายใส่ %s (เป้าหมายรับดาเมจเดี่ยว +20%%)", target.Name))
					}
					if target.HP == 0 {
						target.Alive = false
						enc.Log = append(enc.Log, target.Name+" was defeated!")
					}
				}
			}
		}

		if playerFirst {
			castSkill()
			if enc.AliveCount() == 0 {
				finish = true
			}
		} else {
			m.enemiesAttack(p, enc, stats, rng)
			if p.State.HP == 0 {
				died = true
				finish = true
			} else if enc.AliveCount() == 0 {
				finish = true
			} else {
				castSkill()
				if enc.AliveCount() == 0 {
					finish = true
				}
			}
		}

	case "DEFEND":
		enc.Guarding = true
		enc.Log = append(enc.Log, "You raise your guard for the incoming turn.")
		enc.ShieldBarrier = m.equipmentAbilityValue(p, "guard_barrier")
		if enc.ShieldBarrier > 0 {
			enc.Log = append(enc.Log, fmt.Sprintf("🛡️ อุปกรณ์ตั้งรับสร้างบาเรียรับความเสียหาย %d หน่วย", enc.ShieldBarrier))
		}
		if stats.Archetype == "staff" {
			p.State.RestoreMP(3, m.catalog)
			enc.Log = append(enc.Log, "Mana Barrier restored 3 MP!")
		} else {
			p.State.RestoreMP(2, m.catalog)
			enc.Log = append(enc.Log, "🧘 ตั้งรับและรวบรวมพลัง: ฟื้น 2 MP")
		}

	case "GEAR_ABILITY":
		itemDef, ok := m.catalog.Get(itemID)
		if !ok || itemDef.AbilityType != "active" || itemDef.AbilityEffect == nil || itemDef.AbilityEffect.Type != "speed_buff" || !playerHasEquipped(p, itemID) {
			return combat.ErrInvalidAction
		}
		if enc.EquipmentCooldowns[itemID] > 0 {
			return errors.New("EQUIPMENT_SKILL_ON_COOLDOWN")
		}
		if !p.State.ConsumeMP(itemDef.AbilityCost) {
			return playerstate.ErrNotEnoughMP
		}
		applyCombatBuff(enc, combat.StatBuff{Name: itemDef.AbilityName, Stat: "spd", Value: itemDef.AbilityEffect.Value, TurnsRemaining: itemDef.AbilityEffect.Duration + 1})
		if enc.EquipmentCooldowns == nil {
			enc.EquipmentCooldowns = make(map[string]int)
		}
		enc.EquipmentCooldowns[itemID] = itemDef.AbilityCooldown + 1
		enc.Log = append(enc.Log, fmt.Sprintf("💨 %s! SPD +%d ชั่วคราว %d รอบ · ใช้ %d MP", itemDef.AbilityName, itemDef.AbilityEffect.Value, itemDef.AbilityEffect.Duration, itemDef.AbilityCost))

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
		} else if itemDef.Effect != nil && itemDef.Effect.Type == "speed_buff" {
			if err := p.State.Use(itemID, "combat", m.catalog); err != nil {
				return err
			}
			applyCombatBuff(enc, combat.StatBuff{Name: itemDef.Name, Stat: "spd", Value: itemDef.Effect.Value, TurnsRemaining: 3})
			enc.Log = append(enc.Log, fmt.Sprintf("⚡ %s เพิ่ม SPD +%d ชั่วคราว 2 รอบ", itemDef.Name, itemDef.Effect.Value))
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

	// Counter-attack if enemies are alive and player did not already take enemy attacks first
	if !finish && !died && (action != "ATTACK" && action != "SKILL") {
		m.enemiesAttack(p, enc, stats, rng)
		if p.State.HP == 0 {
			died = true
			finish = true
		} else if enc.AliveCount() == 0 {
			finish = true
		}
	} else if !finish && !died && (action == "ATTACK" || action == "SKILL") {
		if playerFirst {
			m.enemiesAttack(p, enc, stats, rng)
			if p.State.HP == 0 {
				died = true
				finish = true
			} else if enc.AliveCount() == 0 {
				finish = true
			}
		}
	}

	if died {
		enc.Log = append(enc.Log, "💀 You were defeated and returned to checkpoint.")
		m.pveRespawn(p, rng)
		finish = true
	}
	if !finish && !died && enc.PlayerStatus != "" {
		switch enc.PlayerStatus {
		case "poison":
			p.State.Damage(3)
			enc.PlayerStatusTurns--
			enc.Log = append(enc.Log, "☠️ พิษกัดกร่อน HP 3 หน่วย")
		case "bleed":
			p.State.Damage(4)
			enc.PlayerStatusTurns--
			enc.Log = append(enc.Log, "🩸 เลือดไหลซึม HP 4 หน่วย")
		case "burn":
			p.State.Damage(4)
			enc.PlayerStatusTurns--
			enc.Log = append(enc.Log, "🔥 เปลวไฟเผาไหม้ HP 4 หน่วย")
		case "freeze":
			enc.PlayerStatusTurns--
			enc.Log = append(enc.Log, "❄️ ตัวแข็งจากความเย็นยะเยือก (-4 SPD)")
		default:
			enc.PlayerStatusTurns--
		}
		if enc.PlayerStatusTurns <= 0 {
			enc.PlayerStatus = ""
			enc.PlayerStatusTurns = 0
		}
		if p.State.HP == 0 {
			died = true
			finish = true
			enc.Log = append(enc.Log, "💀 คุณหมดสติจากอาการบาดเจ็บและพิษร้าย")
			m.pveRespawn(p, rng)
		}
	}

	if !finish {
		for i := range enc.Enemies {
			e := &enc.Enemies[i]
			if !e.Alive {
				continue
			}
			if e.Status != "" {
				dot := 0
				switch e.Status {
				case "bleed":
					dot = 4
					enc.Log = append(enc.Log, fmt.Sprintf("🩸 เลือดไหลซึมจากบาดแผล! %s เสีย 4 HP", e.Name))
				case "burn":
					dot = 4
					enc.Log = append(enc.Log, fmt.Sprintf("🔥 ไฟเผาไหม้ต่อเนื่อง! %s เสีย 4 HP", e.Name))
				case "poison":
					dot = 3
					enc.Log = append(enc.Log, fmt.Sprintf("☠️ พิษกัดกร่อน! %s เสีย 3 HP", e.Name))
				case "freeze":
					enc.Log = append(enc.Log, fmt.Sprintf("❄️ %s เย็นยะเยือกขยับช้าลง (-4 SPD)", e.Name))
				}
				if dot > 0 {
					e.HP = max(0, e.HP-dot)
					if e.HP == 0 {
						e.Alive = false
						enc.Log = append(enc.Log, fmt.Sprintf("💀 %s สิ้นชีพจากผลของสถานะผิดปกติ!", e.Name))
					}
				}
				e.StatusTurns--
				if e.StatusTurns <= 0 {
					e.Status = ""
					e.StatusTurns = 0
				}
			}
			if e.StunResist {
				e.StunResist = false
			}
		}
		if enc.AliveCount() == 0 {
			finish = true
		}
	}

	if !finish {
		if enc.SkillCooldown > 0 {
			enc.SkillCooldown--
		}
		tickEquipmentCooldowns(enc)
		tickCombatBuffs(enc)
		enc.Guarding = false
		if enc.Summon != nil {
			enc.Summon.TurnsRemaining--
			if enc.Summon.TurnsRemaining <= 0 || enc.Summon.HP <= 0 {
				enc.Log = append(enc.Log, enc.Summon.Name+" สลายไป")
				enc.Summon = nil
			}
		}
		enc.Turn++
		for i := range enc.Enemies {
			e := &enc.Enemies[i]
			if !e.Alive {
				continue
			}
			if e.AoEChance > 0 && rng.Intn(100) < e.AoEChance {
				e.Intent = "AOE"
				e.IntentValue = e.Attack
			} else if (enc.Turn+i)%3 == 0 {
				e.Intent = "HEAVY"
				e.IntentValue = max(e.Attack+2, int(float64(e.Attack)*1.35))
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
				reward := 0
				for _, e := range enc.Enemies {
					mDef := m.monsterByID(e.MonsterID)
					if mDef.ID == "" {
						mDef = monsterDef
					}
					reward += rng.Intn(max(1, mDef.GoldMax-mDef.GoldMin+1)) + mDef.GoldMin
				}
				if reward == 0 {
					reward = rng.Intn(max(1, monsterDef.GoldMax-monsterDef.GoldMin+1)) + monsterDef.GoldMin
				}
				if bonus := m.equipmentAbilityValue(p, "gold_bonus"); bonus > 0 {
					reward += reward * bonus / 100
				}
				p.State.Gold += reward
				source := "monster_kill"
				if monsterDef.Tier == "elite" {
					source = "elite_kill"
				}
				if monsterDef.Tier == "boss" {
					source = "boss_kill"
				}
				scoreReward := monsterDef.ScoreReward
				if len(enc.Enemies) > 1 {
					scoreReward += (len(enc.Enemies) - 1) * 5
				}
				if m.recordScoreEvent(r, ScoreEvent{ID: "pve:monster:" + p.ID + ":" + p.Dungeon.CurrentNodeID, PlayerID: p.ID, Category: "pve", Source: source, Points: scoreReward}) {
					p.State.Score += scoreReward
				}
				expReward := scoreReward * 2
				leveled := p.State.GainEXP(expReward, m.catalog)
				enc.Log = append(enc.Log, fmt.Sprintf("🎉 Victory! +%d Gold, +%d Score, +%d EXP.", reward, scoreReward, expReward))
				if leveled {
					enc.Log = append(enc.Log, fmt.Sprintf("🌟 LEVEL UP! You reached Level %d! All stats increased!", p.State.Level))
				}
				mpRestored := max(3, stats.MaxMP*3/10)
				p.State.RestoreMP(mpRestored, m.catalog)
				enc.Log = append(enc.Log, fmt.Sprintf("✨ สมาธิฟื้นคืนหลังกำชัย: ฟื้นฟู %d MP", mpRestored))
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
		if e.Status == "stun" {
			if e.Tier == "boss" {
				enc.Log = append(enc.Log, fmt.Sprintf("👑 %s เป็นบอส ต้านทานสตันเต็มรูปแบบ! (ลดพลังโจมตีและความเร็ว)", e.Name))
				e.Status = ""
				e.StatusTurns = 0
				e.StunResist = true
			} else {
				enc.Log = append(enc.Log, fmt.Sprintf("💫 %s ติดสตัน ข้ามการกระทำในเทิร์นนี้!", e.Name))
				e.Status = ""
				e.StatusTurns = 0
				continue
			}
		}
		effectiveSpeed := e.Speed
		if e.Status == "freeze" {
			effectiveSpeed = max(1, effectiveSpeed-4)
		}
		extraStrike := e.Intent == "ATTACK" && combat.ExtraStrikeEligible(effectiveSpeed, stats.SPD) && e.Status != "freeze"
		strikeCount := 1
		if extraStrike {
			strikeCount = 2
		}
		var firstStrikeDmg int
		for strike := 0; strike < strikeCount; strike++ {
			if !e.Alive {
				break
			}
			areaAttack := strike == 0 && e.Intent == "AOE"
			heavyAttack := strike == 0 && e.Intent == "HEAVY"
			rawDmg := e.Attack
			if len(enc.Enemies) == 1 && enc.Attack > rawDmg {
				rawDmg = enc.Attack
			}
			if heavyAttack {
				rawDmg = e.IntentValue
				enc.Log = append(enc.Log, fmt.Sprintf("⚠️ %s unleashes a HEAVY SMASH!", e.Name))
			} else if areaAttack {
				rawDmg = e.IntentValue
				enc.Log = append(enc.Log, fmt.Sprintf("💥 %s unleashes an area attack!", e.Name))
			} else if strike > 0 {
				enc.Log = append(enc.Log, fmt.Sprintf("⚡ %s เร็วกว่า (SPD %d vs %d) โจมตีซ้ำปกติอีก 1 ครั้ง (40%%)", e.Name, effectiveSpeed, stats.SPD))
			}
			dmg := combat.Damage(rawDmg, stats.DEF)
			if strike == 0 {
				firstStrikeDmg = dmg
			} else {
				dmg = max(1, int(float64(firstStrikeDmg)*0.40))
			}
			if heavyAttack {
				if reduction := m.equipmentAbilityValue(p, "heavy_reduction"); reduction > 0 {
					dmg = max(1, dmg*(100-reduction)/100)
				}
			}
			if areaAttack {
				if reduction := m.equipmentAbilityValue(p, "aoe_reduction"); reduction > 0 {
					dmg = max(1, dmg*(100-reduction)/100)
				}
			}
			if enc.Guarding {
				if stats.Archetype == "daggers" && rng.Intn(100) < 50 {
					dmg = 0
					enc.Log = append(enc.Log, "💨 Shadow Dodge! You completely evaded the attack!")
				} else if stats.Archetype == "sword" && p.State.Equipment.Offhand != "" {
					dmg = max(1, int(float64(dmg)*0.3))
					reflect := max(2, stats.DEF)
					e.HP = max(0, e.HP-reflect)
					enc.Log = append(enc.Log, fmt.Sprintf("🛡️ Shield Block! Reduced damage to %d and reflected %d to %s!", dmg, reflect, e.Name))
					if e.HP == 0 {
						e.Alive = false
						enc.Log = append(enc.Log, fmt.Sprintf("💥 %s was defeated by your shield reflection!", e.Name))
					}
				} else if stats.Archetype == "staff" {
					dmg = max(1, int(float64(dmg)*0.4))
					enc.Log = append(enc.Log, fmt.Sprintf("✨ Mana Barrier absorbed the hit! Took only %d damage.", dmg))
				} else {
					dmg = max(1, int(float64(dmg)*0.5))
					enc.Log = append(enc.Log, fmt.Sprintf("🛡️ Guard reduced damage to %d.", dmg))
				}
			}
			if enc.Summon != nil && enc.Summon.Element == "earth" && enc.Summon.HP > 0 && !areaAttack {
				enc.Summon.HP = max(0, enc.Summon.HP-dmg)
				enc.Log = append(enc.Log, fmt.Sprintf("🪨 %s โจมตีโกเลมศิลาแทนคุณ โกเลมเหลือ HP %d/%d", e.Name, enc.Summon.HP, enc.Summon.MaxHP))
				if enc.Summon.HP == 0 {
					enc.Log = append(enc.Log, "💥 โกเลมศิลาพังทลาย! ศัตรูหันกลับมาเล็งคุณ")
					enc.Summon = nil
				}
				continue
			}
			if !areaAttack {
				if chance := min(20, m.equipmentAbilityValue(p, "evasion")); chance > 0 && rng.Intn(100) < chance {
					enc.Log = append(enc.Log, "💨 อุปกรณ์หลบหลีกช่วยให้คุณหลบการโจมตีได้!")
					continue
				}
			}
			if !enc.FirstHitGuardUsed {
				if reduction := m.equipmentAbilityValue(p, "first_hit_reduction"); reduction > 0 {
					dmg = max(1, dmg*(100-reduction)/100)
					enc.FirstHitGuardUsed = true
					enc.Log = append(enc.Log, fmt.Sprintf("🪖 เกราะศีรษะลดความเสียหายครั้งแรก %d%%", reduction))
				}
			}
			// Area attacks hit both the player and earth golem; player barriers
			// absorb only the player's share of that damage.
			summonDamage := dmg
			if dmg > 0 && enc.ShieldBarrier > 0 {
				blocked := min(dmg, enc.ShieldBarrier)
				dmg -= blocked
				enc.ShieldBarrier -= blocked
				enc.Log = append(enc.Log, fmt.Sprintf("🛡️ โล่ดูดซับความเสียหาย %d หน่วย", blocked))
			}
			if dmg > 0 {
				p.State.Damage(dmg)
				enc.Log = append(enc.Log, fmt.Sprintf("%s dealt %d damage.", e.Name, dmg))
				statusChance := e.StatusChance * (100 - m.equipmentAbilityValue(p, "poison_resist")) / 100
				if e.AttackStatus != "" && statusChance > 0 && rng.Intn(100) < statusChance {
					enc.PlayerStatus = e.AttackStatus
					enc.PlayerStatusTurns = 2
					enc.Log = append(enc.Log, "☠️ คุณติดพิษ! สปิริตน้ำสามารถล้างสถานะนี้ได้")
				}
			}
			if areaAttack && enc.Summon != nil && enc.Summon.Element == "earth" {
				enc.Summon.HP = max(0, enc.Summon.HP-summonDamage)
				enc.Log = append(enc.Log, fmt.Sprintf("🪨 โกเลมศิลาถูกโจมตีวงกว้างด้วย เสีย HP %d", summonDamage))
				if enc.Summon.HP == 0 {
					enc.Log = append(enc.Log, "💥 โกเลมศิลาพังทลายจากการโจมตีวงกว้าง!")
					enc.Summon = nil
				}
			}
			if p.State.HP == 0 {
				break
			}
		}
		if p.State.HP == 0 {
			break
		}
	}
}

func applyCombatBuff(enc *combat.State, buff combat.StatBuff) {
	if enc == nil || buff.Stat == "" || buff.Value <= 0 || buff.TurnsRemaining <= 0 {
		return
	}
	for i := range enc.PlayerBuffs {
		current := &enc.PlayerBuffs[i]
		if current.Stat != buff.Stat {
			continue
		}
		if buff.Value > current.Value {
			current.Name = buff.Name
			current.Value = buff.Value
		}
		current.TurnsRemaining = max(current.TurnsRemaining, buff.TurnsRemaining)
		return
	}
	enc.PlayerBuffs = append(enc.PlayerBuffs, buff)
}

func tickCombatBuffs(enc *combat.State) {
	if enc == nil || len(enc.PlayerBuffs) == 0 {
		return
	}
	remaining := enc.PlayerBuffs[:0]
	for _, buff := range enc.PlayerBuffs {
		buff.TurnsRemaining--
		if buff.TurnsRemaining > 0 {
			remaining = append(remaining, buff)
		}
	}
	enc.PlayerBuffs = remaining
}

func combatStatsWithBuffs(stats playerstate.StatsSnapshot, enc *combat.State) playerstate.StatsSnapshot {
	if enc == nil {
		return stats
	}
	for _, buff := range enc.PlayerBuffs {
		switch buff.Stat {
		case "atk":
			stats.ATK += buff.Value
		case "def":
			stats.DEF += buff.Value
		case "spd":
			stats.SPD += buff.Value
		}
	}
	return stats
}

func playerHasEquipped(p *player, itemID string) bool {
	if p == nil || p.State == nil || itemID == "" {
		return false
	}
	e := p.State.Equipment
	if e.Weapon == itemID || e.Offhand == itemID || e.Armor == itemID || e.Helmet == itemID {
		return true
	}
	return e.Accessories[0] == itemID || e.Accessories[1] == itemID
}

func (m *Manager) equipmentAbilityValue(p *player, effectType string) int {
	if m.catalog == nil || p == nil || p.State == nil {
		return 0
	}
	e := p.State.Equipment
	ids := []string{e.Weapon, e.Offhand, e.Armor, e.Helmet, e.Accessories[0], e.Accessories[1]}
	value := 0
	for _, id := range ids {
		item, ok := m.catalog.Get(id)
		if ok && item.AbilityEffect != nil && item.AbilityEffect.Type == effectType {
			value += item.AbilityEffect.Value
		}
	}
	return value
}

func (m *Manager) healFromDamage(p *player, targetHPBefore, damage int, enc *combat.State) {
	percent := m.equipmentAbilityValue(p, "lifesteal")
	dealt := min(targetHPBefore, damage)
	if percent <= 0 || dealt <= 0 {
		return
	}
	if enc == nil {
		return
	}
	enc.LifeStealRemainder += dealt * percent
	healing := enc.LifeStealRemainder / 100
	enc.LifeStealRemainder %= 100
	if healing == 0 {
		return
	}
	before := p.State.HP
	p.State.RestoreHP(before+healing, m.catalog)
	if healed := p.State.HP - before; healed > 0 {
		enc.Log = append(enc.Log, fmt.Sprintf("🩸 Blood Chalice ดูดเลือดฟื้น HP %d", healed))
	}
}

func tickEquipmentCooldowns(enc *combat.State) {
	for id, turns := range enc.EquipmentCooldowns {
		if turns <= 1 {
			delete(enc.EquipmentCooldowns, id)
		} else {
			enc.EquipmentCooldowns[id] = turns - 1
		}
	}
}

func cloneIntMap(values map[string]int) map[string]int {
	if len(values) == 0 {
		return nil
	}
	copyOfValues := make(map[string]int, len(values))
	for key, value := range values {
		copyOfValues[key] = value
	}
	return copyOfValues
}

func (m *Manager) maybePreparation(r *room) {
	for _, player := range r.Players {
		if !player.IsBot {
			if player.Dungeon == nil || !player.Dungeon.Completed {
				return
			}
		}
	}
	for _, player := range r.Players {
		if player.IsBot {
			if player.Dungeon != nil {
				player.Dungeon.Completed = true
			}
			player.Ready = true
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
		player.PvPSmoke = false
		player.PvPWins = 0
		player.PvPLosses = 0
	}
	r.Phase = "pvp"
	m.initPvPTournamentLocked(r)
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
	chance := 55
	if monster.Tier == "elite" {
		chance = 100
	} else if monster.Tier == "boss" {
		chance = 100
	}
	if rng.Intn(100) >= chance {
		return
	}
	pool := []string{
		"health_potion", "greater_health_potion", "mana_potion", "greater_mana_potion",
		"iron_sword", "knights_claymore", "wooden_staff", "crystal_wand",
		"elemental_grimoire",
		"assassins_twinblades", "shadowfang", "elven_recurve", "windrunner_bow",
		"chainmail_hauberk", "iron_helm", "shadow_cowl", "iron_shield",
		"boots_of_haste", "thiefs_coin", "blood_chalice", "fire_bomb", "smoke_bomb",
	}
	if monster.Tier == "boss" {
		pool = []string{
			"flamebrand", "excalibur", "archmage_staff", "void_scepter",
			"shadowfang", "grim_scythe", "windrunner_bow", "dragons_breath_bow",
			"dragonscale_armor", "crown_of_kings", "aegis_shield", "blood_chalice",
		}
	}
	kit := p.State.Kit
	if kit == "" || kit == "default" || kit == "warrior" {
		kit = "knight"
	}
	affinityWeapons := make([]string, 0, len(pool))
	crossWeapons := make([]string, 0, len(pool))
	otherItems := make([]string, 0, len(pool))

	for _, itemID := range pool {
		def, ok := m.catalog.Get(itemID)
		if !ok {
			continue
		}
		if def.Type == items.Weapon {
			if weaponClass(def) == kit {
				affinityWeapons = append(affinityWeapons, itemID)
			} else {
				crossWeapons = append(crossWeapons, itemID)
			}
		} else {
			otherItems = append(otherItems, itemID)
		}
	}

	var id string
	totalWeapons := len(affinityWeapons) + len(crossWeapons)
	if totalWeapons > 0 && (len(otherItems) == 0 || rng.Intn(100) < 50) {
		if len(affinityWeapons) > 0 && (len(crossWeapons) == 0 || rng.Intn(100) < 60) {
			id = affinityWeapons[rng.Intn(len(affinityWeapons))]
		} else if len(crossWeapons) > 0 {
			id = crossWeapons[rng.Intn(len(crossWeapons))]
		} else {
			id = affinityWeapons[rng.Intn(len(affinityWeapons))]
		}
	} else if len(otherItems) > 0 {
		id = otherItems[rng.Intn(len(otherItems))]
	} else if len(affinityWeapons) > 0 {
		id = affinityWeapons[rng.Intn(len(affinityWeapons))]
	} else if len(crossWeapons) > 0 {
		id = crossWeapons[rng.Intn(len(crossWeapons))]
	}
	if id == "" {
		return
	}
	if err := p.State.Add(id, 1, m.catalog); err == nil {
		p.Combat.LootItemID = id
		p.recordAcquiredItem(id)
		if def, ok := m.catalog.Get(id); ok {
			p.Combat.Log = append(p.Combat.Log, "🎁 Loot dropped: "+def.Name+"!")
		} else {
			p.Combat.Log = append(p.Combat.Log, "Found "+id+".")
		}
	}
}

func (m *Manager) leave(r *room, playerID string, conn interface{ WriteJSON(any) error }) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	for index, p := range r.Players {
		if p.ID != playerID {
			continue
		}
		if conn != nil && p.Conn != conn {
			return ErrPlayerNotFound
		}
		r.Players = append(r.Players[:index], r.Players[index+1:]...)
		humanCount := 0
		for _, remaining := range r.Players {
			if !remaining.IsBot {
				humanCount++
			}
		}
		if humanCount == 0 {
			delete(m.rooms, r.Code)
			return nil
		}
		if r.HostID == playerID {
			for _, remaining := range r.Players {
				if !remaining.IsBot {
					r.HostID = remaining.ID
					break
				}
			}
		}
		if r.Phase == "lobby" {
			for _, remaining := range r.Players {
				if !remaining.IsBot {
					remaining.Ready = false
				}
			}
		} else if r.Phase == "dungeon" || r.Phase == "preparation" {
			m.maybePreparation(r)
		} else if r.Phase == "pvp" {
			if m.allPvPBoutsFinished(r) {
				m.finishPvPLocked(r)
			}
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

func (m *Manager) addBot(r *room, playerID, difficulty, kit string) error {
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
	if len(r.Players) >= m.maxPlayers {
		return ErrRoomFull
	}
	if difficulty == "" {
		difficulty = "normal"
	}
	diffLabels := map[string]string{
		"easy":      "ง่าย",
		"normal":    "ปานกลาง",
		"hard":      "ยาก",
		"nightmare": "มหาโหด",
	}
	diffLabel, ok := diffLabels[difficulty]
	if !ok {
		difficulty = "normal"
		diffLabel = "ปานกลาง"
	}
	validKits := []string{"knight", "mage", "rogue", "ranger"}
	if kit == "" || kit == "random" {
		kit = validKits[mathrand.Intn(len(validKits))]
	}
	botNames := map[string][]string{
		"knight": {"เซอร์โรแลนด์", "กาเร็ธโล่เหล็ก", "เบอริคอัศวิน"},
		"mage":   {"จอมเวทไลรา", "อิกนิสเพลิงผลาญ", "เซฟีร์มนตรา"},
		"rogue":  {"ชาโดว์เวน", "ไคเอลมีดว่องไว", "นิกซ์เงามืด"},
		"ranger": {"โรบินเหยี่ยวเวหา", "เฟย์ธนูทิพย์", "ไซลัสศรทะลวง"},
	}
	nameOptions := botNames[kit]
	pickedName := "บอทนักผจญภัย"
	if len(nameOptions) > 0 {
		pickedName = nameOptions[mathrand.Intn(len(nameOptions))]
	}
	botDisplayName := fmt.Sprintf("🤖 %s (%s)", pickedName, diffLabel)

	p, err := newPlayer(botDisplayName)
	if err != nil {
		return err
	}
	p.IsBot = true
	p.BotDifficulty = difficulty
	p.Kit = kit
	p.Connected = true
	p.Ready = true

	r.Players = append(r.Players, p)
	r.Version++
	m.broadcastLocked(r, "room.player_joined", map[string]any{"player_id": p.ID, "name": p.Name})
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) removeBot(r *room, playerID, botID string) error {
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
	for i, p := range r.Players {
		if p.ID == botID && p.IsBot {
			r.Players = append(r.Players[:i], r.Players[i+1:]...)
			r.Version++
			m.broadcastLocked(r, "room.player_left", map[string]any{"player_id": botID})
			m.broadcastSnapshotsLocked(r)
			return nil
		}
	}
	return ErrPlayerNotFound
}

func applyBotDifficulty(state *playerstate.State, difficulty string, catalog *items.Catalog) {
	if state == nil {
		return
	}
	switch difficulty {
	case "easy":
		state.MaxHP = 75
		state.HP = 75
		state.BaseATK = max(5, state.BaseATK-2)
		state.BaseDEF = max(1, state.BaseDEF-1)
		state.BaseSPD = max(6, state.BaseSPD-2)
	case "normal":
		// default kit stats
	case "hard":
		state.MaxHP += 35
		state.HP += 35
		state.BaseATK += 3
		state.BaseDEF += 2
		state.BaseSPD += 3
		state.Gold += 60
		_ = state.Add("greater_health_potion", 1, catalog)
	case "nightmare":
		state.MaxHP += 70
		state.HP += 70
		state.BaseATK += 6
		state.BaseDEF += 4
		state.BaseSPD += 5
		state.Gold += 150
		_ = state.Add("greater_health_potion", 2, catalog)
		_ = state.Add("greater_mana_potion", 1, catalog)
	}
}

func (m *Manager) simulateBotsProgress(r *room) {
	allHumansCompleted := true
	for _, p := range r.Players {
		if !p.IsBot {
			if p.Dungeon == nil || !p.Dungeon.Completed {
				allHumansCompleted = false
				break
			}
		}
	}

	for _, bot := range r.Players {
		if !bot.IsBot || bot.Dungeon == nil || bot.Dungeon.Completed {
			continue
		}

		if allHumansCompleted {
			bot.Dungeon.Completed = true
			bot.BossesDefeated = max(bot.BossesDefeated, 3)
			bot.State.Score += 80
			bot.Ready = true
			continue
		}

		bot.Dungeon.Progress = min(6, bot.Dungeon.Progress+1)
		bot.State.Score += 10
		bot.State.Gold += 10
		if bot.Dungeon.Progress >= 6 {
			completed, err := bot.Dungeon.AdvanceFloor()
			if err == nil {
				bot.BossesDefeated++
				bot.State.Score += 20
				if completed {
					bot.Dungeon.Completed = true
					bot.State.Score += 40
					bot.Ready = true
				}
			}
		}
	}
}
