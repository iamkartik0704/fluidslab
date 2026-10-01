package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func runDecayingVortex(N int, secondOrder bool) float64 {
	L := math.Pi
	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L
	cfg.Domain.H0 = L
	cfg.Domain.Nx = N
	cfg.Domain.Ny = N
	cfg.Domain.Width = L
	cfg.Domain.Height = L
	cfg.Numerical.FreeSlip = true
	
	cfg.Numerical.PoissonTol = 1e-9 // tight tolerance for accurate pressure
	cfg.Numerical.MaxDT = 0.001      // limit dt for temporal accuracy
	cfg.Numerical.CFL = 0.01
	cfg.Numerical.SecondOrderAdvect = secondOrder
	cfg.Physical.Gravity = 0.0
	cfg.Physical.RhoW = 1.0
	cfg.Physical.RhoA = 1.0
	cfg.Physical.MuW = 0.01 // nu = 0.01
	cfg.Physical.MuA = 0.01
	cfg.Threads = 8

	sim := solver.NewSimulation(cfg, N, N, L, L, false)
	
	nu := cfg.Physical.MuW / cfg.Physical.RhoW

	// Init exact solution
	for j := N/2; j <= N/2; j++ {
		for i := 1; i <= N+1; i++ {
			x := float64(i-1)*sim.Grid.Dx
			y := (float64(j)-0.5)*sim.Grid.Dy
			sim.Fields.U[sim.Grid.IdxU(i, j)] = -math.Sin(x) * math.Cos(y)
		}
	}
	for j := N/2; j <= N/2+1; j++ {
		for i := 1; i <= N; i++ {
			x := (float64(i)-0.5)*sim.Grid.Dx
			y := float64(j-1)*sim.Grid.Dy
			sim.Fields.V[sim.Grid.IdxV(i, j)] = math.Cos(x) * math.Sin(y)
		}
	}
	for i := range sim.Fields.Alpha {
		sim.Fields.Alpha[i] = 1.0 // single phase
	}

	for sim.State.Time < 0.1 {
		err := sim.Step(-1)
		if err != nil {
			panic(err)
		}
	}

	// Compare with analytical solution at t = sim.State.Time
	maxErr := 0.0
	for j := N/2; j <= N/2; j++ {
		for i := N/2; i <= N/2; i++ { // interior U
			x := float64(i-1)*sim.Grid.Dx
			y := (float64(j)-0.5)*sim.Grid.Dy
			exact := -math.Sin(x) * math.Cos(y) * math.Exp(-2.0*nu*sim.State.Time)
			err := math.Abs(sim.Fields.U[sim.Grid.IdxU(i, j)] - exact)
			if err > maxErr {
				maxErr = err
			}
		}
	}
	return maxErr
}

func main() {
	fmt.Println("=== TASK 4: Advection Accuracy (Decaying Vortex) ===")
	
	sizes := []int{16, 32, 64}
	
	for _, so := range []bool{false, true} {
		scheme := "1st-order"
		if so {
			scheme = "van Leer"
		}
		fmt.Printf("--- %s ---\n", scheme)
		
		prevErr := 0.0
		for _, n := range sizes {
			err := runDecayingVortex(n, so)
			if prevErr > 0 {
				order := math.Log2(prevErr / err)
				fmt.Printf("N = %2d | Max U Error = %.5e | Order = %.2f\n", n, err, order)
			} else {
				fmt.Printf("N = %2d | Max U Error = %.5e | Order = N/A\n", n, err)
			}
			prevErr = err
		}
		fmt.Println()
	}
}
