package claude

import (
	"errors"
	"testing"
)

func TestScopeOf(t *testing.T) {
	root := errors.New("root")
	turnErr := wrapError(ScopeTurn, "run turn", root)
	if got := ScopeOf(turnErr); got != ScopeTurn {
		t.Fatalf("ScopeOf(turnErr) = %q, want %q", got, ScopeTurn)
	}
	if !errors.Is(turnErr, root) {
		t.Fatal("wrapped error must preserve its cause")
	}
	if got := ScopeOf(root); got != ScopeSession {
		t.Fatalf("unknown errors must default to session scope, got %q", got)
	}
}
