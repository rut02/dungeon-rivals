import { Component, ErrorInfo, FormEvent, ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import './inventory.css'
import './map.css'
import './combat-fx.css'
import { sound } from './sound'

type Session = { room_code: string; player_id: string; reconnect_token: string; is_host: boolean; ws_path: string }
type Player = { player_id: string; name: string; is_host: boolean; ready: boolean; connected: boolean; kit?: string; is_bot?: boolean; bot_difficulty?: string }

type ItemDefinition = {
  id: string
  name: string
  description: string
  type: 'weapon' | 'armor' | 'helmet' | 'accessory' | 'consumable'
  rarity: string
  slot: string
  stats: { attack: number; defense: number; max_hp: number; speed?: number; max_mp?: number }
  effect: { type: string; value: number } | null
  price: number
  stackable: boolean
  max_stack: number
  usable_in_combat: boolean
  archetype?: string
  skill_name?: string
  skill_cost?: number
  skill_desc?: string
  skill_target?: string
}

type DungeonNode = { id: string; floor: number; depth: number; type: string; status: string; path_lane?: string; connections: string[] }
type DungeonView = {
  current_floor: number
  current_node_id: string
  checkpoint_node_id: string
  active_node_id?: string
  progress: number
  completed: boolean
  floors: { number: number; difficulty_multiplier: number; nodes: DungeonNode[] }[]
}

type EnemyView = {
  index: number
  monster_id: string
  name: string
  tier: string
  hp: number
  max_hp: number
  attack: number
  defense: number
  speed: number
  intent: string
  intent_value: number
  alive: boolean
}

type CombatView = {
  monster_id: string
  name: string
  tier: string
  hp: number
  max_hp: number
  attack: number
  defense: number
  guarding: boolean
  finished: boolean
  victory?: boolean
  loot_item_id?: string
  log: string[]
  enemies?: EnemyView[]
  selected_enemy?: number
  turn?: number
  first_strike?: boolean
  skill_cooldown?: number
}

type ActivityView = {
  type: string
  options?: { id: string; label: string; description: string; kind: string; item_id?: string; value?: number }[]
  stock?: { item_id: string; name: string; price: number; available: boolean }[]
}

type PvPBoutView = {
  bout_number: number
  player_a_id: string
  player_a_name: string
  player_b_id: string
  player_b_name: string
  winner_id?: string
  winner_name?: string
  finished: boolean
}

type StatsState = {
  hp: number
  max_hp: number
  mp?: number
  max_mp?: number
  level?: number
  exp?: number
  max_exp?: number
  base_atk: number
  base_def: number
  base_spd?: number
  atk: number
  def: number
  spd?: number
  archetype?: string
  weapon_skill?: string
  weapon_skill_cost?: number
  weapon_skill_desc?: string
  weapon_skill_target?: string
}

type PrivateState = {
  stats: StatsState
  gold: number
  score: number
  deaths: number
  kit?: string
  inventory: { item_id: string; quantity: number }[]
  equipment: { weapon: string; armor: string; helmet: string; accessories: [string, string] }
  dungeon?: DungeonView
  combat?: CombatView
  activity?: ActivityView
  last_acquired_item_id?: string
  last_acquired_seq?: number
  pvp_actions_used?: number
  pvp_current_turn_player_id?: string
  pvp_your_turn?: boolean
  pvp_score?: number
  pvp_damage?: number
  pvp_knockouts?: number
  pvp_wins?: number
  pvp_losses?: number
  pvp_current_opponent_id?: string
  pvp_bout_number?: number
  pvp_total_bouts?: number
  pvp_log?: string[]
  pvp_bouts?: PvPBoutView[]
}

type PublicPlayer = Player & {
  stage: string
  floor: number
  progress: number
  score: number
  completed: boolean
  hp: number
  max_hp: number
  mp?: number
  max_mp?: number
  atk?: number
  def?: number
  spd?: number
  weapon_skill?: string
  pvp_score: number
  pvp_damage: number
  pvp_knockouts: number
  pvp_wins?: number
  pvp_losses?: number
}

type ResultEntry = {
  player_id: string
  name: string
  rank: number
  total_score: number
  pve_score: number
  pvp_score: number
  gold_score: number
  bonus: number
  penalties: number
  pvp_damage: number
  pvp_knockouts: number
  pvp_deaths: number
  bosses_defeated: number
  gold: number
}

type Snapshot = {
  version: number
  room: { phase: string; host_player_id: string; player_count: number; min_players: number; max_players: number }
  players: PublicPlayer[]
  me: Player & { private?: PrivateState }
  results?: ResultEntry[]
}

type WireMessage = { type: string; request_id?: string; payload?: any }

const SESSION_KEY = 'dungeon-rivals-session'

const ERROR_TRANSLATIONS: Record<string, string> = {
  INVALID_PHASE: 'ระบบยังไม่พร้อมสำหรับคำสั่งนี้ในสถานะปัจจุบัน',
  NO_ACTIVE_COMBAT: 'ไม่ได้อยู่ในการต่อสู้ หรือกรุณาจบการต่อสู้ก่อนจัดการอุปกรณ์',
  NO_ENCOUNTER_ROOM: 'ห้องนี้ไม่มีกิจกรรมหรือศัตรูให้เริ่มต้น',
  NOT_ENOUGH_MANA: 'มานา (MP) ไม่เพียงพอสำหรับการใช้สกิลนี้!',
  NOT_ENOUGH_HP: 'พลังชีวิต (HP) ไม่เพียงพอสำหรับทางเลือกนี้!',
  TARGET_NOT_FOUND: 'ไม่พบศัตรูเป้าหมาย กรุณาเลือกศัตรูที่ยังมีชีวิตอยู่',
  TARGET_ALREADY_DEAD: 'ศัตรูตัวนี้ล้มไปแล้ว กรุณาเลือกเป้าหมายอื่น',
  CONSUMABLE_ONLY: 'สามารถใช้งานได้เฉพาะไอเทมประเภทกดใช้เท่านั้น',
  ITEM_NOT_USABLE: 'พลังชีวิตหรือมานาเต็มอยู่แล้ว หรือไอเทมนี้กดใช้ไม่ได้',
  ROOM_NOT_FOUND: 'ไม่พบรหัสห้องนี้ในระบบ',
  PLAYER_NOT_FOUND: 'ไม่พบข้อมูลผู้เล่นในห้อง',
  INVALID_RECONNECT_TOKEN: 'โทเค็นการเชื่อมต่อไม่ถูกต้อง กรุณาเข้าห้องใหม่อีกครั้ง',
  INVENTORY_FULL: 'กระเป๋า (Backpack) เต็มแล้ว (สูงสุด 12 ช่อง) กรุณาใช้หรือจัดการของในกระเป๋าก่อน',
  NOT_ENOUGH_GOLD: 'Gold ไม่เพียงพอสำหรับซื้อไอเทมนี้',
  ITEM_NOT_FOUND: 'ไม่พบไอเทมนี้ในคลังเก็บของ',
  SLOT_MISMATCH: 'ไม่สามารถสวมใส่ไอเทมในช่องนี้ได้',
  INVALID_SLOT: 'ช่องอุปกรณ์ไม่ตรงกับประเภทไอเทม',
  NODE_NOT_FOUND: 'ไม่พบเส้นทางที่เลือกบนแผนที่',
  NODE_NOT_AVAILABLE: 'ห้องนี้ไม่ได้เชื่อมต่อกับตำแหน่งปัจจุบันของคุณ กรุณาเลือกห้องที่มีเส้นสีทองเชื่อมถึง',
  ENCOUNTER_IN_PROGRESS: 'คุณต้องจบกิจกรรมหรือการต่อสู้ในห้องปัจจุบันก่อนเดินทางต่อ',
  ROOM_FULL: 'ห้องนี้มีผู้เล่นเต็มแล้ว',
  NOT_ROOM_HOST: 'เฉพาะเจ้าของห้องเท่านั้นที่มีสิทธิ์ดำเนินการนี้',
  NOT_READY: 'ผู้เล่นทุกคนยังไม่พร้อม กรุณารอให้ทุกคนกดพร้อมก่อน',
  BOSS_ESCAPE_NOT_ALLOWED: 'ไม่สามารถหนีจากการต่อสู้กับบอสได้!',
}

const STARTER_KITS = [
  {
    id: 'knight',
    name: 'อัศวิน (Knight)',
    archetype: 'ดาบยาว (Sword)',
    icon: '🛡️',
    weaponId: 'iron_sword',
    weaponName: 'Iron Sword (ดาบเหล็กกล้า)',
    gearIds: [],
    gearNames: 'พกพาเฉพาะอาวุธเริ่มต้น + 1x Health Potion, 1x Mana Potion (เกราะและของอื่นต้องฟาร์มในดันเจียน)',
    description: 'สมดุล แข็งแกร่ง เลือดเยอะ ตั้งรับลดดาเมจ 70% พร้อมสะท้อนดาเมจ',
    perk: '🛡️ สกิล Shield Bash (5 MP): ผสานพลัง ATK+DEF กระแทกศัตรูรุนแรง (คูลดาวน์ 2 เทิร์น)',
    stats: 'HP 110 · MP 25 · ATK 11 · DEF 4 · SPD 6',
  },
  {
    id: 'mage',
    name: 'นักเวท (Mage)',
    archetype: 'คทาเวทมนตร์ (Staff)',
    icon: '🔮',
    weaponId: 'apprentice_staff',
    weaponName: 'Apprentice Staff (คทาฝึกหัดเวท)',
    gearIds: [],
    gearNames: 'พกพาเฉพาะอาวุธเริ่มต้น + 1x Health Potion, 1x Mana Potion (เกราะและของอื่นต้องฟาร์มในดันเจียน)',
    description: 'มานาสูง โจมตีทะลุเกราะ 50% และตั้งรับกางบาเรียฟื้นฟู +8 MP',
    perk: '🔮 สกิล Arcane Spark (5 MP): ระเบิดเวทหมู่โจมตีศัตรูทุกตัวในสนาม (คูลดาวน์ 2 เทิร์น)',
    stats: 'HP 90 · MP 35 · ATK 13 · DEF 2 · SPD 6',
  },
  {
    id: 'rogue',
    name: 'โร้ก (Rogue)',
    archetype: 'มีดสั้นคู่ (Dual Daggers)',
    icon: '🗡️',
    weaponId: 'iron_dagger',
    weaponName: 'Iron Dagger (มีดสั้นเหล็กกล้า)',
    gearIds: [],
    gearNames: 'พกพาเฉพาะอาวุธเริ่มต้น + 1x Health Potion, 1x Mana Potion (เกราะและของอื่นต้องฟาร์มในดันเจียน)',
    description: 'ว่องไวสูงสุด มีโอกาสคริติคอล 1.8 เท่า (30%) และตั้งรับหลบหลีก 50%',
    perk: '🗡️ สกิล Quick Stab (5 MP): แทงจุดตายรวดเร็ว 1.65 เท่าอย่างแม่นยำ (คูลดาวน์ 2 เทิร์น)',
    stats: 'HP 100 · MP 30 · ATK 11 · DEF 2 · SPD 9',
  },
  {
    id: 'ranger',
    name: 'เรนเจอร์ (Ranger)',
    archetype: 'ธนูยาว (Longbow)',
    icon: '🏹',
    weaponId: 'hunting_bow',
    weaponName: 'Hunting Bow (ธนูล่าสัตว์)',
    gearIds: [],
    gearNames: 'พกพาเฉพาะอาวุธเริ่มต้น + 1x Health Potion, 1x Mana Potion (เกราะและของอื่นต้องฟาร์มในดันเจียน)',
    description: 'ยิงเปิดฉากฟรีในเทิร์นแรกเสมอ (First Strike) ก่อนเริ่มแบทเทิล',
    perk: '🏹 สกิล Piercing Shot (5 MP): ศรเจาะเกราะทะลวงพลังป้องกันครึ่งหนึ่ง (คูลดาวน์ 2 เทิร์น)',
    stats: 'HP 100 · MP 30 · ATK 12 · DEF 2 · SPD 8',
  },
]

const BOSS_LORE: Record<string, { title: string; quote: string }> = {
  warden_of_ash: {
    title: 'ผู้คุมเพลิงผลาญแห่งสุสานเถ้า',
    quote: '« เถ้ากระดูกของเจ้า จะเป็นฟืนต่อลมหายใจแห่งเพลิงมืดนี้! จงมอดไหม้ไปเสีย! »',
  },
  frost_colossus: {
    title: 'จอมยักษ์ศิลาเยือกแข็งพันปี',
    quote: '« น้ำแข็งนิรันดร์จะผนึกทุกดวงวิญญาณ... แม้แต่ความหวังและเสียงกรีดร้องของเจ้าก็จะแข็งตัว »',
  },
  dragon_regent: {
    title: 'จอมราชันย์มังกรอเวจีทมิฬ',
    quote: '« หนอนแมลงต่ำช้า บังอาจเหยียบย่ำเข้ามาถึงบัลลังก์แห่งข้า... จงมลายสิ้นสู่อเวจีชั่วกัลปาวสาน! »',
  },
}

const ITEM_LORE: Record<string, string> = {
  training_sword: '« ดาบไม้ฝึกซ้อมของทหารเกณฑ์ผู้ฝันถึงเกียรติยศ »',
  iron_sword: '« ตีขึ้นจากเหล็กดำสุสาน คมดาบอาบเลือดศัตรูมานับร้อย »',
  knights_claymore: '« ดาบใหญ่ของอัศวินผู้พิทักษ์ที่สาบานว่าจะไม่ยอมคุกเข่าแก่ความมืด »',
  flamebrand: '« เปลวเพลิงที่ไม่มีวันมอด ดับสูญได้เพียงเมื่อผู้ถือครองสิ้นลมหายใจ »',
  excalibur: '« แสงสว่างสุดท้ายของอาณาจักรที่ล่มสลาย ส่องประกายตัดผ่านราตรี »',
  apprentice_staff: '« สลักจากกิ่งไม้ศักดิ์สิทธิ์ กักเก็บเศษเสี้ยวของมนตราโบราณ »',
  wooden_staff: '« สลักอักขระเวทวนเวียน เพิ่มพูนกระแสมานาของผู้ร่าย »',
  crystal_wand: '« ไม้กายสิทธิ์หัวอเมทิสต์ ปลดปล่อยประกายดาวทะลวงพลังป้องกัน »',
  archmage_staff: '« คทาแห่งมหาจอมเวท อัญมณีมรกตลอยเคว้งด้วยอำนาจจิต »',
  void_scepter: '« เมื่อจ้องมองเข้าไปในเนตรแห่งความว่างเปล่า ความว่างเปล่าจะจ้องกลับมา »',
  iron_dagger: '« มีดสั้นปลายแหลม คล่องแคล่วและเข้าถึงจุดตายในพริบตา »',
  shadowfang: '« คมมีดที่กรีดเฉือนไร้เสียง อาบยาพิษสกัดจากงูอเวจี »',
  assassins_twinblades: '« มีดคู่ประกบคู่ของนักฆ่าหลวง ดื่มด่ำโลหิตเพื่อความตายอันเงียบงัน »',
  grim_scythe: '« เคียวเก็บเกี่ยววิญญาณ ทุกการฟันคือการพรากลมหายใจสุดท้าย »',
  hunting_bow: '« ธนูไม้พิกุล น้าวสายไว เหมาะแก่การซุ่มโจมตีในเงามืด »',
  elven_recurve: '« คันธนูแกะสลักลวดลายพฤกษาแห่งเอลฟ์ สายธนูขับขานเสียงเพลงแห่งความตาย »',
  windrunner_bow: '« ธนูร้อยสายลม ลูกศรแหวกอากาศเร็วเกินกว่าสายตามนุษย์จะมองทัน »',
  dragons_breath_bow: '« เหลาขึ้นจากซี่โครงมังกร ยามน้าวสายจะเกิดเสียงคำรามดั่งพายุเพลิง »',
  wooden_shield: '« โล่ไม้เสริมขอบเหล็ก รองรับแรงปะทะหนักหน่วงได้พอตัว »',
  iron_shield: '« โล่เหล็กสลักกางเขนศักดิ์สิทธิ์ ต้านทานทั้งคมดาบและเวทมนตร์ »',
  aegis_shield: '« โล่สุริยันในตำนาน เปล่งรัศมีทองคำสะท้อนความมืดมิด »',
  cloth_armor: '« เสื้อคลุมนักเดินทาง ทอจากเส้นใยเหนียวแน่นเพื่อความคล่องตัว »',
  iron_armor: '« แผ่นเกราะเหล็กดำ สวมทับร่างกายเพื่อต้านทานการฟันแทง »',
  chainmail_hauberk: '« เสื้อเกราะโซ่ถักละเอียด ยืดหยุ่นแต่ยากที่คมดาบจะแทงทะลุ »',
  dragonscale_armor: '« เกล็ดมังกรทมิฬที่ไม่อาจหลอมละลายได้แม้ในธารลาวาเดือด »',
  leather_cap: '« หมวกหนังของโจรเงามืด ช่วยพรางใบหน้าในความมืด »',
  iron_helm: '« หมวกเหล็กปิดหน้า มีเพียงช่องแคบให้ดวงตามองผ่าน »',
  shadow_cowl: '« ผ้าคลุมศีรษะภูตพราย กลืนกินตัวตนของผู้สวมใส่ให้กลายเป็นหมอก »',
  crown_of_kings: '« มงกุฎแห่งกษัตริย์องค์สุดท้าย หนักอึ้งด้วยบาปและความรับผิดชอบ »',
  bronze_ring: '« แหวนสำริดโบราณ สลักอักษรรูนเสริมพละกำลัง »',
  apprentice_ring: '« แหวนเงินฝังไพลิน สั่นสะเทือนตามกระแสมานาในบรรยากาศ »',
  quiver_charm: '« เครื่องรางขนนกอินทรี เพิ่มความแม่นยำและความเร็วในการชักลูกศร »',
  swift_boots: '« รองเท้าติดปีกเงิน ก้าวย่างแผ่วเบาดั่งไร้น้ำหนัก »',
  boots_of_haste: '« รองเท้าต้องมนต์วายุเร่งความเร็ว ชิงลงมือก่อนศัตรูเสมอ »',
  blood_chalice: '« จอกทองคำโบราณ ดื่มด่ำโลหิตเพื่อแลกกับพลังชีวิตอันมหาศาล »',
  thiefs_coin: '« เหรียญทองต้องสาปของโจรสลัดวิญญาณ ผู้ใดถือครองจะมีโชคและหายนะตามติด »',
  health_potion: '« น้ำยาฟื้นฟูสีแดงฉาน กลิ่นคล้ายสมุนไพรและโลหิตเข้มข้น »',
  greater_health_potion: '« ยาฟื้นฟูชั้นสูงในขวดคริสตัล ประกายสีทับทิมคืนชีพให้ร่างใกล้แตกสลาย »',
  mana_potion: '« น้ำยาสีฟ้าเรืองแสง สกัดจากผลึกมนตราใต้พิภพ »',
  greater_mana_potion: '« ละอองดาราสีม่วงเข้ม ฟื้นคืนกระแสจิตและเวทมนตร์จนเต็มเปี่ยม »',
  fire_bomb: '« ระเบิดเหล็กดำ บรรจุดินปืนภูเขาไฟพร้อมเผาผลาญศัตรูทุกตัว »',
  smoke_bomb: '« ระเบิดควันทมิฬ ปล่อยหมอกม่วงพรางตัวหลบหนีจากสมรภูมิ »',
}

// Fallback map in case an older session references a legacy ID
const ITEM_SPRITE_FALLBACK: Record<string, string> = {
  twin_daggers: 'iron_dagger',
  void_stiletto: 'assassins_twinblades',
  pyroclasm_rod: 'wooden_staff',
  archmage_scepter: 'archmage_staff',
  dragonbone_greatbow: 'dragons_breath_bow',
  mystic_robe: 'cloth_armor',
  shadow_vest: 'cloth_armor',
  scout_tunic: 'cloth_armor',
  sorcerer_vestments: 'chainmail_hauberk',
  dragonscale_plate: 'dragonscale_armor',
  iron_greathelm: 'iron_helm',
  wizard_hat: 'shadow_cowl',
  amulet_of_iron: 'iron_shield',
  ring_of_vigor: 'blood_chalice',
}

const MONSTER_SPRITE_FALLBACK: Record<string, string> = {}

function getMonsterSpritePath(monsterId: string): string {
  const mapped = MONSTER_SPRITE_FALLBACK[monsterId] || monsterId
  return `/art/pixel/${mapped}.png`
}

function getItemSpritePath(itemId: string): string {
  const mapped = ITEM_SPRITE_FALLBACK[itemId] || itemId
  return `/art/pixel/${mapped}.png`
}

function ItemSprite({
  itemId,
  alt,
  className,
  title,
  onClick,
}: {
  itemId: string
  alt?: string
  className?: string
  title?: string
  onClick?: () => void
}) {
  return (
    <img
      src={getItemSpritePath(itemId)}
      alt={alt || itemId}
      title={title || alt || itemId}
      className={`${className || 'item-pixel-art'} ${onClick ? 'item-inspect-trigger' : ''}`}
      onClick={onClick}
      onError={(e) => {
        const target = e.currentTarget
        if (!target.src.endsWith('/art/pixel/training_sword.png')) {
          target.src = '/art/pixel/training_sword.png'
        }
      }}
    />
  )
}

const FLOOR_LORE: Record<number, { title: string; desc: string }> = {
  1: {
    title: 'สุสานเถ้าธุลี (The Ashen Crypt)',
    desc: 'ซากกระดูกที่ถูกเผาผลาญและกลิ่นกำมะถัน เฝ้าโดย Warden of Ash ผู้คุมเพลิงวิบัติ',
  },
  2: {
    title: 'วิหารน้ำแข็งนิรันดร์ (The Glacial Vault)',
    desc: 'ธารน้ำแข็งโปร่งแสงที่กักขังดวงวิญญาณคนบาป เฝ้าโดย Frost Colossus ยักษ์ศิลาเยือกแข็ง',
  },
  3: {
    title: 'บัลลังก์มังกรอเวจี (The Dragon\'s Maw)',
    desc: 'ธารลาวาเดือดและหินออบซิเดียน ที่สถิตของ Dragon Regent จอมราชันย์มังกรอมตะ',
  },
}

function readSession(): Session | null {
  try { return JSON.parse(sessionStorage.getItem(SESSION_KEY) || 'null') } catch { return null }
}

function AppContent() {
  const [session, setSession] = useState<Session | null>(readSession)
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [itemCatalog, setItemCatalog] = useState<ItemDefinition[]>([])
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')
  const [name, setName] = useState('')
  const [joinCode, setJoinCode] = useState('')
  const [pending, setPending] = useState(false)
  const [showLore, setShowLore] = useState(false)
  const [showBag, setShowBag] = useState(false)
  const [botDifficulty, setBotDifficulty] = useState<'easy' | 'normal' | 'hard' | 'nightmare'>('normal')
  const [botKit, setBotKit] = useState<string>('random')
  const [sparkleItem, setSparkleItem] = useState<{ itemId: string; seq: number } | null>(null)
  const [seenAcquireSeq, setSeenAcquireSeq] = useState<number>(0)
  const [inspectedItemId, setInspectedItemId] = useState<string | null>(null)
  const inspectedItem = useMemo(() => itemCatalog.find((it) => it.id === inspectedItemId) || null, [itemCatalog, inspectedItemId])

  useEffect(() => {
    if (!session) return
    let disposed = false
    let socket: WebSocket | undefined
    let retry: number | undefined
    let attempts = 0
    const connect = () => {
      if (disposed) return
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      socket = new WebSocket(`${protocol}//${window.location.host}${session.ws_path}`)
      socket.onopen = () => {
        attempts = 0
        socket?.send(JSON.stringify({ type: 'session.authenticate', request_id: crypto.randomUUID(), payload: { room_code: session.room_code, player_id: session.player_id, reconnect_token: session.reconnect_token } }))
      }
      socket.onmessage = (event) => {
        const message = JSON.parse(event.data) as WireMessage
        if (message.type === 'session.authenticated') setConnected(true)
        if (message.type === 'room.snapshot') setSnapshot((previous) => !previous || message.payload.version >= previous.version ? message.payload : previous)
        if (message.type === 'command.error') {
          const code = message.payload?.code || ''
          const friendly = ERROR_TRANSLATIONS[code] || message.payload?.message || code || 'คำสั่งไม่สำเร็จ'
          setError(friendly)
          if (code === 'ROOM_NOT_FOUND' || code === 'PLAYER_NOT_FOUND' || code === 'INVALID_RECONNECT_TOKEN') {
            sessionStorage.removeItem(SESSION_KEY)
            setSession(null)
            setSnapshot(null)
          }
        }
      }
      socket.onclose = () => {
        setConnected(false)
        if (!disposed) retry = window.setTimeout(connect, Math.min(1000 * 2 ** attempts++, 10000))
      }
      socket.onerror = () => socket?.close()
    }
    connect()
    const send = (type: string, payload: object = {}) => {
      if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type, request_id: crypto.randomUUID(), payload }))
      else setError('กำลังเชื่อมต่อกับห้อง ลองอีกครั้งในอีกสักครู่')
    }
    const commandListener = ((event: CustomEvent) => send(event.detail.type, event.detail.payload)) as EventListener
    window.addEventListener('dungeon-command', commandListener)
    return () => { disposed = true; window.clearTimeout(retry); window.removeEventListener('dungeon-command', commandListener); socket?.close() }
  }, [session])

  useEffect(() => {
    if (!session) { setItemCatalog([]); return }
    let cancelled = false
    fetch('/api/v1/items').then(async (response) => {
      if (!response.ok) throw new Error('โหลดข้อมูลไอเทมไม่สำเร็จ')
      return response.json() as Promise<ItemDefinition[]>
    }).then((items) => { if (!cancelled) setItemCatalog(items) }).catch((e) => { if (!cancelled) setError((e as Error).message) })
    return () => { cancelled = true }
  }, [session])

  const inActiveCombat = Boolean(snapshot?.me?.private?.combat && !snapshot.me.private.combat.finished)
  const currentPhase = snapshot?.room?.phase || 'lobby'
  const canOpenBag = Boolean(snapshot?.me?.private && currentPhase !== 'lobby' && !inActiveCombat && currentPhase !== 'pvp')

  useEffect(() => {
    if (!canOpenBag && showBag) {
      setShowBag(false)
    }
  }, [canOpenBag, showBag])

  useEffect(() => {
    const seq = snapshot?.me?.private?.last_acquired_seq || 0
    const itemId = snapshot?.me?.private?.last_acquired_item_id || ''
    if (seq > seenAcquireSeq && itemId) {
      setSeenAcquireSeq(seq)
      // When winning combat, center victory modal already showcases the loot; suppress bottom popup
      const isCombatVictory = Boolean(snapshot?.me?.private?.combat?.finished && snapshot?.me?.private?.combat?.victory)
      if (!isCombatVictory) {
        setSparkleItem({ itemId, seq })
        sound.playLevelUp()
        const timer = window.setTimeout(() => {
          setSparkleItem((curr) => (curr?.seq === seq ? null : curr))
        }, 4200)
        return () => window.clearTimeout(timer)
      }
    }
  }, [snapshot?.me?.private?.last_acquired_seq, snapshot?.me?.private?.last_acquired_item_id, seenAcquireSeq, snapshot?.me?.private?.combat?.finished, snapshot?.me?.private?.combat?.victory])

  // Keyboard shortcut 'b' or 'i' to toggle bag when allowed
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement || e.target instanceof HTMLSelectElement) return
      if ((e.key === 'b' || e.key === 'B' || e.key === 'i' || e.key === 'I') && canOpenBag) {
        setShowBag((prev) => !prev)
      } else if (e.key === 'Escape') {
        setShowBag(false)
        setShowLore(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [canOpenBag])

  const host = useMemo(() => snapshot?.players.find((player) => player.player_id === snapshot.room.host_player_id), [snapshot])

  async function createRoom(event: FormEvent) {
    event.preventDefault()
    setPending(true); setError('')
    try {
      const response = await fetch('/api/v1/rooms', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ player_name: name }) })
      const body = await response.json()
      if (!response.ok) throw new Error(body.message || body.code || 'สร้างห้องไม่สำเร็จ')
      saveSession(body)
    } catch (e) { setError((e as Error).message) } finally { setPending(false) }
  }

  async function joinRoom(event: FormEvent) {
    event.preventDefault()
    setPending(true); setError('')
    try {
      const response = await fetch(`/api/v1/rooms/${encodeURIComponent(joinCode.trim().toUpperCase())}/join`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ player_name: name }) })
      const body = await response.json()
      if (!response.ok) throw new Error(body.message || body.code || 'เข้าห้องไม่สำเร็จ')
      saveSession(body)
    } catch (e) { setError((e as Error).message) } finally { setPending(false) }
  }

  function saveSession(next: Session) { sessionStorage.setItem(SESSION_KEY, JSON.stringify(next)); setSession(next) }
  function command(type: string, payload: object = {}) { window.dispatchEvent(new CustomEvent('dungeon-command', { detail: { type, payload } })) }
  function forgetSession() { sessionStorage.removeItem(SESSION_KEY); setSession(null); setSnapshot(null); setConnected(false) }
  function leave() {
    command('room.leave')
    window.setTimeout(forgetSession, 120)
  }
  function confirmLeave() {
    const inGame = snapshot && snapshot.room.phase !== 'lobby' && snapshot.room.phase !== 'results'
    if (inGame) {
      if (!window.confirm('คุณกำลังอยู่ระหว่างการเล่น ต้องการออกจากห้องและกลับสู่หน้าหลักหรือไม่?')) {
        return
      }
    }
    leave()
  }

  if (session && !snapshot) {
    return <main className="shell lobby-shell">
      <header className="topbar">
        <div className="brand"><RoomIcon type="START" className="brand-sigil"/><span>DUNGEON <b>RIVALS</b></span></div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <button className="lore-open-btn" onClick={() => setShowLore(true)}>📜 ตำนานสุสาน (Lore)</button>
          <button className="leave-topbar-btn" onClick={confirmLeave} title="ออกจากห้องและกลับสู่หน้าหลัก">🚪 ออกจากห้อง</button>
          <div className="status"><span className={connected ? 'pulse online' : 'pulse'} />{connected ? 'กำลังโหลดห้อง' : 'กำลังเชื่อมต่อ'}</div>
        </div>
      </header>
      <section className="panel roster-panel loading-panel">
        <span className="eyebrow">กำลังเข้าสู่ปาร์ตี้</span>
        <h2>{session.room_code}</h2>
        <p className="muted">กำลังเชื่อมต่อกับห้องของคุณ…</p>
        {error && <div className="form-error">{error}</div>}
        <button className="button button-quiet" onClick={forgetSession}>กลับหน้าหลัก</button>
      </section>
      {showLore && <LoreModal onClose={() => setShowLore(false)} />}
    </main>
  }

  if (session && snapshot) {
    const me = snapshot.me
    const phase = snapshot.room.phase
    const allReady = snapshot.players.length >= snapshot.room.min_players && snapshot.players.every((player) => player.ready)
    const currentKit = me.private?.kit || me.kit || 'knight'
    const privateState = me.private

    const hasActiveCombat = Boolean(privateState?.combat)
    const hasActiveRoom = Boolean(!hasActiveCombat && privateState?.activity)

    return <main className="shell game-shell">
      <header className="topbar">
        <div className="brand">
          <RoomIcon type="START" className="brand-sigil"/>
          <span>DUNGEON <b>RIVALS</b></span>
          <span className="room-pill-mini">ห้อง: <b>{session.room_code}</b></span>
        </div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
          {privateState && phase !== 'lobby' && (
            <button
              type="button"
              className={`bag-toggle-btn ${showBag ? 'is-open' : ''} ${!canOpenBag ? 'is-locked' : ''}`}
              disabled={!canOpenBag}
              onClick={() => canOpenBag && setShowBag(true)}
              title={canOpenBag ? 'เปิดกระเป๋าและจัดการอุปกรณ์สวมใส่ (คีย์ลัด B)' : 'ไม่สามารถเปิดกระเป๋าระหว่างการต่อสู้ได้ (กดใช้ยา/ระเบิดได้จากแถบคำสั่งด้านล่าง)'}
            >
              {canOpenBag ? '🎒 กระเป๋า & อุปกรณ์' : '🔒 กระเป๋าล็อกขณะต่อสู้'}
              <span className="bag-count-badge">{privateState.inventory.length}/12</span>
            </button>
          )}
          <button className="lore-open-btn" onClick={() => setShowLore(true)}>📜 ตำนานสุสาน</button>
          <button className="leave-topbar-btn" onClick={confirmLeave} title="ออกจากห้องและกลับสู่หน้าหลัก">🚪 ออกจากห้อง</button>
          <div className="status"><span className={connected ? 'pulse online' : 'pulse'} />{connected ? 'ออนไลน์' : 'กำลังเชื่อมต่อ'}</div>
        </div>
      </header>

      {/* IN-GAME HUD BAR + COMPACT PARTY BAR (Shown whenever game is active) */}
      {phase !== 'lobby' && privateState && (
        <HeroHudBar
          state={privateState}
          players={snapshot.players}
          myId={session.player_id}
          canOpenBag={canOpenBag}
          onOpenBag={() => canOpenBag && setShowBag(true)}
        />
      )}

      {/* PHASE 1: LOBBY VIEW */}
      {phase === 'lobby' && (
        <>
          <section className="lobby-head">
            <div>
              <p className="eyebrow">ห้องรอผู้เล่น · ASHEN REPUTATION</p>
              <h1>เตรียมทีมให้พร้อม</h1>
              <p className="muted">เลือกสายอาชีพเริ่มต้นและวางกลยุทธ์ แต่ละคนจะสำรวจเส้นทางดันเจียน 3 ชั้นของตนเองก่อนตัดสินใน Arena</p>
            </div>
            <div className="room-code"><span>ROOM CODE</span><strong>{session.room_code}</strong></div>
          </section>

          <section className="panel roster-panel">
            <div className="panel-title"><div><span className="eyebrow">ADVENTURERS</span><h2>สมาชิกในห้อง</h2></div><span className="count">{snapshot.players.length}<i> / {snapshot.room.max_players}</i></span></div>
            <div className="roster">
              {[...snapshot.players, ...Array.from({ length: Math.max(0, snapshot.room.max_players - snapshot.players.length) }, () => null)].map((player, i) => player ? (
                <article className="player-card" key={player.player_id}>
                  <div className={`avatar avatar-${i % 4}`}><img src="/art/pixel/hero.png" alt="" className="roster-hero-art"/></div>
                  <div className="player-info">
                    <strong>
                      {player.name}
                      {player.player_id === session.player_id && <small>คุณ</small>}
                      {player.is_bot && (
                        <span className={`bot-pill bot-pill-${player.bot_difficulty || 'normal'}`}>
                          🤖 BOT · {player.bot_difficulty === 'easy' ? 'ง่าย' : player.bot_difficulty === 'hard' ? 'ยาก' : player.bot_difficulty === 'nightmare' ? 'มหาโหด' : 'ปานกลาง'}
                        </span>
                      )}
                    </strong>
                    <span>{player.is_host ? 'เจ้าของห้อง' : player.is_bot ? 'AI บอท' : 'นักผจญภัย'} · {player.connected ? 'ออนไลน์' : 'ออฟไลน์'}</span>
                  </div>
                  {session.is_host && player.is_bot && (
                    <button
                      type="button"
                      className="remove-bot-btn"
                      onClick={() => command('lobby.remove_bot', { bot_player_id: player.player_id })}
                      title="ลบบอทตัวนี้"
                    >
                      ✕ ลบ
                    </button>
                  )}
                  <div className={`ready-tag ${player.ready ? 'is-ready' : ''}`}>{player.ready ? 'พร้อม' : 'รออยู่'}</div>
                </article>
              ) : (
                <article className="player-card empty-card" key={`empty-${i}`}><div className="avatar empty-avatar">＋</div><div className="player-info"><strong>รอผู้เล่นหรือบอท</strong><span>แชร์รหัสห้องหรือกด 'เพิ่มบอท' ด้านล่างเพื่อเล่นคนเดียว</span></div><span className="empty-dot" /></article>
              ))}
            </div>

            {session.is_host && (
              <div className="bot-manager-panel">
                <div className="bot-manager-title">
                  <div>
                    <span className="eyebrow">SOLO TESTING & AI OPPONENTS</span>
                    <h3>🤖 บอทสำหรับทดสอบเดี่ยว (Solo / Bots)</h3>
                  </div>
                  <span className="bot-desc-hint">เลือกระดับความยากของบอทเพื่อเริ่มเล่นคนเดียวได้ทันที</span>
                </div>
                <div className="bot-controls">
                  <div className="bot-field">
                    <label>ระดับความยากของบอท (Difficulty)</label>
                    <select
                      value={botDifficulty}
                      onChange={(e) => setBotDifficulty(e.target.value as any)}
                      className="bot-select"
                    >
                      <option value="easy">🌱 ง่าย (Easy - ผู้ฝึกหัด / HP 75, ตีเบา)</option>
                      <option value="normal">⚔️ ปานกลาง (Normal - ทหารกล้า / สแตทมาตรฐาน)</option>
                      <option value="hard">🔥 ยาก (Hard - แชมเปี้ยน / HP+35, ATK+3)</option>
                      <option value="nightmare">☠️ มหาโหด (Nightmare - จอมมาร / ดุดันมาก)</option>
                    </select>
                  </div>
                  <div className="bot-field">
                    <label>สายอุปกรณ์ของบอท (Starter Kit)</label>
                    <select
                      value={botKit}
                      onChange={(e) => setBotKit(e.target.value)}
                      className="bot-select"
                    >
                      <option value="random">🎲 สุ่มสายอาชีพ (Random)</option>
                      <option value="knight">🛡️ อัศวิน (Knight)</option>
                      <option value="mage">🔮 นักเวท (Mage)</option>
                      <option value="rogue">🗡️ โร้ก (Rogue)</option>
                      <option value="ranger">🏹 เรนเจอร์ (Ranger)</option>
                    </select>
                  </div>
                  <div className="bot-btn-group">
                    <button
                      type="button"
                      className="button button-outline"
                      disabled={snapshot.players.length >= snapshot.room.max_players}
                      onClick={() => command('lobby.add_bot', { difficulty: botDifficulty, kit: botKit })}
                    >
                      ＋ เพิ่มบอท
                    </button>
                    {snapshot.players.length === 1 && (
                      <button
                        type="button"
                        className="button button-gold solo-start-btn"
                        onClick={() => {
                          command('lobby.add_bot', { difficulty: botDifficulty, kit: botKit })
                          command('lobby.ready', { ready: true })
                          window.setTimeout(() => command('game.start'), 300)
                        }}
                      >
                        ⚡ เริ่มทดสอบเดี่ยวทันที
                      </button>
                    )}
                  </div>
                </div>
              </div>
            )}

            <div className="kit-selector-wrap">
              <div className="kit-selector-title">
                <h3>เลือกอาวุธเริ่มต้น (Starting Weapon Kit)</h3>
                <span>เลือกเฉพาะอาวุธประจำสายของคุณ (เริ่มต้นมี 1x Health Potion, 1x Mana Potion · เกราะและเครื่องประดับทั้งหมดต้องค้นหาในดันเจียน)</span>
              </div>
              <div className="kit-grid">
                {STARTER_KITS.map((kit) => (
                  <div
                    key={kit.id}
                    className={`kit-card ${currentKit === kit.id ? 'is-selected' : ''}`}
                    onClick={() => {
                      sound.playSlash()
                      command('lobby.select_kit', { kit: kit.id })
                    }}
                  >
                    <div className="kit-top-row">
                      <div
                        className="kit-weapon-box"
                        title={`คลิกหรือชี้เพื่อดูรายละเอียด ${kit.weaponName}`}
                        onClick={(e) => {
                          e.stopPropagation()
                          const found = itemCatalog.find((it) => it.id === kit.weaponId)
                          if (found) setInspectedItemId(found.id)
                        }}
                      >
                        <ItemSprite itemId={kit.weaponId} alt={kit.weaponName} className="kit-weapon-sprite" />
                      </div>
                      <div className="kit-title-group">
                        <strong className="kit-name">{kit.icon} {kit.name}</strong>
                        <span className="kit-archetype">{kit.archetype}</span>
                        <small style={{ color: '#ffd166', fontWeight: 600 }}>⚔️ {kit.weaponName}</small>
                      </div>
                    </div>

                    <div className="kit-gear-preview-row">
                      <span className="kit-gear-label">เสบียงพกพา:</span>
                      <div
                        title="คลิกดูรายละเอียด Health Potion"
                        onClick={(e) => {
                          e.stopPropagation()
                          setInspectedItemId('health_potion')
                        }}
                      >
                        <ItemSprite itemId="health_potion" className="kit-gear-item-mini item-inspect-trigger" />
                      </div>
                      <div
                        title="คลิกดูรายละเอียด Mana Potion"
                        onClick={(e) => {
                          e.stopPropagation()
                          setInspectedItemId('mana_potion')
                        }}
                      >
                        <ItemSprite itemId="mana_potion" className="kit-gear-item-mini item-inspect-trigger" />
                      </div>
                      <small style={{ color: '#aaa', fontSize: '11px', marginLeft: '6px' }}>
                        1x HP + 1x MP Potion (เกราะหาในดันเจียน)
                      </small>
                    </div>

                    <div className="kit-skill-highlight">
                      <span>{kit.perk}</span>
                    </div>
                    <span className="kit-stats-tag">สเตตัสเริ่มต้น: {kit.stats}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="lobby-actions">
              <button className={`button ${me?.ready ? 'button-muted' : 'button-primary'}`} onClick={() => command('lobby.ready', { ready: !me?.ready })}>
                {me?.ready ? 'ยกเลิกความพร้อม' : '✓ พร้อมลุยดันเจียน'}
              </button>
              {session.is_host && (
                <button className="button button-gold" disabled={!allReady} onClick={() => command('game.start')}>
                  เริ่มเกม <span>→</span>
                </button>
              )}
              <button className="button button-quiet" onClick={confirmLeave}>ออกจากห้อง</button>
            </div>
            <p className="hint">{host?.name || 'เจ้าของห้อง'} จะเริ่มเกมได้เมื่อมีผู้เล่นหรือบอทอย่างน้อย 2 คนและทุกคนพร้อม</p>
          </section>
        </>
      )}

      {/* PHASE 2: DUNGEON EXPLORATION — SINGLE FOCUSED STAGE VIEWPORT */}
      {phase === 'dungeon' && privateState && (
        <>
          {hasActiveCombat && (
            <CombatPanel
              state={privateState}
              catalog={itemCatalog}
              onCommand={command}
              onOpenBag={() => setShowBag(true)}
            />
          )}

          {hasActiveRoom && (
            <RoomActivityPanel
              state={privateState}
              catalog={itemCatalog}
              onCommand={command}
              onOpenBag={() => setShowBag(true)}
            />
          )}

          {!hasActiveCombat && !hasActiveRoom && privateState.dungeon && !privateState.dungeon.completed && (
            <DungeonMap
              state={privateState.dungeon}
              onMove={(nodeID) => command('dungeon.move', { node_id: nodeID })}
              onOpenBag={() => setShowBag(true)}
              bagCount={privateState.inventory.length}
            />
          )}
        </>
      )}

      {/* PHASE 3: PREPARATION BEFORE PVP */}
      {((phase === 'dungeon' && privateState?.dungeon?.completed && !hasActiveCombat) || phase === 'preparation') && (
        <PreparationPanel
          players={snapshot.players}
          isReady={me.ready}
          phase={phase}
          isHost={me.is_host}
          onReady={(ready) => command('preparation.ready', { ready })}
          onKick={(playerID) => command('room.kick_disconnected', { target_player_id: playerID })}
          onOpenBag={() => setShowBag(true)}
        />
      )}

      {/* PHASE 4: PVP ARENA */}
      {phase === 'pvp' && privateState && (
        <ArenaPanel
          players={snapshot.players}
          me={me}
          isHost={me.is_host}
          state={privateState}
          catalog={itemCatalog}
          onCommand={command}
          onOpenBag={() => setShowBag(true)}
        />
      )}

      {/* PHASE 5: FINAL RESULTS */}
      {phase === 'results' && snapshot.results && (
        <ResultsPanel
          roomCode={session.room_code}
          results={snapshot.results}
          onReplay={() => command('room.replay')}
          onLeave={leave}
        />
      )}

      {/* SPARKLE ITEM PICKUP ANIMATION POPUP ("วิ้ง! ได้รับไอเทมใหม่") */}
      {sparkleItem && !snapshot?.me?.private?.combat?.finished && (
        <SparkleItemPopup
          itemId={sparkleItem.itemId}
          catalog={itemCatalog}
          onClose={() => setSparkleItem(null)}
        />
      )}

      {/* ITEM INSPECTOR MODAL */}
      {inspectedItem && (
        <ItemInspectorModal
          item={inspectedItem}
          onClose={() => setInspectedItemId(null)}
        />
      )}

      {/* BAG / INVENTORY MODAL DRAWER */}
      {showBag && canOpenBag && privateState && (
        <div className="bag-modal-overlay" onClick={() => setShowBag(false)}>
          <div className="bag-modal-window" onClick={(e) => e.stopPropagation()}>
            <InventoryPanel
              state={privateState}
              catalog={itemCatalog}
              onCommand={command}
              onClose={() => setShowBag(false)}
              inCombat={Boolean(privateState.combat && !privateState.combat.finished)}
              onInspectItem={(id) => setInspectedItemId(id)}
            />
          </div>
        </div>
      )}

      {error && (
        <div className="error-toast">
          <span>⚠️ {error}</span>
          <button onClick={() => setError('')}>×</button>
        </div>
      )}
      {showLore && <LoreModal onClose={() => setShowLore(false)} />}
      <footer>สร้างปาร์ตี้ · สำรวจดันเจียน · ชิงชัยในอารีนา</footer>
    </main>
  }

  return <main className="shell landing-shell">
    <div className="topbar">
      <div className="brand"><RoomIcon type="START" className="brand-sigil"/><span>DUNGEON <b>RIVALS</b></span></div>
      <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
        <button className="lore-open-btn" onClick={() => setShowLore(true)}>📜 ตำนานสุสาน (Lore)</button>
        <span className="version">MULTIPLAYER DUNGEON ROGUELITE</span>
      </div>
    </div>
    <div className="landing-grid">
      <section className="hero">
        <div className="rune rune-one">✧</div><div className="rune rune-two">✦</div>
        <p className="eyebrow">CHRONICLES OF THE ASHEN CRYPT</p>
        <h1>สุสานเถ้าธุลีโบราณ<br/><em>เส้นทางสู่อเวจี</em></h1>
        <p className="hero-copy">รวมทีมกับเพื่อนหรือดวลกับ AI บอท ก้าวลงสู่สุสาน 3 ชั้น ล่ามอนสเตอร์และไขปริศนาโบราณ<br/>สะสมอาวุธระดับตำนาน ก่อนประลองฝีมือชิงศิลาราชันย์มังกรในสนามประลอง</p>
        <button className="button button-quiet" style={{ margin: '8px 0 16px', display: 'inline-flex', alignItems: 'center', gap: '6px' }} onClick={() => setShowLore(true)}>
          📜 อ่านพงศาวดารแห่งสุสานเถ้าธุลี (Story Lore) →
        </button>
        <figure className="hero-art"><img src="/art/dungeon-cover.png" alt="สุสานเถ้าธุลีโบราณ จอมราชันย์มังกรและเหล่านักรบ"/><figcaption>CHRONICLES OF ASHEN CRYPT · สุสานเถ้าธุลีโบราณและจอมราชันย์มังกร</figcaption></figure>
        <div className="hero-stats"><span><b>02–04</b><small>ผู้เล่นหรือบอท</small></span><i /><span><b>3 ชั้น</b><small>แผนที่เลือกเส้นทาง</small></span><i /><span><b>PVP</b><small>ชิงชัยช่วงท้าย</small></span></div>
      </section>
      <section className="panel entry-panel">
        <div className="panel-title"><div><span className="eyebrow">เริ่มการผจญภัย</span><h2>เข้าสู่ดันเจียน</h2></div><span className="crystal">◈</span></div>
        <form onSubmit={createRoom}>
          <label htmlFor="player-name">ชื่อนักผจญภัย</label>
          <input id="player-name" maxLength={24} placeholder="ใส่ชื่อของคุณ" value={name} onChange={(e) => setName(e.target.value)} required/>
          <button className="button button-primary full-width" disabled={pending}>{pending ? 'กำลังสร้างห้อง…' : 'สร้างห้องใหม่'} <span>→</span></button>
        </form>
        <div className="divider"><span>หรือเข้าร่วมปาร์ตี้</span></div>
        <form className="join-form" onSubmit={joinRoom}>
          <label htmlFor="room-code">รหัสห้อง</label>
          <div className="join-row">
            <input id="room-code" maxLength={6} placeholder="เช่น A7K9Q2" value={joinCode} onChange={(e) => setJoinCode(e.target.value.toUpperCase())} required/>
            <button className="button button-outline" disabled={pending}>เข้าร่วม</button>
          </div>
        </form>
        {error && <div className="form-error">{error}</div>}
        <p className="privacy-note">รองรับทั้งการเล่นกับเพื่อนและโหมดทดสอบเดี่ยวกับ AI Bot 4 ระดับความยาก</p>
      </section>
    </div>
    {showLore && <LoreModal onClose={() => setShowLore(false)} />}
    <footer>สร้างปาร์ตี้ · สำรวจดันเจียน · ชิงชัยในอารีนา</footer>
  </main>
}

function SparkleItemPopup({
  itemId,
  catalog,
  onClose,
}: {
  itemId: string
  catalog: ItemDefinition[]
  onClose: () => void
}) {
  const item = catalog.find((entry) => entry.id === itemId)
  const rarity = item?.rarity || 'common'
  return (
    <div className="sparkle-item-toast" onClick={onClose} role="status" aria-live="polite">
      <div className="sparkle-rays" />
      <div className="sparkle-stars">
        <span className="sp-star s1">✦</span>
        <span className="sp-star s2">✧</span>
        <span className="sp-star s3">✨</span>
        <span className="sp-star s4">✦</span>
      </div>
      <div className="sparkle-icon-box">
        <ItemSprite itemId={itemId} alt={item?.name || itemId} className="sparkle-pixel-sprite" />
      </div>
      <div className="sparkle-info">
        <div className="sparkle-eyebrow">✨ วิ้ง! ได้รับไอเทมเข้ากระเป๋าแล้ว!</div>
        <div className="sparkle-title-row">
          <strong>{item?.name || itemId}</strong>
          <span className={`rarity-pill rarity-pill-${rarity}`}>{rarity.toUpperCase()}</span>
        </div>
        {item?.description && <p className="sparkle-desc">{item.description}</p>}
        {item?.skill_name && <small className="sparkle-skill">⚡ สกิลติดอาวุธ: {item.skill_name} ({item.skill_cost} MP)</small>}
      </div>
      <button type="button" className="sparkle-close-btn" onClick={onClose}>✕</button>
    </div>
  )
}

function ItemInspectorModal({
  item,
  onClose,
}: {
  item: ItemDefinition
  onClose: () => void
}) {
  const rarity = item.rarity || 'common'
  const slotLabels: Record<string, string> = {
    weapon: '⚔️ อาวุธหลัก (Main Hand)',
    armor: '🛡️ ชุดเกราะลำตัว (Body Armor)',
    helmet: '🪖 หมวกเกราะ (Headgear)',
    accessory: '💍 เครื่องประดับเวทมนตร์ (Accessory)',
    consumable: '🧪 ยา & ระเบิดกดใช้ (Consumable)',
  }

  return (
    <div className="item-inspector-overlay" onClick={onClose} role="dialog" aria-modal="true">
      <div className="item-inspector-card" onClick={(e) => e.stopPropagation()}>
        <button type="button" className="inspector-close-btn" onClick={onClose} title="ปิดหน้าต่าง (ESC)">✕</button>

        <div className="inspector-header">
          <div className={`inspector-sprite-frame rarity-border-${rarity}`}>
            <ItemSprite itemId={item.id} alt={item.name} className="inspector-sprite-img" />
          </div>
          <div className="inspector-meta">
            <span className={`inspector-rarity-pill rarity-pill-${rarity}`}>{rarity.toUpperCase()}</span>
            <h3>{item.name}</h3>
            <span className="inspector-slot">{slotLabels[item.slot || item.type] || item.type}</span>
            <small className="inspector-item-id">ID: {item.id}</small>
          </div>
        </div>

        <div className="inspector-body">
          {item.description && (
            <p className="inspector-desc">{item.description}</p>
          )}

          {/* Stats Grid */}
          <div className="inspector-stats-section">
            <span className="inspector-section-label">📊 ค่าสเตตัสที่มอบให้:</span>
            <div className="inspector-stats-grid">
              {item.stats.attack ? <div className="stat-chip atk"><span>⚔️ ATK</span> <strong>+{item.stats.attack}</strong></div> : null}
              {item.stats.defense ? <div className="stat-chip def"><span>🛡️ DEF</span> <strong>+{item.stats.defense}</strong></div> : null}
              {item.stats.max_hp ? <div className="stat-chip hp"><span>❤️ HP</span> <strong>+{item.stats.max_hp}</strong></div> : null}
              {item.stats.max_mp ? <div className="stat-chip mp"><span>💧 MP</span> <strong>+{item.stats.max_mp}</strong></div> : null}
              {item.stats.speed ? <div className="stat-chip spd"><span>⚡ SPD</span> <strong>{item.stats.speed > 0 ? `+${item.stats.speed}` : item.stats.speed}</strong></div> : null}
              {!item.stats.attack && !item.stats.defense && !item.stats.max_hp && !item.stats.max_mp && !item.stats.speed && (
                <div className="stat-chip neutral"><span>สเตตัส:</span> <strong>ไม่มีโบนัสสเตตัสตรง</strong></div>
              )}
            </div>
          </div>

          {/* Weapon Skill Details */}
          {item.skill_name && (
            <div className="inspector-skill-box">
              <div className="inspector-skill-title">
                <span className="eyebrow">✨ สกิลประจำอาวุธ</span>
                <strong>{item.skill_name}</strong>
                <span className="skill-cost-pill">ใช้ {item.skill_cost || 5} MP</span>
              </div>
              <p className="skill-desc-text">
                {item.skill_desc || 'ปลดปล่อยพลังการโจมตีพิเศษประจำอาวุธ'}
              </p>
              <small className="skill-target-text">
                🎯 รูปแบบเป้าหมาย: <b>{item.skill_target === 'aoe' ? 'โจมตีศัตรูทั้งหมดในสนาม (AoE)' : 'เป้าหมายเดี่ยว'}</b>
              </small>
            </div>
          )}

          {/* Consumable Effect */}
          {item.type === 'consumable' && item.effect && (
            <div className="inspector-effect-box">
              <span className="eyebrow">🧪 ประสิทธิภาพการกดใช้</span>
              <p>
                {item.effect.type === 'heal' && `❤️ ฟื้นฟูพลังชีวิตทันที +${item.effect.value} หน่วย`}
                {item.effect.type === 'heal_mp' && `💧 ฟื้นฟูพลังเวทมนตร์ทันที +${item.effect.value} MP`}
                {item.effect.type === 'damage_aoe' && `🔥 ขว้างระเบิดใส่ศัตรูทั้งหมด สร้างความเสียหายทันที ${item.effect.value} หน่วย`}
                {item.effect.type === 'smoke' && `💨 พรางตัวด้วยม่านหมอก เพิ่มอัตราหลบหลีกการโจมตี 75% พร้อมโอกาสซุ่มโจมตีสวนกลับ`}
              </p>
            </div>
          )}

          {/* Lore Quote */}
          {ITEM_LORE[item.id] && (
            <blockquote className="inspector-lore">
              {ITEM_LORE[item.id]}
            </blockquote>
          )}

          {/* Price / Trading info */}
          <div className="inspector-footer-strip">
            <span>🛒 ราคาซื้อในร้าน: <b>◈ {item.price || 20} Gold</b></span>
            <span>💰 มูลค่าขายคืน: <b>◈ {Math.floor((item.price || 20) / 2)} Gold</b></span>
          </div>
        </div>

        <button type="button" className="button button-gold full-width" onClick={onClose}>
          ✕ ปิดหน้าต่าง
        </button>
      </div>
    </div>
  )
}

function HeroHudBar({
  state,
  players,
  myId,
  canOpenBag,
  onOpenBag,
}: {
  state: PrivateState
  players: PublicPlayer[]
  myId: string
  canOpenBag: boolean
  onOpenBag: () => void
}) {
  const hpPct = state.stats.max_hp ? Math.min(100, Math.max(0, Math.round((state.stats.hp / state.stats.max_hp) * 100))) : 0
  const mpPct = state.stats.max_mp ? Math.min(100, Math.max(0, Math.round(((state.stats.mp || 0) / state.stats.max_mp) * 100))) : 0
  const expPct = state.stats.max_exp ? Math.min(100, Math.max(0, Math.round(((state.stats.exp || 0) / state.stats.max_exp) * 100))) : 0

  const archetypeNames: Record<string, string> = {
    sword: '🛡️ อัศวินดาบ & โล่',
    staff: '🔮 จอมเวทมนตรา',
    daggers: '🗡️ นักฆ่ามีดคู่',
    bow: '🏹 เรนเจอร์ธนูยาว',
  }

  return (
    <section className="hud-bar-wrap">
      <div className="hud-main-row">
        <div className="hud-hero-identity">
          <div className="hud-avatar-frame">
            <img src="/art/pixel/hero.png" alt="Hero" />
            <span className="hud-level-badge">Lv.{state.stats.level || 1}</span>
          </div>
          <div className="hud-hero-meta">
            <span className="hud-archetype-label">{archetypeNames[state.stats.archetype || 'sword'] || 'นักผจญภัย'}</span>
            <div className="hud-exp-bar" title={`EXP: ${state.stats.exp || 0} / ${state.stats.max_exp || 30}`}>
              <i style={{ width: `${expPct}%` }} />
            </div>
            <small>EXP {state.stats.exp || 0}/{state.stats.max_exp || 30}</small>
          </div>
        </div>

        <div className="hud-bars-group">
          <div className="hud-meter">
            <div className="hud-meter-label">
              <span>❤️ พลังชีวิต (HP)</span>
              <strong>{state.stats.hp} / {state.stats.max_hp}</strong>
            </div>
            <div className="hud-meter-track hp-track">
              <i style={{ width: `${hpPct}%` }} />
            </div>
          </div>

          <div className="hud-meter">
            <div className="hud-meter-label">
              <span>💧 มานา (MP)</span>
              <strong>{state.stats.mp || 0} / {state.stats.max_mp || 20}</strong>
            </div>
            <div className="hud-meter-track mp-track">
              <i style={{ width: `${mpPct}%` }} />
            </div>
          </div>
        </div>

        <div className="hud-stats-pills">
          <div className="hud-stat-pill"><span>ATK</span><strong>⚔️ {state.stats.atk}</strong></div>
          <div className="hud-stat-pill"><span>DEF</span><strong>🛡️ {state.stats.def}</strong></div>
          <div className="hud-stat-pill"><span>SPD</span><strong>⚡ {state.stats.spd || state.stats.base_spd || 5}</strong></div>
          <div className="hud-stat-pill gold"><span>GOLD</span><strong>◈ {state.gold}</strong></div>
          <div className="hud-stat-pill score"><span>SCORE</span><strong>★ {state.score}</strong></div>
        </div>

        <button
          type="button"
          className={`hud-bag-cta ${!canOpenBag ? 'is-locked' : ''}`}
          disabled={!canOpenBag}
          onClick={() => canOpenBag && onOpenBag()}
          title={canOpenBag ? 'กดเปิดกระเป๋าและอุปกรณ์สวมใส่' : 'กระเป๋าล็อกขณะอยู่ในการต่อสู้'}
        >
          <span className="hud-bag-icon">{canOpenBag ? '🎒' : '🔒'}</span>
          <div>
            <strong>{canOpenBag ? 'เปิดกระเป๋า' : 'ล็อกขณะต่อสู้'}</strong>
            <small>{state.inventory.length} / 12 ช่อง</small>
          </div>
        </button>
      </div>

      {/* Compact Party / Rivals Progress Strip */}
      <div className="hud-rivals-strip">
        <span className="rivals-strip-title">👥 สถานะผู้เล่นในห้อง:</span>
        <div className="rivals-pills-list">
          {players.map((p) => (
            <div key={p.player_id} className={`rival-mini-pill ${p.player_id === myId ? 'is-me' : ''} ${p.completed ? 'is-done' : ''}`}>
              <strong>{p.name}{p.player_id === myId ? ' (คุณ)' : ''}</strong>
              <span>
                {p.completed ? '🏁 จบดันเจียนแล้ว' : `ชั้น ${p.floor || 1} · ก้าว ${p.progress || 0}/6`}
              </span>
              <span className="rival-score-mini">★ {p.score}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function LoreModal({ onClose }: { onClose: () => void }) {
  return (
    <div className="lore-modal-overlay" onClick={onClose}>
      <div className="lore-tome" onClick={(e) => e.stopPropagation()}>
        <div className="lore-tome-header">
          <div>
            <h2>พงศาวดารแห่งสุสานเถ้าธุลี</h2>
            <p>Chronicles of the Ashen Catacombs · เรื่องเล่าแห่งความมืดและศิลาราชันย์มังกร</p>
          </div>
          <button className="button button-quiet" onClick={onClose}>✕ ปิด</button>
        </div>

        <div className="lore-chapter">
          <h4>🏰 ปฐมบท: อาณาจักรที่ล่มสลาย</h4>
          <p>
            กาลครั้งหนึ่งนานมาแล้ว อาณาจักร Eldergate เคยรุ่งเรืองด้วยอารยธรรมมนตราและอำนาจแห่งศิลาโบราณ ทว่าในคืนที่จันทรคราสสีเลือดปรากฏ ประตูมิติอเวจีได้เปิดออก เพลิงดำและพายุหมอกเถ้าธุลีกลืนกินเมืองทั้งเมืองให้จมดิ่งลงสู่ใต้พิภพ กลายเป็นสุสานวงกต 3 ชั้นที่ไร้แสงตะวัน
          </p>
        </div>

        <div className="lore-chapter">
          <h4>🔥 ชั้นที่ 1: สุสานเถ้าธุลี (The Ashen Crypt)</h4>
          <p>
            ชั้นบนสุดที่ปกคลุมด้วยเถ้ากระดูก ซากโครงกระดูกนักรบโบราณ และฝูงค้างคาวกระหายเลือด ณ ปลายทางของชั้นนี้มี <strong>Warden of Ash</strong> ผู้คุมวิญญาณสวมมงกุฎหินออบซิเดียน ถือค้อนเพลิงที่ลุกไหม้ด้วยวิญญาณคนบาป คอยพิพากษาผู้บุกรุกทุกคน
          </p>
        </div>

        <div className="lore-chapter">
          <h4>❄️ ชั้นที่ 2: วิหารน้ำแข็งนิรันดร์ (The Glacial Vault)</h4>
          <p>
            เมื่อก้าวลึกลงไป อุณหภูมิจะลดต่ำลงจนไอหายใจกลายเป็นเกล็ดน้ำแข็ง ทางเดินกลายเป็นธารน้ำแข็งโปร่งแสงที่กักขังนักโทษโบราณไว้ใต้พื้น มี <strong>Frost Colossus</strong> อสูรศิลาเยือกแข็งที่มีหัวใจเป็นผลึกน้ำแข็งพันปี คอยบดขยี้ผู้ที่กล้าฝ่าเข้ามา
          </p>
        </div>

        <div className="lore-chapter">
          <h4>🐲 ชั้นที่ 3: บัลลังก์มังกรอเวจี (The Dragon's Maw)</h4>
          <p>
            ชั้นลึกสุดที่เต็มไปด้วยธารลาวาเดือดและผลึกออบซิเดียน เป็นที่สถิตของ <strong>Dragon Regent</strong> จอมราชันย์มังกรทมิฬ ผู้พิทักษ์ <em>ศิลาราชันย์มังกร (Dragon Core)</em> ซึ่งสามารถมอบชีวิตอมตะและพลังอำนาจไร้ขีดจำกัด
          </p>
        </div>

        <div className="lore-chapter">
          <h4>⚔️ สังเวียนประลอง (The Final Arena)</h4>
          <p>
            เมื่อเหล่าผู้รอดชีวิตโค่นมังกรลงได้ ประตูมิติจะนำพาทุกคนเข้าสู่สังเวียนประลองศักดิ์สิทธิ์ ทุกอุปกรณ์ ความเร็ว และกลยุทธ์ที่รวบรวมมาตลอดการสำรวจ จะถูกตัดสิน ณ ที่แห่งนี้ เพื่อหาผู้ชนะเลิศเพียงหนึ่งเดียวที่จะได้ครอบครองศิลาและออกจากสุสานเถ้าธุลี!
          </p>
        </div>

        <div style={{ textAlign: 'center', marginTop: '16px' }}>
          <button className="button button-gold" onClick={onClose}>ข้าพร้อมจะเผชิญหน้ากับความมืดแล้ว ⚔️</button>
        </div>
      </div>
    </div>
  )
}

function PreparationPanel({
  players,
  isReady,
  phase,
  isHost,
  onReady,
  onKick,
  onOpenBag,
}: {
  players: PublicPlayer[]
  isReady: boolean
  phase: string
  isHost: boolean
  onReady: (ready: boolean) => void
  onKick: (playerID: string) => void
  onOpenBag: () => void
}) {
  const allFinished = players.every((player) => player.completed)
  return <section className="panel preparation-panel">
    <div className="panel-title">
      <div>
        <span className="eyebrow">FINAL CHECK · PREPARATION</span>
        <h2>{phase === 'pvp' ? 'พร้อมสู้ใน Arena' : 'เตรียมความพร้อมก่อนเข้าสู่สังเวียน PvP'}</h2>
      </div>
      <span className={`ready-tag ${isReady ? 'is-ready' : ''}`}>{isReady ? 'พร้อมแล้ว' : 'ยังไม่พร้อม'}</span>
    </div>
    <p className="muted" style={{ fontSize: '15px', marginBottom: '16px' }}>
      {allFinished
        ? 'ผู้เล่นทุกคนพิชิตดันเจียนครบทั้ง 3 ชั้นแล้ว! กรุณาเปิดกระเป๋าเพื่อตรวจสอบอุปกรณ์สวมใส่และกดใช้ยาฟื้นฟู จากนั้นกดปุ่มพร้อมเพื่อเริ่มศึกตัดสิน'
        : 'คุณพิชิตดันเจียนครบ 3 ชั้นแล้ว! สามารถเปิดกระเป๋าจัดเตรียมอุปกรณ์ระหว่างรอผู้เล่นคนอื่นได้'}
    </p>
    <div className="prep-roster">
      {players.map((player) => (
        <span key={player.player_id} className={player.completed ? 'prep-complete' : ''}>
          {player.name} · {player.completed ? '✓ พิชิตดันเจียนแล้ว' : player.connected ? `กำลังลุยชั้น ${player.floor}` : 'หลุดการเชื่อมต่อ'}
          {isHost && !player.connected && <button className="text-button" onClick={() => onKick(player.player_id)}>นำออก</button>}
        </span>
      ))}
    </div>
    <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', marginTop: '18px' }}>
      <button type="button" className="button button-outline" onClick={onOpenBag}>
        🎒 เปิดกระเป๋าจัดอุปกรณ์ก่อนสู้
      </button>
      {phase !== 'pvp' && (
        <button className={`button ${isReady ? 'button-muted' : 'button-gold'}`} disabled={phase === 'preparation' && !allFinished} onClick={() => onReady(!isReady)}>
          {isReady ? 'ยกเลิกความพร้อม' : '⚔️ พร้อมเข้าสู่สนามประลอง PvP'}
        </button>
      )}
    </div>
  </section>
}

function ArenaPanel({
  players,
  me,
  isHost,
  state,
  catalog,
  onCommand,
}: {
  players: PublicPlayer[]
  me: Player
  isHost: boolean
  state: PrivateState
  catalog: ItemDefinition[]
  onCommand: (type: string, payload?: object) => void
  onOpenBag: () => void
}) {
  const combatItems = state.inventory.filter((stack) => {
    const def = catalog.find((item) => item.id === stack.item_id)
    return def?.type === 'consumable' && def.usable_in_combat
  })
  const canSkill = Boolean(state.stats.weapon_skill && (state.stats.mp || 0) >= (state.stats.weapon_skill_cost || 5))
  const opponents = players.filter((player) => player.player_id !== me.player_id)
  const currentOpponent =
    opponents.find((p) => p.player_id === state.pvp_current_opponent_id) ||
    opponents.find((p) => p.hp > 0) ||
    opponents[0]

  const boutNum = state.pvp_bout_number || 1
  const totalBouts = state.pvp_total_bouts || Math.max(1, opponents.length)
  const isYourTurn = Boolean(state.pvp_your_turn)
  const pvpLog = state.pvp_log || []
  const pvpBouts = state.pvp_bouts || []

  return (
    <section className="panel arena-panel">
      <div className="panel-title">
        <div>
          <span className="eyebrow">
            {`ROUND-ROBIN PVP TOURNAMENT · คู่ที่ ${boutNum} จาก ${totalBouts} · สู้จนกว่า HP ฝ่ายใดจะเป็น 0`}
          </span>
          <h2>⚔️ สังเวียนประลองจัดอันดับ (Round-Robin Arena)</h2>
        </div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
          <span className="gold-badge">🏆 ชนะ {state.pvp_wins || 0} · แพ้ {state.pvp_losses || 0}</span>
          <span className="map-difficulty">★ คะแนน PvP: {state.pvp_score || 0}</span>
        </div>
      </div>

      {/* Round-Robin Schedule & Standings Bar */}
      {pvpBouts.length > 0 && (
        <div className="pvp-bracket-strip">
          <div className="pvp-bracket-title">📋 ตารางการประลองพบกันหมด (Round-Robin):</div>
          <div className="pvp-bouts-row">
            {pvpBouts.map((b) => {
              const isCurrentBout = !b.finished && b.bout_number === boutNum
              return (
                <div
                  key={b.bout_number}
                  className={`pvp-bout-pill ${b.finished ? 'is-done' : ''} ${isCurrentBout ? 'is-active' : ''}`}
                >
                  <span className="bout-num-tag">{`คู่ที่ ${b.bout_number}`}</span>
                  <strong>{b.player_a_name} vs {b.player_b_name}</strong>
                  {b.finished ? (
                    <span className="bout-winner-tag">👑 {b.winner_name} ชนะ!</span>
                  ) : isCurrentBout ? (
                    <span className="bout-live-tag">🔥 กำลังดวลเดือด</span>
                  ) : (
                    <span className="bout-wait-tag">⏳ รอแข่ง</span>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Main 1v1 Giant Duel Stage */}
      {currentOpponent && (
        <div className="pvp-duel-stage">
          {/* Left Fighter: You */}
          <div className="duel-fighter-card is-self">
            <span className="duel-corner-badge self-badge">🛡️ ตัวละครของคุณ</span>
            <div className="duel-sprite-box">
              <img src="/art/pixel/hero.png" alt={me.name} className="duel-giant-sprite" />
              {state.equipment.weapon && (
                <div className="duel-weapon-float" title={state.equipment.weapon}>
                  <ItemSprite itemId={state.equipment.weapon} />
                </div>
              )}
            </div>
            <strong className="duel-fighter-name">{me.name} (คุณ)</strong>
            <div className="duel-stat-badges">
              <span>⚔️ ATK {state.stats.atk}</span>
              <span>🛡️ DEF {state.stats.def}</span>
              <span>⚡ SPD {state.stats.spd || 5}</span>
            </div>
            <div className="duel-bars">
              <div className="hp-bar-row"><span>❤️ HP</span><strong>{state.stats.hp} / {state.stats.max_hp}</strong></div>
              <div className="health-track"><i style={{ width: `${Math.max(0, (state.stats.hp / (state.stats.max_hp || 1)) * 100)}%` }} /></div>
              <div className="hp-bar-row" style={{ marginTop: '6px' }}><span>💧 MP</span><strong>{state.stats.mp || 0} / {state.stats.max_mp || 20}</strong></div>
              <div className="hud-meter-track mp-track"><i style={{ width: `${Math.max(0, ((state.stats.mp || 0) / (state.stats.max_mp || 20)) * 100)}%` }} /></div>
            </div>
          </div>

          <div className="duel-vs-column">
            <div className="duel-vs-emblem">VS</div>
            <small>สู้กันจนกว่า HP ฝ่ายใดจะเป็น 0</small>
          </div>

          {/* Right Fighter: Current Opponent */}
          <div className="duel-fighter-card is-rival">
            <span className="duel-corner-badge rival-badge">
              🎯 คู่ต่อสู้ปัจจุบัน {currentOpponent.is_bot ? '· 🤖 AI BOT' : ''}
            </span>
            <div className="duel-sprite-box">
              <img src="/art/pixel/elite_knight.png" alt={currentOpponent.name} className="duel-giant-sprite rival-Tint" />
            </div>
            <strong className="duel-fighter-name">{currentOpponent.name}</strong>
            <div className="duel-stat-badges">
              <span>⚔️ ATK {currentOpponent.atk || 10}</span>
              <span>🛡️ DEF {currentOpponent.def || 2}</span>
              <span>🏆 ชนะ {currentOpponent.pvp_wins ?? currentOpponent.pvp_knockouts}</span>
            </div>
            <div className="duel-bars">
              <div className="hp-bar-row"><span>❤️ HP ศัตรู</span><strong>{currentOpponent.hp} / {currentOpponent.max_hp}</strong></div>
              <div className="health-track"><i style={{ width: `${Math.max(0, (currentOpponent.hp / (currentOpponent.max_hp || 1)) * 100)}%` }} /></div>
            </div>
          </div>
        </div>
      )}

      {/* Live PvP Combat Log */}
      {pvpLog.length > 0 && (
        <div className="combat-log pvp-combat-log">
          {pvpLog.slice(-6).map((entry, idx) => (
            <p key={`${idx}-${entry}`}>{entry}</p>
          ))}
        </div>
      )}

      {/* Full Action Bar: Normal Attack, Weapon Skill, Defend, Bombs & Potions */}
      {currentOpponent && (
        <div className="combat-actions pvp-action-bar">
          <p className="pvp-turn-status" role="status">
            {isYourTurn ? 'ตาของคุณ เลือกแอ็กชันได้เลย' : `กำลังรอ ${currentOpponent.name} เล่น`}
          </p>
          <button
            className="button button-primary"
            disabled={!isYourTurn}
            onClick={() => {
              sound.playSlash()
              onCommand('pvp.action', { action: 'ATTACK', target_player_id: currentOpponent.player_id })
            }}
          >
            ⚔️ โจมตีปกติใส่ {currentOpponent.name}
          </button>

          {state.stats.weapon_skill && (
            <button
              className="button skill-button"
              disabled={!isYourTurn || !canSkill}
              onClick={() => {
                sound.playCrit()
                onCommand('pvp.action', { action: 'SKILL', target_player_id: currentOpponent.player_id })
              }}
              title={state.stats.weapon_skill_desc}
            >
              <span>✨ ใช้สกิลอาวุธ: {state.stats.weapon_skill}</span>
              <span className="skill-cost-badge">{state.stats.weapon_skill_cost || 5} MP</span>
            </button>
          )}

          <button
            className="button button-outline"
            disabled={!isYourTurn}
            onClick={() => {
              sound.playShield()
              onCommand('pvp.action', { action: 'DEFEND', target_player_id: currentOpponent.player_id })
            }}
          >
            🛡️ ตั้งรับ & สวนกลับ (+8 MP)
          </button>

          {combatItems.map((item) => {
            const def = catalog.find((entry) => entry.id === item.item_id)
            const isBomb = def?.effect?.type === 'damage_aoe' || def?.effect?.type === 'smoke'
            return (
              <button
                className={`button ${isBomb ? 'button-gold' : 'button-outline'} button-with-icon`}
                key={item.item_id}
                disabled={!isYourTurn}
                onClick={() => {
                  if (isBomb) sound.playCrit()
                  else sound.playShield()
                  onCommand('pvp.action', { action: 'USE_ITEM', target_player_id: currentOpponent.player_id, item_id: item.item_id })
                }}
              >
                <ItemSprite itemId={item.item_id} className="btn-pixel-icon" />
                {def?.effect?.type === 'damage_aoe'
                  ? `💣 ปา ${def.name} (ระเบิด 30 ดาเมจ!) ×${item.quantity}`
                  : def?.effect?.type === 'smoke'
                  ? `💨 ปา ${def.name} (พรางตัวลดดาเมจ 75% + ลอบโจมตี) ×${item.quantity}`
                  : `🧪 ใช้ ${def?.name || item.item_id} (×${item.quantity})`}
              </button>
            )
          })}
        </div>
      )}

      {isHost && players.some((player) => !player.connected) && (
        <button className="button button-quiet activity-leave" onClick={() => onCommand('pvp.force_finish')}>
          จบ PvP สำหรับผู้เล่นที่หลุด
        </button>
      )}
    </section>
  )
}

function ResultsPanel({ roomCode, results, onReplay, onLeave }: { roomCode: string; results: ResultEntry[]; onReplay: () => void; onLeave: () => void }) {
  return <section className="panel results-panel">
    <div className="panel-title">
      <div><span className="eyebrow">MATCH COMPLETE</span><h2>🏆 สรุปผลการแข่งขัน</h2></div>
      <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
        <button className="button button-gold" onClick={onReplay}>🔄 เล่นใหม่อีกรอบ</button>
        <a className="button button-outline" style={{ display: 'inline-flex', alignItems: 'center' }} href={`/api/v1/rooms/${encodeURIComponent(roomCode)}/results.csv`} download>ดาวน์โหลด CSV</a>
        <button className="button button-quiet" onClick={onLeave}>ออกจากห้อง</button>
      </div>
    </div>
    <div className="results-list">
      {results.map((result) => (
        <article className={`result-card ${result.rank === 1 ? 'winner' : ''}`} key={result.player_id}>
          <div className="result-rank">#{result.rank}</div>
          <div className="result-player">
            <strong>{result.name} {result.rank === 1 && '👑'}</strong>
            <small>คะแนนดันเจียน (PvE): {result.pve_score} · คะแนนประลอง (PvP): {result.pvp_score} · โบนัสทอง: +{result.gold_score} · โบนัสแชมป์: +{result.bonus} · หักพ่ายแพ้: −{result.penalties}</small>
            <small>ดาเมจ PvP: {result.pvp_damage} · สังหาร (KO): {result.pvp_knockouts} · ตายใน PvP: {result.pvp_deaths} · ปราบบอส: {result.bosses_defeated} ตัว</small>
          </div>
          <strong className="result-total">{result.total_score} คะแนน</strong>
        </article>
      ))}
    </div>
  </section>
}

function RoomActivityPanel({
  state,
  catalog,
  onCommand,
  onOpenBag,
}: {
  state: PrivateState
  catalog: ItemDefinition[]
  onCommand: (type: string, payload?: object) => void
  onOpenBag: () => void
}) {
  const activity = state.activity!
  const [shopFilter, setShopFilter] = useState<'all' | 'consumable' | 'weapon' | 'armor'>('all')

  const roomLabel: Record<string, string> = {
    TREASURE: '🎁 ห้องสมบัติโบราณ (Treasure Vault)',
    SHOP: '🛒 ตลาดร้านค้านักเดินทาง (Dungeon Market)',
    FORGE: '⚒️ โรงตีเหล็กอัปเกรดศัสตรา (Ancient Blacksmith Forge)',
    EVENT: '🔥 เหตุการณ์พิเศษในดันเจียน (Special Event)',
    PUZZLE: '🧩 ห้องปริศนาอักษรรูน (Puzzle Chamber)',
  }

  const filteredStock = useMemo(() => {
    if (!activity.stock) return []
    return activity.stock.filter((s) => {
      if (shopFilter === 'all') return true
      const def = catalog.find((c) => c.id === s.item_id)
      if (!def) return true
      if (shopFilter === 'consumable') return def.type === 'consumable'
      if (shopFilter === 'weapon') return def.type === 'weapon'
      if (shopFilter === 'armor') return def.type === 'armor' || def.type === 'helmet' || def.type === 'accessory'
      return true
    })
  }, [activity.stock, shopFilter, catalog])

  return <section className="panel activity-panel">
    <div className="panel-title">
      <div>
        <span className="eyebrow">ROOM ENCOUNTER · {activity.type}</span>
        <h2>{roomLabel[activity.type] || activity.type}</h2>
      </div>
      <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
        <button type="button" className="button button-outline" onClick={onOpenBag}>
          🎒 เปิดกระเป๋า ({state.inventory.length}/12)
        </button>
        <span className="gold-badge">◈ {state.gold} Gold</span>
      </div>
    </div>

    {activity.options && (
      <div className="activity-options">
        {activity.options.map((option) => {
          const item = catalog.find((entry) => entry.id === option.item_id)
          return (
            <article className="activity-option" key={option.id}>
              {option.item_id ? (
                <ItemSprite itemId={option.item_id} alt={item?.name || option.label} className="item-pixel-art-lg" />
              ) : (
                <div className="activity-choice-icon">
                  {option.kind === 'gold'
                    ? '💰'
                    : option.kind === 'heal'
                    ? '🔥'
                    : option.kind === 'exp_gold'
                    ? '📜'
                    : option.kind === 'forge_atk'
                    ? '⚔️'
                    : option.kind === 'forge_def'
                    ? '🛡️'
                    : option.kind === 'forge_spd'
                    ? '⚡'
                    : '✨'}
                </div>
              )}
              <div className="activity-option-body">
                <strong>{option.label}</strong>
                <p>{option.description}</p>
                {item && <small className="item-stat-line">{item.description}</small>}
                {option.item_id && ITEM_LORE[option.item_id] && <small className="item-lore-quote">{ITEM_LORE[option.item_id]}</small>}
              </div>
              <button className="button button-primary" onClick={() => onCommand('room.action', { action: 'claim', choice_id: option.id })}>
                เลือกทางนี้ →
              </button>
            </article>
          )
        })}
      </div>
    )}

    {activity.stock && (
      <>
        <div className="shop-filter-tabs">
          <button type="button" className={`shop-tab ${shopFilter === 'all' ? 'active' : ''}`} onClick={() => setShopFilter('all')}>ทั้งหมด ({activity.stock.length})</button>
          <button type="button" className={`shop-tab ${shopFilter === 'consumable' ? 'active' : ''}`} onClick={() => setShopFilter('consumable')}>🧪 ยา & ระเบิด</button>
          <button type="button" className={`shop-tab ${shopFilter === 'weapon' ? 'active' : ''}`} onClick={() => setShopFilter('weapon')}>⚔️ อาวุธ</button>
          <button type="button" className={`shop-tab ${shopFilter === 'armor' ? 'active' : ''}`} onClick={() => setShopFilter('armor')}>🛡️ เกราะ & เครื่องประดับ</button>
        </div>
        <div className="shop-stock-grid">
          {filteredStock.map((stock) => {
            const def = catalog.find((entry) => entry.id === stock.item_id)
            return (
              <article className={`activity-option shop-card ${!stock.available ? 'is-sold' : ''}`} key={stock.item_id}>
                <ItemSprite itemId={stock.item_id} alt={stock.name} className="item-pixel-art-lg" />
                <div className="activity-option-body">
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <strong>{stock.name}</strong>
                    {def?.rarity && <span className={`rarity-pill rarity-pill-${def.rarity}`}>{def.rarity.toUpperCase()}</span>}
                  </div>
                  <p>{def?.description || 'ไอเทมสำหรับนักผจญภัย'}</p>
                  {def?.skill_name && <small style={{ color: '#c09bff', fontWeight: 600 }}>✨ สกิล: {def.skill_name} ({def.skill_cost} MP)</small>}
                  {ITEM_LORE[stock.item_id] && <small className="item-lore-quote">{ITEM_LORE[stock.item_id]}</small>}
                </div>
                <button
                  className="button button-gold"
                  disabled={!stock.available || state.gold < stock.price}
                  onClick={() => onCommand('room.action', { action: 'buy', item_id: stock.item_id })}
                >
                  {stock.available ? `ซื้อ · ${stock.price} G` : 'ซื้อแล้ว'}
                </button>
              </article>
            )
          })}
        </div>
        {state.inventory.length > 0 && (
          <div className="shop-sell-section">
            <div className="section-heading">
              <h3>💰 ขายไอเทมในกระเป๋าแลกทอง (Sell Unwanted Gear · ได้รับ 50% ของราคา)</h3>
              <span>ขายของที่ไม่ได้ใช้เพื่อนำ Gold ไปซื้ออาวุธ ยา หรือระเบิดเพิ่มได้</span>
            </div>
            <div className="shop-sell-grid">
              {state.inventory.map((stack) => {
                const def = catalog.find((entry) => entry.id === stack.item_id)
                const sellGold = Math.max(5, Math.floor((def?.price || 10) / 2))
                return (
                  <div className="shop-sell-chip" key={stack.item_id}>
                    <ItemSprite itemId={stack.item_id} className="btn-pixel-icon" />
                    <div className="shop-sell-meta">
                      <strong>{def?.name || stack.item_id} ×{stack.quantity}</strong>
                      <small>ขายได้ +{sellGold} Gold</small>
                    </div>
                    <button
                      type="button"
                      className="button button-outline compact"
                      onClick={() => onCommand('room.action', { action: 'sell', item_id: stack.item_id })}
                    >
                      ขาย (+{sellGold}G)
                    </button>
                  </div>
                )
              })}
            </div>
          </div>
        )}
      </>
    )}
    {(activity.type === 'SHOP' || activity.type === 'FORGE') && (
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '20px' }}>
        <button className="button button-primary" onClick={() => onCommand('room.action', { action: 'leave' })}>
          เดินทางต่อออกจากห้องนี้ →
        </button>
      </div>
    )}
  </section>
}

type VFX = { id: number; type: 'slash' | 'magic' | 'arrow' | 'shield' | 'crit'; text?: string; colorClass?: string }

function CombatPanel({
  state,
  catalog,
  onCommand,
  onOpenBag,
}: {
  state: PrivateState
  catalog: ItemDefinition[]
  onCommand: (type: string, payload?: object) => void
  onOpenBag: () => void
}) {
  const combat = state.combat!
  const potions = state.inventory.filter((stack) => catalog.find((item) => item.id === stack.item_id)?.type === 'consumable' && catalog.find((item) => item.id === stack.item_id)?.usable_in_combat)

  const [selectedTarget, setSelectedTarget] = useState<number>(combat.selected_enemy ?? 0)
  const [vfxList, setVfxList] = useState<VFX[]>([])
  const [shake, setShake] = useState(false)
  const combatLogRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (combatLogRef.current) {
      combatLogRef.current.scrollTop = combatLogRef.current.scrollHeight
    }
  }, [combat.log])

  useEffect(() => {
    if (combat.selected_enemy !== undefined) {
      setSelectedTarget(combat.selected_enemy)
    }
  }, [combat.selected_enemy])

  const enemies = combat.enemies && combat.enemies.length > 0 ? combat.enemies : [
    {
      index: 0,
      monster_id: combat.monster_id,
      name: combat.name,
      tier: combat.tier,
      hp: combat.hp,
      max_hp: combat.max_hp,
      attack: combat.attack,
      defense: combat.defense,
      speed: 5,
      intent: 'ATTACK',
      intent_value: combat.attack,
      alive: combat.hp > 0,
    }
  ]

  const activeTargetEnemy = enemies.find((e) => e.index === selectedTarget && e.alive) || enemies.find((e) => e.alive) || enemies[0]
  const playerSpeed = state.stats.spd || state.stats.base_spd || 5
  const isPlayerFaster = combat.first_strike || (activeTargetEnemy && playerSpeed >= activeTargetEnemy.speed)

  function triggerVFX(type: 'slash' | 'magic' | 'arrow' | 'shield' | 'crit', text?: string, colorClass?: string) {
    const id = Date.now() + Math.random()
    setVfxList((prev) => [...prev, { id, type, text, colorClass }])
    setTimeout(() => {
      setVfxList((prev) => prev.filter((v) => v.id !== id))
    }, 1100)
  }

  function handleAttack() {
    const archetype = state.stats.archetype || 'sword'
    if (activeTargetEnemy.intent === 'HEAVY') {
      setShake(true)
      setTimeout(() => setShake(false), 380)
    }
    if (archetype === 'bow') {
      sound.playArrow()
      triggerVFX('arrow', `ยิงธนูใส่ ${activeTargetEnemy.name}!`, 'dmg-red')
    } else if (archetype === 'staff') {
      sound.playMagic()
      triggerVFX('magic', `ยิงเวทใส่ ${activeTargetEnemy.name}!`, 'dmg-mp')
    } else if (archetype === 'daggers') {
      sound.playCrit()
      triggerVFX('slash', `ฟันมีดคู่ใส่ ${activeTargetEnemy.name}!`, 'dmg-crit')
    } else {
      sound.playSlash()
      triggerVFX('slash', `ฟันดาบใส่ ${activeTargetEnemy.name}!`, 'dmg-red')
    }
    onCommand('combat.action', { action: 'ATTACK', target_enemy: activeTargetEnemy.index })
  }

  function handleSkill() {
    const skillName = state.stats.weapon_skill || 'สกิลอาวุธ'
    const archetype = state.stats.archetype || 'sword'
    setShake(true)
    setTimeout(() => setShake(false), 380)
    if (archetype === 'staff') {
      sound.playMagic()
      triggerVFX('magic', `⚡ ${skillName}!`, 'dmg-crit')
    } else if (archetype === 'bow') {
      sound.playArrow()
      triggerVFX('arrow', `🏹 ${skillName}!`, 'dmg-crit')
    } else if (archetype === 'daggers') {
      sound.playCrit()
      triggerVFX('crit', `💥 ${skillName}!`, 'dmg-crit')
    } else {
      sound.playShield()
      triggerVFX('slash', `⚔️ ${skillName}!`, 'dmg-crit')
    }
    onCommand('combat.action', { action: 'SKILL', target_enemy: activeTargetEnemy.index })
  }

  function handleDefend() {
    sound.playShield()
    triggerVFX('shield', '🛡️ ตั้งรับ!', 'dmg-block')
    onCommand('combat.action', { action: 'DEFEND' })
  }

  function handleUseItem(itemId: string) {
    const item = catalog.find((e) => e.id === itemId)
    if (item?.effect?.type === 'damage_aoe') {
      sound.playCrit()
      triggerVFX('crit', `💣 ปาระเบิดเพลิงใส่ศัตรูทั้งหมด!`, 'dmg-crit')
    } else if (item?.effect?.type === 'heal_mp') {
      sound.playMagic()
      triggerVFX('shield', `💧 ฟื้นฟู +${item.effect.value} MP!`, 'dmg-mp')
    } else {
      sound.playShield()
      triggerVFX('shield', `❤️ ฟื้นฟู +${item?.effect?.value || 0} HP!`, 'dmg-heal')
    }
    onCommand('combat.action', { action: 'USE_ITEM', item_id: itemId })
  }

  // Server sets combat.victory = true only on genuine victory; on death combat.victory is false!
  const isVictory = Boolean(combat.victory)
  const droppedItem = combat.loot_item_id ? catalog.find((item) => item.id === combat.loot_item_id) : undefined

  return (
    <section className={`panel combat-panel ${combat.finished ? 'combat-finished' : ''} ${shake ? 'screen-shake' : ''}`}>
      <div className="panel-title">
        <div>
          <span className="eyebrow">{combat.finished ? 'ENCOUNTER COMPLETE' : `TURN ${combat.turn || 1} · เลือกการกระทำของคุณ`}</span>
          <h2>⚔️ ปะทะศัตรู: {combat.name}</h2>
        </div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <button
            type="button"
            className="button button-outline compact"
            disabled={!combat.finished}
            onClick={() => combat.finished && onOpenBag()}
            title={combat.finished ? 'เปิดกระเป๋าจัดการอุปกรณ์' : 'กระเป๋าล็อกขณะต่อสู้ (กดใช้ยาหรือระเบิดจากแถบคำสั่งด้านล่าง)'}
          >
            {combat.finished ? '🎒 เปิดกระเป๋า' : '🔒 กระเป๋าล็อกขณะต่อสู้'}
          </button>
          <span className="map-difficulty">{combat.tier.toUpperCase()}</span>
        </div>
      </div>

      {/* Dark Fantasy Boss Encounter Banner */}
      {combat.tier === 'boss' && (
        <div className="boss-lore-banner">
          <div className="boss-title-subtitle">👑 BOSS ENCOUNTER · {BOSS_LORE[combat.monster_id]?.title || 'จอมอสูรแห่งสุสาน'}</div>
          <p className="boss-quote">{BOSS_LORE[combat.monster_id]?.quote || '« เจ้ากล้าดียังไงที่เหยียบย่ำเข้ามาในเขตแดนของข้า! »'}</p>
        </div>
      )}

      {/* Speed & Initiative Turn Order Banner */}
      <div className="turn-order-banner">
        <div className={`speed-indicator ${isPlayerFaster ? 'faster' : 'slower'}`}>
          {combat.first_strike ? (
            <span>🎯 <strong>Ranger First Strike!</strong> คุณเปิดฉากยิงฟรีในเทิร์นแรกเรียบร้อยแล้ว</span>
          ) : isPlayerFaster ? (
            <span>⚡ <strong>ความเร็วเหนือกว่า!</strong> (SPD ของคุณ {playerSpeed} vs ศัตรู {activeTargetEnemy?.speed || 5}) คุณได้โจมตีก่อน</span>
          ) : (
            <span>⚠️ <strong>ศัตรูว่องไวกว่า!</strong> (SPD ของคุณ {playerSpeed} vs ศัตรู {activeTargetEnemy?.speed || 5}) ศัตรูจะชิงโจมตีก่อน</span>
          )}
        </div>
        <div style={{ fontSize: '14px', color: '#d4cae3' }}>
          คลิกที่การ์ดศัตรูเพื่อเปลี่ยนเป้าหมาย · เป้าหมายปัจจุบัน: <strong style={{ color: '#ffd166' }}>{activeTargetEnemy.name}</strong>
        </div>
      </div>

      {/* Multi-Monster Combat Stage with Giant Hero & Monster Sprites */}
      <div className="combat-stage-multi">
        {vfxList.map((vfx) => (
          <div className="floating-text-container" key={vfx.id}>
            {vfx.text && <div className={`floating-number ${vfx.colorClass || 'dmg-red'}`}>{vfx.text}</div>}
            {vfx.type === 'slash' && <div className="fx-slash-blade" />}
            {vfx.type === 'magic' && <div className="fx-magic-nova" />}
            {vfx.type === 'arrow' && <div className="fx-arrow-trail" />}
            {vfx.type === 'shield' && <div className="fx-shield-barrier" />}
            {vfx.type === 'crit' && <div className="fx-magic-nova" />}
          </div>
        ))}

        {enemies.map((enemy) => {
          const isSelected = activeTargetEnemy.index === enemy.index
          const isHeavy = enemy.intent === 'HEAVY'
          return (
            <div
              key={enemy.index}
              className={`monster-card enemy-battle-card ${enemy.tier === 'boss' ? 'is-boss-card' : ''} ${isSelected ? 'is-selected' : ''} ${!enemy.alive ? 'is-dead' : ''}`}
              onClick={() => {
                if (enemy.alive) {
                  setSelectedTarget(enemy.index)
                  onCommand('combat.select_target', { target_enemy: enemy.index })
                }
              }}
            >
              {isSelected && enemy.alive && <span className="target-badge">🎯 เป้าหมายหลัก</span>}
              <div className="monster-card-header">
                <span className="monster-name-wrap">{enemy.name}</span>
                <span className="monster-spd-tag">⚡ SPD {enemy.speed}</span>
              </div>

              {enemy.alive ? (
                <div className={`monster-intent-pill ${isHeavy ? 'intent-heavy' : ''}`}>
                  {isHeavy ? `⚠️ เตรียมทุบหนัก! (${enemy.intent_value} ATK)` : `🗡️ เตรียมโจมตี (${enemy.intent_value} ATK)`}
                </div>
              ) : (
                <div className="monster-intent-pill" style={{ color: '#888' }}>
                  💀 กำจัดแล้ว
                </div>
              )}

              <div className="monster-card-sprite">
                <img
                  src={getMonsterSpritePath(enemy.monster_id)}
                  alt={enemy.name}
                  onError={(e) => {
                    const target = e.currentTarget
                    if (!target.src.endsWith('/art/pixel/goblin.png')) {
                      target.src = '/art/pixel/goblin.png'
                    }
                  }}
                />
              </div>

              <div className="monster-card-stats">
                <div className="hp-bar-row">
                  <span>❤️ HP ศัตรู</span>
                  <strong>{enemy.hp} / {enemy.max_hp}</strong>
                </div>
                <div className="health-track">
                  <i style={{ width: `${Math.max(0, (enemy.hp / (enemy.max_hp || 1)) * 100)}%` }} />
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', color: '#cfc5df', marginTop: '8px', fontWeight: 700 }}>
                  <span>⚔️ ATK {enemy.attack}</span>
                  <span>🛡️ DEF {enemy.defense}</span>
                </div>
              </div>
            </div>
          )
        })}

        {/* Combat Finished Overlay Modal (Victory / Defeat Dialog with Sparkle Loot Showcase) */}
        {combat.finished && (
          <div className="combat-finish-overlay">
            <div className={`combat-finish-card ${isVictory ? 'is-victory' : 'is-defeat'}`}>
              <h3 className="combat-finish-title">
                {isVictory ? '🎉 ได้รับชัยชนะเหนือศัตรู!' : '💀 พ่ายแพ้ในการต่อสู้!'}
              </h3>
              <p className="combat-finish-desc">
                {isVictory
                  ? 'กำจัดศัตรูสำเร็จ! ได้รับค่าประสบการณ์ (EXP), คะแนน และ Gold เรียบร้อยแล้ว'
                  : 'คุณล้มลงในการต่อสู้ ระบบได้ส่งคุณกลับมายังห้องล่าสุดที่ผ่านแล้ว (Checkpoint) พร้อมฟื้นฟู HP 50% เพื่อให้คุณปรับอุปกรณ์หรือเลือกเส้นทางลุยต่อได้ทันที'}
              </p>

              {isVictory && combat.loot_item_id && (
                <div className="victory-loot-sparkle-box">
                  <div className="victory-loot-glow" />
                  <ItemSprite itemId={combat.loot_item_id} className="victory-loot-sprite" />
                  <div className="victory-loot-meta">
                    <span className="sparkle-eyebrow">✨ วิ้ง! มอนสเตอร์ดรอปไอเทมใหม่!</span>
                    <strong>{droppedItem?.name || combat.loot_item_id}</strong>
                    {droppedItem?.description && <small>{droppedItem.description}</small>}
                  </div>
                </div>
              )}

              <button
                className={`button ${isVictory ? 'button-gold' : 'button-outline'}`}
                style={{ width: '100%', fontSize: '16px', padding: '14px 22px', marginTop: '10px' }}
                onClick={() => {
                  sound.playLevelUp()
                  onCommand('combat.dismiss')
                }}
              >
                {isVictory ? '🗺️ เดินทางต่อในแผนที่ดันเจียน' : '🛡️ กลับสู่จุดเซฟบนแผนที่ดันเจียน'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Combat Log */}
      <div className="combat-log" ref={combatLogRef}>
        {combat.log.map((entry, index) => <p key={`${index}-${entry}`}>{entry}</p>)}
      </div>

      {/* Action Buttons */}
      {!combat.finished && (
        <div className="combat-actions">
          <button className="button button-primary" onClick={handleAttack}>
            ⚔️ โจมตีปกติ ({activeTargetEnemy.name})
          </button>

          {state.stats.weapon_skill && (
            <button
              className="button skill-button"
              disabled={(state.stats.mp || 0) < (state.stats.weapon_skill_cost || 0) || (combat.skill_cooldown || 0) > 0}
              onClick={handleSkill}
              title={state.stats.weapon_skill_desc}
            >
              <span>✨ {state.stats.weapon_skill} {state.stats.weapon_skill_target === 'aoe' ? '(หมู่)' : ''}</span>
              {(combat.skill_cooldown || 0) > 0 ? (
                <span className="skill-cost-badge cooldown-badge">⏳ คูลดาวน์ {combat.skill_cooldown} เทิร์น</span>
              ) : (
                <span className="skill-cost-badge">{state.stats.weapon_skill_cost} MP</span>
              )}
            </button>
          )}

          <button className="button button-outline" onClick={handleDefend}>
            🛡️ ตั้งรับ {state.stats.archetype === 'sword' ? '(ลดดาเมจ 70% + สะท้อน)' : state.stats.archetype === 'daggers' ? '(โอกาสหลบ 50%)' : state.stats.archetype === 'staff' ? '(ลดดาเมจ 60% + ฟื้น 8 MP)' : '(ลดดาเมจ 50%)'}
          </button>

          {potions.map((item) => {
            const entry = catalog.find((cat) => cat.id === item.item_id)
            const isBomb = entry?.effect?.type === 'damage_aoe' || entry?.effect?.type === 'smoke'
            return (
              <button
                className={`button ${isBomb ? 'button-gold' : 'button-outline'} button-with-icon`}
                key={item.item_id}
                onClick={() => handleUseItem(item.item_id)}
              >
                <ItemSprite itemId={item.item_id} className="btn-pixel-icon" />
                {entry?.effect?.type === 'damage_aoe'
                  ? `💣 ปา ${entry.name} (ระเบิดหมู่ 30 ดาเมจ!) ×${item.quantity}`
                  : entry?.effect?.type === 'smoke'
                  ? `💨 ปา ${entry.name} (หนีทันที 100%) ×${item.quantity}`
                  : `🧪 ใช้ ${entry?.name || item.item_id} (×${item.quantity})`}
              </button>
            )
          })}

          {combat.tier !== 'boss' && (
            <button className="button button-quiet" onClick={() => onCommand('combat.action', { action: 'ESCAPE' })}>
              🏃 หนี (โอกาส 50%)
            </button>
          )}
        </div>
      )}
    </section>
  )
}

function InventoryPanel({
  state,
  catalog,
  onCommand,
  onClose,
  inCombat,
  onInspectItem,
}: {
  state: PrivateState
  catalog: ItemDefinition[]
  onCommand: (type: string, payload?: object) => void
  onClose?: () => void
  inCombat?: boolean
  onInspectItem?: (itemId: string) => void
}) {
  const itemByID = (id: string) => catalog.find((item) => item.id === id)
  const equipped: { slot: string; label: string; itemID: string; index?: number }[] = [
    { slot: 'weapon', label: '⚔️ อาวุธหลัก', itemID: state.equipment.weapon },
    { slot: 'armor', label: '🛡️ เกราะลำตัว', itemID: state.equipment.armor },
    { slot: 'helmet', label: '🪖 หมวกเกราะ', itemID: state.equipment.helmet },
    ...state.equipment.accessories.map((itemID, index) => ({ slot: 'accessory', index, label: `💍 เครื่องประดับ ${index + 1}`, itemID })),
  ]

  const archetypeLabels: Record<string, string> = {
    sword: '🛡️ ดาบ & โล่ (Knight)',
    staff: '🔮 คทาเวทมนตร์ (Mage)',
    daggers: '🗡️ มีดสั้นคู่ (Rogue)',
    bow: '🏹 ธนูยาว (Ranger)',
  }

  return <section className="panel inventory-panel">
    <div className="panel-title">
      <div>
        <span className="eyebrow">INVENTORY & EQUIPMENT · ช่องเก็บของและอุปกรณ์สวมใส่</span>
        <h2>🎒 กระเป๋าสัมภาระและอุปกรณ์</h2>
      </div>
      <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
        <span className="archetype-tag">{archetypeLabels[state.stats.archetype || 'sword'] || 'นักผจญภัย'}</span>
        <div className="gold-badge">◈ {state.gold} Gold</div>
        {onClose && (
          <button type="button" className="button button-outline" onClick={onClose}>
            ✕ ปิดกระเป๋า (ESC)
          </button>
        )}
      </div>
    </div>

    {inCombat && (
      <div className="combat-bag-notice">
        ⚠️ ขณะอยู่ในการต่อสู้ ไม่สามารถถอดหรือเปลี่ยนอุปกรณ์สวมใส่ได้ (สามารถกดใช้ยาหรือระเบิดได้จากปุ่มในหน้าฉากต่อสู้)
      </div>
    )}

    {state.stats.weapon_skill && (
      <div className="active-skill-banner">
        <div>
          <span className="eyebrow">ACTIVE WEAPON SKILL</span>
          <strong>✨ {state.stats.weapon_skill} ({state.stats.weapon_skill_cost} MP) — {state.stats.weapon_skill_target === 'aoe' ? 'โจมตีหมู่ทุกตัว' : 'โจมตีเป้าหมายเดี่ยว'}</strong>
          <p>{state.stats.weapon_skill_desc}</p>
        </div>
      </div>
    )}

    <div className="inventory-section">
      <div className="section-heading">
        <h3>อุปกรณ์ที่สวมใส่อยู่ (Equipped Gear)</h3>
        <span>รวมสแตตปัจจุบัน: HP {state.stats.max_hp} · MP {state.stats.max_mp || 20} · ATK {state.stats.atk} · DEF {state.stats.def} · SPD {state.stats.spd || 5} · (คลิกไอเทมเพื่อดูรายละเอียด)</span>
      </div>
      <div className="equipment-grid">
        {equipped.map((entry) => {
          const item = itemByID(entry.itemID)
          return (
            <article className={`equipment-cell ${item ? 'is-equipped' : ''}`} key={`${entry.slot}-${entry.index ?? 0}`}>
              <span className="slot-title">{entry.label}</span>
              {item ? (
                <div className="equipped-item-row">
                  <ItemSprite
                    itemId={item.id}
                    alt={item.name}
                    className="item-pixel-art-lg item-inspect-trigger"
                    title="คลิกเพื่อดูรายละเอียดสเตตัสและสกิล"
                    onClick={() => onInspectItem && onInspectItem(item.id)}
                  />
                  <div>
                    <strong style={{ cursor: 'pointer' }} onClick={() => onInspectItem && onInspectItem(item.id)} title="คลิกเพื่อดูรายละเอียด">
                      {item.name} 🔍
                    </strong>
                    <small className="equip-stats-text">
                      {item.stats.attack ? `ATK +${item.stats.attack} ` : ''}
                      {item.stats.defense ? `DEF +${item.stats.defense} ` : ''}
                      {item.stats.max_hp ? `HP +${item.stats.max_hp} ` : ''}
                      {item.stats.max_mp ? `MP +${item.stats.max_mp} ` : ''}
                      {item.stats.speed ? `SPD ${item.stats.speed > 0 ? `+${item.stats.speed}` : item.stats.speed}` : ''}
                    </small>
                    {item.skill_name && <small style={{ color: '#c49bff' }}>✨ สกิล: {item.skill_name}</small>}
                  </div>
                </div>
              ) : <strong className="empty-slot-label">— ยังไม่ได้สวมใส่ —</strong>}
              {item && !inCombat && (
                <div style={{ display: 'flex', gap: '6px', marginTop: '6px' }}>
                  <button
                    type="button"
                    className="button button-quiet compact"
                    onClick={() => onInspectItem && onInspectItem(item.id)}
                  >
                    🔍 สเตตัส
                  </button>
                  <button
                    type="button"
                    className="button button-outline compact"
                    onClick={() => onCommand('inventory.unequip', { slot: entry.slot, ...(entry.index === undefined ? {} : { index: entry.index }) })}
                  >
                    ถอดออก
                  </button>
                </div>
              )}
            </article>
          )
        })}
      </div>
    </div>

    <div className="inventory-section">
      <div className="section-heading">
        <h3>ช่องเก็บของในกระเป๋า (Backpack)</h3>
        <span>ใช้ไปแล้ว {state.inventory.length} / 12 ช่อง · คลิกไอเทมเพื่อดูรายละเอียด หรือกด "สวมใส่" เพื่อเปลี่ยนสายทันที</span>
      </div>
      {state.inventory.length ? (
        <div className="backpack-grid">
          {state.inventory.map((stack) => {
            const item = itemByID(stack.item_id)
            const canHealHP = item?.type === 'consumable' && item.effect?.type === 'heal' && state.stats.hp < state.stats.max_hp
            const canHealMP = item?.type === 'consumable' && item.effect?.type === 'heal_mp' && (state.stats.mp || 0) < (state.stats.max_mp || 20)
            const canUse = !inCombat && (canHealHP || canHealMP)
            return (
              <article className="item-cell" key={stack.item_id}>
                <div className={`rarity-mark rarity-${item?.rarity || 'common'}`} />
                <ItemSprite
                  itemId={stack.item_id}
                  alt={item?.name || stack.item_id}
                  className="item-pixel-art-lg item-inspect-trigger"
                  title="คลิกเพื่อดูรายละเอียดสเตตัสและสกิล"
                  onClick={() => onInspectItem && onInspectItem(stack.item_id)}
                />
                <div className="item-copy">
                  <strong style={{ cursor: 'pointer' }} onClick={() => onInspectItem && onInspectItem(stack.item_id)} title="คลิกเพื่อดูรายละเอียด">
                    {item?.name || stack.item_id}{stack.quantity > 1 && <small> ×{stack.quantity}</small>} 🔍
                  </strong>
                  <span>{item?.description || 'ไอเทมสำหรับผจญภัย'}</span>
                  {item?.skill_name && <span style={{ color: '#c49bff', fontWeight: 600 }}>✨ มอบสกิล: {item.skill_name} ({item.skill_cost} MP)</span>}
                  {ITEM_LORE[stack.item_id] && <span className="item-lore-quote">{ITEM_LORE[stack.item_id]}</span>}
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  <button
                    type="button"
                    className="button button-quiet compact"
                    onClick={() => onInspectItem && onInspectItem(stack.item_id)}
                    title="ดูรายละเอียดสเตตัส สกิล และตำนานไอเทม"
                  >
                    🔍 ข้อมูล
                  </button>
                  {item && item.type !== 'consumable' && (
                    <button
                      className="button button-primary compact"
                      disabled={inCombat}
                      onClick={() => onCommand('inventory.equip', { item_id: item.id, slot: item.slot })}
                    >
                      สวมใส่
                    </button>
                  )}
                  {item?.type === 'consumable' && (
                    <button
                      className="button button-outline compact"
                      disabled={!canUse}
                      onClick={() => onCommand('inventory.use', { item_id: item.id })}
                    >
                      {canUse ? 'กดใช้' : item.effect?.type === 'damage_aoe' || item.effect?.type === 'smoke' ? 'ใช้ในฉากต่อสู้' : 'HP/MP เต็ม'}
                    </button>
                  )}
                </div>
              </article>
            )
          })}
        </div>
      ) : <div className="empty-inventory">กระเป๋า (Backpack) ยังว่างอยู่</div>}
    </div>
  </section>
}

function DungeonMap({
  state,
  onMove,
  onOpenBag,
  bagCount,
}: {
  state: DungeonView
  onMove: (nodeID: string) => void
  onOpenBag: () => void
  bagCount: number
}) {
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null)
  const floor = state.floors.find((entry) => entry.number === state.current_floor)
  if (!floor) return null

  const roomNames: Record<string, string> = {
    START: 'จุดเริ่มต้น',
    MONSTER: 'มอนสเตอร์',
    ELITE: 'ศัตรูอีลีท',
    TREASURE: 'หีบสมบัติ',
    SHOP: 'ตลาดร้านค้า',
    FORGE: 'ช่างตีเหล็ก',
    EVENT: 'เหตุการณ์',
    REST: 'แคมป์ไฟ',
    PUZZLE: 'ห้องปริศนา',
    BOSS: 'บอสประจำชั้น',
  }

  const roomSubtitles: Record<string, string> = {
    START: 'ทางเข้าชั้นนี้',
    MONSTER: 'สู้รับ Gold & EXP',
    ELITE: 'ทางลัดอันตราย · ดรอป 100%',
    TREASURE: 'รับไอเทม/ทองฟรี',
    SHOP: 'ซื้อ-ขายอุปกรณ์ & ยา',
    FORGE: 'ตีบวก ATK/DEF ถาวร',
    EVENT: 'เสี่ยงโชค & ฟื้นฟู',
    REST: 'ฟื้นฟู +35% HP & +15 MP',
    PUZZLE: 'ไขรูนรับ EXP & ของดี',
    BOSS: 'โค่นบอสลงชั้นถัดไป',
  }

  const depths = Array.from(new Set(floor.nodes.map((node) => node.depth))).sort((a, b) => a - b)
  const maxDepth = depths[depths.length - 1] || 6

  // Compute deterministic (x, y) coordinates on a wide canvas for Circular Nodes + Shortcut Arcs
  const COL_WIDTH = 225
  const CANVAS_HEIGHT = 580
  const CANVAS_WIDTH = Math.max(1380, depths.length * COL_WIDTH)

  const nodeCoords = useMemo(() => {
    const coords: Record<string, { x: number; y: number; node: DungeonNode }> = {}
    depths.forEach((depth, colIdx) => {
      const nodesInDepth = floor.nodes.filter((n) => n.depth === depth)
      const count = nodesInDepth.length
      const x = colIdx * COL_WIDTH + COL_WIDTH / 2
      nodesInDepth.forEach((node, rowIdx) => {
        const usableHeight = CANVAS_HEIGHT - 140
        const y = 85 + ((rowIdx + 1) / (count + 1)) * usableHeight
        coords[node.id] = { x, y, node }
      })
    })
    return coords
  }, [floor.nodes, depths])

  const currentNode = floor.nodes.find((n) => n.id === state.current_node_id)

  return <section className="panel dungeon-map-panel">
    <div className="panel-title">
      <div>
        <span className="eyebrow">ASYMMETRIC NODE MAP · ชั้นที่ {floor.number} จาก 3 · เลือกสายสั้นบุกเร็ว หรือ สายฟาร์มยาว</span>
        <h2>🗺️ แผนที่โหนดวงกลม — {FLOOR_LORE[floor.number]?.title || `ชั้นที่ ${floor.number}`}</h2>
      </div>
      <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
        <button type="button" className="button button-outline" onClick={onOpenBag}>
          🎒 เปิดกระเป๋า ({bagCount}/12)
        </button>
        <span className="map-difficulty">ระดับความยาก ×{floor.difficulty_multiplier}</span>
      </div>
    </div>

    <div className="map-route-guide-strip">
      <div className="route-lane-pill rush-lane">
        <strong>⚡ สายบน (ทางลัดบุกเร็ว 3 จุดถึงบอส):</strong>
        <span>ผ่านห้องน้อยกว่า แต่ต้องดวลกับมอนสเตอร์ระดับ <b>ELITE</b> (ดรอปไอเทม 100%)</span>
      </div>
      <div className="route-lane-pill balanced-lane">
        <strong>⚖️ สายกลาง (สมดุล 5 จุดถึงบอส):</strong>
        <span>เก็บเลเวล ปริศนาอักษรรูน และเหตุการณ์พิเศษ</span>
      </div>
      <div className="route-lane-pill farm-lane">
        <strong>🌾 สายล่าง (ฟาร์มยาว 5 จุด · ตลาด & ช่างตีเหล็ก):</strong>
        <span>แวะ <b>🛒 ตลาดร้านค้า</b>, <b>⚒️ โรงตีเหล็ก</b>, <b>🎁 สมบัติ</b> และ <b>⛺ แคมป์ไฟ</b></span>
      </div>
    </div>

    <div className="map-legend-bar">
      <span>📍 <strong>ตำแหน่งของคุณ:</strong> ก้าวที่ {state.progress} / {maxDepth}</span>
      <div className="map-legend-items">
        <span className="legend-dot available-dot" /> เส้นสีทอง = โหนดถัดไปที่เลือกเดินได้
        <span className="legend-dot shortcut-dot" /> เส้นโค้งสีส้ม = ทางลัดข้ามก้าว (Rush Shortcut)
        <span className="legend-dot cleared-dot" /> เส้นสีเขียว = เส้นทางที่ผ่านแล้ว
      </div>
    </div>

    <div className="map-graph-scroll">
      <div className="map-graph-canvas" style={{ width: `${CANVAS_WIDTH}px`, height: `${CANVAS_HEIGHT}px` }}>
        {/* Column Headers */}
        {depths.map((depth, colIdx) => (
          <div
            key={`header-${depth}`}
            className={`map-col-header ${depth === state.progress ? 'is-current-depth' : ''}`}
            style={{ left: `${colIdx * COL_WIDTH}px`, width: `${COL_WIDTH}px` }}
          >
            {depth === 0 ? '🚪 จุดเริ่มต้น' : depth === maxDepth ? '👑 บอสประจำชั้น' : `จุดที่ ${depth}`}
          </div>
        ))}

        {/* SVG Connection Paths Layer */}
        <svg className="map-connections-svg" viewBox={`0 0 ${CANVAS_WIDTH} ${CANVAS_HEIGHT}`} preserveAspectRatio="none">
          <defs>
            <linearGradient id="activeRouteGrad" x1="0%" y1="0%" x2="100%" y2="0%">
              <stop offset="0%" stopColor="#ffd166" />
              <stop offset="100%" stopColor="#ff9f43" />
            </linearGradient>
            <linearGradient id="shortcutRouteGrad" x1="0%" y1="0%" x2="100%" y2="0%">
              <stop offset="0%" stopColor="#ff6b6b" />
              <stop offset="100%" stopColor="#ffa502" />
            </linearGradient>
            <linearGradient id="clearedRouteGrad" x1="0%" y1="0%" x2="100%" y2="0%">
              <stop offset="0%" stopColor="#2ed573" />
              <stop offset="100%" stopColor="#7bed9f" />
            </linearGradient>
          </defs>
          {floor.nodes.flatMap((fromNode) => {
            const fromPos = nodeCoords[fromNode.id]
            const connections = fromNode.connections || []
            if (!fromPos || !connections.length) return []
            return connections.map((targetId) => {
              const toPos = nodeCoords[targetId]
              if (!toPos) return null
              const toNode = toPos.node

              const isShortcut = toNode.depth - fromNode.depth > 1
              const isNextAvailable = fromNode.id === state.current_node_id && toNode.status === 'AVAILABLE'
              const isClearedEdge =
                (fromNode.status === 'CLEARED' || fromNode.status === 'SKIPPED') &&
                (toNode.status === 'CLEARED' || toNode.status === 'SKIPPED' || toNode.status === 'IN_PROGRESS')
              const isHoveredEdge = hoveredNodeId === fromNode.id || hoveredNodeId === toNode.id
              const isPastUnchosen = fromNode.depth <= state.progress && !isNextAvailable && !isClearedEdge

              const x1 = fromPos.x + 38
              const y1 = fromPos.y
              const x2 = toPos.x - 38
              const y2 = toPos.y
              const cx1 = x1 + (x2 - x1) * 0.42
              const cx2 = x1 + (x2 - x1) * 0.58
              const arcLift = isShortcut ? -52 : 0
              const d = `M ${x1} ${y1} C ${cx1} ${y1 + arcLift}, ${cx2} ${y2 + arcLift}, ${x2} ${y2}`

              let edgeClass = isShortcut ? 'map-edge-shortcut' : 'map-edge-future'
              if (isNextAvailable) edgeClass = 'map-edge-available'
              else if (isClearedEdge) edgeClass = 'map-edge-cleared'
              else if (isPastUnchosen) edgeClass = 'map-edge-locked'
              if (isHoveredEdge) edgeClass += ' is-hovered'

              return (
                <g key={`${fromNode.id}->${targetId}`}>
                  {isNextAvailable && <path d={d} className="map-edge-glow" />}
                  <path d={d} className={`map-edge ${edgeClass}`} />
                  <circle cx={x2 - 3} cy={y2} r={isNextAvailable ? 5.5 : 3.5} className={`map-edge-dot ${edgeClass}`} />
                </g>
              )
            })
          })}
        </svg>

        {/* Circular Interactive Nodes (Slay the Spire / Roguelite Node Style) */}
        {floor.nodes.map((node) => {
          const pos = nodeCoords[node.id]
          if (!pos) return null
          const isCurrent = node.id === state.current_node_id
          const isAvailable = node.status === 'AVAILABLE'
          const isCleared = node.status === 'CLEARED' || node.status === 'SKIPPED'
          const isPastLocked = node.depth <= state.progress && node.status === 'LOCKED'
          const isConnectedFromCurrent = Boolean(currentNode?.connections && currentNode.connections.includes(node.id))
          const isShortcutNode = node.path_lane === 'rush' && (node.depth === 1 || node.depth === 3)
          const isHovered = hoveredNodeId === node.id

          return (
            <div key={node.id} style={{ position: 'static' }}>
              <button
                type="button"
                style={{ left: `${pos.x}px`, top: `${pos.y}px` }}
                className={[
                  'map-circle-node',
                  `status-${node.status.toLowerCase().replace('_', '-')}`,
                  `room-type-${node.type.toLowerCase()}`,
                  isCurrent ? 'is-current-position' : '',
                  isPastLocked ? 'is-past-locked' : '',
                  isConnectedFromCurrent ? 'is-connected-next' : '',
                ].join(' ')}
                disabled={!isAvailable}
                onMouseEnter={() => setHoveredNodeId(node.id)}
                onMouseLeave={() => setHoveredNodeId(null)}
                onClick={() => onMove(node.id)}
              >
                {isCurrent && <span className="node-pin-badge current-pin">📍 คุณอยู่ที่นี่</span>}
                {isAvailable && (
                  <span className="node-pin-badge available-pin">
                    {isShortcutNode ? '⚡ ทางลัดบุกเร็ว!' : '✨ เลือกเดินได้'}
                  </span>
                )}
                {isCleared && !isCurrent && <span className="node-pin-badge cleared-pin">✓ ผ่านแล้ว</span>}

                <div className="circle-node-orb">
                  <RoomIcon type={node.type} />
                </div>
                <div className="circle-node-caption">
                  <strong>{roomNames[node.type] || node.type}</strong>
                </div>
              </button>

              {/* Floating Node Inspection Tooltip Card on Hover */}
              {isHovered && (
                <div
                  className="map-node-tooltip-card"
                  style={{
                    left: `${pos.x}px`,
                    top: `${pos.y - 58}px`,
                  }}
                >
                  <h4>
                    <RoomIcon type={node.type} />
                    {roomNames[node.type] || node.type}
                  </h4>
                  <p>{roomSubtitles[node.type] || ''}</p>
                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                    {isShortcutNode && <span className="tooltip-meta-tag">⚡ ทางลัดบุกเร็ว (3 จุดถึงบอส)</span>}
                    {isCurrent && <span className="tooltip-meta-tag" style={{ color: '#2ed573' }}>📍 คุณอยู่ที่นี่</span>}
                    {isAvailable && !isCurrent && <span className="tooltip-meta-tag" style={{ color: '#ffd166' }}>✨ โหนดที่เลือกเดินได้</span>}
                    {isCleared && !isCurrent && <span className="tooltip-meta-tag" style={{ color: '#7bed9f' }}>✓ เคลียร์แล้ว</span>}
                    {isPastLocked && <span className="tooltip-meta-tag" style={{ color: '#a4b0be' }}>🔒 ข้ามแล้ว</span>}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>

    <div className="map-footer">
      <div>
        <strong>ชั้นที่ {floor.number} / 3</strong> · ความคืบหน้า {state.progress} / {maxDepth} ก้าว
        {currentNode && <span> · ตำแหน่งล่าสุด: <b>{roomNames[currentNode.type] || currentNode.type}</b> (มีเส้นทางไปต่อ {(currentNode.connections || []).length} เส้นทาง)</span>}
      </div>
      <div className="other-floors">
        {[1, 2, 3].map((fNum) => (
          <span
            key={fNum}
            className={fNum === state.current_floor ? 'floor-pill-active' : fNum < state.current_floor ? 'floor-pill-cleared' : ''}
          >
            {fNum < state.current_floor ? `✓ ชั้น ${fNum}` : fNum === state.current_floor ? `📍 ชั้น ${fNum} (ปัจจุบัน)` : `🔒 ชั้น ${fNum}`}
          </span>
        ))}
      </div>
    </div>
  </section>
}

function RoomIcon({ type, className }: { type: string; className?: string }) {
  const icon: Record<string, string> = {
    START: 'room-start',
    MONSTER: 'room-monster',
    ELITE: 'room-elite',
    TREASURE: 'room-treasure',
    SHOP: 'room-shop',
    FORGE: 'room-elite',
    EVENT: 'room-event',
    PUZZLE: 'room-event',
    REST: 'room-rest',
    BOSS: 'room-boss',
  }
  return (
    <svg className={className || 'room-icon'} viewBox="0 0 48 48" aria-hidden="true" focusable="false">
      <use href={`/art/room-icons.svg#${icon[type] || 'room-monster'}`} />
    </svg>
  )
}

class ErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean; error: Error | null }> {
  constructor(props: { children: ReactNode }) {
    super(props)
    this.state = { hasError: false, error: null }
  }
  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error }
  }
  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Dungeon Rivals Caught UI Error:', error, errorInfo)
  }
  render() {
    if (this.state.hasError) {
      return (
        <main className="shell landing-shell">
          <div className="error-fallback-panel">
            <h2>⚠️ เกิดข้อผิดพลาดในการแสดงผลหน้าจอเกม</h2>
            <p>ระบบตรวจพบข้อผิดพลาดที่ไม่คาดคิด: {this.state.error?.message || 'ข้อผิดพลาดไม่ทราบสาเหตุ'}</p>
            <div style={{ display: 'flex', gap: '12px' }}>
              <button className="button button-gold" onClick={() => window.location.reload()}>
                🔄 รีโหลดหน้าเว็บ
              </button>
              <button
                className="button button-quiet"
                onClick={() => {
                  sessionStorage.removeItem(SESSION_KEY)
                  window.location.reload()
                }}
              >
                🚪 ออกสู่หน้าหลัก
              </button>
            </div>
          </div>
        </main>
      )
    }
    return this.props.children
  }
}

export default function App() {
  return (
    <ErrorBoundary>
      <AppContent />
    </ErrorBoundary>
  )
}
