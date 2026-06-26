package process

import (
	"errors"
	"sync"
	"testing"
)

// happyPath returns the canonical successful transition sequence.
func happyPath() []Event {
	return []Event{
		EventStartProcess,
		EventChangesFound,
		EventContentLoaded,
		EventExtractionOK,
		EventDuplicatesResolved,
		EventMergeComplete,
		EventPersistOK,
		EventSummaryOK,
		EventReportEmitted,
	}
}

func TestHappyPathReachesDone(t *testing.T) {
	m := NewMachine("run-1", "hash-abc", 2)

	for _, event := range happyPath() {
		if err := m.Transition(event, ""); err != nil {
			t.Fatalf("transition %s failed: %v", event, err)
		}
	}

	if m.State() != StateDone {
		t.Fatalf("expected terminal state %s, got %s", StateDone, m.State())
	}
	if !m.IsTerminal() {
		t.Fatal("expected Done to be terminal")
	}

	log := m.Log()
	if len(log) != len(happyPath()) {
		t.Fatalf("expected %d log entries, got %d", len(happyPath()), len(log))
	}

	for i, entry := range log {
		if entry.Event != happyPath()[i] {
			t.Fatalf("log[%d].Event = %s, want %s", i, entry.Event, happyPath()[i])
		}
		if entry.RunID != "run-1" {
			t.Fatalf("log[%d].RunID = %s, want run-1", i, entry.RunID)
		}
	}
}

func TestNoChangesPath(t *testing.T) {
	m := NewMachine("run-nochanges", "hash-no", 0)

	if err := m.Transition(EventStartProcess, ""); err != nil {
		t.Fatalf("StartProcess failed: %v", err)
	}
	if err := m.Transition(EventNoChanges, "no files changed"); err != nil {
		t.Fatalf("NoChanges failed: %v", err)
	}

	if m.State() != StateNoChanges {
		t.Fatalf("expected %s, got %s", StateNoChanges, m.State())
	}

	if err := m.Transition(EventStartProcess, ""); !errors.Is(err, ErrTerminalState) {
		t.Fatalf("expected terminal-state error, got %v", err)
	}
}

func TestIdempotencyHitPath(t *testing.T) {
	m := NewMachine("run-idem", "hash-idem", 0)

	if err := m.Transition(EventIdempotencyHit, "already processed"); err != nil {
		t.Fatalf("IdempotencyHit failed: %v", err)
	}

	if m.State() != StateAlreadyProcessed {
		t.Fatalf("expected %s, got %s", StateAlreadyProcessed, m.State())
	}
	if !m.IsTerminal() {
		t.Fatal("expected AlreadyProcessed to be terminal")
	}
}

func TestRetrySuccessThenDone(t *testing.T) {
	m := NewMachine("run-retry", "hash-r", 2)

	path := []Event{
		EventStartProcess,
		EventChangesFound,
		EventContentLoaded,
		EventTransientAIError, // 1st AI call fails
		EventRetry,            // retry 1
		EventTransientAIError, // 2nd AI call fails
		EventRetry,            // retry 2
		EventExtractionOK,
		EventDuplicatesResolved,
		EventMergeComplete,
		EventPersistOK,
		EventSummaryOK,
		EventReportEmitted,
	}

	for _, event := range path {
		if err := m.Transition(event, ""); err != nil {
			t.Fatalf("transition %s failed: %v", event, err)
		}
	}

	if m.State() != StateDone {
		t.Fatalf("expected %s, got %s", StateDone, m.State())
	}
	if m.RetryCount() != 2 {
		t.Fatalf("expected retryCount=2, got %d", m.RetryCount())
	}
}

func TestRetryExhaustionGoesFatal(t *testing.T) {
	m := NewMachine("run-retry-exhaust", "hash-re", 1)

	steps := []Event{
		EventStartProcess,
		EventChangesFound,
		EventContentLoaded,
		EventTransientAIError,
		EventRetry,
		EventTransientAIError,
	}

	for _, event := range steps {
		if err := m.Transition(event, ""); err != nil {
			t.Fatalf("transition %s failed: %v", event, err)
		}
	}

	// At this point we are in RetryPending with retryCount == maxRetries == 1.
	if m.RetryCount() != 1 {
		t.Fatalf("expected retryCount=1, got %d", m.RetryCount())
	}

	// Another Retry must fail because we are at the ceiling.
	if err := m.Transition(EventRetry, ""); !errors.Is(err, ErrRetryExhausted) {
		t.Fatalf("expected ErrRetryExhausted, got %v", err)
	}

	// The proper recovery is to fire MaxRetriesExceeded.
	if err := m.Transition(EventMaxRetriesExceeded, "ai keeps failing"); err != nil {
		t.Fatalf("MaxRetriesExceeded failed: %v", err)
	}

	if m.State() != StateFatalError {
		t.Fatalf("expected %s, got %s", StateFatalError, m.State())
	}
}

func TestInvalidTransitionsRejected(t *testing.T) {
	m := NewMachine("run-invalid", "hash-inv", 0)

	// Event not allowed in Idle.
	if err := m.Transition(EventChangesFound, ""); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	// Cannot skip scanning.
	if err := m.Transition(EventStartProcess, ""); err != nil {
		t.Fatalf("StartProcess failed: %v", err)
	}
	if err := m.Transition(EventContentLoaded, ""); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition for skipping Reading, got %v", err)
	}
}

func TestTerminalStatesBlockTransitions(t *testing.T) {
	terminals := []struct {
		name   string
		reach  func(*Machine) error
		state  State
	}{
		{
			name: "NoChanges",
			reach: func(m *Machine) error {
				return errors.Join(
					m.Transition(EventStartProcess, ""),
					m.Transition(EventNoChanges, ""),
				)
			},
			state: StateNoChanges,
		},
		{
			name: "AlreadyProcessed",
			reach: func(m *Machine) error {
				return m.Transition(EventIdempotencyHit, "")
			},
			state: StateAlreadyProcessed,
		},
		{
			name: "Done",
			reach: func(m *Machine) error {
				for _, e := range happyPath() {
					if err := m.Transition(e, ""); err != nil {
						return err
					}
				}
				return nil
			},
			state: StateDone,
		},
		{
			name: "FatalError",
			reach: func(m *Machine) error {
				return errors.Join(
					m.Transition(EventStartProcess, ""),
					m.Transition(EventChangesFound, ""),
					m.Transition(EventIOError, "disk full"),
				)
			},
			state: StateFatalError,
		},
	}

	for _, tc := range terminals {
		t.Run(tc.name, func(t *testing.T) {
			m := NewMachine("run-"+tc.name, "hash-"+tc.name, 0)
			if err := tc.reach(m); err != nil {
				t.Fatalf("failed to reach %s: %v", tc.state, err)
			}
			if m.State() != tc.state {
				t.Fatalf("expected %s, got %s", tc.state, m.State())
			}
			if err := m.Transition(EventStartProcess, ""); !errors.Is(err, ErrTerminalState) {
				t.Fatalf("expected ErrTerminalState from %s, got %v", tc.state, err)
			}
		})
	}
}

func TestFatalErrorFromIO(t *testing.T) {
	m := NewMachine("run-io", "hash-io", 0)

	steps := []Event{
		EventStartProcess,
		EventChangesFound,
		EventIOError,
	}
	for _, e := range steps {
		if err := m.Transition(e, "permission denied"); err != nil {
			t.Fatalf("transition %s failed: %v", e, err)
		}
	}

	if m.State() != StateFatalError {
		t.Fatalf("expected %s, got %s", StateFatalError, m.State())
	}
	if len(m.Log()) != 3 {
		t.Fatalf("expected 3 log entries, got %d", len(m.Log()))
	}
	last := m.Log()[len(m.Log())-1]
	if last.Reason != "permission denied" {
		t.Fatalf("expected reason 'permission denied', got %s", last.Reason)
	}
}

func TestForceFatal(t *testing.T) {
	m := NewMachine("run-force", "hash-f", 0)
	if err := m.Transition(EventStartProcess, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.ForceFatal("panic recovered"); err != nil {
		t.Fatalf("ForceFatal failed: %v", err)
	}
	if m.State() != StateFatalError {
		t.Fatalf("expected %s, got %s", StateFatalError, m.State())
	}
	// ForceFatal from terminal must fail.
	if err := m.ForceFatal("again"); !errors.Is(err, ErrTerminalState) {
		t.Fatalf("expected ErrTerminalState, got %v", err)
	}
}

func TestConcurrentReadsAreSafe(t *testing.T) {
	m := NewMachine("run-concurrent", "hash-c", 2)
	for _, e := range happyPath() {
		if err := m.Transition(e, ""); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.State()
			_ = m.IsTerminal()
			_ = m.Log()
		}()
	}
	wg.Wait()
}

func TestConcurrentTransitionsAreSafe(t *testing.T) {
	m := NewMachine("run-concurrent-tx", "hash-ct", 2)

	// Drive the happy path from many goroutines; only one transition wins per step,
	// but we still expect to finish in Done because transitions are serialized.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, e := range happyPath() {
				_ = m.Transition(e, "")
			}
		}()
	}
	wg.Wait()

	if m.State() != StateDone {
		t.Fatalf("expected Done after concurrent transitions, got %s", m.State())
	}
}

func TestAuditLogImmutable(t *testing.T) {
	m := NewMachine("run-log", "hash-l", 0)
	if err := m.Transition(EventStartProcess, ""); err != nil {
		t.Fatal(err)
	}

	log := m.Log()
	if len(log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(log))
	}

	// Mutate the returned slice; original log must stay intact.
	log[0].Event = "Tampered"
	original := m.Log()
	if original[0].Event != EventStartProcess {
		t.Fatalf("audit log was mutable from outside")
	}
}
