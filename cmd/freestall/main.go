package main

import (
	"fmt"
	"math"

	"dambreak/internal/solver"
)

func main() {
	L0 := 0.057
	cellsPerL0 := 16
	width := 8.0 * L0
	height := 4.0 * L0
	nx := int(math.Round(width / L0 * float64(cellsPerL0)))
	ny := int(math.Round(height / L0 * float64(cellsPerL0)))

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = 2.0 * L0
	cfg.Domain.Width = width
	cfg.Domain.Height = height
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.OpenTop = true // true because it's a dam break
	cfg.Numerical.ClipRedistribute = false

	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	fmt.Printf("FreeSlip test: %dx%d, L0=%.3f\n", nx, ny, L0)

	for sim.State.Time < (8.01 * math.Sqrt(2.0*L0/9.81)) {
		err := sim.Step(-1)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}
		if sim.State.Step%10 == 0 {
			fmt.Printf("t* = %.3f, step = %d, maxDiv = %.2e\n", sim.State.TStar, sim.State.Step, sim.State.MaxDiv)
		}
	}
	fmt.Printf("Finished. t* = %.3f, step = %d\n", sim.State.TStar, sim.State.Step)
}
