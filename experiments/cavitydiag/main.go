package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

// ghiaRe100 is TABLE I from Ghia, Ghia & Shin (1982), Re=100
var ghiaU = [][2]float64{
	{1.0000, 1.00000},
	{0.9766, 0.84123},
	{0.9688, 0.78871},
	{0.9609, 0.73722},
	{0.9531, 0.68717},
	{0.8516, 0.23151},
	{0.7344, 0.00332},
	{0.6172, -0.13641},
	{0.5000, -0.20581},
	{0.4531, -0.21090},
	{0.2813, -0.15662},
	{0.1719, -0.10150},
	{0.1016, -0.06434},
	{0.0703, -0.04775},
	{0.0625, -0.04192},
	{0.0547, -0.03717},
	{0.0000, 0.00000},
}

var ghiaV = [][2]float64{
	{1.0000, 0.00000},
	{0.9688, -0.05906},
	{0.9531, -0.07391},
	{0.9453, -0.07615},
	{0.9063, -0.07920},
	{0.8594, -0.07936},
	{0.8047, -0.06941},
	{0.5000, 0.05186},
	{0.2344, 0.17527},
	{0.2266, 0.17507},
	{0.1563, 0.16077},
	{0.0938, 0.12317},
	{0.0781, 0.10890},
	{0.0703, 0.10091},
	{0.0625, 0.09233},
	{0.0000, 0.00000},
}

func sampleCenterlineU(g *solver.Grid, f *solver.Fields) func(yNorm float64) float64 {
	return func(yNorm float64) float64 {
		y := yNorm * float64(g.Ny) * g.Dy
		jf := y/g.Dy + 0.5
		j0 := int(math.Floor(jf))
		w := jf - float64(j0)
		i0 := 1 + g.Nx/2
		val := func(jj int) float64 {
			jc := jj
			if jc < 1 {
				jc = 1
			}
			if jc > g.Ny {
				jc = g.Ny
			}
			return 0.5 * (f.U[g.IdxU(i0-1, jc)] + f.U[g.IdxU(i0, jc)])
		}
		return (1-w)*val(j0) + w*val(j0+1)
	}
}

func sampleCenterlineV(g *solver.Grid, f *solver.Fields) func(xNorm float64) float64 {
	return func(xNorm float64) float64 {
		x := xNorm * float64(g.Nx) * g.Dx
		i_f := x/g.Dx + 0.5
		i0 := int(math.Floor(i_f))
		w := i_f - float64(i0)
		j0 := 1 + g.Ny/2
		val := func(ii int) float64 {
			ic := ii
			if ic < 1 {
				ic = 1
			}
			if ic > g.Nx {
				ic = g.Nx
			}
			return 0.5 * (f.V[g.IdxV(ic, j0-1)] + f.V[g.IdxV(ic, j0)])
		}
		return (1-w)*val(i0) + w*val(i0+1)
	}
}

func main() {
	n := 49
	fmt.Printf("=== Cavity Diagnosis (Grid %d^2) ===\n", n)

	cfg := solver.DefaultConfig()
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0
	cfg.Physical.Gravity = 0
	cfg.Numerical.CFL = 0.4
	cfg.Numerical.MaxDT = 1.0
	cfg.Numerical.PoissonTol = 1e-9
	cfg.Physical.RhoW, cfg.Physical.MuW = 1.0, 0.01
	cfg.Physical.RhoA, cfg.Physical.MuA = 1.0, 0.01
	cfg.Threads = 8

	sim := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)

	var lastU []float64
	maxDuDt := 0.0

	for sim.State.Time < 40.0 {
		if sim.State.Step%100 == 0 {
			lastU = make([]float64, len(sim.Fields.U))
			copy(lastU, sim.Fields.U)
		}

		err := sim.Step(-1)
		if err != nil {
			fmt.Println("Error:", err)
			break
		}

		if sim.State.Step%100 == 1 && sim.State.Step > 1 {
			maxD := 0.0
			for i := range sim.Fields.U {
				d := math.Abs(sim.Fields.U[i]-lastU[i]) / sim.State.DT
				if d > maxD {
					maxD = d
				}
			}
			maxDuDt = maxD
		}
	}

	fmt.Printf("Steps: %d, Final Time: %.2f\n", sim.State.Step, sim.State.Time)
	fmt.Printf("Steady state measure (max |du/dt| over last 100 steps): %.3e\n", maxDuDt)
	fmt.Println()

	sU := sampleCenterlineU(sim.Grid, sim.Fields)
	fmt.Println("--- U Velocity (x=0.5) ---")
	fmt.Printf("%10s | %10s | %10s | %10s\n", "y", "Ghia U", "Sim U", "Error")
	maxErrU := 0.0
	maxErrUIdx := -1
	for i, pt := range ghiaU {
		simVal := sU(pt[0])
		err := math.Abs(simVal - pt[1])
		if pt[0] > 0 && pt[0] < 1 && pt[0] != 0.9766 && pt[0] != 0.0547 { // ignore near-wall points for max error
			if err > maxErrU {
				maxErrU = err
				maxErrUIdx = i
			}
		}
		fmt.Printf("%10.4f | %10.5f | %10.5f | %10.5f\n", pt[0], pt[1], simVal, err)
	}
	fmt.Printf("Max Interior Error U: %.5f at index %d (y=%.4f)\n", maxErrU, maxErrUIdx, ghiaU[maxErrUIdx][0])
	fmt.Println()

	sV := sampleCenterlineV(sim.Grid, sim.Fields)
	fmt.Println("--- V Velocity (y=0.5) ---")
	fmt.Printf("%10s | %10s | %10s | %10s\n", "x", "Ghia V", "Sim V", "Error")
	maxErrV := 0.0
	maxErrVIdx := -1
	for i, pt := range ghiaV {
		simVal := sV(pt[0])
		err := math.Abs(simVal - pt[1])
		if pt[0] > 0 && pt[0] < 1 && pt[0] != 0.9688 && pt[0] != 0.0625 {
			if err > maxErrV {
				maxErrV = err
				maxErrVIdx = i
			}
		}
		fmt.Printf("%10.4f | %10.5f | %10.5f | %10.5f\n", pt[0], pt[1], simVal, err)
	}
	fmt.Printf("Max Interior Error V: %.5f at index %d (x=%.4f)\n", maxErrV, maxErrVIdx, ghiaV[maxErrVIdx][0])
}
