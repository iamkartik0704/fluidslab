package solver_test

import (
	"dambreak/internal/solver"
	"math"
	"testing"
)

// TestCavityViscousStability verifies that the 65^2 lid-driven cavity at
// CFL=0.4 reaches a steady state. With the old viscous dt formula
// (dtVisc = ViscousCFL * h^2 / nu, no /4), max|du/dt| stays > 300
// indefinitely. The corrected formula (dtVisc = ViscousCFL * h^2 / (4*nu))
// ensures dt < h^2/(4*nu), and max|du/dt| decays below 1.0.
func TestCavityViscousStability(t *testing.T) {
	n := 65
	cfg := solver.DefaultConfig()
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0
	cfg.Physical.Gravity = 0
	cfg.Numerical.CFL = 0.4
	cfg.Numerical.MaxDT = 1.0
	cfg.Numerical.PoissonTol = 1e-4
	cfg.Numerical.SecondOrderAdvect = false
	cfg.Physical.RhoW, cfg.Physical.MuW = 1.0, 0.01
	cfg.Physical.RhoA, cfg.Physical.MuA = 1.0, 0.01
	cfg.Threads = 1

	sim := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)

	lastU := make([]float64, len(sim.Fields.U))
	copy(lastU, sim.Fields.U)
	var maxDuDt float64

	// Run to t=30 (steady state)
	for sim.State.Time < 30.0 {
		sim.Step(-1)
	}
	copy(lastU, sim.Fields.U)

	// Measure max|du/dt| over the next 10 time units [30, 40]
	for sim.State.Time < 40.0 {
		sim.Step(-1)
		dt := sim.State.DT
		for idx := range sim.Fields.U {
			d := math.Abs(sim.Fields.U[idx]-lastU[idx]) / dt
			if d > maxDuDt {
				maxDuDt = d
			}
		}
		copy(lastU, sim.Fields.U)
	}

	// With the old formula, maxDuDt is ~300-500.
	// With the fix, it should be well below 1.0.
	threshold := 1.0
	if maxDuDt > threshold {
		t.Errorf("65^2 cavity at CFL=0.4 not steady: max|du/dt| = %.3e (threshold %.1f)", maxDuDt, threshold)
	} else {
		t.Logf("65^2 cavity at CFL=0.4 steady: max|du/dt| = %.3e", maxDuDt)
	}
}
