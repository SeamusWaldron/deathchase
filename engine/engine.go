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
	Field [PlayingAreaLarge + FieldCols]byte

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
}

// NewGameEnv creates a new game environment.
func NewGameEnv() *GameEnv {
	g := &GameEnv{
		Buf:          screen.NewBuffer(),
		State:        StateTitle,
		rng:          rand.New(rand.NewSource(42)),
		FlipFlop:     0x55,
		DayNightFlag: 0x55,
	}
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
	g.BoltOffset = 0
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

	// Run initial tree generation (10 iterations like the original at $6601-$6608)
	g.BikeMoving = true
	for i := 0; i < 10; i++ {
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
	// Draw title screen
	g.clearScreen()
	g.Buf.PrintString(0x48AB, "DEATHCHASE")
	g.Buf.PrintString(0x488A, " 1=KEYBOARD ")
	g.Buf.PrintString(0x48CA, " 2=KEMPSTON ")
	g.Buf.PrintString(0x5006, "\x7F MERVYN ESTCOURT 1983")

	if act.Enter {
		g.UseKempston = false
		g.Reset()
	}

	return StepResult{State: g.State}
}

func (g *GameEnv) stepPlaying(act action.Action) StepResult {
	result := StepResult{State: g.State}

	// 1. Check input (speed)
	g.handleSpeedInput(act)

	// 2. Check if bolt hit something (from previous frame)
	if !g.BoltHit {
		g.handleFireInput(act)
	}

	// 3. Respond to steering
	g.handleSteering(act)

	// 4. Move trees
	g.moveTrees()

	// 5. Adjust photon bolt position if firing
	if g.Firing {
		g.adjustBoltPosition()
	}

	// 6. Clear and redraw the playing field
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
		return result
	}

	// 11. Move bikes nearer/further
	g.moveBikesDistance()

	// 12. Move bonus enemy
	g.moveBonusEnemy()

	// 13. Draw photon bolt trail + check hits
	if !g.BoltHit && g.Firing {
		g.checkBoltHit()
	}

	// 14. Draw HUD
	g.drawBike()
	g.drawHUD()

	result.Score = g.scoreValue()
	result.Lives = g.Lives
	result.Sector = g.Sector
	return result
}

func (g *GameEnv) stepDead(act action.Action) StepResult {
	// Brief pause then check lives
	g.FrameCount++
	if g.FrameCount > 30 {
		if g.Lives <= 0 {
			g.State = StateGameOver
			g.FrameCount = 0
		} else {
			g.State = StatePlaying
			g.Speed = SpeedIdle
			g.BikeMoving = false
			g.Firing = false
			g.resetBolt()
			g.clearField()
			g.BikeMoving = true
			for i := 0; i < 10; i++ {
				g.moveTrees()
			}
			g.BikeMoving = false
			g.clearScreen()
			g.renderField()
			g.drawBike()
			g.drawHUD()
		}
	}
	return StepResult{State: g.State, Lives: g.Lives, Score: g.scoreValue(), Sector: g.Sector}
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
	return StepResult{State: g.State, Score: g.scoreValue(), Lives: g.Lives, Sector: g.Sector}
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

	return StepResult{State: g.State, Score: g.scoreValue(), Lives: g.Lives, Sector: g.Sector}
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

func (g *GameEnv) handleFireInput(act action.Action) {
	if !act.Fire {
		return
	}
	if g.Speed != SpeedFast {
		return // Can only fire at full speed
	}
	if g.BonusFlags&0x80 != 0 {
		return // Something already being blown up
	}
	g.Firing = true
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
func (g *GameEnv) moveTrees() {
	if !g.BikeMoving {
		return
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
		backRow := g.playingAreaRows() - 1
		for col := 0; col < 30; col++ {
			idx := (srcPos - col) & 0xFF
			g.setField(backRow, col, g.TreeBuf[idx])
		}
	}

	// Shift all rows forward (towards the player) by one row
	totalRows := g.playingAreaRows()
	for row := 0; row < totalRows-1; row++ {
		for col := 0; col < FieldCols; col++ {
			g.setField(row, col, g.getField(row+1, col))
		}
	}

	// Apply perspective widening for close trees
	g.applyPerspective()
}

// applyPerspective widens trees in the closest rows to simulate perspective.
// In the original, this handles the left shunt ($5E2C), right shunt ($5E53),
// and closest-row expansion ($5EE1).
func (g *GameEnv) applyPerspective() {
	totalRows := g.playingAreaRows()
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

	// Expand closest trees to take up more area ($5EE1)
	for col := 0; col < FieldCols-3; col++ {
		if g.getField(0, col) == TreeSmall {
			g.setField(0, col+1, TreeSmall)
			g.setField(0, col+2, TreeLarge)
			g.setField(0, col+3, TreeLarge)
		}
	}
}

func (g *GameEnv) playingAreaRows() int {
	return g.PlayingArea / FieldCols
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
// This replicates the Print objects routine at $5F89.
func (g *GameEnv) renderField() {
	numRows := NumScreenRows
	if numRows > g.playingAreaRows() {
		numRows = g.playingAreaRows()
	}

	for screenRow := 0; screenRow < numRows; screenRow++ {
		// Map screen row to field row (front rows first)
		fieldRow := screenRow

		if screenRow >= len(data.ScreenOffsets) {
			break
		}
		offEntry := data.ScreenOffsets[screenRow]
		baseCol := offEntry[0]
		dispHi := offEntry[1]
		attrHi := offEntry[2]

		for col := 0; col < 30; col++ {
			treeVal := g.getField(fieldRow, col+1)
			if treeVal == 0 {
				continue
			}

			// Calculate screen position
			screenCol := baseCol + byte(col)
			dispAddr := uint16(dispHi)<<8 | uint16(screenCol)
			attrAddr := uint16(attrHi)<<8 | uint16(screenCol)

			// Draw tree graphic based on size and screen row
			if treeVal == TreeLarge {
				g.drawLargeTree(dispAddr, attrAddr, screenRow)
			} else {
				g.drawSmallTree(dispAddr, attrAddr, screenRow)
			}

			// Draw enemy bikes at this position
			for e := 0; e < 2; e++ {
				if g.EnemyActive[e] && g.EnemyPos[e] == col+1 {
					g.drawEnemyBike(e, dispAddr, attrAddr)
				}
			}
		}
	}
}

func (g *GameEnv) drawSmallTree(dispAddr, attrAddr uint16, depth int) {
	// Small tree: 8 scanlines of pixel data
	// Tree appearance varies by depth — further trees are smaller
	if depth < 10 {
		// Far trees — just a vertical line pattern
		for i := 0; i < 8; i++ {
			pattern := byte(0x18) // simple trunk
			if i < 4 {
				pattern = 0x3C // simple canopy
			}
			g.Buf.DrawByteOR(dispAddr, pattern)
			dispAddr = screen.NextScanLine(dispAddr)
		}
	} else {
		// Near trees — bigger
		for i := 0; i < 8; i++ {
			pattern := byte(0x18)
			if i < 6 {
				pattern = 0x7E
			}
			g.Buf.DrawByteOR(dispAddr, pattern)
			dispAddr = screen.NextScanLine(dispAddr)
		}
	}

	// Set tree colour attribute
	if g.IsNight {
		g.Buf.Poke(attrAddr, g.Buf.Peek(attrAddr)&0xF8|0x04) // green ink
	} else {
		g.Buf.Poke(attrAddr, 0x24) // green ink, green paper
	}
}

func (g *GameEnv) drawLargeTree(dispAddr, attrAddr uint16, depth int) {
	// Large tree: fills more space
	for i := 0; i < 8; i++ {
		pattern := byte(0x18)
		if i < 7 {
			pattern = 0xFF
		}
		g.Buf.DrawByteOR(dispAddr, pattern)
		dispAddr = screen.NextScanLine(dispAddr)
	}
	if g.IsNight {
		g.Buf.Poke(attrAddr, g.Buf.Peek(attrAddr)&0xF8|0x04)
	} else {
		g.Buf.Poke(attrAddr, 0x2C) // green ink, green paper, bright
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

	// For near/in-range bikes, draw second row
	if g.EnemyDistance >= DistanceNear && len(spriteData) > 8 {
		// Move to next character row
		nextRowAddr := dispAddr + 0x20 // next char row in display
		for i := 8; i < 16 && i < len(spriteData); i++ {
			g.Buf.DrawByteOR(nextRowAddr, spriteData[i])
			nextRowAddr = screen.NextScanLine(nextRowAddr)
		}
	}

	// Set attribute — yellow for bike 1, blue for bike 2
	if enemyIdx == 0 {
		g.Buf.Poke(attrAddr, g.Buf.Peek(attrAddr)|0x06) // yellow ink
	} else {
		g.Buf.Poke(attrAddr, g.Buf.Peek(attrAddr)|0x01) // blue ink
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

func (g *GameEnv) handleDeath() {
	g.Lives--
	g.Speed = SpeedIdle
	g.BikeMoving = false
	g.Firing = false
	g.resetBolt()
	g.FrameCount = 0

	if g.Lives <= 0 {
		g.State = StateGameOver
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

func (g *GameEnv) adjustBoltPosition() {
	// Bolt moves with player's steering
	g.BoltX += g.Handlebar
}

func (g *GameEnv) checkBoltHit() {
	if !g.Firing {
		return
	}

	g.BoltFrames--
	if g.BoltFrames <= 0 {
		g.resetBolt()
		return
	}

	g.BoltOffset++
	g.BoltGraphic++

	// Check if bolt hit an enemy bike (only if in range)
	if g.EnemyDistance == DistanceInRange {
		for e := 0; e < 2; e++ {
			if !g.EnemyActive[e] {
				continue
			}
			// Simple hit detection: bolt X matches enemy position
			diff := g.BoltX - g.EnemyPos[e]
			if diff >= -1 && diff <= 1 {
				g.hitEnemy(e)
				return
			}
		}
	}

	// Check if bolt hit bonus enemy
	if g.BonusFlags&0x01 != 0 || g.BonusFlags&0x02 != 0 {
		diff := g.BoltX - g.BonusX
		if diff >= -1 && diff <= 1 {
			g.hitBonus()
			return
		}
	}
}

func (g *GameEnv) hitEnemy(idx int) {
	g.EnemyActive[idx] = false
	g.EnemyPos[idx] = 0
	g.EnemyMoveUnits[idx] = 0
	g.ExplodeFrames = 7
	g.ExplodePos = g.EnemyPos[idx]
	g.BoltHit = true
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

func (g *GameEnv) resetBolt() {
	g.Firing = false
	g.BoltX = 0x10
	g.BoltFrames = 7
	g.BoltOffset = 0
	g.BoltGraphic = 0
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

func (g *GameEnv) switchSector() {
	g.NumEnemies = 2
	g.EnemyActive = [2]bool{true, true}
	g.EnemyPos = [2]int{0, 0x20}
	g.EnemyDistance = DistanceFar
	g.EnemyDistCount = 3
	g.BonusFlags &^= 0x20 // reset bonus appearance flag

	// Toggle day/night
	g.DayNightFlag = (g.DayNightFlag >> 1) | (g.DayNightFlag << 7)
	g.IsNight = g.DayNightFlag&0x01 != 0

	// Advance sector
	g.Sector++
	if g.Sector > MaxSector {
		g.Sector = 1
		// Big bonus for completing all 8 sectors
		for i := 0; i < 15; i++ {
			g.incrementScore()
		}
	}

	// Populate trees for new sector (only on day transitions)
	if !g.IsNight {
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

func (g *GameEnv) scoreValue() int {
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
