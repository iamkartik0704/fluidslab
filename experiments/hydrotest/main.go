package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func main() {
	L0 := 0.05715
	H0 := 2.0 * L0
	W := 4.0 * L0
	H := 4.0 * L0
	nx := 64
	ny := 64

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Domain.Width = W
	cfg.Domain.Height = H
	cfg.Numerical.PoissonTol = 1e-9
	cfg.Numerical.OpenTop = true
	cfg.Numerical.DilatationCorr = true
	cfg.Numerical.SplitDivFix = true
	cfg.Threads = 8
	cfg.Physical.RhoA = 1.2

	sim := solver.NewSimulation(cfg, nx, ny, W, H, false)
	
	for j := 1; j <= ny; j++ {
		for i := 1; i <= nx; i++ {
			y := sim.Grid.Yc[sim.Grid.IdxCC(i, j)]
			if y <= H0 {
				sim.Fields.Alpha[sim.Grid.IdxCC(i, j)] = 1.0
			} else {
				sim.Fields.Alpha[sim.Grid.IdxCC(i, j)] = 0.0
			}
		}
	}
	solver.ApplyAlphaBC(sim.Grid, sim.Fields)
	solver.UpdateProperties(sim.Grid, sim.Fields, &cfg); solver.InterpolateRhoToFaces(sim.Grid, sim.Fields, &cfg)

	err := sim.Step(0)
	fmt.Printf("Step error: %v\n", err)

	for j := 1; j <= ny; j++ {
		div := (sim.Fields.UStar[sim.Grid.EastVFace(32, j)] - sim.Fields.UStar[sim.Grid.WestVFace(32, j)])*sim.Grid.InvDx + (sim.Fields.VStar[sim.Grid.NorthHFace(32, j)] - sim.Fields.VStar[sim.Grid.SouthHFace(32, j)])*sim.Grid.InvDy
		fmt.Printf("j=%d, P=%f, div=%f, VStarN=%f, VStarS=%f\n", j, sim.Fields.P[sim.Grid.IdxCC(32, j)], div, sim.Fields.VStar[sim.Grid.NorthHFace(32, j)], sim.Fields.VStar[sim.Grid.SouthHFace(32, j)])
	}

	maxErr := 0.0
	maxErrI, maxErrJ := -1, -1
	isInterface := false
	sumErr := 0.0
	countFullyWater := 0
	rhoW := cfg.Physical.RhoW
	rhoA := cfg.Physical.RhoA
	g := cfg.Physical.Gravity

	// Theoretical pressure P(y)
	// P(y) = rho_w * g * (H0 - y) + rho_a * g * (H - H0) for y < H0
	// P(y) = rho_a * g * (H - y) for y > H0
	for j := 1; j <= ny; j++ {
		for i := 1; i <= nx; i++ {
			idx := sim.Grid.IdxCC(i, j)
			a := sim.Fields.Alpha[idx]
			y := sim.Grid.Yc[idx]
			
			pExact := 0.0
			if y <= H0 {
				pExact = rhoW * g * (H0 - y) + rhoA * g * (H - H0)
			} else {
				pExact = rhoA * g * (H - y)
			}
			
			pSim := sim.Fields.P[idx]
			e := math.Abs(pSim - pExact)
			
			if a > 0.99 && y <= H0 {
				sumErr += e
				countFullyWater++
			}
			
			if e > maxErr {
				maxErr = e
				maxErrI = i
				maxErrJ = j
				isInterface = (a > 0.01 && a < 0.99) || (math.Abs(y-H0) <= sim.Grid.Dy)
			}
		}
	}

	meanErr := 0.0
	if countFullyWater > 0 {
		meanErr = sumErr / float64(countFullyWater)
	}

	fmt.Printf("=== TASK 7: Hydrostatic Pressure Error ===\n")
	fmt.Printf("Max Error: %.5e at cell (i=%d, j=%d)\n", maxErr, maxErrI, maxErrJ)
	fmt.Printf("Is max error at/near interface? %v\n", isInterface)
	fmt.Printf("Mean Error (fully water cells): %.5e\n", meanErr)
}
