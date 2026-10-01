package main

import (
	"dambreak/internal/solver"
	"fmt"
)

func main() {
	fmt.Println("=== Alpha Clipping Redistribution Test ===")
	L0 := 0.05715
	H0 := 2.0 * L0
	width := 4.0 * L0
	height := 3.0 * H0

	cellsPerL0 := 16
	nx := int(width / L0 * float64(cellsPerL0))
	ny := int(height / L0 * float64(cellsPerL0))

	// Run with redistribution ON
	fmt.Println("\n--- ClipRedistribute = ON ---")
	runClipTest(nx, ny, width, height, L0, H0, cellsPerL0, true)

	// Run with redistribution OFF
	fmt.Println("\n--- ClipRedistribute = OFF ---")
	runClipTest(nx, ny, width, height, L0, H0, cellsPerL0, false)
}

func runClipTest(nx, ny int, width, height, L0, H0 float64, cellsPerL0 int, redistribute bool) {
	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Width = width
	cfg.Domain.Height = height
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny

	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = false
	cfg.Numerical.SecondOrderAdvect = false
	cfg.Numerical.ClipAlpha = true
	cfg.Numerical.ClipRedistribute = redistribute

	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	for sim.State.TStar < 4.5 {
		sim.Step(-1)
	}

	driftPct := sim.State.VolumeDrift / sim.State.Volume * 100.0
	fmt.Printf("  Drift:        %.4f%%\n", driftPct)
	fmt.Printf("  Clipped Mass: %.4e\n", sim.State.ClippedMass)
	fmt.Printf("  Top Outflow:  %.4e\n", sim.State.TopOutflow)
	fmt.Printf("  Steps:        %d\n", sim.State.Step)
}
