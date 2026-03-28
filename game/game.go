package game

import (
	"image"

	"deathchase/engine"
	"deathchase/input"
	"deathchase/screen"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	Scale  = 3
	Width  = screen.ScreenWidth
	Height = screen.ScreenHeight
)

// Game is the Ebitengine wrapper around the headless engine.
type Game struct {
	env      *engine.GameEnv
	renderer *screen.Renderer
	offImg   *ebiten.Image
}

func New() *Game {
	g := &Game{
		env:      engine.NewGameEnv(),
		renderer: screen.NewRenderer(),
		offImg:   ebiten.NewImage(Width, Height),
	}
	return g
}

func (g *Game) Update() error {
	act := input.ReadAction()

	if act.Escape {
		return ebiten.Termination
	}

	g.env.Step(act)
	return nil
}

func (g *Game) Draw(scr *ebiten.Image) {
	g.renderer.Render(g.env.Buf, g.offImg)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(Scale, Scale)
	scr.DrawImage(g.offImg, op)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return Width * Scale, Height * Scale
}

func Run() error {
	ebiten.SetWindowSize(Width*Scale, Height*Scale)
	ebiten.SetWindowTitle("Deathchase")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenClearedEveryFrame(true)

	// Set icon (just use a small green square for now)
	icon := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			icon.Pix[(y*16+x)*4+1] = 200 // green
			icon.Pix[(y*16+x)*4+3] = 255 // alpha
		}
	}
	ebiten.SetWindowIcon([]image.Image{icon})

	g := New()
	return ebiten.RunGame(g)
}
