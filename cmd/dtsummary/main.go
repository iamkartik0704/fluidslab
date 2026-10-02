package main

import (
	"fmt"
	"math"
	"dambreak/internal/solver"
)

func dumpDT(name string, cfg solver.Config, sim *solver.Simulation) {
	hmin := math.Min(sim.Grid.Dx, sim.Grid.Dy)
	umax := sim.Fields.MaxAbsVel(sim.Grid)
	nuMax := cfg.Physical.MuW / cfg.Physical.RhoW

	cflAdv := cfg.Numerical.CFL
	cflVisc := cfg.Numerical.ViscousCFL
	cflGrav := cfg.Numerical.GravityCFL
	maxDT := cfg.Numerical.MaxDT

	if cfg.Numerical.MaxCFLFrac > 0.0 {
		cflAdv *= cfg.Numerical.MaxCFLFrac
		cflVisc *= cfg.Numerical.MaxCFLFrac
		cflGrav *= cfg.Numerical.MaxCFLFrac
		maxDT *= cfg.Numerical.MaxCFLFrac
	}

	dtAdv := maxDT
	if umax > 0 { dtAdv = cflAdv * hmin / umax }
	
	dtVisc := maxDT
	if nuMax > 0 { dtVisc = cflVisc * hmin * hmin / (4.0 * nuMax) }
	
	fmt.Printf("%s | h=%.6f umax=%.6f nuMax=%.6f | dtAdv=%.6f dtVisc=%.6f chosenDt=%.6f\n", name, hmin, umax, nuMax, dtAdv, dtVisc, sim.State.DT)
}

func runCavity(n int, cfl float64) {
	cfg := solver.DefaultConfig()
	cfg.Domain.Width = 1.0
	cfg.Domain.Height = 1.0
	cfg.Domain.Nx = n
	cfg.Domain.Ny = n
	cfg.Domain.L0 = 1.0
	cfg.Domain.H0 = 1.0
	cfg.Physical.RhoW = 1.0
	cfg.Physical.MuW = 0.01
	cfg.Numerical.CFL = cfl
	cfg.Numerical.ViscousCFL = cfl
	cfg.Numerical.MaxDT = 0.1
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0

	sim := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)
	sim.Step(-1)
	dumpDT(fmt.Sprintf("Cavity N=%d CFL=%v Step=1  ", n, cfl), cfg, sim)
	for sim.State.Step < 100 { sim.Step(-1) }
	dumpDT(fmt.Sprintf("Cavity N=%d CFL=%v Step=End", n, cfl), cfg, sim)
}

func runDam(n int, dtScale float64) {
	cfg := solver.DefaultConfig()
	L0 := 0.05715
	H0 := 2.0 * L0
	width := 15.0 * L0
	height := 4.0 * L0
	nx := n * 15
	ny := n * 4
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Width = width
	cfg.Domain.Height = height
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = true
	if dtScale < 1.0 {
		cfg.Numerical.MaxCFLFrac = dtScale
	}
	// Note: cmd/finalbench used MaxDT=0.05
	cfg.Numerical.MaxDT = 0.05
	
	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()
	sim.Step(-1)
	dumpDT(fmt.Sprintf("DamBreak N=%d dtScale=%v Step=1  ", n, dtScale), cfg, sim)
	for sim.State.FrontXStar < 14.0 && sim.State.TStar < 12.0 { sim.Step(-1) }
	dumpDT(fmt.Sprintf("DamBreak N=%d dtScale=%v Step=End", n, dtScale), cfg, sim)
}

func main() {
	for _, n := range []int{17, 33, 65, 129} {
		for _, cfl := range []float64{0.4, 0.2} {
			runCavity(n, cfl)
		}
	}
	runDam(16, 1.0)
	runDam(32, 0.25)
}
