package engine

// StepResult is the observation returned after each Step call.
type StepResult struct {
	State  int
	Score  int
	Lives  int
	Sector int

	// Audio events for the current frame.
	EngineOn     bool // bike is moving
	Fired        bool // photon bolt just fired
	EnemyHit     bool // enemy bike destroyed
	TreeCrash    bool // player hit a tree
	SectorChange bool // sector transition started
	GameOver     bool // game over triggered
}
