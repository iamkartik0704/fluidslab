package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"dambreak/internal/benchmark"
	"dambreak/internal/solver"
)

type BenchmarkData struct {
	TimeNormalisation struct {
		AnchorZ float64 `json:"anchor_Z"`
		AnchorT float64 `json:"anchor_T"`
	} `json:"time_normalisation"`
	Scales struct {
		A2p25In struct {
			Records []struct {
				Label string    `json:"label"`
				Z     []float64 `json:"Z"`
				T     []float64 `json:"T"`
			} `json:"records"`
			Mean struct {
				Z []float64 `json:"Z"`
				T []float64 `json:"T"`
			} `json:"mean"`
		} `json:"a_2p25_in"`
		A1p125In struct {
			Mean struct {
				Z []float64 `json:"Z"`
				T []float64 `json:"T"`
			} `json:"mean"`
		} `json:"a_1p125_in"`
	} `json:"scales"`
}

func getGitInfo() (string, bool) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	hash := "unknown"
	if err == nil {
		hash = strings.TrimSpace(string(out))
	}
	out, err = exec.Command("git", "status", "--porcelain").Output()
	dirty := true
	if err == nil && len(strings.TrimSpace(string(out))) == 0 {
		dirty = false
	}
	return hash, dirty
}

type RunResult struct {
	Name            string
	WallTime        time.Duration
	Steps           int
	MaxVolDrift     float64
	Warnings        int
	MaxDiv          float64
	MeanPoissonIter float64
	MaxPoissonIter  int
	TotalPoisson    int
	MaxCFL          float64
	FirstDT         float64

	Z_sim []float64
	T_sim []float64

	// Drift decomposition
	VolSweepX   float64
	VolSweepY   float64
	VolClip     float64
	TopOutflow  float64
	FinalDrift  float64
	InitialVol  float64
	ClipRedist  bool
}

func runSim(name string, cellsPerL0 int, dtScale float64, subMom, subVOF int, cflFrac float64, gitHash string, gitDirty bool) RunResult {
	start := time.Now()

	L0 := 0.05715
	H0 := 2.0 * L0
	width := 15.0 * L0
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
	cfg.Physical.RhoA = 1000.0 / 1000.0
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = true
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.SplitDivFix = true
	cfg.Numerical.ClipRedistribute = true

	cfg.Numerical.CFL *= dtScale
	cfg.Numerical.ViscousCFL *= dtScale
	cfg.Numerical.GravityCFL *= dtScale
	cfg.Numerical.MaxDT *= dtScale

	cfg.Numerical.SubstepMom = subMom
	cfg.Numerical.SubstepVOF = subVOF
	if cflFrac > 0 {
		cfg.Numerical.MaxCFLFrac = cflFrac
	}

	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0

	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	res := RunResult{
		Name:       name,
		ClipRedist: cfg.Numerical.ClipRedistribute,
		InitialVol: sim.State.Volume,
	}

	os.MkdirAll("out/study", 0755)
	fCsv, err := os.Create(fmt.Sprintf("out/study/%s.csv", name))
	if err != nil {
		log.Fatal(err)
	}
	defer fCsv.Close()

	advStr := "VanLeer"
	wallStr := "FreeSlip"
	fmt.Fprintf(fCsv, "# commit=%s dirty=%v domain=15L0x4L0 cellsPerL0=%d advection=%s wall=%s density_ratio=1000 t*=t_sqrt(2g/L0) dtScale=%.4f subMom=%d subVOF=%d cflFrac=%.4f\n",
		gitHash, gitDirty, cellsPerL0, advStr, wallStr, dtScale, subMom, subVOF, cflFrac)
	fmt.Fprintf(fCsv, "t_star,X_star_05\n")

	sumIter := 0
	countIter := 0

	for {
		sim.Step(0)
		t_star := sim.State.TStar

		if sim.State.FrontXStar > 0 {
			res.Z_sim = append(res.Z_sim, sim.State.FrontXStar)
			res.T_sim = append(res.T_sim, t_star)
			fmt.Fprintf(fCsv, "%.6f,%.6f\n", t_star, sim.State.FrontXStar)
		}

		pIter := sim.State.PoissonIter
		sumIter += pIter
		countIter++
		if pIter > res.MaxPoissonIter {
			res.MaxPoissonIter = pIter
		}
		if sim.State.CFL > res.MaxCFL {
			res.MaxCFL = sim.State.CFL
		}
		if sim.State.MaxDiv > res.MaxDiv {
			res.MaxDiv = sim.State.MaxDiv
		}
		drift := math.Abs(sim.State.VolumeDrift) / res.InitialVol
		if drift > res.MaxVolDrift {
			res.MaxVolDrift = drift
		}
		if res.FirstDT == 0 {
			res.FirstDT = sim.State.DT
		}

		if sim.State.FrontXStar >= 14.5 || t_star >= 12.0 {
			break
		}
	}

	res.WallTime = time.Since(start)
	res.Steps = sim.State.Step
	res.Warnings = sim.State.CFLWarnings
	res.MeanPoissonIter = float64(sumIter) / float64(countIter)
	res.TotalPoisson = sumIter

	res.VolSweepX = sim.State.VolSweepX
	res.VolSweepY = sim.State.VolSweepY
	res.VolClip = sim.State.VolClip
	res.TopOutflow = sim.State.TopOutflow
	res.FinalDrift = sim.State.VolumeDrift

	fmt.Fprintf(os.Stderr, "Finished %s in %v (%d steps, %d Poisson solves, mean %.1f iter)\n",
		name, res.WallTime, res.Steps, res.TotalPoisson, res.MeanPoissonIter)
	return res
}

func main() {
	b, err := ioutil.ReadFile("benchmark/martin_moyce.json")
	if err != nil {
		log.Fatal(err)
	}
	var data BenchmarkData
	json.Unmarshal(b, &data)

	gitHash, gitDirty := getGitInfo()
	fmt.Printf("=== COMPREHENSIVE STUDY ===\n")
	fmt.Printf("Git: commit=%s dirty=%v\n\n", gitHash, gitDirty)

	anchorZ := data.TimeNormalisation.AnchorZ
	anchorT := data.TimeNormalisation.AnchorT

	targetZs := []float64{3, 5, 7, 10, 14}

	// Helper: print T_align at target Zs
	printSeries := func(r RunResult) {
		if len(r.Z_sim) == 0 {
			fmt.Printf("  [no data]\n")
			return
		}
		alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, anchorZ, anchorT)
		tRawAnchor := benchmark.InterpTSim(r.Z_sim, r.T_sim, anchorZ)
		fmt.Printf("  Anchor T(Z=1.44)_raw = %.4f  dt_first = %.6e  maxCFL = %.4f  steps = %d\n",
			tRawAnchor, r.FirstDT, r.MaxCFL, r.Steps)
		fmt.Printf("  Poisson: total=%d mean=%.1f max=%d\n", r.TotalPoisson, r.MeanPoissonIter, r.MaxPoissonIter)
		for _, z := range targetZs {
			t_align := benchmark.InterpTSim(r.Z_sim, alignedT, z)
			fmt.Printf("    Z=%5.1f: T_align=%.4f\n", z, t_align)
		}
	}

	// ========== PART 2: TIME-STEP STUDY ==========
	fmt.Println("========== PART 2: TIME-STEP STUDY ==========")

	type dtCase struct {
		name    string
		N       int
		dtScale float64
	}
	dtCases := []dtCase{
		{"N8_dt1", 8, 1.0},
		{"N8_dt2", 8, 0.5},
		{"N8_dt4", 8, 0.25},
		{"N8_dt8", 8, 0.125},
		{"N16_dt1", 16, 1.0},
		{"N16_dt2", 16, 0.5},
		{"N16_dt4", 16, 0.25},
		{"N16_dt8", 16, 0.125},
		// N32 dt/2 only if feasible
		{"N32_dt1", 32, 1.0},
		{"N32_dt2", 32, 0.5},
		{"N32_dt4", 32, 0.25},
	}

	dtResults := make(map[string]RunResult)
	for _, c := range dtCases {
		// skip N32 dt/4 if N32 dt/2 takes > 25min (we just run all and report)
		r := runSim(c.name, c.N, c.dtScale, 1, 1, 0, gitHash, gitDirty)
		dtResults[c.name] = r
		fmt.Printf("\n%s (WallTime: %v)\n", c.name, r.WallTime)
		printSeries(r)
	}

	// Print convergence table
	fmt.Println("\n--- dt Convergence Table (T_align) ---")
	for _, N := range []int{8, 16, 32} {
		suffixes := []string{"dt1", "dt2", "dt4", "dt8"}
		if N == 32 {
			suffixes = []string{"dt1", "dt2", "dt4"}
		}
		fmt.Printf("\nN=%d:\n", N)
		fmt.Printf("  %8s", "Z")
		for _, s := range suffixes {
			fmt.Printf(" %12s", s)
		}
		fmt.Println()
		for _, z := range targetZs {
			fmt.Printf("  %8.1f", z)
			for _, s := range suffixes {
				key := fmt.Sprintf("N%d_%s", N, s)
				r := dtResults[key]
				alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, anchorZ, anchorT)
				t := benchmark.InterpTSim(r.Z_sim, alignedT, z)
				fmt.Printf(" %12.4f", t)
			}
			fmt.Println()
		}
		// Successive differences
		fmt.Printf("  Successive diffs (dt1->dt2, dt2->dt4, ...):\n")
		for _, z := range targetZs {
			fmt.Printf("  %8.1f", z)
			prev := math.NaN()
			for _, s := range suffixes {
				key := fmt.Sprintf("N%d_%s", N, s)
				r := dtResults[key]
				alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, anchorZ, anchorT)
				t := benchmark.InterpTSim(r.Z_sim, alignedT, z)
				if !math.IsNaN(prev) {
					fmt.Printf(" %+12.4f", t-prev)
				} else {
					fmt.Printf(" %12s", "-")
				}
				prev = t
			}
			fmt.Println()
		}
	}

	// ========== PART 3: SPACE STUDY at dt/4 ==========
	fmt.Println("\n========== PART 3: SPACE STUDY at dt/4 ==========")

	spaceNs := []int{8, 12, 16, 24, 32}
	spaceResults := make(map[int]RunResult)
	for _, N := range spaceNs {
		key := fmt.Sprintf("N%d_dt4", N)
		if r, ok := dtResults[key]; ok {
			spaceResults[N] = r
		} else {
			r := runSim(key, N, 0.25, 1, 1, 0, gitHash, gitDirty)
			spaceResults[N] = r
			fmt.Printf("\n%s (WallTime: %v)\n", key, r.WallTime)
			printSeries(r)
		}
	}

	fmt.Println("\n--- Space Convergence Table at dt/4 (T_align) ---")
	fmt.Printf("  %8s", "Z")
	for _, N := range spaceNs {
		fmt.Printf(" %12s", fmt.Sprintf("N=%d", N))
	}
	fmt.Println()
	for _, z := range targetZs {
		fmt.Printf("  %8.1f", z)
		for _, N := range spaceNs {
			r := spaceResults[N]
			alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, anchorZ, anchorT)
			t := benchmark.InterpTSim(r.Z_sim, alignedT, z)
			fmt.Printf(" %12.4f", t)
		}
		fmt.Println()
	}
	fmt.Printf("  Successive diffs:\n")
	for _, z := range targetZs {
		fmt.Printf("  %8.1f", z)
		prev := math.NaN()
		for _, N := range spaceNs {
			r := spaceResults[N]
			alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, anchorZ, anchorT)
			t := benchmark.InterpTSim(r.Z_sim, alignedT, z)
			if !math.IsNaN(prev) {
				fmt.Printf(" %+12.4f", t-prev)
			} else {
				fmt.Printf(" %12s", "-")
			}
			prev = t
		}
		fmt.Println()
	}

	// ========== PART 4: SUBSTEPPING AND CFL CAP ==========
	fmt.Println("\n========== PART 4: SUBSTEPPING / CFL CAP COST ==========")

	p4runs := []struct {
		name    string
		dtScale float64
		subMom  int
		subVOF  int
		cflFrac float64
	}{
		{"N16_native", 1.0, 1, 1, 0},
		{"N16_dt4_global", 0.25, 1, 1, 0},
		{"N16_subMom4", 1.0, 4, 1, 0},
		{"N16_subVOF4", 1.0, 1, 4, 0},
		{"N16_cflcap_1.0", 1.0, 1, 1, 1.0},  // same as native
		{"N16_cflcap_0.25", 1.0, 1, 1, 0.25}, // same as dt/4
	}

	for _, c := range p4runs {
		r := runSim(c.name, 16, c.dtScale, c.subMom, c.subVOF, c.cflFrac, gitHash, gitDirty)
		fmt.Printf("\n%s (WallTime: %v)\n", c.name, r.WallTime)
		printSeries(r)
	}

	// ========== PART 6: DRIFT DECOMPOSITION ==========
	fmt.Println("\n========== PART 6: DRIFT DECOMPOSITION ==========")

	// Run N16 native with ClipRedistribute=true (default)
	driftRun := dtResults["N16_dt1"]
	// Compute cell area for drift decomposition
	L0 := 0.05715
	width := 15.0 * L0
	nx := int(math.Round(width / L0 * 16.0))
	dx := width / float64(nx)
	dy := (4.0 * L0) / float64(int(math.Round(4.0*L0/(L0/16.0))))
	cellArea := dx * dy
	initCells := driftRun.InitialVol / cellArea

	fmt.Printf("N16 native (ClipRedistribute=%v):\n", driftRun.ClipRedist)
	fmt.Printf("  InitialVol      = %.6e m^2 (%.2f cells)\n", driftRun.InitialVol, initCells)
	fmt.Printf("  VolSweepX       = %.6e m^2 (%.4f cells)\n", driftRun.VolSweepX*cellArea, driftRun.VolSweepX)
	fmt.Printf("  VolSweepY       = %.6e m^2 (%.4f cells)\n", driftRun.VolSweepY*cellArea, driftRun.VolSweepY)
	fmt.Printf("  VolClip         = %.6e m^2 (%.6f cells)\n", driftRun.VolClip*cellArea, driftRun.VolClip)
	fmt.Printf("  TopOutflow      = %.6e m^2 (%.6f cells)\n", driftRun.TopOutflow*cellArea, driftRun.TopOutflow)
	fmt.Printf("  FinalDrift      = %.6e m^2 (%.6f cells, %.6f%%)\n",
		driftRun.FinalDrift, driftRun.FinalDrift/cellArea,
		100.0*driftRun.FinalDrift/driftRun.InitialVol)
	fmt.Printf("  Sweep imbalance = sweepX+sweepY = %.6f cells\n", driftRun.VolSweepX+driftRun.VolSweepY)

	// Also run with ClipRedistribute=false to understand the 3e-13 case
	fmt.Println("\nRunning N16 native with ClipRedistribute=false for comparison...")
	noRedistRun := func() RunResult {
		start := time.Now()
		cfgLocal := solver.DefaultConfig()
		cfgLocal.Domain.L0 = L0
		cfgLocal.Domain.H0 = 2.0 * L0
		cfgLocal.Domain.Width = width
		cfgLocal.Domain.Height = 4.0 * L0
		cfgLocal.Domain.Nx = nx
		cfgLocal.Domain.Ny = int(math.Round(4.0 * L0 / (L0 / 16.0)))
		cfgLocal.Physical.RhoW = 1000.0
		cfgLocal.Physical.RhoA = 1.0
		cfgLocal.Numerical.PoissonTol = 1e-6
		cfgLocal.Numerical.FreeSlip = true
		cfgLocal.Numerical.SecondOrderAdvect = true
		cfgLocal.Numerical.SplitDivFix = true
		cfgLocal.Numerical.ClipRedistribute = false
		cfgLocal.TimeScale = solver.TimeScaleSqrt2gOverL0

		sim := solver.NewSimulation(cfgLocal, cfgLocal.Domain.Nx, cfgLocal.Domain.Ny, width, 4.0*L0, false)
		sim.InitDamBreak()

		for sim.State.TStar < 12.0 && sim.State.FrontXStar < 14.5 {
			sim.Step(0)
		}
		return RunResult{
			WallTime:   time.Since(start),
			Steps:      sim.State.Step,
			VolSweepX:  sim.State.VolSweepX,
			VolSweepY:  sim.State.VolSweepY,
			VolClip:    sim.State.VolClip,
			TopOutflow: sim.State.TopOutflow,
			FinalDrift: sim.State.VolumeDrift,
			InitialVol: sim.State.Volume + sim.State.VolumeDrift, // approximate initial
			ClipRedist: false,
		}
	}()
	initCells2 := noRedistRun.InitialVol / cellArea
	fmt.Printf("\nN16 native (ClipRedistribute=false):\n")
	fmt.Printf("  InitialVol      = %.6e m^2 (%.2f cells)\n", noRedistRun.InitialVol, initCells2)
	fmt.Printf("  VolClip         = %.6f cells\n", noRedistRun.VolClip)
	fmt.Printf("  TopOutflow      = %.6f cells\n", noRedistRun.TopOutflow)
	fmt.Printf("  FinalDrift      = %.6e m^2 (%.6f cells, %.6f%%)\n",
		noRedistRun.FinalDrift, noRedistRun.FinalDrift/cellArea,
		100.0*noRedistRun.FinalDrift/noRedistRun.InitialVol)
	fmt.Printf("  Sweep imbalance = %.6f cells\n", noRedistRun.VolSweepX+noRedistRun.VolSweepY)

	// ========== PART 7: BENCHCOMPARE ==========
	fmt.Println("\n========== PART 7: BENCHCOMPARE per-Z ==========")

	// Compute band min/max from individual runs
	computeBand := func(records []struct {
		Label string    `json:"label"`
		Z     []float64 `json:"Z"`
		T     []float64 `json:"T"`
	}, zTarget float64) (float64, float64) {
		minT := math.Inf(1)
		maxT := math.Inf(-1)
		for _, r := range records {
			t := benchmark.InterpTSim(r.Z, r.T, zTarget)
			if !math.IsNaN(t) {
				if t < minT {
					minT = t
				}
				if t > maxT {
					maxT = t
				}
			}
		}
		return minT, maxT
	}

	benchRuns := []struct {
		label string
		key   string
	}{
		{"VL FS N=16 dt/4", "N16_dt4"},
		{"VL FS N=32 dt/4", "N32_dt4"},
	}

	Z_exp := data.Scales.A2p25In.Mean.Z
	T_exp := data.Scales.A2p25In.Mean.T

	for _, br := range benchRuns {
		r := spaceResults[16]
		if br.key == "N32_dt4" {
			r = spaceResults[32]
		}
		if br.key == "N16_dt4" {
			r = spaceResults[16]
		}

		alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, anchorZ, anchorT)

		fmt.Printf("\n--- %s vs a=2.25in (all 25 Z values) ---\n", br.label)
		fmt.Printf("  %6s %6s %8s %8s %8s %8s %8s %6s\n",
			"Z", "T_exp", "BandMin", "BandMax", "T_raw", "T_align", "dT/T%", "InBand")

		var rmsRaw7, rmsAlign7 float64
		var maxRaw7, maxAlign7 float64
		var c7 int
		var rmsRaw14, rmsAlign14 float64
		var maxRaw14, maxAlign14 float64
		var c14 int

		for i, z := range Z_exp {
			if z > r.Z_sim[len(r.Z_sim)-1] {
				continue
			}
			bandMin, bandMax := computeBand(data.Scales.A2p25In.Records, z)
			t_raw := benchmark.InterpTSim(r.Z_sim, r.T_sim, z)
			t_align := benchmark.InterpTSim(r.Z_sim, alignedT, z)

			pctRaw := 100 * (t_raw - T_exp[i]) / T_exp[i]
			pctAlign := 100 * (t_align - T_exp[i]) / T_exp[i]

			inBand := "NO"
			if t_align >= bandMin && t_align <= bandMax {
				inBand = "YES"
			}

			fmt.Printf("  %6.2f %6.2f %8.2f %8.2f %8.3f %8.3f %+7.1f%% %6s\n",
				z, T_exp[i], bandMin, bandMax, t_raw, t_align, pctAlign, inBand)

			if z >= 1.44 && z <= 7.0 {
				rmsRaw7 += pctRaw * pctRaw
				rmsAlign7 += pctAlign * pctAlign
				c7++
				if math.Abs(pctRaw) > maxRaw7 {
					maxRaw7 = math.Abs(pctRaw)
				}
				if math.Abs(pctAlign) > maxAlign7 {
					maxAlign7 = math.Abs(pctAlign)
				}
			}
			if z >= 1.44 && z <= 14.0 {
				rmsRaw14 += pctRaw * pctRaw
				rmsAlign14 += pctAlign * pctAlign
				c14++
				if math.Abs(pctRaw) > maxRaw14 {
					maxRaw14 = math.Abs(pctRaw)
				}
				if math.Abs(pctAlign) > maxAlign14 {
					maxAlign14 = math.Abs(pctAlign)
				}
			}
		}
		if c7 > 0 {
			fmt.Printf("  -> [1.44, 7]  RMS_raw=%.1f%% Max_raw=%.1f%% | RMS_align=%.1f%% Max_align=%.1f%%\n",
				math.Sqrt(rmsRaw7/float64(c7)), maxRaw7, math.Sqrt(rmsAlign7/float64(c7)), maxAlign7)
		}
		if c14 > 0 {
			fmt.Printf("  -> [1.44, 14] RMS_raw=%.1f%% Max_raw=%.1f%% | RMS_align=%.1f%% Max_align=%.1f%%\n",
				math.Sqrt(rmsRaw14/float64(c14)), maxRaw14, math.Sqrt(rmsAlign14/float64(c14)), maxAlign14)
		}
	}

	// a=1.125 range info
	z1125 := data.Scales.A1p125In.Mean.Z
	fmt.Printf("\na=1.125in data Z range: [%.2f, %.2f] (%d points)\n", z1125[0], z1125[len(z1125)-1], len(z1125))
	fmt.Println("Because the a=1.125in data only goes to Z=6.76 (< 7), [1.44,7] and [1.44,14] bands are identical.")

	fmt.Println("\n=== STUDY COMPLETE ===")
}
