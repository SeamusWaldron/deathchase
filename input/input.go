package input

import (
	"deathchase/action"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// ReadAction reads the current keyboard state and returns an Action.
// Controls (matching original Spectrum keys, avoiding F-keys for macOS):
//   1 = left, 0 = right (original keys)
//   9 = accelerate, 8 = brake
//   Space/B-SPACE row = fire
//   Arrow keys as alternative
//   Enter = start/select
//   Escape = quit
func ReadAction() action.Action {
	a := action.Action{}

	// Steering: 1 or Left arrow
	if ebiten.IsKeyPressed(ebiten.Key1) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		a.Left = true
	}
	// Steering: 0 or Right arrow
	if ebiten.IsKeyPressed(ebiten.Key0) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		a.Right = true
	}
	// Speed up: 9 or Up arrow
	if ebiten.IsKeyPressed(ebiten.Key9) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		a.Accelerate = true
	}
	// Slow down: 8 or Down arrow
	if ebiten.IsKeyPressed(ebiten.Key8) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		a.Brake = true
	}
	// Fire: Space or B-N-M-Symbol Shift row (just use Space)
	if ebiten.IsKeyPressed(ebiten.KeySpace) {
		a.Fire = true
	}
	// Enter
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		a.Enter = true
	}
	// Escape
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.Escape = true
	}

	return a
}
