package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func main() {
	runTest("Baseline", false, false, 8.0, true, false, false, false)
	runTest("(a) ClipRedistribute ON", true, false, 8.0, true, false, false, false)
	runTest("(b) ForceSerialPoisson ON", false, true, 8.0, true, false, false, false)
	runTest("(c) Domain 10L0", false, false, 10.0, true, false, false, false)
	runTest("(d) Closed Top", false, false, 8.0, false, false, false, false)
	runTest("(e) Old CF bug", false, false, 8.0, true, true, false, false)
	runTest("(f) FreeSlip walls", false, false, 8.0, true, false, true, false)
	runTest("(g) Density Interpolation Bug", false, false, 8.0, true, false, false, true)
}

func runTest(name string, clipRedistribute, forceSerial bool, widthMulti float64, openTop bool, oldCF bool, freeSlip bool, densityBug bool) {
	L0 := 0.05715
	H0 := 2.0 * L0
	width := widthMulti * L0
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
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.SecondOrderAdvect = false
	cfg.Numerical.OpenTop = openTop

	
	// Turn off clipping redistribution
	cfg.Numerical.ClipRedistribute = clipRedistribute
	cfg.Numerical.OldCFBug = oldCF
	cfg.Numerical.DensityBug = densityBug
	
	// Control Serial Poisson
	solver.ForceSerialPoisson = forceSerial

	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	tStars := []float64{1.0, 2.0, 3.0, 4.0}
	tIdx := 0

	fmt.Printf("--- %s ---\n", name)
	for sim.State.Time < (4.01 * math.Sqrt(2.0*L0/9.81)) {
		err := sim.Step(-1)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}
		if tIdx < len(tStars) && sim.State.TStar >= tStars[tIdx] {
			fmt.Printf("t* = %.1f, X*(0.5) = %.4f\n", tStars[tIdx], sim.State.FrontXStar)
			tIdx++
		}
	}
}
