package action

// Action represents a player input for one frame.
type Action struct {
	Left       bool
	Right      bool
	Accelerate bool
	Brake      bool
	Fire       bool
	Enter      bool
	Escape     bool
}
