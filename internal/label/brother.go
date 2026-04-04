package label

import (
	"bytes"
	"encoding/binary"
	"image"
)

// Brother PT-CBP raster protocol constants
const (
	ptDPI          = 180
	ptMaxDots      = 128 // max printable dots across tape width (18mm at 180 DPI)
	ptBytesPerLine = 16  // 128 dots / 8 bits
)

// EncodeBrotherRaster converts a grayscale image to Brother PT raster format
// for the PT-P750W. The image should be oriented with:
//   - width  = label length (feed direction)
//   - height = tape width (print head direction, max 170px for 24mm)
//
// The protocol sends data column-by-column as the tape feeds.
func EncodeBrotherRaster(img *image.Gray, tapeWidthMM float64, autoCut bool) []byte {
	bounds := img.Bounds()
	labelLength := bounds.Dx() // columns to send
	tapeHeight := bounds.Dy()  // pixels across tape

	var buf bytes.Buffer

	// 1. Initialize: 100 null bytes
	buf.Write(make([]byte, 100))

	// 2. ESC @ (reset printer)
	buf.WriteByte(0x1B)
	buf.WriteByte('@')

	// 3. Switch to raster mode: ESC i a 0x01
	buf.Write([]byte{0x1B, 'i', 'a', 0x01})

	// 4. Set media info: ESC i z flags type width_mm length_mm raster_lines(4B LE) page reserved
	//    flags: bit1=type valid, bit2=width valid, bit3=raster count valid, bit7=recovery
	buf.Write([]byte{0x1B, 'i', 'z'})
	buf.WriteByte(0x8E)              // flags: recovery + raster count valid + width valid + type valid
	buf.WriteByte(0x0A)              // media type: laminated tape
	buf.WriteByte(byte(tapeWidthMM)) // tape width in mm
	buf.WriteByte(0x00)              // media length in mm (0 = continuous tape)
	binary.Write(&buf, binary.LittleEndian, uint32(labelLength)) // number of raster lines
	buf.WriteByte(0x00)              // page number (starting page)
	buf.WriteByte(0x00)              // reserved

	// 5. Set mode: ESC i M 0x00
	buf.Write([]byte{0x1B, 'i', 'M', 0x00})

	// 6. Set auto-cut: ESC i K
	if autoCut {
		buf.Write([]byte{0x1B, 'i', 'K', 0x08})
	} else {
		buf.Write([]byte{0x1B, 'i', 'K', 0x00})
	}

	// 7. Set margin: ESC i d 0x00 0x00 (minimal margin)
	buf.Write([]byte{0x1B, 'i', 'd', 0x00, 0x00})

	// 8. Set compression: M 0x00 (no compression)
	buf.Write([]byte{'M', 0x00})

	// Calculate centering offset: center the printable area within tape height
	printableStart := (tapeHeight - ptMaxDots) / 2
	if printableStart < 0 {
		printableStart = 0
	}

	// 9. Send raster data: one line per column of the image
	for x := 0; x < labelLength; x++ {
		// Check if this column is entirely white (blank)
		allWhite := true
		for dot := 0; dot < ptMaxDots; dot++ {
			py := printableStart + dot
			if py >= 0 && py < tapeHeight {
				if img.GrayAt(bounds.Min.X+x, bounds.Min.Y+py).Y < 128 {
					allWhite = false
					break
				}
			}
		}

		if allWhite {
			// Send blank line: 'Z'
			buf.WriteByte('Z')
		} else {
			// Send raster line: 'g' + uint16LE(length) + data
			buf.WriteByte('g')
			binary.Write(&buf, binary.LittleEndian, uint16(ptBytesPerLine))

			// Pack 128 dots into 16 bytes (MSB = top of tape)
			lineData := make([]byte, ptBytesPerLine)
			for dot := 0; dot < ptMaxDots; dot++ {
				py := printableStart + dot
				if py >= 0 && py < tapeHeight {
					pixel := img.GrayAt(bounds.Min.X+x, bounds.Min.Y+py).Y
					if pixel < 128 {
						// Black pixel: set bit (MSB first within each byte)
						byteIdx := dot / 8
						bitIdx := 7 - (dot % 8)
						lineData[byteIdx] |= 1 << uint(bitIdx)
					}
				}
			}
			buf.Write(lineData)
		}
	}

	// 10. Print and feed: 0x1A
	buf.WriteByte(0x1A)

	return buf.Bytes()
}
