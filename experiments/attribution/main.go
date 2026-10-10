package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"

	"dambreak/internal/solver"
)

type BenchmarkData struct {
	Scales struct {
		A2p25In struct {
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
	return &data, nil
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

func runSimAttr(cellsPerL0 int, freeSlip, vanLeer bool, rhoRatio float64, halfDt bool) ([]float64, []float64, []float64, []float64) {
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

	cfg.Physical.RhoW = rhoRatio
	cfg.Physical.RhoA = 1.0
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Numerical.FreeSlip = freeSlip
	cfg.Numerical.SecondOrderAdvect = vanLeer
	cfg.Numerical.SplitDivFix = true
	cfg.Numerical.ClipRedistribute = true

	
	if halfDt {
		cfg.Numerical.MaxDT = 0.5 * cfg.Numerical.MaxDT
		cfg.Numerical.CFL = 0.5 * cfg.Numerical.CFL
	}
	
	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()
	
	var Z_sim, T_sim, Z_01, Z_001 []float64
	maxZ := -1.0
	
	for {
		sim.Step(-1)
		
		if sim.State.FrontXStar > maxZ {
			maxZ = sim.State.FrontXStar
			Z_sim = append(Z_sim, sim.State.FrontXStar)
			T_sim = append(T_sim, sim.State.TStar)
			Z_01 = append(Z_01, sim.State.FrontXStar01)
			Z_001 = append(Z_001, sim.State.FrontXStar001)
		}
		
		if sim.State.FrontXStar >= 8.0 || sim.State.TStar >= 6.0 {
			break
		}
	}
	return Z_sim, T_sim, Z_01, Z_001
}

func reportDiff(name string, Z_exp, T_exp, Z_sim, T_sim []float64) {
	fmt.Printf("\n--- %s ---\n", name)
	for i, z := range Z_exp {
		if z < 1.44 { continue }
		if z > Z_sim[len(Z_sim)-1] { continue }
		if z > 7.0 { continue }
		t_sim := interpTSim(Z_sim, T_sim, z)
		dt := t_sim - T_exp[i]
		relErr := dt / T_exp[i]
		fmt.Printf("  Z=%.2f: T_exp=%.2f T_sim=%.2f dT=%.2f (%.1f%%)\n", z, T_exp[i], t_sim, dt, relErr*100)
	}
}

func main() {
	data, err := loadBenchmark("benchmark/martin_moyce.json")
	if err != nil {
		fmt.Printf("Load err: %v\n", err)
		return
	}
	Z_exp := data.Scales.A2p25In.Mean.Z
	T_exp := data.Scales.A2p25In.Mean.T

	fmt.Println("Baseline (16, FS, 1st-order, 1000 ratio)")
	Z_base, T_base, Z01_base, Z001_base := runSimAttr(16, true, false, 1000.0, false)
	reportDiff("Baseline (0.5 crossing)", Z_exp, T_exp, Z_base, T_base)
	reportDiff("Baseline (0.1 crossing)", Z_exp, T_exp, Z01_base, T_base)
	reportDiff("Baseline (0.01 crossing)", Z_exp, T_exp, Z001_base, T_base)
	
	fmt.Println("Van Leer")
	Z_vl, T_vl, _, _ := runSimAttr(16, true, true, 1000.0, false)
	reportDiff("Van Leer (0.5 crossing)", Z_exp, T_exp, Z_vl, T_vl)
	
	fmt.Println("32 cells/L0")
	Z_32, T_32, _, _ := runSimAttr(32, true, false, 1000.0, false)
	reportDiff("32 cells/L0 (0.5 crossing)", Z_exp, T_exp, Z_32, T_32)
	
	fmt.Println("Density ratio 100")
	Z_100, T_100, _, _ := runSimAttr(16, true, false, 100.0, false)
	reportDiff("Density ratio 100", Z_exp, T_exp, Z_100, T_100)
	
	fmt.Println("Half DT")
	Z_dt, T_dt, _, _ := runSimAttr(16, true, false, 1000.0, true)
	reportDiff("Half DT", Z_exp, T_exp, Z_dt, T_dt)
}
