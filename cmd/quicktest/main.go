package main

import (
	"dambreak/internal/solver"
	"fmt"
)

func main() {
	cfg := solver.DefaultConfig()
	cfg.Numerical.SecondOrderAdvect = false
	cfg.Numerical.FreeSlip = true

	a_in := 2.25
	a := a_in * 0.0254
	L0 := a
	n := 16
	nx := n * 15
	ny := n * 4
	dx := L0 / float64(n)

	sim := solver.NewSimulation(cfg, nx, ny, float64(nx)*dx, float64(ny)*dx, false)
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
	sim.InitDamBreak()
	solver.ApplyAlphaBC(sim.Grid, sim.Fields)

	for sim.State.TStar < 2.0 {
		sim.Step(-1)
	}

	fmt.Printf("X* at t* = 2.0: %f\n", sim.State.FrontXStar)
}
