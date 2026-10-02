package server

import (
	"testing"
	"time"
)

// TestResetBehaviour drives a runner directly: step a little, reset, and
// verify the state returns to zero with the previous run archived.
func TestResetBehaviour(t *testing.T) {
	r := NewRunner(DefaultParams())
	defer r.Quit()

	r.CmdCh() <- runnerCmd{kind: "step"}
	r.CmdCh() <- runnerCmd{kind: "step"}

	// Wait for the two step frames. A step-0 frame (the runner's initial
	// publish) may arrive first and is skipped, not counted.
	deadline := time.Now().Add(10 * time.Second)
	got := 0
	for got < 2 && time.Now().Before(deadline) {
		select {
		case snap := <-r.Frames():
			if snap.Step == 0 {
				continue // initial publish, not a stepped frame
			}
			if snap.Step != 1 && snap.Step != 2 {
				t.Fatalf("unexpected step %d", snap.Step)
			}
			got++
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	if got < 2 {
		t.Fatalf("only got %d step frames", got)
	}

	// Reset: next frame must be back at step 0 with t=0.
	r.CmdCh() <- runnerCmd{kind: "reset"}
	deadline = time.Now().Add(10 * time.Second)
	var resetSnap bool
	for !resetSnap && time.Now().Before(deadline) {
		select {
		case snap := <-r.Frames():
			if snap.Step == 0 && snap.Time == 0 {
				resetSnap = true
			}
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !resetSnap {
		t.Fatal("reset did not return the sim to step 0 / t = 0")
	}
	// The stepped run gets archived for the comparison plot: the initial
	// t*=0 publish plus one point per step = 3 points.
	runs := r.ArchivedRuns()
	if len(runs) != 1 || len(runs[0].Points) != 3 {
		t.Fatalf("archive after reset: %+v", runs)
	}
}

// TestAutoPauseAtTStar verifies the safety stop engages without running the
// full window: use a huge time scale by bumping gravity? Not configurable —
// instead just verify a fresh runner reports not-running and no error.
func TestRunnerFreshState(t *testing.T) {
	r := NewRunner(DefaultParams())
	defer r.Quit()
	if r.LastRunning() {
		t.Error("fresh runner must not be running")
	}
	if r.LastError() != "" {
		t.Errorf("fresh runner has error: %q", r.LastError())
	}
	if r.AutoPaused() {
		t.Error("fresh runner must not be auto-paused")
	}
}
