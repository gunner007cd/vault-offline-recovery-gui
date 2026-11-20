package main

import (
	"testing"
	"time"
)

func TestMascotSixStatesAndRestoreSafety(t *testing.T) {
	var m mascotTracker
	if m.phase != mascotIdle {
		t.Fatal("not idle at launch")
	}
	m.begin("scan")
	if m.phase != mascotScanning {
		t.Fatal("scan not shown")
	}
	m.begin("inspect")
	if m.phase != mascotScanning {
		t.Fatal("inspect not shown as scan")
	}
	m.begin("restore")
	if m.phase != mascotPreparing {
		t.Fatal("restore beginning not shown")
	}
	m.elapsed(3*time.Minute + 29*time.Second)
	if m.phase != mascotPreparing {
		t.Fatal("long-wait image shown too soon")
	}
	m.elapsed(3*time.Minute + 30*time.Second)
	if m.phase != mascotWaiting {
		t.Fatal("3:30 impatient image missing")
	}
	m.extractionStarted()
	if m.phase != mascotExtracting {
		t.Fatal("no transition on engine message")
	}
	m.elapsed(20 * time.Minute)
	if m.phase != mascotExtracting {
		t.Fatal("elapsed must not override actual extraction")
	}
	m.complete("restore", false)
	if m.phase != mascotFailed {
		t.Fatal("engine failure must show failure, not success or waiting")
	}
	m.cancelled()
	if m.phase != mascotCancelled {
		t.Fatal("operator cancellation must show cancellation, not failure")
	}
	m.begin("scan")
	m.fail()
	if m.phase != mascotFailed {
		t.Fatal("failed scans also show the failure mascot")
	}
	m.begin("restore")
	m.extractionStarted()
	m.complete("restore", true)
	if m.phase != mascotSuccess {
		t.Fatal("no confirmed success image")
	}
	m.begin("scan")
	if m.phase != mascotScanning {
		t.Fatal("new scan should clear prior success")
	}
	m.reset()
	if m.phase != mascotIdle {
		t.Fatal("reset not idle")
	}
}

func TestExtractorMilestoneOnlyFromRealLineAndOnlyOnce(t *testing.T) {
	var p extractorMilestone
	for _, chunk := range []string{"warning: read index 42\n", "ex", "tracting Flash Drive (folder, ", "deduplicated) to C:/out", "\r\n"} {
		hit := p.feed(chunk)
		if chunk != "\r\n" && hit {
			t.Fatalf("premature detection at %q", chunk)
		}
		if chunk == "\r\n" && !hit {
			t.Fatal("failed to recognize true extracting line")
		}
	}
	if p.feed("extracting second item\n") {
		t.Fatal("duplicate milestone")
	}
	var q extractorMilestone
	if q.feed("not extracting anything\n") {
		t.Fatal("false positive")
	}
}

func TestLockedBackupWarningIsNotRecoveryFailure(t *testing.T) {
	var m mascotTracker
	m.begin("scan")
	locked, readable := m.scanned([]Point{{Locked: true, LockedBy: "dedup"}, {Locked: true, LockedBy: "age"}})
	if locked != 2 || readable != 0 || m.phase != mascotLocked {
		t.Fatalf("all locked must show warning, got %d locked %d readable phase %d", locked, readable, m.phase)
	}
	m.begin("scan")
	locked, readable = m.scanned([]Point{{Locked: true, LockedBy: "dedup"}, {Locked: false}})
	if locked != 1 || readable != 1 || m.phase != mascotScanning {
		t.Fatalf("mixed results should show thinking, not failure/idle: phase %d", m.phase)
	}
	m.selectedLockedPoint(true)
	if m.phase != mascotLocked {
		t.Fatal("selecting locked restore point must warn")
	}
	m.selectedLockedPoint(false)
	if m.phase != mascotScanning {
		t.Fatal("selecting readable point should return to thinking")
	}
	m.begin("scan")
	m.scanned([]Point{{Locked: false}})
	if m.phase != mascotScanning {
		t.Fatal("valid key should restore thinking expression")
	}
	m.begin("scan")
	m.fail()
	if m.phase != mascotFailed {
		t.Fatal("real Vault error remains a failure")
	}
	m.begin("restore")
	m.complete("restore", true)
	if m.phase != mascotSuccess {
		t.Fatal("success reporting must be unaffected")
	}
	m.begin("scan")
	m.scanned(nil)
	if m.phase != mascotIdle {
		t.Fatal("empty results are not a locked warning")
	}
}

// Regression: the tested eight-state GUI left the thinking expression in place
// after successful discovery. The nine-state locked warning must not reset a
// readable scan to the opening salute.
func TestSuccessfulReadableScanKeepsThinkingMascot(t *testing.T) {
	var m mascotTracker
	m.begin("scan")
	locked, readable := m.scanned([]Point{{Locked: false}, {Locked: false}})
	if locked != 0 || readable != 2 || m.phase != mascotScanning {
		t.Fatalf("readable scan should remain on thinking mascot, got phase=%d (locked=%d readable=%d)", m.phase, locked, readable)
	}
	m.selectedLockedPoint(true)
	if m.phase != mascotLocked {
		t.Fatal("locked selection should display warning")
	}
	m.selectedLockedPoint(false)
	if m.phase != mascotScanning {
		t.Fatalf("choosing readable point should return to thinking, got %d", m.phase)
	}
	m.begin("scan")
	locked, readable = m.scanned([]Point{{Locked: true}, {Locked: false}})
	if locked != 1 || readable != 1 || m.phase != mascotScanning {
		t.Fatalf("mixed scan with readable points should stay thinking until locked point selected, got %d", m.phase)
	}
	m.begin("scan")
	locked, readable = m.scanned([]Point{{Locked: true}})
	if locked != 1 || readable != 0 || m.phase != mascotLocked {
		t.Fatal("all locked still needs amber warning")
	}
	m.reset()
	if m.phase != mascotIdle {
		t.Fatal("changing key/connection should still reset to launch pose")
	}
}
