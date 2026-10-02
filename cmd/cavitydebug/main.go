package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func maxAbs(arr []float64) float64 {
	m := 0.0
	for _, v := range arr {
		if math.Abs(v) > m {
			m = math.Abs(v)
		}
	}
	return m
}

func calcKE(g *solver.Grid, f *solver.Fields) float64 {
	ke := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			u := 0.5 * (f.U[g.IdxU(i-1, j)] + f.U[g.IdxU(i, j)])
			v := 0.5 * (f.V[g.IdxV(i, j-1)] + f.V[g.IdxV(i, j)])
			ke += 0.5 * (u*u + v*v)
		}
	}
	return ke / float64(g.Nx*g.Ny)
}

func runDebug(n int, so bool, label string, dtDiv float64, ptolo float64, regLid bool) {
	fmt.Printf("\n=== %s ===\n", label)
	cfg := solver.DefaultConfig()
	cfg.Numerical.SecondOrderAdvect = so
	cfg.Physical.MuW = 0.01 // Re = 100 for U=1, L=1
	cfg.Physical.MuA = 0.01
	cfg.Physical.RhoW = 1.0
	cfg.Physical.RhoA = 1.0
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0
	cfg.Physical.Gravity = 0
	cfg.Numerical.CFL = 0.4
	cfg.Numerical.MaxDT = 1.0

	if dtDiv > 1 {
		cfg.Numerical.CFL = 0.4 / dtDiv
	}
	if ptolo > 0 {
		cfg.Numerical.PoissonTol = ptolo
	}

	sim := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)

	prevU := make([]float64, len(sim.Fields.U))
	copy(prevU, sim.Fields.U)

	nextPrint := 1.0
	for sim.State.Time <= 60.0 {
		err := sim.Step(-1)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}

		if sim.State.Time >= nextPrint {
			maxDU := 0.0
			maxI := -1
			maxJ := -1

			for j := 1; j <= sim.Grid.Ny; j++ {
				for i := 1; i <= sim.Grid.Nx; i++ {
					idx := sim.Grid.IdxU(i, j)
					du := math.Abs(sim.Fields.U[idx]-prevU[idx]) / sim.State.DT
					if du > maxDU {
						maxDU = du
						maxI = i
						maxJ = j
					}
				}
			}

			ke := calcKE(sim.Grid, sim.Fields)
			maxU := maxAbs(sim.Fields.U)

			fmt.Printf(" t=%5.1f  KE=%.6e  max|u|=%.6f  max|du/dt|=%.6e at (i=%d, j=%d)\n", sim.State.Time, ke, maxU, maxDU, maxI, maxJ)
			nextPrint += 1.0
		}
		copy(prevU, sim.Fields.U)

		// re-enforce lid BC if it gets overwritten
		for i := 1; i <= n; i++ {
			uVal := 1.0
			if regLid {
				if i == 1 {
					uVal = 0.5
				} else if i == n {
					uVal = 0.5
				}
			}
			sim.Fields.U[sim.Grid.IdxU(i, n+1)] = uVal*2.0 - sim.Fields.U[sim.Grid.IdxU(i, n)]
		}
	}
}

func main() {
	runDebug(65, false, "N=65 FirstOrder Native", 1.0, 0, false)
	runDebug(65, true, "N=65 VanLeer Native", 1.0, 0, false)

	// b. repeat with dt cap halved and quartered
	runDebug(65, true, "N=65 VanLeer dt/2", 2.0, 0, false)
	runDebug(65, true, "N=65 VanLeer dt/4", 4.0, 0, false)

	// c. repeat with Poisson tolerance 1e-9
	runDebug(65, true, "N=65 VanLeer Poisson 1e-9", 1.0, 1e-9, false)

	// d. repeat with lid corner cells treated differently
	runDebug(65, true, "N=65 VanLeer RegLid", 1.0, 0, true)
}
