package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// 64x64 Native Grid Canvas (upscaled 4x to 256x256 for uniform, crisp 64-bit Dark Fantasy Pixel Art)
type Sprite64 struct {
	w, h   int
	pixels [][]color.RGBA
}

func NewSprite64() *Sprite64 {
	w, h := 64, 64
	p := make([][]color.RGBA, h)
	for y := 0; y < h; y++ {
		p[y] = make([]color.RGBA, w)
	}
	return &Sprite64{w: w, h: h, pixels: p}
}

func (s *Sprite64) Pset(x, y int, c color.RGBA) {
	if x >= 0 && x < s.w && y >= 0 && y < s.h {
		s.pixels[y][x] = c
	}
}

func (s *Sprite64) PsetSym(x, y int, c color.RGBA) {
	s.Pset(x, y, c)
	s.Pset(s.w-1-x, y, c)
}

func (s *Sprite64) Line(x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		s.Pset(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func (s *Sprite64) ThickLine(x0, y0, x1, y1, width int, c color.RGBA) {
	for w := -width / 2; w <= width/2; w++ {
		s.Line(x0+w, y0, x1+w, y1, c)
		s.Line(x0, y0+w, x1, y1+w, c)
	}
}

func (s *Sprite64) Rect(x, y, w, h int, c color.RGBA) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			s.Pset(i, j, c)
		}
	}
}

func (s *Sprite64) RectSym(x, y, w, h int, c color.RGBA) {
	s.Rect(x, y, w, h, c)
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			s.Pset(s.w-1-i, j, c)
		}
	}
}

func (s *Sprite64) FillCircle(cx, cy, r int, c color.RGBA) {
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				s.Pset(cx+x, cy+y, c)
			}
		}
	}
}

func (s *Sprite64) FillTriangle(ax, ay, bx, by, cx, cy int, c color.RGBA) {
	minX, maxX := min(ax, min(bx, cx)), max(ax, max(bx, cx))
	minY, maxY := min(ay, min(by, cy)), max(ay, max(by, cy))
	edge := func(x1, y1, x2, y2, x, y int) int {
		return (x-x1)*(y2-y1) - (y-y1)*(x2-x1)
	}
	area := edge(ax, ay, bx, by, cx, cy)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			u, v, w := edge(ax, ay, bx, by, x, y), edge(bx, by, cx, cy, x, y), edge(cx, cy, ax, ay, x, y)
			if (area > 0 && u >= 0 && v >= 0 && w >= 0) || (area < 0 && u <= 0 && v <= 0 && w <= 0) {
				s.Pset(x, y, c)
			}
		}
	}
}

func (s *Sprite64) Circle(cx, cy, r int, c color.RGBA) {
	x, y := r, 0
	err := 0
	for x >= y {
		s.Pset(cx+x, cy+y, c)
		s.Pset(cx+y, cy+x, c)
		s.Pset(cx-y, cy+x, c)
		s.Pset(cx-x, cy+y, c)
		s.Pset(cx-x, cy-y, c)
		s.Pset(cx-y, cy-x, c)
		s.Pset(cx+y, cy-x, c)
		s.Pset(cx+x, cy-y, c)
		if err <= 0 {
			y += 1
			err += 2*y + 1
		}
		if err > 0 {
			x -= 1
			err -= 2*x + 1
		}
	}
}

func (s *Sprite64) DitherRect(x, y, w, h int, c1, c2 color.RGBA) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if (i+j)%2 == 0 {
				s.Pset(i, j, c1)
			} else {
				s.Pset(i, j, c2)
			}
		}
	}
}

func (s *Sprite64) DitherRectSym(x, y, w, h int, c1, c2 color.RGBA) {
	s.DitherRect(x, y, w, h, c1, c2)
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if (i+j)%2 == 0 {
				s.Pset(s.w-1-i, j, c1)
			} else {
				s.Pset(s.w-1-i, j, c2)
			}
		}
	}
}

func (s *Sprite64) AddOutline(outCol color.RGBA) {
	copyP := make([][]color.RGBA, s.h)
	for y := 0; y < s.h; y++ {
		copyP[y] = make([]color.RGBA, s.w)
		copy(copyP[y], s.pixels[y])
	}
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			if copyP[y][x].A == 0 {
				hasNeighbor := false
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						ny, nx := y+dy, x+dx
						if ny >= 0 && ny < s.h && nx >= 0 && nx < s.w {
							if copyP[ny][nx].A > 0 {
								hasNeighbor = true
								break
							}
						}
					}
					if hasNeighbor {
						break
					}
				}
				if hasNeighbor {
					s.pixels[y][x] = outCol
				}
			}
		}
	}
}

func (s *Sprite64) SavePNG(path string, scale int) error {
	img := image.NewRGBA(image.Rect(0, 0, s.w*scale, s.h*scale))
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			c := s.pixels[y][x]
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.Set(x*scale+dx, y*scale+dy, c)
				}
			}
		}
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// 64-bit Dark Fantasy Multi-Tone Color Palettes
var (
	cOut   = color.RGBA{12, 10, 16, 255}
	cShade = color.RGBA{20, 17, 26, 255}

	// Obsidian & Dark Steel (5 tones)
	cObsidian0 = color.RGBA{16, 14, 22, 255}
	cObsidian1 = color.RGBA{28, 24, 38, 255}
	cObsidian2 = color.RGBA{48, 42, 62, 255}
	cObsidian3 = color.RGBA{82, 74, 102, 255}
	cObsidian4 = color.RGBA{142, 134, 168, 255}

	// Damascus Steel & Iron (5 tones)
	cSteel0 = color.RGBA{32, 34, 42, 255}
	cSteel1 = color.RGBA{58, 62, 76, 255}
	cSteel2 = color.RGBA{95, 102, 122, 255}
	cSteel3 = color.RGBA{152, 160, 184, 255}
	cSteel4 = color.RGBA{222, 228, 244, 255}

	// Tarnished Gothic Gold (5 tones)
	cGold0 = color.RGBA{58, 36, 12, 255}
	cGold1 = color.RGBA{112, 72, 24, 255}
	cGold2 = color.RGBA{184, 130, 42, 255}
	cGold3 = color.RGBA{235, 184, 72, 255}
	cGold4 = color.RGBA{255, 242, 168, 255}

	// Blood & Crimson (5 tones)
	cBlood0 = color.RGBA{44, 8, 12, 255}
	cBlood1 = color.RGBA{92, 16, 26, 255}
	cBlood2 = color.RGBA{168, 26, 42, 255}
	cBlood3 = color.RGBA{232, 54, 72, 255}
	cBlood4 = color.RGBA{255, 132, 145, 255}

	// Volcanic Magma & Fire (5 tones)
	cFire0 = color.RGBA{82, 18, 10, 255}
	cFire1 = color.RGBA{168, 38, 14, 255}
	cFire2 = color.RGBA{238, 88, 22, 255}
	cFire3 = color.RGBA{255, 172, 44, 255}
	cFire4 = color.RGBA{255, 246, 162, 255}

	// Glacial Frost & Ice (5 tones)
	cIce0 = color.RGBA{14, 38, 62, 255}
	cIce1 = color.RGBA{28, 76, 118, 255}
	cIce2 = color.RGBA{58, 138, 196, 255}
	cIce3 = color.RGBA{115, 202, 248, 255}
	cIce4 = color.RGBA{218, 244, 255, 255}

	// Eldritch Void & Cosmic Arcane (5 tones)
	cVoid0 = color.RGBA{24, 8, 38, 255}
	cVoid1 = color.RGBA{56, 18, 88, 255}
	cVoid2 = color.RGBA{112, 34, 168, 255}
	cVoid3 = color.RGBA{178, 76, 238, 255}
	cVoid4 = color.RGBA{242, 172, 255, 255}

	// Necrotic Venom & Acid (5 tones)
	cVenom0 = color.RGBA{12, 38, 18, 255}
	cVenom1 = color.RGBA{26, 84, 38, 255}
	cVenom2 = color.RGBA{48, 154, 68, 255}
	cVenom3 = color.RGBA{92, 224, 118, 255}
	cVenom4 = color.RGBA{184, 255, 198, 255}

	// Ancient Weathered Bone (5 tones)
	cBone0 = color.RGBA{48, 42, 34, 255}
	cBone1 = color.RGBA{88, 80, 68, 255}
	cBone2 = color.RGBA{148, 138, 122, 255}
	cBone3 = color.RGBA{204, 196, 180, 255}
	cBone4 = color.RGBA{246, 242, 232, 255}

	// Cured Leather & Wood (5 tones)
	cWood0 = color.RGBA{42, 24, 14, 255}
	cWood1 = color.RGBA{78, 46, 26, 255}
	cWood2 = color.RGBA{128, 78, 44, 255}
	cWood3 = color.RGBA{182, 122, 74, 255}
	cWood4 = color.RGBA{228, 174, 126, 255}
)

func main() {
	outDir := os.Getenv("ART_DIR")
	if outDir == "" {
		outDir = filepath.Join("..", "..", "frontend", "public", "art", "pixel")
	}
	fmt.Printf("=== 64-bit Dark Fantasy Pixel Art Generator ===\n")
	fmt.Printf("Output: %s (Format: 256x256 PNG, Transparent Background)\n", outDir)
	if os.Getenv("ART_ONLY_NEW") == "1" {
		genClassAndSummonSprites64(outDir)
		fmt.Println("=== New class, spirit, and grimoire sprites generated ===")
		return
	}

	genHero64(filepath.Join(outDir, "hero.png"))
	genClassAndSummonSprites64(outDir)
	genMonsters64(outDir)
	genItems64(outDir)

	fmt.Println("=== All 50 Dark Fantasy 64-bit Assets Successfully Generated ===")
}

func genClassAndSummonSprites64(dir string) {
	// Character silhouettes use distinct equipment, poses and layered palette ramps.
	{
		s := NewSprite64()
		s.Rect(16, 27, 9, 27, cBlood0) // split cape behind the plate
		s.Line(17, 48, 13, 58, cBlood1)
		s.Rect(25, 42, 6, 16, cSteel0)
		s.Rect(34, 42, 6, 16, cSteel0)
		s.Rect(22, 56, 10, 5, cSteel2)
		s.Rect(33, 56, 10, 5, cSteel2)
		s.Rect(21, 25, 22, 20, cSteel1)
		s.Rect(24, 28, 16, 13, cSteel2)
		s.Rect(27, 29, 10, 2, cSteel3)
		s.FillTriangle(24, 31, 40, 31, 32, 42, cSteel3)
		s.FillTriangle(27, 33, 37, 33, 32, 40, cSteel2)
		s.Line(25, 31, 25, 40, cSteel4)
		s.Line(39, 31, 39, 40, cSteel1)
		s.Line(31, 31, 31, 39, cGold3)
		s.Line(32, 31, 32, 39, cGold2)
		s.FillCircle(32, 36, 2, cGold3)
		s.Pset(32, 36, cIce3)
		s.Rect(17, 26, 8, 7, cSteel2)
		s.Rect(39, 26, 8, 7, cSteel0)
		s.Line(18, 27, 23, 27, cSteel4)
		s.Line(40, 27, 45, 27, cSteel2)
		s.Line(17, 28, 12, 23, cSteel3)
		s.Rect(25, 10, 14, 15, cSteel0)
		s.Rect(23, 15, 18, 8, cSteel1)
		s.Rect(25, 18, 14, 3, cObsidian0)
		s.Rect(27, 18, 3, 2, cIce3)
		s.Rect(34, 18, 3, 2, cIce3)
		s.Rect(29, 22, 6, 2, cSteel3)
		s.Rect(9, 31, 10, 18, cSteel0) // kite shield
		s.Rect(11, 33, 6, 13, cSteel2)
		s.Line(14, 34, 14, 44, cGold3)
		s.Pset(14, 38, cGold4)
		s.ThickLine(52, 12, 48, 48, 2, cSteel4)
		s.Line(49, 19, 47, 38, cIce3)
		s.Line(44, 46, 54, 48, cGold3)
		s.Rect(47, 48, 4, 7, cWood1)
		s.FillCircle(49, 56, 2, cGold3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "hero_knight.png"), 4)
	}
	{
		s := NewSprite64()
		// A pointed, wide-brimmed hat sits over a readable face and mantle.
		s.FillTriangle(24, 16, 30, 3, 39, 16, cVoid1)
		s.Line(25, 14, 30, 4, cVoid3)
		s.Pset(30, 3, cIce4)
		s.Line(18, 16, 45, 16, cVoid0)
		s.Line(20, 15, 43, 15, cVoid3)
		s.Rect(23, 16, 18, 13, cVoid0)
		s.Rect(26, 17, 12, 9, cWood2)
		s.Rect(27, 18, 9, 5, cWood3)
		s.Rect(26, 22, 12, 5, cVoid0) // shadowed lower face
		s.Rect(27, 20, 3, 2, cIce4)
		s.Rect(35, 20, 3, 2, cIce4)
		s.Pset(29, 20, cVoid3)
		s.Pset(37, 20, cVoid3)
		s.Line(28, 25, 36, 25, cGold2)
		// Split cloak with flared cloth and a bright inner lining.
		s.FillTriangle(22, 29, 42, 29, 52, 57, cVoid0)
		s.FillTriangle(22, 29, 52, 57, 12, 57, cVoid1)
		s.FillTriangle(27, 33, 37, 33, 43, 54, cVoid2)
		s.FillTriangle(27, 33, 43, 54, 21, 54, cVoid1)
		s.Rect(16, 30, 8, 8, cVoid2)
		s.Rect(40, 30, 8, 8, cVoid2)
		s.Line(17, 34, 13, 43, cVoid3)
		s.Line(47, 34, 51, 43, cVoid3)
		s.Line(24, 31, 23, 51, cGold2)
		s.Line(40, 31, 41, 51, cGold2)
		s.Line(31, 33, 31, 52, cIce3)
		s.Line(32, 33, 32, 52, cIce2)
		s.FillCircle(32, 41, 6, cGold1)
		s.FillCircle(32, 41, 4, cVoid3)
		s.FillCircle(32, 41, 2, cIce4)
		s.Line(24, 48, 28, 51, cVoid4)
		s.Line(40, 48, 36, 51, cVoid4)
		s.Rect(19, 56, 10, 5, cObsidian0)
		s.Rect(35, 56, 10, 5, cObsidian0)
		s.Line(19, 60, 28, 60, cSteel3)
		s.Line(35, 60, 44, 60, cSteel3)
		// rune staff with a faceted crystal
		s.ThickLine(53, 10, 53, 58, 2, cWood1)
		s.Line(53, 14, 53, 47, cWood3)
		s.FillCircle(53, 9, 7, cGold1)
		s.FillCircle(53, 9, 5, cVoid2)
		s.FillTriangle(49, 10, 53, 2, 57, 10, cVoid3)
		s.Pset(53, 4, cIce4)
		s.Pset(50, 9, cVoid4)
		s.Pset(56, 9, cIce4)
		s.Line(49, 16, 57, 16, cGold3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "hero_mage.png"), 4)
	}
	{
		s := NewSprite64()
		// Asymmetric hood, mask and short red scarf
		s.Rect(20, 11, 19, 14, cObsidian0)
		s.Rect(17, 16, 23, 9, cObsidian1)
		s.Rect(24, 19, 14, 8, cWood1)
		s.Rect(24, 21, 5, 2, cBlood4)
		s.Rect(34, 21, 4, 2, cBlood4)
		s.FillTriangle(20, 17, 39, 17, 30, 8, cObsidian2)
		s.Line(21, 16, 29, 9, cObsidian4)
		s.Line(39, 16, 33, 10, cBlood2)
		s.Rect(26, 24, 11, 3, cObsidian0) // shadow veil
		s.Line(18, 19, 11, 28, cBlood1)
		s.Rect(13, 27, 10, 23, cBlood0)
		s.Rect(24, 26, 17, 18, cObsidian1)
		s.Rect(27, 29, 12, 12, cBlood1)
		s.FillTriangle(24, 28, 41, 28, 32, 43, cBlood2)
		s.FillTriangle(26, 30, 39, 30, 32, 41, cBlood1)
		s.Line(27, 28, 39, 40, cSteel3)
		s.Line(28, 30, 38, 40, cGold1)
		s.Rect(30, 32, 4, 2, cGold3)
		s.Rect(31, 37, 3, 2, cBlood4)
		s.Rect(23, 43, 7, 15, cObsidian0)
		s.Rect(35, 43, 7, 15, cObsidian0)
		s.Rect(20, 56, 11, 5, cSteel1)
		s.Rect(34, 56, 11, 5, cSteel1)
		s.Line(22, 57, 28, 57, cSteel3)
		s.Line(36, 57, 43, 57, cSteel3)
		s.Rect(22, 40, 22, 3, cGold1)
		// paired daggers, aimed outward
		s.ThickLine(14, 29, 6, 15, 2, cSteel3)
		s.Line(7, 15, 10, 19, cIce4)
		s.Rect(12, 29, 5, 3, cWood2)
		s.ThickLine(45, 30, 55, 16, 2, cSteel4)
		s.Line(54, 15, 51, 20, cBlood4)
		s.Rect(43, 30, 5, 3, cWood2)
		s.Pset(29, 36, cGold3)
		s.Pset(36, 36, cGold3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "hero_rogue.png"), 4)
	}
	{
		s := NewSprite64()
		// Ranger hood, face wrap, shoulder mantle and leather travel gear
		s.Rect(22, 9, 17, 16, cWood1)
		s.Rect(19, 15, 22, 10, cWood2)
		s.Rect(24, 18, 13, 7, cObsidian0)
		s.Rect(26, 19, 3, 2, cIce3)
		s.Rect(34, 19, 3, 2, cIce3)
		s.FillTriangle(21, 18, 40, 18, 30, 8, cWood3)
		s.Line(22, 17, 29, 10, cWood4)
		s.Line(40, 17, 35, 11, cWood1)
		s.Rect(24, 22, 14, 4, cObsidian1) // face scarf
		s.Line(25, 22, 37, 22, cSteel3)
		s.Rect(17, 25, 29, 19, cWood0)
		s.Rect(21, 27, 21, 14, cWood2)
		s.FillTriangle(18, 27, 45, 27, 32, 45, cWood3)
		s.FillTriangle(21, 29, 42, 29, 32, 42, cWood2)
		s.Line(21, 27, 39, 41, cGold3)
		s.Line(23, 28, 40, 42, cWood4)
		s.Rect(27, 32, 9, 3, cObsidian1)
		s.Rect(30, 31, 5, 5, cGold2)
		s.Pset(32, 33, cIce4)
		s.Rect(20, 42, 9, 15, cWood1)
		s.Rect(35, 42, 9, 15, cWood1)
		s.Rect(18, 55, 13, 5, cObsidian0)
		s.Rect(34, 55, 13, 5, cObsidian0)
		s.Line(19, 56, 28, 56, cWood4)
		s.Line(35, 56, 44, 56, cWood4)
		s.Rect(16, 29, 6, 17, cWood3)
		// quiver and arrow feathers
		s.Rect(42, 24, 7, 19, cBlood0)
		s.Rect(44, 26, 3, 13, cWood3)
		s.Line(43, 29, 47, 29, cGold2)
		s.Line(43, 23, 41, 15, cSteel3)
		s.Line(46, 23, 46, 13, cIce4)
		s.Line(49, 23, 52, 15, cSteel3)
		// tall recurved bow and taut string
		s.Line(11, 11, 7, 31, cWood3)
		s.Line(7, 31, 11, 52, cWood2)
		s.Line(11, 11, 16, 31, cWood1)
		s.Line(16, 31, 11, 52, cWood1)
		s.Line(11, 11, 11, 52, cIce3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "hero_ranger.png"), 4)
	}

	// Elemental spirits have distinct silhouettes and readable focal details.
	{
		s := NewSprite64()
		// Tapered ember body, layered flame tongues, and a hot inner core.
		s.FillTriangle(21, 35, 43, 35, 32, 59, cFire0)
		s.FillTriangle(22, 38, 42, 38, 32, 54, cFire1)
		s.FillTriangle(25, 40, 39, 40, 32, 51, cFire2)
		s.FillTriangle(18, 37, 26, 21, 29, 42, cFire1)
		s.FillTriangle(22, 30, 27, 18, 31, 40, cFire3)
		s.FillTriangle(36, 41, 39, 20, 47, 34, cFire1)
		s.FillTriangle(35, 39, 39, 15, 43, 33, cFire2)
		s.FillTriangle(27, 26, 31, 10, 34, 28, cFire3)
		s.FillTriangle(33, 27, 41, 7, 39, 29, cFire2)
		s.FillCircle(32, 33, 11, cFire2)
		s.FillCircle(32, 32, 8, cFire3)
		s.FillCircle(32, 33, 5, cFire4)
		s.Rect(26, 30, 4, 3, cObsidian0)
		s.Rect(36, 30, 4, 3, cObsidian0)
		s.Pset(27, 30, cGold4)
		s.Pset(37, 30, cGold4)
		s.Line(27, 37, 32, 39, cFire0)
		s.Line(37, 37, 32, 39, cFire0)
		s.ThickLine(22, 39, 14, 34, 2, cFire2)
		s.ThickLine(42, 39, 50, 33, 2, cFire2)
		s.FillCircle(13, 33, 3, cFire3)
		s.FillCircle(51, 32, 3, cFire3)
		s.Pset(13, 32, cFire4)
		s.Pset(51, 31, cFire4)
		s.Line(28, 46, 25, 51, cFire4)
		s.Line(36, 48, 39, 53, cFire4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "summon_fire.png"), 4)
	}
	{
		s := NewSprite64()
		// Broad rock silhouette, planted feet and a glowing core
		s.Rect(14, 25, 10, 15, cBone0)
		s.Rect(40, 24, 10, 17, cBone0)
		s.Rect(20, 16, 25, 10, cBone1)
		s.Rect(16, 24, 33, 18, cBone2)
		s.Rect(20, 39, 25, 12, cBone1)
		s.Rect(18, 48, 12, 10, cBone0)
		s.Rect(34, 48, 13, 10, cBone0)
		s.Rect(20, 55, 13, 5, cBone2)
		s.Rect(33, 55, 15, 5, cBone2)
		// faceted head and amber eyes
		s.Rect(23, 8, 18, 11, cBone0)
		s.Rect(26, 10, 12, 8, cBone2)
		s.Rect(25, 14, 4, 3, cFire3)
		s.Rect(35, 14, 4, 3, cFire3)
		s.Pset(26, 14, cFire4)
		s.Pset(36, 14, cFire4)
		s.Line(29, 19, 25, 24, cBone3)
		s.Line(36, 19, 41, 24, cBone1)
		// cracks, moss and chest sigil
		s.Line(18, 30, 24, 34, cBone0)
		s.Line(45, 29, 39, 34, cBone0)
		s.Line(21, 25, 25, 31, cBone3)
		s.Line(42, 25, 39, 31, cBone3)
		s.Line(18, 40, 24, 43, cBone3)
		s.Line(46, 40, 40, 44, cBone3)
		s.Line(21, 47, 25, 52, cBone3)
		s.Line(42, 47, 39, 53, cBone3)
		s.Rect(28, 29, 10, 10, cBone0)
		s.FillCircle(33, 34, 4, cGold2)
		s.Pset(33, 34, cFire4)
		s.Line(32, 25, 35, 28, cBone4)
		s.Line(34, 28, 31, 31, cBone3)
		s.Rect(12, 34, 4, 10, cBone1)
		s.Rect(48, 34, 4, 10, cBone1)
		s.Rect(12, 43, 8, 5, cBone0)
		s.Rect(46, 43, 8, 5, cBone0)
		s.Rect(18, 27, 4, 3, cVenom2)
		s.Rect(42, 38, 4, 3, cVenom2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "summon_earth.png"), 4)
	}
	{
		s := NewSprite64()
		// Water elemental with a droplet head and layered curling wave body
		s.Line(24, 24, 32, 10, cIce2)
		s.Line(32, 10, 41, 24, cIce3)
		s.Rect(23, 22, 19, 11, cIce2)
		s.Rect(25, 25, 15, 7, cIce3)
		s.Rect(28, 27, 3, 2, cIce0)
		s.Rect(36, 27, 3, 2, cIce0)
		s.FillTriangle(22, 31, 42, 31, 50, 48, cIce1)
		s.FillTriangle(22, 31, 50, 48, 32, 56, cIce2)
		s.FillTriangle(22, 31, 32, 56, 13, 48, cIce2)
		s.FillTriangle(25, 35, 39, 35, 43, 47, cIce3)
		s.FillTriangle(25, 35, 43, 47, 32, 52, cIce2)
		s.FillTriangle(25, 35, 32, 52, 20, 47, cIce3)
		s.Line(13, 49, 8, 56, cIce3)
		s.Line(45, 49, 54, 56, cIce3)
		s.Line(24, 36, 38, 36, cIce4)
		s.Line(19, 43, 26, 46, cIce4)
		s.Line(37, 45, 45, 41, cIce4)
		s.Pset(21, 42, cIce4)
		s.Pset(44, 43, cIce4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "summon_water.png"), 4)
	}
	{
		s := NewSprite64()
		// Wind spirit: airy mask surrounded by crossing gust ribbons
		s.ThickLine(12, 20, 23, 24, 2, cIce2)
		s.ThickLine(23, 24, 28, 31, 2, cIce3)
		s.ThickLine(52, 18, 41, 23, 2, cSteel4)
		s.ThickLine(41, 23, 37, 31, 2, cIce3)
		s.ThickLine(10, 43, 22, 39, 2, cIce2)
		s.ThickLine(22, 39, 27, 33, 2, cIce3)
		s.ThickLine(54, 45, 42, 39, 2, cSteel4)
		s.ThickLine(42, 39, 37, 33, 2, cIce3)
		s.Rect(25, 25, 14, 16, cIce1)
		s.Rect(28, 28, 8, 9, cIce3)
		s.Rect(28, 31, 3, 2, cObsidian0)
		s.Rect(34, 31, 3, 2, cObsidian0)
		s.Pset(30, 31, cIce4)
		s.Pset(36, 31, cIce4)
		s.Line(31, 8, 27, 17, cIce4)
		s.Line(34, 8, 39, 17, cIce3)
		s.Line(20, 52, 28, 47, cIce3)
		s.Line(44, 52, 36, 47, cSteel4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "summon_wind.png"), 4)
	}
	{
		s := NewSprite64()
		// Spellbook with a visible spine, inset frame, four elemental seals.
		s.Rect(13, 17, 37, 35, cWood0)
		s.Rect(17, 14, 30, 39, cGold1)
		s.Rect(19, 17, 25, 33, cObsidian1)
		s.Rect(22, 20, 19, 27, cVoid0)
		s.Rect(17, 14, 4, 39, cWood3)
		s.Line(20, 17, 20, 52, cGold3)
		s.Line(22, 18, 44, 18, cGold2)
		s.Line(22, 51, 44, 51, cGold2)
		s.FillCircle(28, 27, 4, cFire2)
		s.FillCircle(38, 27, 4, cBone2)
		s.FillCircle(28, 40, 4, cIce2)
		s.FillCircle(38, 40, 4, cIce3)
		s.Pset(28, 26, cFire4)
		s.Pset(38, 26, cGold3)
		s.Pset(28, 39, cIce4)
		s.Pset(38, 39, cIce4)
		s.Rect(48, 31, 4, 9, cGold2)
		s.Pset(50, 34, cFire4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "elemental_grimoire.png"), 4)
	}
	{
		s := NewSprite64()
		// Travel flask with metal cap and luminous wind glyph inside.
		s.Rect(28, 9, 9, 7, cGold2)
		s.Rect(25, 15, 15, 7, cIce3)
		s.Rect(21, 21, 23, 29, cIce1)
		s.Rect(18, 28, 29, 19, cIce2)
		s.Rect(22, 45, 21, 7, cIce1)
		s.Rect(25, 23, 15, 24, cIce3)
		s.Rect(28, 27, 9, 15, cIce1)
		s.ThickLine(29, 37, 35, 30, 2, cIce4)
		s.Pset(34, 29, cIce4)
		s.Rect(23, 29, 3, 11, cIce4)
		s.Pset(43, 32, cGold4)
		s.Pset(17, 25, cIce3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "wind_draught.png"), 4)
	}
}

// =========================================================================
// 1. HERO (Dark Knight Slayer)
// =========================================================================
func genHero64(path string) {
	s := NewSprite64()
	// Tattered Hood & Visor
	s.RectSym(24, 8, 8, 6, cObsidian1)
	s.RectSym(22, 14, 10, 10, cObsidian2)
	s.DitherRectSym(23, 11, 9, 3, cObsidian0, cObsidian1)
	// Cyan Glowing Soul Slit Eyes
	s.Rect(26, 18, 4, 2, cIce3)
	s.Rect(34, 18, 4, 2, cIce3)
	s.Pset(27, 18, cIce4)
	s.Pset(35, 18, cIce4)

	// Spiked Gothic Pauldrons
	s.RectSym(14, 22, 9, 8, cSteel1)
	s.RectSym(13, 20, 4, 3, cSteel2) // shoulder spikes
	s.PsetSym(13, 19, cSteel4)
	s.DitherRectSym(15, 24, 7, 5, cSteel0, cSteel1)

	// Ornate Breastplate with Runic Spine
	s.RectSym(22, 24, 10, 16, cObsidian1)
	s.RectSym(24, 26, 8, 12, cSteel1)
	s.Line(31, 26, 31, 38, cGold2)
	s.Line(32, 26, 32, 38, cGold2)
	s.Pset(31, 30, cGold4)
	s.Pset(32, 30, cGold4)

	// Tattered Cloak Billowing Behind
	s.RectSym(11, 28, 5, 26, cObsidian0)
	s.RectSym(13, 44, 4, 10, cShade)

	// Broadsword in Hand
	s.ThickLine(48, 10, 48, 46, 2, cSteel4)
	s.ThickLine(49, 12, 49, 44, 2, cSteel2)
	s.Line(44, 46, 52, 46, cGold3)  // crossguard
	s.Rect(47, 47, 3, 5, cWood0)    // grip
	s.FillCircle(48, 54, 2, cGold3) // pommel
	// Runic etchings on blade
	s.Line(48, 18, 48, 36, cIce3)

	// Armored Greaves
	s.RectSym(22, 42, 6, 18, cSteel1)
	s.RectSym(23, 44, 4, 14, cSteel0)
	s.RectSym(21, 56, 7, 5, cSteel2)

	s.AddOutline(cOut)
	_ = s.SavePNG(path, 4) // 64 * 4 = 256
}

// =========================================================================
// 2. MONSTERS (11 Creatures & Bosses)
// =========================================================================
func genMonsters64(dir string) {
	// 1. Bat Swarm
	{
		s := NewSprite64()
		drawBat64 := func(cx, cy, scale int) {
			s.FillCircle(cx, cy, scale, cObsidian0)
			s.Pset(cx-1, cy-1, cBlood3)
			s.Pset(cx+1, cy-1, cBlood3)
			s.Line(cx-scale*4, cy-scale*2, cx-scale, cy, cObsidian1)
			s.Line(cx+scale, cy, cx+scale*4, cy-scale*2, cObsidian1)
			s.Line(cx-scale*3, cy+scale*2, cx-scale, cy+scale, cObsidian0)
			s.Line(cx+scale, cy+scale, cx+scale*3, cy+scale*2, cObsidian0)
		}
		drawBat64(32, 18, 3)
		drawBat64(18, 34, 2)
		drawBat64(46, 38, 2)
		drawBat64(28, 48, 1)
		drawBat64(38, 48, 1)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "bat_swarm.png"), 4)
	}

	// 2. Goblin
	{
		s := NewSprite64()
		// Head & Pointed Ears
		s.RectSym(24, 14, 8, 14, cVenom1)
		s.Line(16, 18, 23, 22, cVenom2) // long pointed ear
		s.Line(47, 18, 40, 22, cVenom2)
		s.Line(16, 19, 23, 25, cVenom0)
		s.Line(47, 19, 40, 25, cVenom0)
		// Glowing yellow eyes
		s.Rect(26, 20, 3, 2, cFire3)
		s.Rect(35, 20, 3, 2, cFire3)
		s.Pset(27, 20, cFire4)
		s.Pset(36, 20, cFire4)
		// Fanged jagged mouth
		s.Line(28, 26, 35, 26, cObsidian0)
		s.Pset(29, 25, cBone4)
		s.Pset(34, 25, cBone4)
		// Tattered leather tunic
		s.RectSym(22, 28, 10, 16, cWood1)
		s.DitherRectSym(24, 30, 8, 12, cWood0, cWood1)
		// Bone Dagger
		s.Line(14, 26, 10, 14, cBone3)
		s.Line(15, 26, 11, 14, cBone1)
		s.Pset(10, 13, cBlood2) // blood-stained tip
		// Legs & Feet
		s.RectSym(24, 44, 6, 14, cVenom0)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "goblin.png"), 4)
	}

	// 3. Slime
	{
		s := NewSprite64()
		s.FillCircle(32, 40, 18, cVenom0)
		s.FillCircle(32, 36, 16, cVenom1)
		s.FillCircle(30, 32, 12, cVenom2)
		s.FillCircle(28, 28, 7, cVenom3)
		// Trapped floating skull inside
		s.FillCircle(34, 36, 6, cBone2)
		s.Rect(32, 34, 2, 3, cObsidian0) // eye socket
		s.Rect(36, 34, 2, 3, cObsidian0)
		s.Line(33, 39, 36, 39, cBone0)
		// Acidic bubbling highlights
		s.FillCircle(22, 42, 3, cVenom3)
		s.Pset(22, 41, cVenom4)
		s.FillCircle(42, 38, 2, cVenom4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "slime.png"), 4)
	}

	// 4. Goblin Archer
	{
		s := NewSprite64()
		// Hooded head & ears
		s.RectSym(22, 12, 10, 14, cWood0)
		s.RectSym(25, 16, 7, 8, cVenom1)
		s.Rect(27, 18, 3, 2, cFire3) // yellow eyes
		s.Rect(34, 18, 3, 2, cFire3)
		// Camouflage cloak
		s.RectSym(20, 26, 12, 18, cWood1)
		// Drawn composite shortbow
		s.Line(12, 12, 12, 50, cWood2)
		s.Line(12, 12, 24, 32, cBone3) // drawn string
		s.Line(12, 50, 24, 32, cBone3)
		s.Line(10, 32, 28, 32, cVenom3) // glowing poison arrow
		s.FillCircle(10, 32, 2, cVenom4)
		// Legs
		s.RectSym(24, 44, 5, 14, cWood0)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "goblin_archer.png"), 4)
	}

	// 5. Skeleton Guard
	{
		s := NewSprite64()
		// Horned Iron Helm
		s.RectSym(24, 8, 8, 8, cSteel1)
		s.Line(18, 6, 24, 10, cBone2) // Horns
		s.Line(45, 6, 39, 10, cBone2)
		// Skull & Cyan Soul Flames
		s.RectSym(24, 16, 8, 8, cBone2)
		s.Rect(26, 18, 3, 3, cIce3)
		s.Rect(35, 18, 3, 3, cIce3)
		s.Pset(27, 19, cIce4)
		s.Pset(36, 19, cIce4)
		// Exposed Ribcage & Spine
		s.Line(24, 26, 39, 26, cBone3)
		s.Line(25, 29, 38, 29, cBone2)
		s.Line(26, 32, 37, 32, cBone1)
		s.Line(31, 26, 32, 40, cBone3)
		// Iron Shield in Left Hand
		s.FillCircle(16, 34, 10, cSteel1)
		s.Circle(16, 34, 8, cSteel0)
		s.FillCircle(16, 34, 3, cGold2)
		// Rusted Arming Sword in Right Hand
		s.ThickLine(48, 12, 48, 42, 2, cSteel2)
		s.Line(44, 42, 52, 42, cSteel1)
		s.Pset(48, 24, cBlood1) // blood stains
		// Bony Legs
		s.Line(26, 40, 26, 58, cBone2)
		s.Line(37, 40, 37, 58, cBone2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "skeleton_guard.png"), 4)
	}

	// 6. Orc Warrior
	{
		s := NewSprite64()
		// Massive Head & Tusks
		s.RectSym(22, 10, 10, 16, cVenom0)
		s.Rect(25, 16, 4, 3, cBlood3) // blood-red eyes
		s.Rect(35, 16, 4, 3, cBlood3)
		s.Pset(26, 17, cBlood4)
		s.Pset(36, 17, cBlood4)
		// Protruding tusks
		s.Line(24, 24, 23, 20, cBone4)
		s.Line(39, 24, 40, 20, cBone4)
		// Brutal Spiked Iron Pauldrons
		s.RectSym(10, 24, 12, 10, cSteel0)
		s.Line(10, 22, 14, 24, cSteel3)
		s.Line(53, 22, 49, 24, cSteel3)
		// Muscular Torso with Battle Scars
		s.RectSym(20, 26, 12, 18, cVenom0)
		s.Line(24, 30, 36, 36, cBlood1) // slashed scar
		// Giant Executioner Battleaxe
		s.ThickLine(52, 8, 52, 54, 2, cWood1)
		s.Rect(44, 12, 8, 16, cSteel2)
		s.Line(42, 10, 42, 28, cSteel4)
		// Legs
		s.RectSym(22, 44, 8, 16, cSteel0)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "orc_warrior.png"), 4)
	}

	// 7. Elite Knight (Corrupted Black Knight)
	{
		s := NewSprite64()
		// Horned Gothic Black Helm
		s.RectSym(22, 8, 10, 14, cObsidian0)
		s.Line(16, 4, 22, 10, cGold2) // ornate crest
		s.Line(47, 4, 41, 10, cGold2)
		// Blood-red Visor Slit
		s.Line(25, 16, 38, 16, cBlood3)
		s.Line(26, 17, 37, 17, cBlood2)
		// Massive Gothic Spiked Shoulders
		s.RectSym(12, 20, 10, 10, cObsidian1)
		s.PsetSym(11, 19, cGold3)
		// Heavy Black Armor & Tattered Crimson Cape
		s.RectSym(20, 22, 12, 20, cObsidian0)
		s.Line(31, 24, 31, 40, cGold2)
		s.Line(32, 24, 32, 40, cGold2)
		s.RectSym(8, 26, 6, 26, cBlood1) // cape
		// Corrupted Greatsword
		s.ThickLine(50, 6, 50, 48, 2, cVoid3)
		s.ThickLine(51, 8, 51, 46, 2, cSteel4)
		s.Line(46, 48, 54, 48, cGold2)
		// Armored Greaves
		s.RectSym(22, 42, 7, 18, cObsidian1)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "elite_knight.png"), 4)
	}

	// 8. Shadow Assassin
	{
		s := NewSprite64()
		// Obsidian Mask & Violet Eyes
		s.RectSym(24, 10, 8, 12, cShade)
		s.Rect(26, 15, 3, 2, cVoid3)
		s.Rect(35, 15, 3, 2, cVoid3)
		s.Pset(27, 15, cVoid4)
		s.Pset(36, 15, cVoid4)
		// Phantom Cloak & Wisps of Violet Smoke
		s.RectSym(20, 22, 12, 22, cVoid0)
		s.DitherRectSym(14, 28, 8, 18, cVoid0, cVoid1)
		s.FillCircle(14, 44, 4, cVoid2)
		s.FillCircle(50, 42, 5, cVoid2)
		// Dual Poisoned Kamas / Sickles
		s.Line(10, 16, 10, 36, cSteel2)
		s.Line(6, 16, 14, 16, cVenom3) // poisoned edge
		s.Line(54, 16, 54, 36, cSteel2)
		s.Line(50, 16, 58, 16, cVenom3)
		// Boots
		s.RectSym(24, 44, 5, 16, cObsidian0)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "shadow_assassin.png"), 4)
	}

	// 9. Warden of Ash (Floor 1 Boss)
	{
		s := NewSprite64()
		// Horned Skull Crown of Cinders
		s.RectSym(22, 6, 10, 14, cObsidian0)
		s.Line(14, 2, 22, 8, cFire3) // burning horns
		s.Line(49, 2, 41, 8, cFire3)
		s.Line(16, 4, 23, 10, cFire2)
		s.Line(47, 4, 40, 10, cFire2)
		// Blazing Molten Eyes
		s.Rect(26, 14, 3, 3, cFire3)
		s.Rect(35, 14, 3, 3, cFire3)
		s.Pset(27, 15, cFire4)
		s.Pset(36, 15, cFire4)
		// Towering Magma Furnace Chest
		s.RectSym(16, 20, 16, 22, cObsidian0)
		s.RectSym(24, 24, 8, 14, cFire1)
		s.FillCircle(32, 31, 5, cFire3)
		s.FillCircle(32, 31, 2, cFire4) // White-hot heart
		// Colossal Smoking Volcanic Warhammer
		s.ThickLine(52, 4, 52, 56, 3, cWood0)
		s.Rect(44, 8, 16, 14, cObsidian1)
		s.DitherRect(46, 10, 12, 10, cFire2, cFire3)
		// Ash Greaves
		s.RectSym(20, 42, 8, 18, cObsidian0)
		s.DitherRectSym(22, 54, 5, 4, cFire1, cFire2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "warden_of_ash.png"), 4)
	}

	// 10. Frost Colossus (Floor 2 Boss)
	{
		s := NewSprite64()
		// Giant Glacial Horns and Crystalline Brow
		s.RectSym(20, 8, 12, 14, cIce0)
		s.Line(12, 2, 20, 8, cIce3)
		s.Line(51, 2, 43, 8, cIce3)
		s.Line(14, 4, 22, 10, cIce4)
		s.Line(49, 4, 41, 10, cIce4)
		// Piercing Ice Crystal Eyes
		s.Rect(25, 16, 4, 3, cIce3)
		s.Rect(35, 16, 4, 3, cIce3)
		s.Pset(26, 17, cIce4)
		s.Pset(36, 17, cIce4)
		// Monolithic Jagged Ice Shoulders
		s.RectSym(8, 22, 14, 12, cIce1)
		s.Line(8, 20, 14, 22, cIce4)
		s.Line(55, 20, 49, 22, cIce4)
		// Torso with Frozen Trapped Prisoner Soul
		s.RectSym(18, 24, 14, 18, cIce0)
		s.FillCircle(32, 33, 6, cIce2)
		s.FillCircle(32, 33, 3, cBone3) // frozen skull
		// Frozen Broken Iron Chains
		s.PsetSym(12, 36, cSteel3)
		s.PsetSym(12, 42, cSteel2)
		s.PsetSym(12, 48, cSteel1)
		// Glacial Pillar Legs
		s.RectSym(18, 42, 10, 18, cIce1)
		s.DitherRectSym(20, 52, 7, 6, cIce2, cIce3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "frost_colossus.png"), 4)
	}

	// 11. Dragon Regent (Floor 3 Supreme Boss)
	{
		s := NewSprite64()
		// Sweeping Dragon Horns
		s.Line(14, 4, 24, 14, cGold3)
		s.Line(49, 4, 39, 14, cGold3)
		s.Line(12, 6, 22, 16, cGold1)
		s.Line(51, 6, 41, 16, cGold1)
		// Draconic Snout and Glowing Molten Eyes
		s.RectSym(22, 12, 10, 16, cBlood0)
		s.Rect(26, 17, 4, 3, cFire3)
		s.Rect(34, 17, 4, 3, cFire3)
		s.Pset(27, 18, cFire4)
		s.Pset(35, 18, cFire4)
		// Razor Fangs
		s.Line(25, 26, 38, 26, cBone4)
		s.Pset(26, 27, cBone4)
		s.Pset(37, 27, cBone4)
		// Giant Obsidian Dragon Wings
		s.ThickLine(6, 14, 22, 24, 2, cObsidian1)
		s.ThickLine(57, 14, 41, 24, 2, cObsidian1)
		s.DitherRect(4, 16, 16, 20, cBlood0, cObsidian0)
		s.DitherRect(44, 16, 16, 20, cBlood0, cObsidian0)
		// Burning Dragon Core Torso
		s.RectSym(18, 28, 14, 16, cBlood0)
		s.FillCircle(32, 36, 7, cFire2)
		s.FillCircle(32, 36, 3, cFire4) // Dragon Heart
		// Taloned Dragon Legs & Tail
		s.RectSym(20, 44, 8, 14, cBlood0)
		s.Line(20, 58, 18, 60, cBone4)
		s.Line(24, 58, 24, 61, cBone4)
		s.Line(43, 58, 45, 60, cBone4)
		s.Line(39, 58, 39, 61, cBone4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "dragon_regent.png"), 4)
	}
}

// =========================================================================
// 3. ITEMS (38 Distinct Weapons, Armors, Accessories & Potions)
// =========================================================================
func genItems64(dir string) {
	// ---------------- SWORDS ----------------
	{
		// training_sword
		s := NewSprite64()
		s.ThickLine(18, 44, 44, 18, 3, cWood2)
		s.Line(17, 45, 43, 19, cWood1)
		s.Line(19, 43, 45, 17, cWood3)
		s.Line(20, 38, 28, 46, cWood0) // crossguard
		s.Line(15, 47, 12, 50, cWood1) // hilt
		s.FillCircle(11, 51, 2, cWood2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "training_sword.png"), 4)
	}
	{
		// iron_sword
		s := NewSprite64()
		s.ThickLine(16, 46, 46, 16, 3, cSteel4)
		s.Line(15, 47, 45, 17, cSteel2)
		s.Line(17, 45, 47, 15, cSteel3)
		s.Line(20, 38, 28, 46, cSteel1) // guard
		s.Line(16, 48, 12, 52, cWood0)  // grip
		s.FillCircle(11, 53, 3, cGold2) // pommel
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "iron_sword.png"), 4)
	}
	{
		// knights_claymore
		s := NewSprite64()
		s.ThickLine(12, 50, 50, 12, 4, cSteel4)
		s.Line(10, 52, 48, 14, cSteel1)
		s.Line(14, 48, 52, 10, cSteel2)
		// Wide spiked crossguard
		s.ThickLine(18, 38, 32, 52, 3, cGold2)
		s.Pset(17, 37, cGold4)
		s.Pset(33, 53, cGold4)
		s.ThickLine(12, 54, 8, 58, 2, cBlood0) // two-handed grip
		s.FillCircle(7, 59, 3, cGold3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "knights_claymore.png"), 4)
	}
	{
		// flamebrand
		s := NewSprite64()
		s.ThickLine(14, 48, 48, 14, 4, cFire2)
		s.ThickLine(15, 47, 49, 13, 2, cFire4)
		// Floating ash & fire sparks
		s.Pset(42, 10, cFire3)
		s.Pset(48, 8, cFire4)
		s.Pset(52, 16, cFire2)
		s.Pset(34, 20, cFire3)
		// Gargoyle horn crossguard
		s.ThickLine(18, 38, 32, 52, 3, cObsidian1)
		s.Pset(17, 37, cFire3)
		s.Pset(33, 53, cFire3)
		s.Line(14, 52, 10, 56, cObsidian0)
		s.FillCircle(9, 57, 3, cFire2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "flamebrand.png"), 4)
	}
	{
		// excalibur
		s := NewSprite64()
		s.ThickLine(12, 50, 50, 12, 4, cGold4)
		s.ThickLine(13, 49, 49, 13, 2, cIce4) // radiant holy edge
		s.Line(16, 46, 46, 16, cIce3)         // divine runic fuller
		// Angel wing crossguard
		s.ThickLine(16, 36, 30, 50, 3, cGold3)
		s.FillCircle(23, 43, 3, cIce4) // holy sapphire gem
		s.Line(13, 53, 9, 57, cBone4)  // ivory grip
		s.FillCircle(8, 58, 3, cGold4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "excalibur.png"), 4)
	}

	// ---------------- STAVES & SCEPTERS ----------------
	{
		// apprentice_staff
		s := NewSprite64()
		s.ThickLine(14, 50, 46, 18, 2, cWood1)
		s.FillCircle(48, 16, 5, cIce2)
		s.FillCircle(48, 16, 2, cIce4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "apprentice_staff.png"), 4)
	}
	{
		// wooden_staff
		s := NewSprite64()
		s.ThickLine(14, 50, 46, 18, 2, cWood2)
		// Spiral head
		s.Circle(46, 16, 6, cWood3)
		s.FillCircle(46, 16, 3, cIce3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "wooden_staff.png"), 4)
	}
	{
		// crystal_wand
		s := NewSprite64()
		s.ThickLine(18, 46, 42, 22, 2, cGold2)
		s.FillCircle(45, 18, 6, cVoid3)
		s.FillCircle(45, 18, 3, cVoid4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "crystal_wand.png"), 4)
	}
	{
		// archmage_staff
		s := NewSprite64()
		s.ThickLine(12, 52, 44, 20, 2, cGold2)
		// Golden winged crown
		s.Line(38, 14, 50, 8, cGold3)
		s.Line(44, 22, 54, 16, cGold3)
		s.FillCircle(46, 15, 6, cVenom3)
		s.FillCircle(46, 15, 2, cVenom4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "archmage_staff.png"), 4)
	}
	{
		// void_scepter
		s := NewSprite64()
		s.ThickLine(14, 50, 44, 20, 3, cObsidian0)
		s.FillCircle(46, 18, 8, cVoid1)
		s.FillCircle(46, 18, 5, cVoid3)
		s.FillCircle(46, 18, 2, cBlood3) // Eye of the Void
		s.Pset(46, 18, cFire4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "void_scepter.png"), 4)
	}

	// ---------------- DAGGERS & SCYTHE ----------------
	{
		// iron_dagger
		s := NewSprite64()
		s.ThickLine(20, 42, 42, 20, 2, cSteel4)
		s.Line(19, 43, 41, 21, cSteel2)
		s.Line(22, 38, 26, 42, cSteel1)
		s.Line(19, 45, 16, 48, cWood0)
		s.FillCircle(15, 49, 2, cGold2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "iron_dagger.png"), 4)
	}
	{
		// shadowfang
		s := NewSprite64()
		s.ThickLine(18, 44, 44, 18, 3, cObsidian0)
		s.Line(17, 45, 43, 19, cVoid3)
		// Dripping toxic venom
		s.Pset(45, 17, cVenom3)
		s.Pset(46, 16, cVenom4)
		s.Pset(40, 24, cVenom3)
		s.Line(20, 40, 26, 46, cSteel0)
		s.Line(18, 46, 14, 50, cObsidian1)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "shadowfang.png"), 4)
	}
	{
		// assassins_twinblades
		s := NewSprite64()
		s.ThickLine(16, 46, 46, 16, 2, cBlood3)
		s.ThickLine(16, 16, 46, 46, 2, cBlood3)
		s.FillCircle(31, 31, 4, cGold3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "assassins_twinblades.png"), 4)
	}
	{
		// grim_scythe
		s := NewSprite64()
		s.ThickLine(12, 54, 48, 14, 3, cBone1)
		// Sweeping phantom blade
		s.ThickLine(48, 14, 56, 16, 3, cSteel4)
		s.ThickLine(56, 16, 54, 32, 3, cIce3)
		s.ThickLine(54, 32, 42, 38, 2, cIce1)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "grim_scythe.png"), 4)
	}

	// ---------------- BOWS ----------------
	{
		// hunting_bow
		s := NewSprite64()
		s.ThickLine(20, 10, 42, 20, 2, cWood2)
		s.ThickLine(42, 20, 42, 44, 2, cWood2)
		s.ThickLine(42, 44, 20, 54, 2, cWood2)
		s.Line(20, 10, 20, 54, cBone3) // bowstring
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "hunting_bow.png"), 4)
	}
	{
		// elven_recurve
		s := NewSprite64()
		s.ThickLine(18, 10, 44, 18, 2, cVenom2)
		s.ThickLine(44, 18, 44, 46, 2, cVenom2)
		s.ThickLine(44, 46, 18, 54, 2, cVenom2)
		s.Line(18, 10, 18, 54, cVenom4)
		s.FillCircle(44, 32, 3, cGold3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "elven_recurve.png"), 4)
	}
	{
		// windrunner_bow
		s := NewSprite64()
		s.ThickLine(18, 10, 46, 18, 2, cIce3)
		s.ThickLine(46, 18, 46, 46, 2, cIce3)
		s.ThickLine(46, 46, 18, 54, 2, cIce3)
		s.Line(18, 10, 18, 54, cIce4)
		// Winged fins
		s.Line(46, 24, 52, 20, cIce4)
		s.Line(46, 40, 52, 44, cIce4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "windrunner_bow.png"), 4)
	}
	{
		// dragons_breath_bow
		s := NewSprite64()
		s.ThickLine(18, 8, 46, 18, 3, cFire1)
		s.ThickLine(46, 18, 46, 46, 3, cFire1)
		s.ThickLine(46, 46, 18, 56, 3, cFire1)
		s.Line(18, 8, 18, 56, cFire3)
		s.FillCircle(46, 32, 4, cFire4)
		s.Pset(48, 22, cFire3)
		s.Pset(48, 42, cFire3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "dragons_breath_bow.png"), 4)
	}

	// ---------------- SHIELDS ----------------
	{
		// wooden_shield
		s := NewSprite64()
		s.FillCircle(32, 32, 20, cWood1)
		s.Circle(32, 32, 18, cSteel1)
		s.FillCircle(32, 32, 6, cSteel2)
		s.FillCircle(32, 32, 2, cSteel4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "wooden_shield.png"), 4)
	}
	{
		// iron_shield
		s := NewSprite64()
		s.RectSym(18, 12, 14, 24, cSteel1)
		s.Line(18, 36, 31, 52, cSteel1)
		s.Line(45, 36, 32, 52, cSteel1)
		s.DitherRectSym(20, 14, 12, 20, cSteel0, cSteel1)
		// Holy Cross Relief
		s.Rect(30, 16, 4, 30, cGold2)
		s.Rect(22, 24, 20, 4, cGold2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "iron_shield.png"), 4)
	}
	{
		// aegis_shield
		s := NewSprite64()
		s.RectSym(16, 10, 16, 26, cGold2)
		s.Line(16, 36, 31, 54, cGold2)
		s.Line(47, 36, 32, 54, cGold2)
		s.DitherRectSym(18, 12, 14, 22, cGold1, cGold2)
		// Radiating Sun Face
		s.FillCircle(32, 28, 8, cGold4)
		s.FillCircle(32, 28, 4, cIce4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "aegis_shield.png"), 4)
	}

	// ---------------- ARMORS ----------------
	{
		// cloth_armor
		s := NewSprite64()
		s.RectSym(22, 16, 10, 32, cWood1)
		s.DitherRectSym(24, 18, 8, 28, cWood0, cWood1)
		s.Rect(20, 32, 24, 4, cGold2) // belt
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "cloth_armor.png"), 4)
	}
	{
		// iron_armor
		s := NewSprite64()
		s.RectSym(18, 14, 14, 34, cSteel1)
		s.RectSym(22, 18, 10, 24, cSteel2)
		s.DitherRectSym(24, 20, 8, 20, cSteel0, cSteel1)
		s.Rect(18, 34, 28, 4, cGold2) // belt
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "iron_armor.png"), 4)
	}
	{
		// chainmail_hauberk
		s := NewSprite64()
		s.RectSym(20, 14, 12, 36, cSteel2)
		for y := 16; y <= 46; y += 3 {
			for x := 20; x <= 44; x += 3 {
				s.Pset(x, y, cSteel0)
				s.Pset(x+1, y, cSteel4)
			}
		}
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "chainmail_hauberk.png"), 4)
	}
	{
		// dragonscale_armor
		s := NewSprite64()
		s.RectSym(18, 14, 14, 34, cBlood1)
		for y := 16; y <= 44; y += 4 {
			for x := 20; x <= 42; x += 4 {
				s.Pset(x, y, cBlood3)
				s.Pset(x+1, y, cFire3)
				s.Pset(x, y+1, cBlood0)
			}
		}
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "dragonscale_armor.png"), 4)
	}

	// ---------------- HELMETS ----------------
	{
		// leather_cap
		s := NewSprite64()
		s.RectSym(20, 20, 12, 22, cWood1)
		s.Rect(18, 38, 28, 5, cWood0)
		s.FillCircle(32, 40, 3, cGold2)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "leather_cap.png"), 4)
	}
	{
		// iron_helm
		s := NewSprite64()
		s.RectSym(18, 16, 14, 26, cSteel1)
		s.DitherRectSym(20, 18, 12, 22, cSteel0, cSteel2)
		s.Rect(22, 28, 20, 4, cObsidian0) // visor slit
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "iron_helm.png"), 4)
	}
	{
		// shadow_cowl
		s := NewSprite64()
		s.RectSym(18, 16, 14, 28, cObsidian0)
		s.Rect(24, 26, 4, 3, cVoid3) // glowing phantom eyes
		s.Rect(36, 26, 4, 3, cVoid3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "shadow_cowl.png"), 4)
	}
	{
		// crown_of_kings
		s := NewSprite64()
		s.Line(18, 26, 18, 38, cGold3)
		s.Line(45, 26, 45, 38, cGold3)
		s.Line(25, 22, 25, 38, cGold3)
		s.Line(38, 22, 38, 38, cGold3)
		s.Line(31, 18, 32, 38, cGold4) // center high spire
		s.Rect(18, 38, 28, 6, cGold2)
		s.FillCircle(32, 41, 3, cBlood3) // blood ruby
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "crown_of_kings.png"), 4)
	}

	// ---------------- ACCESSORIES ----------------
	{
		// bronze_ring
		s := NewSprite64()
		s.FillCircle(32, 32, 16, cGold1)
		s.FillCircle(32, 32, 10, color.RGBA{0, 0, 0, 0})
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "bronze_ring.png"), 4)
	}
	{
		// apprentice_ring
		s := NewSprite64()
		s.FillCircle(32, 32, 16, cSteel2)
		s.FillCircle(32, 32, 10, color.RGBA{0, 0, 0, 0})
		s.FillCircle(32, 18, 5, cIce3)
		s.FillCircle(32, 18, 2, cIce4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "apprentice_ring.png"), 4)
	}
	{
		// quiver_charm
		s := NewSprite64()
		s.Rect(22, 20, 16, 28, cWood1)
		s.Line(25, 12, 25, 20, cBone3)
		s.Line(29, 10, 29, 20, cBone3)
		s.Line(33, 14, 33, 20, cBone3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "quiver_charm.png"), 4)
	}
	{
		// swift_boots
		s := NewSprite64()
		s.Rect(20, 20, 12, 24, cWood2)
		s.Rect(16, 36, 20, 8, cWood1)
		s.Line(14, 26, 18, 30, cSteel4) // wing
		s.Line(12, 24, 16, 28, cSteel4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "swift_boots.png"), 4)
	}
	{
		// boots_of_haste
		s := NewSprite64()
		s.Rect(20, 20, 12, 24, cObsidian0)
		s.Rect(16, 36, 20, 8, cObsidian1)
		s.Line(12, 24, 18, 30, cIce3) // haste wisps
		s.Line(10, 36, 16, 42, cIce3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "boots_of_haste.png"), 4)
	}
	{
		// blood_chalice
		s := NewSprite64()
		s.Rect(22, 16, 20, 14, cGold2)
		s.Rect(24, 18, 16, 8, cBlood3) // blood pool
		s.ThickLine(32, 30, 32, 44, 4, cGold2)
		s.Rect(22, 44, 20, 6, cGold1)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "blood_chalice.png"), 4)
	}
	{
		// thiefs_coin
		s := NewSprite64()
		s.FillCircle(32, 32, 18, cGold2)
		s.FillCircle(32, 32, 14, cGold3)
		// Skull imprint
		s.FillCircle(32, 30, 6, cObsidian0)
		s.Rect(30, 28, 2, 2, cGold3)
		s.Rect(34, 28, 2, 2, cGold3)
		s.Line(30, 34, 34, 34, cGold3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "thiefs_coin.png"), 4)
	}

	// ---------------- CONSUMABLES ----------------
	{
		// health_potion
		s := NewSprite64()
		s.Rect(28, 14, 8, 6, cWood1) // cork
		s.Rect(26, 20, 12, 6, cSteel2)
		s.FillCircle(32, 38, 14, cBlood2)
		s.FillCircle(30, 36, 8, cBlood3)
		s.FillCircle(28, 34, 3, cBlood4) // specular highlight
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "health_potion.png"), 4)
	}
	{
		// greater_health_potion
		s := NewSprite64()
		s.Rect(28, 10, 8, 6, cGold3)
		s.Rect(26, 16, 12, 6, cGold2)
		s.FillCircle(32, 38, 16, cBlood2)
		s.FillCircle(32, 38, 11, cBlood3)
		s.FillCircle(30, 34, 4, cBlood4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "greater_health_potion.png"), 4)
	}
	{
		// mana_potion
		s := NewSprite64()
		s.Rect(28, 14, 8, 6, cWood1)
		s.Rect(26, 20, 12, 6, cSteel2)
		s.FillCircle(32, 38, 14, cIce1)
		s.FillCircle(30, 36, 8, cIce3)
		s.FillCircle(28, 34, 3, cIce4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "mana_potion.png"), 4)
	}
	{
		// greater_mana_potion
		s := NewSprite64()
		s.Rect(28, 10, 8, 6, cGold3)
		s.Rect(26, 16, 12, 6, cGold2)
		s.FillCircle(32, 38, 16, cVoid2)
		s.FillCircle(32, 38, 11, cVoid3)
		s.FillCircle(30, 34, 4, cVoid4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "greater_mana_potion.png"), 4)
	}
	{
		// fire_bomb
		s := NewSprite64()
		s.FillCircle(32, 38, 16, cObsidian0)
		s.Rect(30, 18, 4, 6, cSteel1) // fuse collar
		s.Line(32, 18, 38, 12, cFire2)
		s.FillCircle(40, 10, 3, cFire4) // spark
		s.Pset(26, 32, cSteel2)         // round metallic specular
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "fire_bomb.png"), 4)
	}
	{
		// smoke_bomb
		s := NewSprite64()
		s.FillCircle(32, 38, 16, cObsidian1)
		s.Rect(30, 18, 4, 6, cWood1)
		s.Line(30, 14, 38, 8, cVoid3) // purple smoke tendril
		s.FillCircle(42, 8, 4, cVoid2)
		s.FillCircle(48, 6, 3, cVoid3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "smoke_bomb.png"), 4)
	}
	{
		// gold_coin (Shiny ancient gold coin medallion with engraved sigil)
		s := NewSprite64()
		s.FillCircle(32, 32, 20, cGold0)
		s.FillCircle(32, 32, 18, cGold2)
		s.FillCircle(30, 30, 16, cGold3)
		// Inner coin ridge
		s.Circle(32, 32, 14, cGold1)
		// Center emblem (Crown / Diamond Crest)
		s.Line(26, 35, 32, 25, cGold4)
		s.Line(32, 25, 38, 35, cGold4)
		s.Line(26, 35, 38, 35, cGold1)
		s.FillCircle(32, 32, 3, cGold4)
		s.FillCircle(32, 25, 2, cGold4)
		// Specular glint at top-left rim
		s.Line(22, 22, 28, 22, cGold4)
		s.Line(22, 23, 24, 25, cGold4)
		s.Pset(24, 21, cGold4)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "gold_coin.png"), 4)
		_ = s.SavePNG(filepath.Join(dir, "gold.png"), 4)
	}
	{
		// gold_pouch (Bulging dark leather coin pouch overflowing with gold)
		s := NewSprite64()
		// Main pouch leather bulb
		s.FillCircle(32, 40, 18, cWood0)
		s.FillCircle(32, 39, 16, cWood1)
		s.FillCircle(30, 37, 13, cWood2)
		// Pouch cinch neck
		s.Rect(26, 24, 12, 5, cWood0)
		s.Rect(27, 24, 10, 4, cWood1)
		// Cinch ruffle collar
		s.FillCircle(32, 20, 9, cWood1)
		s.FillCircle(32, 19, 7, cWood2)
		// Overflowing gold coins peeking out
		s.FillCircle(29, 17, 5, cGold1)
		s.FillCircle(29, 16, 4, cGold3)
		s.FillCircle(35, 16, 5, cGold2)
		s.FillCircle(35, 15, 4, cGold4)
		// Golden tied cord with hanging tassels
		s.Line(24, 26, 40, 26, cGold3)
		s.Line(38, 26, 44, 36, cGold3)
		s.FillCircle(44, 37, 3, cGold4)
		s.Line(26, 26, 22, 34, cGold2)
		s.FillCircle(22, 35, 2, cGold3)
		// Leather highlights and stitching
		s.Line(20, 36, 24, 46, cWood3)
		s.AddOutline(cOut)
		_ = s.SavePNG(filepath.Join(dir, "gold_pouch.png"), 4)
	}
}
