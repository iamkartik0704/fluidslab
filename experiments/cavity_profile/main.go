package main

import (
	"fmt"
	"math"

	"dambreak/internal/solver"
)

func main() {
	nx, ny := 65, 65
	L := 1.0
	U0 := 1.0

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L
	cfg.Domain.H0 = L
	cfg.Domain.Width = L
	cfg.Domain.Height = L
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Numerical.PoissonTol = 1e-7
	cfg.Numerical.FreeSlip = false
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.LidVelocity = U0

	sim := solver.NewSimulation(cfg, nx, ny, L, L, true)
	sim.Fields.Alpha = make([]float64, len(sim.Fields.Alpha))
	for i := range sim.Fields.Alpha {
		sim.Fields.Alpha[i] = 1.0
	}

	for sim.State.TStar < 30.0 {
		sim.Step(0)
	}
	
	// Print U profile at x=0.5
	fmt.Printf("\n--- u(y) at x=0.5 (65^2) ---\n")
	ghiaU := solver.Ghia100U()
	i_center := nx / 2
	
	maxErr := 0.0
	maxErrLoc := 0.0
	fmt.Printf("y\tu_sim\tu_ghia\n")
	for y, expected := range ghiaU {
		j := int(math.Round(y * float64(ny-1)))
		if j < 1 {
			j = 1
		}
		if j > ny-2 {
			j = ny - 2
		}
		idx := sim.Grid.IdxFaceX(i_center, j)
		u := sim.Fields.U[idx]
		
		err := math.Abs(u - expected)
		if err > maxErr {
			maxErr = err
			maxErrLoc = y
		}
		fmt.Printf("%.4f\t%.4f\t%.4f\n", y, u, expected)
	}
	fmt.Printf("\nMax error in u(y): %.4f at y=%.4f\n", maxErr, maxErrLoc)
}
