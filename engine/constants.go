package engine

// Game states
const (
	StateTitle   = iota
	StatePlaying
	StateDead
	StateGameOver
	StateSectorChange
)

// Speed values (matching original: 0=fast, 2=slow/stopped)
const (
	SpeedFast = 0
	SpeedSlow = 1
	SpeedIdle = 2
)

// Handlebar positions
const (
	HandlebarLeft    = -1
	HandlebarCentre  = 0
	HandlebarRight   = 1
)

// Enemy bike distance states
const (
	DistanceFar     = 0
	DistanceMedium  = 1
	DistanceNear    = 2
	DistanceInRange = 3
)

// Playing area sizes (from $5DE6)
const (
	PlayingAreaSmall = 0x01A0 // 416
	PlayingAreaLarge = 0x02E0 // 736
)

// Direction values for enemy bikes
const (
	DirLeft   = -1
	DirCentre = 0
	DirRight  = 1
)

// MaxSector is the highest sector before wrapping
const MaxSector = 8

// Tree buffer size
const TreeBufferSize = 256

// Playing field dimensions in the working buffer
// The working buffer at $7CA0-$7F9F represents the playing field
// It's 32 columns wide, always 24 rows deep
const (
	FieldCols  = 0x20 // 32 columns per row
	FieldDepth = 24   // always 24 rows ($7CA0-$7F9F = 768 bytes)
)

// Tree markers in the field buffer
const (
	TreeNone  = 0x00
	TreeSmall = 0x20
	TreeLarge = 0x40
)

// Number of screen offset entries (rows rendered from top to bottom)
const NumScreenRows = 20

// Default attribute for clear screen
const DefaultAttr = 0x3B // bright white paper, cyan ink
