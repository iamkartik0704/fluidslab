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

func saveAlphaPNG(g *solver.Grid, f *solver.Fields, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, g.Nx, g.Ny))
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			a := f.Alpha[g.IdxCC(i, j)]
			blue := uint8(255 * a)
			r := uint8(255 * (1 - a))
			gv := uint8(255 * (1 - a))
			py := g.Ny - j
			px := i - 1
			img.Set(px, py, color.RGBA{R: r, G: gv, B: 255, A: 255 - blue/2})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func main() {
	L0 := 0.05715
	H0 := 2.0 * L0
	nx := 64 // 16 cells per L0
	ny := 64

	cfg := solver.DefaultConfig()
	cfg.Domain.L0 = L0
	cfg.Domain.H0 = H0
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Domain.Width = 4.0 * L0
	cfg.Domain.Height = 4.0 * L0
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Threads = 8

	sim := solver.NewSimulation(cfg, nx, ny, cfg.Domain.Width, cfg.Domain.Height, false)
	sim.InitDamBreak()

	os.MkdirAll("out/snapshots", 0755)

	snapTargets := map[int]bool{5: true, 8: true}
	reportTargets := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8}
	rIdx := 1 // skip t*=0

	fmt.Println("=== Long run: 16 cells/L0, t*=8 ===")
	fmt.Printf("| t* | X* | Drift%% | MaxDiv | Iters | minAlpha | maxAlpha |\n")
	fmt.Printf("|----|----|--------|--------|-------|----------|----------|\n")
	fmt.Printf("| 0.0 | %.3f | 0.000 | 0.000 | 0 | 0.00 | 1.00 |\n", sim.State.FrontXStar)

	for sim.State.TStar < 8.05 {
		err := sim.Step(0)
		if err != nil {
			fmt.Printf("FAILED at step %d (t*=%.2f): %v\n", sim.State.Step, sim.State.TStar, err)
			return
		}

		if rIdx < len(reportTargets) && sim.State.TStar >= reportTargets[rIdx] {
			minA, maxA := 1.0, 0.0
			for jj := 1; jj <= sim.Grid.Ny; jj++ {
				for ii := 1; ii <= sim.Grid.Nx; ii++ {
					a := sim.Fields.Alpha[sim.Grid.IdxCC(ii, jj)]
					if a < minA {
						minA = a
					}
					if a > maxA {
						maxA = a
					}
				}
			}
			driftPct := 100 * sim.State.VolumeDrift / sim.State.Volume
			fmt.Printf("| %.1f | %.3f | %.3e | %.3e | %d | %.2e | %.4f |\n",
				reportTargets[rIdx], sim.State.FrontXStar, driftPct,
				sim.State.MaxDiv, sim.State.PoissonIter, minA, maxA)

			tInt := int(math.Round(reportTargets[rIdx]))
			if snapTargets[tInt] {
				path := fmt.Sprintf("out/snapshots/alpha_t%d.png", tInt)
				saveAlphaPNG(sim.Grid, sim.Fields, path)
				fmt.Printf("  -> Saved %s\n", path)
			}
			rIdx++
		}
	}
	fmt.Printf("\nFinal: steps=%d, drift=%.4f%%, clippedMass=%.4f cells, topOutflow=%.6e\n",
		sim.State.Step, 100*sim.State.VolumeDrift/sim.State.Volume,
		sim.State.ClippedMass, sim.State.TopOutflow)
	fmt.Println("Stability: PASSED (reached t*=8)")
}
