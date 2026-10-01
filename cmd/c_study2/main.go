package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func main() {
	fmt.Println("=== 2. Taylor-Green defect ===")
	// run a baseline 32x32 Taylor-Green up to t=1.0, printing error every 0.1
	runTGTime(32, false, false, false, 1.0)
	runTGIso(32, false, true, false, 1.0) // no advection
	runTGIso(32, false, false, true, 1.0) // no viscosity
	runTGIso(32, false, false, false, 0.5) // dt halved
	runTGIso(32, false, false, false, 0.25) // dt quartered
	runTGIso(32, true, false, false, 1.0) // van Leer
}

func getCfg(N int) solver.Config {
	L := 2 * math.Pi
	nx, ny := N, N
	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = 1.0
	cfg.Domain.H0 = 1.0
	cfg.Domain.Width = L
	cfg.Domain.Height = L
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Numerical.PoissonTol = 1e-9
	cfg.Numerical.PoissonMaxIter = 50000
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.OpenTop = false
	cfg.Physical.RhoW = 1.0
	cfg.Physical.MuW = 0.01 // nu = 0.01
	return cfg
}

func initTG(sim *solver.Simulation) {
	g := sim.Grid
	f := sim.Fields
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ {
			x := g.Xu[g.IdxU(i, j)]
			y := g.Yu[g.IdxU(i, j)]
			f.U[g.IdxU(i, j)] = math.Sin(x) * math.Cos(y)
		}
	}
	for j := 1; j < g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			x := g.Xv[g.IdxV(i, j)]
			y := g.Yv[g.IdxV(i, j)]
			f.V[g.IdxV(i, j)] = -math.Cos(x) * math.Sin(y)
		}
	}
}

func calcErr(sim *solver.Simulation) (float64, float64, int, int) {
	g := sim.Grid
	f := sim.Fields
	nu := 0.01
	decay := math.Exp(-2.0 * nu * sim.State.Time)
	var maxErr, sumErr float64
	var count int
	var maxI, maxJ int
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ {
			x := g.Xu[g.IdxU(i, j)]
			y := g.Yu[g.IdxU(i, j)]
			exact := math.Sin(x) * math.Cos(y) * decay
			err := math.Abs(f.U[g.IdxU(i, j)] - exact)
			if err > maxErr {
				maxErr = err
				maxI, maxJ = i, j
			}
			sumErr += err
			count++
		}
	}
	return sumErr / float64(count), maxErr, maxI, maxJ
}

func runTGTime(N int, vanLeer bool, noAdv bool, noVisc bool, dtScale float64) {
	cfg := getCfg(N)
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.CFL = 0.4 * dtScale
	if noAdv { cfg.Numerical.CFL = 1000.0 } // hack to skip advection or something?
	// actually we must disable advection in solver
	
	L := 2 * math.Pi
	sim := solver.NewSimulation(cfg, N, N, L, L, false)
	initTG(sim)
	solver.ApplyVelocityBC(sim.Fields, &cfg, sim.Grid)
	
	// Error at t=0
	meanErr, maxErr, i, j := calcErr(sim)
	fmt.Printf("TG %dx%d (t=%.2f): MeanErr=%.2e, MaxErr=%.2e at (i=%d, j=%d)\n", N, N, sim.State.Time, meanErr, maxErr, i, j)
	
	nextT := 0.1
	for sim.State.Time < 1.0 {
		sim.Step(-1)
		if sim.State.Time >= nextT {
			meanErr, maxErr, i, j := calcErr(sim)
			fmt.Printf("TG %dx%d (t=%.2f): MeanErr=%.2e, MaxErr=%.2e at (i=%d, j=%d)\n", N, N, sim.State.Time, meanErr, maxErr, i, j)
			nextT += 0.1
		}
	}
}

func runTGIso(N int, vanLeer bool, noAdv bool, noVisc bool, dtScale float64) {
	cfg := getCfg(N)
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.SkipAdvection = noAdv
	cfg.Numerical.SkipViscosity = noVisc
	cfg.Numerical.CFL = 0.4 * dtScale
	L := 2 * math.Pi
	sim := solver.NewSimulation(cfg, N, N, L, L, false)
	initTG(sim)
	solver.ApplyVelocityBC(sim.Fields, &cfg, sim.Grid)
	for sim.State.Time < 1.0 { sim.Step(-1) }
	meanErr, maxErr, i, j := calcErr(sim)
	fmt.Printf("TG %dx%d (t=1.00, adv=%v, visc=%v, vanLeer=%v, dtScale=%v): MeanErr=%.2e, MaxErr=%.2e at (i=%d, j=%d)\n", N, N, !noAdv, !noVisc, vanLeer, dtScale, meanErr, maxErr, i, j)
}

