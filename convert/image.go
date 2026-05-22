package convert

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"

	"golang.org/x/image/bmp"
)

// ConvertBytesToBMP converts CNV image data to BMP format.
// CNV header layout (17 bytes):
//
//	[0]      bpp         (uint8)
//	[1:5]    width       (uint32 LE)
//	[5:9]    height      (uint32 LE)
//	[9:13]   width2      (uint32 LE) — redundant width, must match
//	[13:16]  unknown     (3 bytes, reserved/ignored)
//	[16]     zero        (uint8, must be 0)
func ConvertBytesToBMP(data []byte) ([]byte, error) {
	const headerSize = 17

	if len(data) < headerSize {
		return nil, errors.New("data too short to contain CNV header")
	}

	bpp := data[0]
	width := binary.LittleEndian.Uint32(data[1:5])
	height := binary.LittleEndian.Uint32(data[5:9])
	width2 := binary.LittleEndian.Uint32(data[9:13])
	// bytes [13:16] are unknown/reserved — not validated
	zero := data[16]

	if width != width2 {
		log.Printf("cnv: width mismatch in header: %d vs %d", width, width2)
	}

	if zero != 0 {
		return nil, errors.New("cnv: expected zero byte at header offset 16, got non-zero")
	}

	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("cnv: unsupported BPP %d (expected 24 or 32)", bpp)
	}

	bytesPerPixel := int(bpp) / 8 // 3 for 24bpp, 4 for 32bpp
	expectedLen := headerSize + int(width)*int(height)*bytesPerPixel
	if len(data) != expectedLen {
		return nil, fmt.Errorf("cnv: data length mismatch: expected %d, got %d", expectedLen, len(data))
	}

	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))

	for row := 0; row < int(height); row++ {
		for col := 0; col < int(width); col++ {
			src := headerSize + bytesPerPixel*(row*int(width)+col)

			// CNV stores pixels as BGRA (or BGR for 24bpp)
			b := data[src+0]
			g := data[src+1]
			r := data[src+2]
			a := byte(0xFF)
			if bytesPerPixel == 4 {
				a = data[src+3]
			}

			img.SetNRGBA(col, row, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}

	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("cnv: BMP encoding failed: %w", err)
	}

	return buf.Bytes(), nil
}
