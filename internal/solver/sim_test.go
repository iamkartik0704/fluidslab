package solver

import (
	"math"
	"testing"
)

func TestFrontInterpolation(t *testing.T) {
	cfg := DefaultConfig()
	nx := 10
	ny := 5
	sim := NewSimulation(cfg, nx, ny, 1.0, 0.5, false)
	g := sim.Grid
	f := sim.Fields

	// Set up an analytic alpha profile in the lowest row (j=1)
	// dx = 1.0 / 10 = 0.1
	// Cell centers Xc are at 0.05, 0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75, 0.85, 0.95
	// Let's set cell i=4 (Xc = 0.35) to alpha=0.8
	// Let's set cell i=5 (Xc = 0.45) to alpha=0.2
	// Target alpha=0.5 crossing is exactly halfway between them: X = 0.40

	for i := 1; i <= g.Nx; i++ {
		f.Alpha[g.idxCC(i, 1)] = 0.0 // default
	}
	f.Alpha[g.idxCC(1, 1)] = 1.0
	f.Alpha[g.idxCC(2, 1)] = 1.0
	f.Alpha[g.idxCC(3, 1)] = 1.0
	f.Alpha[g.idxCC(4, 1)] = 0.8
	f.Alpha[g.idxCC(5, 1)] = 0.2

	sim.UpdateDiagnostics()

	expectedX := 0.40
	if math.Abs(sim.State.FrontX-expectedX) > 1e-6 {
		t.Errorf("Expected FrontX %.4f, got %.4f", expectedX, sim.State.FrontX)
	}

	// Test edge case: exact drop-off
	f.Alpha[g.idxCC(4, 1)] = 1.0
	f.Alpha[g.idxCC(5, 1)] = 0.0
	sim.UpdateDiagnostics()
	expectedX = 0.40
	if math.Abs(sim.State.FrontX-expectedX) > 1e-6 {
		t.Errorf("Expected FrontX %.4f, got %.4f", expectedX, sim.State.FrontX)
	}

	// Test case where no cell > 0.5
	for i := 1; i <= g.Nx; i++ {
		f.Alpha[g.idxCC(i, 1)] = 0.0
	}
	sim.UpdateDiagnostics()
	if sim.State.FrontX != 0.0 {
		t.Errorf("Expected FrontX 0.0 when dry, got %.4f", sim.State.FrontX)
	}
}
