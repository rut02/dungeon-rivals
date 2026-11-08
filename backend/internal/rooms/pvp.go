package rooms

import (
	"errors"
	"fmt"
	mathrand "math/rand"
	"sort"

	"dungeon-rivals/backend/internal/combat"
	"dungeon-rivals/backend/internal/playerstate"
)

func (m *Manager) initPvPTournamentLocked(r *room) {
	r.PvPBouts = nil
	r.CurrentBoutIdx = 0
	r.PvPLog = []string{"⚔️ ศึกประลองจัดอันดับ (Round-Robin Arena) เริ่มต้นขึ้นแล้ว! สู้กันตัวต่อตัวจนกว่า HP ฝ่ายใดจะเป็น 0!"}

	n := len(r.Players)
	boutNum := 1
	// Schedule human bouts first so human players immediately fight their Round-Robin opponents, followed by bot-vs-bot bouts
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				pA := r.Players[i]
				pB := r.Players[j]
				bothBots := pA.IsBot && pB.IsBot
				if (pass == 0 && !bothBots) || (pass == 1 && bothBots) {
					firstTurnID := pA.ID
					aSpeed := pA.State.Stats(m.catalog).SPD
					bSpeed := pB.State.Stats(m.catalog).SPD
					if bSpeed > aSpeed || (bSpeed == aSpeed && mathrand.Intn(2) == 1) {
						firstTurnID = pB.ID
					}
					r.PvPBouts = append(r.PvPBouts, PvPBout{
						BoutNumber:          boutNum,
						PlayerAID:           pA.ID,
						PlayerBID:           pB.ID,
						CurrentTurnPlayerID: firstTurnID,
					})
					boutNum++
				}
			}
		}
	}
	m.advancePvPBoutsLocked(r)
}

func (m *Manager) currentBoutLocked(r *room) *PvPBout {
	if r == nil || r.CurrentBoutIdx < 0 || r.CurrentBoutIdx >= len(r.PvPBouts) {
		return nil
	}
	return &r.PvPBouts[r.CurrentBoutIdx]
}

func (m *Manager) restoreFighterForBout(p *player) {
	if p == nil || p.State == nil {
		return
	}
	stats := p.State.Stats(m.catalog)
	p.State.RestoreHP(stats.MaxHP, m.catalog)
	p.State.RestoreMP(stats.MaxMP, m.catalog)
	p.PvPGuarding = false
	p.PvPSmoke = false
	p.PvPFirstHitGuardUsed = false
	p.PvPBarrier = 0
	p.PvPLifeStealRemainder = 0
	p.PvPTempSPD = 0
	p.PvPTempSPDTurns = 0
	p.PvPEquipmentCooldowns = nil
}

func (m *Manager) appendPvPLog(r *room, msg string) {
	r.PvPLog = append(r.PvPLog, msg)
	if len(r.PvPLog) > 36 {
		r.PvPLog = append([]string(nil), r.PvPLog[len(r.PvPLog)-36:]...)
	}
}

func (m *Manager) advancePvPBoutsLocked(r *room) {
	for r.CurrentBoutIdx < len(r.PvPBouts) {
		bout := &r.PvPBouts[r.CurrentBoutIdx]
		if bout.Finished {
			r.CurrentBoutIdx++
			continue
		}
		pA := findPlayer(r, bout.PlayerAID)
		pB := findPlayer(r, bout.PlayerBID)
		if pA == nil || pB == nil || pA.State == nil || pB.State == nil {
			bout.Finished = true
			r.CurrentBoutIdx++
			continue
		}
		if bout.Turns == 0 {
			m.restoreFighterForBout(pA)
			m.restoreFighterForBout(pB)
			tag := fmt.Sprintf("🔔 คู่ที่ %d/%d", bout.BoutNumber, len(r.PvPBouts))
			m.appendPvPLog(r, fmt.Sprintf("%s: %s ปะทะ %s (HP/MP ฟื้นฟูเต็ม 100%%)", tag, pA.Name, pB.Name))
		}
		if bout.CurrentTurnPlayerID == "" {
			bout.CurrentTurnPlayerID = bout.PlayerAID
		}
		if pA.IsBot && pB.IsBot {
			m.simulateBotVsBotBoutLocked(r, bout, pA, pB)
			r.CurrentBoutIdx++
			continue
		}
		// Bots take their turn automatically, including when a bot is Player A.
		for {
			bot := findPlayer(r, bout.CurrentTurnPlayerID)
			if bot == nil || !bot.IsBot {
				break
			}
			opponent := pA
			if bot.ID == pA.ID {
				opponent = pB
			}
			m.executeBotBoutTurnLocked(r, bout, bot, opponent)
			bout.Turns++
			if opponent.State.HP <= 0 {
				m.recordBoutWinLocked(r, bout, bot, opponent)
				r.CurrentBoutIdx++
				break
			}
			bout.CurrentTurnPlayerID = opponent.ID
		}
		if bout.Finished {
			continue
		}
		// Active bout involves at least one human player
		return
	}
	// All scheduled round-robin bouts are complete; calculate the final standings.
	m.finishPvPLocked(r)
}

func (m *Manager) simulateBotVsBotBoutLocked(r *room, bout *PvPBout, pA, pB *player) {
	for pA.State.HP > 0 && pB.State.HP > 0 {
		actor, opponent := pA, pB
		if bout.CurrentTurnPlayerID == pB.ID {
			actor, opponent = pB, pA
		}
		m.executeBotBoutTurnLocked(r, bout, actor, opponent)
		bout.Turns++
		if opponent.State.HP <= 0 {
			m.recordBoutWinLocked(r, bout, actor, opponent)
			return
		}
		bout.CurrentTurnPlayerID = opponent.ID
	}
}

func (m *Manager) recordBoutWinLocked(r *room, bout *PvPBout, winner, loser *player) {
	if bout.Finished {
		return
	}
	bout.Finished = true
	bout.WinnerID = winner.ID
	winner.PvPWins++
	loser.PvPLosses++
	loser.PvPDeaths++

	koEventID := fmt.Sprintf("pvp-knockout:%s:bout-%d", winner.ID, bout.BoutNumber)
	if m.recordScoreEvent(r, ScoreEvent{ID: koEventID, PlayerID: winner.ID, Category: "pvp", Source: "pvp_knockout", Points: 30}) {
		winner.PvPKnockouts++
		winner.PvPScore += 30
	}
	deathEventID := fmt.Sprintf("pvp-death:%s:bout-%d", loser.ID, bout.BoutNumber)
	if m.recordScoreEvent(r, ScoreEvent{ID: deathEventID, PlayerID: loser.ID, Category: "penalty", Source: "pvp_death", Points: -10}) {
		loser.PvPPenalty += 10
	}
	winner.State.Score = winner.PveScore + winner.PvPScore - winner.PvPPenalty
	loser.State.Score = loser.PveScore + loser.PvPScore - loser.PvPPenalty

	m.appendPvPLog(r, fmt.Sprintf("🏆 %s เอาชนะ %s ในคู่ที่ %d! (+30 คะแนน PvP, ชนะสะสม %d ครั้ง)", winner.Name, loser.Name, bout.BoutNumber, winner.PvPWins))
}

func (m *Manager) addPvPDamageScoreLocked(r *room, attacker *player, dealt int) {
	if dealt <= 0 {
		return
	}
	oldPoints := attacker.PvPDamage / 5
	attacker.PvPDamage += dealt
	attacker.PvPScoreRemainder = attacker.PvPDamage % 5
	if points := attacker.PvPDamage/5 - oldPoints; points > 0 {
		eventID := fmt.Sprintf("pvp-damage:%s:%d:%d", attacker.ID, attacker.PvPActionsUsed, attacker.PvPDamage)
		if m.recordScoreEvent(r, ScoreEvent{ID: eventID, PlayerID: attacker.ID, Category: "pvp", Source: "pvp_damage", Points: points}) {
			attacker.PvPScore += points
		}
	}
	attacker.State.Score = attacker.PveScore + attacker.PvPScore - attacker.PvPPenalty
}

func (m *Manager) applyPvPDefenseModifiers(attacker, defender *player, rawDamage int, tStats playerstate.StatsSnapshot) int {
	damage := max(1, rawDamage)
	if defender.PvPSmoke {
		damage = max(1, damage/4)
		defender.PvPSmoke = false
		defender.PvPGuarding = false
		return damage
	}
	if chance := min(20, m.equipmentAbilityValue(defender, "evasion")); chance > 0 && mathrand.Intn(100) < chance {
		defender.PvPGuarding = false
		return 0
	}
	if !defender.PvPFirstHitGuardUsed {
		if reduction := m.equipmentAbilityValue(defender, "first_hit_reduction"); reduction > 0 {
			damage = max(1, damage*(100-reduction)/100)
			defender.PvPFirstHitGuardUsed = true
		}
	}
	if defender.PvPBarrier > 0 {
		blocked := min(damage, defender.PvPBarrier)
		damage -= blocked
		defender.PvPBarrier -= blocked
	}
	if damage <= 0 {
		defender.PvPGuarding = false
		return 0
	}
	if defender.PvPGuarding {
		if tStats.Archetype == "daggers" && mathrand.Intn(100) < 50 {
			damage = 0
		} else if tStats.Archetype == "sword" && defender.State != nil && defender.State.Equipment.Offhand != "" {
			damage = max(1, int(float64(damage)*0.35))
			reflect := max(2, tStats.DEF)
			attacker.State.Damage(reflect)
		} else if tStats.Archetype == "staff" {
			damage = max(1, int(float64(damage)*0.40))
		} else {
			damage = max(1, damage/2)
		}
		defender.PvPGuarding = false
	}
	return damage
}

func (m *Manager) healFromPvPDamage(p *player, dealt int) int {
	percent := m.equipmentAbilityValue(p, "lifesteal")
	if p == nil || p.State == nil || percent <= 0 || dealt <= 0 {
		return 0
	}
	p.PvPLifeStealRemainder += dealt * percent
	healing := p.PvPLifeStealRemainder / 100
	p.PvPLifeStealRemainder %= 100
	if healing <= 0 {
		return 0
	}
	before := p.State.HP
	p.State.RestoreHP(before+healing, m.catalog)
	return p.State.HP - before
}

func (m *Manager) tickPvPEquipmentEffects(p *player) {
	if p.PvPTempSPDTurns > 0 {
		p.PvPTempSPDTurns--
		if p.PvPTempSPDTurns == 0 {
			p.PvPTempSPD = 0
		}
	}
	for id, turns := range p.PvPEquipmentCooldowns {
		if turns <= 1 {
			delete(p.PvPEquipmentCooldowns, id)
		} else {
			p.PvPEquipmentCooldowns[id] = turns - 1
		}
	}
}

func (m *Manager) resolveBoutOpponentLocked(r *room, bout *PvPBout, actor *player, targetID string) *player {
	if bout != nil {
		if bout.PlayerAID == actor.ID {
			if opp := findPlayer(r, bout.PlayerBID); opp != nil && opp != actor {
				return opp
			}
		} else if bout.PlayerBID == actor.ID {
			if opp := findPlayer(r, bout.PlayerAID); opp != nil && opp != actor {
				return opp
			}
		}
	}
	if targetID != "" {
		if t := findPlayer(r, targetID); t != nil && t != actor {
			return t
		}
	}
	for _, candidate := range r.Players {
		if candidate.ID != actor.ID && candidate.State != nil {
			return candidate
		}
	}
	return nil
}

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
	if p == nil || p.State == nil {
		return ErrPlayerNotFound
	}
	if len(r.PvPBouts) == 0 {
		m.initPvPTournamentLocked(r)
	}
	bout := m.currentBoutLocked(r)
	if bout == nil {
		m.finishPvPLocked(r)
		r.Version++
		m.broadcastSnapshotsLocked(r)
		return nil
	}
	if bout.CurrentTurnPlayerID != playerID {
		return errors.New("PVP_NOT_YOUR_TURN")
	}
	target := m.resolveBoutOpponentLocked(r, bout, p, targetID)
	if target == nil || target.State == nil {
		return ErrInvalidChoice
	}

	pStats := p.State.Stats(m.catalog)
	if p.PvPTempSPDTurns > 0 {
		pStats.SPD += p.PvPTempSPD
	}
	tStats := target.State.Stats(m.catalog)

	switch action {
	case "ATTACK":
		if target.State.HP <= 0 {
			return ErrInvalidChoice
		}
		rawDmg := combat.Damage(pStats.ATK, tStats.DEF)
		critText := ""
		critChance := min(50, 30+m.equipmentAbilityValue(p, "rogue_crit"))
		if pStats.Archetype == "daggers" && mathrand.Intn(100) < critChance {
			rawDmg = int(float64(rawDmg) * 1.75)
			critText = " (คริติคอล!)"
		} else if pStats.Archetype == "staff" {
			rawDmg = combat.Damage(pStats.ATK, tStats.DEF/2)
		}
		damage := m.applyPvPDefenseModifiers(p, target, rawDmg, tStats)
		before := target.State.HP
		target.State.Damage(damage)
		dealt := before - target.State.HP
		m.addPvPDamageScoreLocked(r, p, dealt)
		if healed := m.healFromPvPDamage(p, dealt); healed > 0 {
			m.appendPvPLog(r, fmt.Sprintf("🩸 Blood Chalice ฟื้น HP %d ให้ %s", healed, p.Name))
		}
		m.appendPvPLog(r, fmt.Sprintf("⚔️ %s โจมตี %s สร้างความเสียหาย %d หน่วย%s", p.Name, target.Name, dealt, critText))
		if target.State.HP > 0 && p.State.HP > 0 && combat.ExtraStrikeEligible(pStats.SPD, tStats.SPD) {
			followUpRaw := max(1, int(float64(rawDmg)*0.40))
			followUpDamage := m.applyPvPDefenseModifiers(p, target, followUpRaw, tStats)
			before = target.State.HP
			target.State.Damage(followUpDamage)
			followUpDealt := before - target.State.HP
			m.addPvPDamageScoreLocked(r, p, followUpDealt)
			if healed := m.healFromPvPDamage(p, followUpDealt); healed > 0 {
				m.appendPvPLog(r, fmt.Sprintf("🩸 Blood Chalice ฟื้น HP %d ให้ %s", healed, p.Name))
			}
			m.appendPvPLog(r, fmt.Sprintf("⚡ %s เร็วกว่า (SPD %d vs %d) โจมตีซ้ำปกติอีก 1 ครั้ง สร้าง %d หน่วย (40%%)", p.Name, pStats.SPD, tStats.SPD, followUpDealt))
		}

	case "SKILL":
		if target.State.HP <= 0 {
			return ErrInvalidChoice
		}
		cost := pStats.WeaponSkillCost
		if cost <= 0 {
			cost = 5
		}
		cost = max(1, cost-m.equipmentAbilityValue(p, "skill_cost"))
		if !p.State.ConsumeMP(cost) {
			return playerstate.ErrNotEnoughMP
		}
		skillName := pStats.WeaponSkill
		if skillName == "" {
			skillName = "ท่าไม้ตายอาวุธ"
		}
		skillATK := (pStats.ATK*(100+m.equipmentAbilityValue(p, "skill_damage")) + 99) / 100
		var rawDmg int
		switch pStats.Archetype {
		case "sword":
			rawDmg = combat.Damage(int(float64(skillATK)*1.40)+pStats.DEF, tStats.DEF)
			p.PvPGuarding = true
		case "daggers":
			rawDmg = combat.Damage(int(float64(skillATK)*1.75), tStats.DEF/2)
		case "bow":
			rawDmg = combat.Damage(int(float64(skillATK)*1.55), tStats.DEF/2)
		case "staff":
			rawDmg = combat.Damage(int(float64(skillATK)*1.60), tStats.DEF/3)
		default:
			rawDmg = combat.Damage(int(float64(skillATK)*1.45), tStats.DEF/2)
		}
		damage := m.applyPvPDefenseModifiers(p, target, rawDmg, tStats)
		before := target.State.HP
		target.State.Damage(damage)
		dealt := before - target.State.HP
		m.addPvPDamageScoreLocked(r, p, dealt)
		if healed := m.healFromPvPDamage(p, dealt); healed > 0 {
			m.appendPvPLog(r, fmt.Sprintf("🩸 Blood Chalice ฟื้น HP %d ให้ %s", healed, p.Name))
		}
		m.appendPvPLog(r, fmt.Sprintf("⚡ %s ใช้สกิล %s (-%d MP) โจมตี %s สร้างความเสียหาย %d หน่วย!", p.Name, skillName, cost, target.Name, dealt))

	case "DEFEND":
		p.PvPGuarding = true
		p.PvPBarrier = m.equipmentAbilityValue(p, "guard_barrier")
		mpRecovery := 3
		if pStats.Archetype == "staff" {
			mpRecovery = 5
		}
		p.State.RestoreMP(mpRecovery, m.catalog)
		// Tactical counter-jab so bouts always progress
		jab := max(3, pStats.ATK/3)
		before := target.State.HP
		target.State.Damage(jab)
		dealt := before - target.State.HP
		m.addPvPDamageScoreLocked(r, p, dealt)
		m.appendPvPLog(r, fmt.Sprintf("🛡️ %s ตั้งการ์ดป้องกัน (+%d MP) และกระแทกสวนกลับ %s %d หน่วย", p.Name, mpRecovery, target.Name, dealt))
		if p.PvPBarrier > 0 {
			m.appendPvPLog(r, fmt.Sprintf("🛡️ บาเรียจากอุปกรณ์รับความเสียหาย %d หน่วย", p.PvPBarrier))
		}

	case "GEAR_ABILITY":
		itemDef, ok := m.catalog.Get(itemID)
		if !ok || itemDef.AbilityType != "active" || itemDef.AbilityEffect == nil || itemDef.AbilityEffect.Type != "speed_buff" || !playerHasEquipped(p, itemID) {
			return combat.ErrInvalidAction
		}
		if p.PvPEquipmentCooldowns[itemID] > 0 {
			return errors.New("EQUIPMENT_SKILL_ON_COOLDOWN")
		}
		if !p.State.ConsumeMP(itemDef.AbilityCost) {
			return playerstate.ErrNotEnoughMP
		}
		p.PvPTempSPD = max(p.PvPTempSPD, itemDef.AbilityEffect.Value)
		p.PvPTempSPDTurns = max(p.PvPTempSPDTurns, itemDef.AbilityEffect.Duration+1)
		if p.PvPEquipmentCooldowns == nil {
			p.PvPEquipmentCooldowns = make(map[string]int)
		}
		p.PvPEquipmentCooldowns[itemID] = itemDef.AbilityCooldown + 1
		m.appendPvPLog(r, fmt.Sprintf("💨 %s ใช้ %s: SPD +%d ชั่วคราว %d เทิร์น (-%d MP)", p.Name, itemDef.AbilityName, itemDef.AbilityEffect.Value, itemDef.AbilityEffect.Duration, itemDef.AbilityCost))

	case "USE_ITEM":
		if itemID == "" {
			return errors.New("INVALID_REQUEST")
		}
		itemDef, ok := m.catalog.Get(itemID)
		if !ok {
			return playerstate.ErrItemNotFound
		}
		if itemDef.Effect != nil && itemDef.Effect.Type == "damage_aoe" {
			if err := p.State.Remove(itemID, 1); err != nil {
				return err
			}
			bombDmg := itemDef.Effect.Value
			if bombDmg <= 0 {
				bombDmg = 30
			}
			before := target.State.HP
			target.State.Damage(bombDmg)
			dealt := before - target.State.HP
			m.addPvPDamageScoreLocked(r, p, dealt)
			m.appendPvPLog(r, fmt.Sprintf("💣 %s ปาระเบิดเพลิง %s ใส่ %s สร้างความเสียหายระเบิดโดยตรง %d หน่วย!", p.Name, itemDef.Name, target.Name, dealt))
		} else if itemDef.Effect != nil && itemDef.Effect.Type == "smoke" {
			if err := p.State.Remove(itemID, 1); err != nil {
				return err
			}
			p.PvPSmoke = true
			p.PvPGuarding = true
			ambushDmg := max(8, pStats.ATK)
			before := target.State.HP
			target.State.Damage(ambushDmg)
			dealt := before - target.State.HP
			m.addPvPDamageScoreLocked(r, p, dealt)
			m.appendPvPLog(r, fmt.Sprintf("💨 %s ปาระเบิดควัน %s อำพรางตัว (ลดดาเมจถัดไป 75%%) และลอบโจมตี %s %d หน่วย!", p.Name, itemDef.Name, target.Name, dealt))
		} else {
			if err := p.State.Use(itemID, "pvp", m.catalog); err != nil {
				return err
			}
			m.appendPvPLog(r, fmt.Sprintf("🧪 %s ใช้ไอเทม %s ฟื้นฟูพลังในสนามประลอง!", p.Name, itemDef.Name))
		}

	default:
		return combat.ErrInvalidAction
	}

	p.PvPActionsUsed++
	m.tickPvPEquipmentEffects(p)
	bout.Turns++
	p.State.Score = p.PveScore + p.PvPScore - p.PvPPenalty

	// Check if target or player was knocked out
	if target.State.HP <= 0 {
		m.recordBoutWinLocked(r, bout, p, target)
		r.CurrentBoutIdx++
		m.advancePvPBoutsLocked(r)
	} else if p.State.HP <= 0 {
		m.recordBoutWinLocked(r, bout, target, p)
		r.CurrentBoutIdx++
		m.advancePvPBoutsLocked(r)
	} else {
		bout.CurrentTurnPlayerID = target.ID
		if target.IsBot {
			// The bot answers immediately, then the turn returns to the human.
			m.executeBotBoutTurnLocked(r, bout, target, p)
			bout.Turns++
			if p.State.HP <= 0 {
				m.recordBoutWinLocked(r, bout, target, p)
				r.CurrentBoutIdx++
				m.advancePvPBoutsLocked(r)
			} else if target.State.HP <= 0 {
				m.recordBoutWinLocked(r, bout, p, target)
				r.CurrentBoutIdx++
				m.advancePvPBoutsLocked(r)
			} else {
				bout.CurrentTurnPlayerID = p.ID
			}
		}
	}

	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}

func (m *Manager) executeBotBoutTurnLocked(r *room, bout *PvPBout, bot, opponent *player) {
	if bot == nil || opponent == nil || bot.State == nil || opponent.State == nil || bot.State.HP <= 0 || opponent.State.HP <= 0 {
		return
	}
	param, ok := botDifficultyParams[bot.BotDifficulty]
	if !ok {
		param = botDifficultyParams["normal"]
	}
	bStats := bot.State.Stats(m.catalog)
	oStats := opponent.State.Stats(m.catalog)
	hpRatio := float64(bot.State.HP) / float64(max(1, bStats.MaxHP))

	// 1. Heal if low HP and has potion
	if hpRatio <= param.healThreshold {
		for _, potID := range []string{"greater_health_potion", "health_potion"} {
			if bot.State.Count(potID) > 0 {
				_ = bot.State.Use(potID, "pvp", m.catalog)
				bot.PvPActionsUsed++
				m.appendPvPLog(r, fmt.Sprintf("🧪 %s ดื่มยาฟื้นฟูพลังชีวิต!", bot.Name))
				return
			}
		}
	}

	// 2. Use Fire Bomb if available and opponent HP <= 35
	if opponent.State.HP <= 35 && bot.State.Count("fire_bomb") > 0 {
		if err := bot.State.Remove("fire_bomb", 1); err == nil {
			before := opponent.State.HP
			opponent.State.Damage(30)
			dealt := before - opponent.State.HP
			bot.PvPActionsUsed++
			m.addPvPDamageScoreLocked(r, bot, dealt)
			m.appendPvPLog(r, fmt.Sprintf("💣 %s ปาระเบิดเพลิงใส่ %s สร้างความเสียหาย %d หน่วย!", bot.Name, opponent.Name, dealt))
			return
		}
	}

	// 3. Use Weapon Skill if bot has enough MP
	skillCost := max(5, bStats.WeaponSkillCost)
	if bot.State.MP >= skillCost && mathrand.Intn(100) < 55 {
		if bot.State.ConsumeMP(skillCost) {
			rawDmg := combat.Damage(int(float64(bStats.ATK)*1.35*param.damageMult), oStats.DEF/2)
			damage := m.applyPvPDefenseModifiers(bot, opponent, rawDmg, oStats)
			before := opponent.State.HP
			opponent.State.Damage(damage)
			dealt := before - opponent.State.HP
			bot.PvPActionsUsed++
			m.addPvPDamageScoreLocked(r, bot, dealt)
			skillName := bStats.WeaponSkill
			if skillName == "" {
				skillName = "ท่าไม้ตาย"
			}
			m.appendPvPLog(r, fmt.Sprintf("⚡ %s ใช้สกิล %s โจมตี %s สร้างความเสียหาย %d หน่วย!", bot.Name, skillName, opponent.Name, dealt))
			return
		}
	}

	// 4. Normal Attack
	rawDmg := combat.Damage(bStats.ATK, oStats.DEF)
	damage := m.applyPvPDefenseModifiers(bot, opponent, max(1, int(float64(rawDmg)*param.damageMult)), oStats)
	before := opponent.State.HP
	opponent.State.Damage(damage)
	dealt := before - opponent.State.HP
	bot.PvPActionsUsed++
	m.addPvPDamageScoreLocked(r, bot, dealt)
	m.appendPvPLog(r, fmt.Sprintf("⚔️ %s โจมตีสวนกลับ %s สร้างความเสียหาย %d หน่วย", bot.Name, opponent.Name, dealt))
}

func (m *Manager) pvpStatusForPlayer(r *room, playerID string) (oppID string, boutNum int, totalBouts int) {
	if r == nil || len(r.PvPBouts) == 0 {
		return "", 0, 0
	}
	totalBouts = len(r.PvPBouts)
	bout := m.currentBoutLocked(r)
	if bout == nil {
		return "", totalBouts, totalBouts
	}
	boutNum = bout.BoutNumber
	if bout.PlayerAID == playerID {
		oppID = bout.PlayerBID
	} else if bout.PlayerBID == playerID {
		oppID = bout.PlayerAID
	} else {
		oppID = bout.PlayerBID
	}
	return oppID, boutNum, totalBouts
}

func (m *Manager) pvpBoutViews(r *room) []PvPBoutView {
	if r == nil || len(r.PvPBouts) == 0 {
		return nil
	}
	views := make([]PvPBoutView, 0, len(r.PvPBouts))
	for _, b := range r.PvPBouts {
		pA := findPlayer(r, b.PlayerAID)
		pB := findPlayer(r, b.PlayerBID)
		w := findPlayer(r, b.WinnerID)
		aName, bName, wName := b.PlayerAID, b.PlayerBID, ""
		if pA != nil {
			aName = pA.Name
		}
		if pB != nil {
			bName = pB.Name
		}
		if w != nil {
			wName = w.Name
		}
		views = append(views, PvPBoutView{
			BoutNumber:  b.BoutNumber,
			PlayerAID:   b.PlayerAID,
			PlayerAName: aName,
			PlayerBID:   b.PlayerBID,
			PlayerBName: bName,
			WinnerID:    b.WinnerID,
			WinnerName:  wName,
			Finished:    b.Finished,
		})
	}
	return views
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

func (m *Manager) allPvPBoutsFinished(r *room) bool {
	if len(r.Players) == 0 {
		return false
	}
	if len(r.PvPBouts) == 0 {
		return false
	}
	for _, b := range r.PvPBouts {
		if !b.Finished {
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
	// Rank players by PvP tournament wins & PvP score to award placement bonus
	maxScore := 0
	maxWins := 0
	for _, p := range r.Players {
		if p.PvPScore > maxScore {
			maxScore = p.PvPScore
		}
		if p.PvPWins > maxWins {
			maxWins = p.PvPWins
		}
	}
	for _, p := range r.Players {
		if (maxWins > 0 && p.PvPWins == maxWins) || (maxWins == 0 && p.PvPScore == maxScore) {
			p.PvPBonus = 50
			_ = m.recordScoreEvent(r, ScoreEvent{ID: "pvp-leader:" + p.ID, PlayerID: p.ID, Category: "pvp", Source: "pvp_leader_bonus", Points: 50})
		}
		goldScore := p.State.Gold / 10
		_ = m.recordScoreEvent(r, ScoreEvent{ID: "gold-conversion:" + p.ID, PlayerID: p.ID, Category: "economy", Source: "gold_conversion", Points: goldScore})
		p.State.Score = p.PveScore + p.PvPScore + p.PvPBonus + goldScore - p.PvPPenalty
	}
	results := make([]ResultEntry, 0, len(r.Players))
	for _, p := range r.Players {
		results = append(results, ResultEntry{
			PlayerID:       p.ID,
			Name:           p.Name,
			PvEScore:       p.PveScore,
			PvPScore:       p.PvPScore,
			GoldScore:      p.State.Gold / 10,
			Bonus:          p.PvPBonus,
			Penalties:      p.PvPPenalty,
			TotalScore:     p.State.Score,
			PvPDamage:      p.PvPDamage,
			PvPKnockouts:   p.PvPKnockouts,
			PvPDeaths:      p.PvPDeaths,
			BossesDefeated: p.BossesDefeated,
			Gold:           p.State.Gold,
		})
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
	if a.PvPKnockouts != b.PvPKnockouts {
		return a.PvPKnockouts > b.PvPKnockouts
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
	return a.TotalScore == b.TotalScore && a.PvPKnockouts == b.PvPKnockouts && a.PvPScore == b.PvPScore && a.PvPDamage == b.PvPDamage && a.BossesDefeated == b.BossesDefeated && a.Gold == b.Gold
}

type botDifficultyParam struct {
	healThreshold float64
	damageMult    float64
}

var botDifficultyParams = map[string]botDifficultyParam{
	"easy":      {healThreshold: 0.30, damageMult: 0.8},
	"normal":    {healThreshold: 0.45, damageMult: 1.0},
	"hard":      {healThreshold: 0.60, damageMult: 1.2},
	"nightmare": {healThreshold: 0.70, damageMult: 1.4},
}
