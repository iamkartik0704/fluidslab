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

type WindowStat struct {
	SumIter   int
	CountIter int
	MaxIter   int
}

type RunResult struct {
	Name string
	WallTime time.Duration
	Steps int
	MaxVolDrift float64
	Warnings int
	MaxDiv float64
	FinalDiv float64
	MeanPoissonIter float64
	MaxPoissonIter int
	
	Windows [4]WindowStat // 0-2, 2-4, 4-8, 8-12
	
	Z_sim []float64
	T_sim []float64
	Z_01  []float64
	Z_001 []float64
	CFLMax float64
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

func runSimAttr(name string, cellsPerL0 int, freeSlip, vanLeer bool, dtScale float64, subMom, subVOF int, rhoRatio float64, gitHash string, gitDirty bool) RunResult {
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
	cfg.Physical.RhoA = 1000.0 / rhoRatio
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.SplitDivFix = true
	cfg.Numerical.ClipRedistribute = true
	
	cfg.Numerical.CFL *= dtScale
	cfg.Numerical.ViscousCFL *= dtScale
	cfg.Numerical.GravityCFL *= dtScale
	cfg.Numerical.MaxDT *= dtScale

	cfg.Numerical.SubstepMom = subMom
	cfg.Numerical.SubstepVOF = subVOF

	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0 
	
	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()
	
	res := RunResult{
		Name: name,
	}
	
	initialVol := sim.State.Volume
	maxVolDrift := 0.0
	sumIter := 0
	countIter := 0

	os.MkdirAll("out/finalpass", 0755)
	fCsv, err := os.Create(fmt.Sprintf("out/finalpass/%s.csv", name))
	if err != nil {
		log.Fatal(err)
	}
	defer fCsv.Close()

	advStr := "FirstOrder"
	if vanLeer { advStr = "VanLeer" }
	wallStr := "NoSlip"
	if freeSlip { wallStr = "FreeSlip" }

	fmt.Fprintf(fCsv, "# commit=%s dirty=%v domain=15L0x4L0 cellsPerL0=%d advection=%s wall=%s density_ratio=%.1f t*=t_sqrt(2g/L0)\n",
		gitHash, gitDirty, cellsPerL0, advStr, wallStr, rhoRatio)
	fmt.Fprintf(fCsv, "t_star,X_star_05,X_star_01,X_star_001\n")

	for {
		sim.Step(0)
		
		t_star := sim.State.TStar
		
		if sim.State.FrontXStar > 0 {
			res.Z_sim = append(res.Z_sim, sim.State.FrontXStar)
			res.T_sim = append(res.T_sim, t_star)
			res.Z_01 = append(res.Z_01, sim.State.FrontXStar01)
			res.Z_001 = append(res.Z_001, sim.State.FrontXStar001)
			fmt.Fprintf(fCsv, "%.5f,%.5f,%.5f,%.5f\n", t_star, sim.State.FrontXStar, sim.State.FrontXStar01, sim.State.FrontXStar001)
		}

		driftFrac := math.Abs(sim.State.VolumeDrift) / initialVol
		if driftFrac > maxVolDrift {
			maxVolDrift = driftFrac
		}
		if sim.State.MaxDiv > res.MaxDiv { res.MaxDiv = sim.State.MaxDiv }
		res.FinalDiv = sim.State.MaxDiv // updates every step
		
		pIter := sim.State.PoissonIter
		sumIter += pIter
		countIter++
		if pIter > res.MaxPoissonIter {
			res.MaxPoissonIter = pIter
		}
		
		// Window stats: 0-2, 2-4, 4-8, 8-12
		wIdx := -1
		if t_star <= 2.0 { wIdx = 0 } else if t_star <= 4.0 { wIdx = 1 } else if t_star <= 8.0 { wIdx = 2 } else if t_star <= 12.0 { wIdx = 3 }
		if wIdx >= 0 {
			res.Windows[wIdx].SumIter += pIter
			res.Windows[wIdx].CountIter++
			if pIter > res.Windows[wIdx].MaxIter {
				res.Windows[wIdx].MaxIter = pIter
			}
		}

		if sim.State.CFL > res.CFLMax {
			res.CFLMax = sim.State.CFL
		}

		if sim.State.FrontXStar >= 14.0 || t_star >= 12.0 {
			break
		}
	}
	
	res.WallTime = time.Since(start)
	res.Steps = sim.State.Step
	res.MaxVolDrift = maxVolDrift
	res.Warnings = sim.State.CFLWarnings
	res.MeanPoissonIter = float64(sumIter) / float64(countIter)
	
	fmt.Printf("Finished %s in %v\n", name, res.WallTime)
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
	fmt.Printf("Git State: commit=%s dirty=%v\n\n", gitHash, gitDirty)

	// 1. Matrix cases
	runs := []RunResult{
		// Native dt: N=8, 16, 32
		runSimAttr("N8_FStrue_VLtrue", 8, true, true, 1.0, 1, 1, 1000.0, gitHash, gitDirty),
		runSimAttr("N16_FStrue_VLtrue", 16, true, true, 1.0, 1, 1, 1000.0, gitHash, gitDirty),
		runSimAttr("N32_FStrue_VLtrue", 32, true, true, 1.0, 1, 1, 1000.0, gitHash, gitDirty),
		// dt/4: N=8, 16, 32
		runSimAttr("N8_FStrue_VLtrue_dt4", 8, true, true, 0.25, 1, 1, 1000.0, gitHash, gitDirty),
		runSimAttr("N16_FStrue_VLtrue_dt4", 16, true, true, 0.25, 1, 1, 1000.0, gitHash, gitDirty),
		runSimAttr("N32_FStrue_VLtrue_dt4", 32, true, true, 0.25, 1, 1, 1000.0, gitHash, gitDirty),
		// Substepped
		runSimAttr("N16_FStrue_VLtrue_subVOF4", 16, true, true, 1.0, 1, 4, 1000.0, gitHash, gitDirty),
		runSimAttr("N16_FStrue_VLtrue_subMom4", 16, true, true, 1.0, 4, 1, 1000.0, gitHash, gitDirty),
		// Reference
		runSimAttr("N16_FStrue_VLfalse", 16, true, false, 1.0, 1, 1, 1000.0, gitHash, gitDirty),
		runSimAttr("N16_FSfalse_VLtrue", 16, false, true, 1.0, 1, 1, 1000.0, gitHash, gitDirty),
	}
	
	fmt.Println("\n--- Convergence Series (T_sim at Z=1.44, 3, 5, 7, 10, 14) ---")
	targetZs := []float64{1.44, 3, 5, 7, 10, 14}
	for _, r := range runs {
		fmt.Printf("%s (WallTime: %v)\n", r.Name, r.WallTime)
		alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, data.TimeNormalisation.AnchorZ, data.TimeNormalisation.AnchorT)
		for _, z := range targetZs {
			t_sim := benchmark.InterpTSim(r.Z_sim, r.T_sim, z)
			t_sim_aligned := benchmark.InterpTSim(r.Z_sim, alignedT, z)
			fmt.Printf("  Z=%.2f: T_sim_raw=%.3f T_sim_aligned=%.3f\n", z, t_sim, t_sim_aligned)
		}
	}
	
	fmt.Println("\n--- Full Benchcompare ---")
	benchConfigs := []int{1, 2, 4, 8, 9} // indices of interesting runs (N16, N32 native, N16 dt4, N16 FO FS, N16 VL NS)
	
	for _, confIdx := range benchConfigs {
		r := runs[confIdx]
		fmt.Printf("\nBenchcompare for %s:\n", r.Name)
		alignedT := benchmark.AlignTime(r.Z_sim, r.T_sim, data.TimeNormalisation.AnchorZ, data.TimeNormalisation.AnchorT)
		
		for _, aType := range []string{"a=2.25", "a=1.125"} {
			fmt.Printf("  [Data: %s]\n", aType)
			
			var Z_exp, T_exp []float64
			if aType == "a=2.25" {
				Z_exp = data.Scales.A2p25In.Mean.Z
				T_exp = data.Scales.A2p25In.Mean.T
			} else {
				Z_exp = data.Scales.A1p125In.Mean.Z
				T_exp = data.Scales.A1p125In.Mean.T
			}
			
			var rmsRaw7, rmsAligned7 float64
			var maxRaw7, maxAligned7 float64
			var countRMS7 int
			
			var rmsRaw14, rmsAligned14 float64
			var maxRaw14, maxAligned14 float64
			var countRMS14 int
			
			for i, z := range Z_exp {
				if z < 1.44 || z > r.Z_sim[len(r.Z_sim)-1] { continue }
				t_sim := benchmark.InterpTSim(r.Z_sim, r.T_sim, z)
				t_sim_aligned := benchmark.InterpTSim(r.Z_sim, alignedT, z)
				dT := t_sim - T_exp[i]
				dT_aligned := t_sim_aligned - T_exp[i]
				
				pctRaw := 100 * dT / T_exp[i]
				pctAligned := 100 * dT_aligned / T_exp[i]
				
				var inBand string
				if pctAligned >= -5.0 && pctAligned <= 5.0 {
					inBand = "IN_BAND"
				} else {
					inBand = "OUT"
				}
				
				fmt.Printf("    Z=%.2f: T_exp=%.2f T_raw=%.2f T_aligned=%.2f dT/T_raw=%+5.1f%% dT/T_aligned=%+5.1f%% [%s]\n", 
					z, T_exp[i], t_sim, t_sim_aligned, pctRaw, pctAligned, inBand)
					
				if z <= 7.0 {
					rmsRaw7 += pctRaw * pctRaw
					rmsAligned7 += pctAligned * pctAligned
					countRMS7++
					if math.Abs(pctRaw) > maxRaw7 { maxRaw7 = math.Abs(pctRaw) }
					if math.Abs(pctAligned) > maxAligned7 { maxAligned7 = math.Abs(pctAligned) }
				}
				if z <= 14.0 {
					rmsRaw14 += pctRaw * pctRaw
					rmsAligned14 += pctAligned * pctAligned
					countRMS14++
					if math.Abs(pctRaw) > maxRaw14 { maxRaw14 = math.Abs(pctRaw) }
					if math.Abs(pctAligned) > maxAligned14 { maxAligned14 = math.Abs(pctAligned) }
				}
			}
			if countRMS7 > 0 {
				fmt.Printf("    -> [1.44, 7]  RMS_raw=%.1f%% Max_raw=%.1f%% | RMS_aligned=%.1f%% Max_aligned=%.1f%%\n", 
					math.Sqrt(rmsRaw7/float64(countRMS7)), maxRaw7, math.Sqrt(rmsAligned7/float64(countRMS7)), maxAligned7)
			}
			if countRMS14 > 0 {
				fmt.Printf("    -> [1.44, 14] RMS_raw=%.1f%% Max_raw=%.1f%% | RMS_aligned=%.1f%% Max_aligned=%.1f%%\n", 
					math.Sqrt(rmsRaw14/float64(countRMS14)), maxRaw14, math.Sqrt(rmsAligned14/float64(countRMS14)), maxAligned14)
			}
		}
	}
}
