package game

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"time"

	"deathchase/audio"
	"deathchase/engine"
	"deathchase/input"
	"deathchase/screen"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	snd      *audio.Audio
	wasEngine bool // track engine state for start/stop transitions
}

func New() *Game {
	g := &Game{
		env:      engine.NewGameEnv(),
		renderer: screen.NewRenderer(),
		offImg:   ebiten.NewImage(Width, Height),
		snd:      audio.New(),
	}
	return g
}

func (g *Game) Update() error {
	act := input.ReadAction()

	if act.Escape {
		return ebiten.Termination
	}

	// Screenshot on * key (Shift+8 or numpad multiply)
	if inpututil.IsKeyJustPressed(ebiten.KeyKPMultiply) ||
		(ebiten.IsKeyPressed(ebiten.KeyShift) && inpututil.IsKeyJustPressed(ebiten.Key8)) {
		g.takeScreenshot()
	}

	result := g.env.Step(act)

	// Drive audio from game events
	if result.EngineOn && !g.wasEngine {
		g.snd.StartEngine()
	} else if !result.EngineOn && g.wasEngine {
		g.snd.StopEngine()
	}
	g.wasEngine = result.EngineOn

	if result.Fired {
		g.snd.Play(audio.SfxBolt)
	}
	if result.EnemyHit {
		g.snd.Play(audio.SfxExplosion)
	}
	if result.TreeCrash {
		g.snd.StopEngine()
		g.wasEngine = false
		g.snd.Play(audio.SfxCrash)
	}
	if result.GameOver {
		g.snd.Play(audio.SfxDescending)
	}
	if result.SectorChange {
		g.snd.Play(audio.SfxAscending)
	}

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

// takeScreenshot saves the current screen as a PNG file.
// Filename encodes game state: state_sector_score_lives_timestamp.png
func (g *Game) takeScreenshot() {
	stateName := "unknown"
	switch g.env.State {
	case engine.StateTitle:
		stateName = "title"
	case engine.StatePlaying:
		stateName = "playing"
	case engine.StateDead:
		stateName = "dead"
	case engine.StateGameOver:
		stateName = "gameover"
	case engine.StateSectorChange:
		stateName = "sector-change"
	}

	ts := time.Now().Format("150405") // HHMMSS
	filename := fmt.Sprintf("screenshot_%s_s%d_score%d_lives%d_%s.png",
		stateName, g.env.Sector, g.env.ScoreValue(), g.env.Lives, ts)

	// Render to an image.RGBA at native resolution
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	g.renderer.RenderToImage(g.env.Buf, img)

	f, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Screenshot failed: %v\n", err)
		return
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		fmt.Printf("Screenshot encode failed: %v\n", err)
		return
	}
	fmt.Printf("Screenshot saved: %s\n", filename)
}

func Run() error {
	ebiten.SetWindowSize(Width*Scale, Height*Scale)
	ebiten.SetWindowTitle("Deathchase")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenClearedEveryFrame(true)
	ebiten.SetTPS(50) // ZX Spectrum PAL = 50Hz

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
