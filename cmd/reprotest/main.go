package main

import (
	"dambreak/internal/solver"
	"fmt"
)

func runDamBreak(n int, so, fs bool, dtDiv int, vofSub int, serial bool) {
	cfg := solver.DefaultConfig()
	cfg.Numerical.SecondOrderAdvect = so
	cfg.Numerical.FreeSlip = fs
	cfg.Numerical.SubstepVOF = vofSub
	if dtDiv > 1 {
		cfg.Numerical.MaxCFLFrac = 0.5 / float64(dtDiv)
	}
	cfg.Numerical.MaxDT = 0.05
	if serial {
		cfg.Threads = 1
	} else {
		cfg.Threads = 8
	}

	L0 := 2.25 * 0.0254
	nx := n * 15
	ny := n * 4
	dx := L0 / float64(n)

	sim := solver.NewSimulation(cfg, nx, ny, float64(nx)*dx, float64(ny)*dx, false)
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
	sim.InitDamBreak()
	// solver.ApplyAlphaBC(sim.Grid, sim.Fields)

	targets := []float64{3.0, 5.0, 7.0, 10.0, 14.0}
	idx := 0
	
	fmt.Printf("Results: ")
	for {
		if err := sim.Step(-1); err != nil {
			break
		}
		z := sim.State.FrontXStar
		for idx < len(targets) && z >= targets[idx] {
			fmt.Printf("Z=%.1f->T=%.6f  ", targets[idx], sim.State.TStar)
			idx++
		}
		if idx >= len(targets) || sim.State.TStar > 15.0 {
			break
		}
	}
	fmt.Println()
}

func main() {
	fmt.Println("--- N=16 Native ---")
	runDamBreak(16, true, true, 1, 1, false)
	runDamBreak(16, true, true, 1, 1, false)
	
	fmt.Println("--- N=16 Demo Preset ---")
	runDamBreak(16, true, true, 1, 4, false)
	runDamBreak(16, true, true, 1, 4, false)

	fmt.Println("--- N=32 Parallel vs Serial ---")
	runDamBreak(32, true, true, 1, 1, false)
	runDamBreak(32, true, true, 1, 1, true)
}
