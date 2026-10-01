package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

func main() {
	fmt.Println("=== 3. Front convergence ===")
	resolutions := []int{8, 16, 32} // cells per L0

	// Print table headers
	fmt.Printf("%-10s %-10s %-8s %-12s %-12s %-12s %-12s\n", "Type", "Res", "Time", "X*(0.5)", "X*(99%)", "TipThick", "WallShear")

	for _, freeSlip := range []bool{false, true} {
		for _, cellsPerL0 := range resolutions {
			runFrontConv(cellsPerL0, freeSlip)
		}
	}
}

func runFrontConv(cellsPerL0 int, freeSlip bool) {
	L0 := 0.05715
	H0 := 2.0 * L0
	width := 10.0 * L0
	height := 4.0 * L0

	nx := int(math.Round(width / L0 * float64(cellsPerL0)))
	ny := int(math.Round(height / L0 * float64(cellsPerL0)))

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Width = width
	cfg.Domain.Height = height
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny

	cfg.Physical.RhoW = 1000.0
	cfg.Physical.RhoA = 1.0
	cfg.Physical.MuW = 1.0e-3
	cfg.Physical.MuA = 1.8e-5
	cfg.Physical.Gravity = 9.81

	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.SecondOrderAdvect = false

	// Open top (not enclosed)
	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	// Use the built-in InitDamBreak which sets alpha=1.0 for x<=L0, y<=H0
	sim.InitDamBreak()

	tStars := []float64{1.0, 2.0, 3.0, 4.0}
	tIdx := 0

	for sim.State.Time < (4.01 * math.Sqrt(2.0*L0/9.81)) {
		sim.Step(-1)
		if tIdx < len(tStars) && sim.State.TStar >= tStars[tIdx] {
			reportFront(sim, tStars[tIdx], cellsPerL0, freeSlip)
			tIdx++
		}
	}
}

func reportFront(sim *solver.Simulation, targetT float64, cellsPerL0 int, freeSlip bool) {
	g := sim.Grid
	f := sim.Fields
	L0 := sim.Cfg.Domain.L0

	// X*(0.5): rightmost 0.5 crossing in the lowest row (j=1)
	xStarCross := 0.0
	for i := g.Nx; i >= 1; i-- {
		idx := g.IdxCC(i, 1)
		a := f.Alpha[idx]
		if a >= 0.5 {
			// sub-cell interpolation
			if i < g.Nx {
				aNext := f.Alpha[g.IdxCC(i+1, 1)]
				if aNext < 0.5 {
					t := (0.5 - a) / (aNext - a)
					xFront := g.Xc[idx] + t*g.Dx
					xStarCross = (xFront - L0) / L0
				} else {
					xStarCross = (g.Xc[idx] + 0.5*g.Dx - L0) / L0
				}
			} else {
				xStarCross = (g.Xc[idx] + 0.5*g.Dx - L0) / L0
			}
			break
		}
	}

	// X*(99%): x such that 99% of total water lies at x' <= x
	totalVol := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			totalVol += f.Alpha[g.IdxCC(i, j)] * g.Dx * g.Dy
		}
	}

	cumVol := 0.0
	xStar99 := 0.0
	for i := 1; i <= g.Nx; i++ {
		colVol := 0.0
		for j := 1; j <= g.Ny; j++ {
			colVol += f.Alpha[g.IdxCC(i, j)] * g.Dx * g.Dy
		}
		cumVol += colVol
		if cumVol >= 0.99*totalVol {
			xStar99 = (g.Xc[g.IdxCC(i, 1)] - L0) / L0
			break
		}
	}

	// Tip thickness: cells with alpha > 0.5 in the column at the rightmost 0.01 crossing
	frontI := 1
	for i := g.Nx; i >= 1; i-- {
		if f.Alpha[g.IdxCC(i, 1)] > 0.01 {
			frontI = i
			break
		}
	}

	tipCells := 0
	for j := 1; j <= g.Ny; j++ {
		if f.Alpha[g.IdxCC(frontI, j)] > 0.5 {
			tipCells++
		}
	}

	// Wall shear near front
	var wallShear float64
	for i := frontI - 5; i <= frontI; i++ {
		if i < 1 {
			continue
		}
		idxU := g.IdxU(i, 1)
		uCell := f.U[idxU]
		tau := sim.Cfg.Physical.MuW * uCell / (0.5 * g.Dy)
		wallShear += tau * g.Dx
	}

	typStr := "NoSlip"
	if freeSlip {
		typStr = "FreeSlip"
	}

	wallShearStr := fmt.Sprintf("%.2e", wallShear)
	if targetT != 2.0 {
		wallShearStr = "-"
	}
	tipThickStr := fmt.Sprintf("%d", tipCells)
	if targetT != 2.0 && targetT != 3.0 {
		tipThickStr = "-"
	}

	xCrossStr := fmt.Sprintf("%.4f", xStarCross)
	if xStarCross*L0+L0 >= 10.0*L0-0.5*L0 {
		xCrossStr = "WALL"
	}
	x99Str := fmt.Sprintf("%.4f", xStar99)
	if xStar99*L0+L0 >= 10.0*L0-0.5*L0 {
		x99Str = "WALL"
	}

	fmt.Printf("%-10s %-10d %-8.1f %-12s %-12s %-12s %-12s\n", typStr, cellsPerL0, targetT, xCrossStr, x99Str, tipThickStr, wallShearStr)
}
