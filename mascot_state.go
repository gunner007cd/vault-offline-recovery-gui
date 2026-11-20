package main

import (
	"strings"
	"time"
)

// mascotPhase is a *presentation* state, not a recovery engine progress counter.
// In particular, waiting five minutes never means a particular fraction is done.
type mascotPhase uint8

const (
	mascotIdle mascotPhase = iota
	mascotScanning
	mascotPreparing
	mascotWaiting
	mascotExtracting
	mascotSuccess
	mascotFailed
	mascotCancelled
	mascotLocked
)

const mascotLongWait = 3*time.Minute + 30*time.Second

type mascotTracker struct{ phase mascotPhase }

func (m *mascotTracker) begin(mode string) {
	switch mode {
	case "scan", "inspect":
		m.phase = mascotScanning
	case "restore":
		m.phase = mascotPreparing
	case "check":
		m.phase = mascotIdle
	}
}

func (m *mascotTracker) elapsed(d time.Duration) {
	if m.phase == mascotPreparing && d >= mascotLongWait {
		m.phase = mascotWaiting
	}
}

func (m *mascotTracker) extractionStarted() {
	if m.phase == mascotPreparing || m.phase == mascotWaiting {
		m.phase = mascotExtracting
	}
}

// A successful restore requires both a zero exit code AND a valid Vault report,
// as checked by restore(). The caller must pass true only after that returns nil.
func (m *mascotTracker) complete(mode string, confirmed bool) {
	if mode == "restore" && confirmed {
		m.phase = mascotSuccess
	} else if !confirmed {
		m.phase = mascotFailed
	}
}

// Failure means the executable or its report did not confirm success.
// Cancellation means the operator cancelled the in-flight command.
// Neither is interchangeable with the verified-success state.
func (m *mascotTracker) fail()      { m.phase = mascotFailed }
func (m *mascotTracker) cancelled() { m.phase = mascotCancelled }

// scanned shows the amber warning when every discovered point is locked.
// Otherwise preserve the tested eight-state UI behavior: a successful scan
// with readable points keeps the thinking/ready-to-browse expression.
// A valid listing of encrypted backup metadata is not an executable failure.
func (m *mascotTracker) scanned(points []Point) (locked int, readable int) {
	for _, p := range points {
		if p.Locked {
			locked++
		} else {
			readable++
		}
	}
	switch {
	case locked > 0 && readable == 0:
		m.phase = mascotLocked
	case readable > 0:
		m.phase = mascotScanning
	default:
		m.phase = mascotIdle // Empty repository: nothing available to browse.
	}
	return
}

// selectedLockedPoint shows a warning for a selected inaccessible backup even
// when other backups on the same storage remain readable. Selecting a readable
// point returns to the thinking/ready-to-browse expression, not launch idle.
func (m *mascotTracker) selectedLockedPoint(isLocked bool) {
	if isLocked {
		m.phase = mascotLocked
	} else {
		m.phase = mascotScanning
	}
}

func (m *mascotTracker) reset() { m.phase = mascotIdle }

// extractorMilestone observes only the original CLI's stderr lines. It does
// not scan recovered files, guess a percentage or modify engine behavior.
// Writers may deliver a single CLI line in several chunks.
type extractorMilestone struct {
	pending string
	seen    bool
}

func (m *extractorMilestone) feed(chunk string) bool {
	if m.seen {
		return false
	}
	for _, c := range chunk {
		if c == '\n' || c == '\r' {
			line := strings.TrimSpace(m.pending)
			m.pending = ""
			if strings.HasPrefix(line, "extracting ") {
				m.seen = true
				return true
			}
			continue
		}
		if len(m.pending) < 4096 {
			m.pending += string(c)
		}
	}
	return false
}
