package orchestrator

// Canonical state string values. These are the literals persisted to the
// database. Both FSMs reference these constants so the values stay in one
// place and can never silently diverge.
const (
	statePending   = "pending"
	stateRunning   = "running"
	stateSuccess   = "success"
	stateFailed    = "failed"
	stateCanceled  = "canceled"
	stateSucceeded = "succeeded"
)
