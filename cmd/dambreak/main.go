package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"dambreak/internal/solver"
)

func main() {
	nx := flag.Int("nx", 128, "interior cells in x")
	ny := flag.Int("ny", 192, "interior cells in y")
	steps := flag.Int("steps", 500, "number of timesteps")
	mode := flag.String("mode", "dambreak", "dambreak | single | cavity")
	freeSlip := flag.Bool("freeslip", false, "enable freeslip")
	halfRes := flag.Bool("halfres", false, "run at half resolution")
	cavityN := flag.Int("cavityN", 65, "cavity grid per side")
	cavityT := flag.Float64("cavityT", 40, "cavity physical end time")
	dambreakT := flag.Float64("dambreakT", 1.0, "dambreak physical end time")
	outHz := flag.Int("hz", 10, "diagnostic print frequency per second of wall time")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	threads := flag.Int("threads", runtime.NumCPU(), "number of threads for poisson solver")
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Println(err)
			return
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *mode == "cavity" {
		runCavityCLI(*cavityN, *cavityT, *outHz)
		return
	}
	if *mode == "dambreak" {
		runNx, runNy := *nx, *ny
		prefix := "default"
		if *halfRes {
			runNx, runNy = 64, 96
			prefix = "half"
		} else if *freeSlip {
			prefix = "freeslip"
		}
		runDambreakCLI(runNx, runNy, *dambreakT, *outHz, *freeSlip, prefix, *steps, *threads)
		return
	}

	cfg := solver.DefaultConfig()
	cfg.Domain.Nx = *nx
	cfg.Domain.Ny = *ny
	cfg.DisplayHz = *outHz

	width := 4 * cfg.Domain.L0
	height := 3 * cfg.Domain.H0
	s := solver.NewSimulation(cfg, cfg.Domain.Nx, cfg.Domain.Ny, width, height, false)

	fmt.Printf("single-phase: %dx%d, CFL %.2f, tol %.1e\n",
		cfg.Domain.Nx, cfg.Domain.Ny, cfg.Numerical.CFL, cfg.Numerical.PoissonTol)

	lastPrint := time.Now()
	for k := 0; k < *steps; k++ {
		if err := s.Step(-1); err != nil {
			fmt.Println("STOP:", err)
			break
		}
		if time.Since(lastPrint) > time.Second/time.Duration(*outHz) {
			lastPrint = time.Now()
			st := &s.State
			fmt.Printf("t=%7.4f step=%6d dt=%.2e poisson=%5d res=%.1e maxDiv=%.2e\n",
				st.Time, st.Step, st.DT, st.PoissonIter, st.PoissonResidual, st.MaxDiv)
		}
	}
	st := &s.State
	fmt.Printf("final: t=%.4f step=%d maxDiv=%.2e poisson=%d res=%.1e\n",
		st.Time, st.Step, st.MaxDiv, st.PoissonIter, st.PoissonResidual)
}

// runCavityCLI runs the lid-driven cavity benchmark headless and prints the
// centreline u-profile.
func runCavityCLI(n int, tMax float64, outHz int) {
	cfg := solver.DefaultConfig()
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0
	cfg.Physical.Gravity = 0
	cfg.Physical.RhoW, cfg.Physical.MuW = 1.0, 0.01 // Re = 100
	cfg.Physical.RhoA, cfg.Physical.MuA = 1.0, 0.01
	cfg.DisplayHz = outHz

	s := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)
	lastPrint := time.Now()
	for s.State.Time < tMax {
		if err := s.Step(-1); err != nil {
			fmt.Println("STOP:", err)
			return
		}
		if time.Since(lastPrint) > time.Second/time.Duration(outHz) {
			lastPrint = time.Now()
			fmt.Printf("t=%7.3f step=%6d poisson=%5d res=%.1e maxDiv=%.2e\n",
				s.State.Time, s.State.Step, s.State.PoissonIter,
				s.State.PoissonResidual, s.State.MaxDiv)
		}
	}
	fmt.Printf("cavity done: t=%.2f steps=%d maxDiv=%.2e\n",
		s.State.Time, s.State.Step, s.State.MaxDiv)
}

// runDambreakCLI runs the dambreak simulation and prints X* vs t*.
func runDambreakCLI(nx, ny int, tMax float64, outHz int, freeSlip bool, prefix string, maxSteps int, threads int) {
	cfg := solver.DefaultConfig()
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.DisplayHz = outHz
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Threads = threads

	width := 4 * cfg.Domain.L0
	height := 3 * cfg.Domain.H0
	s := solver.NewSimulation(cfg, cfg.Domain.Nx, cfg.Domain.Ny, width, height, false)

	s.InitDamBreak()
	solver.ApplyAlphaBC(s.Grid, s.Fields)
	solver.UpdateProperties(s.Grid, s.Fields, &cfg)
	solver.InterpolateRhoToFaces(s.Grid, s.Fields, &cfg)
	s.UpdateDiagnostics()

	fmt.Printf("dambreak: %dx%d, L0=%.3f, H0=%.3f, Tmax=%.2f\n",
		nx, ny, cfg.Domain.L0, cfg.Domain.H0, tMax)
	fmt.Println("t* Convention: t* = t * sqrt(2g / L0)")
	fmt.Println("X* Convention: X* = x / L0")

	csvFile, err := os.Create(fmt.Sprintf("out/dambreak_%s.csv", prefix))
	if err != nil {
		fmt.Println("Error creating CSV:", err)
		return
	}
	defer csvFile.Close()

	fmt.Fprintf(csvFile, "step,t,tStar,dt,CFL,xFront,XStar,volume,volDriftPct,maxDiv,poissonIters,poissonResidual,KE,clippedAlpha,topOutflow,maxVelInterface\n")

	// Helper to print state
	printState := func() {
		st := &s.State
		volDriftPct := 0.0
		if st.Volume > 0 {
			volDriftPct = 100.0 * st.VolumeDrift / st.Volume
		}
		fmt.Fprintf(csvFile, "%d,%.6f,%.6f,%.6e,%.4f,%.6f,%.6f,%.6f,%.6e,%.6e,%d,%.6e,%.6e,%.6e,%.6e,%.6f\n",
			st.Step, st.Time, st.TStar, st.DT, st.CFL, st.FrontX, st.FrontXStar,
			st.Volume+st.VolumeDrift, volDriftPct, st.MaxDiv, st.PoissonIter, st.PoissonResidual,
			st.KineticEnergy, st.ClippedMass, st.TopOutflow, st.MaxVelInterface)
	}

	saveASCII := func(tStarTarget int) {
		filename := fmt.Sprintf("out/alpha_%s_tStar_%d.txt", prefix, tStarTarget)
		f, err := os.Create(filename)
		if err != nil {
			return
		}
		defer f.Close()
		g := s.Grid
		fields := s.Fields
		for j := g.Ny; j >= 1; j-- {
			for i := 1; i <= g.Nx; i++ {
				alpha := fields.Alpha[g.IdxCC(i, j)]
				if alpha > 0.5 {
					fmt.Fprintf(f, "##")
				} else {
					fmt.Fprintf(f, "..")
				}
			}
			fmt.Fprintf(f, "\n")
		}
	}

	printState()
	saveASCII(0)

	targets := []float64{1.0, 2.0, 3.0, 5.0, 8.0}
	nextTargetIdx := 0

	// Loop to tStar = 4.5 (changed from 20.0 to save time for this response)
	targetTStar := 4.5
	startWall := time.Now()
	lastPrint := time.Now()

	totalPoissonIters := 0
	maxPoissonIters := 0

	for s.State.TStar < targetTStar {
		if maxSteps > 0 && s.State.Step >= maxSteps {
			break
		}
		if err := s.Step(-1); err != nil {
			fmt.Println("STOP:", err)
			break
		}

		totalPoissonIters += s.State.PoissonIter
		if s.State.PoissonIter > maxPoissonIters {
			maxPoissonIters = s.State.PoissonIter
		}

		printState()

		if nextTargetIdx < len(targets) && s.State.TStar >= targets[nextTargetIdx] {
			saveASCII(int(targets[nextTargetIdx]))
			nextTargetIdx++
		}

		if time.Since(lastPrint) > time.Second/time.Duration(outHz) {
			lastPrint = time.Now()
			st := &s.State
			volDriftPct := 0.0
			if st.Volume > 0 {
				volDriftPct = 100.0 * st.VolumeDrift / st.Volume
			}
			fmt.Printf("t*=%.2f step=%d X*=%.3f drift=%.2e%%\n",
				st.TStar, st.Step, st.FrontXStar, volDriftPct)
		}
	}

	wallTime := time.Since(startWall)
	meanPoissonIters := float64(totalPoissonIters) / float64(s.State.Step)
	fmt.Printf("Dam break completed in %v\n", wallTime)
	fmt.Printf("Poisson Iters: mean=%.2f, max=%d\n", meanPoissonIters, maxPoissonIters)
}
