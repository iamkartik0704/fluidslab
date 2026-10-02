package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func runDecayingVortex(N int, secondOrder bool) (float64, float64) {
	L := 2.0 * math.Pi
	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L
	cfg.Domain.H0 = L
	cfg.Domain.Nx = N
	cfg.Domain.Ny = N
	cfg.Domain.Width = L
	cfg.Domain.Height = L
	cfg.Numerical.FreeSlip = true

	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.MaxDT = 100.0 // let CFL govern
	cfg.Numerical.CFL = 0.5
	cfg.Numerical.SecondOrderAdvect = secondOrder
	cfg.Physical.Gravity = 0.0
	cfg.Physical.RhoW = 1.0
	cfg.Physical.RhoA = 1.0
	cfg.Physical.MuW = 0.01 // nu = 0.01
	cfg.Physical.MuA = 0.01
	cfg.Threads = 8

	sim := solver.NewSimulation(cfg, N, N, L, L, false)

	nu := cfg.Physical.MuW / cfg.Physical.RhoW

	for j := 0; j <= N+1; j++ {
		for i := 0; i <= N+1; i++ {
			x := float64(i-1) * sim.Grid.Dx
			y := (float64(j) - 0.5) * sim.Grid.Dy
			sim.Fields.U[sim.Grid.IdxU(i, j)] = math.Sin(x) * math.Cos(y)
		}
	}
	for j := 0; j <= N+1; j++ {
		for i := 0; i <= N+1; i++ {
			x := (float64(i) - 0.5) * sim.Grid.Dx
			y := float64(j-1) * sim.Grid.Dy
			sim.Fields.V[sim.Grid.IdxV(i, j)] = -math.Cos(x) * math.Sin(y)
		}
	}
	for i := range sim.Fields.Alpha {
		sim.Fields.Alpha[i] = 1.0
	}

	for sim.State.Time < 1.0 {
		err := sim.Step(-1)
		if err != nil {
			panic(err)
		}
	}

	maxErr := 0.0
	meanErr := 0.0
	count := 0

	exclude := 3
	for j := 1 + exclude; j <= N-exclude; j++ {
		for i := 1 + exclude; i <= N-1-exclude; i++ { // interior U
			x := float64(i) * sim.Grid.Dx
			y := (float64(j) - 0.5) * sim.Grid.Dy
			exact := math.Sin(x) * math.Cos(y) * math.Exp(-2.0*nu*sim.State.Time)
			err := math.Abs(sim.Fields.U[sim.Grid.IdxU(i+1, j)] - exact) // Wait, idxU(i, j) for i=1..N+1. U(i) is at x = (i-1)*dx. So interior is i=2..N. Let me just use i=1..N-1 and index is i+1. Then x = i*dx.
			if err > maxErr {
				maxErr = err
			}
			meanErr += err
			count++
		}
	}
	return meanErr / float64(count), maxErr
}

func main() {
	fmt.Println("=== TASK 4: Advection Accuracy (Taylor-Green Vortex t=1.0) ===")

	sizes := []int{16, 32, 64}

	for _, so := range []bool{false, true} {
		scheme := "1st-order"
		if so {
			scheme = "van Leer"
		}
		fmt.Printf("--- %s ---\n", scheme)

		prevErr := 0.0
		for _, n := range sizes {
			meanErr, maxErr := runDecayingVortex(n, so)
			if prevErr > 0 {
				order := math.Log2(prevErr / meanErr)
				fmt.Printf("N = %2d | Mean U Error = %.4f | Max U Error = %.4f | Order = %.2f\n", n, meanErr, maxErr, order)
			} else {
				fmt.Printf("N = %2d | Mean U Error = %.4f | Max U Error = %.4f | Order = N/A\n", n, meanErr, maxErr)
			}
			prevErr = meanErr
		}
		fmt.Println()
	}
}
