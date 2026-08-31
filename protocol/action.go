package protocol

import "errors"

// The interaction control sentinels. They are public so an external host's
// Controller can return them and Transport still maps them to the same stable
// command-result codes.
var (
	// ErrActionNotPending is the single stable answer to any decision that lost
	// the race: unknown, already resolved, or ended by cancel/stop/failure.
	ErrActionNotPending = errors.New("action not pending")
	// ErrInvalidAction reports a resolution that does not fit the action shape.
	ErrInvalidAction = errors.New("invalid action resolution")
	// ErrInvalidAnswer reports answers that do not cover the questions exactly.
	ErrInvalidAnswer = errors.New("invalid question answers")
	// ErrTurnNotCancellable reports that the named Turn is not the running one.
	ErrTurnNotCancellable = errors.New("turn not cancellable")
	// ErrRequestNotQueued reports that the named request is not a FIFO entry.
	ErrRequestNotQueued = errors.New("request not queued")
)

// ActionDecision is the decision a client applies to a tool approval. Question
// answers carry no decision value and leave it empty.
type ActionDecision string

const (
	DecisionAllowOnce    ActionDecision = "allow_once"
	DecisionAllowSession ActionDecision = "allow_session"
	DecisionDeny         ActionDecision = "deny"
)

// IsKnown reports whether the decision is one of the closed approval set.
func (decision ActionDecision) IsKnown() bool {
	switch decision {
	case DecisionAllowOnce, DecisionAllowSession, DecisionDeny:
		return true
	}
	return false
}

// QuestionAnswer answers exactly one question of a pending ask_user_question
// action. Clients address questions by questionId, never by position.
type QuestionAnswer struct {
	QuestionID string   `json:"questionId"`
	Values     []string `json:"values"`
}
