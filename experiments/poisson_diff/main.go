package main

import (
	"fmt"
	"math"

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
	
	for {
		sim.Step(0)
		if sim.State.TStar >= 2.0 {
			fmt.Printf("t*=%f steps=%d poissonIter=%d poissonRes=%.3e maxDiv=%.3e\n",
				sim.State.TStar, sim.State.Step, sim.State.PoissonIter, sim.State.PoissonResidual, sim.State.MaxDiv)
			break
		}
	}
}
