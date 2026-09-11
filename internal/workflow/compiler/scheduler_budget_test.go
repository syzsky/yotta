package compiler

import (
	"context"
	"strings"
	"testing"
)

func TestReadyQueueBudgetIncludesSuspendedParentWork(t *testing.T) {
	s := &scheduler{queue: make([]scheduledInvocation, 1), frames: []*controlFrame{{parent: make([]scheduledInvocation, MaxReadyInvocations)}}}
	err := s.step(context.Background())
	if err == nil || !strings.Contains(err.Error(), "ready queue budget") {
		t.Fatalf("unbounded suspended parent queue: %v", err)
	}
}
