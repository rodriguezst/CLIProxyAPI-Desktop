// Package branding provides the application's open-door icons.
package branding

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

//go:embed assets/launcher.png
var launcherPNG []byte

//go:embed assets/launcher-stopped.png
var stoppedPNG []byte

//go:embed assets/menu-template.png
var templatePNG []byte

var (
	launcherIcon = decodeIcon(launcherPNG)
	stoppedIcon  = decodeIcon(stoppedPNG)
	templateIcon = decodeIcon(templatePNG)
)

func decodeIcon(data []byte) image.Image {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic("invalid embedded application icon: " + err.Error())
	}
	return img
}

func PNG(size int, template bool, running bool) []byte {
	source := launcherIcon
	if template {
		source = templateIcon
	} else if !running {
		source = stoppedIcon
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(img, img.Bounds(), source, source.Bounds(), draw.Src, nil)
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}

func ICO(running bool) []byte {
	// Windows selects a native size for Explorer, shortcuts, and the system tray.
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	images := make([][]byte, len(sizes))
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, []uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		images[i] = PNG(size, false, running)
		// ICO represents a 256-pixel dimension with a zero byte.
		out.Write([]byte{byte(size), byte(size), 0, 0})
		_ = binary.Write(&out, binary.LittleEndian, []uint16{1, 32})
		_ = binary.Write(&out, binary.LittleEndian, []uint32{uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, data := range images {
		out.Write(data)
	}
	return out.Bytes()
}
