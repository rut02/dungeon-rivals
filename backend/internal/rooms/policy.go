package rooms

import "errors"

func (m *Manager) kickDisconnected(r *room, hostID, targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil || m.rooms[r.Code] != r {
		return ErrRoomNotFound
	}
	if r.Phase != "dungeon" && r.Phase != "preparation" {
		return ErrInvalidPhase
	}
	if r.HostID != hostID {
		return ErrNotHost
	}
	index := -1
	for i, p := range r.Players {
		if p.ID == targetID {
			index = i
			break
		}
	}
	if index < 0 {
		return ErrPlayerNotFound
	}
	if r.Players[index].Connected {
		return errors.New("PLAYER_STILL_CONNECTED")
	}
	if targetID == hostID {
		return ErrNotHost
	}
	r.Players = append(r.Players[:index], r.Players[index+1:]...)
	if len(r.Players) == 0 {
		delete(m.rooms, r.Code)
		return nil
	}
	if r.Phase == "dungeon" {
		m.maybePreparation(r)
	} else {
		allReady := true
		for _, p := range r.Players {
			if !p.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			m.startPvPLocked(r)
		}
	}
	r.Version++
	m.broadcastLocked(r, "room.player_kicked", map[string]any{"player_id": targetID})
	m.broadcastSnapshotsLocked(r)
	return nil
}
