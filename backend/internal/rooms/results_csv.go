package rooms

import (
	"bytes"
	"encoding/csv"
	"strconv"
)

// ResultsCSV returns the final in-memory scoreboard as UTF-8 CSV.
func (m *Manager) ResultsCSV(code string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rooms[code]
	if r == nil {
		return nil, ErrRoomNotFound
	}
	if r.Phase != "results" {
		return nil, ErrInvalidPhase
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.Write([]string{"rank", "player_id", "name", "total_score", "pve_score", "pvp_score", "gold_score", "bonus", "penalties", "pvp_damage", "pvp_knockouts", "pvp_deaths", "bosses_defeated", "gold"})
	for _, entry := range r.Results {
		_ = writer.Write([]string{strconv.Itoa(entry.Rank), entry.PlayerID, entry.Name, strconv.Itoa(entry.TotalScore), strconv.Itoa(entry.PvEScore), strconv.Itoa(entry.PvPScore), strconv.Itoa(entry.GoldScore), strconv.Itoa(entry.Bonus), strconv.Itoa(entry.Penalties), strconv.Itoa(entry.PvPDamage), strconv.Itoa(entry.PvPKnockouts), strconv.Itoa(entry.PvPDeaths), strconv.Itoa(entry.BossesDefeated), strconv.Itoa(entry.Gold)})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
