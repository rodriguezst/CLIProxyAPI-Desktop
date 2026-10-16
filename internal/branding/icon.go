// Package branding draws the application's simple proxy-arrow mark.
package branding

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

func PNG(size int, template bool, running bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			u, v := float64(x)/float64(size), float64(y)/float64(size)
			dx, dy := u-.5, v-.5
			if !template && dx*dx+dy*dy < .235 {
				c := color.NRGBA{R: 67, G: 80, B: 104, A: 255}
				if running {
					c = color.NRGBA{R: 18, G: 128, B: 115, A: 255}
				}
				img.SetNRGBA(x, y, c)
			}
			arrow := (u > .22 && u < .7 && v > .29 && v < .39) ||
				(u > .58 && u < .79 && v > .19 && v < .49 && u < .79-abs(v-.34)) ||
				(u > .3 && u < .78 && v > .61 && v < .71) ||
				(u > .21 && u < .42 && v > .51 && v < .81 && u > .21+abs(v-.66))
			if arrow {
				c := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
				if template {
					c = color.NRGBA{A: 255}
				}
				img.SetNRGBA(x, y, c)
			}
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}

func ICO(running bool) []byte {
	data := PNG(64, false, running)
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, []uint16{0, 1, 1})
	out.Write([]byte{64, 64, 0, 0})
	_ = binary.Write(&out, binary.LittleEndian, []uint16{1, 32})
	_ = binary.Write(&out, binary.LittleEndian, []uint32{uint32(len(data)), 22})
	out.Write(data)
	return out.Bytes()
}
func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
