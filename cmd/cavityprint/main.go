package main

import (
	"dambreak/internal/solver"
	"fmt"
	"math"
)

var ghiaRe100 = [][2]float64{
	{1.0000, 1.00000}, {0.9766, 0.84123}, {0.9688, 0.78871}, {0.9609, 0.73722},
	{0.9531, 0.68717}, {0.8516, 0.23151}, {0.7344, 0.00332}, {0.6172, -0.13641},
	{0.5000, -0.20581}, {0.4531, -0.21090}, {0.2813, -0.15662}, {0.1719, -0.10150},
	{0.1016, -0.06434}, {0.0703, -0.04775}, {0.0625, -0.04192}, {0.0547, -0.03717},
	{0.0000, 0.00000},
}

var ghiaRe100V = [][2]float64{
	{1.0000, 0.00000}, {0.9688, -0.05906}, {0.9609, -0.07391}, {0.9531, -0.08864},
	{0.9453, -0.10313}, {0.9063, -0.16914}, {0.8594, -0.22445}, {0.8047, -0.24533},
	{0.5000, 0.05454}, {0.2344, 0.17527}, {0.2266, 0.17507}, {0.1563, 0.16077},
	{0.0938, 0.12317}, {0.0781, 0.10890}, {0.0703, 0.10091}, {0.0625, 0.09266},
	{0.0000, 0.00000},
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sampleCenterlineU(g *solver.Grid, f *solver.Fields) func(yNorm float64) float64 {
	return func(yNorm float64) float64 {
		y := yNorm * float64(g.Ny) * g.Dy
		jf := y/g.Dy + 0.5
		j0 := int(math.Floor(jf))
		w := jf - float64(j0)
		i0 := 1 + g.Nx/2
		val := func(jj int) float64 {
			jc := clampi(jj, 1, g.NyG-2)
			return 0.5 * (f.U[g.IdxU(i0-1, jc)] + f.U[g.IdxU(i0, jc)])
		}
		return (1-w)*val(j0) + w*val(j0+1)
	}
}

func sampleCenterlineV(g *solver.Grid, f *solver.Fields) func(xNorm float64) float64 {
	return func(xNorm float64) float64 {
		x := xNorm * float64(g.Nx) * g.Dx
		iF := x/g.Dx + 0.5
		i0 := int(math.Floor(iF))
		w := iF - float64(i0)
		j0 := 1 + g.Ny/2
		val := func(ii int) float64 {
			ic := clampi(ii, 1, g.NxG-2)
			return 0.5 * (f.V[g.IdxV(ic, j0-1)] + f.V[g.IdxV(ic, j0)])
		}
		return (1-w)*val(i0) + w*val(i0+1)
	}
}

func computeMaxError(g *solver.Grid, f *solver.Fields) float64 {
	sample := sampleCenterlineU(g, f)
	maxErr := 0.0
	for _, pt := range ghiaRe100 {
		y, want := pt[0], pt[1]
		if y == 0 || y == 1 || y == 0.9766 || y == 0.0547 {
			continue
		}
		err := math.Abs(sample(y) - want)
		if err > maxErr {
			maxErr = err
		}
	}
	return maxErr
}

func main() {
	runCavitySuite()
}

func runCavitySuite() {
	grids := []int{17}
	for _, n := range grids {
		for _, so := range []bool{false} {
			scheme := "FirstOrder"
			if so {
				scheme = "VanLeer"
			}
			fmt.Printf("\n=== N=%d %s ===\n", n, scheme)

			cfg := solver.DefaultConfig()
			cfg.Numerical.OpenTop = false
			cfg.Numerical.LidVelocity = 1.0
			cfg.Physical.Gravity = 0
			cfg.Numerical.CFL = 0.4
			cfg.Numerical.MaxDT = 1.0
			cfg.Numerical.PoissonTol = 1e-9
			cfg.Physical.RhoW, cfg.Physical.MuW = 1.0, 0.01
			cfg.Physical.RhoA, cfg.Physical.MuA = 1.0, 0.01
			cfg.Numerical.SecondOrderAdvect = so

			sim := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)

			for sim.State.Time < 10.0 {
				sim.Step(-1)
			}

			su := sampleCenterlineU(sim.Grid, sim.Fields)
			fmt.Printf("u(y=0) = %g\n", su(0))
			fmt.Printf("u(y=1) = %g\n", su(1))
			err := computeMaxError(sim.Grid, sim.Fields)
			fmt.Printf("maxErr = %g\n", err)
		}
	}
}

func printProfiles(g *solver.Grid, f *solver.Fields) {
	fmt.Println("  -- Profiles --")
	su := sampleCenterlineU(g, f)
	sv := sampleCenterlineV(g, f)
	var mu, mv, uLoc, vLoc float64
	for _, pt := range ghiaRe100 {
		y, w := pt[0], pt[1]
		if y == 0 || y == 1 || y == 0.9766 || y == 0.0547 {
			continue
		}
		err := math.Abs(su(y) - w)
		if err > mu {
			mu = err
			uLoc = y
		}
	}
	for _, pt := range ghiaRe100V {
		x, w := pt[0], pt[1]
		if x == 0 || x == 1 {
			continue
		}
		err := math.Abs(sv(x) - w)
		if err > mv {
			mv = err
			vLoc = x
		}
	}
	fmt.Printf("  maxErrU=%.5f at y=%.4f\n", mu, uLoc)
	fmt.Printf("  maxErrV=%.5f at x=%.4f\n", mv, vLoc)
}
