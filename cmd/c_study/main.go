package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func main() {
	fmt.Println("=== C.1 Taylor-Green Vortex ===")
	for _, cpl := range []int{16, 32, 64} {
		for _, adv := range []bool{false, true} {
			fmt.Printf("Resolution: %dx%d, VanLeer: %v\n", cpl, cpl, adv)
			runTaylorGreen(cpl, adv)
		}
	}

	fmt.Println("\n=== C.2 Cavity Rerun (17, 33, 65) ===")
	for _, c := range []int{17, 33, 65} {
		runCavity(c)
	}
}

func runTaylorGreen(N int, vanLeer bool) {
	// Taylor-Green vortex: U = sin(x)cos(y)*F(t), V = -cos(x)sin(y)*F(t)
	// Decay F(t) = exp(-2*nu*t)
	// Domain [0, 2*pi] x [0, 2*pi]
	// BUT solver doesn't support periodic boundaries!
	// Wait, without periodic boundaries, TG isn't valid for long.
	// Is it possible to run it with no-slip walls? No, it doesn't satisfy no-slip.
	// Maybe free-slip? U=0 at left/right (x=0, 2pi), V=0 at top/bottom (y=0, 2pi).
	// TG has U=sin(x)cos(y). At x=0 and 2pi, U=0! At y=0 and 2pi, V=-cos(x)sin(y)=0!
	// Yes! Taylor-Green exactly satisfies free-slip solid walls on [0, 2pi]^2 !!

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
	cfg.Numerical.CFL = 0.4 * (16.0 / float64(N))
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.OpenTop = false

	// Reynolds number = 100? Let's say nu = 0.01
	cfg.Physical.RhoW = 1.0
	cfg.Physical.MuW = 0.01
	nu := 0.01

	sim := solver.NewSimulation(cfg, nx, ny, L, L, false)
	g := sim.Grid
	f := sim.Fields

	// Initialize TG
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
	solver.ApplyVelocityBC(f, &cfg, g)

	tMax := 1.0
	steps := 0

	for sim.State.Time < tMax {
		sim.Step(-1)
		steps++
	}

	decay := math.Exp(-2.0 * nu * sim.State.Time)

	var maxErr, sumErr float64
	var count int

	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ {
			x := g.Xu[g.IdxU(i, j)]
			y := g.Yu[g.IdxU(i, j)]
			exact := math.Sin(x) * math.Cos(y) * decay
			err := math.Abs(f.U[g.IdxU(i, j)] - exact)
			if err > maxErr {
				maxErr = err
			}
			sumErr += err
			count++
		}
	}
	meanErr := sumErr / float64(count)

	fmt.Printf("TG %dx%d (t=%.2f, steps=%d): MeanErr=%.2e, MaxErr=%.2e\n", N, N, sim.State.Time, steps, meanErr, maxErr)
}

func runCavity(N int) {
	L := 1.0
	nx, ny := N, N

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L
	cfg.Domain.H0 = L
	cfg.Domain.Width = L
	cfg.Domain.Height = L
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny

	cfg.Numerical.PoissonTol = 1e-9
	cfg.Numerical.PoissonMaxIter = 50000
	cfg.Numerical.CFL = 0.4 * (16.0 / float64(N))
	cfg.Numerical.FreeSlip = false
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0

	cfg.Physical.RhoW = 1.0
	cfg.Physical.MuW = 0.01 // Re = 100

	sim := solver.NewSimulation(cfg, nx, ny, L, L, false)
	g := sim.Grid
	f := sim.Fields

	solver.ApplyVelocityBC(f, &cfg, g)

	tMax := 15.0
	for sim.State.Time < tMax {
		sim.Step(-1)
	}

	// Interpolate profiles
	uProf := make([]float64, g.Ny)
	for j := 1; j <= g.Ny; j++ {
		midI := g.Nx / 2
		uProf[j-1] = 0.5 * (f.U[g.IdxU(midI, j)] + f.U[g.IdxU(midI+1, j)])
	}
	vProf := make([]float64, g.Nx)
	for i := 1; i <= g.Nx; i++ {
		midJ := g.Ny / 2
		vProf[i-1] = 0.5 * (f.V[g.IdxV(i, midJ)] + f.V[g.IdxV(i, midJ+1)])
	}

	// Ghia Re=100
	ghiaY := []float64{0, 0.0547, 0.0625, 0.0703, 0.1016, 0.1719, 0.2813, 0.4531, 0.5, 0.6172, 0.7344, 0.8516, 0.9531, 0.9609, 0.9688, 0.9766, 1}
	ghiaU := []float64{0, -0.03717, -0.04192, -0.04775, -0.06434, -0.10150, -0.15662, -0.21090, -0.20581, -0.13641, 0.00332, 0.23151, 0.68717, 0.73722, 0.78871, 0.84123, 1}
	ghiaX := []float64{0, 0.0625, 0.0703, 0.0781, 0.0938, 0.1563, 0.2266, 0.2344, 0.5, 0.8047, 0.8594, 0.9063, 0.9453, 0.9531, 0.9609, 0.9688, 1}
	ghiaV := []float64{0, 0.09233, 0.10091, 0.10890, 0.12317, 0.16077, 0.17507, 0.17527, 0.05454, -0.24533, -0.22445, -0.16914, -0.10313, -0.08864, -0.07391, -0.05906, 0}

	interp := func(y float64, vals []float64, yGrid []float64) float64 {
		for k := 1; k < len(yGrid); k++ {
			if y <= yGrid[k] {
				t := (y - yGrid[k-1]) / (yGrid[k] - yGrid[k-1])
				return vals[k-1] + t*(vals[k]-vals[k-1])
			}
		}
		return vals[len(vals)-1]
	}

	var maxErrU, maxErrV float64
	var errUIdx, errVIdx int

	// Custom interp to avoid slice bounds issues
	yc := make([]float64, g.Ny)
	for j := 1; j <= g.Ny; j++ {
		yc[j-1] = g.Yc[g.IdxCC(1, j)]
	}
	xc := make([]float64, g.Nx)
	for i := 1; i <= g.Nx; i++ {
		xc[i-1] = g.Xc[g.IdxCC(i, 1)]
	}

	for k, gy := range ghiaY {
		u := interp(gy, uProf, yc)
		if err := math.Abs(u - ghiaU[k]); err > maxErrU {
			maxErrU = err
			errUIdx = k
		}
	}
	for k, gx := range ghiaX {
		v := interp(gx, vProf, xc)
		if err := math.Abs(v - ghiaV[k]); err > maxErrV {
			maxErrV = err
			errVIdx = k
		}
	}

	fmt.Printf("Cavity %dx%d: Max U Err=%.4f (Ghia sample %d), Max V Err=%.4f (Ghia sample %d)\n", N, N, maxErrU, errUIdx, maxErrV, errVIdx)
}
