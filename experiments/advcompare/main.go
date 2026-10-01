package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
	"time"
)

func runCavityGhia(secondOrder bool) float64 {
	cfg := solver.DefaultConfig()
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0
	cfg.Physical.Gravity = 0
	cfg.Numerical.CFL = 0.4
	cfg.Numerical.MaxDT = 1.0
	cfg.Numerical.PoissonTol = 1e-9
	cfg.Physical.RhoW, cfg.Physical.MuW = 1.0, 0.01
	cfg.Physical.RhoA, cfg.Physical.MuA = 1.0, 0.01
	cfg.Numerical.SecondOrderAdvect = secondOrder
	cfg.Threads = 8

	n := 33
	sim := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)
	for sim.State.Time < 30.0 {
		sim.Step(-1)
	}

	// Ghia Re=100 data (excluding walls and near-wall rows)
	ghia := [][2]float64{
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
	}

	g := sim.Grid
	f := sim.Fields
	maxErr := 0.0
	for _, pt := range ghia {
		yNorm, want := pt[0], pt[1]
		y := yNorm * float64(g.Ny) * g.Dy
		jf := y/g.Dy + 0.5
		j0 := int(math.Floor(jf))
		w := jf - float64(j0)
		i0 := 1 + g.Nx/2
		clamp := func(v, lo, hi int) int {
			if v < lo { return lo }
			if v > hi { return hi }
			return v
		}
		val := func(jj int) float64 {
			jc := clamp(jj, 1, g.NyG-1)
			return 0.5 * (f.U[g.IdxU(i0-1, jc)] + f.U[g.IdxU(i0, jc)])
		}
		got := (1-w)*val(j0) + w*val(j0+1)
		if d := math.Abs(got - want); d > maxErr {
			maxErr = d
		}
	}
	return maxErr
}

func runDamBreak(cellsPerL0 int, secondOrder bool) {
	L0 := 0.05715
	H0 := 2.0 * L0
	nx := 4 * cellsPerL0
	ny := 4 * cellsPerL0

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Domain.Width = 4.0 * L0
	cfg.Domain.Height = 4.0 * L0
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.SecondOrderAdvect = secondOrder
	cfg.Threads = 8

	sim := solver.NewSimulation(cfg, nx, ny, cfg.Domain.Width, cfg.Domain.Height, false)
	sim.InitDamBreak()

	targets := []float64{1.0, 2.0, 3.0}
	tIdx := 0

	label := "1st-order"
	if secondOrder {
		label = "van Leer"
	}

	for tIdx < len(targets) && sim.State.TStar < 3.05 {
		sim.Step(0)
		if sim.State.TStar >= targets[tIdx] {
			fmt.Printf("  [%s %d/L0] t*=%.1f  X*=%.4f  drift=%.3e%%\n",
				label, cellsPerL0, targets[tIdx], sim.State.FrontXStar,
				100*sim.State.VolumeDrift/sim.State.Volume)
			tIdx++
		}
	}
}

func main() {
	fmt.Println("=== Priority B.5: Second-Order Momentum Advection Comparison ===")
	fmt.Println()

	// Cavity
	fmt.Println("--- Cavity Re=100, 33x33 ---")
	t0 := time.Now()
	err1 := runCavityGhia(false)
	fmt.Printf("  1st-order: max|u-Ghia| = %.4f (%.2fs)\n", err1, time.Since(t0).Seconds())
	t0 = time.Now()
	err2 := runCavityGhia(true)
	fmt.Printf("  van Leer:  max|u-Ghia| = %.4f (%.2fs)\n", err2, time.Since(t0).Seconds())
	fmt.Println()

	// Dam-break at 16 cells/L0
	fmt.Println("--- Dam-break 16 cells/L0 (64x64) ---")
	runDamBreak(16, false)
	runDamBreak(16, true)
	fmt.Println()

	// Dam-break at 32 cells/L0 (first-order only, for reference)
	fmt.Println("--- Dam-break 32 cells/L0 (128x128), 1st-order only ---")
	runDamBreak(32, false)
}
