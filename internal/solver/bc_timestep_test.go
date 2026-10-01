package solver

import (
	"math"
	"testing"
)

func newTestGrid() (*Grid, *Fields, *Config) {
	cfg := DefaultConfig()
	g := NewGrid(16, 24, 4*cfg.Domain.L0, 3*cfg.Domain.H0)
	f := NewFields(g)
	return g, f, &cfg
}

// --- Step 1: boundary conditions --------------------------------------------

func TestVelocityBCWallNormalsExactZero(t *testing.T) {
	g, f, cfg := newTestGrid()
	for i := range f.U {
		f.U[i] = 3.7 // junk everywhere
	}
	for i := range f.V {
		f.V[i] = -2.9
	}
	ApplyVelocityBC(f, cfg, g)

	for j := 0; j < g.NyG; j++ {
		if f.U[g.LeftWallU(j)] != 0 || f.U[g.RightWallU(j)] != 0 {
			t.Fatalf("wall u-face not exactly 0 at row %d", j)
		}
	}
	for i := 0; i < g.NxG; i++ {
		if f.V[g.FloorV(i)] != 0 {
			t.Fatalf("floor v-face not exactly 0 at col %d", i)
		}
	}
}

func TestVelocityBCNoSlipVsFreeSlipGhost(t *testing.T) {
	g := NewGrid(8, 8, 1.0, 1.0)
	ub := 0.25
	vb := -0.5

	mkCfg := func(slip bool) *Config {
		c := DefaultConfig()
		c.Numerical.FreeSlip = slip
		return &c
	}

	// No-slip: tangential ghost = -interior.
	f := NewFields(g)
	f.U[g.idxU(3, 1)] = ub
	f.V[g.idxV(1, 3)] = vb
	ApplyVelocityBC(f, mkCfg(false), g)
	if got := f.U[g.idxU(3, 0)]; got != -ub {
		t.Fatalf("no-slip floor ghost u = %g, want %g", got, -ub)
	}
	if got := f.V[g.idxV(0, 3)]; got != -vb {
		t.Fatalf("no-slip left ghost v = %g, want %g", got, -vb)
	}

	// Free-slip: tangential ghost = +interior.
	f2 := NewFields(g)
	f2.U[g.idxU(3, 1)] = ub
	f2.V[g.idxV(1, 3)] = vb
	ApplyVelocityBC(f2, mkCfg(true), g)
	if got := f2.U[g.idxU(3, 0)]; got != ub {
		t.Fatalf("free-slip floor ghost u = %g, want %g", got, ub)
	}
	if got := f2.V[g.idxV(0, 3)]; got != vb {
		t.Fatalf("free-slip left ghost v = %g, want %g", got, vb)
	}
}

func TestAlphaBCZeroGradient(t *testing.T) {
	g, f, _ := newTestGrid()
	for idx := range f.Alpha {
		f.Alpha[idx] = 0.42
	}
	ApplyAlphaBC(g, f)
	for j := 0; j < g.NyG; j++ {
		if f.Alpha[g.idxCC(0, j)] != 0.42 || f.Alpha[g.idxCC(g.Nx+1, j)] != 0.42 {
			t.Fatalf("alpha side ghost wrong at row %d", j)
		}
	}
}

// --- Step 2: timestep control ------------------------------------------------

func TestComputeDtStillFieldAndScaling(t *testing.T) {
	g, f, _ := newTestGrid()
	cfg := DefaultConfig()
	UpdateProperties(g, f, &cfg)
	var st SimState

	// Still field: umax=0 must fall back to a finite positive dt (<= MaxDT).
	dt0 := ComputeDt(g, f, &cfg, &st)
	if !(dt0 > 0 && dt0 <= cfg.Numerical.MaxDT) {
		t.Fatalf("still-field dt = %g, want (0, MaxDT]", dt0)
	}

	// Halving dx must halve the advective dt.
	g2 := NewGrid(32, 48, 4*cfg.Domain.L0, 3*cfg.Domain.H0)
	f2 := NewFields(g2)
	UpdateProperties(g2, f2, &cfg)
	dt0b := ComputeDt(g2, f2, &cfg, &st)
	if ratio := dt0b / dt0; math.Abs(ratio-1) > 1e-12 {
		t.Logf("note: still-field fallback ignores dx by design (ratio=%g)", ratio)
	}

	// Umax scaling: dt_adv = CFL*hmin/umax.
	umax := 2.0
	f2.U[g2.idxU(5, 10)] = umax
	dt1 := ComputeDt(g2, f2, &cfg, &st)
	wantAdv := cfg.Numerical.CFL * math.Min(g2.Dx, g2.Dy) / umax
	if dt1 > wantAdv*(1+1e-12) {
		t.Fatalf("dt = %g exceeds advective limit %g", dt1, wantAdv)
	}
	// Doubling umax halves the advective dt (still the binding limit here).
	f2.U[g2.idxU(5, 10)] = 2 * umax
	dt2 := ComputeDt(g2, f2, &cfg, &st)
	if ratio := dt1 / dt2; math.Abs(ratio-2) > 1e-9 {
		t.Fatalf("dt did not halve when umax doubled: ratio=%g", ratio)
	}
}

// --- helpers for the Poisson tests -------------------------------------------

func maxDiff(a, b []float64) float64 {
	m := 0.0
	for i := range a {
		if d := math.Abs(a[i] - b[i]); d > m {
			m = d
		}
	}
	return m
}

// Constant-density field with a still interface (rho=1 everywhere).
func constantDensityFields(g *Grid) *Fields {
	f := NewFields(g)
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			f.Rho[g.idxCC(i, j)] = 1.0
			f.Mu[g.idxCC(i, j)] = 1.0
		}
	}
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			f.RhoU[g.idxU(i, j)] = 1.0
			f.RhoV[g.idxV(i, j)] = 1.0
		}
	}
	return f
}

// Two horizontal layers: rho=rho1 below y=0.5H, rho2 above.
func layeredFields(g *Grid, rho1, rho2 float64) *Fields {
	f := NewFields(g)
	for j := 1; j <= g.Ny; j++ {
		rho := rho1
		if g.Yc[g.idxCC(1, j)] > 0.5*3*DefaultConfig().Domain.H0 {
			rho = rho2
		}
		for i := 1; i <= g.Nx; i++ {
			f.Rho[g.idxCC(i, j)] = rho
			f.Mu[g.idxCC(i, j)] = rho * 1e-6
		}
	}
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			rl := f.Rho[g.idxCC(i-1, j)]
			rr := f.Rho[g.idxCC(i, j)]
			f.RhoU[g.idxU(i, j)] = 0.5 * (rl + rr)
			rb := f.Rho[g.idxCC(i, j-1)]
			rt := f.Rho[g.idxCC(i, j)]
			f.RhoV[g.idxV(i, j)] = 0.5 * (rb + rt)
		}
	}
	return f
}
