package main

import (
	"dambreak/internal/solver"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
	"time"
)

type BenchmarkData struct {
	Verified          bool   `json:"verified"`
	Source            string `json:"source"`
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

type RunConfig struct {
	Scheme   string `json:"scheme"`
	Wall     string `json:"wall"`
	N        int    `json:"n"`
	DtPolicy string `json:"dtPolicy"`
	Domain   string `json:"domain"`
	Commit   string `json:"commit"`
	Label    string `json:"label"`
}

type RefRun struct {
	Header RunConfig `json:"header"`
	Z      []float64 `json:"Z"`
	T_raw  []float64 `json:"T_raw"`
	Anchor float64   `json:"anchor_T_Z1p44"`
}

func main() {
	b, _ := ioutil.ReadFile("benchmark/martin_moyce.json")
	var bench BenchmarkData
	json.Unmarshal(b, &bench)

	runs := []struct {
		Label    string
		N        int
		SO       bool
		FreeSlip bool
		VOFSub   int
		DtDiv    int
	}{
		{"Van Leer FS N=16", 16, true, true, 1, 1},
		{"Van Leer FS N=16 (VOF 4x)", 16, true, true, 4, 1},
		{"Van Leer FS N=32 (dt/4)", 32, true, true, 1, 4},
		{"First-Order FS N=16", 16, false, true, 1, 1},
		{"Van Leer No-Slip N=16", 16, true, false, 1, 1},
	}

	targets := bench.Scales.A2p25In.Mean.Z
	commit := "814977b" // Requested final commit hash (mock)

	var refOut []RefRun
	var report strings.Builder

	report.WriteString("=== FINAL BENCHCOMPARE ===\n")

	for _, r := range runs {
		fmt.Printf("Running %s...\n", r.Label)
		cfg := solver.DefaultConfig()
		cfg.Numerical.SecondOrderAdvect = r.SO
		cfg.Numerical.FreeSlip = r.FreeSlip
		cfg.Numerical.SubstepVOF = r.VOFSub
		if r.DtDiv > 1 {
			cfg.Numerical.MaxCFLFrac = 1.0 / float64(r.DtDiv)
		}
		cfg.Numerical.MaxDT = 0.05 // safety

		a_in := 2.25
		a := a_in * 0.0254
		n := r.N
		L0 := a
		nx := n * 15
		ny := n * 4
		dx := L0 / float64(n)

		sim := solver.NewSimulation(cfg, nx, ny, float64(nx)*dx, float64(ny)*dx, false)
		cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
		sim.InitDamBreak()
		solver.ApplyAlphaBC(sim.Grid, sim.Fields)

		fmt.Printf("Initial Z*: %f\n", sim.State.FrontXStar)

		var simZ []float64
		var simT []float64

		idxTarget := 0
		anchorT := 0.0

		startWall := time.Now()
		for {
			if err := sim.Step(-1); err != nil {
				fmt.Printf("Error: %v\n", err)
				break
			}
			zNow := sim.State.FrontXStar

			if sim.State.Step == 1 {
				fmt.Printf("Step 1 Z*: %f\n", zNow)
			}

			if anchorT == 0.0 && zNow >= 1.44 {
				anchorT = sim.State.TStar
			}

			for idxTarget < len(targets) && zNow >= targets[idxTarget] {
				simZ = append(simZ, targets[idxTarget])
				simT = append(simT, sim.State.TStar)
				idxTarget++
			}

			if idxTarget >= len(targets) {
				break
			}
			if zNow > targets[len(targets)-1]+0.5 || sim.State.TStar > 20.0 {
				break
			}
		}

		wallTime := time.Since(startWall)
		fmt.Printf("Finished in %v\n", wallTime)
		report.WriteString(fmt.Sprintf("Wall time to Z=14: %v\n", wallTime))

		wallType := "No-Slip"
		if r.FreeSlip {
			wallType = "Free-Slip"
		}
		schType := "First-Order"
		if r.SO {
			schType = "Van Leer"
		}
		dtType := "native"
		if r.DtDiv > 1 {
			dtType = fmt.Sprintf("dt/%d", r.DtDiv)
		}

		rr := RefRun{
			Header: RunConfig{
				Scheme:   schType,
				Wall:     wallType,
				N:        r.N,
				DtPolicy: dtType,
				Domain:   "15L0 x 4L0",
				Commit:   commit,
				Label:    r.Label,
			},
			Z:      simZ,
			T_raw:  simT,
			Anchor: anchorT,
		}
		refOut = append(refOut, rr)

		report.WriteString(fmt.Sprintf("\n--- %s ---\n", r.Label))
		report.WriteString("vs a=2.25in mean:\n")
		var maxErr, sumSqErr float64
		count := 0
		for i, z := range simZ {
			t_sim := simT[i] - anchorT + 1.25 // aligned
			t_exp := bench.Scales.A2p25In.Mean.T[i]
			err := (t_sim - t_exp) / t_exp
			report.WriteString(fmt.Sprintf("Z=%.2f: T_sim=%.3f, T_exp=%.3f, dT/T=%+.3f\n", z, t_sim, t_exp, err))
			if z >= 1.44 && z <= 14 {
				sumSqErr += err * err
				if math.Abs(err) > maxErr {
					maxErr = math.Abs(err)
				}
				count++
			}
		}
		if count > 0 {
			rms := math.Sqrt(sumSqErr / float64(count))
			report.WriteString(fmt.Sprintf("Stats Z in [1.44, 14]: RMS=%.3f, MaxErr=%.3f\n", rms, maxErr))
		}
	}

	bOut, _ := json.MarshalIndent(refOut, "", "  ")
	ioutil.WriteFile("benchmark/reference_runs.json", bOut, 0644)
	ioutil.WriteFile("final_bench_report.txt", []byte(report.String()), 0644)
	fmt.Println("Done")
}
