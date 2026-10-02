package main

import (
	"dambreak/internal/solver"
	"fmt"
)

func runSim(n int, freeSlip, vanLeer bool, dtScale float64) {
	cfg := solver.DefaultConfig()
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.MaxDT = 1.0e-3 // THIS IS WHAT FINALPASS USES

	a_in := 2.25
	a := a_in * 0.0254
	L0 := a
	nx := n * 15
	ny := n * 4
	dx := L0 / float64(n)

	sim := solver.NewSimulation(cfg, nx, ny, float64(nx)*dx, float64(ny)*dx, false)
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
	sim.InitDamBreak()
	solver.ApplyAlphaBC(sim.Grid, sim.Fields)
	
	targets := []float64{5.0, 7.0, 10.0, 14.0}
	idxTarget := 0
	anchorT := 0.0

	for {
		if err := sim.Step(-1); err != nil {
			break
		}
		zNow := sim.State.FrontXStar
		if anchorT == 0.0 && zNow >= 1.44 {
			anchorT = sim.State.TStar
		}
		for idxTarget < len(targets) && zNow >= targets[idxTarget] {
			fmt.Printf("Z=%.0f: T_raw=%.3f T_aligned=%.3f\n", targets[idxTarget], sim.State.TStar, sim.State.TStar-anchorT+1.25)
			idxTarget++
		}
		if idxTarget >= len(targets) || zNow > targets[len(targets)-1]+0.5 {
			break
		}
	}
}

func main() {
	runSim(16, true, true, 1.0)
}
