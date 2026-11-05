# DUNGEON RIVALS — Master Game Design & Development Plan

**เอกสารหลักสำหรับ Coding Agent / Source of Truth**  
**เวอร์ชัน:** 0.1  
**ประเภทเกม:** Web-based Multiplayer Competitive Dungeon Roguelite  
**สถานะ:** MVP planning  
**ภาษาเอกสาร:** ไทย (ชื่อ type, field, event และคำศัพท์เทคนิคใช้ภาษาอังกฤษ)

---

# 0. คำสั่งสำหรับ Coding Agent (อ่านก่อนเริ่มทำงาน)

เอกสารนี้รวบรวมแนวคิดและข้อกำหนดที่ตกลงกันไว้สำหรับโปรเจกต์ Dungeon Rivals ให้ถือเป็น **แหล่งข้อมูลหลัก (Source of Truth)** ในการวิเคราะห์ ออกแบบ และพัฒนา ห้ามเปลี่ยนแนวเกมหรือเพิ่มระบบใหญ่เองโดยไม่จำเป็น

## 0.1 สิ่งที่ต้องเข้าใจให้ตรงกัน

1. เกมนี้เป็น **เว็บเกม Multiplayer** เล่นผ่าน Browser
2. ผู้เล่น 2–4 คน อยู่ใน Game Room เดียวกัน แต่ **แต่ละคนเลือกเส้นทาง Dungeon ของตัวเอง** ไม่จำเป็นต้องเดินห้องเดียวกัน
3. เป็นเกม **Turn-based แบบ Independent Turns** ผู้เล่นคนหนึ่งทำ Action ได้โดยไม่ต้องรอผู้เล่นคนอื่นเลือก Action ให้ครบก่อน
4. ผู้เล่นสำรวจ Dungeon ต่อสู้กับ Monster เก็บ Gold, Weapon, Armor, Accessory และ Consumable ระหว่างทาง
5. เมื่อผู้เล่นตายต้อง **Respawn และกลับมาเล่นต่อ** ไม่ถูกตัดออกจากเกมถาวร
6. เมื่อจบ Dungeon ทุกคนจะเข้าสู่ Final PvP และผลแพ้ชนะหลักตัดสินจาก **คะแนนรวม (Score-based)** ไม่ใช่เหลือผู้รอดชีวิตคนสุดท้ายเท่านั้น
7. ช่วงเริ่มต้น **ไม่ใช้ Database** ให้ Static Game Data อยู่ใน JSON และ Runtime Game State อยู่ใน RAM ของ Go Server เท่านั้น อาจเขียนผลเกมที่จบแล้วลง CSV เป็นทางเลือกภายหลัง
8. ภาพในเกมใช้ **Card-like UI / Panel / Icon / Static Image** เพื่อแสดง Room, Monster, Item และ Player แทนการทำ 3D Model หรือ Character Model และไม่ต้องทำ Animation ใน MVP
9. การ์ดเป็นเพียงรูปแบบการนำเสนอ UI **ไม่ใช่ระบบ Card Battler / Deckbuilding** ไม่มีการจั่วการ์ดหรือใช้การ์ดเป็น Action หลัก
10. Stack ที่กำหนดไว้คือ **React + TypeScript + Vite** ฝั่ง Frontend, **Go + Fiber** ฝั่ง Backend และ **WebSocket** สำหรับ Multiplayer

## 0.2 ขอบเขตที่ห้ามหลุดใน MVP

- ห้ามเพิ่มระบบ Login, OAuth, ระบบเพื่อน, Trading, Guild, Ranking ถาวร หรือ Database โดยที่ยังไม่จำเป็น
- ห้ามเริ่มจาก 3D, skeletal animation, physics หรือระบบเดินตัวละครแบบ Real-time
- ห้ามเปลี่ยนเกมเป็น Card Battler
- ห้ามให้ Frontend เป็นผู้ตัดสิน HP, Damage, Loot, Score หรือผลการต่อสู้เอง ทุกอย่างต้อง Validate และคำนวณที่ Server
- ห้ามทำระบบใหญ่ทั้งหมดในครั้งเดียว ให้พัฒนาตาม Phase ในเอกสารและทดสอบก่อนข้าม Phase
- อย่าเพิ่ม Dependency โดยไม่มีเหตุผล และอย่าเพิ่ม Abstraction ที่ทำให้ MVP ซับซ้อนเกินจำเป็น
- ถ้ามีรายละเอียดใดที่ยังไม่ได้กำหนด ให้ใช้ค่าเริ่มต้นในเอกสารนี้ พร้อมแยก Config ให้ง่ายต่อการปรับภายหลัง

## 0.3 วิธีทำงานที่ต้องปฏิบัติ

1. ตรวจสอบ Repository ปัจจุบันก่อนแก้ไข ห้ามสมมติว่าเป็นโปรเจกต์ว่าง
2. สรุปโครงสร้างและแผนการเปลี่ยนแปลงสั้น ๆ ก่อนเริ่มแก้โค้ดในแต่ละ Phase
3. ทำทีละ Feature ให้ Compile / Type-check / Test ผ่านก่อน
4. เขียน Unit Test ให้ Game Rules ที่มีผลต่อความถูกต้อง เช่น Damage, Respawn, Loot, Score และ Turn validation
5. ใช้ Server-authoritative design เท่านั้น
6. ทุก WebSocket command ต้อง Validate ผู้เล่น, Room, Phase, State, Request ID และเงื่อนไขของ Action
7. ป้องกันการกด Action ซ้ำจาก Network Retry ด้วย `request_id` หรือ Idempotency Key
8. เขียน Go code พร้อม Comment อธิบายส่วนสำคัญ โดยเฉพาะ Exported Types / Functions และ Game Rules
9. รายงานไฟล์ที่สร้างหรือแก้ไข, วิธีรัน, Test ที่ผ่าน และสิ่งที่ยังไม่ทำทุกครั้ง
10. ห้ามอ้างว่าระบบทำงานแล้ว หากยังไม่ได้รันหรือทดสอบจริง

---

# 1. Product Vision

## 1.1 Working Title

**Dungeon Rivals** (ชื่อชั่วคราว เปลี่ยนภายหลังได้)

## 1.2 Elevator Pitch

Dungeon Rivals คือเว็บเกม Multiplayer สำหรับผู้เล่น 2–4 คน ทุกคนเริ่มผจญภัยใน Dungeon รอบเดียวกัน แต่มีเส้นทางและเหตุการณ์ของตัวเอง ผู้เล่นเลือกเส้นทาง ต่อสู้กับ Monster สำรวจห้องสมบัติ ซื้อของ และสะสมอุปกรณ์เพื่อสร้างความได้เปรียบ จากนั้นนำอุปกรณ์ที่หาได้ไปแข่งขันกันใน Final PvP โดยตัดสินจากคะแนนรวมที่ได้จากการผจญภัยและการต่อสู้

## 1.3 Genre และรูปแบบ

- Competitive Dungeon Roguelite
- Dungeon Crawler
- PvE + PvP
- Turn-based Multiplayer
- Node-based Exploration
- Loot and Equipment Progression
- Card-like UI Presentation

## 1.4 เป้าหมายประสบการณ์ผู้เล่น

- เข้าใจวิธีเล่นได้เร็ว ไม่ต้องเรียนรู้ระบบการ์ดซับซ้อน
- การเลือกเส้นทางมีความหมาย: เส้นทางปลอดภัยให้รางวัลน้อย เส้นทางเสี่ยงให้โอกาสได้ของดี
- ผู้เล่นแต่ละคนมีอิสระ ไม่ต้องรอกันทุก Action
- การเก็บของและการจัดอุปกรณ์ทำให้ Build ของแต่ละคนแตกต่างกัน
- การตายไม่ทำให้ผู้เล่นออกจากเกม แต่มีต้นทุนให้ต้องระวัง
- Final PvP เปิดโอกาสให้ผู้เล่นทำคะแนนจากหลายทาง ไม่ใช่เฉพาะผู้เล่นที่เก่ง PvP เท่านั้น
- เกมหนึ่งรอบมีจุดเริ่ม จุดจบ และสรุปคะแนนชัดเจน

---

# 2. ข้อกำหนดผลิตภัณฑ์ (Product Requirements)

## 2.1 ข้อกำหนดที่ผู้ใช้เลือกแล้ว (Confirmed Decisions)

| เรื่อง | ข้อสรุป |
|---|---|
| Platform | Web Browser |
| Player count | 2–4 คนต่อห้อง |
| Turn model | Independent Turns — ผู้เล่นทำ Action ของตัวเองได้โดยไม่ต้องรอคนอื่น |
| Dungeon route | แยกเส้นทาง ผู้เล่นเลือกทางของตัวเอง |
| Main activity | สำรวจ Dungeon, สู้ Monster, เก็บ Item / Equipment |
| Death | Respawn กลับมาเล่นต่อ |
| Final phase | PvP |
| Winner | ตัดสินจาก Score รวม |
| Visual direction | Card-like UI / Panel / Static Icon; ไม่มี Model ใน MVP |
| Frontend | React |
| Backend | Go |
| Multiplayer transport | WebSocket |
| Persistence | ไม่มี Database ใน MVP; Static JSON + Runtime RAM; CSV เป็น Optional |

## 2.2 ค่าเริ่มต้นที่กำหนดเพิ่มเพื่อให้เริ่มพัฒนาได้ (Proposed MVP Defaults)

รายการต่อไปนี้เป็นค่าเริ่มต้นเชิงออกแบบ ไม่ใช่ข้อกำหนดที่ผู้ใช้ยืนยันแยกทีละค่า ให้แยกไว้ใน Config และปรับได้

| เรื่อง | ค่าเริ่มต้น |
|---|---|
| Player count | 2–4 |
| Dungeon Floors | 3 Floors |
| Map | Branching Node Map แยกต่อผู้เล่น |
| จำนวนชั้นเส้นทาง | 5 Depth ต่อ Floor (Start + 3 Decision Depth + Boss) |
| Normal / Special Rooms | Monster, Elite, Treasure, Shop, Event, Rest, Boss |
| Base HP | 100 |
| Starting Gold | 20 |
| Starting Equipment | Training Sword, Cloth Armor |
| Starting Consumable | Health Potion x1 |
| Backpack capacity | 12 Slots (Stackable Item นับตาม Stack) |
| Equipment slots | Weapon 1, Armor 1, Helmet 1, Accessory 2 |
| Consumable quick slots | 3 (อ้างอิง Item ใน Backpack) |
| PvP Actions | Round-Robin ทุกคู่; แต่ละคู่สู้จนผู้เล่นฝ่ายหนึ่ง HP หมด |
| PvP action resolution | สลับ Turn ระหว่างคู่ต่อสู้; Server ปฏิเสธ Action จากผู้เล่นที่ยังไม่ถึง Turn |
| Inactivity handling | MVP ใช้ Reconnect และ Host Kick / Force Finish; ยังไม่บังคับเวลาต่อ Turn |
| Result persistence | ไม่บันทึกถาวรในรอบแรก; เพิ่ม CSV หลัง Game Result ทำงานแล้ว |

**หมายเหตุเรื่อง Independent Turns:** ใน Dungeon ผู้เล่นแต่ละคนมี Progress และ Encounter ของตัวเอง การกระทำของผู้เล่นหนึ่งไม่บังคับให้คนอื่นต้องส่ง Action ก่อน ใน Final PvP ก็ใช้ระบบ Action อิสระเช่นกัน แต่ Server จะ Serialize การเปลี่ยน State เพื่อไม่ให้คำสั่งที่มาพร้อมกันเขียนข้อมูลชนกัน จำนวน Action ของแต่ละคนจำกัดเท่ากันเพื่อควบคุมความยุติธรรม

---

# 3. Core Game Loop

## 3.1 ภาพรวมหนึ่ง Match

```text
CREATE / JOIN ROOM
        |
        v
      LOBBY
        |
        | Host starts when 2-4 players are ready
        v
  DUNGEON EXPLORATION
        |
        | Each player explores their own path independently
        | - Choose a node
        | - Resolve room / combat
        | - Get loot / gold / score
        | - Move to next available node
        v
  FLOOR BOSS / NEXT FLOOR
        |
        | Repeat until all players clear final Dungeon Boss
        v
    PREPARATION
        |
        | Equip items / manage inventory / ready up
        | Restore HP to full for fair PvP start (MVP rule)
        v
     FINAL PVP
        |
        | Each player has 5 independent actions
        | Attack / Defend / Use Item
        | Damage, KO and other PvP events generate score
        v
      RESULTS
        |
        | Final score table + winner + score breakdown
        v
    ROOM COMPLETE
```

## 3.2 Phase Definitions

### A. Lobby

- ผู้เล่นสร้างห้องหรือเข้าห้องด้วย Room Code
- Room รองรับ 2–4 คน
- ผู้เล่นตั้ง Display Name ได้
- ทุกคนกด Ready
- Host เท่านั้นที่กด Start ได้
- เริ่มเกมได้เมื่อมีผู้เล่นอย่างน้อย 2 คนและทุกคน Ready
- หลังเริ่มเกมแล้วห้าม Join เพิ่มใน MVP

### B. Dungeon Exploration

- ทุก Player มี `DungeonRun` ของตัวเอง
- Map ของแต่ละ Player แยกกันและสุ่มตาม Seed ที่ Server สร้าง
- ผู้เล่นเลือกเฉพาะ Node ที่เชื่อมต่อกับ Node ปัจจุบันและมีสถานะ Available
- เมื่อเลือก Node แล้ว Server สร้าง Encounter ของ Node นั้น
- ผู้เล่นอื่นยังเลือกเส้นทางหรือเล่น Combat ของตัวเองได้ตามปกติ
- Combat เป็น Turn-based ระหว่าง Player กับ Monster: Player Action หนึ่งครั้ง แล้ว Monster ตอบโต้หากยังไม่ตาย
- เมื่อ Encounter จบ จึงเปิดทางให้เลือก Node ถัดไป
- Floor สุดท้ายมี Boss เป็นจุดจบของ Dungeon

### C. Preparation

- เมื่อผู้เล่นคนหนึ่งเคลียร์ Final Boss ของตัวเอง จะเข้าสถานะ `PREPARATION`
- ผู้เล่นสามารถจัด Equipment, ตรวจ Inventory และกด Ready
- ผู้เล่นที่ยังอยู่ใน Dungeon เล่นต่อได้ ไม่ต้องหยุดทั้งห้อง
- เมื่อผู้เล่นทุกคนที่ยังอยู่ใน Match อยู่ใน Preparation และ Ready จึงเริ่ม Final PvP
- ผู้เล่นที่หลุดการเชื่อมต่อยังคงอยู่ใน Room และสามารถ Reconnect ได้
- MVP ให้ Host Kick ได้เฉพาะผู้เล่นที่ Disconnected / ออกจากเกมถาวรระหว่าง Match เท่านั้น; ห้าม Host Kick ผู้เล่นที่ยัง Connected และกำลังเล่นอยู่ เพื่อป้องกันการ Abuse
- ก่อน PvP ให้ Restore HP เป็น Max HP เพื่อให้การเสีย HP จาก Dungeon ไม่กลายเป็นความเสียเปรียบที่มากเกินไป แต่ไอเทม Consumable ที่ใช้ไปแล้วไม่คืน

### D. Final PvP

- ผู้เล่นทุกคนเข้าสู่ Arena เดียวกัน
- แต่ละคนมี 5 PvP Actions
- ผู้เล่นเลือก Action ได้โดยไม่ต้องรอให้ทุกคนเลือกพร้อมกัน
- Server รับ Action แล้ว Resolve ทันทีตามลำดับที่ได้รับ
- Action ที่ใช้ได้ใน MVP: Attack, Defend, Use Item
- Attack ต้องเลือก Target Player ที่ยังอยู่ใน Match
- Defend สร้าง Guard สำหรับลด Damage จากการโจมตีครั้งถัดไป
- Use Item ใช้ Consumable ที่มีอยู่จริงใน Inventory
- เมื่อ HP เหลือ 0 ให้ Respawn ใน Arena ด้วย HP 50% และหักคะแนน PvP 10 คะแนน; ผู้เล่นไม่ถูกตัดออก
- จบ Phase เมื่อผู้เล่นทุกคนใช้ Action ครบ 5 ครั้ง หรือ Host ใช้ Force Finish (กรณีจำเป็น)
- ผู้ชนะคือผู้มี Total Score สูงสุด ไม่ใช่ผู้ที่ HP เหลือมากที่สุด

### E. Results

- แสดงอันดับตาม Total Score จากมากไปน้อย
- แสดง Score Breakdown ของ PvE, PvP, Gold Conversion และ Penalty
- แสดงสถิติ เช่น Monsters Defeated, Bosses Defeated, Treasure Found, PvP Damage, Knockouts, Deaths
- ผู้เล่นสามารถกลับ Lobby ใหม่หรือสร้าง Room ใหม่ได้

---

# 4. Dungeon Map Design

## 4.1 รูปแบบ Map

ใช้ Node-based Branching Map คล้ายโครงสร้างเส้นทาง Roguelike ไม่ต้องวาดฉาก Dungeon จริงใน MVP

แต่ละ Floor มี 5 Depth:

- Depth 0: Start Node (จุดเริ่มต้น)
- Depth 1: มี 2 Node ให้เลือก
- Depth 2: มี 2–3 Node ให้เลือก
- Depth 3: มี 2 Node ให้เลือก
- Depth 4: Boss Node (ทางออกของ Floor)

ผู้เล่นจะเดินผ่านหนึ่ง Node ต่อหนึ่ง Depth ตามเส้นทางที่เชื่อมต่อกัน ไม่สามารถข้าม Depth หรือย้อนกลับไป Node ที่ Clear แล้วได้ใน MVP

มี 3 Floors รวมเป็นเส้นทางหลักประมาณ 12 Room Encounters ต่อคน (3 Decision Nodes + 1 Boss ต่อ Floor) ไม่รวม Start Node ทั้งนี้ Map Generator ต้องสร้างจำนวน Node และ Connection ให้ผู้เล่นทุกคนมีจำนวน Decision Depth เท่ากัน

## 4.2 Room Types

| Room Type | Function | Reward / Effect |
|---|---|---|
| MONSTER | ต่อสู้ Monster ปกติ | Gold, Item Chance, PvE Score |
| ELITE | ต่อสู้ Monster แข็งแกร่ง | Gold / Loot ดีกว่า, Score มากกว่า |
| TREASURE | เปิดหีบสมบัติ | เลือก Item หรือรับ Gold |
| SHOP | ซื้อ Item | ใช้ Gold แลก Item |
| EVENT | เหตุการณ์พิเศษ | เลือก 1 ใน 2–3 ตัวเลือก |
| REST | ฟื้นฟู | Heal HP บางส่วน หรือเอา Debuff ออก |
| BOSS | Boss ประจำ Floor | Reward และเปิด Floor ถัดไป |

## 4.3 Room Distribution (Configurable)

ค่าเริ่มต้นสำหรับ Node ปกติ (ไม่รวม Boss ที่กำหนดตายตัว):

| Type | Weight |
|---|---:|
| MONSTER | 40 |
| TREASURE | 15 |
| SHOP | 10 |
| EVENT | 10 |
| REST | 10 |
| ELITE | 15 |
| **รวม** | **100** |

Generator ควรมี Guarantee แบบง่าย เพื่อไม่ให้ Floor ใดไม่มีโอกาสพักหรือได้รับ Item เลย เช่น อย่างน้อยหนึ่ง Rest หรือ Treasure ในแต่ละ Floor หากจำนวน Node เอื้ออำนวย

## 4.4 Difficulty Scaling

ค่าเริ่มต้น:

- Floor 1: Monster base stats x 1.0
- Floor 2: Monster base stats x 1.25
- Floor 3: Monster base stats x 1.5

ให้ใช้ Config multiplier ไม่ Hardcode ใน Combat Service

## 4.5 Node State

Node แต่ละตัวมีสถานะ:

- `LOCKED`: ยังไปไม่ได้
- `AVAILABLE`: เลือกได้
- `IN_PROGRESS`: กำลังทำ Encounter
- `CLEARED`: ผ่านแล้ว
- `SKIPPED`: หนีหรือข้ามโดยมีผลตามกติกา
- `FAILED`: Encounter ล้มเหลวและยังไม่ได้รับ Reward; สำหรับ Respawn ใน MVP ให้คืนเป็น `AVAILABLE` เมื่อผู้เล่นกลับ Checkpoint เพื่อให้ Retry ได้

Server ต้องเป็นผู้กำหนด Connection และ State ของ Node ห้ามรับ Node list จาก Client มาเชื่อถือโดยตรง

## 4.6 Map Generation Algorithm

1. Server สร้าง Random Seed ต่อ Player ต่อ Match
2. สร้าง Nodes ตาม Depth Template
3. สร้าง Connection ให้แต่ละ Node เชื่อมกับ Node ใน Depth ถัดไปอย่างน้อยหนึ่ง Node
4. กำหนด Boss Node ใน Depth สุดท้าย
5. กำหนด Room Type ด้วย Weighted Random
6. ใส่ Guarantee Rules เช่นมีทางเลือกมากกว่าหนึ่งทางใน Depth ที่กำหนด และมีโอกาสได้รับ Rest / Treasure
7. เก็บ Map ทั้งหมดใน Player Dungeon State ใน RAM
8. ส่งเฉพาะ Map ของ Player คนนั้นใน Private Snapshot ส่วนผู้เล่นอื่นเห็นเพียง Floor / Progress / Status / Score ที่เป็นข้อมูลสาธารณะ

---

# 5. Turn-based และ Encounter Rules

## 5.1 Independent Turns

- ไม่มี Global Turn Queue ใน Dungeon
- Player A สามารถเลือก Node ได้ ขณะที่ Player B กำลังต่อสู้ และ Player C กำลังอยู่ใน Shop
- ทุกคำสั่งของผู้เล่นแต่ละคนถูกประมวลผลแยกตาม State ของคนนั้น
- Backend ใช้ Room Actor หรือ Room Mutex เพื่อ Serialize การแก้ไข State ใน RAM ให้ปลอดภัยจาก Concurrent Requests
- การ Serialize ใน Server มีไว้ป้องกัน Data Race ไม่ได้แปลว่าผู้เล่นต้องรอ Turn กัน

## 5.2 Monster Combat

Combat เป็นการสลับ Action ระหว่าง Player และ Monster ภายใน Encounter ของ Player คนนั้นเท่านั้น

ลำดับ:

1. Player ส่ง Action
2. Server Validate ว่า Player อยู่ใน Combat และเป็น Player Turn
3. Server Execute Action
4. ถ้า Monster HP เหลือ 0: Combat จบ, แจก Reward, เพิ่ม Score
5. ถ้า Monster ยังอยู่: Monster ใช้ AI Action
6. คำนวณ Damage ใส่ Player
7. ถ้า Player HP เหลือ 0: เรียก Respawn Flow
8. หากยังไม่ตาย ให้กลับเป็น Player Turn

## 5.3 Action ที่มีใน MVP

| Action | ผล |
|---|---|
| ATTACK | โจมตี Monster ด้วย Total ATK |
| DEFEND | ลด Damage จากการโจมตีถัดไปของ Monster 50% |
| USE_ITEM | ใช้ Consumable ที่ใช้ใน Combat ได้ เช่น Health Potion |
| ESCAPE | มีโอกาสหนี 50%; สำเร็จแล้ว Node ถูก Mark เป็น SKIPPED และไม่ได้รับ Reward; ล้มเหลว Monster โจมตีกลับ |

สำหรับ MVP Monster AI มี Basic Attack ก่อน ยังไม่ต้องมี Skill Tree หรือ Ability ซับซ้อน

## 5.4 Damage Formula

คำนวณค่าจาก Base Stats + Equipment + Buff:

```text
TotalATK = BaseATK + WeaponATK + AccessoryATK + BuffATK
TotalDEF = BaseDEF + ArmorDEF + HelmetDEF + AccessoryDEF + BuffDEF

RawDamage = AttackerATK - DefenderDEF
Damage = max(1, RawDamage)
```

เมื่อมี Defend Guard:

```text
DamageAfterGuard = floor(Damage * 0.5)
FinalDamage = max(1, DamageAfterGuard)
```

กติกา:

- Damage ต่ำสุด 1 เมื่อโจมตีสำเร็จ
- HP ต้องไม่ต่ำกว่า 0
- Server คำนวณ Damage เท่านั้น
- Client ส่งเพียง Action Type และ Target/Item ID ที่ต้องการ
- Random Critical หรือ Dodge ยังไม่อยู่ใน MVP เพื่อลดความซับซ้อน

## 5.5 Escape

- Base success chance = 50%
- เมื่อสำเร็จ: Player ได้ออกจาก Combat, Node เป็น `SKIPPED`, ไม่ได้ Gold / Loot / Kill Score
- เมื่อไม่สำเร็จ: Monster โจมตีกลับหนึ่งครั้ง
- ไม่สามารถ Escape จาก Boss ได้ใน MVP

---

# 6. Death, Respawn และ Checkpoint

## 6.1 PvE Respawn

เมื่อ Player HP <= 0 ใน Dungeon:

1. เพิ่ม `Deaths` 1
2. หัก Gold 20% (ปัดลง)
3. สุ่มลบ Backpack Item 1 ชิ้น หากมี (ห้ามสุ่มลบ Equipped Item; Consumable stack อาจลด 1 ชิ้น)
4. กลับไปยัง Checkpoint ซึ่งคือ Node ล่าสุดที่ Clear ใน Floor ปัจจุบัน (ถ้ายังไม่เคย Clear ให้ใช้ Start Node)
5. Restore HP เป็น 50% ของ MaxHP โดยปัดขึ้น และไม่น้อยกว่า 1
6. ล้าง Combat State และ Encounter ที่กำลังทำอยู่
7. Node ที่กำลังต่อสู้ยังไม่ถือว่า Clear และกลับเป็น `AVAILABLE` เพื่อให้ผู้เล่นเลือกเข้าไปลองใหม่ได้ โดยไม่แจก Reward หรือ Score จากการพยายามครั้งก่อน
8. Player สามารถเลือก Node ที่เชื่อมต่อจาก Checkpoint ได้อีกครั้งตาม Graph Rules

**Checkpoint:** เป็น Start Node ก่อนผ่าน Encounter แรก และอัปเดตเป็น Node ล่าสุดที่ Clear ทุกครั้งที่ผ่านห้องสำเร็จ หากตายจะกลับ Checkpoint ล่าสุดใน Floor ปัจจุบัน ไม่ย้อนกลับไป Floor ก่อนหน้า

## 6.2 PvP Respawn

เมื่อ Player HP <= 0 ใน Arena:

1. เพิ่ม Death Count
2. หัก PvP Score 10 คะแนน (Score รวมต้องไม่ต่ำกว่า 0 จาก Penalty นี้)
3. Respawn ใน Arena ด้วย HP 50% ของ MaxHP
4. เก็บ Equipment เดิมไว้
5. Player ยังคงเหลือ PvP Actions ตามจำนวนที่ยังไม่ได้ใช้
6. ไม่แจก Kill Score ซ้ำจาก Event ID เดิม

## 6.3 ป้องกันการ Abuse

- Respawn ต้องเป็น Server-side transition ไม่รับค่า HP ใหม่จาก Client
- การแจก Death Penalty และการลบ Item ต้องเกิดครั้งเดียวต่อ Death Event
- ทุก Reward และ Score Event ต้องมี unique event ID เพื่อป้องกันการบันทึกซ้ำ

---

# 7. Item, Equipment และ Inventory

## 7.1 Item Categories

| Category | ตัวอย่าง | Function |
|---|---|---|
| WEAPON | Training Sword, Iron Sword, Staff, Bow | เพิ่ม ATK |
| ARMOR | Cloth Armor, Iron Armor | เพิ่ม DEF |
| HELMET | Leather Cap, Iron Helmet | เพิ่ม DEF หรือ HP |
| ACCESSORY | Ring, Amulet | เพิ่ม Stat หรือ Passive Effect |
| CONSUMABLE | Health Potion, Bomb | ใช้แล้วหมดไป |

## 7.2 Equipment Slots

- Weapon: 1
- Armor: 1
- Helmet: 1
- Accessory: 2

## 7.3 Inventory Rules

- Backpack capacity = 12 slots
- Item ที่ `stackable=true` รวม Stack ได้จนถึง `max_stack`
- Equipment ที่ยังไม่สวมใส่เก็บใน Backpack
- Equipment Slot มีของได้ 1 ชิ้น ยกเว้น Accessory มี 2 ช่อง
- Consumable Quick Slots มี 3 ช่อง แต่เป็น Shortcut อ้างอิง Item ใน Backpack ไม่ใช่ Item Copy
- เมื่อ Backpack เต็ม ต้องปฏิเสธ Loot หรือให้ผู้เล่นเลือกทิ้ง Item ก่อนรับของ ห้ามลบ Item อัตโนมัติโดยไม่แจ้งผู้เล่น

## 7.4 Rarity

- COMMON
- RARE
- EPIC
- LEGENDARY

ค่า Drop Weight เริ่มต้น (ปรับได้):

| Rarity | Weight |
|---|---:|
| COMMON | 70 |
| RARE | 20 |
| EPIC | 8 |
| LEGENDARY | 2 |

## 7.5 Starting Player Setup

ค่าเริ่มต้น:

- Base HP = 100
- Base ATK = 5
- Base DEF = 0
- Gold = 20
- Equipment: Training Sword (+3 ATK), Cloth Armor (+1 DEF)
- Consumable: Health Potion x1 (Heal 20)
- Score = 0

Total ATK และ DEF คำนวณจาก Base + Equipment + Buff เสมอ ไม่บันทึก Total Stat ที่ Client ส่งมา

## 7.6 Item Data Schema (JSON)

ตัวอย่าง `backend/data/items.json`:

```json
[
  {
    "id": "training_sword",
    "name": "Training Sword",
    "description": "A basic sword for new adventurers.",
    "type": "weapon",
    "rarity": "common",
    "slot": "weapon",
    "stats": {
      "attack": 3,
      "defense": 0,
      "max_hp": 0
    },
    "effect": null,
    "price": 10,
    "stackable": false,
    "max_stack": 1,
    "usable_in_combat": false
  },
  {
    "id": "health_potion",
    "name": "Health Potion",
    "description": "Restore 20 HP.",
    "type": "consumable",
    "rarity": "common",
    "slot": null,
    "stats": {
      "attack": 0,
      "defense": 0,
      "max_hp": 0
    },
    "effect": {
      "type": "heal",
      "value": 20
    },
    "price": 15,
    "stackable": true,
    "max_stack": 5,
    "usable_in_combat": true
  }
]
```

ข้อกำหนด:

- JSON เป็น Static Definition เท่านั้น
- Player Inventory เก็บ Item ID + Quantity ไม่ Copy Definition ทั้งก้อน
- Server โหลด JSON ตอนเริ่มโปรแกรมและ Validate ข้อมูลก่อนเปิด Server
- หาก JSON ไม่ถูกต้อง ให้ Server แจ้ง Error ชัดเจนและไม่ Start แบบข้อมูลไม่ครบ

## 7.7 Equip / Unequip

- Equip ได้เฉพาะ Item ประเภท Equipment
- Item Slot ต้องตรงกับ Equipment Slot
- ถ้า Slot เต็ม ให้ Swap Item เก่ากลับ Backpack เมื่อมีที่ว่าง
- ถ้า Backpack เต็มและไม่มีที่ว่างสำหรับ Item ที่ถูกถอด ห้าม Equip และแจ้ง Error
- Unequip ย้าย Item กลับ Backpack
- ทุกคำสั่งตรวจว่า Item อยู่ใน Inventory ของ Player จริง

---

# 8. Monster, Treasure, Shop และ Event

## 8.1 Monster Definition

ตัวอย่าง `backend/data/monsters.json`:

```json
[
  {
    "id": "goblin",
    "name": "Goblin",
    "description": "A small but aggressive dungeon creature.",
    "base_hp": 25,
    "base_attack": 6,
    "base_defense": 1,
    "gold_min": 5,
    "gold_max": 10,
    "score_reward": 10,
    "loot_table": "common_monster_loot",
    "tier": "normal"
  },
  {
    "id": "skeleton_guard",
    "name": "Skeleton Guard",
    "description": "An old warrior guarding the dungeon.",
    "base_hp": 35,
    "base_attack": 8,
    "base_defense": 2,
    "gold_min": 8,
    "gold_max": 14,
    "score_reward": 10,
    "loot_table": "common_monster_loot",
    "tier": "normal"
  }
]
```

Floor Multiplier ปรับ HP / ATK / DEF ตาม Config เมื่อสร้าง Encounter เท่านั้น เพื่อให้ Encounter ที่สร้างแล้วไม่เปลี่ยนค่าเมื่อ Config ถูก Reload

## 8.2 MVP Monster Pool

เริ่มต้นอย่างน้อย:

- Goblin: HP ต่ำ, ATK ต่ำ
- Skeleton Guard: HP กลาง, DEF เล็กน้อย
- Slime: HP กลาง, ATK ต่ำ
- Orc Warrior: HP สูง, ATK สูง
- Elite Knight: HP/DEF สูง
- Floor Boss 3 ตัว (หนึ่งตัวต่อ Floor)

เริ่มด้วย AI โจมตีธรรมดา และค่อยเพิ่ม Special Attack หลัง Combat พื้นฐานเสถียร

## 8.3 Treasure

- เปิด Treasure Room แล้ว Server สุ่ม Reward
- MVP ให้ผู้เล่นเลือก 1 ใน 2–3 ตัวเลือก หรือเลือก Gold แทนได้
- เพิ่ม Treasure Score เมื่อเปิดสำเร็จครั้งเดียว
- ต้องมี Flow กรณี Backpack เต็ม

## 8.4 Shop

- Stock สุ่มจาก Item Pool ที่อนุญาตใน Floor นั้น
- Server เป็นผู้ตรวจ Gold และ Stock
- Purchase เป็น Atomic Operation: ตรวจราคา → ตรวจ Stock → ตรวจ Inventory capacity → หัก Gold → เพิ่ม Item → ลด Stock
- หากขั้นตอนใดไม่ผ่าน ต้องไม่หัก Gold หรือเปลี่ยน Stock
- MVP ยังไม่ต้องมีระบบขาย Item หากจะทำให้ Scope บาน สามารถเพิ่มหลัง Purchase เสถียร

## 8.5 Event

- Event แต่ละชนิดมีตัวเลือก 2–3 แบบ
- ตัวอย่าง: จ่าย Gold เพื่อรับ Item, เสี่ยงเลือดเพื่อรับ Reward, เลือก Heal หรือ Gold
- ผลลัพธ์ต้องกำหนดใน JSON หรือ Service แบบ Data-driven เท่าที่ไม่ทำให้ Logic ซับซ้อนเกินไป
- Server ตรวจ Choice ID ว่าเป็นตัวเลือกของ Event ปัจจุบันจริง
- Event เดียวรับ Reward ได้ครั้งเดียว

## 8.6 Rest

- Heal เป็นเปอร์เซ็นต์ของ Max HP เช่น 30%
- ไม่สามารถ Heal เกิน Max HP
- Rest ไม่มี Combat

---

# 9. Score System

## 9.1 หลักการ

- คะแนนต้องมาจาก Server เท่านั้น
- Score เป็นผลรวมของ Dungeon Performance + PvP Performance + Gold Conversion - Penalties
- ผู้เล่นสามารถทำคะแนนได้แม้ไม่ได้ KO ผู้เล่นคนอื่น
- ทุกการเพิ่ม/ลดคะแนนบันทึกเป็น Score Event ภายใน RAM เพื่อแสดง Breakdown และป้องกันการคิดซ้ำ

## 9.2 Proposed MVP Score Table

### PvE Score

| Event | Score |
|---|---:|
| Defeat Normal Monster | +10 |
| Defeat Elite Monster | +25 |
| Defeat Floor Boss | +50 |
| Open Treasure Room | +5 |
| Complete Event Successfully | +10 |
| Complete a Floor | +20 |
| Clear Final Dungeon | +40 |

### PvP Score

| Event | Score |
|---|---:|
| Deal PvP Damage | +1 per 5 total damage |
| Knockout another Player | +30 |
| Finish Final PvP with highest PvP score | +50 |
| Die in PvP | -10 |

`+1 per 5 damage` ให้สะสมเศษ Damage ต่อ Attacker เพื่อไม่ให้การโจมตี 1–4 Damage สูญเปล่าตลอด เช่น `pvp_damage_remainder` และเมื่อครบ 5 จึงเพิ่ม 1 คะแนน

### PvP Performance Bonus

หลังคำนวณ PvP Action Points จาก Damage และ Knockout แล้ว ให้ผู้เล่นที่มี PvP Action Points สูงสุดได้รับโบนัส +50 คะแนน หากมีผู้เล่นเสมอกันในอันดับสูงสุด ให้ทุกคนที่เสมอกันได้รับ +50 เพื่อไม่ให้ผลลัพธ์ขึ้นกับลำดับการประมวลผล จากนั้นจึงคำนวณ Total Score และตัดสินผู้ชนะรวม

### Gold Conversion

เมื่อจบ PvP:

```text
GoldScore = floor(RemainingGold / 10)
```

## 9.3 Total Score

```text
TotalScore = PvEScore + PvPScore + GoldScore - Penalties
```

แนะนำให้เก็บ Score Breakdown แยกประเภท ไม่เก็บเพียง TotalScore อย่างเดียว

## 9.4 Tie-breaker

เมื่อ Total Score เท่ากัน ให้ตัดสินตามลำดับ:

1. PvP Score สูงกว่า
2. PvP Damage รวมสูงกว่า
3. Bosses Defeated มากกว่า
4. Gold คงเหลือมากกว่า
5. หากยังเท่ากัน ให้เสมอกัน

## 9.5 Score Event Model

```go
// ScoreEvent records one score change so rewards cannot be applied twice.
type ScoreEvent struct {
    ID        string    `json:"id"`
    PlayerID  string    `json:"player_id"`
    Category  string    `json:"category"` // pve, pvp, economy, penalty
    Source    string    `json:"source"`   // monster_kill, treasure, pvp_damage, etc.
    Points    int       `json:"points"`
    CreatedAt time.Time `json:"created_at"`
}
```

ก่อนเพิ่ม Score Event ให้ตรวจ Event ID ว่าเคยถูกใช้แล้วหรือไม่

---

# 10. Final PvP Design

## 10.1 Arena Rules

- ผู้เล่นทุกคนเข้าสู่ Arena หลัง Ready ครบ
- HP ถูก Restore เป็น Max HP ณ จุดเริ่ม PvP
- Equipment และ Consumables ที่เหลือจาก Dungeon ติดตัวไปด้วย
- จัดคู่ Round-Robin ให้ผู้เล่นทุกคู่ได้ต่อสู้กัน
- ผู้เล่น SPD สูงกว่าได้เริ่มก่อน; หาก SPD เท่ากันสุ่มผู้เริ่ม แล้วสลับ Turn จนผู้เล่นฝ่ายหนึ่ง HP หมด
- บันทึกผลชนะ/แพ้และ Knockout หลังจบแต่ละคู่
- ฟื้นฟู HP/MP เต็มก่อนเริ่มคู่ถัดไป เพื่อให้ทุกคู่เริ่มต้นอย่างยุติธรรม
- เมื่อทุกคู่จบแล้วจึงรวมคะแนนและประกาศอันดับ ไม่มีรอบตัดสินพิเศษเพิ่ม

## 10.2 PvP Actions

| Action | Rule |
|---|---|
| ATTACK | โจมตี Target หนึ่งคนด้วย Total ATK |
| DEFEND | ตั้ง Guard ลด Damage จากการโจมตีครั้งถัดไป 50% |
| USE_ITEM | ใช้ Item ที่ `usable_in_combat=true` และมีอยู่จริง |

ทุก Action ใช้ Action Budget 1 ครั้ง ยกเว้น Request ที่ไม่ผ่าน Validation ซึ่งต้องไม่หัก Action Budget

## 10.3 Match End

- จบเมื่อทุกคู่ในตาราง Round-Robin แข่งจนมีผู้เล่นฝ่ายหนึ่ง HP หมด
- Host สามารถ Force Finish ได้เฉพาะเมื่อมีเหตุจำเป็น เช่น Player หลุดถาวรและ Room ค้าง โดยต้องแสดงผลคะแนนตาม State ที่ Server ยืนยัน ณ เวลานั้น
- เมื่อจบ ให้รวม Score และประกาศอันดับ
- ไม่จำเป็นต้องเหลือ Player คนสุดท้าย

## 10.4 หมายเหตุเรื่องความยุติธรรม

PvP เป็นการต่อสู้แบบ Turn-based ภายในคู่ปัจจุบันเท่านั้น ผู้เล่น SPD สูงกว่าได้เริ่มก่อนและสุ่มผู้เริ่มเมื่อ SPD เท่ากัน Server ตรวจสอบผู้เล่นที่ได้ Turn ทุกครั้งเพื่อป้องกันการกด Action ซ้ำก่อนคู่ต่อสู้ตอบกลับ การต่อสู้จบเมื่อฝ่ายใดฝ่ายหนึ่ง HP หมด และดำเนินต่อจนครบทุกคู่ในตาราง Round-Robin ก่อนคำนวณอันดับรวม

---

# 11. UI / UX Design

## 11.1 Visual Direction

- Dark Fantasy / Dungeon Adventure
- Card-like Panels สำหรับ Room, Monster, Item และ Player
- ใช้ Icon, Static Image หรือ Pixel Art ภาพนิ่ง
- ไม่มี 3D Model, Rigged Character หรือ Animation ใน MVP
- ใช้สีแยก Rarity และสถานะ แต่ต้องอ่านง่าย
- Layout Responsive สำหรับ Desktop เป็นหลัก และใช้งานบน Tablet / Mobile ได้
- ไม่ต้องทำ UI เหมือนเกมการ์ด; Card Component เป็นเพียงกล่องข้อมูล

## 11.2 Main Screens

### Screen 1: Home

องค์ประกอบ:
- Game Title / Logo
- Create Room
- Join Room (กรอก Room Code)
- Display Name
- How to Play แบบสั้น

### Screen 2: Lobby

องค์ประกอบ:
- Room Code + Copy Button
- Player Slots 2–4
- Player Name และ Connection Status
- Ready Toggle
- Start Game (Host Only)
- Leave Room

### Screen 3: Dungeon

Layout แนะนำ Desktop:

```text
+-------------------------------------------------------------+
| Room Code | Floor 2 | My Progress | Phase | Players Online  |
+---------------------------+---------------------------------+
|                           | Player Status                   |
|      DUNGEON MAP          | HP / Gold / Score                |
|      Node Cards           | Equipment Summary                |
|      Available Paths      | Other Players Progress           |
|                           |                                 |
+---------------------------+---------------------------------+
| Current Room / Encounter / Choices                          |
+-------------------------------------------------------------+
| Event Log                                                    |
+-------------------------------------------------------------+
```

ข้อมูลสำคัญ:
- แสดง Node ที่เลือกได้
- แสดง Node ที่ผ่านแล้วและเส้นทางที่ถูกล็อก
- แสดง Room Type และ Risk / Reward
- แสดง Floor, Progress, HP, Gold, Score
- แสดงสถานะของผู้เล่นอื่นในเชิงสรุป แต่ไม่เปิดเผย Inventory และ Map ส่วนตัวทั้งหมด

### Screen 4: Combat

องค์ประกอบ:
- Monster Panel: Image/Icon, Name, HP Bar, ATK/DEF
- Player Panel: HP, ATK, DEF, Equipment
- Action Buttons: Attack, Defend, Use Item, Escape
- Combat Log
- Turn Indicator: `Your Turn` / `Monster Turn`
- Action Buttons ต้อง Disable เมื่อไม่ใช่ Player Turn หรือกำลังรอผลจาก Server

### Screen 5: Treasure / Shop / Event / Rest

ใช้ Panel หรือ Modal แยกตาม Room Type
- Treasure: Item Choice Cards และ Gold Choice
- Shop: Item list, Price, Buy button, Player Gold
- Event: Story text + Choice buttons
- Rest: Heal button และผลที่ได้รับ

### Screen 6: Inventory / Equipment

- Equipment Slots
- Backpack 12 Slots
- Item Detail Panel
- Equip / Unequip / Use / Drop (Drop อาจเลื่อนไป Post-MVP)
- ป้องกันการกดซ้ำระหว่าง Request

### Screen 7: Preparation

- Player Equipment Summary
- Backpack / Consumable
- HP / Gold / Score
- Ready Button
- รายชื่อผู้เล่นที่ยังไม่ Ready

### Screen 8: Final PvP

- Player Panels พร้อม HP และ Score
- PvP Action Budget คงเหลือ
- Target selector
- Attack / Defend / Use Item
- Combat Log
- Scoreboard แบบ Live

### Screen 9: Results

- Winner / Draw
- Final Ranking
- Total Score
- PvE Score Breakdown
- PvP Score Breakdown
- Gold Bonus / Penalty
- สถิติสำคัญ
- Play Again / Return to Lobby

## 11.3 UI State Principles

- Server State เป็น Source of Truth
- UI State เก็บเฉพาะสิ่งชั่วคราว เช่น Modal เปิดอยู่, Tab ที่เลือก, Input Form
- ห้ามคำนวณ Score หรือ Damage เองเพื่อใช้เป็นค่าจริง
- กดปุ่มแล้วแสดง Loading / Pending State จน Server ตอบกลับ
- เมื่อ WebSocket ขาด ให้แสดง Reconnecting และห้ามส่ง Action ซ้ำโดยไม่ใช้ Request ID เดิม
- เมื่อได้รับ Snapshot ใหม่ ให้ UI Render จาก Snapshot ไม่ Mutate Game State เอง

---

# 12. Technical Architecture

## 12.1 Stack

### Frontend

- React
- TypeScript
- Vite
- CSS Modules หรือ Tailwind CSS (เลือกอย่างใดอย่างหนึ่ง; MVP ไม่ต้องใช้ทั้งคู่)
- React Router สำหรับหน้าเกม
- Zustand หรือ React Context + Reducer สำหรับ Client Session/UI State (เลือกอย่างใดอย่างหนึ่ง)
- Native WebSocket API

### Backend

- Go
- Fiber v2
- WebSocket package ที่เข้ากันกับ Fiber v2
- Standard Library สำหรับ JSON, Random, Time, Concurrency, CSV
- ไม่มี Database Driver ใน MVP

### Communication

- REST สำหรับ Create / Join / Health
- WebSocket สำหรับ Game Commands และ State Updates

## 12.2 High-level Architecture

```text
Browser (React + TypeScript)
  |
  | REST: create/join/health
  | WebSocket: commands + snapshots
  v
Go Server (Fiber)
  |
  +-- HTTP Handlers
  |
  +-- WebSocket Handler
  |
  +-- Room Manager (in-memory)
        |
        +-- Room Actor / Serialized Command Processor
              |
              +-- Game State
              +-- Dungeon Service
              +-- Combat Service
              +-- Inventory Service
              +-- Shop / Event Service
              +-- Respawn Service
              +-- PvP Service
              +-- Score Service
  |
  +-- Static JSON Config Loader
```

## 12.3 Server-authoritative Rule

Client ส่ง Intent เช่น `combat.attack` หรือ `dungeon.move` เท่านั้น ห้ามส่งผลลัพธ์ เช่น Damage, New HP, Reward, New Score หรือ Drop Item ที่คำนวณเอง

ตัวอย่างที่ถูก:

```json
{
  "type": "combat.action",
  "request_id": "uuid-from-client",
  "payload": {
    "action": "attack"
  }
}
```

ตัวอย่างที่ห้ามเชื่อถือ:

```json
{
  "type": "combat.action",
  "payload": {
    "damage": 9999,
    "new_hp": 1,
    "score": 500
  }
}
```

## 12.4 In-memory Room Management

- `RoomManager` เก็บ Map ของ Room Code → Room Runtime
- Room หนึ่งห้องมี State, Player Sessions และ Command Processor ของตัวเอง
- ใช้ Room Actor Pattern (แนะนำ) หรือ Mutex ต่อ Room หาก Repository ปัจจุบันเหมาะกว่า
- Room Actor รับ Commands ผ่าน Channel แล้วประมวลผลทีละ Command ทำให้ State Mutation เป็นลำดับ
- Independent Turns ยังคงอยู่ เพราะ Player ไม่ต้องรอ Action ของ Player คนอื่น; Actor เพียงจัดลำดับการเขียนข้อมูลให้ปลอดภัย
- ห้ามถือ Lock ขณะรอ WebSocket Network Write
- แต่ละ Client มี Send Channel / Writer Loop เพื่อป้องกัน Concurrent Write ไปยัง WebSocket Connection เดียวกัน
- เมื่อ Room จบแล้วลบออกจาก RAM หลังส่ง Result และช่วงเวลาสั้น ๆ ให้ผู้เล่นอ่านผล

## 12.5 Reconnect

- หลัง Create / Join Server ออก `player_id` และ `reconnect_token` แบบสุ่ม
- Frontend เก็บข้อมูล Session ใน `sessionStorage` สำหรับ Tab ปัจจุบัน
- WebSocket Reconnect ส่ง Room Code, Player ID และ Token
- Server ตรวจ Token กับ Session ใน RAM แล้วผูก Connection ใหม่
- ถ้า Server Restart ข้อมูล RAM หายและ Reconnect ไม่ได้ ถือเป็นข้อจำกัดที่ยอมรับได้ใน MVP
- ห้ามเก็บ Token ลง URL ที่ถูก Log หากสามารถส่งผ่าน WebSocket Authentication Message หลัง Connect ได้

---

# 13. Suggested Project Structure

แนะนำ Monorepo เพื่อให้ Agent เข้าใจโปรเจกต์เดียวกันง่าย:

```text
dungeon-rivals/
├── README.md
├── docs/
│   ├── MASTER_GAME_SPEC.md
│   ├── API_WEBSOCKET.md
│   ├── GAME_RULES.md
│   └── DEVELOPMENT_LOG.md
│
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── src/
│       ├── app/
│       │   ├── App.tsx
│       │   ├── router.tsx
│       │   └── providers.tsx
│       ├── pages/
│       │   ├── HomePage.tsx
│       │   ├── LobbyPage.tsx
│       │   ├── GamePage.tsx
│       │   └── ResultPage.tsx
│       ├── features/
│       │   ├── lobby/
│       │   ├── dungeon/
│       │   ├── combat/
│       │   ├── inventory/
│       │   ├── shop/
│       │   ├── preparation/
│       │   ├── pvp/
│       │   └── results/
│       ├── components/
│       │   ├── ui/
│       │   ├── game/
│       │   └── layout/
│       ├── services/
│       │   ├── api.ts
│       │   └── websocket.ts
│       ├── stores/
│       │   ├── sessionStore.ts
│       │   └── gameStore.ts
│       ├── types/
│       │   ├── api.ts
│       │   └── game.ts
│       └── styles/
│           ├── tokens.css
│           └── global.css
│
├── backend/
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── data/
│   │   ├── items.json
│   │   ├── monsters.json
│   │   ├── events.json
│   │   ├── dungeon.json
│   │   └── scoring.json
│   ├── internal/
│   │   ├── config/
│   │   ├── transport/
│   │   │   ├── http/
│   │   │   └── websocket/
│   │   ├── room/
│   │   ├── game/
│   │   │   ├── model/
│   │   │   ├── service/
│   │   │   ├── dungeon/
│   │   │   ├── combat/
│   │   │   ├── inventory/
│   │   │   ├── economy/
│   │   │   ├── respawn/
│   │   │   ├── pvp/
│   │   │   └── scoring/
│   │   └── shared/
│   └── tests/
│
└── scripts/
    ├── dev.sh
    └── dev.ps1
```

หมายเหตุ: ถ้า Repository มีโครงสร้างอยู่แล้ว ให้ปรับให้เข้ากับของเดิม ไม่ต้องย้ายหรือสร้าง Folder ใหม่ทั้งหมดโดยไม่มีเหตุผล

---

# 14. Backend Code Design

## 14.1 Domain Model (ตัวอย่างแนวทาง)

```go
package model

import "time"

// GamePhase represents the global phase of one room.
type GamePhase string

const (
    PhaseLobby       GamePhase = "lobby"
    PhaseDungeon     GamePhase = "dungeon"
    PhasePreparation GamePhase = "preparation"
    PhasePvP          GamePhase = "pvp"
    PhaseResult       GamePhase = "result"
)

// PlayerStage represents the current activity of one player.
type PlayerStage string

const (
    StageLobby       PlayerStage = "lobby"
    StageExploring   PlayerStage = "exploring"
    StageCombat      PlayerStage = "combat"
    StagePreparation PlayerStage = "preparation"
    StagePvP         PlayerStage = "pvp"
    StageFinished    PlayerStage = "finished"
    StageDisconnected PlayerStage = "disconnected"
)

// GameRoom is the authoritative state for one multiplayer match.
type GameRoom struct {
    ID           string                `json:"id"`
    Code         string                `json:"code"`
    HostPlayerID string                `json:"host_player_id"`
    Phase        GamePhase             `json:"phase"`
    PlayerOrder  []string              `json:"player_order"`
    Players      map[string]*Player    `json:"-"`
    CreatedAt    time.Time             `json:"created_at"`
    UpdatedAt    time.Time             `json:"updated_at"`
    Version      uint64                `json:"version"`
    Game         *GameState            `json:"-"`
}

// Player contains mutable player state owned by the server.
type Player struct {
    ID          string           `json:"id"`
    Name        string           `json:"name"`
    Ready       bool             `json:"ready"`
    Connected   bool             `json:"connected"`
    Stage       PlayerStage      `json:"stage"`
    Stats       PlayerStats      `json:"stats"`
    Gold        int              `json:"gold"`
    Score       ScoreBreakdown   `json:"score"`
    Inventory   Inventory        `json:"inventory"`
    Equipment   Equipment        `json:"equipment"`
    Dungeon     PlayerDungeon    `json:"dungeon"`
    Combat      *CombatState     `json:"combat,omitempty"`
    PvP         PvPPlayerState   `json:"pvp"`
    Statistics  PlayerStatistics `json:"statistics"`
}

// PlayerStats contains base values; derived values are calculated by services.
type PlayerStats struct {
    HP       int `json:"hp"`
    MaxHP    int `json:"max_hp"`
    BaseATK  int `json:"base_atk"`
    BaseDEF  int `json:"base_def"`
}
```

ตัวอย่างนี้เป็นแนวทาง ไม่จำเป็นต้อง Copy ทั้งหมดโดยไม่พิจารณาความเหมาะสม แต่ต้องรักษา Separation ระหว่าง Domain State, Transport DTO และ Static Config

## 14.2 แยก Domain Model กับ WebSocket DTO

อย่าส่ง `GameRoom` ภายในทั้งก้อนออก Client เพราะอาจเปิดเผย Reconnect Token, Map ของผู้เล่นคนอื่น หรือข้อมูลภายใน Server

ควรมี DTO แยก:

- `PublicRoomSnapshot`: Room Code, Phase, Player Public Status, Scores
- `PrivatePlayerSnapshot`: Inventory, Equipment, Personal Map, Current Encounter
- `RoomSnapshot`: รวม Public + Private Snapshot สำหรับผู้เล่นที่ขอ

สร้าง Snapshot ตาม Player ID ผู้รับ เพื่อให้เห็น Private Data ของตัวเองเท่านั้น

## 14.3 Service Responsibilities

| Service | Responsibility |
|---|---|
| RoomService | Create, Join, Leave, Ready, Start, Kick, Cleanup |
| GameService | Phase transitions, Match lifecycle |
| DungeonService | Generate Map, Move Node, Resolve Room |
| CombatService | Validate Action, Damage, Monster Turn, Combat End |
| InventoryService | Add, Remove, Equip, Unequip, Use Item |
| ShopService | Stock, Purchase, Gold validation |
| EventService | Event choices and outcomes |
| RespawnService | Death penalty, checkpoint, restore HP |
| PvPService | Arena Action, Targeting, PvP Respawn, Action Budget |
| ScoreService | Score Event, Total, Tie-breaker, Results |
| ConfigService | Load and validate JSON definitions |

แยก Service ตาม Domain Behavior ไม่สร้าง Service แยกยิบย่อยโดยไม่มี Logic ที่เป็นอิสระ

## 14.4 Error Handling

ใช้ Error Code ที่คงที่สำหรับ Frontend เช่น:

- `ROOM_NOT_FOUND`
- `ROOM_FULL`
- `ROOM_ALREADY_STARTED`
- `PLAYER_NOT_FOUND`
- `NOT_ROOM_HOST`
- `NOT_READY`
- `INVALID_PHASE`
- `INVALID_NODE`
- `NODE_NOT_AVAILABLE`
- `NOT_IN_COMBAT`
- `NOT_PLAYER_TURN`
- `INVALID_ACTION`
- `ITEM_NOT_FOUND`
- `ITEM_NOT_USABLE`
- `INVENTORY_FULL`
- `NOT_ENOUGH_GOLD`
- `INVALID_TARGET`
- `NO_PVP_ACTIONS_LEFT`
- `DUPLICATE_REQUEST`

Error Response ต้องมี `request_id`, `code`, `message` และอาจมี `details` สำหรับ Debug โดยไม่เปิดเผยข้อมูลลับ

---

# 15. HTTP API Design

ใช้ Prefix `/api/v1`

## 15.1 Health

`GET /api/v1/health`

Response:

```json
{
  "status": "ok"
}
```

## 15.2 Create Room

`POST /api/v1/rooms`

Request:

```json
{
  "player_name": "Player One"
}
```

Response:

```json
{
  "room_code": "A7K9Q2",
  "player_id": "player-id",
  "reconnect_token": "random-secret-token",
  "is_host": true,
  "ws_path": "/ws"
}
```

## 15.3 Join Room

`POST /api/v1/rooms/{room_code}/join`

Request:

```json
{
  "player_name": "Player Two"
}
```

Response รูปแบบเดียวกับ Create Room แต่ `is_host=false`

## 15.4 Optional: Room Preview

`GET /api/v1/rooms/{room_code}`

ส่งข้อมูล Lobby ที่เป็น Public เช่น จำนวนผู้เล่นและสถานะ Started หรือ Waiting เท่านั้น ห้ามเปิด Reconnect Token หรือข้อมูลส่วนตัว

---

# 16. WebSocket Protocol

## 16.1 Connection

`GET /ws`

หลัง Connect ให้ Client ส่ง Authentication Message:

```json
{
  "type": "session.authenticate",
  "request_id": "uuid",
  "payload": {
    "room_code": "A7K9Q2",
    "player_id": "player-id",
    "reconnect_token": "random-secret-token"
  }
}
```

เมื่อสำเร็จ Server ส่ง `session.authenticated` และ `room.snapshot`

## 16.2 Message Envelope

ทุก Message ใช้รูปแบบ:

```json
{
  "type": "namespace.action",
  "request_id": "uuid",
  "payload": {}
}
```

Server Response:

```json
{
  "type": "command.accepted",
  "request_id": "uuid",
  "payload": {
    "message": "Action accepted"
  }
}
```

หรือ Error:

```json
{
  "type": "command.error",
  "request_id": "uuid",
  "payload": {
    "code": "INVALID_PHASE",
    "message": "This action is not available in the current phase"
  }
}
```

State Update:

```json
{
  "type": "room.snapshot",
  "payload": {
    "version": 12,
    "room": {},
    "players": [],
    "me": {}
  }
}
```

## 16.3 Client → Server Events

| Event | Payload | Rule |
|---|---|---|
| `session.authenticate` | room_code, player_id, reconnect_token | ต้อง Authenticate ก่อน |
| `lobby.ready` | ready: boolean | Lobby เท่านั้น |
| `game.start` | empty | Host + all ready + 2–4 players |
| `dungeon.move` | node_id | Node ต้อง Available ของตัวเอง |
| `combat.action` | action, optional item_id | ต้องอยู่ Combat และ Player Turn |
| `inventory.equip` | item_id, slot | ตรวจ Ownership / Slot |
| `inventory.unequip` | slot, optional index | ตรวจ Slot / Capacity |
| `inventory.use` | item_id, optional target_id | ตรวจ Effect / Context |
| `shop.purchase` | item_id, quantity | ตรวจ Stock / Gold / Capacity |
| `treasure.choose` | reward_id | ต้องเป็น Reward Choice ของ Treasure ปัจจุบัน |
| `event.choose` | choice_id | ต้องเป็น Choice ของ Event ปัจจุบัน |
| `rest.use` | rest_option | ต้องอยู่ Rest Room ปัจจุบัน |
| `room.continue` | empty | ปิด Room ที่ Resolve แล้วและกลับไป Node Selection |
| `preparation.ready` | ready: boolean | Preparation เท่านั้น |
| `pvp.action` | action, target_id, optional item_id | PvP + Action Budget > 0 |
| `room.leave` | empty | Leave / Disconnect |
| `room.kick` | target_player_id | Host only, ตามกฎ Phase |
| `pvp.force_finish` | empty | Host only, PvP เท่านั้น |

## 16.4 Server → Client Events

| Event | ใช้ทำอะไร |
|---|---|
| `session.authenticated` | ยืนยัน Session |
| `room.snapshot` | State Snapshot ของผู้รับ |
| `room.player_joined` | แจ้งสมาชิกใหม่ |
| `room.player_left` | แจ้งสมาชิกออก |
| `room.player_connection` | Online / Offline |
| `game.phase_changed` | เปลี่ยน Phase |
| `dungeon.updated` | Map / Progress เปลี่ยน |
| `combat.updated` | Combat State และ Log เปลี่ยน |
| `inventory.updated` | Inventory / Equipment เปลี่ยน |
| `shop.updated` | Stock / Purchase เปลี่ยน |
| `treasure.resolved` | Treasure Reward ถูกเลือกและได้รับแล้ว |
| `event.resolved` | Event Outcome |
| `player.respawned` | แจ้ง Respawn |
| `score.updated` | Scoreboard เปลี่ยน |
| `pvp.updated` | PvP State เปลี่ยน |
| `game.result` | Final Results |
| `command.error` | Error จาก Command |

## 16.5 Snapshot Strategy

สำหรับผู้เล่น 2–4 คน ใช้ **Full Snapshot หลังทุก State Mutation** ใน MVP เพื่อให้พัฒนาและ Debug ง่ายก่อน Optimize

- Snapshot ต้องมี `version` เพิ่มขึ้นทุกครั้งที่ State เปลี่ยน
- Client ทิ้ง Snapshot ที่มี Version ต่ำกว่าค่าปัจจุบัน
- สร้าง Snapshot แยกต่อผู้รับ เพื่อไม่เปิด Private State ของคนอื่น
- ห้ามส่ง Token กลับไปใน Snapshot
- เมื่อ Performance มีปัญหาค่อยเปลี่ยนเป็น Delta Events

## 16.6 Idempotency / Duplicate Requests

- ทุก Client Command ต้องมี `request_id`
- Server เก็บผล Request ล่าสุดต่อ Player ใน RAM เช่น LRU/Map ที่มีขนาดจำกัด
- หาก Request ID เดิมเข้าซ้ำ ให้คืนผลเดิมหรือ Ignore โดยไม่ Execute Game Action ซ้ำ
- Reward, Score, Shop Purchase และ Death Event ต้องมี Internal Event ID ป้องกันการคิดซ้ำอีกชั้น

---

# 17. Frontend Code Design

## 17.1 State Ownership

แยก State เป็น 3 กลุ่ม:

1. **Server Game State:** รับจาก `room.snapshot` เท่านั้น เช่น HP, Gold, Inventory, Map, Combat, Score
2. **Session State:** Room Code, Player ID, Reconnect Token, Connection State
3. **UI State:** Modal, Selected Tab, Selected Target, Form Input, Pending Button

ห้ามให้ Frontend แก้ Server Game State แล้วถือว่าเป็นผลจริง

## 17.2 TypeScript Types

สร้าง Types กลางให้ตรงกับ WebSocket DTO ของ Go เช่น:

```ts
export type GamePhase =
  | "lobby"
  | "dungeon"
  | "preparation"
  | "pvp"
  | "result";

export type PlayerStage =
  | "lobby"
  | "exploring"
  | "combat"
  | "preparation"
  | "pvp"
  | "finished"
  | "disconnected";

export interface PlayerPublicView {
  id: string;
  name: string;
  stage: PlayerStage;
  connected: boolean;
  floor: number;
  progress: number;
  score: number;
}

export interface ItemView {
  id: string;
  quantity: number;
}

export interface RoomSnapshot {
  version: number;
  phase: GamePhase;
  players: PlayerPublicView[];
  me: {
    id: string;
    hp: number;
    maxHp: number;
    gold: number;
    inventory: ItemView[];
    // เพิ่ม equipment, dungeon, combat, pvp ตาม DTO จริง
  };
}
```

ห้ามใช้ `any` กับ Game State หลัก หากไม่ทราบ Shape ให้กำหนด Type อย่างชัดเจน

## 17.3 WebSocket Client

`services/websocket.ts` ควรรับผิดชอบ:

- Connect / Disconnect
- Authenticate
- Send Command พร้อม Request ID
- Parse และ Validate Message Envelope
- Dispatch Server Events ไป Store
- Reconnect ด้วย Exponential Backoff แบบมีเพดาน
- ป้องกันสร้าง WebSocket ซ้ำหลาย Connection
- ปิด Connection เมื่อออกจาก Room หรือ Unmount ตาม Lifecycle

## 17.4 Game Store

Store ควรเก็บ:

- Latest Room Snapshot
- Latest Snapshot Version
- Connection Status
- Pending Request IDs (สำหรับ Loading)
- Last Error

ไม่เก็บ Game Rule หรือคำนวณ Combat ใน Store

## 17.5 Component Design

ตัวอย่าง Components:

- `RoomCodePanel`
- `PlayerSlot`
- `ReadyButton`
- `DungeonMap`
- `DungeonNodeCard`
- `PlayerStatusPanel`
- `MonsterPanel`
- `CombatActionBar`
- `CombatLog`
- `ItemCard`
- `InventoryGrid`
- `EquipmentSlots`
- `ShopItemCard`
- `EventChoicePanel`
- `PreparationPanel`
- `PvPArena`
- `PvPPlayerPanel`
- `Scoreboard`
- `ResultSummary`

Component ควรรับ Props และแสดงผล ไม่ควรเรียก WebSocket โดยตรงทุก Component ให้เรียก Action ผ่าน Feature Hook / Store Action ชั้นกลาง

---

# 18. Static Config และ JSON Files

## 18.1 Files

- `items.json`: Item Definitions
- `monsters.json`: Monster Definitions
- `events.json`: Event Definitions
- `dungeon.json`: Floors, Weights, Difficulty, Map Rules
- `scoring.json`: Score Values, Gold Conversion, Penalties

## 18.2 Loader Requirements

- อ่านไฟล์จาก Path ที่กำหนดผ่าน Environment หรือ Config
- Validate Duplicate IDs
- Validate Item Type / Rarity / Slot
- Validate Monster Stats ไม่ติดลบ
- Validate Drop Table Reference ว่ามีจริง
- Validate Room Weights และ Score Config
- โหลดครั้งเดียวตอน Start สำหรับ MVP
- ถ้า Config Invalid ให้ Fail Fast พร้อมบอกไฟล์และ Field ที่ผิด

## 18.3 Runtime Config Safety

เมื่อสร้าง Encounter / Shop Stock / Reward ให้ Copy ค่าที่จำเป็นเข้า Runtime State เพื่อไม่ให้การ Reload Config ภายหลังเปลี่ยน Encounter ที่กำลังเล่นอยู่โดยไม่ตั้งใจ

---

# 19. Persistence และ Server Lifecycle

## 19.1 MVP Storage Policy

| Data | Storage |
|---|---|
| Item / Monster / Dungeon Definition | JSON Files |
| Active Rooms | RAM |
| Active Players | RAM |
| Current HP / Inventory / Score | RAM |
| Current Map / Encounter | RAM |
| Finished Match Result | RAM ก่อน; CSV Optional |

## 19.2 CSV Result Log (Optional หลัง Core Game ทำงาน)

ไฟล์ตัวอย่าง `backend/data/results/matches.csv`:

```csv
match_id,finished_at,room_code,player_id,player_name,rank,total_score,pve_score,pvp_score,gold_score,deaths
```

ข้อกำหนด:

- เขียน CSV เฉพาะเมื่อ Match จบ
- ใช้ Mutex ป้องกัน Concurrent Write
- เขียน Header เมื่อสร้างไฟล์ครั้งแรก
- Escape Comma / Quote / Newline ให้ถูกต้องด้วย `encoding/csv`
- ถ้าเขียน CSV ไม่สำเร็จ ต้องไม่ทำให้ Final Result ใน RAM หายหรือคำนวณซ้ำ
- CSV ไม่ใช่ Database และไม่ใช้โหลด Active Match กลับมาหลัง Server Restart ใน MVP

## 19.3 Server Restart

- Active Match หายเมื่อ Restart เป็นข้อจำกัดที่ยอมรับได้ใน MVP
- UI ควรแจ้งว่า Room หมดอายุหรือหาไม่พบหลัง Restart
- ไม่ต้องทำ Snapshot Recovery หรือ Persistent Session ใน Phase แรก

---

# 20. Security, Validation และ Reliability

แม้เป็นเกมเล็กก็ต้องป้องกัน Client ส่งข้อมูลผิด:

1. Validate Room Membership ทุก Command
2. Validate Player ID จาก Authenticated Session ไม่รับ Player ID จาก Payload เพื่อระบุตัวผู้กระทำ
3. Validate Current Phase
4. Validate Player Stage
5. Validate Node Connection / Node Status
6. Validate Item Ownership / Quantity / Equip Slot
7. Validate Gold ก่อนซื้อ
8. Validate Target Player และห้าม Target ตัวเองใน Attack
9. Validate PvP Action Budget
10. Rate Limit Command ที่อาจ Spam เช่น Join, Chat (ถ้ามี), Action
11. จำกัดขนาด WebSocket Message
12. จำกัด Player Name และ Sanitize ข้อความก่อน Render
13. ห้าม Expose Reconnect Token และ Internal Seed ต่อผู้เล่นอื่น
14. ใช้ HTTPS/WSS เมื่อ Deploy Public
15. ใช้ Random ที่เหมาะสมฝั่ง Server และไม่ให้ Client เลือก Loot Result

## 20.1 Concurrency

- Room Manager ป้องกัน Race ตอน Create / Join / Delete Room
- Room State เปลี่ยนผ่าน Actor หรือ Mutex เท่านั้น
- WebSocket หนึ่ง Connection มี Writer เดียว
- อย่าส่ง Network Message ขณะถือ Room Lock
- เพิ่ม Test กรณีผู้เล่นหลายคนส่ง Action พร้อมกัน

---

# 21. Testing Plan

## 21.1 Backend Unit Tests

### Damage / Combat

- Attack คิด ATK - DEF ถูกต้อง
- Damage ต่ำสุดเป็น 1
- Defend ลด Damage ตามสูตร
- Monster ตายแล้วแจก Reward ครั้งเดียว
- Player ตายแล้วไม่ติดลบ HP
- Escape สำเร็จ/ล้มเหลวทำงานถูกต้อง
- ใช้ Potion แล้วหัก Quantity หนึ่งครั้ง

### Inventory

- Add Item เมื่อมีพื้นที่
- Add Item เมื่อ Backpack เต็มต้อง Error
- Stackable Item รวม Stack ได้ไม่เกิน Max Stack
- Equip Item ประเภทถูกต้อง
- Equip ผิด Slot ต้อง Error
- Unequip เมื่อ Backpack เต็มต้อง Error
- Use Item ที่ไม่มีอยู่ต้อง Error

### Dungeon

- Generator สร้าง Node และ Connection ถูกต้อง
- Boss อยู่ปลาย Floor
- Move ไป Node ที่เชื่อมต่อได้
- Move ไป Node ที่ Locked / Cleared ไม่ได้
- Player A Move ไม่เปลี่ยน Dungeon State ของ Player B
- Player แต่ละคนมี Seed และ Runtime Map ของตัวเอง

### Respawn

- Gold ถูกหัก 20% แบบปัดลง
- ลบ Backpack Item หนึ่งชิ้นเท่านั้น
- ไม่ลบ Equipped Item
- HP กลับเป็น 50% Max HP
- กลับ Checkpoint Floor ปัจจุบัน
- Death Penalty ไม่เกิดซ้ำจาก Duplicate Request

### Score

- PvE Score ถูกต้อง
- PvP Damage Remainder ทำงานถูกต้อง
- KO Score ไม่แจกซ้ำ
- Penalty ไม่ทำให้ Score ต่ำกว่า 0 หากกติกากำหนด Clamp
- Tie-breaker ให้ผลตามลำดับ

## 21.2 Backend Integration Tests

- Create Room แล้ว Host เป็นคนแรก
- Join Room ด้วย Code ถูกต้อง
- Room เต็มแล้ว Join ไม่ได้
- Start ได้เมื่อ 2–4 คน Ready
- Start ไม่ได้เมื่อมีคนไม่ Ready
- ผู้เล่นที่ไม่ใช่ Host Start ไม่ได้
- WebSocket Authenticate และ Reconnect ได้
- ผู้เล่นสองคน Move Dungeon พร้อมกันได้โดย State ไม่เสียหาย
- ผู้เล่นคนหนึ่ง Combat ขณะที่อีกคน Shop ได้
- Phase เปลี่ยนเมื่อผู้เล่นทุกคน Clear Boss และ Ready
- PvP Actions ของผู้เล่นแต่ละคนถูกนับแยก
- จบ Match และ Scoreboard ถูกต้อง

## 21.3 Frontend Tests

- Home Create / Join validation
- Lobby Ready / Host Start visibility
- Snapshot Version เก่าไม่ทับ State ใหม่
- Reconnect UI แสดงถูกต้อง
- Button Disabled ระหว่าง Pending Request
- Inventory / Equipment Render จาก Server Snapshot
- Combat Buttons แสดงตาม Player Turn
- PvP Action Budget แสดงตรงกับ Server
- Result Ranking เรียงคะแนนถูกต้อง

## 21.4 Manual Acceptance Test

ใช้ Browser 2 หน้าต่างหรือ 2 เครื่อง:

1. Player A สร้าง Room
2. Player B Join ด้วย Room Code
3. ทั้งคู่ Ready และ Host Start
4. Player A เลือก Monster Room
5. Player B เลือก Treasure Room ในเวลาเดียวกัน
6. Player A ต่อสู้ ขณะที่ Player B รับ Reward ได้
7. Player A ตายและ Respawn โดยยังเล่นต่อได้
8. ทั้งคู่ผ่าน Boss และเข้าสู่ Preparation
9. ทั้งคู่ Ready แล้วเข้าสู่ Final PvP
10. ต่อสู้แบบสลับ Turn จนมีฝ่ายหนึ่ง Knockout และตรวจว่าผู้เล่นที่ยังไม่ถึง Turn ถูกปฏิเสธ
11. ตรวจว่าทุกคู่ในตาราง Round-Robin แข่งครบก่อนคำนวณ Score และ Ranking
12. จบเกมแล้วแสดง Result
13. ตรวจว่าไม่มี Database ถูกเรียกใช้

---

# 22. Development Roadmap — ลำดับการพัฒนา

ให้ทำตามลำดับนี้ ไม่ควรเริ่มจาก Content จำนวนมากก่อน Game Loop ทำงาน

## Phase 0 — Repository Setup & Specification

**เป้าหมาย:** เตรียมโครงสร้างโปรเจกต์และเอกสารให้ชัด

Tasks:
- ตรวจ Repository ที่มีอยู่
- สร้างหรือปรับ Monorepo Structure
- ตั้ง React + TypeScript + Vite
- ตั้ง Go + Fiber Server
- เพิ่ม `/api/v1/health`
- ตั้ง CORS / Vite Proxy สำหรับ Dev
- เพิ่ม README วิธี Run Frontend / Backend
- เพิ่ม `.env.example`
- วาง JSON Config ตัวอย่างและ Config Loader เบื้องต้น

Acceptance:
- `npm run dev` เปิด Frontend ได้
- `go run ./cmd/server` เปิด Backend ได้
- Health Check ผ่าน
- Frontend เรียก Backend ได้

## Phase 1 — Lobby & Room Management

**เป้าหมาย:** ผู้เล่น 2–4 คนสร้างและเข้าห้องเดียวกันได้

Tasks:
- Room Manager ใน RAM
- Create Room / Join Room
- Room Code Generator
- Player ID + Reconnect Token
- Lobby Snapshot
- Ready / Unready
- Host Start
- Player Leave / Connection Status
- WebSocket Authentication
- Reconnect เบื้องต้น

Acceptance:
- เปิด Browser 2–4 Clients แล้วอยู่ Lobby เดียวกันได้
- Room เต็มแล้ว Join ไม่ได้
- เฉพาะ Host Start ได้
- Start ต้องมี 2–4 Players และทุกคน Ready
- ไม่มี Database

## Phase 2 — Player State & Inventory Foundation

**เป้าหมาย:** มีตัวละครเชิงข้อมูลและ Item ที่ Server ควบคุม

Tasks:
- Player Stats
- Starting Equipment / Gold / Item
- Item JSON Loader
- Inventory Model
- Equipment Slots
- Equip / Unequip / Use Item
- Derived Stat Calculation
- Private Player Snapshot

Acceptance:
- Player แต่ละคนมี Inventory ส่วนตัว
- ผู้เล่นอื่นไม่เห็น Private Inventory
- Equip แล้ว ATK/DEF ที่ Server คำนวณเปลี่ยนถูกต้อง
- Item ปลอมจาก Client ใช้ไม่ได้

## Phase 3 — Dungeon Map & Independent Movement

**เป้าหมาย:** ผู้เล่นแยกเส้นทางและเดินได้โดยไม่รอกัน

Tasks:
- Dungeon JSON Config
- Seed-based Map Generator
- 3 Floors
- Node Graph / Connections
- Room Type Distribution
- Node Status
- `dungeon.move`
- Room Snapshot แบบ Public + Private
- Player Progress View

Acceptance:
- Player A และ B ได้ Map แยกกัน
- Player A เดินได้โดย Player B ไม่ต้องกด Ready
- Move ไป Node ที่ผิด Connection ไม่ได้
- Node ที่ Clear แล้วกลับไปซ้ำไม่ได้
- Map ถึง Boss Node ตามโครงสร้าง

## Phase 4 — Monster Combat & Respawn

**เป้าหมาย:** Combat Turn-based ทำงานครบ End-to-End

Tasks:
- Monster JSON Loader
- Encounter Creation
- Combat State
- Attack / Defend / Use Item / Escape
- Monster Basic AI
- Damage Formula
- Combat Log
- Loot / Gold Reward
- Score Reward
- PvE Death / Respawn / Checkpoint

Acceptance:
- ต่อสู้จนชนะและรับ Reward ได้
- Monster ตอบโต้ตาม Turn
- ใช้ Item แล้ว Quantity ลด
- ตายแล้ว Respawn ตามกติกา
- Reward และ Penalty ไม่เกิดซ้ำ

## Phase 5 — Room Types, Shop, Event, Rest, Boss

**เป้าหมาย:** Dungeon มีทางเลือกหลากหลาย ไม่ใช่มีแต่ Monster

Tasks:
- Treasure Room
- Shop / Stock / Purchase
- Event Choices
- Rest Room
- Elite Monster
- Floor Boss
- Difficulty Scaling
- Floor Transition

Acceptance:
- ทุก Room Type Resolve ได้
- Purchase เป็น Atomic
- Event Choice ทำงานครั้งเดียว
- Boss ชนะแล้วปลดล็อก Floor ถัดไป
- Clear Final Boss แล้ว Player เข้า Preparation

## Phase 6 — Preparation & Match Phase Transition

**เป้าหมาย:** รวมผู้เล่นที่เล่นเสร็จต่างเวลากันเข้าสู่ Final PvP

Tasks:
- Preparation Screen
- Inventory / Equipment Review
- Restore HP ก่อน PvP
- Ready State
- All Players Ready Transition
- Host Kick / Force Finish Policy สำหรับผู้เล่นที่ออกจากเกม

Acceptance:
- Player ที่จบก่อนรอใน Preparation ได้
- Player อื่นยังเล่น Dungeon ต่อได้
- PvP ไม่เริ่มจนกว่าผู้เล่นที่ยังอยู่ใน Match จะ Ready
- Private Inventory ไม่รั่วไหล

## Phase 7 — Final PvP & Scoring

**เป้าหมาย:** เล่น Arena และตัดสินผู้ชนะจากคะแนน

Tasks:
- Arena State
- Round-Robin PvP พร้อม Turn สลับและจบคู่เมื่อ Knockout
- Attack Target
- Defend
- Use Item
- PvP Damage Score
- Knockout Score
- PvP Respawn
- Scoreboard
- Total Score / Tie-breaker
- Result Screen

Acceptance:
- ทุก Player มี Action Budget เท่ากัน
- Action ถูก Resolve โดย Server
- PvP Score ไม่แจกซ้ำ
- ผู้เล่นตายแล้ว Respawn
- Final Ranking คำนวณถูกต้อง

## Phase 8 — Polish, Reliability & Optional CSV

**เป้าหมาย:** ทำให้เล่นจบ Match ได้อย่างเสถียร

Tasks:
- Reconnect / Disconnect Edge Cases
- WebSocket Error Handling
- Duplicate Request Protection
- Race / Concurrency Tests
- UI Responsive Polish
- Loading / Empty / Error States
- Game Logs
- Optional CSV Result Export
- Balance Test
- README และ Runbook

Acceptance:
- เล่น Match เต็มตั้งแต่ Create Room ถึง Results ได้บน Browser 2–4 Clients
- ไม่มี Duplicate Reward / Score จากการส่ง Request ซ้ำ
- `go test ./...` ผ่าน
- Frontend Type Check / Build ผ่าน
- วิธี Run Local ชัดเจน

---

# 23. MVP Definition of Done

ถือว่า MVP สำเร็จเมื่อเงื่อนไขทั้งหมดต่อไปนี้ผ่าน:

- [ ] เปิดเว็บและ Create Room ได้
- [ ] ผู้เล่น 2–4 คน Join Room ด้วย Code ได้
- [ ] Ready และ Start Game ได้ถูกต้อง
- [ ] ผู้เล่นแต่ละคนมี Dungeon Map ของตัวเอง
- [ ] ผู้เล่นเดินคนละเส้นทางได้อย่างอิสระ
- [ ] ผู้เล่นหนึ่งคน Combat ขณะที่อีกคนทำ Room Action ได้
- [ ] Monster Combat ทำงานแบบ Turn-based
- [ ] ผู้เล่นเก็บ Gold / Item / Equipment ได้
- [ ] Inventory และ Equipment ใช้งานได้
- [ ] มี Monster, Treasure, Shop, Event, Rest, Elite และ Boss อย่างน้อยแบบพื้นฐาน
- [ ] ผู้เล่นตายแล้ว Respawn ตามกติกา
- [ ] ผู้เล่นผ่าน Dungeon และเข้าสู่ Preparation ได้
- [ ] ทุกคน Ready แล้วเข้าสู่ Final PvP ได้
- [ ] Final PvP มี Attack / Defend / Use Item
- [ ] PvP สลับ Turn ภายในคู่และปฏิเสธ Action ผิด Turn
- [ ] PvP ทุกคู่จบด้วย Knockout ก่อนสรุปคะแนนและ Ranking
- [ ] PvP Death มี Respawn
- [ ] คะแนน PvE / PvP / Gold / Penalty คำนวณได้
- [ ] Results แสดง Ranking และ Score Breakdown
- [ ] เล่นได้บน Browser อย่างน้อย 2 Clients
- [ ] ไม่มี Database
- [ ] Server เป็นผู้ควบคุม Game State และ Score

---

# 24. สิ่งที่ยังไม่ทำใน MVP (Post-MVP Backlog)

เก็บไว้หลัง Core Loop เสถียร:

- Login / Account / Persistent Player Profile
- Database และ Cloud Save
- Global Leaderboard
- Friends / Party / Invite Link
- Chat
- Trading / Item Transfer
- Character Classes และ Skill Tree
- Monster Special Abilities หลายรูปแบบ
- Status Effects เช่น Poison, Burn, Freeze
- Equipment Set Bonus
- Legendary Unique Effects
- Daily Quest / Achievement
- More Dungeon Themes / Biomes
- More Floors / More Bosses
- Sound / Music / Animation
- Character Model / 3D / Real-time Movement
- Spectator Mode
- Reconnect หลัง Server Restart
- Ranking Season / Match History
- AI Bot สำหรับเติมผู้เล่น

---

# 25. Open Design Notes / จุดที่ต้อง Balance ภายหลัง

หัวข้อต่อไปนี้ตั้งค่าเริ่มต้นไว้แล้ว แต่ควรปรับจากผล Prototype ไม่ต้องหยุดการพัฒนาเพื่อถกตัวเลขตั้งแต่แรก:

1. Monster HP / ATK / DEF และ Floor Multipliers
2. Item Drop Rates และ Shop Prices
3. Respawn Penalty 20% Gold + Item 1 ชิ้น
4. จำนวน Node และจำนวน Encounter ต่อ Floor
5. คะแนน PvE / PvP และ Gold Conversion
6. PvP จัดตาราง Round-Robin และสู้จน Knockout
7. การสลับ Turn และการปฏิเสธ Action ที่ส่งผิด Turn
8. Inactivity / Disconnect / Host Kick Policy
9. Inventory Capacity และ Stack Limits
10. HP Restore ก่อน Final PvP

ค่าพวกนี้ควรอยู่ใน JSON Config หรือ Constants กลาง ไม่กระจาย Hardcode หลายไฟล์

---

# 26. First Implementation Task สำหรับ Coding Agent

**อย่าเริ่มเขียนทุกระบบพร้อมกัน** ให้เริ่มจาก Phase 0 และ Phase 1 เท่านั้น

ลำดับงานแรก:

1. Inspect Repository และสรุป Tech Stack / Existing Code
2. เสนอ File Tree ที่เข้ากับ Repository ปัจจุบัน
3. ตั้ง React + TypeScript + Vite หากยังไม่มี
4. ตั้ง Go + Fiber Server หากยังไม่มี
5. เพิ่ม Health Check
6. เพิ่ม Create Room / Join Room แบบ In-memory
7. เพิ่ม WebSocket Authentication และ Lobby Snapshot
8. เพิ่ม Ready / Unready / Host Start
9. เขียน Tests สำหรับ Room Rules
10. ทดลองเปิด 2 Browser Clients และยืนยันว่าเห็น Lobby State ตรงกัน

เมื่อ Phase 1 ผ่านแล้วจึงเริ่ม Phase 2 ตามเอกสาร ห้ามกระโดดไปทำ Dungeon Combat หรือ PvP ก่อน Room และ Multiplayer Foundation เสถียร

---

# 27. Prompt สำหรับส่งให้ Coding Agent

คัดลอกข้อความนี้พร้อมแนบไฟล์ `MASTER_GAME_SPEC.md` ให้ Agent:

> อ่านเอกสาร Dungeon Rivals Master Game Design & Development Plan ให้ครบก่อนเริ่มทำงาน เอกสารนี้คือ Source of Truth ของโปรเจกต์
>
> งานของคุณคือพัฒนาเว็บเกม Dungeon Rivals ตามสเปก โดยใช้ React + TypeScript + Vite เป็น Frontend, Go + Fiber เป็น Backend และ WebSocket สำหรับ Multiplayer ช่วง MVP ห้ามใช้ Database ให้ Static Data เก็บใน JSON และ Active Game State เก็บใน RAM ของ Go Server
>
> เกมเป็น Multiplayer 2–4 คน แบบ Turn-based Independent Turns ผู้เล่นเลือกเส้นทาง Dungeon ของตัวเอง ต่อสู้กับ Monster เก็บ Item / Equipment และ Gold เมื่อตายต้อง Respawn ไม่ถูกตัดออกจากเกม เมื่อจบ Dungeon ทุกคนเข้าสู่ Final PvP ซึ่งเป็น Score-based ผู้เล่นแต่ละคนมี Action Budget เท่ากัน
>
> UI ใช้ Card-like Panels / Icons / Static Images เป็นตัวแทน Room, Monster, Item และ Player เท่านั้น ห้ามทำเป็น Card Battler และไม่ต้องทำ 3D Model หรือ Character Animation ใน MVP
>
> Server ต้อง Authoritative: Client ส่ง Intent เท่านั้น ส่วน Server ตรวจสอบและคำนวณ HP, Damage, Loot, Inventory, Respawn และ Score ทั้งหมด
>
> ทำงานตาม Development Roadmap ในเอกสารทีละ Phase เริ่มจาก Phase 0 และ Phase 1 เท่านั้น ก่อนแก้โค้ดให้ Inspect Repository และสรุปแผนสั้น ๆ ทุกครั้ง เขียน Test ให้ Game Rules สำคัญ รัน Test / Build จริง และรายงานสิ่งที่ทำสำเร็จ สิ่งที่ยังไม่ทำ และคำสั่ง Run ให้ชัดเจน ห้ามข้าม Phase โดยไม่ตรวจ Acceptance Criteria ของ Phase ปัจจุบัน
>
> หากรายละเอียดใดไม่ได้กำหนด ให้ใช้ Proposed MVP Defaults ในเอกสาร อย่าเปลี่ยน Product Direction หรือเพิ่มระบบ Post-MVP เอง ให้ถามเฉพาะกรณีที่มีข้อขัดแย้งจริงซึ่งทำให้พัฒนาต่อไม่ได้

---

# 28. สรุป Source of Truth แบบสั้น

**Dungeon Rivals = Web Multiplayer + 2–4 Players + Independent Dungeon Paths + Turn-based PvE + Loot / Equipment + Respawn + Score-based Final PvP.**

**Frontend:** React + TypeScript + Vite  
**Backend:** Go + Fiber  
**Realtime:** WebSocket  
**Data:** Static JSON + In-memory Runtime State  
**Database:** ไม่มีใน MVP  
**Visual:** Card-like UI / Static Icon / Static Image, no 3D Model  
**Development:** ทำทีละ Phase, เริ่มจาก Lobby + Room + WebSocket ก่อน
