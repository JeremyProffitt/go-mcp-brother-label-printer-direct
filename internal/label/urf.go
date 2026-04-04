package label

import (
	"bytes"
	"encoding/binary"
	"image"
)

// URF format constants
const (
	urfColorSpaceGray = 0 // UNIRAST device gray (W)
	urfDPI            = 180
)

// EncodeURF encodes a grayscale image into Apple Raster (URF) format.
// The PT-P750W accepts image/urf with monochrome 8-bit at 180 DPI.
func EncodeURF(img *image.Gray) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var buf bytes.Buffer

	// File header: "UNIRAST\0" + uint32 page count
	buf.WriteString("UNIRAST\x00")
	binary.Write(&buf, binary.BigEndian, uint32(1)) // 1 page

	// Page header (32 bytes):
	// [0] bpp, [1] colorspace, [2] duplex, [3] quality,
	// [4-15] reserved, [16-19] width, [20-23] height,
	// [24-27] xDPI, [28-31] yDPI
	binary.Write(&buf, binary.BigEndian, uint8(8))              // bits per pixel
	binary.Write(&buf, binary.BigEndian, uint8(urfColorSpaceGray)) // color space: sGray
	binary.Write(&buf, binary.BigEndian, uint8(0))              // duplex mode: off
	binary.Write(&buf, binary.BigEndian, uint8(4))              // print quality (PQ4)
	binary.Write(&buf, binary.BigEndian, uint32(0))             // reserved
	binary.Write(&buf, binary.BigEndian, uint32(0))             // reserved
	binary.Write(&buf, binary.BigEndian, uint32(0))             // reserved
	binary.Write(&buf, binary.BigEndian, uint32(width))         // width in pixels
	binary.Write(&buf, binary.BigEndian, uint32(height))        // height in pixels
	binary.Write(&buf, binary.BigEndian, uint32(urfDPI))        // x resolution
	binary.Write(&buf, binary.BigEndian, uint32(urfDPI))        // y resolution

	// Raster data: URF uses PackBits-style run-length encoding per line.
	// Each line is encoded as sequences of: repeat_count (uint8), pixel_data (N bytes per pixel).
	// repeat_count = 0 means 1 line, repeat_count = N means N+1 identical lines.
	bytesPerPixel := 1 // grayscale 8-bit

	for y := bounds.Min.Y; y < bounds.Max.Y; {
		// Count how many identical lines follow
		repeatCount := 0
		for y+repeatCount+1 < bounds.Max.Y && repeatCount < 255 {
			if linesEqual(img, y, y+repeatCount+1) {
				repeatCount++
			} else {
				break
			}
		}

		// Write repeat count
		buf.WriteByte(uint8(repeatCount))

		// Write one line of pixel data using PackBits compression
		line := make([]byte, width*bytesPerPixel)
		for x := 0; x < width; x++ {
			line[x] = img.GrayAt(bounds.Min.X+x, y).Y
		}
		writePackBits(&buf, line)

		y += repeatCount + 1
	}

	return buf.Bytes()
}

// linesEqual checks if two raster lines are identical.
func linesEqual(img *image.Gray, y1, y2 int) bool {
	bounds := img.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		if img.GrayAt(x, y1).Y != img.GrayAt(x, y2).Y {
			return false
		}
	}
	return true
}

// writePackBits encodes a line using PackBits run-length encoding.
// PackBits: N=0..127 means N+1 literal bytes follow; N=129..255 means repeat next byte (257-N) times.
// N=128 is no-op.
func writePackBits(buf *bytes.Buffer, data []byte) {
	i := 0
	n := len(data)

	for i < n {
		// Check for a run of identical bytes
		runLen := 1
		for i+runLen < n && data[i+runLen] == data[i] && runLen < 128 {
			runLen++
		}

		if runLen >= 3 {
			// Encode as repeat: (257-runLen), byte
			buf.WriteByte(byte(257 - runLen))
			buf.WriteByte(data[i])
			i += runLen
		} else {
			// Encode as literal sequence
			litStart := i
			litLen := 0
			for i+litLen < n && litLen < 128 {
				// Check if a run of 3+ starts here
				if i+litLen+2 < n && data[i+litLen] == data[i+litLen+1] && data[i+litLen] == data[i+litLen+2] {
					break
				}
				litLen++
			}
			if litLen == 0 {
				litLen = 1
			}
			buf.WriteByte(byte(litLen - 1))
			buf.Write(data[litStart : litStart+litLen])
			i += litLen
		}
	}
}
