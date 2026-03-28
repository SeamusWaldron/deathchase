package screen

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// ZX Spectrum colour palette (normal + bright)
var Palette = [16]color.RGBA{
	// Normal
	{0, 0, 0, 255},       // 0: black
	{0, 0, 205, 255},     // 1: blue
	{205, 0, 0, 255},     // 2: red
	{205, 0, 205, 255},   // 3: magenta
	{0, 205, 0, 255},     // 4: green
	{0, 205, 205, 255},   // 5: cyan
	{205, 205, 0, 255},   // 6: yellow
	{205, 205, 205, 255}, // 7: white
	// Bright
	{0, 0, 0, 255},       // 8: black (bright)
	{0, 0, 255, 255},     // 9: blue (bright)
	{255, 0, 0, 255},     // 10: red (bright)
	{255, 0, 255, 255},   // 11: magenta (bright)
	{0, 255, 0, 255},     // 12: green (bright)
	{0, 255, 255, 255},   // 13: cyan (bright)
	{255, 255, 0, 255},   // 14: yellow (bright)
	{255, 255, 255, 255}, // 15: white (bright)
}

// Renderer converts a ZX Spectrum buffer to an Ebitengine image.
type Renderer struct {
	pixels     []byte // RGBA pixel data (256*192*4)
	flashState bool   // Toggles every 16 frames
	flashCount int
}

func NewRenderer() *Renderer {
	return &Renderer{
		pixels: make([]byte, ScreenWidth*ScreenHeight*4),
	}
}

// Render converts the buffer to pixels and writes to the given image.
func (r *Renderer) Render(buf *Buffer, img *ebiten.Image) {
	r.flashCount++
	if r.flashCount >= 16 {
		r.flashCount = 0
		r.flashState = !r.flashState
	}

	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			attr := buf.Attrs[row*Cols+col]
			ink := attr & 0x07
			paper := (attr >> 3) & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			if flash == 1 && r.flashState {
				ink, paper = paper, ink
			}

			inkIdx := ink + bright*8
			paperIdx := paper + bright*8
			inkCol := Palette[inkIdx]
			paperCol := Palette[paperIdx]

			for scanLine := 0; scanLine < 8; scanLine++ {
				py := row*8 + scanLine
				addr := DisplayAddr(col*8, py)
				pixelByte := buf.Peek(addr)

				for bit := 0; bit < 8; bit++ {
					px := col*8 + bit
					offset := (py*ScreenWidth + px) * 4
					if pixelByte&(0x80>>bit) != 0 {
						r.pixels[offset] = inkCol.R
						r.pixels[offset+1] = inkCol.G
						r.pixels[offset+2] = inkCol.B
						r.pixels[offset+3] = 255
					} else {
						r.pixels[offset] = paperCol.R
						r.pixels[offset+1] = paperCol.G
						r.pixels[offset+2] = paperCol.B
						r.pixels[offset+3] = 255
					}
				}
			}
		}
	}

	img.WritePixels(r.pixels)
}

// RenderToImage writes the buffer to a standard image.RGBA (for screenshots).
func (r *Renderer) RenderToImage(buf *Buffer, img *image.RGBA) {
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			attr := buf.Attrs[row*Cols+col]
			ink := attr & 0x07
			paper := (attr >> 3) & 0x07
			bright := (attr >> 6) & 0x01

			inkCol := Palette[ink+bright*8]
			paperCol := Palette[paper+bright*8]

			for scanLine := 0; scanLine < 8; scanLine++ {
				py := row*8 + scanLine
				addr := DisplayAddr(col*8, py)
				pixelByte := buf.Peek(addr)

				for bit := 0; bit < 8; bit++ {
					px := col*8 + bit
					if pixelByte&(0x80>>bit) != 0 {
						img.SetRGBA(px, py, inkCol)
					} else {
						img.SetRGBA(px, py, paperCol)
					}
				}
			}
		}
	}
}
