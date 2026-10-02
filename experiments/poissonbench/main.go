package main

import (
	"dambreak/internal/solver"
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== 4. Poisson Benchmark ===")
	// test sizes
	for _, size := range []struct{ nx, ny int }{{64, 64}, {128, 64}} {
		fmt.Printf("Grid: %dx%d\n", size.nx, size.ny)

		for _, preconditioner := range []string{"none", "jacobi", "ic0"} {
			if preconditioner != "none" && size.nx == 64 {
				// skip for just benchmark? No, let's just use ic0 for benchmark,
				// and report the preconditioner iterations too.
				continue
			}
		}

		// Setup dummy state
		cfg := solver.DefaultConfig()
		cfg.Domain.Nx = size.nx
		cfg.Domain.Ny = size.ny
		cfg.Domain.Width = float64(size.nx)
		cfg.Domain.Height = float64(size.ny)
		cfg.Numerical.PoissonTol = 1e-6
		cfg.Numerical.Preconditioner = "ic0"
		cfg.Threads = 8 // say 8 threads

		sim := solver.NewSimulation(cfg, size.nx, size.ny, cfg.Domain.Width, cfg.Domain.Height, false)

		// Fill Right hand side
		b := make([]float64, sim.Grid.TotalCC())
		p := make([]float64, sim.Grid.TotalCC())
		for i := range b {
			b[i] = 1.0
		}

		scratch := make([][]float64, 9)
		for i := range scratch {
			scratch[i] = make([]float64, len(b))
		}

		bench := func(name string, forceSerial bool) {
			solver.ForceSerialPoisson = forceSerial
			// warmup
			for i := 0; i < 5; i++ {
				for j := range p {
					p[j] = 0.0
				}
				solver.SolvePoissonPCG(sim.Grid, sim.Fields, b, p, 1e-6, 100, cfg.Threads, scratch)
			}

			start := time.Now()
			iters := 50
			for i := 0; i < iters; i++ {
				for j := range p {
					p[j] = 0.0
				}
				solver.SolvePoissonPCG(sim.Grid, sim.Fields, b, p, 1e-6, 100, cfg.Threads, scratch)
			}
			dur := time.Since(start)
			fmt.Printf("  %s (Threads=%d): %.2f ms / step\n", name, cfg.Threads, float64(dur.Milliseconds())/float64(iters))
		}

		bench("Parallel (per-iter goroutines)", false)
		bench("Serial", true)
	}

	fmt.Println("\n=== Dam-Break Preconditioner Test (t*=2) ===")
	// Test at 16 and 32 cells/L0
	for _, cellsPerL0 := range []int{16, 32} {
		L0 := 0.05715
		H0 := 2.0 * L0
		width := 10.0 * L0
		height := 4.0 * L0

		nx := int(width / L0 * float64(cellsPerL0))
		ny := int(height / L0 * float64(cellsPerL0))

		cfg := solver.DefaultConfig()
		cfg.Domain.L0 = L0
		cfg.Domain.H0 = H0
		cfg.Domain.Width = width
		cfg.Domain.Height = height
		cfg.Domain.Nx = nx
		cfg.Domain.Ny = ny
		cfg.Numerical.PoissonTol = 1e-6
		cfg.Numerical.FreeSlip = false
		cfg.Numerical.SecondOrderAdvect = false

		// Run until t*=2
		sim := solver.NewSimulation(cfg, nx, ny, width, height, true)
		for sim.State.TStar < 2.0 {
			sim.Step(-1)
		}

		// Now we have the state at t*=2. We will just re-run the projection step
		// with different preconditioners and count iterations.
		dt := sim.State.DT

		testPC := func(pc string) {
			sim.Cfg.Numerical.Preconditioner = pc
			// We need to call project, but Project() uses sim.scratch
			// Let's just create a dummy scratch
			scratch := make([][]float64, 9)
			for i := range scratch {
				scratch[i] = make([]float64, sim.Grid.TotalCC())
			}

			// Zero out p to measure from a cold start (or keep warm start? we want fair comparison, so cold start is better)
			for i := range sim.Fields.P {
				sim.Fields.P[i] = 0
			}

			res := solver.Project(sim.Grid, sim.Fields, &sim.Cfg, dt, scratch)
			fmt.Printf("DamBreak %d cells/L0, Precond: %-6s -> Iters: %d\n", cellsPerL0, pc, res.Iterations)
		}

		testPC("none")
		testPC("jacobi")
		testPC("ic0")
	}
}
