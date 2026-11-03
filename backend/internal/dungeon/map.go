package dungeon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
)

type RoomType string
type NodeStatus string

const (
	StartRoom    RoomType   = "START"
	MonsterRoom  RoomType   = "MONSTER"
	EliteRoom    RoomType   = "ELITE"
	TreasureRoom RoomType   = "TREASURE"
	ShopRoom     RoomType   = "SHOP"
	EventRoom    RoomType   = "EVENT"
	RestRoom     RoomType   = "REST"
	BossRoom     RoomType   = "BOSS"
	PuzzleRoom   RoomType   = "PUZZLE"
	Locked       NodeStatus = "LOCKED"
	Available    NodeStatus = "AVAILABLE"
	InProgress   NodeStatus = "IN_PROGRESS"
	Cleared      NodeStatus = "CLEARED"
	Skipped      NodeStatus = "SKIPPED"
	Failed       NodeStatus = "FAILED"
)

var (
	ErrInvalidMapConfig = errors.New("INVALID_DUNGEON_CONFIG")
	ErrInvalidNode      = errors.New("INVALID_NODE")
	ErrNodeUnavailable  = errors.New("NODE_NOT_AVAILABLE")
	ErrEncounterActive  = errors.New("ENCOUNTER_IN_PROGRESS")
)

type DepthSpec struct {
	Depth    int      `json:"depth"`
	MinNodes int      `json:"min_nodes"`
	MaxNodes int      `json:"max_nodes"`
	RoomType RoomType `json:"room_type,omitempty"`
}

type Config struct {
	FloorCount              int              `json:"floor_count"`
	Depths                  []DepthSpec      `json:"depths"`
	RoomWeights             map[RoomType]int `json:"room_weights"`
	DifficultyMultipliers   []float64        `json:"difficulty_multipliers"`
	GuaranteeRestOrTreasure bool             `json:"guarantee_rest_or_treasure"`
}

type Node struct {
	ID          string     `json:"id"`
	Floor       int        `json:"floor"`
	Depth       int        `json:"depth"`
	Type        RoomType   `json:"type"`
	Status      NodeStatus `json:"status"`
	Connections []string   `json:"connections"`
}

type Floor struct {
	Number               int     `json:"number"`
	DifficultyMultiplier float64 `json:"difficulty_multiplier"`
	Nodes                []Node  `json:"nodes"`
}

// State is private per-player dungeon progression. Seed remains server-only.
type State struct {
	Seed             uint64  `json:"-"`
	CurrentFloor     int     `json:"current_floor"`
	CurrentNodeID    string  `json:"current_node_id"`
	CheckpointNodeID string  `json:"checkpoint_node_id"`
	ActiveNodeID     string  `json:"active_node_id,omitempty"`
	Progress         int     `json:"progress"`
	Completed        bool    `json:"completed"`
	Floors           []Floor `json:"floors"`
}

type PrivateView struct {
	CurrentFloor     int     `json:"current_floor"`
	CurrentNodeID    string  `json:"current_node_id"`
	CheckpointNodeID string  `json:"checkpoint_node_id"`
	ActiveNodeID     string  `json:"active_node_id,omitempty"`
	Progress         int     `json:"progress"`
	Completed        bool    `json:"completed"`
	Floors           []Floor `json:"floors"`
}

// Load reads and validates the map-generation settings from JSON.
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read dungeon config: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse dungeon config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, fmt.Errorf("parse dungeon config: expected one JSON object")
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrInvalidMapConfig, err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.FloorCount < 1 || len(c.DifficultyMultipliers) != c.FloorCount {
		return fmt.Errorf("floor_count must match difficulty_multipliers")
	}
	for _, multiplier := range c.DifficultyMultipliers {
		if multiplier <= 0 {
			return fmt.Errorf("difficulty multipliers must be positive")
		}
	}
	if len(c.Depths) < 5 {
		return fmt.Errorf("at least five depth specifications are required")
	}
	for i, spec := range c.Depths {
		if spec.Depth != i || spec.MinNodes < 1 || spec.MaxNodes < spec.MinNodes {
			return fmt.Errorf("depth %d has invalid range or order", i)
		}
	}
	if c.Depths[0].MinNodes != 1 || c.Depths[0].MaxNodes != 1 || c.Depths[0].RoomType != StartRoom {
		return fmt.Errorf("depth 0 must contain one START node")
	}
	lastIdx := len(c.Depths) - 1
	if c.Depths[lastIdx].MinNodes != 1 || c.Depths[lastIdx].MaxNodes != 1 || c.Depths[lastIdx].RoomType != BossRoom {
		return fmt.Errorf("last depth must contain one BOSS node")
	}
	if c.Depths[1].MinNodes < 2 || c.Depths[lastIdx-1].MinNodes < 2 {
		return fmt.Errorf("depths 1 and %d must have at least two nodes", lastIdx-1)
	}
	weightSum := 0
	for roomType, weight := range c.RoomWeights {
		if !isDecisionRoom(roomType) || weight < 0 {
			return fmt.Errorf("invalid room weight for %q", roomType)
		}
		weightSum += weight
	}
	if weightSum <= 0 {
		return fmt.Errorf("room weights must add to more than zero")
	}
	return nil
}

// Generate creates an independent three-floor graph from a server-generated seed.
func Generate(seed uint64, cfg Config) (*State, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMapConfig, err)
	}
	rng := rand.New(rand.NewSource(int64(seed)))
	state := &State{Seed: seed, CurrentFloor: 1, Floors: make([]Floor, cfg.FloorCount)}
	for floorIndex := 0; floorIndex < cfg.FloorCount; floorIndex++ {
		floorNumber := floorIndex + 1
		floor := Floor{Number: floorNumber, DifficultyMultiplier: cfg.DifficultyMultipliers[floorIndex]}
		layers := make([][]int, len(cfg.Depths))
		lastDepth := len(cfg.Depths) - 1
		for _, spec := range cfg.Depths {
			count := spec.MinNodes
			if spec.MaxNodes > spec.MinNodes {
				count += rng.Intn(spec.MaxNodes - spec.MinNodes + 1)
			}
			for nodeIndex := 0; nodeIndex < count; nodeIndex++ {
				typeForNode := spec.RoomType
				if spec.Depth > 0 && spec.Depth < lastDepth {
					typeForNode = weightedRoom(rng, cfg.RoomWeights)
				}
				node := Node{ID: fmt.Sprintf("f%d-d%d-n%d", floorNumber, spec.Depth, nodeIndex), Floor: floorNumber, Depth: spec.Depth, Type: typeForNode, Status: Locked, Connections: []string{}}
				floor.Nodes = append(floor.Nodes, node)
				layers[spec.Depth] = append(layers[spec.Depth], len(floor.Nodes)-1)
			}
		}
		for depth := 0; depth < len(layers)-1; depth++ {
			connectLayers(rng, &floor, layers[depth], layers[depth+1])
		}
		if cfg.GuaranteeRestOrTreasure {
			guaranteeRestOrTreasure(rng, &floor, layers)
		}
		state.Floors[floorIndex] = floor
	}
	first := &state.Floors[0]
	startIndex := findNodeIndex(first, "f1-d0-n0")
	first.Nodes[startIndex].Status = Cleared
	state.CurrentNodeID = first.Nodes[startIndex].ID
	state.CheckpointNodeID = state.CurrentNodeID
	state.unlockConnected(first, state.CurrentNodeID)
	return state, nil
}

func (s *State) Move(nodeID string) error {
	if s.ActiveNodeID != "" {
		return ErrEncounterActive
	}
	floor := s.currentFloor()
	if floor == nil {
		return ErrInvalidNode
	}
	targetIndex := findNodeIndex(floor, nodeID)
	if targetIndex < 0 {
		return ErrInvalidNode
	}
	if floor.Nodes[targetIndex].Status != Available {
		return ErrNodeUnavailable
	}
	floor.Nodes[targetIndex].Status = InProgress
	s.ActiveNodeID = nodeID
	return nil
}

func (s *State) ResolveActive(skipped bool) error {
	if s.ActiveNodeID == "" {
		return ErrInvalidNode
	}
	floor := s.currentFloor()
	if floor == nil {
		return ErrInvalidNode
	}
	activeIndex := findNodeIndex(floor, s.ActiveNodeID)
	if activeIndex < 0 {
		return ErrInvalidNode
	}
	s.CurrentNodeID = s.ActiveNodeID
	status := Cleared
	if skipped {
		status = Skipped
	}
	floor.Nodes[activeIndex].Status = status
	s.Progress = floor.Nodes[activeIndex].Depth
	if floor.Nodes[activeIndex].Type == BossRoom {
		s.CheckpointNodeID = s.ActiveNodeID
	} else if floor.Nodes[activeIndex].Type == RestRoom {
		s.CheckpointNodeID = s.ActiveNodeID
	}
	s.ActiveNodeID = ""
	s.unlockConnected(floor, s.CurrentNodeID)
	return nil
}

// Respawn resets an unfinished node and returns the player to the latest checkpoint.
func (s *State) Respawn() error {
	floor := s.currentFloor()
	if floor == nil {
		return ErrInvalidNode
	}
	if s.ActiveNodeID != "" {
		if i := findNodeIndex(floor, s.ActiveNodeID); i >= 0 {
			floor.Nodes[i].Status = Available
		}
	}
	s.ActiveNodeID = ""
	s.CurrentNodeID = s.CheckpointNodeID
	s.Progress = 0
	if i := findNodeIndex(floor, s.CurrentNodeID); i < 0 {
		return ErrInvalidNode
	}
	s.unlockConnected(floor, s.CurrentNodeID)
	return nil
}

func (s *State) PrivateSnapshot() PrivateView {
	floors := make([]Floor, len(s.Floors))
	for i, floor := range s.Floors {
		floors[i] = Floor{Number: floor.Number, DifficultyMultiplier: floor.DifficultyMultiplier, Nodes: make([]Node, len(floor.Nodes))}
		for j, node := range floor.Nodes {
			node.Connections = append([]string(nil), node.Connections...)
			floors[i].Nodes[j] = node
		}
	}
	return PrivateView{CurrentFloor: s.CurrentFloor, CurrentNodeID: s.CurrentNodeID, CheckpointNodeID: s.CheckpointNodeID, ActiveNodeID: s.ActiveNodeID, Progress: s.Progress, Completed: s.Completed, Floors: floors}
}

func (s *State) AdvanceFloor() (bool, error) {
	if s.CurrentFloor >= len(s.Floors) {
		s.Completed = true
		return true, nil
	}
	s.CurrentFloor++
	floor := s.currentFloor()
	if floor == nil {
		return false, ErrInvalidNode
	}
	idx := findNodeIndex(floor, fmt.Sprintf("f%d-d0-n0", s.CurrentFloor))
	if idx < 0 {
		idx = 0
	}
	floor.Nodes[idx].Status = Cleared
	s.CurrentNodeID = floor.Nodes[idx].ID
	s.CheckpointNodeID = s.CurrentNodeID
	s.ActiveNodeID = ""
	s.Progress = 0
	s.unlockConnected(floor, s.CurrentNodeID)
	return false, nil
}

func (s *State) currentFloor() *Floor {
	for i := range s.Floors {
		if s.Floors[i].Number == s.CurrentFloor {
			return &s.Floors[i]
		}
	}
	return nil
}

func (s *State) unlockConnected(floor *Floor, nodeID string) {
	index := findNodeIndex(floor, nodeID)
	if index < 0 {
		return
	}
	for _, targetID := range floor.Nodes[index].Connections {
		targetIndex := findNodeIndex(floor, targetID)
		if targetIndex >= 0 && floor.Nodes[targetIndex].Status == Locked {
			floor.Nodes[targetIndex].Status = Available
		}
	}
}

func connectLayers(rng *rand.Rand, floor *Floor, previous, next []int) {
	for _, from := range previous {
		to := next[rng.Intn(len(next))]
		addConnection(&floor.Nodes[from], floor.Nodes[to].ID)
	}
	for _, to := range next {
		from := previous[rng.Intn(len(previous))]
		addConnection(&floor.Nodes[from], floor.Nodes[to].ID)
	}
	for _, from := range previous {
		sort.Strings(floor.Nodes[from].Connections)
	}
}

func addConnection(node *Node, targetID string) {
	for _, existing := range node.Connections {
		if existing == targetID {
			return
		}
	}
	node.Connections = append(node.Connections, targetID)
}

func weightedRoom(rng *rand.Rand, weights map[RoomType]int) RoomType {
	total := 0
	for _, weight := range weights {
		total += weight
	}
	if total <= 0 {
		return MonsterRoom
	}
	choice := rng.Intn(total)
	for _, roomType := range []RoomType{MonsterRoom, TreasureRoom, ShopRoom, EventRoom, RestRoom, EliteRoom, PuzzleRoom} {
		weight := weights[roomType]
		if choice < weight {
			return roomType
		}
		choice -= weight
	}
	return MonsterRoom
}

func guaranteeRestOrTreasure(rng *rand.Rand, floor *Floor, layers [][]int) {
	if len(layers) < 3 {
		return
	}
	for depth := 1; depth < len(layers)-1; depth++ {
		for _, index := range layers[depth] {
			if floor.Nodes[index].Type == RestRoom || floor.Nodes[index].Type == TreasureRoom {
				return
			}
		}
	}
	layer := 1 + rng.Intn(len(layers)-2)
	index := layers[layer][rng.Intn(len(layers[layer]))]
	if rng.Intn(2) == 0 {
		floor.Nodes[index].Type = RestRoom
	} else {
		floor.Nodes[index].Type = TreasureRoom
	}
}

func findNodeIndex(floor *Floor, nodeID string) int {
	for i := range floor.Nodes {
		if floor.Nodes[i].ID == nodeID {
			return i
		}
	}
	return -1
}

func isDecisionRoom(roomType RoomType) bool {
	return roomType == MonsterRoom || roomType == TreasureRoom || roomType == ShopRoom || roomType == EventRoom || roomType == RestRoom || roomType == EliteRoom || roomType == PuzzleRoom
}
