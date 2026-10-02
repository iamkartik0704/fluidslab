package solver

import (
	"testing"
)

func TestFrontInvariant(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Domain.L0 = 1.0
	cfg.Domain.H0 = 1.0
	cfg.Domain.Width = 4.0
	cfg.Domain.Height = 2.0
	cfg.Domain.Nx = 16
	cfg.Domain.Ny = 8

	sim := NewSimulation(cfg, cfg.Domain.Nx, cfg.Domain.Ny, cfg.Domain.Width, cfg.Domain.Height, false)
	sim.InitDamBreak()

	// Check immediately after init
	if sim.State.FrontXStar < 1.0 {
		t.Errorf("Initial FrontXStar = %f, expected >= 1.0", sim.State.FrontXStar)
	}

	// Run 10 steps to see if it drops
	for i := 0; i < 10; i++ {
		err := sim.Step(-1)
		if err != nil {
			t.Fatalf("Step failed: %v", err)
		}
		if sim.State.FrontXStar < 0.99 {
			t.Errorf("Step %d: FrontXStar = %f, dropped below 1.0", i, sim.State.FrontXStar)
		}
	}
}
