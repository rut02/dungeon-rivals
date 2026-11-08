package rooms

import (
	"errors"
	"fmt"

	"dungeon-rivals/backend/internal/dungeon"
	"dungeon-rivals/backend/internal/items"
)

type RoomOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	ItemID      string `json:"item_id,omitempty"`
	Value       int    `json:"value,omitempty"`
}

type ShopStock struct {
	ItemID    string `json:"item_id"`
	Name      string `json:"name"`
	Price     int    `json:"price"`
	Available bool   `json:"available"`
}

type RoomActivity struct {
	Type      string       `json:"type"`
	Options   []RoomOption `json:"options,omitempty"`
	Stock     []ShopStock  `json:"stock,omitempty"`
	Completed bool         `json:"completed,omitempty"`
}

var ErrInvalidChoice = errors.New("INVALID_ROOM_CHOICE")

func (m *Manager) beginActivity(p *player, node dungeon.Node) error {
	switch node.Type {
	case dungeon.RestRoom:
		stats := p.State.Stats(m.catalog)
		heal := (stats.MaxHP*35 + 99) / 100
		p.State.RestoreHP(p.State.HP+heal, m.catalog)
		mpHeal := max(4, (stats.MaxMP*20+99)/100)
		p.State.RestoreMP(mpHeal, m.catalog)
		return m.resolveRoom(p, false)
	case dungeon.TreasureRoom:
		kit := p.State.Kit
		if kit == "" || kit == "default" || kit == "warrior" {
			kit = "knight"
		}
		starterWeapon := map[string]string{"knight": "iron_sword", "mage": "elemental_grimoire", "rogue": "iron_dagger", "ranger": "hunting_bow"}[kit]
		if starterWeapon == "" {
			starterWeapon = "iron_sword"
		}
		p.Activity = &RoomActivity{
			Type: "TREASURE",
			Options: []RoomOption{
				{ID: "health_potion", Label: "Health Potion", Description: "รับยาฟื้นฟูพลังชีวิต 1 ขวด", Kind: "item", ItemID: "health_potion"},
				{ID: starterWeapon, Label: "อาวุธประจำคลาส", Description: "รับอาวุธที่ใช้ได้กับคลาสของคุณ", Kind: "item", ItemID: starterWeapon},
				{ID: "gold", Label: "Gold Cache", Description: "รับ 25 Gold", Kind: "gold", Value: 25},
			},
		}
	case dungeon.ShopRoom:
		activity := &RoomActivity{Type: "SHOP", Stock: []ShopStock{}}
		kit := p.State.Kit
		if kit == "" || kit == "default" || kit == "warrior" {
			kit = "knight"
		}
		for _, item := range m.catalog.List() {
			if item.Price > 0 {
				activity.Stock = append(activity.Stock, ShopStock{ItemID: item.ID, Name: item.Name, Price: item.Price, Available: true})
			}
		}
		p.Activity = activity
	case dungeon.EventRoom:
		p.Activity = &RoomActivity{
			Type: "EVENT",
			Options: []RoomOption{
				{ID: "heal", Label: "พักข้างกองไฟ (Campfire)", Description: "ฟื้นฟู HP 30 และฟื้นฟู MP 15", Kind: "heal", Value: 30},
				{ID: "pay", Label: "แลกเปลี่ยนกับนักเดินทาง (Merchant)", Description: "จ่าย 8 Gold เพื่อรับ Health Potion", Kind: "trade", ItemID: "health_potion", Value: 8},
				{ID: "risk", Label: "เดินลัดทางอันตราย (Shortcut)", Description: "เสีย 10 HP เพื่อรับ 25 Gold", Kind: "risk", Value: 10},
			},
		}
	case dungeon.PuzzleRoom:
		p.Activity = &RoomActivity{
			Type: "PUZZLE",
			Options: []RoomOption{
				{ID: "riddle_wisdom", Label: "ไขรหัสปริศนารูน (Wisdom)", Description: "ตอบปริศนาโบราณ รับ 50 EXP และ 20 Gold", Kind: "exp_gold", Value: 50},
				{ID: "chest_lock", Label: "สะเดาะกลอนหีบสมบัติกลไก (Agility)", Description: "รับรองเท้าลมกรด Boots of Haste (+4 SPD)", Kind: "item", ItemID: "boots_of_haste"},
				{ID: "altar_blood", Label: "แท่นบูชาสังเวยเลือด (Sacrifice)", Description: "สละ 20 HP เพื่อรับ Greater Health Potion x2", Kind: "sacrifice", ItemID: "greater_health_potion", Value: 20},
			},
		}
	case dungeon.ForgeRoom:
		p.Activity = &RoomActivity{
			Type: "FORGE",
			Options: []RoomOption{
				{ID: "forge_temper", Label: "หลอมจิตวิญญาณนักรบ (Soul Temper)", Description: "ฟรี! เพิ่มความเร็วถาวร +2 SPD, +10 MaxMP และฟื้นฟู MP เต็ม", Kind: "forge_spd", Value: 0},
				{ID: "forge_weapon", Label: "ตีบวกคมศัสตรา (Sharpen Blade)", Description: "จ่าย 18 Gold เพื่อเพิ่มพลังโจมตีถาวร +3 ATK", Kind: "forge_atk", Value: 18},
				{ID: "forge_armor", Label: "เสริมเกราะเหล็กกล้า (Reinforce Armor)", Description: "จ่าย 18 Gold เพื่อเพิ่มป้องกันถาวร +2 DEF และ +15 MaxHP", Kind: "forge_def", Value: 18},
			},
		}
	default:
		return fmt.Errorf("unsupported room activity %s", node.Type)
	}
	return nil
}

func weaponClass(item items.Definition) string {
	if item.Class != "" {
		return item.Class
	}
	return map[string]string{"sword": "knight", "staff": "mage", "daggers": "rogue", "bow": "ranger"}[item.Archetype]
}

func (m *Manager) resolveRoom(p *player, skipped bool) error {
	if p.Dungeon == nil {
		return ErrInvalidPhase
	}
	if err := p.Dungeon.ResolveActive(skipped); err != nil {
		return err
	}
	p.Activity = nil
	return nil
}

func (m *Manager) roomAction(r *room, playerID, action, choiceID, itemID string) error {
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
	if p.Activity == nil || p.Dungeon == nil {
		return ErrInvalidPhase
	}
	a := p.Activity
	switch a.Type {
	case "TREASURE", "EVENT", "PUZZLE", "FORGE":
		if action == "leave" && a.Type == "FORGE" {
			if err := m.resolveRoom(p, false); err != nil {
				return err
			}
			break
		}
		if action != "claim" || a.Completed {
			return ErrInvalidPhase
		}
		var option *RoomOption
		for i := range a.Options {
			if a.Options[i].ID == choiceID {
				option = &a.Options[i]
				break
			}
		}
		if option == nil {
			return ErrInvalidChoice
		}
		switch option.Kind {
		case "item":
			if err := p.State.Add(option.ItemID, 1, m.catalog); err != nil {
				return err
			}
			p.recordAcquiredItem(option.ItemID)
		case "gold":
			p.State.Gold += option.Value
		case "heal":
			p.State.RestoreHP(p.State.HP+option.Value, m.catalog)
			p.State.RestoreMP(15, m.catalog)
		case "exp_gold":
			p.State.GainEXP(option.Value, m.catalog)
			p.State.Gold += 20
		case "sacrifice":
			if p.State.HP <= option.Value {
				return errors.New("NOT_ENOUGH_HP")
			}
			p.State.Damage(option.Value)
			if err := p.State.Add(option.ItemID, 2, m.catalog); err == nil {
				p.recordAcquiredItem(option.ItemID)
			}
		case "trade":
			if p.State.Gold < option.Value {
				return errors.New("NOT_ENOUGH_GOLD")
			}
			if err := p.State.Add(option.ItemID, 1, m.catalog); err != nil {
				return err
			}
			p.State.Gold -= option.Value
			p.recordAcquiredItem(option.ItemID)
		case "risk":
			if p.State.HP <= option.Value {
				return errors.New("NOT_ENOUGH_HP")
			}
			p.State.Damage(option.Value)
			p.State.Gold += 25
		case "forge_atk":
			if p.State.Gold < option.Value {
				return errors.New("NOT_ENOUGH_GOLD")
			}
			p.State.Gold -= option.Value
			p.State.BaseATK += 3
		case "forge_def":
			if p.State.Gold < option.Value {
				return errors.New("NOT_ENOUGH_GOLD")
			}
			p.State.Gold -= option.Value
			p.State.BaseDEF += 2
			p.State.MaxHP += 15
			p.State.RestoreHP(p.State.HP+15, m.catalog)
		case "forge_spd":
			p.State.BaseSPD += 2
			p.State.MaxMP += 10
			p.State.RestoreMP(100, m.catalog)
		default:
			return ErrInvalidChoice
		}
		points := 5
		source := "treasure"
		if a.Type == "EVENT" {
			points = 10
			source = "event"
		} else if a.Type == "PUZZLE" {
			points = 15
			source = "puzzle"
		} else if a.Type == "FORGE" {
			points = 15
			source = "forge"
		}
		nodeID := p.Dungeon.ActiveNodeID
		if m.recordScoreEvent(r, ScoreEvent{ID: "pve:" + source + ":" + p.ID + ":" + nodeID, PlayerID: p.ID, Category: "pve", Source: source, Points: points}) {
			p.State.Score += points
		}
		a.Completed = true
		if err := m.resolveRoom(p, false); err != nil {
			return err
		}
	case "SHOP":
		if action == "leave" {
			if err := m.resolveRoom(p, false); err != nil {
				return err
			}
			break
		}
		if action == "sell" {
			if itemID == "" {
				return ErrInvalidChoice
			}
			def, ok := m.catalog.Get(itemID)
			if !ok {
				return ErrInvalidChoice
			}
			if err := p.State.Remove(itemID, 1); err != nil {
				return err
			}
			sellValue := max(5, def.Price/2)
			p.State.Gold += sellValue
			break
		}
		if action != "buy" {
			return ErrInvalidChoice
		}
		var stock *ShopStock
		for i := range a.Stock {
			if a.Stock[i].ItemID == itemID {
				stock = &a.Stock[i]
				break
			}
		}
		if stock == nil || !stock.Available {
			return ErrInvalidChoice
		}
		def, ok := m.catalog.Get(itemID)
		if !ok || def.Price != stock.Price {
			return ErrInvalidChoice
		}
		if p.State.Gold < stock.Price {
			return errors.New("NOT_ENOUGH_GOLD")
		}
		if err := p.State.Add(itemID, 1, m.catalog); err != nil {
			return err
		}
		p.State.Gold -= stock.Price
		stock.Available = false
		p.recordAcquiredItem(itemID)
	default:
		return ErrInvalidPhase
	}
	r.Version++
	m.broadcastSnapshotsLocked(r)
	return nil
}
