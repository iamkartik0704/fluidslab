package main

import (
	"dambreak/internal/solver"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func saveAlphaPNG(g *solver.Grid, f *solver.Fields, path string) error {
	// Let's make it bigger
	scale := 4
	img := image.NewRGBA(image.Rect(0, 0, g.Nx*scale, g.Ny*scale))
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			a := f.Alpha[g.IdxCC(i, j)]
			r := uint8(255 * (1 - a))
			gv := uint8(255 * (1 - a))
			py := g.Ny - j
			px := i - 1
			c := color.RGBA{R: r, G: gv, B: 255, A: 255}
			if a < 0.01 {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					img.Set(px*scale+sx, py*scale+sy, c)
				}
			}
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
	cfg := solver.DefaultConfig()
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.MaxDT = 0.05
	cfg.Threads = 8

	a_in := 2.25
	a := a_in * 0.0254
	L0 := a
	n := 32
	nx := n * 15
	ny := n * 4
	dx := L0 / float64(n)

	sim := solver.NewSimulation(cfg, nx, ny, float64(nx)*dx, float64(ny)*dx, false)
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
	sim.InitDamBreak()
	solver.ApplyAlphaBC(sim.Grid, sim.Fields)

	outDir := `C:\Users\iamka\.gemini\antigravity-ide\brain\2509ce5c-3666-4963-8881-f8d219857e17\snapshots`
	os.MkdirAll(outDir, 0755)

	targets := []float64{2, 4, 6, 8, 10}
	tIdx := 0

	for tIdx < len(targets) {
		err := sim.Step(-1)
		if err != nil {
			fmt.Printf("FAILED at step %d: %v\n", sim.State.Step, err)
			return
		}
		if sim.State.TStar >= targets[tIdx] {
			path := fmt.Sprintf("%s\\alpha_t%d.png", outDir, int(targets[tIdx]))
			saveAlphaPNG(sim.Grid, sim.Fields, path)
			fmt.Printf("Saved %s at t*=%.2f (step %d)\n", path, sim.State.TStar, sim.State.Step)
			tIdx++
		}
	}
	fmt.Println("Done.")
}
