import { FormEvent, useEffect, useMemo, useState } from 'react'
import './inventory.css'
import './map.css'

type Session = { room_code: string; player_id: string; reconnect_token: string; is_host: boolean; ws_path: string }
type Player = { player_id: string; name: string; is_host: boolean; ready: boolean; connected: boolean }
type ItemDefinition = { id: string; name: string; description: string; type: 'weapon' | 'armor' | 'helmet' | 'accessory' | 'consumable'; rarity: string; slot: string; stats: { attack: number; defense: number; max_hp: number }; effect: { type: string; value: number } | null; price: number; stackable: boolean; max_stack: number; usable_in_combat: boolean }
type DungeonNode = { id: string; floor: number; depth: number; type: string; status: string; connections: string[] }
type DungeonView = { current_floor: number; current_node_id: string; checkpoint_node_id: string; active_node_id?: string; progress: number; completed: boolean; floors: { number: number; difficulty_multiplier: number; nodes: DungeonNode[] }[] }
type CombatView = { monster_id: string; name: string; tier: string; hp: number; max_hp: number; attack: number; defense: number; guarding: boolean; finished: boolean; log: string[] }
type ActivityView = { type: string; options?: { id: string; label: string; description: string; kind: string; item_id?: string; value?: number }[]; stock?: { item_id: string; name: string; price: number; available: boolean }[] }
type PrivateState = { stats: { hp: number; max_hp: number; base_atk: number; base_def: number; atk: number; def: number }; gold: number; score: number; deaths: number; inventory: { item_id: string; quantity: number }[]; equipment: { weapon: string; armor: string; helmet: string; accessories: [string, string] }; dungeon?: DungeonView; combat?: CombatView; activity?: ActivityView; pvp_actions_used?: number; pvp_actions_remaining?: number; pvp_score?: number; pvp_damage?: number; pvp_knockouts?: number }
type PublicPlayer = Player & { stage: string; floor: number; progress: number; score: number; completed: boolean; hp: number; max_hp: number; pvp_actions_remaining: number; pvp_score: number; pvp_damage: number; pvp_knockouts: number }
type ResultEntry = { player_id: string; name: string; rank: number; total_score: number; pve_score: number; pvp_score: number; gold_score: number; bonus: number; penalties: number; pvp_damage: number; pvp_knockouts: number; pvp_deaths: number; bosses_defeated: number; gold: number }
type Snapshot = { version: number; room: { phase: string; host_player_id: string; player_count: number; min_players: number; max_players: number }; players: PublicPlayer[]; me: Player & { private?: PrivateState }; results?: ResultEntry[] }
type WireMessage = { type: string; request_id?: string; payload?: any }
const SESSION_KEY = 'dungeon-rivals-session'

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
          const code = message.payload?.code
          setError(message.payload?.message || code || 'คำสั่งไม่สำเร็จ')
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
    return <main className="shell lobby-shell"><header className="topbar"><div className="brand"><RoomIcon type="START" className="brand-sigil"/><span>DUNGEON <b>RIVALS</b></span></div><div className="status"><span className={connected ? 'pulse online' : 'pulse'} />{connected ? 'กำลังโหลดห้อง' : 'กำลังเชื่อมต่อ'}</div></header><section className="panel roster-panel loading-panel"><span className="eyebrow">กำลังเข้าสู่ปาร์ตี้</span><h2>{session.room_code}</h2><p className="muted">กำลังเชื่อมต่อกับห้องของคุณ…</p>{error && <div className="form-error">{error}</div>}<button className="button button-quiet" onClick={forgetSession}>กลับหน้าหลัก</button></section></main>
  }

  if (session && snapshot) {
    const me = snapshot.me
    const allReady = snapshot.players.length >= snapshot.room.min_players && snapshot.players.every((player) => player.ready)
    return <main className="shell lobby-shell">
      <header className="topbar"><div className="brand"><RoomIcon type="START" className="brand-sigil"/><span>DUNGEON <b>RIVALS</b></span></div><div className="status"><span className={connected ? 'pulse online' : 'pulse'} />{connected ? 'เชื่อมต่อแล้ว' : 'กำลังเชื่อมต่อ'}</div></header>
      <section className="lobby-head"><div><p className="eyebrow">ห้องรอผู้เล่น</p><h1>เตรียมทีมให้พร้อม</h1><p className="muted">ผู้เล่นแต่ละคนจะได้สำรวจเส้นทางดันเจียนของตัวเอง</p></div><div className="room-code"><span>ROOM CODE</span><strong>{session.room_code}</strong></div></section>
      <section className="panel roster-panel"><div className="panel-title"><div><span className="eyebrow">ADVENTURERS</span><h2>สมาชิกในห้อง</h2></div><span className="count">{snapshot.players.length}<i> / {snapshot.room.max_players}</i></span></div>
        <div className="roster">{[...snapshot.players, ...Array.from({ length: snapshot.room.max_players - snapshot.players.length }, () => null)].map((player, i) => player ? <article className="player-card" key={player.player_id}><div className={`avatar avatar-${i % 4}`}><img src="/art/pixel/hero.png" alt="" className="roster-hero-art"/></div><div className="player-info"><strong>{player.name}{player.player_id === session.player_id && <small>คุณ</small>}</strong><span>{player.is_host ? 'เจ้าของห้อง' : 'นักผจญภัย'} · {player.connected ? 'ออนไลน์' : 'ออฟไลน์'}{player.floor ? ` · ชั้น ${player.floor} / ความคืบหน้า ${player.progress}` : ''}</span></div><div className={`ready-tag ${player.ready ? 'is-ready' : ''}`}>{player.ready ? 'พร้อม' : player.stage === 'exploring' ? 'สำรวจอยู่' : 'รออยู่'}</div></article> : <article className="player-card empty-card" key={`empty-${i}`}><div className="avatar empty-avatar">＋</div><div className="player-info"><strong>รอผู้เล่นเข้าร่วม</strong><span>แชร์รหัสห้องเพื่อชวนเพื่อน</span></div><span className="empty-dot" /></article>)}</div>
        {snapshot.room.phase === 'lobby' && <><div className="lobby-actions"><button className={`button ${me?.ready ? 'button-muted' : 'button-primary'}`} onClick={() => command('lobby.ready', { ready: !me?.ready })}>{me?.ready ? 'ยกเลิกความพร้อม' : 'พร้อมแล้ว'}</button>{session.is_host && <button className="button button-gold" disabled={!allReady} onClick={() => command('game.start')}>เริ่มเกม <span>→</span></button>}<button className="button button-quiet" onClick={leave}>ออกจากห้อง</button></div><p className="hint">{host?.name || 'เจ้าของห้อง'} จะเริ่มเกมได้เมื่อมีผู้เล่นอย่างน้อย 2 คนและทุกคนพร้อม</p></>}
      </section>
      {snapshot.me.private?.dungeon && <DungeonMap state={snapshot.me.private.dungeon} onMove={(nodeID) => command('dungeon.move', { node_id: nodeID })} />}
      {snapshot.me.private?.combat && <CombatPanel state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />}
      {snapshot.me.private?.activity && <RoomActivityPanel state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />}
      {((snapshot.room.phase === 'dungeon' && snapshot.me.private?.dungeon?.completed) || snapshot.room.phase === 'preparation') && <PreparationPanel players={snapshot.players} isReady={me.ready} phase={snapshot.room.phase} isHost={me.is_host} onReady={(ready) => command('preparation.ready',{ready})} onKick={(playerID)=>command('room.kick_disconnected',{target_player_id:playerID})} />}
      {snapshot.room.phase === 'pvp' && snapshot.me.private && <ArenaPanel players={snapshot.players} me={me} isHost={me.is_host} state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />}
      {snapshot.room.phase === 'results' && snapshot.results && <ResultsPanel roomCode={session.room_code} results={snapshot.results} />}
      {snapshot.me.private && !['pvp','results'].includes(snapshot.room.phase) && <InventoryPanel state={snapshot.me.private} catalog={itemCatalog} onCommand={command} />}
      {snapshot.room.phase !== 'lobby' && <div className="notice">{snapshot.room.phase === 'pvp' ? 'ทุกคนพร้อมแล้ว · เข้าสู่สนามประลอง' : snapshot.room.phase === 'preparation' ? 'ช่วงเตรียมตัว · ตรวจอุปกรณ์และกดพร้อมเมื่อเสร็จ' : 'เกมเริ่มแล้ว · ตัวละครและ Inventory ของคุณถูกสร้างโดย Server'}</div>}
      {error && <div className="error-toast">{error}<button onClick={() => setError('')}>×</button></div>}
      <footer>สร้างปาร์ตี้ · สำรวจดันเจียน · ชิงชัยในอารีนา</footer>
    </main>
  }

  return <main className="shell landing-shell"><div className="topbar"><div className="brand"><RoomIcon type="START" className="brand-sigil"/><span>DUNGEON <b>RIVALS</b></span></div><span className="version">MULTIPLAYER DUNGEON ROGUELITE</span></div><div className="landing-grid"><section className="hero"><div className="rune rune-one">✧</div><div className="rune rune-two">✦</div><p className="eyebrow">YOUR PARTY AWAITS</p><h1>ดันเจียนเดียวกัน<br/><em>เส้นทางของคุณ</em></h1><p className="hero-copy">รวมทีมกับเพื่อน ออกสำรวจและสะสมอุปกรณ์<br/>ก่อนพิสูจน์ฝีมือในสนามประลอง</p><figure className="hero-art"><img src="/art/dungeon-gate.svg" alt="ประตูหินโบราณส่องแสงสีเขียวอมฟ้าอยู่ในถ้ำ"/><figcaption>GATEWAY OF THE FIRST FLOOR</figcaption></figure><div className="hero-stats"><span><b>02–04</b><small>ผู้เล่นต่อห้อง</small></span><i /><span><b>CO-OP</b><small>สำรวจพร้อมกัน</small></span><i /><span><b>PVP</b><small>ชิงชัยช่วงท้าย</small></span></div></section>
      <section className="panel entry-panel"><div className="panel-title"><div><span className="eyebrow">เริ่มการผจญภัย</span><h2>เข้าสู่ดันเจียน</h2></div><span className="crystal">◈</span></div><form onSubmit={createRoom}><label htmlFor="player-name">ชื่อนักผจญภัย</label><input id="player-name" maxLength={24} placeholder="ใส่ชื่อของคุณ" value={name} onChange={(e) => setName(e.target.value)} required/><button className="button button-primary full-width" disabled={pending}>{pending ? 'กำลังสร้างห้อง…' : 'สร้างห้องใหม่'} <span>→</span></button></form><div className="divider"><span>หรือเข้าร่วมปาร์ตี้</span></div><form className="join-form" onSubmit={joinRoom}><label htmlFor="room-code">รหัสห้อง</label><div className="join-row"><input id="room-code" maxLength={6} placeholder="เช่น A7K9Q2" value={joinCode} onChange={(e) => setJoinCode(e.target.value.toUpperCase())} required/><button className="button button-outline" disabled={pending}>เข้าร่วม</button></div></form>{error && <div className="form-error">{error}</div>}<p className="privacy-note">เกมทดลอง · ความคืบหน้าเก็บไว้ชั่วคราวระหว่างที่เซิร์ฟเวอร์ทำงาน</p></section></div><footer>สร้างปาร์ตี้ · สำรวจดันเจียน · ชิงชัยในอารีนา</footer></main>
}

function PreparationPanel({ players, isReady, phase, isHost, onReady, onKick }: { players: PublicPlayer[]; isReady: boolean; phase: string; isHost: boolean; onReady: (ready: boolean) => void; onKick:(playerID:string)=>void }) {
  const allFinished=players.every((player)=>player.completed)
  return <section className="panel preparation-panel"><div className="panel-title"><div><span className="eyebrow">FINAL CHECK · PREPARATION</span><h2>{phase==='pvp'?'พร้อมสู้ใน Arena':'เตรียมตัวก่อน PvP'}</h2></div><span className={`ready-tag ${isReady?'is-ready':''}`}>{isReady?'พร้อมแล้ว':'ยังไม่พร้อม'}</span></div><p className="muted">{allFinished?'ทุกคนจบดันเจียนแล้ว ตรวจ HP และอุปกรณ์ของคุณ จากนั้นกดพร้อม':'คุณจบดันเจียนแล้ว รอเพื่อนร่วมทีมเล่นต่อได้เลย'}</p><div className="prep-roster">{players.map((player)=><span key={player.player_id} className={player.completed?'prep-complete':''}>{player.name} · {player.completed?'จบแล้ว':player.connected?'กำลังเล่น':'หลุดการเชื่อมต่อ'}{isHost&&!player.connected&&<button className="text-button" onClick={()=>onKick(player.player_id)}>นำออก</button>}</span>)}</div>{phase!=='pvp'&&<button className={`button ${isReady?'button-muted':'button-gold'}`} disabled={phase==='preparation'&&!allFinished} onClick={()=>onReady(!isReady)}>{isReady?'ยกเลิกความพร้อม':'พร้อมเข้าสู่ PvP'}</button>}</section>
}

function ArenaPanel({ players, me, isHost, state, catalog, onCommand }: { players: PublicPlayer[]; me: Player; isHost: boolean; state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const left=state.pvp_actions_remaining??0
  const potions=state.inventory.filter((stack)=>catalog.find((item)=>item.id===stack.item_id)?.type==='consumable'&&catalog.find((item)=>item.id===stack.item_id)?.usable_in_combat)
  return <section className="panel arena-panel"><div className="panel-title"><div><span className="eyebrow">FINAL PVP · SERVER RESOLVED</span><h2>สนามประลอง</h2></div><span className="gold-badge">เหลือ {left} Action</span></div><div className="arena-roster">{players.filter((player)=>player.player_id!==me.player_id).map((player)=><article className="arena-opponent" key={player.player_id}><div className="arena-opponent-header"><img src="/art/pixel/hero.png" alt="" className="arena-hero-art" /><div><strong>{player.name}</strong><small>HP {player.hp} / {player.max_hp} · PvP Score {player.pvp_score}</small></div></div><div className="arena-health"><i style={{width:`${player.max_hp?player.hp/player.max_hp*100:0}%`}} /></div><button className="button button-primary" disabled={!left} onClick={()=>onCommand('pvp.action',{action:'ATTACK',target_player_id:player.player_id})}>โจมตี · ใช้ 1 Action</button></article>)}</div><div className="arena-actions"><button className="button button-outline" disabled={!left} onClick={()=>onCommand('pvp.action',{action:'DEFEND'})}>ตั้งรับ</button>{potions.map((item)=><button className="button button-outline button-with-icon" disabled={!left} key={item.item_id} onClick={()=>onCommand('pvp.action',{action:'USE_ITEM',item_id:item.item_id})}><img src={`/art/pixel/${item.item_id}.png`} alt="" className="btn-pixel-icon"/>ใช้ {catalog.find((entry)=>entry.id===item.item_id)?.name}</button>)}<span>HP {state.stats.hp}/{state.stats.max_hp} · DMG {state.pvp_damage||0} · KO {state.pvp_knockouts||0}</span></div>{left===0&&<p className="muted">ใช้ Action ครบแล้ว กำลังรอผู้เล่นคนอื่น</p>}{isHost&&players.some((player)=>!player.connected)&&<button className="button button-quiet activity-leave" onClick={()=>onCommand('pvp.force_finish')}>จบ PvP สำหรับผู้เล่นที่หลุด</button>}</section>
}

function ResultsPanel({ roomCode, results }: { roomCode: string; results: ResultEntry[] }) {
  return <section className="panel results-panel"><div className="panel-title"><div><span className="eyebrow">MATCH COMPLETE</span><h2>ผลการแข่งขัน</h2></div><a className="button button-outline" href={`/api/v1/rooms/${encodeURIComponent(roomCode)}/results.csv`} download>ดาวน์โหลด CSV</a></div><div className="results-list">{results.map((result)=><article className={`result-card ${result.rank===1?'winner':''}`} key={result.player_id}><div className="result-rank">#{result.rank}</div><div className="result-player"><strong>{result.name}</strong><small>PvE {result.pve_score} · PvP {result.pvp_score} · Gold {result.gold_score} · Bonus +{result.bonus} · Penalty −{result.penalties}</small><small>Damage {result.pvp_damage} · KO {result.pvp_knockouts} · Deaths {result.pvp_deaths} · Boss {result.bosses_defeated}</small></div><strong className="result-total">{result.total_score}</strong></article>)}</div></section>
}

function RoomActivityPanel({ state, catalog, onCommand }: { state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const activity = state.activity!
  const roomLabel: Record<string, string> = { TREASURE: 'ห้องสมบัติ', SHOP: 'ร้านค้า', EVENT: 'เหตุการณ์พิเศษ' }
  return <section className="panel activity-panel"><div className="panel-title"><div><span className="eyebrow">ROOM · {activity.type}</span><h2>{roomLabel[activity.type] || activity.type}</h2></div>{activity.type === 'SHOP' && <span className="gold-badge">◈ {state.gold} Gold</span>}</div>
    {activity.options && <div className="activity-options">{activity.options.map((option) => { const item=catalog.find((entry)=>entry.id===option.item_id); return <article className="activity-option" key={option.id}>{item && <img src={`/art/pixel/${item.id}.png`} alt={item.name} className="item-pixel-art" />}<div><strong>{option.label}</strong><p>{option.description}</p>{item && <small>{item.description}</small>}</div><button className="button button-primary" onClick={()=>onCommand('room.action',{action:'claim',choice_id:option.id})}>เลือก</button></article> })}</div>}
    {activity.stock && <div className="shop-stock">{activity.stock.map((stock) => <article className="activity-option" key={stock.item_id}><img src={`/art/pixel/${stock.item_id}.png`} alt={stock.name} className="item-pixel-art" /><div><strong>{stock.name}</strong><p>{catalog.find((entry)=>entry.id===stock.item_id)?.description}</p></div><button className="button button-outline" disabled={!stock.available||state.gold<stock.price} onClick={()=>onCommand('room.action',{action:'buy',item_id:stock.item_id})}>{stock.available?`${stock.price} Gold`:'หมด'}</button></article>)}</div>}
    {activity.type==='SHOP' && <button className="button button-quiet activity-leave" onClick={()=>onCommand('room.action',{action:'leave'})}>ออกจากร้าน →</button>}
  </section>
}

function CombatPanel({ state, catalog, onCommand }: { state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const combat = state.combat!
  const potions = state.inventory.filter((stack) => catalog.find((item) => item.id === stack.item_id)?.type === 'consumable' && catalog.find((item) => item.id === stack.item_id)?.usable_in_combat)
  return <section className={`panel combat-panel ${combat.finished ? 'combat-finished' : ''}`}>
    <div className="panel-title"><div><span className="eyebrow">{combat.finished ? 'ENCOUNTER COMPLETE' : 'PLAYER TURN'}</span><h2>{combat.name}</h2></div><span className="map-difficulty">{combat.tier.toUpperCase()}</span></div>
    <div className="combat-stage">
      <div className="monster-sprite-box">
        <img src={`/art/pixel/${combat.monster_id}.png`} alt={combat.name} className="monster-pixel-art" onError={(e) => { (e.currentTarget as HTMLElement).style.display = 'none'; }} />
      </div>
    </div>
    <div className="combat-health"><div><span>Monster HP</span><strong>{combat.hp} / {combat.max_hp}</strong></div><div className="health-track"><i style={{ width: `${Math.max(0, combat.hp / combat.max_hp * 100)}%` }} /></div><small>ATK {combat.attack} · DEF {combat.defense}</small></div>
    <div className="combat-log">{combat.log.slice(-5).map((entry, index) => <p key={`${index}-${entry}`}>{entry}</p>)}</div>
    {!combat.finished && <div className="combat-actions"><button className="button button-primary" onClick={() => onCommand('combat.action', { action: 'ATTACK' })}>โจมตี</button><button className="button button-outline" onClick={() => onCommand('combat.action', { action: 'DEFEND' })}>ตั้งรับ</button>{potions.map((item) => <button className="button button-outline button-with-icon" key={item.item_id} onClick={() => onCommand('combat.action', { action: 'USE_ITEM', item_id: item.item_id })}><img src={`/art/pixel/${item.item_id}.png`} alt="" className="btn-pixel-icon"/>ใช้ {catalog.find((entry) => entry.id === item.item_id)?.name}</button>)}{combat.tier !== 'boss' && <button className="button button-quiet" onClick={() => onCommand('combat.action', { action: 'ESCAPE' })}>หนี</button>}</div>}
  </section>
}

function InventoryPanel({ state, catalog, onCommand }: { state: PrivateState; catalog: ItemDefinition[]; onCommand: (type: string, payload?: object) => void }) {
  const itemByID = (id: string) => catalog.find((item) => item.id === id)
  const equipped: { slot: string; label: string; itemID: string; index?: number }[] = [
    { slot: 'weapon', label: 'อาวุธ', itemID: state.equipment.weapon },
    { slot: 'armor', label: 'เกราะ', itemID: state.equipment.armor },
    { slot: 'helmet', label: 'หมวก', itemID: state.equipment.helmet },
    ...state.equipment.accessories.map((itemID, index) => ({ slot: 'accessory', index, label: `เครื่องประดับ ${index + 1}`, itemID })),
  ]
  return <section className="panel inventory-panel">
    <div className="panel-title"><div><span className="eyebrow">PLAYER STATE · SERVER CONTROLLED</span><h2>ตัวละครและอุปกรณ์</h2></div><div className="gold-badge">◈ {state.gold} Gold</div></div>
    <div className="player-profile-strip">
      <div className="hero-avatar-box">
        <img src="/art/pixel/hero.png" alt="Hero" className="hero-pixel-art" />
      </div>
      <div className="stat-strip">
        <div><span>HP</span><strong>{state.stats.hp}<i> / {state.stats.max_hp}</i></strong></div>
        <div><span>ATK</span><strong>{state.stats.atk}</strong></div>
        <div><span>DEF</span><strong>{state.stats.def}</strong></div>
        <div><span>SCORE</span><strong>{state.score}</strong></div>
      </div>
    </div>
    <div className="inventory-section"><div className="section-heading"><h3>Equipment</h3><span>ค่าสถานะคำนวณจาก Server</span></div><div className="equipment-grid">{equipped.map((entry) => { const item = itemByID(entry.itemID); return <article className="equipment-cell" key={`${entry.slot}-${entry.index ?? 0}`}><span>{entry.label}</span>{item ? <div className="equipped-item-row"><img src={`/art/pixel/${item.id}.png`} alt={item.name} className="item-pixel-art" /><div><strong>{item.name}</strong><small>ATK +{item.stats.attack} · DEF +{item.stats.defense}{item.stats.max_hp ? ` · HP +${item.stats.max_hp}` : ''}</small></div></div> : <strong>ว่าง</strong>}{item && <button className="text-button" onClick={() => onCommand('inventory.unequip', { slot: entry.slot, ...(entry.index === undefined ? {} : { index: entry.index }) })}>ถอดออก</button>}</article> })}</div></div>
    <div className="inventory-section"><div className="section-heading"><h3>Backpack</h3><span>{state.inventory.length} / 12 ช่อง</span></div>{state.inventory.length ? <div className="backpack-grid">{state.inventory.map((stack) => { const item = itemByID(stack.item_id); const canHeal = item?.type === 'consumable' && item.effect?.type === 'heal' && state.stats.hp < state.stats.max_hp; return <article className="item-cell" key={stack.item_id}><div className={`rarity-mark rarity-${item?.rarity || 'common'}`} /><img src={`/art/pixel/${stack.item_id}.png`} alt={item?.name || stack.item_id} className="item-pixel-art" /><div className="item-copy"><strong>{item?.name || stack.item_id}{stack.quantity > 1 && <small> ×{stack.quantity}</small>}</strong><span>{item?.description || 'ไอเทม'}</span></div>{item && item.type !== 'consumable' && <button className="button button-outline compact" onClick={() => onCommand('inventory.equip', { item_id: item.id, slot: item.slot })}>สวมใส่</button>}{item?.type === 'consumable' && <button className="button button-outline compact" disabled={!canHeal} onClick={() => onCommand('inventory.use', { item_id: item.id })}>{canHeal ? 'ใช้' : state.stats.hp >= state.stats.max_hp ? 'HP เต็ม' : 'ใช้ไม่ได้'}</button>}</article> })}</div> : <div className="empty-inventory">Backpack ยังว่างอยู่</div>}</div>
  </section>
}

function DungeonMap({ state, onMove }: { state: DungeonView; onMove: (nodeID: string) => void }) {
  const floor = state.floors.find((entry) => entry.number === state.current_floor)
  if (!floor) return null
  const roomNames: Record<string, string> = { START: 'ทางเข้า', MONSTER: 'Monster', ELITE: 'Elite', TREASURE: 'สมบัติ', SHOP: 'ร้านค้า', EVENT: 'Event', REST: 'จุดพัก', BOSS: 'Boss' }
  const statusNames: Record<string, string> = { LOCKED: 'ยังไม่ปลดล็อก', AVAILABLE: 'เลือกได้', IN_PROGRESS: 'กำลังสำรวจ', CLEARED: 'ผ่านแล้ว', SKIPPED: 'ข้ามแล้ว', FAILED: 'ลองใหม่ได้' }
  return <section className="panel dungeon-map-panel">
    <div className="panel-title"><div><span className="eyebrow">INDEPENDENT ROUTE · FLOOR {floor.number}</span><h2>แผนที่ดันเจียน</h2></div><span className="map-difficulty">ความยาก ×{floor.difficulty_multiplier}</span></div>
    <div className="map-depths">{[0, 1, 2, 3, 4].map((depth) => <div className="map-depth" key={depth}><div className="depth-label">{depth === 0 ? 'จุดเริ่ม' : depth === 4 ? 'Boss' : `เส้นทาง ${depth}`}</div><div className="depth-nodes">{floor.nodes.filter((node) => node.depth === depth).map((node) => <button key={node.id} className={`map-node status-${node.status.toLowerCase().replace('_', '-')}`} disabled={node.status !== 'AVAILABLE'} onClick={() => onMove(node.id)}><span className="node-glyph"><RoomIcon type={node.type}/></span><strong>{roomNames[node.type] || node.type}</strong><small>{statusNames[node.status] || node.status}</small></button>)}</div></div>)}</div>
    <div className="map-footer">ความคืบหน้าชั้นนี้ {state.progress} / 4{state.active_node_id && <span> · กำลังอยู่ในห้องนี้: {roomNames[floor.nodes.find((node) => node.id === state.active_node_id)?.type || ''] || state.active_node_id}</span>}</div>
    <div className="other-floors">{state.floors.filter((entry) => entry.number > state.current_floor).map((entry) => <span key={entry.number}>ชั้น {entry.number} · ยังไม่ปลดล็อก</span>)}</div>
  </section>
}

function RoomIcon({ type, className }: { type: string; className?: string }) {
  const icon: Record<string,string> = { START:'room-start', MONSTER:'room-monster', ELITE:'room-elite', TREASURE:'room-treasure', SHOP:'room-shop', EVENT:'room-event', REST:'room-rest', BOSS:'room-boss' }
  return <svg className={className || 'room-icon'} viewBox="0 0 48 48" aria-hidden="true" focusable="false"><use href={`/art/room-icons.svg#${icon[type] || 'room-monster'}`} /></svg>
}
