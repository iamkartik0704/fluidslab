package main

import (
	"fmt"
	"dambreak/internal/solver"
)

func main() {
	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = 0.05715
	cfg.Domain.H0 = 0.11430
	cfg.Domain.Width = 15.0 * 0.05715
	cfg.Domain.Height = 4.0 * 0.05715
	cfg.Domain.Nx = 15 * 16
	cfg.Domain.Ny = 4 * 16

	cfg.Physical.RhoW = 1000.0
	cfg.Physical.RhoA = 1.0
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.SplitDivFix = true
	cfg.Numerical.ClipRedistribute = true
	
	// dtScale = 0.8 (0.8 * 0.25 = 0.2)
	cfg.Numerical.CFL *= 0.8
	cfg.Numerical.ViscousCFL *= 0.8
	cfg.Numerical.GravityCFL *= 0.8
	cfg.Numerical.MaxDT *= 0.8

	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0 
	
	sim := solver.NewSimulation(cfg, cfg.Domain.Nx, cfg.Domain.Ny, cfg.Domain.Width, cfg.Domain.Height, false)
	sim.InitDamBreak()
	
	Zs := []float64{3, 5, 7, 10, 14}
	nextZIdx := 0
	
	for {
		sim.Step(0)
		if sim.State.FrontXStar >= Zs[nextZIdx] {
			fmt.Printf("Z=%f, T=%f\n", sim.State.FrontXStar, sim.State.TStar)
			nextZIdx++
			if nextZIdx >= len(Zs) {
				break
			}
		}
	}
}
