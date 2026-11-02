package rooms

import (
	"errors"
	"fmt"
	"sort"

	"dungeon-rivals/backend/internal/combat"
)

var ErrPvPActionBudget = errors.New("PVP_ACTION_BUDGET_EXHAUSTED")

func (m *Manager) pvpAction(r *room, playerID, action, targetID, itemID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "pvp" {
		return ErrInvalidPhase
	}
	p := findPlayer(r, playerID)
	if p == nil {
		return ErrPlayerNotFound
	}
	if p.PvPActionsUsed >= 5 {
		return ErrPvPActionBudget
	}
	turn := p.PvPActionsUsed + 1
	switch action {
	case "ATTACK":
		target := findPlayer(r, targetID)
		if target == nil || target == p || target.State == nil {
			return ErrInvalidChoice
		}
		before := target.State.HP
		if before <= 0 {
			return ErrInvalidChoice
		}
		damage := combat.Damage(p.State.Stats(m.catalog).ATK, target.State.Stats(m.catalog).DEF)
		if target.PvPGuarding {
			damage = max(1, damage/2)
			target.PvPGuarding = false
		}
		target.State.Damage(damage)
		dealt := before - target.State.HP
		oldPoints := p.PvPDamage / 5
		p.PvPDamage += dealt
		p.PvPScoreRemainder = p.PvPDamage % 5
		if points := p.PvPDamage/5 - oldPoints; points > 0 {
			if m.recordScoreEvent(r, ScoreEvent{ID: fmt.Sprintf("pvp-damage:%s:%d", p.ID, turn), PlayerID: p.ID, Category: "pvp", Source: "pvp_damage", Points: points}) {
				p.PvPScore += points
			}
		}
		if target.State.HP == 0 {
			target.PvPDeaths++
			if m.recordScoreEvent(r, ScoreEvent{ID: fmt.Sprintf("pvp-knockout:%s:%d", p.ID, turn), PlayerID: p.ID, Category: "pvp", Source: "pvp_knockout", Points: 30}) {
				p.PvPKnockouts++
				p.PvPScore += 30
			}
			if m.recordScoreEvent(r, ScoreEvent{ID: fmt.Sprintf("pvp-death:%s:%d", target.ID, target.PvPDeaths), PlayerID: target.ID, Category: "penalty", Source: "pvp_death", Points: -10}) {
				target.PvPPenalty += 10
			}
			target.State.Score = target.PveScore + target.PvPScore - target.PvPPenalty
			maxHP := target.State.Stats(m.catalog).MaxHP
			target.State.RestoreHP((maxHP+1)/2, m.catalog)
		}
	case "DEFEND":
		p.PvPGuarding = true
	case "USE_ITEM":
		if itemID == "" {
			return errors.New("INVALID_REQUEST")
		}
		if err := p.State.Use(itemID, "pvp", m.catalog); err != nil {
			return err
		}
	default:
		return combat.ErrInvalidAction
	}
	p.PvPActionsUsed++
	p.State.Score = p.PveScore + p.PvPScore - p.PvPPenalty
	if m.allPvPActionsUsed(r) {
		m.finishPvPLocked(r)
	}
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) recordScoreEvent(r *room, event ScoreEvent) bool {
	if r.ScoreEventIDs == nil {
		r.ScoreEventIDs = make(map[string]struct{})
	}
	if _, exists := r.ScoreEventIDs[event.ID]; exists {
		return false
	}
	r.ScoreEventIDs[event.ID] = struct{}{}
	r.ScoreEvents = append(r.ScoreEvents, event)
	return true
}

func (m *Manager) allPvPActionsUsed(r *room) bool {
	if len(r.Players) == 0 {
		return false
	}
	for _, p := range r.Players {
		if p.PvPActionsUsed < 5 {
			return false
		}
	}
	return true
}

func (m *Manager) forceFinishPvp(r *room, playerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "pvp" {
		return ErrInvalidPhase
	}
	if r.HostID != playerID {
		return ErrNotHost
	}
	disconnected := false
	for _, p := range r.Players {
		if !p.Connected {
			p.PvPActionsUsed = 5
			disconnected = true
		}
	}
	if !disconnected {
		return errors.New("FORCE_FINISH_REQUIRES_DISCONNECTED_PLAYER")
	}
	m.finishPvPLocked(r)
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) finishPvPLocked(r *room) {
	if r.Phase == "results" {
		return
	}
	maxScore := 0
	for _, p := range r.Players {
		if p.PvPScore > maxScore {
			maxScore = p.PvPScore
		}
	}
	for _, p := range r.Players {
		if p.PvPScore == maxScore {
			p.PvPBonus = 50
			_ = m.recordScoreEvent(r, ScoreEvent{ID: "pvp-leader:" + p.ID, PlayerID: p.ID, Category: "pvp", Source: "pvp_leader_bonus", Points: 50})
		}
		goldScore := p.State.Gold / 10
		_ = m.recordScoreEvent(r, ScoreEvent{ID: "gold-conversion:" + p.ID, PlayerID: p.ID, Category: "economy", Source: "gold_conversion", Points: goldScore})
		p.State.Score = p.PveScore + p.PvPScore + p.PvPBonus + goldScore - p.PvPPenalty
	}
	results := make([]ResultEntry, 0, len(r.Players))
	for _, p := range r.Players {
		results = append(results, ResultEntry{PlayerID: p.ID, Name: p.Name, PvEScore: p.PveScore, PvPScore: p.PvPScore, GoldScore: p.State.Gold / 10, Bonus: p.PvPBonus, Penalties: p.PvPPenalty, TotalScore: p.State.Score, PvPDamage: p.PvPDamage, PvPKnockouts: p.PvPKnockouts, PvPDeaths: p.PvPDeaths, BossesDefeated: p.BossesDefeated, Gold: p.State.Gold})
	}
	sort.SliceStable(results, func(i, j int) bool { return resultBefore(results[i], results[j]) })
	for i := range results {
		if i == 0 || !resultTied(results[i-1], results[i]) {
			results[i].Rank = i + 1
		} else {
			results[i].Rank = results[i-1].Rank
		}
	}
	r.Results = results
	r.Phase = "results"
	r.Version++
	m.broadcastLocked(r, "game.phase_changed", map[string]any{"phase": r.Phase})
}

func resultBefore(a, b ResultEntry) bool {
	if a.TotalScore != b.TotalScore {
		return a.TotalScore > b.TotalScore
	}
	if a.PvPScore != b.PvPScore {
		return a.PvPScore > b.PvPScore
	}
	if a.PvPDamage != b.PvPDamage {
		return a.PvPDamage > b.PvPDamage
	}
	if a.BossesDefeated != b.BossesDefeated {
		return a.BossesDefeated > b.BossesDefeated
	}
	return a.Gold > b.Gold
}
func resultTied(a, b ResultEntry) bool {
	return a.TotalScore == b.TotalScore && a.PvPScore == b.PvPScore && a.PvPDamage == b.PvPDamage && a.BossesDefeated == b.BossesDefeated && a.Gold == b.Gold
}
