package label

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// RenderBigLabel renders text lines at the largest possible font size
// that fits within the tape width. Each line gets an equal share of the
// usable tape height.
func RenderBigLabel(lines []string, tapeWidthMM float64) *image.Gray {
	usableH := tapeWidthMM - topMarginMM - bottomMarginMM
	numRows := len(lines)
	if numRows == 0 {
		numRows = 1
	}
	rowHeightMM := usableH / float64(numRows)

	// The basicfont.Face7x13 renders at 13px height.
	// At 180 DPI, 13px = 13/180*25.4 = ~1.83mm per character height.
	// We want to scale up to fill rowHeightMM.
	baseFace := basicfont.Face7x13
	baseCharH := 13.0 // pixels
	targetRowH := mmToPx(rowHeightMM)
	scale := int(math.Floor(float64(targetRowH) / baseCharH))
	if scale < 1 {
		scale = 1
	}

	charW := 7 * scale
	charH := 13 * scale
	marginL := mmToPx(leftMarginMM)
	marginR := mmToPx(rightMarginMM)
	marginT := mmToPx(topMarginMM)

	// Calculate image width based on longest line
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}
	imgW := marginL + maxLen*charW + marginR
	imgH := mmToPx(tapeWidthMM)

	// Create white image
	img := image.NewGray(image.Rect(0, 0, imgW, imgH))
	for y := 0; y < imgH; y++ {
		for x := 0; x < imgW; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}

	// Draw each line scaled up
	for i, line := range lines {
		rowY := marginT + i*targetRowH
		textY := rowY + (targetRowH-charH)/2 // vertically center

		for j, ch := range line {
			drawScaledChar(img, baseFace, marginL+j*charW, textY, scale, ch)
		}
	}

	return img
}

// drawScaledChar renders a single character scaled up by drawing it at 1x
// into a temp image, then scaling up with nearest-neighbor.
func drawScaledChar(dst *image.Gray, face font.Face, x, y, scale int, ch rune) {
	// Render char at 1x scale into a small temp image
	srcW, srcH := 7, 13
	tmp := image.NewGray(image.Rect(0, 0, srcW, srcH))
	for ty := 0; ty < srcH; ty++ {
		for tx := 0; tx < srcW; tx++ {
			tmp.SetGray(tx, ty, color.Gray{Y: 255})
		}
	}

	d := &font.Drawer{
		Dst:  tmp,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(0, 11), // baseline at y=11 for Face7x13
	}
	d.DrawString(string(ch))

	// Scale up with nearest-neighbor
	for sy := 0; sy < srcH; sy++ {
		for sx := 0; sx < srcW; sx++ {
			pixel := tmp.GrayAt(sx, sy)
			if pixel.Y < 200 { // has ink
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						px := x + sx*scale + dx
						py := y + sy*scale + dy
						if px >= 0 && py >= 0 && px < dst.Bounds().Dx() && py < dst.Bounds().Dy() {
							dst.SetGray(px, py, pixel)
						}
					}
				}
			}
		}
	}
}

// MaxRowsForTape returns the maximum number of 5mm rows that fit on a given tape width.
func MaxRowsForTape(tapeWidthMM float64) int {
	usable := tapeWidthMM - topMarginMM - bottomMarginMM
	rows := int(usable / rowHeightMM)
	if rows < 1 {
		return 1
	}
	return rows
}
