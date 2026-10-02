package main

import (
	"fmt"
	"math"
	"time"

	"dambreak/internal/solver"
)

func main() {
	cellsPerL0 := 16
	L0 := 0.05715
	H0 := 2.0 * L0
	width := 15.0 * L0
	height := 4.0 * L0

	nx := int(math.Round(width / L0 * float64(cellsPerL0)))
	ny := int(math.Round(height / L0 * float64(cellsPerL0)))

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Width = width
	cfg.Domain.Height = height
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny

	cfg.Physical.RhoW = 1000.0
	cfg.Physical.RhoA = 1.0
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.SplitDivFix = true
	cfg.Numerical.ClipRedistribute = true

	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0

	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	initialVol := sim.State.Volume

	start := time.Now()
	for {
		sim.Step(0)
		if sim.State.TStar >= 12.0 || sim.State.FrontXStar >= 14.0 {
			break
		}
	}

	fmt.Printf("\n--- Drift Decomposition (N16_FStrue_VLtrue) ---\n")
	fmt.Printf("WallTime: %v\n", time.Since(start))
	fmt.Printf("Initial Volume: %.6e m^3\n", initialVol)
	fmt.Printf("Total Clipped Mass (redistributed if true): %.6e m^3\n", sim.State.VolClip)
	fmt.Printf("Top Outflow (open BC): %.6e m^3\n", sim.State.TopOutflow)
	fmt.Printf("X-sweep imbalance: %.6e m^3\n", sim.State.VolSweepX)
	fmt.Printf("Y-sweep imbalance: %.6e m^3\n", sim.State.VolSweepY)
	fmt.Printf("Final Drift (Vol(t)-Vol(0)): %.6e m^3 (%.3e%%)\n\n",
		sim.State.VolumeDrift, 100*sim.State.VolumeDrift/initialVol)
}
