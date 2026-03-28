package engine

// StepResult is the observation returned after each Step call.
type StepResult struct {
	State  int
	Score  int
	Lives  int
	Sector int
}
