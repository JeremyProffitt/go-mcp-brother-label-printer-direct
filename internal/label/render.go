package label

import (
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	dpi            = 180.0
	mmPerInch      = 25.4
	rowHeightMM    = 5.0
	bufferMM       = 4.0  // gap between key and value
	columnGapMM    = 10.0 // gap between key-value columns
	topMarginMM    = 2.0
	bottomMarginMM = 2.0
	leftMarginMM   = 3.0
	rightMarginMM  = 3.0
)

// KeyValue represents a single key-value pair for the label.
type KeyValue struct {
	Key   string
	Value string
}

// TableLayout holds the computed layout for rendering.
type TableLayout struct {
	TapeWidthMM  float64
	Rows         int
	Columns      int
	Items        []KeyValue
	UsableHeight float64
}

func mmToPx(mm float64) int {
	return int(math.Round(mm * dpi / mmPerInch))
}

// ComputeLayout determines rows, columns based on tape width and item count.
func ComputeLayout(items []KeyValue, tapeWidthMM float64) *TableLayout {
	usable := tapeWidthMM - topMarginMM - bottomMarginMM
	rows := int(usable / rowHeightMM)
	if rows < 1 {
		rows = 1
	}
	cols := (len(items) + rows - 1) / rows

	return &TableLayout{
		TapeWidthMM:  tapeWidthMM,
		Rows:         rows,
		Columns:      cols,
		Items:        items,
		UsableHeight: usable,
	}
}

// RenderTable renders key-value pairs into a monochrome image suitable for the printer.
func RenderTable(items []KeyValue, tapeWidthMM float64) *image.Gray {
	layout := ComputeLayout(items, tapeWidthMM)
	face := basicfont.Face7x13

	rowH := mmToPx(rowHeightMM)
	bufferPx := mmToPx(bufferMM)
	colGapPx := mmToPx(columnGapMM)
	marginLeft := mmToPx(leftMarginMM)
	marginRight := mmToPx(rightMarginMM)
	marginTop := mmToPx(topMarginMM)

	// Distribute items into columns
	type column struct {
		pairs    []KeyValue
		maxKeyW  int
		maxValW  int
	}

	columns := make([]column, layout.Columns)
	for i, kv := range items {
		colIdx := i / layout.Rows
		if colIdx >= len(columns) {
			colIdx = len(columns) - 1
		}
		columns[colIdx].pairs = append(columns[colIdx].pairs, kv)
	}

	// Measure text widths per column
	for ci := range columns {
		for _, kv := range columns[ci].pairs {
			kw := measureText(face, kv.Key+":")
			vw := measureText(face, kv.Value)
			if kw > columns[ci].maxKeyW {
				columns[ci].maxKeyW = kw
			}
			if vw > columns[ci].maxValW {
				columns[ci].maxValW = vw
			}
		}
	}

	// Calculate total image width
	totalW := marginLeft
	for ci, col := range columns {
		totalW += col.maxKeyW + bufferPx + col.maxValW
		if ci < len(columns)-1 {
			totalW += colGapPx
		}
	}
	totalW += marginRight

	// Image height = full tape width in pixels
	totalH := mmToPx(tapeWidthMM)

	// Create white image
	img := image.NewGray(image.Rect(0, 0, totalW, totalH))
	for y := 0; y < totalH; y++ {
		for x := 0; x < totalW; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}

	// Draw separator lines between columns
	for ci := range columns {
		if ci == 0 {
			continue
		}
		x := marginLeft
		for j := 0; j < ci; j++ {
			x += columns[j].maxKeyW + bufferPx + columns[j].maxValW + colGapPx
		}
		lineX := x - colGapPx/2
		for y := marginTop; y < totalH-mmToPx(bottomMarginMM); y++ {
			img.SetGray(lineX, y, color.Gray{Y: 180})
		}
	}

	// Draw text
	for ci, col := range columns {
		// X offset for this column
		colX := marginLeft
		for j := 0; j < ci; j++ {
			colX += columns[j].maxKeyW + bufferPx + columns[j].maxValW + colGapPx
		}

		for ri, kv := range col.pairs {
			// Y position: center text vertically in the row
			baseY := marginTop + ri*rowH
			textY := baseY + (rowH+13)/2 // 13 = font ascent

			// Key: right-aligned within maxKeyW
			keyStr := kv.Key + ":"
			keyW := measureText(face, keyStr)
			keyX := colX + col.maxKeyW - keyW
			drawText(img, face, keyX, textY, keyStr)

			// Value: left-aligned after buffer
			valX := colX + col.maxKeyW + bufferPx
			drawText(img, face, valX, textY, kv.Value)
		}
	}

	return img
}

func measureText(face font.Face, s string) int {
	w := font.MeasureString(face, s)
	return w.Ceil()
}

func drawText(img *image.Gray, face font.Face, x, y int, s string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// ParseTapeWidthMM extracts tape width in mm from IPP media-ready string.
// e.g., "roll_current_24x0mm" → 24.0
func ParseTapeWidthMM(mediaReady string) float64 {
	// Format: roll_current_24x0mm or roll_24x0mm_24x0mm
	s := strings.ToLower(mediaReady)

	// Find a pattern like _NNx where NN is the width
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '_' && s[i+1] >= '0' && s[i+1] <= '9' {
			j := i + 1
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == '.') {
				j++
			}
			if j < len(s) && s[j] == 'x' {
				numStr := s[i+1 : j]
				var width float64
				for _, c := range numStr {
					if c == '.' {
						// handle decimal
						break
					}
					width = width*10 + float64(c-'0')
				}
				if width > 0 {
					return width
				}
			}
		}
	}

	return 24.0 // default to 24mm
}

// RenderTableTransposed renders the table with axes swapped:
// image width = tape height (print head width), image height = label length (feed direction).
// This is the orientation label printers typically expect.
func RenderTableTransposed(items []KeyValue, tapeWidthMM float64) *image.Gray {
	normal := RenderTable(items, tapeWidthMM)
	bounds := normal.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Transpose: swap x and y
	transposed := image.NewGray(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			transposed.SetGray(y, x, normal.GrayAt(x, y))
		}
	}
	return transposed
}
