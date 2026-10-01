package main

import (
	"fmt"
	"math"
	"dambreak/internal/solver"
)

func interpZSim(T_sim, Z_sim []float64, T_target float64) float64 {
	for i := 0; i < len(T_sim)-1; i++ {
		if T_target >= T_sim[i] && T_target <= T_sim[i+1] {
			if T_sim[i+1] == T_sim[i] {
				return Z_sim[i]
			}
			t := (T_target - T_sim[i]) / (T_sim[i+1] - T_sim[i])
			return Z_sim[i] + t*(Z_sim[i+1]-Z_sim[i])
		}
	}
	return math.NaN()
}

func main() {
	fmt.Println("=== Free-Slip Reproducibility Check ===")
	runFS("Run 1: First-Order", false)
	runFS("Run 2: First-Order (Identical)", false)
	runFS("Run 3: Van Leer", true)
	runFS("Run 4: Van Leer (Identical)", true)
}

func runFS(name string, vanLeer bool) {
	fmt.Printf("\n%s\n", name)
	L0 := 0.05715
	H0 := 2.0 * L0
	width := 15.0 * L0
	height := 4.0 * L0

	cellsPerL0 := 16
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
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.ClipRedistribute = true
	cfg.Numerical.SplitDivFix = true
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0

	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	var T_sim []float64
	var Z_sim []float64

	for sim.State.TStar <= 4.02 {
		sim.Step(0)
		if sim.State.FrontXStar > 0 {
			T_sim = append(T_sim, sim.State.TStar)
			Z_sim = append(Z_sim, sim.State.FrontXStar)
		}
	}

	targets := []float64{2.0, 3.0, 4.0}
	for _, target := range targets {
		z := interpZSim(T_sim, Z_sim, target)
		fmt.Printf("  T* = %.1f, X*(0.5) = %.4f\n", target, z)
	}
}
