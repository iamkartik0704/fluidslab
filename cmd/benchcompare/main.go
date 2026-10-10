package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"time"

	"dambreak/internal/solver"
)

type BenchmarkData struct {
	Verified         bool   `json:"verified"`
	Source           string `json:"source"`
	TimeNormalisation struct {
		AnchorZ float64 `json:"anchor_Z"`
		AnchorT float64 `json:"anchor_T"`
	} `json:"time_normalisation"`
	Scales struct {
		A1p125In struct {
			Mean struct {
				Z []float64 `json:"Z"`
				T []float64 `json:"T"`
			} `json:"mean"`
		} `json:"a_1p125_in"`
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
	} `json:"scales"`
}

func loadBenchmark(path string) (*BenchmarkData, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data BenchmarkData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	
	// Validate
	for _, rec := range data.Scales.A2p25In.Records {
		if rec.Z[0] < 1.0 {
			return nil, fmt.Errorf("Z[0] < 1.0 in %s", rec.Label)
		}
		if len(rec.Z) != len(rec.T) {
			return nil, fmt.Errorf("len(Z) != len(T) in %s", rec.Label)
		}
		for i := 1; i < len(rec.Z); i++ {
			if rec.Z[i] <= rec.Z[i-1] {
				return nil, fmt.Errorf("Z not strictly increasing in %s", rec.Label)
			}
			if rec.T[i] < rec.T[i-1] {
				return nil, fmt.Errorf("T not increasing in %s", rec.Label)
			}
		}
	}
	
	return &data, nil
}

// Invert and interpolate
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

type RunResult struct {
	Name string
	WallTime time.Duration
	Steps int
	MaxVolDrift float64
	AlphaMin float64
	AlphaMax float64
	Warnings int
	MaxDiv float64
	MeanPoissonIter float64
	MaxPoissonIter int
	
	Z_sim []float64
	T_sim []float64
	Z_01  []float64
	Z_001 []float64
}

func runSim(cellsPerL0 int, freeSlip, vanLeer bool) RunResult {
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
	cfg.Physical.RhoA = 1.0
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.SplitDivFix = true
	cfg.Numerical.ClipRedistribute = true
	// Part B.2: T = t * sqrt(2g/L0) for H0=2L0

	
	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()
	
	res := RunResult{
		Name: fmt.Sprintf("N%d_FS%v_VL%v", cellsPerL0, freeSlip, vanLeer),
		AlphaMin: 1.0,
		AlphaMax: 0.0,
	}
	
	sumIter := 0
	
	maxZ := -1.0
	for {
		err := sim.Step(-1)
		if err != nil {
			log.Fatalf("Sim error: %v", err)
		}
		
		res.Steps = sim.State.Step
		drift := math.Abs(sim.State.VolumeDrift / sim.State.Volume)
		if drift > res.MaxVolDrift { res.MaxVolDrift = drift }
		if sim.State.MaxDiv > res.MaxDiv { res.MaxDiv = sim.State.MaxDiv }
		sumIter += sim.State.PoissonIter
		if sim.State.PoissonIter > res.MaxPoissonIter { res.MaxPoissonIter = sim.State.PoissonIter }
		res.Warnings += sim.State.CFLWarnings
		
		for _, a := range sim.Fields.Alpha {
			if float64(a) < res.AlphaMin { res.AlphaMin = float64(a) }
			if float64(a) > res.AlphaMax { res.AlphaMax = float64(a) }
		}
		
		if sim.State.FrontXStar > maxZ {
			maxZ = sim.State.FrontXStar
			res.Z_sim = append(res.Z_sim, sim.State.FrontXStar)
			res.T_sim = append(res.T_sim, sim.State.TStar)
			res.Z_01 = append(res.Z_01, sim.State.FrontXStar01)
			res.Z_001 = append(res.Z_001, sim.State.FrontXStar001)
		}
		
		if sim.State.FrontXStar >= 14.0 || sim.State.TStar >= 12.0 {
			break
		}
	}
	
	res.MeanPoissonIter = float64(sumIter) / float64(res.Steps)
	res.WallTime = time.Since(start)
	return res
}

func main() {
	data, err := loadBenchmark("benchmark/martin_moyce.json")
	if err != nil {
		log.Fatalf("Failed to load benchmark: %v", err)
	}
	
	var warning string
	if !data.Verified {
		warning = "DATA NOT YET CHECKED BY A PERSON"
		fmt.Println(warning)
	}
	
	configs := []struct{n int; fs, vl bool}{
		{16, false, false},
		{16, false, true},
		{16, true, false},
		{16, true, true},
		{32, false, true},
		{32, true, true},
	}
	
	results := make([]RunResult, 0)
	for _, c := range configs {
		if c.n == 32 && !c.vl {
			// user: "otherwise only the two van Leer runs"
			// I'll run all 32 but print warning if long.
			fmt.Printf("Running %d FS=%v VL=%v...\n", c.n, c.fs, c.vl)
		} else {
			fmt.Printf("Running %d FS=%v VL=%v...\n", c.n, c.fs, c.vl)
		}
		res := runSim(c.n, c.fs, c.vl)
		results = append(results, res)
		fmt.Printf("Done in %v\n", res.WallTime)
	}
	
	fmt.Printf("\n=== RUN MATRIX ===\n")
	fmt.Printf("%-20s %-10s %-8s %-12s %-12s %-8s %-10s %-10s\n", 
		"Name", "WallTime", "Steps", "MaxVolDrift", "AlphaRange", "Warn", "MaxDiv", "Poisson")
	for _, r := range results {
		fmt.Printf("%-20s %-10s %-8d %.4e   [%.2e, %.2e] %-8d %.2e   %.1f/%d\n",
			r.Name, r.WallTime.Round(time.Second), r.Steps, r.MaxVolDrift,
			r.AlphaMin, r.AlphaMax, r.Warnings, r.MaxDiv, r.MeanPoissonIter, r.MaxPoissonIter)
	}
	
	// Comparison
	// a = 2.25 in mean
	Z_exp := data.Scales.A2p25In.Mean.Z
	T_exp := data.Scales.A2p25In.Mean.T
	
	for _, r := range results {
		// Output comparison for alpha=0.5
		fmt.Printf("\nCompare %s (alpha=0.5, Raw Time):\n", r.Name)
		var sumSq, count, maxErr float64
		for i, z := range Z_exp {
			if z < 1.44 { continue }
			if z > r.Z_sim[len(r.Z_sim)-1] { continue }
			
			t_sim := interpTSim(r.Z_sim, r.T_sim, z)
			dt := t_sim - T_exp[i]
			relErr := dt / T_exp[i]
			if math.Abs(relErr) > maxErr { maxErr = math.Abs(relErr) }
			sumSq += relErr * relErr
			count++
			fmt.Printf("  Z=%.2f: T_exp=%.2f T_sim=%.2f dT=%.2f (%.1f%%)\n", z, T_exp[i], t_sim, dt, relErr*100)
		}
		rms := math.Sqrt(sumSq / count)
		fmt.Printf("  RMS(dT/T) = %.1f%%, Max |dT/T| = %.1f%%\n", rms*100, maxErr*100)
		
		// Aligned Time
		alignedT := alignTime(r.Z_sim, r.T_sim, data.TimeNormalisation.AnchorZ, data.TimeNormalisation.AnchorT)
		fmt.Printf("Compare %s (alpha=0.5, Aligned Time):\n", r.Name)
		sumSq, count, maxErr = 0, 0, 0
		for i, z := range Z_exp {
			if z < 1.44 { continue }
			if z > r.Z_sim[len(r.Z_sim)-1] { continue }
			
			t_sim := interpTSim(r.Z_sim, alignedT, z)
			dt := t_sim - T_exp[i]
			relErr := dt / T_exp[i]
			if math.Abs(relErr) > maxErr { maxErr = math.Abs(relErr) }
			sumSq += relErr * relErr
			count++
		}
		rms = math.Sqrt(sumSq / count)
		fmt.Printf("  RMS(dT/T) = %.1f%%, Max |dT/T| = %.1f%%\n", rms*100, maxErr*100)
	}
	f, err := os.Create("bench_data.csv")
	if err == nil {
		f.WriteString("Name,Aligned,Z,T_exp,T_sim\n")
		for _, r := range results {
			for i, z := range Z_exp {
				if z < 1.44 || z > r.Z_sim[len(r.Z_sim)-1] { continue }
				t_sim := interpTSim(r.Z_sim, r.T_sim, z)
				f.WriteString(fmt.Sprintf("%s,false,%.2f,%.2f,%.2f\n", r.Name, z, T_exp[i], t_sim))
				
				alignedT := alignTime(r.Z_sim, r.T_sim, data.TimeNormalisation.AnchorZ, data.TimeNormalisation.AnchorT)
				t_sim_aligned := interpTSim(r.Z_sim, alignedT, z)
				f.WriteString(fmt.Sprintf("%s,true,%.2f,%.2f,%.2f\n", r.Name, z, T_exp[i], t_sim_aligned))
			}
		}
		f.Close()
	}
}
