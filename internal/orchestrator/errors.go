package orchestrator

import "errors"

// ErrInvalidTransition is returned when a state transition is not permitted by the FSM.
var ErrInvalidTransition = errors.New("invalid state transition")
