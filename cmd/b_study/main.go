package main

import (
	"dambreak/internal/solver"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	fmt.Println("=== B.1 Drift Decomposition (16 cells/L0) ===")
	// Configurations: SplitDivFix ON/OFF, tol 1e-6/1e-9
	for _, splitDiv := range []bool{true, false} {
		for _, tol := range []float64{1e-6, 1e-9} {
			fmt.Printf("SplitDiv=%v, Tol=%g\n", splitDiv, tol)
			runDrift(16, splitDiv, tol)
		}
	}

	fmt.Println("\n=== B.2 & B.3 & B.5: Resolutions, X*, Poisson, Snapshots ===")
	for _, cpl := range []int{8, 16, 32} {
		fmt.Printf("Resolution: %d cells/L0\n", cpl)
		runRes(cpl, false, false, false)
	}

	fmt.Println("\n=== B.4: Free-Slip (16 and 32 cells/L0) ===")
	for _, cpl := range []int{16, 32} {
		fmt.Printf("Resolution: %d cells/L0, FREE-SLIP\n", cpl)
		runRes(cpl, true, false, false)
	}

	fmt.Println("\n=== B.6: Van Leer (16 cells/L0) ===")
	runRes(16, false, false, true)
}

func sumAlpha(g *solver.Grid, f *solver.Fields) float64 {
	sum := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			sum += f.Alpha[g.IdxCC(i, j)]
		}
	}
	return sum
}

func runDrift(cpl int, splitDiv bool, tol float64) {
	L0 := 0.05715
	nx := 8 * cpl
	ny := 4 * cpl

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = 2.0 * L0
	cfg.Domain.Width = 8.0 * L0
	cfg.Domain.Height = 4.0 * L0
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny

	cfg.Numerical.PoissonTol = tol
	cfg.Numerical.SplitDivFix = splitDiv
	cfg.Numerical.ClipAlpha = true
	cfg.Numerical.PoissonMaxIter = 5000
	cfg.Numerical.CFL = 0.4

	sim := solver.NewSimulation(cfg, nx, ny, cfg.Domain.Width, cfg.Domain.Height, false)
	sim.InitDamBreak()

	volInit := sumAlpha(sim.Grid, sim.Fields)

	tMax := 4.5 * math.Sqrt(L0/9.81)

	for sim.State.Time < tMax {
		err := sim.Step(-1)
		if err != nil {
			fmt.Println("Error:", err)
			break
		}
	}

	volFinal := sumAlpha(sim.Grid, sim.Fields)

	drift := (volFinal - volInit) / volInit * 100.0
	volClip := sim.State.ClippedMass / volInit * 100.0
	volX := sim.State.VolSweepX / volInit * 100.0
	volY := sim.State.VolSweepY / volInit * 100.0
	outflow := sim.State.TopOutflow / volInit * 100.0

	fmt.Printf("Final Drift: %g%% | Clip: %g%% | SweepX: %g%% | SweepY: %g%% | Outflow: %g%%\n",
		drift, volClip, volX, volY, outflow)
}

func getFront(g *solver.Grid, f *solver.Fields, threshold float64) float64 {
	for i := g.Nx; i >= 1; i-- {
		idx := g.IdxCC(i, 1)
		if f.Alpha[idx] > threshold {
			if i < g.Nx && f.Alpha[g.IdxCC(i+1, 1)] <= threshold {
				a := f.Alpha[idx]
				aNext := f.Alpha[g.IdxCC(i+1, 1)]
				if aNext == a {
					return g.Xc[idx]
				}
				t := (threshold - a) / (aNext - a)
				return g.Xc[idx] + t*g.Dx
			}
			return g.Xc[idx] + 0.5*g.Dx
		}
	}
	return 0.0
}

func getCumFront(g *solver.Grid, f *solver.Fields, frac float64) float64 {
	totVol := sumAlpha(g, f)
	target := frac * totVol
	cum := 0.0
	for i := 1; i <= g.Nx; i++ {
		for j := 1; j <= g.Ny; j++ {
			cum += f.Alpha[g.IdxCC(i, j)]
		}
		if cum >= target {
			return g.Xc[g.IdxCC(i, 1)]
		}
	}
	return g.Xc[g.IdxCC(g.Nx, 1)]
}

func renderPNG(g *solver.Grid, f *solver.Fields, filename string) {
	img := image.NewRGBA(image.Rect(0, 0, g.Nx, g.Ny))
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			a := f.Alpha[g.IdxCC(i, j)]
			v := uint8(255 * (1 - a))
			img.Set(i-1, g.Ny-j, color.RGBA{v, v, 255, 255})
		}
	}
	file, _ := os.Create(filename)
	defer file.Close()
	png.Encode(file, img)
}

func runRes(cpl int, freeSlip bool, drawSnaps bool, adv bool) {
	L0 := 0.05715
	tScale := math.Sqrt(L0 / 9.81)
	nx := 8 * cpl
	ny := 4 * cpl

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = 2.0 * L0
	cfg.Domain.Width = 8.0 * L0
	cfg.Domain.Height = 4.0 * L0
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny

	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.SplitDivFix = true
	cfg.Numerical.ClipAlpha = true
	cfg.Numerical.PoissonMaxIter = 5000
	cfg.Numerical.CFL = 0.4
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.SecondOrderAdvect = adv

	sim := solver.NewSimulation(cfg, nx, ny, cfg.Domain.Width, cfg.Domain.Height, false)
	sim.InitDamBreak()

	volInit := sumAlpha(sim.Grid, sim.Fields)

	checkpts := []float64{0, 1, 2, 3, 4, 5, 6}
	checkIdx := 0

	iterSum := 0
	iterMax := 0
	steps := 0

	maxDrift := 0.0

	for checkIdx < len(checkpts) {
		tStarTarget := checkpts[checkIdx]
		tTarget := tStarTarget * tScale

		for sim.State.Time < tTarget {
			sim.Step(-1)
			steps++
			iterSum += sim.State.PoissonIter
			if sim.State.PoissonIter > iterMax {
				iterMax = sim.State.PoissonIter
			}
			volNow := sumAlpha(sim.Grid, sim.Fields)
			drift := math.Abs((volNow - volInit) / volInit * 100.0)
			if drift > maxDrift {
				maxDrift = drift
			}
		}

		front05 := getFront(sim.Grid, sim.Fields, 0.5) / L0
		front99 := getCumFront(sim.Grid, sim.Fields, 0.99) / L0

		fmt.Printf("t*=%g | X*(0.5)=%.4f | X*(99%%)=%.4f\n", tStarTarget, front05, front99)

		if drawSnaps {
			renderPNG(sim.Grid, sim.Fields, fmt.Sprintf("snap_cpl%d_t%g.png", cpl, tStarTarget))
		}

		checkIdx++
	}

	minA, maxA := 1.0, 0.0
	for _, a := range sim.Fields.Alpha {
		if a < minA {
			minA = a
		}
		if a > maxA {
			maxA = a
		}
	}

	if steps > 0 {
		fmt.Printf("Mean/Max Iters: %d / %d\n", iterSum/steps, iterMax)
	}
	fmt.Printf("Max Drift: %g%%, Alpha Min/Max: %g / %g, CFL Warns: %d\n", maxDrift, minA, maxA, sim.State.CFLWarnings)
}
