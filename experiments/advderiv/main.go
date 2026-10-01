package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func runAdvectionDerivativeTest(N int, secondOrder bool) float64 {
	L := math.Pi
	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L
	cfg.Domain.H0 = L
	cfg.Domain.Nx = N
	cfg.Domain.Ny = N
	cfg.Domain.Width = L
	cfg.Domain.Height = L
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = secondOrder

	sim := solver.NewSimulation(cfg, N, N, L, L, false)
	g := sim.Grid
	f := sim.Fields

	// Set smooth u field: u(x,y) = sin(2x) * cos(y)
	// Analytical du/dx = 2 cos(2x) cos(y)
	// We will compute u * du/dx
	// exact advection = u * du/dx = sin(2x)cos(y) * 2 cos(2x)cos(y) = 2 sin(2x)cos(2x)cos^2(y) = sin(4x)cos^2(y)

	for j := 1; j <= N; j++ {
		for i := 1; i <= N+1; i++ {
			x := float64(i-1) * g.Dx
			y := (float64(j) - 0.5) * g.Dy
			f.U[g.IdxU(i, j)] = math.Sin(2.0*x) * math.Cos(y)
		}
	}

	maxErr := 0.0

	for j := 2; j < N; j++ {
		for i := 3; i < N-1; i++ {
			idx := g.IdxU(i, j)
			uE := f.U[g.IdxU(i+1, j)]
			uW := f.U[g.IdxU(i-1, j)]
			uC := f.U[idx]

			dudx := 0.0
			if uC >= 0 {
				if secondOrder {
					uWW := f.U[g.IdxU(i-2, j)]
					dudx = solver.TvdGradExport(uWW, uW, uC, uE, g.Dx, true)
				} else {
					dudx = (uC - uW) * g.InvDx
				}
			} else {
				if secondOrder {
					uEE := f.U[g.IdxU(i+2, j)]
					dudx = solver.TvdGradExport(uEE, uE, uC, uW, g.Dx, true)
					dudx = -dudx
				} else {
					dudx = (uE - uC) * g.InvDx
				}
			}

			numAdv := uC * dudx

			x := float64(i-1) * g.Dx
			y := (float64(j) - 0.5) * g.Dy
			exactAdv := math.Sin(4.0*x) * math.Cos(y) * math.Cos(y)

			err := math.Abs(numAdv - exactAdv)
			if err > maxErr {
				maxErr = err
			}
		}
	}
	return maxErr
}

func main() {
	fmt.Println("=== TASK 4: Advection Derivative Accuracy ===")
	sizes := []int{16, 32, 64, 128}

	for _, so := range []bool{false, true} {
		scheme := "1st-order"
		if so {
			scheme = "van Leer"
		}
		fmt.Printf("--- %s ---\n", scheme)

		prevErr := 0.0
		for _, n := range sizes {
			err := runAdvectionDerivativeTest(n, so)
			if prevErr > 0 {
				order := math.Log2(prevErr / err)
				fmt.Printf("N = %3d | Max Advection Error = %.5e | Order = %.2f\n", n, err, order)
			} else {
				fmt.Printf("N = %3d | Max Advection Error = %.5e | Order = N/A\n", n, err)
			}
			prevErr = err
		}
		fmt.Println()
	}
}
