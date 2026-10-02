package main

import (
	"dambreak/internal/solver"
	"flag"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"
)

// Ghia et al. (1982) Re=100 centreline data
var ghiaU = [][2]float64{
	{0.0000, 0.0000}, {0.0547, -0.03717}, {0.0625, -0.04192},
	{0.0703, -0.04775}, {0.1016, -0.06434}, {0.1719, -0.10150},
	{0.2813, -0.15662}, {0.4531, -0.21090}, {0.5000, -0.20581},
	{0.6172, -0.13641}, {0.7344, 0.00332}, {0.8516, 0.23151},
	{0.9531, 0.68717}, {0.9609, 0.73722}, {0.9688, 0.78871},
	{0.9766, 0.84123}, {1.0000, 1.00000},
}

var ghiaV = [][2]float64{
	{0.0000, 0.0000}, {0.0625, 0.09233}, {0.0703, 0.10091},
	{0.0781, 0.10890}, {0.0938, 0.12317}, {0.1563, 0.16077},
	{0.2266, 0.17507}, {0.2344, 0.17527}, {0.5000, 0.05454},
	{0.8047, -0.24533}, {0.8594, -0.22445}, {0.9063, -0.16914},
	{0.9453, -0.10313}, {0.9531, -0.08864}, {0.9609, -0.07391},
	{0.9688, -0.05906}, {1.0000, 0.0000},
}

func gitInfo() (string, string) {
	hash, _ := exec.Command("git", "rev-parse", "HEAD").Output()
	status, _ := exec.Command("git", "status", "--short").Output()
	return strings.TrimSpace(string(hash)), strings.TrimSpace(string(status))
}

func sampleCenterlineU(g *solver.Grid, f *solver.Fields, yNorm float64) float64 {
	y := yNorm * float64(g.Ny) * g.Dy
	j := int(y/g.Dy) + 1
	if j < 1 { j = 1 }
	if j > g.Ny { j = g.Ny }
	midI := (g.Nx + 1) / 2
	idx1 := g.IdxU(midI, j)
	idx2 := g.IdxU(midI+1, j)
	return 0.5 * (f.U[idx1] + f.U[idx2])
}

func sampleCenterlineV(g *solver.Grid, f *solver.Fields, xNorm float64) float64 {
	x := xNorm * float64(g.Nx) * g.Dx
	i := int(x/g.Dx) + 1
	if i < 1 { i = 1 }
	if i > g.Nx { i = g.Nx }
	midJ := (g.Ny + 1) / 2
	idx1 := g.IdxV(i, midJ)
	idx2 := g.IdxV(i, midJ+1)
	return 0.5 * (f.V[idx1] + f.V[idx2])
}

type CavityResult struct {
	N       int
	CFL     float64
	Scheme  string
	MaxU    float64
	KE      float64
	MaxDuDt float64  // max over [tStart,tEnd]
	MaxErrU float64
	MaxErrV float64
	RMSErrU float64
	RMSErrV float64
	DtStep1 float64
	DtFinal float64
}

// runCavity runs the lid-driven cavity to tEnd and measures max|du/dt| over [tMeas,tEnd].
// Sampling: max|U| and KE use cell-centre velocity = average of two face values.
// Lid row (j=Ny) IS included in the interior sweep.
// max|du/dt|: for each U-face node, |U^{n+1} - U^n| / dt, max over all nodes and
// all steps in [tMeas, tEnd].
func runCavity(n int, cfl float64, so bool, tEnd, tMeas float64) CavityResult {
	scheme := "FO"
	if so { scheme = "VL" }

	cfg := solver.DefaultConfig()
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0
	cfg.Physical.Gravity = 0
	cfg.Numerical.CFL = cfl
	cfg.Numerical.MaxDT = 1.0
	cfg.Numerical.PoissonTol = 1e-4
	cfg.Numerical.SecondOrderAdvect = so
	cfg.Physical.RhoW, cfg.Physical.MuW = 1.0, 0.01
	cfg.Physical.RhoA, cfg.Physical.MuA = 1.0, 0.01
	cfg.Threads = 1

	sim := solver.NewSimulation(cfg, n, n, 1.0, 1.0, true)

	var lastU []float64
	maxDuDt := 0.0
	dt1 := 0.0

	for sim.State.Time < tEnd {
		sim.Step(-1)
		if sim.State.Step == 1 { dt1 = sim.State.DT }

		if sim.State.Time >= tMeas {
			if lastU != nil {
				dt := sim.State.DT
				for idx := range sim.Fields.U {
					d := math.Abs(sim.Fields.U[idx]-lastU[idx]) / dt
					if d > maxDuDt { maxDuDt = d }
				}
			}
		}
		if lastU == nil || len(lastU) != len(sim.Fields.U) {
			lastU = make([]float64, len(sim.Fields.U))
		}
		copy(lastU, sim.Fields.U)
	}

	// Cell-centre max|U| and KE (interior cells j=1..Ny, i=1..Nx)
	maxU := 0.0
	ke := 0.0
	for j := 1; j <= n; j++ {
		for i := 1; i <= n; i++ {
			u := 0.5 * (sim.Fields.U[sim.Grid.IdxU(i, j)] + sim.Fields.U[sim.Grid.IdxU(i+1, j)])
			v := 0.5 * (sim.Fields.V[sim.Grid.IdxV(i, j)] + sim.Fields.V[sim.Grid.IdxV(i, j+1)])
			sp := math.Sqrt(u*u + v*v)
			if sp > maxU { maxU = sp }
			ke += 0.5 * (u*u + v*v) * sim.Grid.Dx * sim.Grid.Dy
		}
	}

	maxErrU, sumSqU, cntU := 0.0, 0.0, 0
	for _, pt := range ghiaU {
		if pt[0] <= 0 || pt[0] >= 1 { continue }
		if pt[0] == 0.9766 || pt[0] == 0.0547 { continue } // near-wall exclusion
		e := math.Abs(sampleCenterlineU(sim.Grid, sim.Fields, pt[0]) - pt[1])
		if e > maxErrU { maxErrU = e }
		sumSqU += e * e
		cntU++
	}

	maxErrV, sumSqV, cntV := 0.0, 0.0, 0
	for _, pt := range ghiaV {
		if pt[0] <= 0 || pt[0] >= 1 { continue }
		if pt[0] == 0.9688 || pt[0] == 0.0625 { continue }
		e := math.Abs(sampleCenterlineV(sim.Grid, sim.Fields, pt[0]) - pt[1])
		if e > maxErrV { maxErrV = e }
		sumSqV += e * e
		cntV++
	}

	return CavityResult{
		N: n, CFL: cfl, Scheme: scheme,
		MaxU: maxU, KE: ke, MaxDuDt: maxDuDt,
		MaxErrU: maxErrU, MaxErrV: maxErrV,
		RMSErrU: math.Sqrt(sumSqU / float64(cntU)),
		RMSErrV: math.Sqrt(sumSqV / float64(cntV)),
		DtStep1: dt1, DtFinal: sim.State.DT,
	}
}

func runDambreak(cellsPerL0 int, freeSlip, vanLeer bool) map[float64]float64 {
	L0 := 1.0
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
	cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
	cfg.Threads = 1

	sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()

	tAtZ := make(map[float64]float64)
	maxZ := -1.0
	for {
		sim.Step(-1)
		z := sim.State.FrontXStar
		if z > maxZ {
			maxZ = z
			for _, zt := range []float64{1.44, 3.0, 5.0, 7.0, 10.0, 12.0, 14.0} {
				if _, ok := tAtZ[zt]; !ok && z >= zt {
					tAtZ[zt] = sim.State.TStar
				}
			}
		}
		if z >= 14.0 || sim.State.TStar >= 12.0 { break }
	}
	return tAtZ
}

func main() {
	allowDirty := flag.Bool("allow-dirty", false, "allow running on dirty tree")
	flag.Parse()

	hash, status := gitInfo()
	fmt.Println("# Stability Fix Report")
	fmt.Printf("git rev-parse HEAD: %s\n", hash)
	fmt.Printf("git status --short:\n%s\n", status)
	if status != "" && !*allowDirty {
		fmt.Println("ERROR: tree is dirty. Use -allow-dirty to override.")
		return
	}
	fmt.Println()

	// ─── Item 2: dt components ───
	fmt.Println("## 2. dt Components")
	fmt.Println()
	fmt.Println("Definitions:")
	fmt.Println("  dtAdv  = CFL * h / umax                      (advective)")
	fmt.Println("  dtVisc = ViscousCFL * h^2 / (4 * nuMax)       (NEW: 2D explicit diffusion)")
	fmt.Println("  dtGrav = GravityCFL * sqrt(h / g)             (gravity wave)")
	fmt.Println("  dt     = min(dtAdv, dtVisc, dtGrav), capped by MaxDT and growth factor")
	fmt.Println()
	fmt.Println("OLD formula: dtVisc = ViscousCFL * h^2 / nuMax (missing /4)")
	fmt.Println("With ViscousCFL=0.25: OLD dtVisc = h^2/(4*nu), NEW dtVisc = h^2/(16*nu)")
	fmt.Println()

	fmt.Println("### Cavity (Re=100, nu=0.01)")
	fmt.Printf("%-6s %-5s %-11s %-11s %-11s %-11s %-11s\n",
		"Grid", "CFL", "h", "dtAdv(s1)", "dtVisc(new)", "dt(s1)", "dt(final)")
	for _, n := range []int{17, 33, 65, 129} {
		for _, cfl := range []float64{0.4, 0.2} {
			r := runCavity(n, cfl, false, 30.0, 50.0)
			h := 1.0 / float64(n)
			nu := 0.01
			dtViscNew := 0.25 * h * h / (4.0 * nu)
			fmt.Printf("%-6s %-5.2f %-11.5e %-11.4e %-11.4e %-11.4e %-11.4e\n",
				fmt.Sprintf("%d^2", n), cfl, h, r.DtStep1, dtViscNew, r.DtStep1, r.DtFinal)
		}
	}

	fmt.Println("\n### Dam-break (computed, not run)")
	for _, n := range []int{16, 32} {
		L0 := 1.0
		width := 15.0 * L0
		height := 4.0 * L0
		nx := int(math.Round(width / L0 * float64(n)))
		ny := int(math.Round(height / L0 * float64(n)))
		h := math.Min(width/float64(nx), height/float64(ny))
		nuAir := 1.8e-5 / 1.2
		nuWater := 1.0e-3 / 998.0
		dtViscOldAir := 0.25 * h * h / nuAir
		dtViscNewAir := 0.25 * h * h / (4.0 * nuAir)
		dtViscOldWater := 0.25 * h * h / nuWater
		dtViscNewWater := 0.25 * h * h / (4.0 * nuWater)
		fmt.Printf("N=%d: h=%.5e, nuAir=%.3e nuWater=%.3e\n", n, h, nuAir, nuWater)
		fmt.Printf("  OLD dtVisc(air)=%.4e  NEW dtVisc(air)=%.4e\n", dtViscOldAir, dtViscNewAir)
		fmt.Printf("  OLD dtVisc(water)=%.4e NEW dtVisc(water)=%.4e\n", dtViscOldWater, dtViscNewWater)
		fmt.Printf("  dtAdv ~ CFL*h/U_front, MaxDT=1e-3 (caps before viscous binds)\n")
	}

	// ─── Item 3: Mechanism / CFL scan ───
	fmt.Println("\n## 3. CFL Scan (NEW selector, 65^2)")
	fmt.Println("max|du/dt| = max over all U-face nodes and all steps in [50,60] of |U^{n+1}-U^n|/dt")
	h65 := 1.0 / 65.0
	nu := 0.01
	viscLimit := h65 * h65 / (4.0 * nu)
	fmt.Printf("h = %.6e, h^2/(4*nu) = %.6e\n", h65, viscLimit)
	fmt.Printf("%-7s %-11s %-16s %-14s\n", "CFL", "dt(s1)", "dt/(h^2/(4*nu))", "max|du/dt|[50,60]")
	for _, cfl := range []float64{0.300, 0.325, 0.350, 0.375, 0.400, 0.425, 0.450} {
		t0 := time.Now()
		r := runCavity(65, cfl, false, 60.0, 50.0)
		ratio := r.DtStep1 / viscLimit
		fmt.Printf("%-7.3f %-11.4e %-16.6f %-14.3e   (%.0fs)\n",
			cfl, r.DtStep1, ratio, r.MaxDuDt, time.Since(t0).Seconds())
	}

	// 129^2 if time allows
	fmt.Println("\n### CFL Scan 129^2")
	h129 := 1.0 / 129.0
	viscLimit129 := h129 * h129 / (4.0 * nu)
	fmt.Printf("h = %.6e, h^2/(4*nu) = %.6e\n", h129, viscLimit129)
	fmt.Printf("%-7s %-11s %-16s %-14s\n", "CFL", "dt(s1)", "dt/(h^2/(4*nu))", "max|du/dt|[50,60]")
	for _, cfl := range []float64{0.300, 0.400, 0.450} {
		t0 := time.Now()
		r := runCavity(129, cfl, false, 60.0, 50.0)
		ratio := r.DtStep1 / viscLimit129
		fmt.Printf("%-7.3f %-11.4e %-16.6f %-14.3e   (%.0fs)\n",
			cfl, r.DtStep1, ratio, r.MaxDuDt, time.Since(t0).Seconds())
	}

	// ─── Item 4: Fix test ───
	fmt.Println("\n## 4. Fix Test (NEW selector, 65^2, CFL=0.4)")
	r4 := runCavity(65, 0.4, false, 60.0, 50.0)
	fmt.Printf("max|du/dt| over [50,60]: %.3e\n", r4.MaxDuDt)
	fmt.Printf("For regression test: threshold 1.0 (old selector gives ~400)\n")

	// ─── Item 5: Cavity table ───
	fmt.Println("\n## 5. Cavity Table (Ghia errors, CFL=0.4, t=30)")
	fmt.Println("Sampling: cell-centre U,V = average of two bounding face values.")
	fmt.Println("Lid row (j=Ny) included. Near-wall exclusions: y=0.0547,0.9766 for U; x=0.0625,0.9688 for V.")
	fmt.Printf("%-6s %-4s %-9s %-9s %-9s %-9s %-9s %-9s %-9s\n",
		"Grid", "Sch", "Max|U|", "KE", "MaxEU", "RMSEU", "MaxEV", "RMSEV", "max|du/dt|")
	for _, n := range []int{17, 33, 65} {
		for _, so := range []bool{false, true} {
			r := runCavity(n, 0.4, so, 30.0, 20.0)
			fmt.Printf("%-6s %-4s %-9.5f %-9.5f %-9.5f %-9.5f %-9.5f %-9.5f %-9.2e\n",
				fmt.Sprintf("%d^2", n), r.Scheme, r.MaxU, r.KE, r.MaxErrU, r.RMSErrU, r.MaxErrV, r.RMSErrV, r.MaxDuDt)
		}
	}

	fmt.Println("\n### CFL=0.2")
	for _, n := range []int{17, 33, 65} {
		for _, so := range []bool{false, true} {
			r := runCavity(n, 0.2, so, 30.0, 20.0)
			fmt.Printf("%-6s %-4s %-9.5f %-9.5f %-9.5f %-9.5f %-9.5f %-9.5f %-9.2e\n",
				fmt.Sprintf("%d^2", n), r.Scheme, r.MaxU, r.KE, r.MaxErrU, r.RMSErrU, r.MaxErrV, r.RMSErrV, r.MaxDuDt)
		}
	}

	// ─── Item 6: Dam-break ───
	fmt.Println("\n## 6. Dam-break (current commit)")
	zPoints := []float64{1.44, 3.0, 5.0, 7.0, 10.0, 12.0, 14.0}
	for _, cfg := range []struct{ n int; fs bool; name string }{
		{16, false, "N16 no-slip VL"},
		{16, true, "N16 free-slip VL"},
	} {
		t0 := time.Now()
		tAtZ := runDambreak(cfg.n, cfg.fs, true)
		fmt.Printf("\n%s (%.0fs):\n", cfg.name, time.Since(t0).Seconds())
		for _, z := range zPoints {
			if t, ok := tAtZ[z]; ok {
				fmt.Printf("  T(Z=%.2f) = %.4f\n", z, t)
			}
		}
	}

	// ─── Item 8a: dt ladder ───
	fmt.Println("\n## 8a. dt Ladder (N=16, FO, no-slip)")
	fmt.Printf("%-10s %-10s %-10s %-10s %-10s %-10s %-8s\n",
		"CFL", "T(3)", "T(5)", "T(7)", "T(10)", "T(14)", "steps")
	for _, div := range []float64{1, 2, 4, 8} {
		cfl := 0.25 / div
		L0 := 1.0
		H0 := 2.0 * L0
		width := 15.0 * L0
		height := 4.0 * L0
		nx := int(math.Round(width / L0 * 16))
		ny := int(math.Round(height / L0 * 16))
		cfg := solver.DefaultConfig()
		cfg.Domain.L0 = L0; cfg.Domain.H0 = H0; cfg.Domain.Width = width; cfg.Domain.Height = height
		cfg.Domain.Nx = nx; cfg.Domain.Ny = ny
		cfg.Physical.RhoW = 1000.0; cfg.Physical.RhoA = 1.0
		cfg.Numerical.PoissonTol = 1e-6; cfg.Numerical.CFL = cfl
		cfg.Numerical.FreeSlip = false; cfg.Numerical.SecondOrderAdvect = false
		cfg.Numerical.SplitDivFix = true; cfg.Numerical.ClipRedistribute = true
		cfg.TimeScale = solver.TimeScaleSqrt2gOverL0; cfg.Threads = 1

		sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
		sim.InitDamBreak()
		tAtZ := make(map[float64]float64)
		maxZ := -1.0
		for {
			sim.Step(-1)
			z := sim.State.FrontXStar
			if z > maxZ {
				maxZ = z
				for _, zt := range []float64{3, 5, 7, 10, 14} {
					if _, ok := tAtZ[zt]; !ok && z >= zt { tAtZ[zt] = sim.State.TStar }
				}
			}
			if z >= 14.0 || sim.State.TStar >= 12.0 { break }
		}
		fmt.Printf("%-10.4f %-10.4f %-10.4f %-10.4f %-10.4f %-10.4f %-8d\n",
			cfl, tAtZ[3], tAtZ[5], tAtZ[7], tAtZ[10], tAtZ[14], sim.State.Step)
	}

	// ─── Item 8b: space study ───
	fmt.Println("\n## 8b. Space Study (CFL=0.0625, FO, no-slip)")
	fmt.Printf("%-6s %-10s %-10s %-10s %-10s %-10s %-8s\n",
		"N", "T(3)", "T(5)", "T(7)", "T(10)", "T(14)", "steps")
	for _, n := range []int{8, 12, 16, 24} {
		L0 := 1.0
		H0 := 2.0 * L0
		width := 15.0 * L0
		height := 4.0 * L0
		nx := int(math.Round(width / L0 * float64(n)))
		ny := int(math.Round(height / L0 * float64(n)))
		cfg := solver.DefaultConfig()
		cfg.Domain.L0 = L0; cfg.Domain.H0 = H0; cfg.Domain.Width = width; cfg.Domain.Height = height
		cfg.Domain.Nx = nx; cfg.Domain.Ny = ny
		cfg.Physical.RhoW = 1000.0; cfg.Physical.RhoA = 1.0
		cfg.Numerical.PoissonTol = 1e-6; cfg.Numerical.CFL = 0.0625
		cfg.Numerical.FreeSlip = false; cfg.Numerical.SecondOrderAdvect = false
		cfg.Numerical.SplitDivFix = true; cfg.Numerical.ClipRedistribute = true
		cfg.TimeScale = solver.TimeScaleSqrt2gOverL0; cfg.Threads = 1

		sim := solver.NewSimulation(cfg, nx, ny, width, height, false)
		sim.InitDamBreak()
		tAtZ := make(map[float64]float64)
		maxZ := -1.0
		for {
			sim.Step(-1)
			z := sim.State.FrontXStar
			if z > maxZ {
				maxZ = z
				for _, zt := range []float64{3, 5, 7, 10, 14} {
					if _, ok := tAtZ[zt]; !ok && z >= zt { tAtZ[zt] = sim.State.TStar }
				}
			}
			if z >= 14.0 || sim.State.TStar >= 12.0 { break }
		}
		fmt.Printf("%-6d %-10.4f %-10.4f %-10.4f %-10.4f %-10.4f %-8d\n",
			n, tAtZ[3], tAtZ[5], tAtZ[7], tAtZ[10], tAtZ[14], sim.State.Step)
	}

	// ─── Item 8c: go test ───
	fmt.Println("\n## 8c. go test ./... -v")
	fmt.Println("(run separately: go test ./... -v)")

	fmt.Println("\nDone.")
}
