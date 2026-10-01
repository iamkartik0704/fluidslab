package main

import (
	"fmt"
	"math"
	"dambreak/internal/solver"
)

func main() {
	fmt.Println("=== Free-Slip Reproducibility Check ===")
	runFS()
}

func runFS() {
	L0 := 0.05715
	H0 := 2.0 * L0
	width := 10.0 * L0
	height := 4.0 * L0

	cellsPerL0 := 16
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
	cfg.Physical.MuW = 1.0e-3
	cfg.Physical.MuA = 1.8e-5
	cfg.Physical.Gravity = 9.81

	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = false
	cfg.Numerical.ClipRedistribute = true
	cfg.Numerical.SplitDivFix = true
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0

	// Open top
	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	tStars := []float64{1.0, 2.0, 3.0}
	tIdx := 0

	for sim.State.Time < (3.01 * math.Sqrt(2.0*L0/9.81)) {
		sim.Step(-1)
		if tIdx < len(tStars) && sim.State.TStar >= tStars[tIdx] {
			fmt.Printf("t* = %.1f, X*(0.5) = %.4f\n", tStars[tIdx], sim.State.FrontXStar)
			tIdx++
		}
	}
}
