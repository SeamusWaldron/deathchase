package engine

import (
	"deathchase/action"
	"deathchase/data"
	"deathchase/screen"
	"math/rand"
)

// GameEnv is the headless game environment.
type GameEnv struct {
	Buf   *screen.Buffer
	State int

	// Player
	Handlebar    int  // -1, 0, 1
	Speed        int  // 0=fast, 1=slow, 2=idle
	BikeMoving   bool
	Lives        int

	// Enemy bikes
	EnemyDir       [2]int   // direction each enemy is facing (-1, 0, 1)
	EnemyDirFrames [2]int   // frames until direction change
	EnemyMoveUnits [2]int   // units to move next frame
	EnemyPos       [2]int   // horizontal position (0-31)
	EnemyActive    [2]bool  // is this enemy still alive
	EnemyDistance  int      // distance state (0-3)
	EnemyDistCount int      // counter 0-0x1E for distance progression
	NumEnemies     int      // enemies remaining

	// Photon bolt
	Firing       bool
	BoltX        int  // horizontal position
	BoltFrames   int  // frames of bolt remaining
	BoltOffset   int  // index into screen offset table
	BoltGraphic  int  // index into photon bolt graphics
	BoltHit      bool // hit something this frame

	// Bonus enemy
	BonusX       int
	BonusFlags   byte // bit 0=plane on screen, bit 1=tank on screen, bit 5=should appear, bit 7=hit
	BonusTimer   int

	// Game state
	Sector       int  // 1-8
	IsNight      bool
	DayNightFlag byte // alternates $55/$AA
	PlayingArea  int  // $01A0 or $02E0
	Score        [6]byte // ASCII digits
	HighScore    [6]byte
	DebugMode    bool

	// Explosion state
	ExplodeFrames int
	ExplodePos    int

	// Tree field buffer — represents the playing field with trees
	// Rows go from back (horizon) to front (near player)
	// Each row is FieldCols (32) bytes wide
	Field [FieldDepth * FieldCols]byte

	// Tree source buffer ($5B00, 256 bytes)
	TreeBuf [TreeBufferSize]byte

	// Flip-flop flag (alternates each frame to control tree generation)
	FlipFlop byte

	// Random state
	rng *rand.Rand

	// Frame counter (for various timing)
	FrameCount int

	// Control method (not used in engine, but tracked)
	UseKempston bool

	// Range indicator flash state
	RangeFlash bool

	// Per-frame audio event flags (reset each Step)
	evFired        bool
	evEnemyHit     bool
	evTreeCrash    bool
	evSectorChange bool
	evGameOver     bool
}

// NewGameEnv creates a new game environment.
func NewGameEnv() *GameEnv {
	g := &GameEnv{
		Buf:          screen.NewBuffer(),
		State:        StateTitle,
		rng:          rand.New(rand.NewSource(42)),
		FlipFlop:     0x55,
		DayNightFlag: 0x55,
		Sector:       1,
		Lives:        3,
		Score:        [6]byte{'0', '0', '0', '0', '0', '0'},
		PlayingArea:  PlayingAreaSmall,
	}

	// Populate title screen field — same as the original at $65B3-$6608:
	// populate tree buffer, generate trees, render field.
	g.populateTrees()
	g.clearField()
	g.BikeMoving = true
	for i := 0; i < FieldDepth*2; i++ {
		g.moveTrees()
	}
	g.BikeMoving = false

	return g
}

// Reset starts a new game.
func (g *GameEnv) Reset() {
	g.State = StatePlaying
	g.Handlebar = HandlebarCentre
	g.Speed = SpeedIdle
	g.BikeMoving = false
	g.Lives = 3

	g.EnemyDir = [2]int{0, 0}
	g.EnemyDirFrames = [2]int{3, 3}
	g.EnemyMoveUnits = [2]int{0, 0}
	g.EnemyPos = [2]int{0, 0x20}
	g.EnemyActive = [2]bool{true, true}
	g.EnemyDistance = DistanceFar
	g.EnemyDistCount = 3
	g.NumEnemies = 2

	g.Firing = false
	g.BoltX = 0x10
	g.BoltFrames = 7
	g.BoltOffset = 15  // near bottom of screen, matching $686B
	g.BoltGraphic = 0
	g.BoltHit = false

	g.BonusX = 0
	g.BonusFlags = 0
	g.BonusTimer = 3

	g.Sector = 0
	g.IsNight = false
	g.DayNightFlag = 0x55
	g.PlayingArea = PlayingAreaSmall
	g.Score = [6]byte{'0', '0', '0', '0', '0', '0'}
	g.FlipFlop = 0x55

	g.ExplodeFrames = 0
	g.ExplodePos = 0

	// Clear tree buffer
	for i := range g.TreeBuf {
		g.TreeBuf[i] = 0
	}

	// Populate initial trees based on sector density
	g.populateTrees()

	// Clear the screen and set up initial display
	g.clearScreen()

	// Set up initial playing field
	g.clearField()

	// Run initial tree generation to fill the field.
	// Original runs 10 iterations at $6601-$6608, but with 24-row deep field
	// we need more to fill it. Run enough to scroll through the full depth.
	g.BikeMoving = true
	for i := 0; i < FieldDepth*2; i++ {
		g.moveTrees()
	}
	g.BikeMoving = false

	// Reset positions after initial generation
	g.EnemyPos = [2]int{0, 0x20}
	g.EnemyDistance = DistanceFar
	g.Speed = SpeedIdle

	// Switch to first sector
	g.switchSector()

	// Render initial display
	g.renderField()
	g.drawBike()
	g.drawHUD()
}

// Step processes one frame with the given action and returns the observation.
func (g *GameEnv) Step(act action.Action) StepResult {
	g.FrameCount++

	// Reset per-frame event flags
	g.evFired = false
	g.evEnemyHit = false
	g.evTreeCrash = false
	g.evSectorChange = false
	g.evGameOver = false

	switch g.State {
	case StateTitle:
		return g.stepTitle(act)
	case StatePlaying:
		return g.stepPlaying(act)
	case StateDead:
		return g.stepDead(act)
	case StateGameOver:
		return g.stepGameOver(act)
	case StateSectorChange:
		return g.stepSectorChange(act)
	}

	return StepResult{}
}

func (g *GameEnv) stepTitle(act action.Action) StepResult {
	// Draw title screen — matches original at $628D/$6298/$62A3:
	// playing field with trees, bike, then text overlay
	g.clearScreen()
	g.renderField()
	g.drawBike()
	g.drawHUD()
	// Text from $628D, $6298, $62A3 — no "DEATHCHASE" title in the original
	g.Buf.PrintString(0x488A, " 1=KEYBOARD ")
	g.Buf.PrintString(0x48CA, " 2=KEMPSTON ")
	g.Buf.PrintString(0x5006, "\x7F MERVYN ESTCOURT 1983")

	// Original uses 1=keyboard, 2=kempston to start.
	// We accept Enter, Left (key 1), or Fire (space) to start with keyboard.
	if act.Enter || act.Left || act.Fire {
		g.UseKempston = false
		g.Reset()
	}

	return StepResult{State: g.State}
}

func (g *GameEnv) stepPlaying(act action.Action) StepResult {
	result := StepResult{State: g.State}

	// 1. Check input (speed)
	g.handleSpeedInput(act)

	// 2. Handle fire input (only if bolt not already hit something)
	if !g.BoltHit {
		g.handleFireInput(act)
	}

	// 3. Respond to steering
	g.handleSteering(act)

	// 4. Move trees
	g.moveTrees()

	// 5. Adjust photon bolt horizontal position if firing
	if g.Firing {
		g.adjustBoltPosition()
	}

	// 6. Clear and redraw the playing field (trees + enemies)
	g.clearScreen()
	g.renderField()

	// 7. Process hit bikes (explosion animation)
	if g.BoltHit {
		g.processHitBike()
	}

	// 8. Toggle range indicator
	g.RangeFlash = g.EnemyDistance == DistanceInRange

	// 9. Recalculate enemy positions
	g.recalcEnemyPositions()

	// 10. Check tree collision
	if g.checkTreeCollision() {
		g.handleDeath()
		result.State = g.State
		result.TreeCrash = g.evTreeCrash
		result.GameOver = g.evGameOver
		return result
	}

	// 11. Move bikes nearer/further
	g.moveBikesDistance()

	// 12. Move bonus enemy
	g.moveBonusEnemy()

	// 13. Draw photon bolt AFTER trees/enemies so we can detect hits by
	//     checking existing screen content (pixels + attributes).
	if !g.BoltHit && g.Firing {
		g.checkBoltHit()
	}

	// 14. Draw player bike and HUD
	g.drawBike()
	g.drawHUD()

	result.EngineOn = g.BikeMoving
	result.Fired = g.evFired
	result.EnemyHit = g.evEnemyHit
	result.TreeCrash = g.evTreeCrash
	result.SectorChange = g.evSectorChange
	result.Score = g.ScoreValue()
	result.Lives = g.Lives
	result.Sector = g.Sector
	return result
}

func (g *GameEnv) stepDead(act action.Action) StepResult {
	// Brief pause then rebuild the field and resume.
	// Matches original at $6711: clear screen, wipe field, redraw, continue.
	g.FrameCount++
	if g.FrameCount > 50 { // ~1 second pause at 50 TPS
		g.State = StatePlaying
		g.Speed = SpeedIdle
		g.BikeMoving = false
		g.Firing = false
		g.Handlebar = HandlebarCentre
		g.resetBolt()

		// Wipe and rebuild field ($6772 + $5E00 loop)
		g.clearField()
		g.BikeMoving = true
		for i := 0; i < FieldDepth*2; i++ {
			g.moveTrees()
		}
		g.BikeMoving = false

		g.clearScreen()
		g.renderField()
		g.drawBike()
		g.drawHUD()
	}
	return StepResult{State: g.State, Lives: g.Lives, Score: g.ScoreValue(), Sector: g.Sector}
}

func (g *GameEnv) stepGameOver(act action.Action) StepResult {
	g.clearScreen()
	g.renderField()
	g.drawBike()
	g.drawHUD()
	g.Buf.PrintString(0x48AB, "GAME OVER")

	if act.Enter || g.FrameCount > 150 {
		g.State = StateTitle
		g.updateHighScore()
	}
	return StepResult{State: g.State, Score: g.ScoreValue(), Lives: g.Lives, Sector: g.Sector}
}

func (g *GameEnv) stepSectorChange(act action.Action) StepResult {
	g.FrameCount++
	if g.FrameCount > 60 {
		g.State = StatePlaying
	}
	g.clearScreen()
	g.renderField()
	g.drawBike()
	g.drawHUD()

	// Show sector message
	sectorStr := "SECTOR " + string(rune('0'+g.Sector))
	g.Buf.PrintString(0x48CB, sectorStr)
	if g.IsNight {
		g.Buf.PrintString(0x488A, "NIGHT PATROL")
	} else {
		g.Buf.PrintString(0x488A, "DAY   PATROL")
	}

	return StepResult{State: g.State, Score: g.ScoreValue(), Lives: g.Lives, Sector: g.Sector}
}

// --- Input handling ---

func (g *GameEnv) handleSpeedInput(act action.Action) {
	if act.Accelerate {
		if g.Speed > SpeedFast {
			g.Speed--
		}
		g.BikeMoving = true
	}
	if act.Brake {
		if g.Speed < SpeedIdle {
			g.Speed++
		}
		if g.Speed == SpeedIdle {
			g.BikeMoving = false
		}
	}
}

func (g *GameEnv) handleSteering(act action.Action) {
	if act.Right {
		if g.Handlebar != HandlebarRight {
			g.Handlebar = HandlebarRight
		}
	} else if act.Left {
		if g.Handlebar != HandlebarLeft {
			g.Handlebar = HandlebarLeft
		}
	} else {
		if g.Handlebar != HandlebarCentre {
			g.Handlebar = HandlebarCentre
		}
	}
}

// handleFireInput initiates a photon bolt. Matches $68F0:
// fire only at full speed, nothing exploding, bolt not already in flight.
func (g *GameEnv) handleFireInput(act action.Action) {
	if !act.Fire {
		return
	}
	if g.Firing {
		return // bolt already in flight
	}
	if g.Speed != SpeedFast {
		return // can only fire at full speed
	}
	if g.ExplodeFrames > 0 {
		return // something already exploding
	}
	// Initiate bolt — matches $68F0 setup
	g.Firing = true
	g.BoltX = 0x10       // centre of screen
	g.BoltFrames = 7     // 7 frames of travel (6 visible + 1 initial)
	g.BoltOffset = 15    // start near bottom (ScreenOffsets[15])
	g.BoltGraphic = 0    // largest bolt graphic
	g.BoltHit = false
	g.evFired = true
}

// --- Tree system ---

// populateTrees fills the tree source buffer based on sector density.
// Replicates $64B7.
func (g *GameEnv) populateTrees() {
	// Clear tree source buffer
	for i := range g.TreeBuf {
		g.TreeBuf[i] = 0
	}

	// Get density for current sector
	sectorIdx := g.Sector
	if sectorIdx < 1 {
		sectorIdx = 1
	}
	if sectorIdx > 8 {
		sectorIdx = 8
	}
	density := int(data.TreeDensity[sectorIdx-1])

	// Place trees randomly
	for i := 0; i < density; i++ {
		pos := g.randomByte()
		g.TreeBuf[pos] = TreeSmall
	}

	// Remove adjacent trees (no two trees next to each other)
	for i := 0; i < 256; i++ {
		if g.TreeBuf[i] == TreeSmall {
			for j := 1; j <= 3 && i+j < 256; j++ {
				if g.TreeBuf[i+j] == TreeSmall {
					g.TreeBuf[i+j] = 0
				}
			}
		}
	}
}

// clearField zeros the playing field buffer.
func (g *GameEnv) clearField() {
	for i := range g.Field {
		g.Field[i] = 0
	}
}

// moveTrees scrolls the tree field forward and generates new trees at the horizon.
// Replicates the routine at $5E00.
//
// On the real Z80 at 3.5MHz, the tree rendering loop ($5F89) takes ~100-150K
// T-states per frame. At full speed, the game runs at roughly 25-35 FPS effective,
// NOT 50. We keep TPS at 50 for responsive input but throttle scrolling:
//   Speed 0 (fast):   scroll every 2nd frame  (~25 scrolls/sec)
//   Speed 1 (medium): scroll every 4th frame  (~12 scrolls/sec)
func (g *GameEnv) moveTrees() {
	if !g.BikeMoving {
		return
	}

	switch g.Speed {
	case SpeedFast:
		if g.FrameCount%2 != 0 {
			return
		}
	case SpeedSlow:
		if g.FrameCount%4 != 0 {
			return
		}
	default:
		return // SpeedIdle — not moving
	}

	// Alternate flip-flop each frame
	g.FlipFlop = (g.FlipFlop >> 1) | (g.FlipFlop << 7) // RRCA
	if g.FlipFlop&0x01 == 0 {
		// "evens" phase — don't generate new trees, just scroll
	} else {
		// "odds" phase — generate new tree row at the back
		r := g.randomByte()
		srcPos := int(r&0x7F) + 0x20
		if srcPos >= TreeBufferSize {
			srcPos = srcPos & 0xFF
		}

		// Copy a line from the tree source buffer to the back row of the field
		backRow := FieldDepth - 1
		for col := 0; col < 30; col++ {
			idx := (srcPos - col) & 0xFF
			g.setField(backRow, col, g.TreeBuf[idx])
		}
	}

	// Shift all rows forward (towards the player) by one row
	totalRows := FieldDepth
	for row := 0; row < totalRows-1; row++ {
		for col := 0; col < FieldCols; col++ {
			g.setField(row, col, g.getField(row+1, col))
		}
	}

	// Apply horizontal shift based on handlebar direction.
	// In the original at $5EFE/$5F28/$5F47, steering shifts the tree
	// field left or right to create the turning visual.
	if g.Handlebar != 0 {
		for row := 0; row < FieldDepth; row++ {
			if g.Handlebar > 0 {
				// Turning right: shift trees left
				for col := 0; col < FieldCols-1; col++ {
					g.setField(row, col, g.getField(row, col+1))
				}
				g.setField(row, FieldCols-1, 0)
			} else {
				// Turning left: shift trees right
				for col := FieldCols - 1; col > 0; col-- {
					g.setField(row, col, g.getField(row, col-1))
				}
				g.setField(row, 0, 0)
			}
		}
	}

	// Apply perspective widening for close trees
	g.applyPerspective()
}

// applyPerspective widens trees in the closest rows to simulate perspective.
// In the original, this handles the left shunt ($5E2C), right shunt ($5E53),
// and closest-row expansion ($5EE1).
func (g *GameEnv) applyPerspective() {
	totalRows := FieldDepth
	if totalRows < 4 {
		return
	}

	// Shunt left side outward for the closest 13 rows
	rowsToShunt := 13
	if rowsToShunt > totalRows {
		rowsToShunt = totalRows
	}
	colsToShift := 14
	for i := 0; i < rowsToShunt; i++ {
		row := i
		// Shift columns left by 1
		for c := 0; c < colsToShift-1; c++ {
			g.setField(row, c, g.getField(row, c+1))
		}
		g.setField(row, colsToShift-1, 0)
		if i%2 == 0 && colsToShift > 1 {
			colsToShift--
		}
	}

	// Shunt right side outward
	colsToShift = 13
	for i := 0; i < rowsToShunt; i++ {
		row := i
		// Shift columns right by 1
		for c := FieldCols - 1; c > FieldCols-1-colsToShift; c-- {
			g.setField(row, c, g.getField(row, c-1))
		}
		g.setField(row, FieldCols-1-colsToShift, 0)
		if i%2 == 0 && colsToShift > 1 {
			colsToShift--
		}
	}

	// Clear row 0 of the field ($5E79-$5EDF): the original does several
	// extra operations to clear the centre of the front row:
	// 1. Zero out the back row completely
	// 2. Shift row 0 left half further left (4 times)
	// 3. Remove adjacent trees from front rows
	// For simplicity, clear the centre columns of row 0 where collision checks happen
	for col := 10; col <= 22; col++ {
		if g.getField(0, col) != 0 {
			// Only keep trees that were naturally shifted in, not expanded
		}
	}

	// Expand closest trees to take up more area ($5EE1).
	// Original uses CPIR which advances past the found byte, so expansions
	// don't cascade. Skip forward by 4 after each expansion to prevent this.
	for col := 0; col < FieldCols-3; col++ {
		if g.getField(0, col) == TreeSmall {
			g.setField(0, col+1, TreeSmall)
			g.setField(0, col+2, TreeLarge)
			g.setField(0, col+3, TreeLarge)
			col += 3 // skip past expansion to prevent cascading
		}
	}

	// Safety: ensure centre columns (where collision detection checks) are clear
	// unless a real tree was shifted there. The original's complex perspective
	// routines at $5ECE-$5EDF clear centre-left and centre-right columns.
	// We approximate: only allow trees at the edges of the front row.
	for col := 12; col <= 19; col++ {
		// Check if this tree was a natural shift-in (came from row 1)
		// If row 1 at this col is also a tree, it's a legitimate tree approaching
		if g.getField(0, col) != 0 && g.getField(1, col) == 0 {
			g.setField(0, col, 0) // Remove spurious expansion into centre
		}
	}
}

func (g *GameEnv) getField(row, col int) byte {
	if row < 0 || col < 0 || col >= FieldCols {
		return 0
	}
	idx := row*FieldCols + col
	if idx >= len(g.Field) {
		return 0
	}
	return g.Field[idx]
}

func (g *GameEnv) setField(row, col int, val byte) {
	if row < 0 || col < 0 || col >= FieldCols {
		return
	}
	idx := row*FieldCols + col
	if idx >= len(g.Field) {
		return
	}
	g.Field[idx] = val
}

// --- Rendering ---

// clearScreen clears the buffer to match the original at $6530.
func (g *GameEnv) clearScreen() {
	// Clear display file to 0
	for i := range g.Buf.Display {
		g.Buf.Display[i] = 0
	}
	// Set all attributes to $3B (bright, white paper, cyan ink)
	for i := range g.Buf.Attrs {
		g.Buf.Attrs[i] = DefaultAttr
	}

	// Set bottom row attributes ($5AC0) — playing area border
	// $5AC0 = attr row 21
	g.Buf.Poke(screen.AttrAddr(0, 21), 0x7B) // left border
	for col := 1; col <= 30; col++ {
		g.Buf.Poke(screen.AttrAddr(col, 21), 0x78) // bright, white paper, black ink
	}
	g.Buf.Poke(screen.AttrAddr(31, 21), 0x7B) // right border

	// Draw left border at $50C0 (col 0, y=160, 8 scan lines)
	for line := 0; line < 8; line++ {
		addr := screen.DisplayAddr(0, 160+line)
		g.Buf.DrawByteOverwrite(addr, 0xE0)
	}
	// Draw right border at $50DF (col 31, y=160, 8 scan lines)
	for line := 0; line < 8; line++ {
		addr := screen.DisplayAddr(31*8, 160+line)
		g.Buf.DrawByteOverwrite(addr, 0x0F)
	}
	// Fill bottom rows with $FF ($50E0, 4 scan lines x 32 cols)
	for line := 0; line < 4; line++ {
		for col := 0; col < 32; col++ {
			addr := screen.DisplayAddr(col*8, 168+line)
			g.Buf.DrawByteOverwrite(addr, 0xFF)
		}
	}
}

// renderField draws the playing field trees onto the screen buffer.
//
// Replicates the original routine at $5F89. For each column (1-30), scan
// the field buffer from front (row 0) to back. Each empty row advances
// the sprite table index. When a tree is found at depth D, sprite table
// entry D is used. If NO tree is found, the last entry (beyond-horizon)
// is used — its blank pixels still paint the sky/ground attributes.
//
// The tree sprite data contains attribute bytes that CREATE the sky (cyan
// paper) and ground (green paper). When trees are drawn at all 30 columns,
// their attributes paint the entire playing area background.
func (g *GameEnv) renderField() {
	spriteTable := data.TreeSpriteTable
	numEntries := len(spriteTable)

	for col := 1; col <= 30; col++ {
		// Scan front to back, advancing sprite table index for empty rows
		spriteIdx := 0
		foundTree := false
		for row := 0; row < FieldDepth; row++ {
			cell := g.getField(row, col)
			if cell >= TreeSmall {
				foundTree = true
				break
			}
			// Empty row — advance to next sprite table entry
			spriteIdx++
			if spriteIdx >= numEntries {
				spriteIdx = numEntries - 1
			}
		}

		// If no tree found, use the last entry (beyond-horizon blank)
		if !foundTree {
			spriteIdx = numEntries - 1
		}

		// Clamp index
		if spriteIdx >= numEntries {
			spriteIdx = numEntries - 1
		}

		graphic := &spriteTable[spriteIdx].Graphic

		// Number of screen rows to draw: 18 normally, 16 for centre columns
		// (replicates the check at $5FED-$5FF5)
		treeHeight := 18
		if col >= 13 && col < 20 {
			treeHeight = 16
		}
		numScreenRows := len(data.ScreenOffsets)
		if treeHeight > numScreenRows {
			treeHeight = numScreenRows
		}

		// Draw each row: write attribute byte then 8 pixel bytes
		for screenRow := 0; screenRow < treeHeight; screenRow++ {
			treeRow := &graphic.Rows[screenRow]

			offEntry := data.ScreenOffsets[screenRow]
			screenCol := offEntry[0] + byte(col)

			// Write attribute byte
			attrAddr := uint16(offEntry[2])<<8 | uint16(screenCol)
			attr := treeRow.Attr
			if g.IsNight {
				// Night mode: convert day attrs to night equivalents
				attr = g.nightAttr(attr)
			}
			g.Buf.Poke(attrAddr, attr)

			// Write 8 pixel bytes (scan lines within this character row)
			dispAddr := uint16(offEntry[1])<<8 | uint16(screenCol)
			addr := dispAddr
			for line := 0; line < 8; line++ {
				g.Buf.DrawByteOR(addr, treeRow.Pixels[line])
				addr = screen.NextScanLine(addr)
			}
		}
	}

	g.drawEnemiesOnField()
	g.drawBonusEnemy()
}

// drawBonusEnemy draws a plane or tank if one is active.
// Drawn at $4860+BonusX (same row as enemy bikes), using plane/tank sprites.
func (g *GameEnv) drawBonusEnemy() {
	if g.BonusX < 1 || g.BonusX > 30 {
		return
	}
	if g.BonusFlags&0x01 == 0 && g.BonusFlags&0x02 == 0 {
		return
	}

	screenCol := byte(0x60) + byte(g.BonusX)
	dispAddr := uint16(0x48)<<8 | uint16(screenCol)
	attrAddr := uint16(0x59)<<8 | uint16(screenCol)

	// Choose sprite: plane or tank, normal or exploded
	var leftSprite, rightSprite []byte
	if g.BonusFlags&0x80 != 0 {
		// Being blown up
		leftSprite = data.PlaneExplodedLeft
		rightSprite = data.PlaneExplodedRight
	} else if g.Sector%2 == 0 {
		// Even sectors: tank
		leftSprite = data.TankLeft
		rightSprite = data.TankRight
	} else {
		// Odd sectors: plane
		leftSprite = data.PlaneLeft
		rightSprite = data.PlaneRight
	}

	// Draw left half
	addr := dispAddr
	for i := 0; i < 8 && i < len(leftSprite); i++ {
		g.Buf.DrawByteOR(addr, leftSprite[i])
		addr = screen.NextScanLine(addr)
	}

	// Draw right half at next column
	rightAddr := dispAddr + 1
	addr = rightAddr
	for i := 0; i < 8 && i < len(rightSprite); i++ {
		g.Buf.DrawByteOR(addr, rightSprite[i])
		addr = screen.NextScanLine(addr)
	}

	// Set attribute: white ink
	g.Buf.Poke(attrAddr, g.Buf.Peek(attrAddr)|0x07)
	g.Buf.Poke(attrAddr+1, g.Buf.Peek(attrAddr+1)|0x07)
}

// nightAttr converts a day-mode attribute to its night equivalent.
// Day sky ($2B/$2C/$2A/$28) → black paper with green ink ($04).
// Day ground ($23/$22/$20) → black paper, black ink ($00).
func (g *GameEnv) nightAttr(dayAttr byte) byte {
	paper := (dayAttr >> 3) & 0x07
	if paper >= 4 { // cyan (5) or green (4) paper = sky/canopy area
		return 0x04 // black paper, green ink
	}
	return 0x00 // black paper, black ink (ground)
}

// drawEnemiesOnField draws enemy bikes at the correct screen position.
// From the ASM at $6150: ALL distances use the same base address $4860+C
// (character row 11, pixel Y=88). Distance only changes sprite size.
// Attribute address is $5960+C (attribute row 11).
func (g *GameEnv) drawEnemiesOnField() {
	for e := 0; e < 2; e++ {
		if !g.EnemyActive[e] {
			continue
		}
		pos := g.EnemyPos[e]
		if pos < 1 || pos > 30 {
			continue
		}

		// Fixed screen position: $4860+pos (display), $5960+pos (attribute)
		// This is character row 11 (pixel Y=88), matching the original at $6150
		screenCol := byte(0x60) + byte(pos)
		dispAddr := uint16(0x48)<<8 | uint16(screenCol)
		attrAddr := uint16(0x59)<<8 | uint16(screenCol)

		g.drawEnemyBike(e, dispAddr, attrAddr)
	}
}

// drawEnemyBike draws an enemy bike sprite at the correct distance/direction.
func (g *GameEnv) drawEnemyBike(enemyIdx int, dispAddr, attrAddr uint16) {
	if g.EnemyDistance > DistanceInRange {
		return
	}

	// Determine direction: -1, 0, 1 → index 0, 1, 2
	dir := g.EnemyMoveUnits[enemyIdx]
	dirIdx := 1 // centre
	if dir < 0 {
		dirIdx = 0
	} else if dir > 0 {
		dirIdx = 2
	}

	spriteData := data.EnemySprites[g.EnemyDistance][dirIdx]

	// Draw sprite using OR mode
	addr := dispAddr
	for i := 0; i < 8 && i < len(spriteData); i++ {
		g.Buf.DrawByteOR(addr, spriteData[i])
		addr = screen.NextScanLine(addr)
	}

	// For near/in-range bikes (distance 2,3), draw second character row
	// Original at $61C2: second row at dispAddr with L += $20
	if g.EnemyDistance >= DistanceNear && len(spriteData) > 8 {
		lo := byte(dispAddr) + 0x20
		nextRowAddr := (dispAddr & 0xFF00) | uint16(lo)
		for i := 8; i < 16 && i < len(spriteData); i++ {
			g.Buf.DrawByteOR(nextRowAddr, spriteData[i])
			nextRowAddr = screen.NextScanLine(nextRowAddr)
		}
		// Set second row attribute too
		attrLo := byte(attrAddr) + 0x20
		attrAddr2 := (attrAddr & 0xFF00) | uint16(attrLo)
		if enemyIdx == 0 {
			g.Buf.Poke(attrAddr2, g.Buf.Peek(attrAddr2)&0xF8|0x06)
		} else {
			g.Buf.Poke(attrAddr2, g.Buf.Peek(attrAddr2)&0xF8|0x01)
		}
	}

	// Set attribute — yellow (6) for bike 1, blue (1) for bike 2
	// Original at $6193-$619D: OR the colour into existing attribute.
	// We must clear INK bits first (AND $F8) then set the colour,
	// otherwise tree ink bits combine and produce wrong colour for hit detection.
	if enemyIdx == 0 {
		g.Buf.Poke(attrAddr, (g.Buf.Peek(attrAddr)&0xF8)|0x06) // yellow ink
	} else {
		g.Buf.Poke(attrAddr, (g.Buf.Peek(attrAddr)&0xF8)|0x01) // blue ink
	}
}

// drawBike draws the player's bike and handlebars at the bottom of the screen.
// Replicates $6918 — copies 16 scanlines of 32 bytes from $6DBF to $5080.
func (g *GameEnv) drawBike() {
	// The original copies 16 scanlines to display address $5080.
	// $5080 = third 2 (0x50), scan 0, char row 4, col 0 → pixel y=160
	// After 8 scanlines, it switches to $50A0 (char row 5) → pixel y=168

	// Row 1: scanlines 0-7 at y=160..167
	for scanLine := 0; scanLine < 8; scanLine++ {
		for col := 0; col < 32; col++ {
			addr := screen.DisplayAddr(col*8, 160+scanLine)
			g.Buf.DrawByteOverwrite(addr, data.BikeGraphic[scanLine][col])
		}
	}
	// Row 1 attributes
	for col := 0; col < 32; col++ {
		g.Buf.Poke(screen.AttrAddr(col, 20), data.BikeRow1Attrs[col])
	}

	// Row 2: scanlines 8-15 at y=168..175
	for scanLine := 0; scanLine < 8; scanLine++ {
		for col := 0; col < 32; col++ {
			addr := screen.DisplayAddr(col*8, 168+scanLine)
			g.Buf.DrawByteOverwrite(addr, data.BikeGraphic[8+scanLine][col])
		}
	}
	// Row 2 attributes
	for col := 0; col < 32; col++ {
		g.Buf.Poke(screen.AttrAddr(col, 21), data.BikeRow2Attrs[col])
	}

	// Draw handlebars
	g.drawHandlebars()
}

func (g *GameEnv) drawHandlebars() {
	// Handlebars drawn at $504D (col 13, row 18) and next row at $506D
	// $504D = third 2, scan 0, char row 2, col 13 → y=144
	// 7 columns wide, 2 character rows (8 scanlines each)

	var row1 *[8][7]byte
	var row2 *[8][7]byte
	var attr1 *[7]byte
	var attr2 *[7]byte

	switch g.Handlebar {
	case HandlebarLeft:
		row1 = &data.HandlebarLeftRow1
		row2 = &data.HandlebarLeftRow2
		attr1 = &data.HandlebarLeftRow1Attrs
		attr2 = &data.HandlebarLeftRow2Attrs
	case HandlebarRight:
		row1 = &data.HandlebarRightRow1
		row2 = &data.HandlebarRightRow2
		attr1 = &data.HandlebarRightRow1Attrs
		attr2 = &data.HandlebarRightRow2Attrs
	default:
		row1 = &data.HandlebarForwardRow1
		row2 = &data.HandlebarForwardRow2
		attr1 = &data.HandlebarForwardRow1Attrs
		attr2 = &data.HandlebarForwardRow2Attrs
	}

	// Draw row 1 at y=144 (char row 18), cols 13-19
	for scanLine := 0; scanLine < 8; scanLine++ {
		for col := 0; col < 7; col++ {
			addr := screen.DisplayAddr((13+col)*8, 144+scanLine)
			g.Buf.DrawByteOverwrite(addr, row1[scanLine][col])
		}
	}
	for col := 0; col < 7; col++ {
		g.Buf.Poke(screen.AttrAddr(13+col, 18), attr1[col])
	}

	// Draw row 2 at y=152 (char row 19), cols 13-19
	for scanLine := 0; scanLine < 8; scanLine++ {
		for col := 0; col < 7; col++ {
			addr := screen.DisplayAddr((13+col)*8, 152+scanLine)
			g.Buf.DrawByteOverwrite(addr, row2[scanLine][col])
		}
	}
	for col := 0; col < 7; col++ {
		g.Buf.Poke(screen.AttrAddr(13+col, 19), attr2[col])
	}
}

// drawHUD draws score, lives, and range indicator.
func (g *GameEnv) drawHUD() {
	// Score at $50A1 = third 2, scan 0, row 5, col 1
	scoreStr := ":$:" + string(g.Score[:])
	g.Buf.PrintString(0x50A1, scoreStr)

	// Lives at $50C1
	livesStr := ":LIVES :" + string(rune('0'+g.Lives)) + ":"
	g.Buf.PrintString(0x50C1, livesStr)

	// Range indicator at $50D6
	g.Buf.PrintString(0x50D6, "[]RANGE")

	// Flash the range indicator if in range
	if g.RangeFlash && g.FrameCount%8 < 4 {
		g.Buf.Poke(screen.AttrAddr(22, 21), 0xF8) // flash on
		g.Buf.Poke(screen.AttrAddr(23, 21), 0xF8)
	}

	// High score at $50B6
	hiStr := "HI:" + string(g.HighScore[:])
	g.Buf.PrintString(0x50B6, hiStr)
}

// --- Enemy AI ---

func (g *GameEnv) recalcEnemyPositions() {
	for e := 0; e < 2; e++ {
		if !g.EnemyActive[e] {
			continue
		}

		// Clamp position to 0-0x20
		pos := g.EnemyPos[e]
		if pos < 0 {
			pos = 0
		}
		if pos > 0x20 {
			pos = 0x20
		}

		// Apply movement
		pos += g.EnemyMoveUnits[e]

		// Adjust for player steering
		if g.BikeMoving {
			pos -= g.Handlebar
		}

		g.EnemyPos[e] = pos

		// Decrement direction change counter
		g.EnemyDirFrames[e]--
		if g.EnemyDirFrames[e] <= 0 {
			// Pick new direction
			r := g.randomByte()
			frames := int((r&0x1C)>>2) + 1
			g.EnemyDirFrames[e] = frames
			newDir := int(r&0x03) - 1
			if newDir == 2 {
				newDir = 0 // don't change
			}
			g.EnemyMoveUnits[e] = newDir
		}
	}

	// Prevent both bikes from overlapping
	if g.EnemyActive[0] && g.EnemyActive[1] && g.EnemyPos[0] == g.EnemyPos[1] {
		g.EnemyPos[1] += g.EnemyMoveUnits[1]
	}
}

// --- Collision ---

func (g *GameEnv) checkTreeCollision() bool {
	if !g.BikeMoving {
		return false
	}

	// Check the closest row of trees at the player's handlebar position
	// The player is centred around column 0x0E (14), ±1 for handlebar
	centreCol := 14 + g.Handlebar
	for i := 0; i < 5; i++ {
		col := centreCol + i
		if g.getField(0, col) != 0 {
			return true
		}
	}
	return false
}

// handleDeath replicates $6711: place crash trees in front of bike, redraw,
// lose a life, clear field, and restart or game over.
func (g *GameEnv) handleDeath() {
	// Place trees right in front of the player for crash visual ($6711-$6726)
	crashCol := 14 + g.Handlebar // $8D + handlebar, relative to row
	for i := 0; i < 3; i++ {
		g.setField(0, crashCol+i, TreeSmall)
	}
	for i := 3; i < 6; i++ {
		g.setField(0, crashCol+i, TreeLarge)
	}

	// Redraw the scene with the crash tree visible
	g.clearScreen()
	g.renderField()
	g.drawBike()
	g.drawHUD()

	// Stop the bike and firing
	g.Speed = SpeedIdle
	g.BikeMoving = false
	g.Firing = false
	g.resetBolt()

	// Lose a life
	g.Lives--
	g.FrameCount = 0
	g.evTreeCrash = true

	if g.Lives <= 0 {
		g.State = StateGameOver
		g.evGameOver = true
	} else {
		g.State = StateDead
	}
}

// --- Distance system ---

func (g *GameEnv) moveBikesDistance() {
	if g.Speed == SpeedFast {
		// Check if at least one enemy is on-screen (pos < 0x20)
		onScreen := false
		for e := 0; e < 2; e++ {
			if g.EnemyActive[e] && g.EnemyPos[e] >= 0 && g.EnemyPos[e] < 0x20 {
				onScreen = true
				break
			}
		}
		if !onScreen {
			return
		}

		g.EnemyDistCount--
		if g.EnemyDistCount <= 0 {
			g.EnemyDistCount = 0x1E
			if g.EnemyDistance < DistanceInRange {
				g.EnemyDistance++
			}
		}
	} else if g.Speed != SpeedIdle {
		// Moving slowly — bikes get further
		if g.EnemyDistance > DistanceFar {
			g.EnemyDistance--
		}
	}
}

// --- Photon bolt ---

// adjustBoltPosition shifts the bolt horizontally with the player's steering.
// Matches $6836: the original does this twice per frame (left/right scan),
// but once per frame is sufficient for our rendering.
func (g *GameEnv) adjustBoltPosition() {
	g.BoltX += g.Handlebar
}

// checkBoltHit draws the bolt at its current screen position, then checks
// whether it has hit anything by inspecting existing screen content.
// Matches the Z80 bolt loop at $6847:
//   1. Compute screen address from ScreenOffsets[BoltOffset] + BoltX
//   2. Check existing pixels/attrs for a hit
//   3. Draw the bolt graphic
//   4. Colour the trail at the PREVIOUS position
//   5. Advance: decrement BoltFrames, decrement BoltOffset, increment BoltGraphic
func (g *GameEnv) checkBoltHit() {
	if !g.Firing {
		return
	}

	// Clamp BoltOffset and BoltGraphic to valid range
	if g.BoltOffset < 0 || g.BoltOffset >= len(data.ScreenOffsets) {
		g.resetBolt()
		return
	}
	if g.BoltGraphic < 0 || g.BoltGraphic >= len(data.PhotonBolt) {
		g.resetBolt()
		return
	}

	// Compute bolt screen position from the offset table
	offEntry := data.ScreenOffsets[g.BoltOffset]
	screenCol := offEntry[0] + byte(g.BoltX)
	dispAddr := uint16(offEntry[1])<<8 | uint16(screenCol)
	attrAddr := uint16(offEntry[2])<<8 | uint16(screenCol)

	// --- Hit detection by reading existing screen content ---
	// The original at $6813 checks if there are pixels at the bolt's position.
	// If pixels exist, it checks the attribute colour to determine what was hit.
	//
	// We check multiple scan lines (not just the first) since the enemy sprite
	// may not start at scan line 0 of the character cell.
	existingPixels := byte(0)
	checkAddr := dispAddr
	for line := 0; line < 8; line++ {
		existingPixels |= g.Buf.Peek(checkAddr)
		checkAddr = screen.NextScanLine(checkAddr)
	}

	if existingPixels != 0 {
		existingAttr := g.Buf.Peek(attrAddr)
		inkColour := existingAttr & 0x07

		// Check for enemy bike hit (only when in range)
		if g.EnemyDistance == DistanceInRange {
			switch inkColour {
			case 0x06: // yellow = bike 1
				if g.EnemyActive[0] {
					g.hitEnemy(0)
					return
				}
			case 0x01: // blue = bike 2
				if g.EnemyActive[1] {
					g.hitEnemy(1)
					return
				}
			}
		}

		// Check for bonus enemy hit (white ink)
		if inkColour == 0x07 {
			if g.BonusFlags&0x01 != 0 || g.BonusFlags&0x02 != 0 {
				g.hitBonus()
				return
			}
		}

		// Tree hit: only count it if we're in the sky/canopy area (upper half
		// of playing field). In the ground area, tree pixel data is from trunk
		// graphic but shouldn't block the bolt. The original's tree data has
		// zero pixels in ground rows ($20 attr with $00 pixel bytes).
		if g.BoltOffset < 10 {
			// In sky area — hitting a tree canopy, reset bolt
			g.resetBolt()
			return
		}
		// In ground area — ignore tree pixels, continue bolt travel
	}

	// --- No hit: draw the bolt graphic ---
	spriteData := data.PhotonBolt[g.BoltGraphic]
	addr := dispAddr
	for i := 0; i < 8 && i < len(spriteData); i++ {
		g.Buf.DrawByteOR(addr, spriteData[i])
		addr = screen.NextScanLine(addr)
	}
	// Set bolt attribute to white ink on current paper
	g.Buf.Poke(attrAddr, g.Buf.Peek(attrAddr)|0x07)

	// --- Draw trail at the PREVIOUS position (one step closer to player) ---
	// The previous position is BoltOffset+1 (one row nearer the bottom).
	g.drawBoltTrail()

	// --- Advance bolt toward horizon ---
	g.BoltFrames--
	if g.BoltFrames <= 0 {
		g.resetBolt()
		return
	}
	g.BoltOffset--    // move toward horizon (lower index)
	g.BoltGraphic++   // smaller sprite
}

// drawBoltTrail colours the PREVIOUS bolt position with $23 (green paper,
// magenta ink) to leave a visible trail. Matches the trail colouring in the
// original at $6870.
func (g *GameEnv) drawBoltTrail() {
	prevOffset := g.BoltOffset + 1
	if prevOffset >= len(data.ScreenOffsets) {
		return
	}
	offEntry := data.ScreenOffsets[prevOffset]
	screenCol := offEntry[0] + byte(g.BoltX)
	attrAddr := uint16(offEntry[2])<<8 | uint16(screenCol)
	// $23 = green paper (4<<3=0x20), magenta ink (3=0x03) = 0x23
	g.Buf.Poke(attrAddr, 0x23)
}

func (g *GameEnv) hitEnemy(idx int) {
	g.EnemyActive[idx] = false
	g.EnemyPos[idx] = 0
	g.EnemyMoveUnits[idx] = 0
	g.ExplodeFrames = 7
	g.ExplodePos = g.EnemyPos[idx]
	g.BoltHit = true
	g.evEnemyHit = true
	g.incrementScore()
	g.incrementScore()
	g.resetBolt()
}

func (g *GameEnv) hitBonus() {
	g.BonusFlags |= 0x80 // mark as hit
	g.BonusTimer = 7
	g.incrementScore()
	g.incrementScore()
	g.resetBolt()
}

// resetBolt resets all bolt state. Matches $686B exactly:
// BoltOffset=15 (near bottom), BoltFrames=7, BoltGraphic=0, BoltX=$10.
func (g *GameEnv) resetBolt() {
	g.Firing = false
	g.BoltX = 0x10
	g.BoltFrames = 7
	g.BoltOffset = 15  // start near bottom of playing area
	g.BoltGraphic = 0  // largest bolt graphic
	g.BoltHit = false
}

func (g *GameEnv) processHitBike() {
	g.ExplodeFrames--
	if g.ExplodeFrames <= 0 {
		g.BoltHit = false
		g.NumEnemies--

		if g.NumEnemies <= 0 {
			// Both bikes destroyed — advance sector
			g.switchSector()
		}
	}
}

// --- Sector management ---

// switchSector replicates $62CB: reset enemies, flip day/night, advance sector.
func (g *GameEnv) switchSector() {
	g.evSectorChange = true

	// Reset enemy bikes ($62CB-$62D3)
	g.NumEnemies = 2
	g.EnemyActive = [2]bool{true, true}
	g.EnemyPos = [2]int{0, 0x20}
	g.EnemyDir = [2]int{0, 0}
	g.EnemyDirFrames = [2]int{3, 3}
	g.EnemyMoveUnits = [2]int{0, 0}
	g.EnemyDistance = DistanceFar
	g.EnemyDistCount = 3
	g.BonusFlags &^= 0x20 // reset bonus appearance flag
	g.resetBolt()

	// Toggle day/night ($62D5-$62D9: RRCA on DayNightFlag)
	g.DayNightFlag = (g.DayNightFlag >> 1) | (g.DayNightFlag << 7)
	g.IsNight = g.DayNightFlag&0x01 != 0

	// Day transition increments sector ($60C5-$60C8)
	if !g.IsNight {
		g.Sector++
		if g.Sector > MaxSector {
			g.Sector = 1
			// Big bonus ($6C17): increment score 15 times
			for i := 0; i < 15; i++ {
				g.incrementScore()
			}
		}
		// Populate trees for new sector density ($64B7)
		g.populateTrees()
	}

	g.State = StateSectorChange
	g.FrameCount = 0
}

// --- Bonus enemy ---

func (g *GameEnv) moveBonusEnemy() {
	if g.BonusFlags&0x80 != 0 {
		// Being blown up
		g.BonusTimer--
		if g.BonusTimer <= 0 {
			g.BonusFlags = 0x20 // should appear again later
			g.BonusX = 0
			g.BonusTimer = 0x32
		}
		return
	}

	if g.BonusFlags&0x20 != 0 {
		return // Not time yet
	}

	if g.BonusFlags&0x01 == 0 {
		// No bonus enemy active — countdown
		g.BonusTimer--
		if g.BonusTimer <= 0 {
			g.BonusFlags |= 0x01 // activate plane
			g.BonusTimer = 0x32
		}
		return
	}

	// Move the bonus enemy across
	if g.BikeMoving {
		g.BonusX -= g.Handlebar
	}
	g.BonusX++

	if g.BonusX <= 0 || g.BonusX >= 30 {
		g.BonusFlags = 0 // off screen
		g.BonusX = 0
	}
}

// --- Score ---

func (g *GameEnv) incrementScore() {
	// Increment ASCII score digits (rightmost first)
	for i := 5; i >= 0; i-- {
		g.Score[i]++
		if g.Score[i] <= '9' {
			return
		}
		g.Score[i] = '0'
	}
}

func (g *GameEnv) ScoreValue() int {
	val := 0
	for _, d := range g.Score {
		val = val*10 + int(d-'0')
	}
	return val
}

func (g *GameEnv) updateHighScore() {
	for i := 0; i < 6; i++ {
		if g.Score[i] > g.HighScore[i] {
			g.HighScore = g.Score
			return
		}
		if g.Score[i] < g.HighScore[i] {
			return
		}
	}
}

// --- Random ---

func (g *GameEnv) randomByte() byte {
	return byte(g.rng.Intn(256))
}
