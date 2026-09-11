package run

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
)

// Keep canonical sealing work small enough for frequent observation tasks.
// Full history remains in ledger event rows, independent of segment size.
const JournalSegmentEntries = 64

type journalAttemptState struct {
	Failed    bool `json:"failed,omitempty"`
	Cancelled bool `json:"cancelled,omitempty"`
}

// The checkpoint carries validation state across archived event segments.
// Completed attempts leave no per-attempt allocation behind; only the latest
// ordinal/outcome per graph node and currently active attempts remain.
type journalCheckpoint struct {
	Sequence uint64                         `json:"sequence"`
	LastAt   time.Time                      `json:"lastAt"`
	History  artifact.Digest                `json:"history,omitempty"`
	Latest   map[string]int                 `json:"latest"`
	Active   map[string]journalAttemptState `json:"active"`
	Terminal map[string]AttemptOutcome      `json:"terminal"`
}

func newJournalCheckpoint() journalCheckpoint {
	return journalCheckpoint{Latest: map[string]int{}, Active: map[string]journalAttemptState{}, Terminal: map[string]AttemptOutcome{}}
}

func validateJournalSegment(checkpoint *journalCheckpoint, entries []journalEntry, startedAt *time.Time, closed, succeeded bool) (journalCheckpoint, error) {
	cursor := newJournalCheckpoint()
	if checkpoint != nil {
		if err := checkpoint.validate(startedAt); err != nil {
			return cursor, err
		}
		cursor = *checkpoint
	}
	if len(entries) > JournalSegmentEntries {
		return cursor, ErrJournalOrder
	}
	next, err := cursor.advance(entries, startedAt)
	if err != nil {
		return cursor, err
	}
	return next, next.finish(closed, succeeded)
}

func (c journalCheckpoint) validate(startedAt *time.Time) error {
	if startedAt == nil || c.Sequence == 0 || c.Sequence%JournalSegmentEntries != 0 || !c.History.Valid() || c.LastAt.Location() != time.UTC || c.LastAt.Before(*startedAt) || c.Latest == nil || c.Active == nil || c.Terminal == nil || len(c.Latest) > MaxJournalEntries || len(c.Active) > MaxJournalEntries {
		return ErrJournalOrder
	}
	for key, attempt := range c.Latest {
		parts := strings.Split(key, "\x00")
		if len(parts) < 2 || len(parts) > MaxJournalGraphPathSegments+1 || attempt < 1 {
			return ErrJournalOrder
		}
		for _, part := range parts {
			if !attributionPattern.MatchString(part) {
				return ErrJournalOrder
			}
		}
	}
	for key := range c.Active {
		index := strings.LastIndexByte(key, 0)
		if index < 0 {
			return ErrJournalOrder
		}
		attempt, err := strconv.Atoi(key[index+1:])
		if err != nil || attempt < 1 || attempt > c.Latest[key[:index]] {
			return ErrJournalOrder
		}
	}
	for key, outcome := range c.Terminal {
		if c.Latest[key] < 1 || outcome == AttemptStarted || !validAttemptOutcome(outcome) {
			return ErrJournalOrder
		}
	}
	return nil
}

func (c journalCheckpoint) clone() journalCheckpoint {
	c.Latest = maps.Clone(c.Latest)
	c.Active = maps.Clone(c.Active)
	c.Terminal = maps.Clone(c.Terminal)
	return c
}

func (c *journalCheckpoint) append(entry journalEntry, startedAt *time.Time) error {
	if startedAt == nil || entry.Sequence != c.Sequence+1 || validateJournalFact(entry) != nil || entry.OccurredAt.Before(*startedAt) {
		return ErrJournalOrder
	}
	nodeKey := strings.Join(entry.GraphPath, "\x00") + "\x00" + entry.NodeID
	attemptKey := fmt.Sprintf("%s\x00%d", nodeKey, entry.Attempt)
	state, active := c.Active[attemptKey]
	switch entry.Kind {
	case JournalNodeAttempt:
		if entry.AttemptOutcome == AttemptStarted {
			if active || entry.Attempt != c.Latest[nodeKey]+1 || len(c.Active) >= MaxJournalEntries {
				return ErrJournalOrder
			}
			c.Active[attemptKey] = journalAttemptState{}
			c.Latest[nodeKey] = entry.Attempt
		} else {
			if !active || entry.AttemptOutcome == AttemptSucceeded && (state.Failed || state.Cancelled) || entry.AttemptOutcome == AttemptCancelled && state.Failed {
				return ErrJournalOrder
			}
			delete(c.Active, attemptKey)
			c.Terminal[nodeKey] = entry.AttemptOutcome
		}
	case JournalAdapterAction:
		if !active {
			return ErrJournalOrder
		}
		state.Failed = state.Failed || entry.ActionOutcome == ActionFailed
		state.Cancelled = state.Cancelled || entry.ActionOutcome == ActionCancelled
		c.Active[attemptKey] = state
	case JournalNodeStatus:
		if !active {
			return ErrJournalOrder
		}
	}
	c.Sequence = entry.Sequence
	if entry.OccurredAt.After(c.LastAt) {
		c.LastAt = entry.OccurredAt
	}
	return nil
}

func (c journalCheckpoint) finish(closed, succeeded bool) error {
	if closed && len(c.Active) != 0 {
		return ErrJournalOrder
	}
	if succeeded {
		for _, outcome := range c.Terminal {
			if outcome != AttemptSucceeded && outcome != AttemptRouted && outcome != AttemptCancelled {
				return ErrJournalOrder
			}
		}
	}
	return nil
}

func (c journalCheckpoint) advance(entries []journalEntry, startedAt *time.Time) (journalCheckpoint, error) {
	next := c.clone()
	for _, entry := range entries {
		if err := next.append(entry, startedAt); err != nil {
			return journalCheckpoint{}, err
		}
	}
	return next, nil
}

func (c journalCheckpoint) archive(entries []journalEntry, startedAt *time.Time) (journalCheckpoint, error) {
	next, err := c.advance(entries, startedAt)
	if err != nil {
		return journalCheckpoint{}, err
	}
	raw, err := artifact.Marshal(struct {
		Previous artifact.Digest `json:"previous"`
		Entries  []journalEntry  `json:"entries"`
	}{c.History, entries})
	if err != nil {
		return journalCheckpoint{}, err
	}
	next.History, err = artifact.Sum("yotta/run-journal-segment/v1", raw)
	return next, err
}
