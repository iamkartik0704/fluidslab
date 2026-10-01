package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"time"

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
	} `json:"scales"`
}

type RunResult struct {
	Name string
	WallTime time.Duration
	Steps int
	MaxVolDrift float64
	Warnings int
	MaxDiv float64
	MeanPoissonIter float64
	MaxPoissonIter int
	
	Z_sim []float64
	T_sim []float64
	Z_01  []float64
	Z_001 []float64
	CFLMax float64
}

func interpTSim(Z_sim, T_sim []float64, Z_target float64) float64 {
	for i := 0; i < len(Z_sim)-1; i++ {
		if Z_target >= Z_sim[i] && Z_target <= Z_sim[i+1] {
			if Z_sim[i+1] == Z_sim[i] {
				return T_sim[i]
			}
			t := (Z_target - Z_sim[i]) / (Z_sim[i+1] - Z_sim[i])
			return T_sim[i] + t*(T_sim[i+1]-T_sim[i])
		}
	}
	return math.NaN()
}

func alignTime(Z_sim, T_sim []float64, anchorZ, anchorT float64) []float64 {
	tAtAnchor := interpTSim(Z_sim, T_sim, anchorZ)
	shift := anchorT - tAtAnchor
	alignedT := make([]float64, len(T_sim))
	for i, t := range T_sim {
		alignedT[i] = t + shift
	}
	return alignedT
}

func runSimAttr(name string, cellsPerL0 int, freeSlip, vanLeer bool, dtScale float64, rhoRatio float64) RunResult {
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

	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0 
	
	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()
	
	res := RunResult{
		Name: name,
	}
	
	maxVolDrift := 0.0
	sumIter := 0
	countIter := 0

	for {
		sim.Step(0)
		
		t_star := sim.State.TStar
		
		if sim.State.FrontXStar > 0 {
			res.Z_sim = append(res.Z_sim, sim.State.FrontXStar)
			res.T_sim = append(res.T_sim, t_star)
			res.Z_01 = append(res.Z_01, sim.State.FrontXStar01)
			res.Z_001 = append(res.Z_001, sim.State.FrontXStar001)
		}

		if math.Abs(sim.State.VolumeDrift) > maxVolDrift {
			maxVolDrift = math.Abs(sim.State.VolumeDrift)
		}
		if sim.State.MaxDiv > res.MaxDiv { res.MaxDiv = sim.State.MaxDiv }
		
		sumIter += sim.State.PoissonIter
		countIter++
		if sim.State.PoissonIter > res.MaxPoissonIter {
			res.MaxPoissonIter = sim.State.PoissonIter
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
	
	Z_exp := data.Scales.A2p25In.Mean.Z
	T_exp := data.Scales.A2p25In.Mean.T

	// 1. Matrix cases
	runs := []RunResult{
		runSimAttr("N32_FSfalse_VLfalse", 32, false, false, 1.0, 1000.0),
		runSimAttr("N32_FSfalse_VLtrue", 32, false, true, 1.0, 1000.0),
		runSimAttr("N32_FStrue_VLfalse", 32, true, false, 1.0, 1000.0),
		runSimAttr("N32_FStrue_VLtrue", 32, true, true, 1.0, 1000.0),
		// 4. Convergence series (need N=8, N=16 as well)
		runSimAttr("N8_FStrue_VLfalse", 8, true, false, 1.0, 1000.0),
		runSimAttr("N16_FStrue_VLfalse", 16, true, false, 1.0, 1000.0),
		runSimAttr("N8_FStrue_VLtrue", 8, true, true, 1.0, 1000.0),
		runSimAttr("N16_FStrue_VLtrue", 16, true, true, 1.0, 1000.0),
		// 5. Time-step series (dt=1/2, 1/4, 1/8) for 16, FS, VL
		runSimAttr("N16_FStrue_VLtrue_dt2", 16, true, true, 0.5, 1000.0),
		runSimAttr("N16_FStrue_VLtrue_dt4", 16, true, true, 0.25, 1000.0),
		runSimAttr("N16_FStrue_VLtrue_dt8", 16, true, true, 0.125, 1000.0),
	}
	
	fmt.Println("\n--- Convergence Series (T_sim at Z=3,5,7,10,14) ---")
	targetZs := []float64{3, 5, 7, 10, 14}
	for _, r := range runs {
		fmt.Printf("%s\n", r.Name)
		alignedT := alignTime(r.Z_sim, r.T_sim, data.TimeNormalisation.AnchorZ, data.TimeNormalisation.AnchorT)
		for _, z := range targetZs {
			t_sim := interpTSim(r.Z_sim, r.T_sim, z)
			t_sim_aligned := interpTSim(r.Z_sim, alignedT, z)
			fmt.Printf("  Z=%.1f: T_sim_raw=%.3f T_sim_aligned=%.3f\n", z, t_sim, t_sim_aligned)
		}
	}
	
	fmt.Println("\n--- Time-step Series CFL Max ---")
	for _, r := range runs {
		fmt.Printf("%s: CFLMax=%.3f\n", r.Name, r.CFLMax)
	}
	
	fmt.Println("\n--- Matrix Stats ---")
	for _, r := range runs {
		fmt.Printf("%s: WallTime=%v Steps=%d MaxDiv=%.2e MeanIter=%.1f MaxVolDrift=%.2e\n", 
			r.Name, r.WallTime, r.Steps, r.MaxDiv, r.MeanPoissonIter, r.MaxVolDrift)
	}

	// Full benchcompare for N16 and N32 cases
	fmt.Println("\n--- Full Benchcompare ---")
	for _, r := range runs {
		fmt.Printf("%s\n", r.Name)
		alignedT := alignTime(r.Z_sim, r.T_sim, data.TimeNormalisation.AnchorZ, data.TimeNormalisation.AnchorT)
		for i, z := range Z_exp {
			if z < 1.44 || z > r.Z_sim[len(r.Z_sim)-1] { continue }
			t_sim := interpTSim(r.Z_sim, r.T_sim, z)
			t_sim_aligned := interpTSim(r.Z_sim, alignedT, z)
			fmt.Printf("  Z=%.2f: T_exp=%.2f T_raw=%.2f T_aligned=%.2f dT/T_raw=%.1f%%\n", 
				z, T_exp[i], t_sim, t_sim_aligned, 100*(t_sim-T_exp[i])/T_exp[i])
		}
	}
	
	// Attribution requires density ratio 100 runs
	r_dens_FO := runSimAttr("N16_FStrue_VLfalse_rho100", 16, true, false, 1.0, 100.0)
	r_dens_VL := runSimAttr("N16_FStrue_VLtrue_rho100", 16, true, true, 1.0, 100.0)
	
	attrRuns := append(runs, r_dens_FO, r_dens_VL)
	
	fmt.Println("\n--- Revised Attribution Table (T_sim at Z=3, 5, 7, 10) ---")
	targetZsAttr := []float64{3, 5, 7, 10}
	for _, r := range attrRuns {
		fmt.Printf("%s\n", r.Name)
		alignedT := alignTime(r.Z_sim, r.T_sim, data.TimeNormalisation.AnchorZ, data.TimeNormalisation.AnchorT)
		for _, z := range targetZsAttr {
			t_sim := interpTSim(r.Z_sim, r.T_sim, z)
			t_sim_aligned := interpTSim(r.Z_sim, alignedT, z)
			t_sim_01 := interpTSim(r.Z_01, r.T_sim, z)
			fmt.Printf("  Z=%.1f: T_raw=%.3f T_aligned=%.3f T_raw(0.1)=%.3f\n", z, t_sim, t_sim_aligned, t_sim_01)
		}
	}
}
