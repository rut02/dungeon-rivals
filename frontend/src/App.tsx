import { FormEvent, useEffect, useMemo, useState } from 'react'
import './inventory.css'
import './map.css'
import './combat-fx.css'
import { sound } from './sound'

type Session = { room_code: string; player_id: string; reconnect_token: string; is_host: boolean; ws_path: string }
type Player = { player_id: string; name: string; is_host: boolean; ready: boolean; connected: boolean; kit?: string }

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

type DungeonNode = { id: string; floor: number; depth: number; type: string; status: string; connections: string[] }
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
  log: string[]
  enemies?: EnemyView[]
  selected_enemy?: number
  turn?: number
  first_strike?: boolean
}

type ActivityView = {
  type: string
  options?: { id: string; label: string; description: string; kind: string; item_id?: string; value?: number }[]
  stock?: { item_id: string; name: string; price: number; available: boolean }[]
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
  pvp_actions_used?: number
  pvp_actions_remaining?: number
  pvp_score?: number
  pvp_damage?: number
  pvp_knockouts?: number
}

type PublicPlayer = Player & {
  stage: string
  floor: number
  progress: number
  score: number
  completed: boolean
  hp: number
  max_hp: number
  pvp_actions_remaining: number
  pvp_score: number
  pvp_damage: number
  pvp_knockouts: number
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
  NO_ACTIVE_COMBAT: 'ไม่ได้อยู่ในการต่อสู้',
  NO_ENCOUNTER_ROOM: 'ห้องนี้ไม่มีกิจกรรมหรือศัตรูให้เริ่มต้น',
  NOT_ENOUGH_MANA: 'มานา (MP) ไม่เพียงพอสำหรับการใช้สกิลนี้!',
  TARGET_NOT_FOUND: 'ไม่พบศัตรูเป้าหมาย กรุณาเลือกศัตรูที่ยังมีชีวิตอยู่',
  TARGET_ALREADY_DEAD: 'ศัตรูตัวนี้ล้มไปแล้ว กรุณาเลือกเป้าหมายอื่น',
  CONSUMABLE_ONLY: 'สามารถใช้งานได้เฉพาะไอเทมประเภทกดใช้เท่านั้น',
  ROOM_NOT_FOUND: 'ไม่พบรหัสห้องนี้ในระบบ',
  PLAYER_NOT_FOUND: 'ไม่พบข้อมูลผู้เล่นในห้อง',
  INVALID_RECONNECT_TOKEN: 'โทเค็นการเชื่อมต่อไม่ถูกต้อง กรุณาเข้าห้องใหม่อีกครั้ง',
  INVENTORY_FULL: 'ช่องเก็บของ (Backpack) เต็มแล้ว (สูงสุด 12 ช่อง)',
  NOT_ENOUGH_GOLD: 'Gold ไม่เพียงพอสำหรับซื้อไอเทมนี้',
  ITEM_NOT_FOUND: 'ไม่พบไอเทมนี้ในคลังเก็บของ',
  SLOT_MISMATCH: 'ไม่สามารถสวมใส่ไอเทมในช่องนี้ได้',
  NODE_NOT_FOUND: 'ไม่พบเส้นทางที่เลือกบนแผนที่',
  NODE_UNAVAILABLE: 'เส้นทางนี้ยังไม่เปิดให้สำรวจ',
  ENCOUNTER_ACTIVE: 'คุณต้องจบกิจกรรมหรือการต่อสู้ในห้องปัจจุบันก่อนเดินทางต่อ',
  ROOM_FULL: 'ห้องนี้มีผู้เล่นเต็มแล้ว',
  HOST_ONLY: 'เฉพาะเจ้าของห้องเท่านั้นที่มีสิทธิ์ดำเนินการนี้',
  NOT_ALL_READY: 'ผู้เล่นทุกคนยังไม่พร้อม กรุณารอให้ทุกคนกดพร้อมก่อน',
  ALREADY_READY: 'คุณอยู่ในสถานะพร้อมแล้ว',
}

const STARTER_KITS = [
  {
    id: 'knight',
    name: 'อัศวิน (Knight)',
    archetype: 'ดาบ & โล่ (Sword & Shield)',
    icon: '🛡️',
    description: 'สมดุล แข็งแกร่ง ตั้งรับลดดาเมจ 70% พร้อมสะท้อนดาเมจใส่ศัตรู',
    perk: 'สกิล Shield Bash (MP 12): สตันและชะลอความเร็วของศัตรู',
    stats: 'HP 110 · DEF สูง · ป้องกันแน่นหนา',
    gear: 'Iron Sword + Shield + ยาฟื้นฟู',
  },
  {
    id: 'mage',
    name: 'นักเวท (Mage)',
    archetype: 'คทาเวทมนตร์ (Staff)',
    icon: '🔮',
    description: 'มานาล้นหลาม สกิลเวทกวาดมอนสเตอร์หลายตัวพร้อมกัน',
    perk: 'สกิล Arcane Nova (MP 15): ระเบิดเวท AoE โจมตีศัตรูทุกตัวในฉาก',
    stats: 'MP 50 · เวทมนตร์รุนแรง · ตั้งรับสร้างเกราะ + ฟื้น MP',
    gear: 'Wooden Staff + แหวนศิษย์ + ยามานา',
  },
  {
    id: 'rogue',
    name: 'โร้ก (Rogue)',
    archetype: 'มีดคู่ (Dual Daggers)',
    icon: '🗡️',
    description: 'ว่องไว ชิงจังหวะโจมตีก่อนเสมอ คริติคอลแรง และหลบหลีก 50%',
    perk: 'สกิล Shadow Strike (MP 10): แทงจุดตาย คริติคอลรุนแรง 2.5 เท่า',
    stats: 'SPD 14 · ชิงลงมือก่อน · ตั้งรับมีโอกาส Dodge 50%',
    gear: 'Iron Dagger + รองเท้าเร่งความเร็ว + ระเบิดควัน',
  },
  {
    id: 'ranger',
    name: 'เรนเจอร์ (Ranger)',
    archetype: 'ธนูยาว (Longbow)',
    icon: '🏹',
    description: 'โจมตีระยะไกล เปิดฉากยิงก่อนเสมอในเทิร์นแรก (First Strike)',
    perk: 'สกิล Piercing Shot (MP 10): ยิงเจาะทะลุพลังป้องกัน (Ignore DEF)',
    stats: 'First Strike เทิร์นแรกเสมอ · ยิงเจาะเกราะบอส',
    gear: 'Hunting Bow + ซองธนู + ระเบิดเพลิง',
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

export default function App() {
  const [session, setSession] = useState<Session | null>(readSession)
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [itemCatalog, setItemCatalog] = useState<ItemDefinition[]>([])
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')
  const [name, setName] = useState('')
  const [joinCode, setJoinCode] = useState('')
  const [pending, setPending] = useState(false)
  const [showLore, setShowLore] = useState(false)

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

  if (session && !snapshot) {
    return <main className="shell lobby-shell">
      <header className="topbar">
        <div className="brand"><RoomIcon type="START" className="brand-sigil"/><span>DUNGEON <b>RIVALS</b></span></div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <button className="lore-open-btn" onClick={() => setShowLore(true)}>📜 ตำนานสุสาน (Lore)</button>
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
    const allReady = snapshot.players.length >= snapshot.room.min_players && snapshot.players.every((player) => player.ready)
    const currentKit = me.private?.kit || me.kit || 'knight'

    return <main className="shell lobby-shell">
      <header className="topbar">
        <div className="brand"><RoomIcon type="START" className="brand-sigil"/><span>DUNGEON <b>RIVALS</b></span></div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <button className="lore-open-btn" onClick={() => setShowLore(true)}>📜 ตำนานสุสาน (Lore)</button>
          <div className="status"><span className={connected ? 'pulse online' : 'pulse'} />{connected ? 'เชื่อมต่อแล้ว' : 'กำลังเชื่อมต่อ'}</div>
        </div>
      </header>
      <section className="lobby-head">
        <div>
          <p className="eyebrow">ห้องรอผู้เล่น · ASHEN REPUTATION</p>
          <h1>เตรียมทีมให้พร้อม</h1>
          <p className="muted">เลือกสายอุปกรณ์เริ่มต้นและวางกลยุทธ์ร่วมกัน แต่ละคนจะสำรวจเส้นทางดันเจียนของตนเอง</p>
        </div>
        <div className="room-code"><span>ROOM CODE</span><strong>{session.room_code}</strong></div>
      </section>
      
      <section className="panel roster-panel">
        <div className="panel-title"><div><span className="eyebrow">ADVENTURERS</span><h2>สมาชิกในห้อง</h2></div><span className="count">{snapshot.players.length}<i> / {snapshot.room.max_players}</i></span></div>
        <div className="roster">
          {[...snapshot.players, ...Array.from({ length: snapshot.room.max_players - snapshot.players.length }, () => null)].map((player, i) => player ? (
            <article className="player-card" key={player.player_id}>
              <div className={`avatar avatar-${i % 4}`}><img src="/art/pixel/hero.png" alt="" className="roster-hero-art"/></div>
              <div className="player-info">
                <strong>{player.name}{player.player_id === session.player_id && <small>คุณ</small>}</strong>
                <span>{player.is_host ? 'เจ้าของห้อง' : 'นักผจญภัย'} · {player.connected ? 'ออนไลน์' : 'ออฟไลน์'}{player.floor ? ` · ชั้น ${player.floor} / ก้าวที่ ${player.progress}` : ''}</span>
              </div>
              <div className={`ready-tag ${player.ready ? 'is-ready' : ''}`}>{player.ready ? 'พร้อม' : player.stage === 'exploring' ? 'สำรวจอยู่' : 'รออยู่'}</div>
            </article>
          ) : (
            <article className="player-card empty-card" key={`empty-${i}`}><div className="avatar empty-avatar">＋</div><div className="player-info"><strong>รอผู้เล่นเข้าร่วม</strong><span>แชร์รหัสห้องเพื่อชวนเพื่อน</span></div><span className="empty-dot" /></article>
          ))}
        </div>

        {snapshot.room.phase === 'lobby' && (
          <>
            <div className="kit-selector-wrap">
              <div className="kit-selector-title">
                <h3>เลือกสายอุปกรณ์เริ่มต้น (Starter Item Kit)</h3>
                <span>ไอเทมมอบสแตตและสกิล สามารถเปลี่ยนสายได้อิสระเมื่อพบอุปกรณ์ใหม่ในดันเจียน</span>
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
                    <span className="kit-icon">{kit.icon}</span>
                    <strong className="kit-name">{kit.name}</strong>
                    <span className="kit-archetype">{kit.archetype}</span>
                    <p className="kit-perk">{kit.perk}</p>
                    <span className="kit-stats-tag">{kit.stats}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="lobby-actions">
              <button className={`button ${me?.ready ? 'button-muted' : 'button-primary'}`} onClick={() => command('lobby.ready', { ready: !me?.ready })}>
                {me?.ready ? 'ยกเลิกความพร้อม' : 'พร้อมแล้ว'}
              </button>
              {session.is_host && (
                <button className="button button-gold" disabled={!allReady} onClick={() => command('game.start')}>
                  เริ่มเกม <span>→</span>
                </button>
              )}
              <button className="button button-quiet" onClick={leave}>ออกจากห้อง</button>
            </div>
            <p className="hint">{host?.name || 'เจ้าของห้อง'} จะเริ่มเกมได้เมื่อมีผู้เล่นอย่างน้อย 2 คนและทุกคนพร้อม</p>
          </>
        )}
      </section>

      {snapshot.me.private?.dungeon && <DungeonMap state={snapshot.me.private.dungeon} onMove={(nodeID) => command('dungeon.move', { node_id: nodeID })} />}
      {snapshot.me.private?.combat && <CombatPanel state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />}
      {snapshot.me.private?.activity && <RoomActivityPanel state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />}
      {((snapshot.room.phase === 'dungeon' && snapshot.me.private?.dungeon?.completed) || snapshot.room.phase === 'preparation') && (
        <PreparationPanel players={snapshot.players} isReady={me.ready} phase={snapshot.room.phase} isHost={me.is_host} onReady={(ready) => command('preparation.ready', { ready })} onKick={(playerID) => command('room.kick_disconnected', { target_player_id: playerID })} />
      )}
      {snapshot.room.phase === 'pvp' && snapshot.me.private && (
        <ArenaPanel players={snapshot.players} me={me} isHost={me.is_host} state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />
      )}
      {snapshot.room.phase === 'results' && snapshot.results && (
        <ResultsPanel roomCode={session.room_code} results={snapshot.results} onReplay={() => command('room.replay')} onLeave={leave} />
      )}
      {snapshot.me.private && !['pvp', 'results'].includes(snapshot.room.phase) && (
        <InventoryPanel state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />
      )}
      {snapshot.room.phase !== 'lobby' && (
        <div className="notice">
          {snapshot.room.phase === 'pvp' ? 'ทุกคนพร้อมแล้ว · เข้าสู่สนามประลอง PvP ชิงความเป็นหนึ่ง' : snapshot.room.phase === 'preparation' ? 'ช่วงเตรียมตัว · ตรวจสอบ HP และอุปกรณ์ จากนั้นกดพร้อมเพื่อเข้าสู่ PvP' : 'เกมกำลังดำเนินอยู่ · ตัวละครและช่องเก็บของถูกควบคุมโดย Server อย่างโปร่งใส'}
        </div>
      )}
      {error && (
        <div className="error-toast">
          <span>{error}</span>
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
        <p className="hero-copy">รวมทีมกับเพื่อน ก้าวลงสู่สุสาน 3 ชั้น ล่ามอนสเตอร์และไขปริศนาโบราณ<br/>สะสมอาวุธระดับตำนาน ก่อนประลองฝีมือชิงศิลาราชันย์มังกรในสนามประลอง</p>
        <button className="button button-quiet" style={{ margin: '8px 0 16px', display: 'inline-flex', alignItems: 'center', gap: '6px' }} onClick={() => setShowLore(true)}>
          📜 อ่านพงศาวดารแห่งสุสานเถ้าธุลี (Story Lore) →
        </button>
        <figure className="hero-art"><img src="/art/dungeon-gate.svg" alt="ประตูหินโบราณส่องแสงสีเขียวอมฟ้าอยู่ในถ้ำ"/><figcaption>THE GATE OF ASHEN CATACOMBS · ประตูสู่สุสานเถ้าธุลีโบราณ</figcaption></figure>
        <div className="hero-stats"><span><b>02–04</b><small>ผู้เล่นต่อห้อง</small></span><i /><span><b>CO-OP</b><small>สำรวจพร้อมกัน</small></span><i /><span><b>PVP</b><small>ชิงชัยช่วงท้าย</small></span></div>
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
        <p className="privacy-note">เกมทดลอง · ความคืบหน้าเก็บไว้ชั่วคราวระหว่างที่เซิร์ฟเวอร์ทำงาน</p>
      </section>
    </div>
    {showLore && <LoreModal onClose={() => setShowLore(false)} />}
    <footer>สร้างปาร์ตี้ · สำรวจดันเจียน · ชิงชัยในอารีนา</footer>
  </main>
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

function PreparationPanel({ players, isReady, phase, isHost, onReady, onKick }: { players: PublicPlayer[]; isReady: boolean; phase: string; isHost: boolean; onReady: (ready: boolean) => void; onKick: (playerID: string) => void }) {
  const allFinished = players.every((player) => player.completed)
  return <section className="panel preparation-panel">
    <div className="panel-title"><div><span className="eyebrow">FINAL CHECK · PREPARATION</span><h2>{phase === 'pvp' ? 'พร้อมสู้ใน Arena' : 'เตรียมตัวก่อน PvP'}</h2></div><span className={`ready-tag ${isReady ? 'is-ready' : ''}`}>{isReady ? 'พร้อมแล้ว' : 'ยังไม่พร้อม'}</span></div>
    <p className="muted">{allFinished ? 'ทุกคนจบดันเจียนแล้ว ตรวจ HP และอุปกรณ์ของคุณ จากนั้นกดพร้อม' : 'คุณจบดันเจียนแล้ว รอเพื่อนร่วมทีมเล่นต่อได้เลย'}</p>
    <div className="prep-roster">{players.map((player) => <span key={player.player_id} className={player.completed ? 'prep-complete' : ''}>{player.name} · {player.completed ? 'จบแล้ว' : player.connected ? 'กำลังเล่น' : 'หลุดการเชื่อมต่อ'}{isHost && !player.connected && <button className="text-button" onClick={() => onKick(player.player_id)}>นำออก</button>}</span>)}</div>
    {phase !== 'pvp' && <button className={`button ${isReady ? 'button-muted' : 'button-gold'}`} disabled={phase === 'preparation' && !allFinished} onClick={() => onReady(!isReady)}>{isReady ? 'ยกเลิกความพร้อม' : 'พร้อมเข้าสู่ PvP'}</button>}
  </section>
}

function ArenaPanel({ players, me, isHost, state, catalog, onCommand }: { players: PublicPlayer[]; me: Player; isHost: boolean; state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const left = state.pvp_actions_remaining ?? 0
  const potions = state.inventory.filter((stack) => catalog.find((item) => item.id === stack.item_id)?.type === 'consumable' && catalog.find((item) => item.id === stack.item_id)?.usable_in_combat)
  return <section className="panel arena-panel">
    <div className="panel-title"><div><span className="eyebrow">FINAL PVP · SERVER RESOLVED</span><h2>สนามประลอง</h2></div><span className="gold-badge">เหลือ {left} Action</span></div>
    <div className="arena-roster">
      {players.filter((player) => player.player_id !== me.player_id).map((player) => (
        <article className="arena-opponent" key={player.player_id}>
          <div className="arena-opponent-header">
            <img src="/art/pixel/hero.png" alt="" className="arena-hero-art" />
            <div>
              <strong>{player.name}</strong>
              <small>HP {player.hp} / {player.max_hp} · PvP Score {player.pvp_score}</small>
            </div>
          </div>
          <div className="arena-health"><i style={{ width: `${player.max_hp ? player.hp / player.max_hp * 100 : 0}%` }} /></div>
          <button className="button button-primary" disabled={!left} onClick={() => onCommand('pvp.action', { action: 'ATTACK', target_player_id: player.player_id })}>โจมตี · ใช้ 1 Action</button>
        </article>
      ))}
    </div>
    <div className="arena-actions">
      <button className="button button-outline" disabled={!left} onClick={() => onCommand('pvp.action', { action: 'DEFEND' })}>ตั้งรับ</button>
      {potions.map((item) => (
        <button className="button button-outline button-with-icon" disabled={!left} key={item.item_id} onClick={() => onCommand('pvp.action', { action: 'USE_ITEM', item_id: item.item_id })}>
          <img src={`/art/pixel/${item.item_id}.png`} alt="" className="btn-pixel-icon"/>
          ใช้ {catalog.find((entry) => entry.id === item.item_id)?.name}
        </button>
      ))}
      <span>HP {state.stats.hp}/{state.stats.max_hp} · MP {state.stats.mp || 0}/{state.stats.max_mp || 20} · DMG {state.pvp_damage || 0} · KO {state.pvp_knockouts || 0}</span>
    </div>
    {left === 0 && <p className="muted">ใช้ Action ครบแล้ว กำลังรอผู้เล่นคนอื่น</p>}
    {isHost && players.some((player) => !player.connected) && <button className="button button-quiet activity-leave" onClick={() => onCommand('pvp.force_finish')}>จบ PvP สำหรับผู้เล่นที่หลุด</button>}
  </section>
}

function ResultsPanel({ roomCode, results, onReplay, onLeave }: { roomCode: string; results: ResultEntry[]; onReplay: () => void; onLeave: () => void }) {
  return <section className="panel results-panel">
    <div className="panel-title">
      <div><span className="eyebrow">MATCH COMPLETE</span><h2>ผลการแข่งขัน</h2></div>
      <div style={{ display: 'flex', gap: '8px' }}>
        <button className="button button-gold" onClick={onReplay}>🔄 เล่นใหม่อีกรอบ</button>
        <a className="button button-outline" href={`/api/v1/rooms/${encodeURIComponent(roomCode)}/results.csv`} download>ดาวน์โหลด CSV</a>
        <button className="button button-quiet" onClick={onLeave}>ออกจากห้อง</button>
      </div>
    </div>
    <div className="results-list">
      {results.map((result) => (
        <article className={`result-card ${result.rank === 1 ? 'winner' : ''}`} key={result.player_id}>
          <div className="result-rank">#{result.rank}</div>
          <div className="result-player">
            <strong>{result.name}</strong>
            <small>PvE {result.pve_score} · PvP {result.pvp_score} · Gold {result.gold_score} · Bonus +{result.bonus} · Penalty −{result.penalties}</small>
            <small>Damage {result.pvp_damage} · KO {result.pvp_knockouts} · Deaths {result.pvp_deaths} · Boss {result.bosses_defeated}</small>
          </div>
          <strong className="result-total">{result.total_score}</strong>
        </article>
      ))}
    </div>
  </section>
}

function RoomActivityPanel({ state, catalog, onCommand }: { state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const activity = state.activity!
  const roomLabel: Record<string, string> = {
    TREASURE: 'ห้องสมบัติ (Treasure Room)',
    SHOP: 'ร้านค้านักเดินทาง (Merchant Shop)',
    EVENT: 'เหตุการณ์พิเศษ (Special Event)',
    PUZZLE: 'ห้องปริศนาโบราณ (Puzzle Chamber)',
  }
  return <section className="panel activity-panel">
    <div className="panel-title">
      <div><span className="eyebrow">ROOM · {activity.type}</span><h2>{roomLabel[activity.type] || activity.type}</h2></div>
      {activity.type === 'SHOP' && <span className="gold-badge">◈ {state.gold} Gold</span>}
    </div>
    {activity.options && (
      <div className="activity-options">
        {activity.options.map((option) => {
          const item = catalog.find((entry) => entry.id === option.item_id)
          return (
            <article className="activity-option" key={option.id}>
              {item && <img src={`/art/pixel/${item.id}.png`} alt={item.name} className="item-pixel-art" />}
              <div>
                <strong>{option.label}</strong>
                <p>{option.description}</p>
                {item && <small>{item.description}</small>}
                {item && ITEM_LORE[item.id] && <small className="item-lore-quote">{ITEM_LORE[item.id]}</small>}
              </div>
              <button className="button button-primary" onClick={() => onCommand('room.action', { action: 'claim', choice_id: option.id })}>เลือกทางนี้</button>
            </article>
          )
        })}
      </div>
    )}
    {activity.stock && (
      <div className="shop-stock">
        {activity.stock.map((stock) => (
          <article className="activity-option" key={stock.item_id}>
            <img src={`/art/pixel/${stock.item_id}.png`} alt={stock.name} className="item-pixel-art" />
            <div>
              <strong>{stock.name}</strong>
              <p>{catalog.find((entry) => entry.id === stock.item_id)?.description}</p>
              {ITEM_LORE[stock.item_id] && <small className="item-lore-quote">{ITEM_LORE[stock.item_id]}</small>}
            </div>
            <button className="button button-outline" disabled={!stock.available || state.gold < stock.price} onClick={() => onCommand('room.action', { action: 'buy', item_id: stock.item_id })}>
              {stock.available ? `${stock.price} Gold` : 'หมด'}
            </button>
          </article>
        ))}
      </div>
    )}
    {activity.type === 'SHOP' && <button className="button button-quiet activity-leave" onClick={() => onCommand('room.action', { action: 'leave' })}>ออกจากร้าน →</button>}
  </section>
}

type VFX = { id: number; type: 'slash' | 'magic' | 'arrow' | 'shield' | 'crit'; text?: string; colorClass?: string }

function CombatPanel({ state, catalog, onCommand }: { state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const combat = state.combat!
  const potions = state.inventory.filter((stack) => catalog.find((item) => item.id === stack.item_id)?.type === 'consumable' && catalog.find((item) => item.id === stack.item_id)?.usable_in_combat)
  
  const [selectedTarget, setSelectedTarget] = useState<number>(combat.selected_enemy ?? 0)
  const [vfxList, setVfxList] = useState<VFX[]>([])
  const [shake, setShake] = useState(false)

  // Keep selectedTarget valid
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
      speed: 10,
      intent: 'ATTACK',
      intent_value: combat.attack,
      alive: combat.hp > 0,
    }
  ]

  const activeTargetEnemy = enemies.find((e) => e.index === selectedTarget && e.alive) || enemies.find((e) => e.alive) || enemies[0]
  const playerSpeed = state.stats.spd || state.stats.base_spd || 10
  const isPlayerFaster = combat.first_strike || (activeTargetEnemy && playerSpeed >= activeTargetEnemy.speed)

  function triggerVFX(type: 'slash' | 'magic' | 'arrow' | 'shield' | 'crit', text?: string, colorClass?: string) {
    const id = Date.now() + Math.random()
    setVfxList((prev) => [...prev, { id, type, text, colorClass }])
    setTimeout(() => {
      setVfxList((prev) => prev.filter((v) => v.id !== id))
    }, 1100)
  }

  function handleAttack() {
    const archetype = state.stats.archetype || 'sword_shield'
    if (activeTargetEnemy.intent === 'HEAVY') {
      setShake(true)
      setTimeout(() => setShake(false), 380)
    }
    if (archetype === 'bow') {
      sound.playArrow()
      triggerVFX('arrow', `ยิงใส่ ${activeTargetEnemy.name}!`, 'dmg-red')
    } else if (archetype === 'staff') {
      sound.playMagic()
      triggerVFX('magic', `ยิงเวทใส่ ${activeTargetEnemy.name}!`, 'dmg-mp')
    } else if (archetype === 'daggers') {
      sound.playCrit()
      triggerVFX('slash', `แทงจุดตาย ${activeTargetEnemy.name}!`, 'dmg-crit')
    } else {
      sound.playSlash()
      triggerVFX('slash', `ฟันดาบใส่ ${activeTargetEnemy.name}!`, 'dmg-red')
    }
    onCommand('combat.action', { action: 'ATTACK', target: activeTargetEnemy.index })
  }

  function handleSkill() {
    const skillName = state.stats.weapon_skill || 'สกิลอาวุธ'
    const archetype = state.stats.archetype || 'sword_shield'
    setShake(true)
    setTimeout(() => setShake(false), 380)
    if (archetype === 'staff') {
      sound.playMagic()
      triggerVFX('magic', `ระเบิด ${skillName}! (AoE)`, 'dmg-crit')
    } else if (archetype === 'bow') {
      sound.playArrow()
      triggerVFX('arrow', `${skillName} เจาะเกราะ!`, 'dmg-crit')
    } else if (archetype === 'daggers') {
      sound.playCrit()
      triggerVFX('crit', `CRITICAL ${skillName}!`, 'dmg-crit')
    } else {
      sound.playShield()
      triggerVFX('slash', `${skillName}!`, 'dmg-crit')
    }
    onCommand('combat.action', { action: 'SKILL', target: activeTargetEnemy.index })
  }

  function handleDefend() {
    sound.playShield()
    triggerVFX('shield', 'ตั้งรับ! ป้องกันดาเมจ', 'dmg-block')
    onCommand('combat.action', { action: 'DEFEND' })
  }

  function handleUseItem(itemId: string) {
    const item = catalog.find((e) => e.id === itemId)
    if (item?.effect?.type === 'damage_aoe') {
      sound.playCrit()
      triggerVFX('crit', `ปาระเบิดเพลิงใส่ศัตรูทั้งหมด!`, 'dmg-crit')
    } else if (item?.effect?.type === 'heal_mp') {
      sound.playMagic()
      triggerVFX('shield', `ฟื้นฟูมานา +${item.effect.value} MP!`, 'dmg-mp')
    } else {
      sound.playShield()
      triggerVFX('shield', `ฟื้นฟู HP +${item?.effect?.value || 0}!`, 'dmg-heal')
    }
    onCommand('combat.action', { action: 'USE_ITEM', item_id: itemId })
  }

  const isDied = state.stats.hp <= 0
  const isVictory = combat.victory || (combat.finished && !isDied)

  return (
    <section className={`panel combat-panel ${combat.finished ? 'combat-finished' : ''} ${shake ? 'screen-shake' : ''}`}>
      <div className="panel-title">
        <div>
          <span className="eyebrow">{combat.finished ? 'ENCOUNTER COMPLETE' : `TURN ${combat.turn || 1} · YOUR MOVE`}</span>
          <h2>{combat.name}</h2>
        </div>
        <span className="map-difficulty">{combat.tier.toUpperCase()}</span>
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
            <span>🎯 <strong>Ranger First Strike!</strong> คุณได้สิทธิ์ชิงเปิดฉากก่อนเสมอในเทิร์นแรก</span>
          ) : isPlayerFaster ? (
            <span>⚡ <strong>ความเร็วเหนือกว่า!</strong> (SPD {playerSpeed} vs {activeTargetEnemy?.speed || 10}) คุณจะได้ออกท่าก่อน</span>
          ) : (
            <span>⚠️ <strong>ศัตรูว่องไวกว่า!</strong> (SPD {playerSpeed} vs {activeTargetEnemy?.speed || 10}) ศัตรูจะชิงโจมตีก่อน</span>
          )}
        </div>
        <div style={{ fontSize: '11px', color: '#9d94b0' }}>
          เป้าหมายปัจจุบัน: <strong>{activeTargetEnemy.name}</strong>
        </div>
      </div>

      {/* Multi-Monster Combat Stage with Visual FX */}
      <div className="combat-stage-multi">
        {/* Floating Combat Text Container */}
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
              className={`monster-card ${isSelected ? 'is-selected' : ''} ${!enemy.alive ? 'is-dead' : ''}`}
              onClick={() => {
                if (enemy.alive) {
                  setSelectedTarget(enemy.index)
                  onCommand('combat.select_target', { target: enemy.index })
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
                  {isHeavy ? `⚠️ ท่าไม้ตายหนัก! (${enemy.intent_value} DMG)` : `🗡️ เตรียมโจมตี (${enemy.intent_value} DMG)`}
                </div>
              ) : (
                <div className="monster-intent-pill" style={{ color: '#888' }}>
                  💀 กำจัดแล้ว
                </div>
              )}

              <div className="monster-card-sprite">
                <img
                  src={`/art/pixel/${enemy.monster_id}.png`}
                  alt={enemy.name}
                  onError={(e) => { (e.currentTarget as HTMLElement).style.display = 'none'; }}
                />
              </div>

              <div className="monster-card-stats">
                <div className="hp-bar-row">
                  <span>HP</span>
                  <strong>{enemy.hp} / {enemy.max_hp}</strong>
                </div>
                <div className="health-track">
                  <i style={{ width: `${Math.max(0, enemy.hp / (enemy.max_hp || 1) * 100)}%` }} />
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '9px', color: '#8d859c', marginTop: '4px' }}>
                  <span>ATK {enemy.attack}</span>
                  <span>DEF {enemy.defense}</span>
                </div>
              </div>
            </div>
          )
        })}

        {/* Combat Finished Overlay Modal (Victory / Defeat Dialog) */}
        {combat.finished && (
          <div className="combat-finish-overlay">
            <div className={`combat-finish-card ${isVictory ? 'is-victory' : 'is-defeat'}`}>
              <h3 className="combat-finish-title">
                {isVictory ? '🎉 ได้รับชัยชนะเหนือศัตรู!' : '💀 พ่ายแพ้ในการต่อสู้!'}
              </h3>
              <p className="combat-finish-desc">
                {isVictory
                  ? 'กำจัดกองทัพศัตรูสำเร็จ! ได้รับค่าประสบการณ์ (EXP) สำหรับอัปเลเวล และได้รับ Gold เรียบร้อยแล้ว'
                  : 'คุณล้มลงจากการต่อสู้อันดุเดือด แต่พลังแห่งนักผจญภัยได้นำคุณกลับสู่จุดเซฟ (Checkpoint) พร้อมฟื้นคืนชีพ HP เต็มเปี่ยม เพื่อให้คุณเตรียมตัวกลับมาลุยต่ออย่างสมศักดิ์ศรี'}
              </p>
              <button
                className={`button ${isVictory ? 'button-gold' : 'button-outline'}`}
                style={{ width: '100%', fontSize: '13px', padding: '10px 18px' }}
                onClick={() => {
                  sound.playLevelUp()
                  onCommand('combat.dismiss')
                }}
              >
                {isVictory ? '🗺️ เดินทางต่อในดันเจียน' : '🛡️ ฟื้นคืนชีพที่จุดเซฟ (กลับสู่แผนที่)'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Combat Log */}
      <div className="combat-log">
        {combat.log.slice(-5).map((entry, index) => <p key={`${index}-${entry}`}>{entry}</p>)}
      </div>

      {/* Action Buttons */}
      {!combat.finished && (
        <div className="combat-actions">
          <button className="button button-primary" onClick={handleAttack}>
            ⚔️ โจมตีเป้าหมาย
          </button>

          {state.stats.weapon_skill && (
            <button
              className="button skill-button"
              disabled={(state.stats.mp || 0) < (state.stats.weapon_skill_cost || 0)}
              onClick={handleSkill}
              title={state.stats.weapon_skill_desc}
            >
              <span>✨ {state.stats.weapon_skill}</span>
              <span className="skill-cost-badge">{state.stats.weapon_skill_cost} MP</span>
            </button>
          )}

          <button className="button button-outline" onClick={handleDefend}>
            🛡️ ตั้งรับ {state.stats.archetype === 'sword_shield' ? '(ลด 70%+สะท้อน)' : state.stats.archetype === 'rogue' ? '(หลบ 50%)' : state.stats.archetype === 'staff' ? '(เกราะ+MP)' : '(ลด 50%)'}
          </button>

          {potions.map((item) => {
            const entry = catalog.find((cat) => cat.id === item.item_id)
            return (
              <button className="button button-outline button-with-icon" key={item.item_id} onClick={() => handleUseItem(item.item_id)}>
                <img src={`/art/pixel/${item.item_id}.png`} alt="" className="btn-pixel-icon"/>
                ใช้ {entry?.name}
              </button>
            )
          })}

          {combat.tier !== 'boss' && (
            <button className="button button-quiet" onClick={() => onCommand('combat.action', { action: 'ESCAPE' })}>
              หนี
            </button>
          )}
        </div>
      )}
    </section>
  )
}

function InventoryPanel({ state, catalog, onCommand }: { state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const itemByID = (id: string) => catalog.find((item) => item.id === id)
  const equipped: { slot: string; label: string; itemID: string; index?: number }[] = [
    { slot: 'weapon', label: 'อาวุธ', itemID: state.equipment.weapon },
    { slot: 'armor', label: 'เกราะ', itemID: state.equipment.armor },
    { slot: 'helmet', label: 'หมวก', itemID: state.equipment.helmet },
    ...state.equipment.accessories.map((itemID, index) => ({ slot: 'accessory', index, label: `เครื่องประดับ ${index + 1}`, itemID })),
  ]

  const archetypeLabels: Record<string, string> = {
    sword_shield: '🛡️ ดาบ & โล่ (Knight)',
    staff: '🔮 คทาเวทมนตร์ (Mage)',
    daggers: '🗡️ มีดสั้นคู่ (Rogue)',
    bow: '🏹 ธนูยาว (Ranger)',
  }

  const expPct = state.stats.max_exp ? Math.min(100, Math.floor(((state.stats.exp || 0) / state.stats.max_exp) * 100)) : 0
  const mpPct = state.stats.max_mp ? Math.min(100, Math.floor(((state.stats.mp || 0) / state.stats.max_mp) * 100)) : 0

  return <section className="panel inventory-panel">
    <div className="panel-title">
      <div><span className="eyebrow">PLAYER STATUS · SERVER DETERMINED</span><h2>ตัวละครและอุปกรณ์</h2></div>
      <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
        <span className="archetype-tag">{archetypeLabels[state.stats.archetype || 'sword_shield'] || 'นักผจญภัย'}</span>
        <div className="gold-badge">◈ {state.gold} Gold</div>
      </div>
    </div>

    <div className="player-profile-strip">
      <div className="hero-avatar-box">
        <img src="/art/pixel/hero.png" alt="Hero" className="hero-pixel-art" />
      </div>
      <div className="stat-strip">
        <div>
          <span>LEVEL & EXP</span>
          <strong>Lv. {state.stats.level || 1}</strong>
          <div className="resource-bar">
            <div className="resource-bar-track"><i className="resource-bar-fill-exp" style={{ width: `${expPct}%` }} /></div>
            <i style={{ fontSize: '9px', color: '#e6c27e' }}>{state.stats.exp || 0} / {state.stats.max_exp || 100} EXP</i>
          </div>
        </div>
        <div>
          <span>HP / MP</span>
          <strong>{state.stats.hp}<i> / {state.stats.max_hp} HP</i></strong>
          <div className="resource-bar">
            <div className="resource-bar-track"><i className="resource-bar-fill-mp" style={{ width: `${mpPct}%` }} /></div>
            <i style={{ fontSize: '9px', color: '#70a1ff' }}>{state.stats.mp || 0} / {state.stats.max_mp || 20} MP</i>
          </div>
        </div>
        <div>
          <span>ATK / DEF</span>
          <strong>{state.stats.atk}<i> ATK</i> · {state.stats.def}<i> DEF</i></strong>
        </div>
        <div>
          <span>SPEED (SPD)</span>
          <strong>⚡ {state.stats.spd || state.stats.base_spd || 10}<i> SPD</i></strong>
          <span style={{ fontSize: '9px', color: '#9d94b0' }}>คะแนน {state.score}</span>
        </div>
      </div>
    </div>

    <div className="inventory-section">
      <div className="section-heading"><h3>Equipment</h3><span>ค่าสถานะและสกิลคำนวณจากอุปกรณ์ที่สวมใส่</span></div>
      <div className="equipment-grid">
        {equipped.map((entry) => {
          const item = itemByID(entry.itemID)
          return (
            <article className="equipment-cell" key={`${entry.slot}-${entry.index ?? 0}`}>
              <span>{entry.label}</span>
              {item ? (
                <div className="equipped-item-row">
                  <img src={`/art/pixel/${item.id}.png`} alt={item.name} className="item-pixel-art" />
                  <div>
                    <strong>{item.name}</strong>
                    <small>
                      ATK +{item.stats.attack} · DEF +{item.stats.defense}
                      {item.stats.speed ? ` · SPD +${item.stats.speed}` : ''}
                      {item.stats.max_mp ? ` · MP +${item.stats.max_mp}` : ''}
                      {item.stats.max_hp ? ` · HP +${item.stats.max_hp}` : ''}
                    </small>
                    {item.skill_name && <small style={{ color: '#b182ff' }}>สกิล: {item.skill_name}</small>}
                    {ITEM_LORE[item.id] && <small className="item-lore-quote">{ITEM_LORE[item.id]}</small>}
                  </div>
                </div>
              ) : <strong>ว่าง</strong>}
              {item && <button className="text-button" onClick={() => onCommand('inventory.unequip', { slot: entry.slot, ...(entry.index === undefined ? {} : { index: entry.index }) })}>ถอดออก</button>}
            </article>
          )
        })}
      </div>
    </div>

    <div className="inventory-section">
      <div className="section-heading"><h3>Backpack</h3><span>{state.inventory.length} / 12 ช่อง (เปลี่ยนสายอาวุธได้ตลอดเวลา)</span></div>
      {state.inventory.length ? (
        <div className="backpack-grid">
          {state.inventory.map((stack) => {
            const item = itemByID(stack.item_id)
            const canHealHP = item?.type === 'consumable' && item.effect?.type === 'heal' && state.stats.hp < state.stats.max_hp
            const canHealMP = item?.type === 'consumable' && item.effect?.type === 'heal_mp' && (state.stats.mp || 0) < (state.stats.max_mp || 20)
            const canUse = canHealHP || canHealMP
            return (
              <article className="item-cell" key={stack.item_id}>
                <div className={`rarity-mark rarity-${item?.rarity || 'common'}`} />
                <img src={`/art/pixel/${stack.item_id}.png`} alt={item?.name || stack.item_id} className="item-pixel-art" />
                <div className="item-copy">
                  <strong>{item?.name || stack.item_id}{stack.quantity > 1 && <small> ×{stack.quantity}</small>}</strong>
                  <span>{item?.description || 'ไอเทม'}</span>
                  {item?.skill_name && <span style={{ color: '#b182ff' }}>มอบสกิล: {item.skill_name} ({item.skill_cost} MP)</span>}
                  {item && ITEM_LORE[item.id] && <span className="item-lore-quote">{ITEM_LORE[item.id]}</span>}
                </div>
                {item && item.type !== 'consumable' && (
                  <button className="button button-outline compact" onClick={() => onCommand('inventory.equip', { item_id: item.id, slot: item.slot })}>สวมใส่</button>
                )}
                {item?.type === 'consumable' && (
                  <button className="button button-outline compact" disabled={!canUse} onClick={() => onCommand('inventory.use', { item_id: item.id })}>
                    {canUse ? 'ใช้' : 'ใช้ไม่ได้'}
                  </button>
                )}
              </article>
            )
          })}
        </div>
      ) : <div className="empty-inventory">Backpack ยังว่างอยู่</div>}
    </div>
  </section>
}

function DungeonMap({ state, onMove }: { state: DungeonView; onMove: (nodeID: string) => void }) {
  const floor = state.floors.find((entry) => entry.number === state.current_floor)
  if (!floor) return null
  const roomNames: Record<string, string> = {
    START: 'ทางเข้า',
    MONSTER: 'ศัตรู',
    ELITE: 'อีลีท',
    TREASURE: 'สมบัติ',
    SHOP: 'ร้านค้า',
    EVENT: 'เหตุการณ์',
    REST: 'จุดพัก',
    PUZZLE: 'ปริศนา',
    BOSS: 'บอส',
  }
  const statusNames: Record<string, string> = {
    LOCKED: 'ยังไม่ปลดล็อก',
    AVAILABLE: 'เลือกได้',
    IN_PROGRESS: 'กำลังสำรวจ',
    CLEARED: 'ผ่านแล้ว',
    SKIPPED: 'ข้ามแล้ว',
    FAILED: 'ลองใหม่ได้',
  }

  // Dynamic depths calculation
  const depths = Array.from(new Set(floor.nodes.map((node) => node.depth))).sort((a, b) => a - b)
  const maxDepth = depths[depths.length - 1] || 6

  return <section className="panel dungeon-map-panel">
    <div className="panel-title">
      <div><span className="eyebrow">BRANCHING ROUTE · FLOOR {floor.number}</span><h2>แผนที่ดันเจียน</h2></div>
      <span className="map-difficulty">ความยาก ×{floor.difficulty_multiplier}</span>
    </div>
    <p className="muted" style={{ margin: '4px 0 14px', fontSize: '11px', color: '#c2b6d1' }}>
      <strong>{FLOOR_LORE[floor.number]?.title || `ชั้นที่ ${floor.number}`}</strong> — {FLOOR_LORE[floor.number]?.desc || 'สำรวจเส้นทางของคุณ'}
    </p>
    <div className="map-depths" style={{ gridTemplateColumns: `repeat(${depths.length}, minmax(118px, 1fr))` }}>
      {depths.map((depth) => (
        <div className="map-depth" key={depth}>
          <div className="depth-label">{depth === 0 ? 'จุดเริ่ม' : depth === maxDepth ? 'Boss Chamber' : `ก้าวที่ ${depth}`}</div>
          <div className="depth-nodes">
            {floor.nodes.filter((node) => node.depth === depth).map((node) => (
              <button
                key={node.id}
                className={`map-node status-${node.status.toLowerCase().replace('_', '-')}`}
                disabled={node.status !== 'AVAILABLE'}
                onClick={() => onMove(node.id)}
              >
                <span className="node-glyph"><RoomIcon type={node.type}/></span>
                <strong>{roomNames[node.type] || node.type}</strong>
                <small>{statusNames[node.status] || node.status}</small>
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
    <div className="map-footer">
      ความคืบหน้าชั้นนี้ {state.progress} / {maxDepth}
      {state.active_node_id && <span> · กำลังอยู่ในห้อง: {roomNames[floor.nodes.find((node) => node.id === state.active_node_id)?.type || ''] || state.active_node_id}</span>}
    </div>
    <div className="other-floors">
      {state.floors.filter((entry) => entry.number > state.current_floor).map((entry) => (
        <span key={entry.number}>ชั้น {entry.number} · ยังไม่ปลดล็อก</span>
      ))}
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
