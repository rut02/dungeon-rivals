# How-To: Procedural 64-bit Dark Fantasy Pixel Art & Sprite Generation
> **คู่มือและแนวทางการสร้าง Sprite และภาพ Pixel Art คุณภาพสูงด้วย Code (Procedural Generation)**  
> ใช้สำหรับสร้าง Asset ในเกม Dungeon Rivals ทั้งไอเทม (Weapons/Armor), มอนสเตอร์ (Monsters/Bosses), และภาพฉาก/ภาพปก (Scene/Cover Art) เมื่อไม่มีโควตา AI หรือต้องการงานที่มีความคมชัดระดับพิกเซล 100%

---

## 1. ปรัชญาและหัวใจสำคัญของ Pixel Art ระดับ 64-bit

ในการสร้าง Pixel Art ยุค Dark Fantasy (สไตล์ *Blasphemous*, *Castlevania: Symphony of the Night*, *Dark Souls*):
1. **ห้ามวาดที่ความละเอียดสูงแล้วย่อ (Downscale แบบ Bilinear/Bicubic)**: ภาพจะมัวและสูญเสียเอกลักษณ์ของพิกเซล
2. **วาดบนผืนผ้าใบ Native Grid ขนาดเล็ก แล้วขยายด้วย Nearest-Neighbor (Integer Scaling)**:
   - **Item Icon / Equipment**: ออกแบบบน **32×32** หรือ **64×64 px** $\rightarrow$ ขยาย 4× เป็น **256×256 px**
   - **Monster / Hero Sprite**: ออกแบบบน **64×64** หรือ **96×96 px** $\rightarrow$ ขยาย 4× เป็น **256×256** หรือ **384×384 px**
   - **Scene / Title Cover (16:9)**: ออกแบบบน **400×225 px** $\rightarrow$ ขยาย 2× เป็น **800×450 px**
3. **การคุม Palette เม็ดสี (Color Ramping)**: แต่ละวัสดุต้องมี 4–6 โทนสี (Shadow, Midtone, Highlight, Core, Rim Light)
4. **Bayer Dithering (การทำเม็ดเกลี่ยแสงเงา)**: ช่วยให้เกิดการไล่เฉดที่นุ่มนวลแต่ยังคงความคลาสสิกของพิกเซล

---

## 2. Dark Fantasy Color Palettes (ชุดสีมาตรฐาน)

| วัสดุ / ธาตุ | รหัสสี (RGB) จากเข้มไปสว่าง | การนำไปใช้งาน |
|---|---|---|
| **Obsidian & Dark Steel** | `(16,14,22)` $\rightarrow$ `(28,24,38)` $\rightarrow$ `(48,42,62)` $\rightarrow$ `(82,74,102)` $\rightarrow$ `(142,134,168)` | หินออบซิเดียน, ชุดเกราะเหล็กดำ, ประตูคุกใต้ดิน |
| **Soulfire Cyan** | `(8,40,55)` $\rightarrow$ `(18,110,140)` $\rightarrow$ `(30,180,210)` $\rightarrow$ `(80,240,250)` $\rightarrow$ `(235,255,255)` | เปลววิญญาณ, ลำแสงเวท, อักขระรูน, ประกายตา Hero |
| **Eldritch Purple** | `(24,8,38)` $\rightarrow$ `(56,18,88)` $\rightarrow$ `(112,34,168)` $\rightarrow$ `(178,76,238)` $\rightarrow$ `(242,172,255)` | เวทความมืด, บรรยากาศหมอกใต้พิภพ, ออร่าบอส |
| **Tarnished Gold** | `(58,36,12)` $\rightarrow$ `(112,72,24)` $\rightarrow$ `(184,130,42)` $\rightarrow$ `(235,184,72)` $\rightarrow$ `(255,242,168)` | ทองโบราณ, มงกุฎ, ลวดลายประดับดาบ Excalibur |
| **Blood & Crimson** | `(44,8,12)` $\rightarrow$ `(92,16,26)` $\rightarrow$ `(168,26,42)` $\rightarrow$ `(232,54,72)` $\rightarrow$ `(255,132,145)` | คราบเลือด, จอกโลหิต, อัญมณีรูบี้, มอนสเตอร์โลหิต |
| **Volcanic Fire** | `(82,18,10)` $\rightarrow$ `(168,38,14)` $\rightarrow$ `(238,88,22)` $\rightarrow$ `(255,172,44)` $\rightarrow$ `(255,246,162)` | เปลวดาบ Flamebrand, ระเบิดเพลิง, ลาวา |
| **Ancient Bone** | `(48,42,34)` $\rightarrow$ `(88,80,68)` $\rightarrow$ `(148,138,122)` $\rightarrow$ `(204,196,180)` $\rightarrow$ `(246,242,232)` | โครงกระดูก, Bone Reaper, เศษซากศพ |

---

## 3. เทคนิคและอัลกอริทึมหลัก (Core Algorithms)

### 3.1 Bayer 4×4 Ordered Dithering (การเกลี่ยสีแบบเม็ดพิกเซล)
ใช้ตาราง Matrix 4×4 เพื่อตัดสินใจสลับเม็ดสีแทนการเบลอ ช่วยสร้างมิติเงาที่สวยงาม:

```python
BAYER4 = [
    [ 0,  8,  2, 10],
    [12,  4, 14,  6],
    [ 3, 11,  1,  9],
    [15,  7, 13,  5]
]

def dither_pixel(c_dark, c_light, factor, x, y):
    """
    factor: 0.0 (dark) ถึง 1.0 (light)
    """
    threshold = (BAYER4[y % 4][x % 4] + 0.5) / 16.0
    return c_light if factor > threshold else c_dark
```

### 3.2 Dynamic Outline Extraction (การใส่เส้นขอบ 1px อัตโนมัติ)
สำหรับ Sprite ไอเทมและตัวละคร เส้นขอบเข้มสีดำอมม่วง `(12, 10, 16)` รอบขอบนอกของภาพ จะช่วยให้ Sprite ลอยเด่นออกจากพื้นหลัง UI ในเกม:

```python
def apply_outline(image, outline_color=(12, 10, 16, 255)):
    w, h = image.size
    pixels = image.load()
    new_img = image.copy()
    new_pixels = new_img.load()

    for y in range(h):
        for x in range(w):
            if pixels[x, y][3] == 0: # พิกเซลโปร่งใส
                # ตรวจสอบ 8 ช่องรอบข้างว่ามีเนื้อภาพหรือไม่
                has_solid = False
                for dy in [-1, 0, 1]:
                    for dx in [-1, 0, 1]:
                        nx, ny = x + dx, y + dy
                        if 0 <= nx < w and 0 <= ny < h:
                            if pixels[nx, ny][3] > 0:
                                has_solid = True
                                break
                    if has_solid:
                        break
                if has_solid:
                    new_pixels[x, y] = outline_color
    return new_img
```

### 3.3 Perspective Floor & Steps (การวาดมุมมองมิติความลึก)
การวาดบันไดหรือพื้นทางเดินให้มีมิติ Perspective กว้างขึ้นเมื่อใกล้หน้าจอ:
- แปลงค่าตำแหน่งแนวตั้ง $y$ เป็นความคืบหน้า $t \in [0.0, 1.0]$ ด้วยสมการยกกำลัง $t^{1.3}$ หรือ $t^{1.5}$
- คำนวณความกว้าง: $w(t) = w_{\text{top}} + (w_{\text{bottom}} - w_{\text{top}}) \times t^{1.3}$
- แบ่งความสูงแต่ละขั้นออกเป็น 2 ส่วน: **Tread (หน้าบันไดที่มองจากบนลงล่าง: สีสว่างกว่า)** และ **Riser (สันขั้นบันไดแนวดิ่ง: เงาเข้ม)**

### 3.4 Volumetric God Rays & Particle Embers (ลำแสงและละอองไฟ)
1. **Light Rays**: ยิงรูปหลายเหลี่ยม Polygon แบบสามเหลี่ยมจากจุดกำเนิดแสง กระจายออกเป็นมุมแฉก และซ้อนด้วยค่า Alpha 20–45%
2. **Mist Fog**: คำนวณคลื่น Sine 2 คลื่นซ้อนกัน `sin(x * 0.04) * 8 + cos(y * 0.08) * 6` ที่ค่า Alpha บางเบา
3. **Embers**: สุ่มจุดพิกเซล 1×1 หรือ 2×2 สีขาวอมฟ้าและม่วงกระจายตัวใกล้แหล่งกำเนิดแสง

---

## 4. โครงสร้างแม่แบบ Code Generator (Python Pillow Template)

สคริปต์แม่แบบนี้สามารถคัดลอกไปสร้าง Sprite หรือภาพฉากได้ทันที:

```python
import math
import random
from PIL import Image, ImageDraw

class DarkFantasySpriteGenerator:
    def __init__(self, width=64, height=64, scale=4):
        self.w = width
        self.h = height
        self.scale = scale
        # โหมด RGBA พร้อมพื้นหลังโปร่งใส (สำหรับ Sprite) หรือทึบ (สำหรับ Scene)
        self.img = Image.new('RGBA', (self.w, self.h), (0, 0, 0, 0))
        self.draw = ImageDraw.Draw(self.img)

    def pset(self, x, y, color):
        """แต้มพิกเซลลงบนแคนวาส"""
        if 0 <= x < self.w and 0 <= y < self.h:
            self.img.putpixel((x, y), color)

    def pset_sym(self, x, y, color):
        """แต้มพิกเซลแบบสมมาตรซ้าย-ขวา (สำหรับดาบ โล่ ชุดเกราะ ตัวละคร)"""
        self.pset(x, y, color)
        self.pset(self.w - 1 - x, y, color)

    def draw_dither_rect(self, x, y, w, h, c_dark, c_light, factor=0.5):
        """วาดกล่องไล่โทนแบบ Bayer Dithering"""
        bayer = [
            [ 0,  8,  2, 10],
            [12,  4, 14,  6],
            [ 3, 11,  1,  9],
            [15,  7, 13,  5]
        ]
        for j in range(y, y + h):
            for i in range(x, x + w):
                thresh = (bayer[j % 4][i % 4] + 0.5) / 16.0
                col = c_light if factor > thresh else c_dark
                self.pset(i, j, col)

    def export(self, filepath):
        """ขยายภาพด้วย Nearest Neighbor และบันทึกเป็น PNG"""
        scaled = self.img.resize(
            (self.w * self.scale, self.h * self.scale),
            Image.Resampling.NEAREST
        )
        scaled.save(filepath, 'PNG')
        print(f"[Done] Saved: {filepath} ({scaled.size[0]}x{scaled.size[1]})")

# ตัวอย่างการสร้างไอเทม: Runic Broadsword (64x64 -> 256x256)
def generate_runic_sword(out_path):
    gen = DarkFantasySpriteGenerator(64, 64, scale=4)
    cx = 32

    # 1. ใบดาบ (Blade) - สี Dark Steel
    c_core = (220, 230, 245, 255)
    c_edge = (95, 102, 122, 255)
    c_shadow = (32, 34, 42, 255)
    c_rune = (80, 240, 255, 255)

    for y in range(8, 44):
        blade_w = 4 if y > 12 else max(1, (y - 8))
        for dx in range(1, blade_w + 1):
            gen.pset(cx - dx, y, c_shadow)
            gen.pset(cx + dx, y, c_edge)
        # สันแกนกลางดาบ
        gen.pset(cx, y, c_core)
        # สลักอักขระรูนเรืองแสง
        if y in [18, 22, 26, 30, 34]:
            gen.pset(cx, y, c_rune)

    # 2. กระบังดาบ (Crossguard) - โกธิกปีกมังกร
    for x in range(cx - 14, cx + 15):
        y_guard = 45 - int(math.sin(abs(x - cx) * 0.15) * 3)
        gen.pset(x, y_guard, (184, 130, 42, 255))
        gen.pset(x, y_guard + 1, (112, 72, 24, 255))

    # 3. ด้ามจับและหัวดาบ (Hilt & Pommel)
    for y in range(46, 56):
        c_hilt = (78, 46, 26, 255) if y % 2 == 0 else (42, 24, 14, 255)
        gen.pset(cx, y, c_hilt)
        gen.pset(cx - 1, y, c_hilt)

    # Pommel กะโหลก/ลูกแก้ว
    gen.pset(cx, 57, (184, 130, 42, 255))
    gen.pset(cx - 1, 57, (184, 130, 42, 255))

    gen.export(out_path)

if __name__ == '__main__':
    generate_runic_sword('example_sword.png')
```

---

## 5. เทียบเท่าในภาษา Go (อ้างอิง `backend/cmd/artgen/main.go`)

ในโปรเจกต์นี้มีเอนจินภาษา Go คุณภาพสูงพร้อมใช้งานอยู่แล้วที่ [backend/cmd/artgen/main.go](file:///c:/Users/admin/Documents/sandbox/dungeon-rivals/backend/cmd/artgen/main.go)
ซึ่งประกอบด้วย:
- โครงสร้าง `Sprite64` สำหรับตาราง 64×64 พิกเซล
- ฟังก์ชันวาดเส้น Bresenham `Line()` / `ThickLine()`
- ฟังก์ชันวาดรูปทรงเรขาคณิต `Rect()`, `FillCircle()`, `Circle()`
- ฟังก์ชันสมมาตรอัตโนมัติ `PsetSym()`, `RectSym()`
- อัลกอริทึมตัดขอบ `AddOutline()`
- ฟังก์ชันเรนเดอร์ Asset ครบทั้ง 50+ รายการในเกม

หากต้องการรันสร้าง Sprite ทั้งหมดใหม่ผ่าน Go:
```bash
cd backend/cmd/artgen
go run main.go
```

---

## 6. Checklist ก่อนนำ Sprite ไปใช้งานในเกม
1. [ ] **พื้นหลังโปร่งใส (Alpha = 0)**: สำหรับไอเทมและตัวละคร (ยกเว้นภาพปก/ภาพฉาก)
2. [ ] **ขอบเขตภาพ (Framing)**: ชิ้นงานไม่ควรชิดขอบจนเกินไป ควรเว้น Padding อย่างน้อย 2–4 พิกเซลใน Grid Native
3. [ ] **เส้นตัดขอบ (Outline)**: มีเส้นขอบทึบช่วยตัดพื้นหลังเวลาไปแสดงผลในช่อง Inventory หรือหน้าจอต่อสู้
4. [ ] **การตั้งชื่อไฟล์**: ให้ตรงกับ `id` ใน `items.json` หรือ `monsters.json` (เช่น `flamebrand.png`, `hero.png`)
5. [ ] **การจัดเก็บ**: บันทึกไว้ที่ `frontend/public/art/pixel/` เพื่อให้ระบบ Web UI เรียกใช้งานได้ทันที
