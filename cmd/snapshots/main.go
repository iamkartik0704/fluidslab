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
	img := image.NewRGBA(image.Rect(0, 0, g.Nx, g.Ny))
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			a := f.Alpha[g.IdxCC(i, j)]
			// Map alpha 0..1 to a blue-white gradient
			// water (alpha=1) = dark blue, air (alpha=0) = white
			blue := uint8(255 * a)
			r := uint8(255 * (1 - a))
			gv := uint8(255 * (1 - a))
			// Flip y: row j=Ny is the top
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

	targets := []float64{0, 1, 2, 3, 4}
	tIdx := 0

	// Save t*=0
	path := fmt.Sprintf("out/snapshots/alpha_t%d.png", 0)
	saveAlphaPNG(sim.Grid, sim.Fields, path)
	fmt.Printf("Saved %s at t*=%.2f\n", path, sim.State.TStar)
	tIdx = 1

	for tIdx < len(targets) {
		err := sim.Step(0)
		if err != nil {
			fmt.Printf("FAILED at step %d: %v\n", sim.State.Step, err)
			return
		}
		if sim.State.TStar >= targets[tIdx] {
			path := fmt.Sprintf("out/snapshots/alpha_t%d.png", int(targets[tIdx]))
			saveAlphaPNG(sim.Grid, sim.Fields, path)
			fmt.Printf("Saved %s at t*=%.2f (step %d)\n", path, sim.State.TStar, sim.State.Step)
			tIdx++
		}
	}
	fmt.Println("Done.")
}
