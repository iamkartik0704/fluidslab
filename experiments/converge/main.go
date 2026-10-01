package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
	"time"
)

func runConvergence(cellsPerL0 int) {
	L0 := 0.05715
	H0 := 2.0 * L0
	W := 4.0 * L0
	H := 4.0 * L0

	nx := 4 * cellsPerL0
	ny := 4 * cellsPerL0

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Domain.Width = W
	cfg.Domain.Height = H
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.DilatationCorr = true
	cfg.Numerical.ClipAlpha = true
	cfg.Threads = 8

	sim := solver.NewSimulation(cfg, nx, ny, W, H, false)
	sim.InitDamBreak()

	targets := []float64{1.0, 2.0, 3.0, 3.5}
	tIdx := 0

	maxDriftPct := 0.0
	totalIters := 0
	maxIters := 0

	t0 := time.Now()

	for tIdx < len(targets) && sim.State.TStar < 3.55 {
		err := sim.Step(0)
		if err != nil {
			fmt.Printf("  [%d cells/L0] FAILED at step %d: %v\n", cellsPerL0, sim.State.Step, err)
			return
		}

		totalIters += sim.State.PoissonIter
		if sim.State.PoissonIter > maxIters {
			maxIters = sim.State.PoissonIter
		}

		driftPct := math.Abs(100 * sim.State.VolumeDrift / sim.State.Volume)
		if driftPct > maxDriftPct {
			maxDriftPct = driftPct
		}

		if sim.State.TStar >= targets[tIdx] {
			fmt.Printf("  t*=%.1f  X*=%.4f  drift=%.3e%%  maxDiv=%.3e  iters=%d\n",
				targets[tIdx], sim.State.FrontXStar, 100*sim.State.VolumeDrift/sim.State.Volume,
				sim.State.MaxDiv, sim.State.PoissonIter)
			tIdx++
		}
	}

	elapsed := time.Since(t0)
	meanIters := float64(totalIters) / float64(sim.State.Step)

	fmt.Printf("  Summary: %d steps, %.2fs wall, maxDrift=%.4f%%, meanIters=%.1f, maxIters=%d\n\n",
		sim.State.Step, elapsed.Seconds(), maxDriftPct, meanIters, maxIters)
}

func main() {
	fmt.Println("=== Convergence Study: 4L0 x 4L0, H0=2L0 ===")
	fmt.Println()

	for _, cpl := range []int{8, 16, 32} {
		fmt.Printf("--- %d cells per L0 (%dx%d) ---\n", cpl, 4*cpl, 4*cpl)
		runConvergence(cpl)
	}
}
