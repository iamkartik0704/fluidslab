package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func runSim(label string, tol float64, dilCorr bool, splitFix bool) {
	nx := 64
	ny := 96

	cfg := solver.DefaultConfig()
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Numerical.PoissonTol = tol
	cfg.Numerical.DilatationCorr = dilCorr
	cfg.Numerical.SplitDivFix = splitFix
	cfg.Numerical.ClipAlpha = true
	cfg.Threads = 8

	sim := solver.NewSimulation(cfg, nx, ny, 4.0*cfg.Domain.L0, 3.0*cfg.Domain.H0, false)
	sim.InitDamBreak()

	initVol := sim.State.Volume // physical volume (m^2 in 2D)
	cellArea := sim.Grid.Dx * sim.Grid.Dy

	maxDriftPct := 0.0

	for sim.State.TStar < 4.5 {
		err := sim.Step(0)
		if err != nil {
			fmt.Printf("[%s] FAILED at step %d: %v\n", label, sim.State.Step, err)
			return
		}
		driftPct := math.Abs(100 * sim.State.VolumeDrift / initVol)
		if driftPct > maxDriftPct {
			maxDriftPct = driftPct
		}
	}

	finalDriftPct := 100 * sim.State.VolumeDrift / initVol
	fmt.Printf("--- %s ---\n", label)
	fmt.Printf("  Steps:       %d\n", sim.State.Step)
	fmt.Printf("  Max Drift:   %.4f%%  (%.3f cells)\n", maxDriftPct, math.Abs(sim.State.VolumeDrift)/cellArea)
	fmt.Printf("  Final Drift: %+.4f%%  (%+.3f cells)\n", finalDriftPct, sim.State.VolumeDrift/cellArea)
	fmt.Printf("  VolSweepX:   %+.4f cells\n", sim.State.VolSweepX)
	fmt.Printf("  VolSweepY:   %+.4f cells\n", sim.State.VolSweepY)
	fmt.Printf("  VolClip:     %+.4f cells\n", sim.State.VolClip)
	fmt.Printf("  MaxDiv:      %.3e (max over all interior cells, units: 1/s)\n", sim.State.MaxDiv)
	fmt.Println()
}

func main() {
	L0 := 0.05715
	H0 := 2.0 * L0
	dx := (4.0 * L0) / 64
	dy := (3.0 * H0) / 96
	cellArea := dx * dy
	initCells := (L0 * H0) / cellArea
	fmt.Printf("=== Volume Drift Diagnosis ===\n")
	fmt.Printf("Grid: 64x96 (16 cells per L0)\n")
	fmt.Printf("L0 = %.5f m,  H0 = %.5f m\n", L0, H0)
	fmt.Printf("dx = %.6f m, dy = %.6f m, cell area = %.4e m^2\n", dx, dy, cellArea)
	fmt.Printf("Initial column volume = L0*H0 = %.4e m^2 = %.2f cells\n\n", L0*H0, initCells)

	// (a) tol 1e-6 vs 1e-9, dilatation ON
	runSim("tol=1e-6, dilCorr=ON", 1e-6, true, false)
	runSim("tol=1e-9, dilCorr=ON", 1e-9, true, false)
	// (b) dilatation OFF
	runSim("tol=1e-6, dilCorr=OFF", 1e-6, false, false)
	runSim("tol=1e-9, dilCorr=OFF", 1e-9, false, false)
	// (c) SplitDivFix
	runSim("tol=1e-6, dilCorr=ON, splitFix=ON", 1e-6, true, true)
}
