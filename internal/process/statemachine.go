// Package process defines the state machine for the diary AI processing pipeline.
//
// Design goal: every state transition must be explicit, logged, and validated.
// Invalid transitions are rejected at runtime so that AI or human bugs cannot
// silently corrupt processing state.
package process

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represents a discrete step in the processing lifecycle.
type State string

// Event represents something that can trigger a state transition.
type Event string

// Severity classifies errors into recoverable or terminal buckets.
type Severity string

const (
	SeverityTransient   Severity = "transient"
	SeverityUnrecoverable Severity = "unrecoverable"
)

// Process-level states.
const (
	StateIdle             State = "Idle"
	StateScanning         State = "Scanning"
	StateNoChanges        State = "NoChanges"
	StateAlreadyProcessed State = "AlreadyProcessed"
	StateReading          State = "Reading"
	StateExtracting       State = "Extracting"
	StateRetryPending     State = "RetryPending"
	StateDeduplicating    State = "Deduplicating"
	StateMerging          State = "Merging"
	StatePersisting       State = "Persisting"
	StateSummarizing      State = "Summarizing"
	StateReporting        State = "Reporting"
	StateDone             State = "Done"
	StateFatalError       State = "FatalError"
)

// Process-level events.
const (
	EventStartProcess          Event = "StartProcess"
	EventIdempotencyHit        Event = "IdempotencyHit"
	EventNoChanges             Event = "NoChanges"
	EventChangesFound          Event = "ChangesFound"
	EventContentLoaded         Event = "ContentLoaded"
	EventIOError               Event = "IOError"
	EventExtractionOK          Event = "ExtractionOK"
	EventTransientAIError      Event = "TransientAIError"
	EventUnrecoverableAIError  Event = "UnrecoverableAIError"
	EventRetry                 Event = "Retry"
	EventMaxRetriesExceeded    Event = "MaxRetriesExceeded"
	EventDuplicatesResolved    Event = "DuplicatesResolved"
	EventConflictUnresolvable  Event = "ConflictUnresolvable"
	EventMergeComplete         Event = "MergeComplete"
	EventMergeInvariantViolated Event = "MergeInvariantViolated"
	EventPersistOK             Event = "PersistOK"
	EventDBError               Event = "DBError"
	EventSummaryOK             Event = "SummaryOK"
	EventMDWriteError          Event = "MDWriteError"
	EventReportEmitted         Event = "ReportEmitted"
)

var (
	// ErrInvalidTransition is returned when an event cannot be applied in the current state.
	ErrInvalidTransition = errors.New("invalid state transition")

	// ErrTerminalState is returned when a transition is attempted from a terminal state.
	ErrTerminalState = errors.New("cannot transition from a terminal state")

	// ErrRetryExhausted is returned when retry is attempted after the maximum count.
	ErrRetryExhausted = errors.New("retry limit exceeded")
)

// transitionTable defines the only legal (state, event) -> nextState moves.
// This is the single source of truth for the process lifecycle.
var transitionTable = map[State]map[Event]State{
	StateIdle: {
		EventStartProcess:   StateScanning,
		EventIdempotencyHit: StateAlreadyProcessed,
	},
	StateScanning: {
		EventNoChanges:    StateNoChanges,
		EventChangesFound: StateReading,
	},
	StateReading: {
		EventContentLoaded: StateExtracting,
		EventIOError:       StateFatalError,
	},
	StateExtracting: {
		EventExtractionOK:         StateDeduplicating,
		EventTransientAIError:     StateRetryPending,
		EventUnrecoverableAIError: StateFatalError,
	},
	StateRetryPending: {
		EventRetry:              StateExtracting,
		EventMaxRetriesExceeded: StateFatalError,
	},
	StateDeduplicating: {
		EventDuplicatesResolved:   StateMerging,
		EventConflictUnresolvable: StateFatalError,
	},
	StateMerging: {
		EventMergeComplete:          StatePersisting,
		EventMergeInvariantViolated: StateFatalError,
	},
	StatePersisting: {
		EventPersistOK: StateSummarizing,
		EventDBError:   StateFatalError,
	},
	StateSummarizing: {
		EventSummaryOK:    StateReporting,
		EventMDWriteError: StateFatalError,
	},
	StateReporting: {
		EventReportEmitted: StateDone,
	},
}

// terminalStates lists states from which no further transition is allowed.
var terminalStates = map[State]struct{}{
	StateNoChanges:        {},
	StateAlreadyProcessed: {},
	StateDone:             {},
	StateFatalError:       {},
}

// TransitionLog records a single state change for audit purposes.
type TransitionLog struct {
	From      State     `json:"from"`
	To        State     `json:"to"`
	Event     Event     `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	RunID     string    `json:"run_id"`
	Reason    string    `json:"reason,omitempty"`
}

// Machine is the runtime state machine for one process run.
type Machine struct {
	mu sync.Mutex

	state      State
	runID      string
	baseHash   string
	maxRetries int
	retryCount int
	log        []TransitionLog
}

// NewMachine creates a state machine starting in StateIdle.
func NewMachine(runID, baseHash string, maxRetries int) *Machine {
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Machine{
		state:      StateIdle,
		runID:      runID,
		baseHash:   baseHash,
		maxRetries: maxRetries,
		log:        make([]TransitionLog, 0),
	}
}

// State returns the current state. Safe for concurrent use.
func (m *Machine) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// IsTerminal reports whether the current state is terminal.
func (m *Machine) IsTerminal() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isTerminalLocked()
}

func (m *Machine) isTerminalLocked() bool {
	_, ok := terminalStates[m.state]
	return ok
}

// RetryCount returns how many retries have been consumed.
func (m *Machine) RetryCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.retryCount
}

// MaxRetries returns the configured retry ceiling.
func (m *Machine) MaxRetries() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxRetries
}

// Log returns a copy of the transition audit log.
func (m *Machine) Log() []TransitionLog {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TransitionLog, len(m.log))
	copy(out, m.log)
	return out
}

// Transition attempts to move the machine to the next state for the given event.
// It enforces:
//   1. terminal-state guard
//   2. allow-list transition table
//   3. retry-count ceiling when leaving RetryPending
func (m *Machine) Transition(event Event, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isTerminalLocked() {
		return fmt.Errorf("%w: current state %s is terminal", ErrTerminalState, m.state)
	}

	next, ok := transitionTable[m.state][event]
	if !ok {
		return fmt.Errorf("%w: event %s not allowed in state %s", ErrInvalidTransition, event, m.state)
	}

	// Guard retry logic explicitly.
	if m.state == StateRetryPending && event == EventRetry {
		if m.retryCount >= m.maxRetries {
			return fmt.Errorf("%w: retryCount=%d maxRetries=%d", ErrRetryExhausted, m.retryCount, m.maxRetries)
		}
		m.retryCount++
	}

	entry := TransitionLog{
		From:      m.state,
		To:        next,
		Event:     event,
		Timestamp: time.Now().UTC(),
		RunID:     m.runID,
		Reason:    reason,
	}
	m.log = append(m.log, entry)
	m.state = next
	return nil
}

// ForceFatal transitions the machine directly to FatalError from any non-terminal state.
// This is reserved for catastrophic failures (e.g., panic recovery) and is itself logged.
func (m *Machine) ForceFatal(reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isTerminalLocked() {
		return fmt.Errorf("%w: current state %s is terminal", ErrTerminalState, m.state)
	}

	entry := TransitionLog{
		From:      m.state,
		To:        StateFatalError,
		Event:     EventUnrecoverableAIError, // generic fatal marker
		Timestamp: time.Now().UTC(),
		RunID:     m.runID,
		Reason:    reason,
	}
	m.log = append(m.log, entry)
	m.state = StateFatalError
	return nil
}

// ProcessRunID returns the identifier for this run.
func (m *Machine) ProcessRunID() string { return m.runID }

// BaseHash returns the content hash used for idempotency.
func (m *Machine) BaseHash() string { return m.baseHash }
