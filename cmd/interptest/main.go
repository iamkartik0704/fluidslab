package main

import (
	"dambreak/internal/solver"
	"fmt"
)

func main() {
	cfg := solver.DefaultConfig()
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.MaxDT = 0.05
	cfg.Threads = 1

	L0 := 2.25 * 0.0254
	n := 16
	nx := n * 15
	ny := n * 4
	dx := L0 / float64(n)

	sim := solver.NewSimulation(cfg, nx, ny, float64(nx)*dx, float64(ny)*dx, false)
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
	sim.InitDamBreak()
	// solver.ApplyAlphaBC(sim.Grid, sim.Fields)

	var zs []float64
	var ts []float64
	for sim.State.TStar < 12.0 {
		sim.Step(-1)
		zs = append(zs, sim.State.FrontXStar)
		ts = append(ts, sim.State.TStar)
		if sim.State.FrontXStar > 14.5 {
			break
		}
	}

	interp := func(z float64) float64 {
		for i := 1; i < len(zs); i++ {
			if zs[i-1] <= z && zs[i] >= z {
				f := (z - zs[i-1]) / (zs[i] - zs[i-1])
				if zs[i] == zs[i-1] {
					f = 0
				}
				return ts[i-1] + f*(ts[i]-ts[i-1])
			}
		}
		return 0
	}

	t144 := interp(1.44)
	t14 := interp(14.0)
	t10 := interp(10.0)

	fmt.Printf("Interpolated T(Z=1.44) = %f\n", t144)
	fmt.Printf("Interpolated T(Z=10.0) = %f\n", t10)
	fmt.Printf("Interpolated T(Z=14.0) = %f\n", t14)
	fmt.Printf("Aligned T(Z=14.0) = %f\n", t14 - t144 + 1.25)
	
	fmt.Printf("Late speed [10, 14]: %f\n", 4.0 / (t14 - t10))
}
