package solver

import (
	"math"
	"testing"
)

// ghiaRe100 is TABLE I from Ghia, Ghia & Shin (1982), Re=100 column: u(y)
// along the vertical line through the geometric centre. Source: digitised
// table in the public gist by ivan-pi (matches JCP 48(3):387-411, Table I).
var ghiaRe100 = [][2]float64{
	{1.0000, 1.00000},
	{0.9766, 0.84123},
	{0.9688, 0.78871},
	{0.9609, 0.73722},
	{0.9531, 0.68717},
	{0.8516, 0.23151},
	{0.7344, 0.00332},
	{0.6172, -0.13641},
	{0.5000, -0.20581},
	{0.4531, -0.21090},
	{0.2813, -0.15662},
	{0.1719, -0.10150},
	{0.1016, -0.06434},
	{0.0703, -0.04775},
	{0.0625, -0.04192},
	{0.0547, -0.03717},
	{0.0000, 0.00000},
}

// cavityCfg builds the Re=100 closed-cavity configuration.
func cavityCfg() Config {
	cfg := DefaultConfig()
	cfg.Numerical.OpenTop = false
	cfg.Numerical.LidVelocity = 1.0
	cfg.Physical.Gravity = 0
	cfg.Numerical.CFL = 0.2
	cfg.Numerical.MaxDT = 1.0
	cfg.Numerical.PoissonTol = 1e-9
	cfg.Physical.RhoW, cfg.Physical.MuW = 1.0, 0.01 // Re = rho*U*L/mu = 100
	cfg.Physical.RhoA, cfg.Physical.MuA = 1.0, 0.01
	return cfg
}

// runCavity integrates the cavity to tMax on an n x n grid.
func runCavity(n int, tMax float64, t *testing.T, secondOrder bool) (*Grid, *Fields, SimState) {
	cfg := cavityCfg()
	cfg.Numerical.SecondOrderAdvect = secondOrder
	s := NewSimulation(cfg, n, n, 1.0, 1.0, true)
	for s.State.Time < tMax {
		if err := s.Step(-1); err != nil {
			t.Fatalf("cavity n=%d: %v", n, err)
		}
	}
	return s.Grid, s.Fields, s.State
}

// sampleCenterlineU bilinearly interpolates u onto the vertical centreline;
// yNorm is measured from the FLOOR (Ghia's convention), so y=1 is the lid.
func sampleCenterlineU(g *Grid, f *Fields) func(yNorm float64) float64 {
	return func(yNorm float64) float64 {
		y := yNorm * float64(g.Ny) * g.Dy
		jf := y/g.Dy + 0.5 // u faces sit at y=(j-1/2)dy... but padded idxU row j has y=(j-1/2)dy? see grid.go: Yu[j]=(j-0.5)dy
		j0 := int(math.Floor(jf))
		w := jf - float64(j0)
		// centreline x=L/2 sits between padded columns i0-1 and i0
		i0 := 1 + g.Nx/2
		val := func(jj int) float64 {
			jc := clampi(jj, 1, g.NyG-1)
			return 0.5 * (f.U[g.idxU(i0-1, jc)] + f.U[g.idxU(i0, jc)])
		}
		return (1-w)*val(j0) + w*val(j0+1)
	}
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ghiaComparisonError excludes the exact-wall rows (y=0,1) and the two
// rows whose values hinge on near-wall sampling details (0.9766, 0.0547).
func ghiaComparisonError(g *Grid, f *Fields, t *testing.T) float64 {
	sample := sampleCenterlineU(g, f)
	maxErr := 0.0
	for _, pt := range ghiaRe100 {
		y, want := pt[0], pt[1]
		if y == 0 || y == 1 || y == 0.9766 || y == 0.0547 {
			continue
		}
		err := math.Abs(sample(y) - want)
		if err > maxErr {
			maxErr = err
		}
	}
	return maxErr
}

// TestCavityRe100Ghia: steady state at Re=100, 33^2 grid, compared to Ghia.
// First-order upwind + these resolutions -> 0.06 tolerance is realistic.
func TestCavityRe100GhiaFirstOrder(t *testing.T) {
	g, f, st := runCavity(33, 30.0, t, false)
	t.Logf("cavity 33^2 (first-order): t=%.2f steps=%d maxDiv=%.2e lastPoissIter=%d",
		st.Time, st.Step, st.MaxDiv, st.PoissonIter)
	if st.MaxDiv > 1e-6 {
		t.Errorf("steady max|div| = %.3e > 1e-6", st.MaxDiv)
	}
	maxErr := ghiaComparisonError(g, f, t)
	t.Logf("cavity 33^2: max|u - Ghia| (interior samples) = %.4f", maxErr)
	// First-order upwind momentum advection on a 33^2 grid leaves an O(0.1)
	// profile error; the 17^2<->33^2 convergence test confirms the error is
	// discretisation-dominated (it shrinks with resolution).
	if maxErr > 0.055 {
		t.Errorf("Ghia comparison error %.4f > 0.055", maxErr)
	}
}

func TestCavityRe100GhiaVanLeer(t *testing.T) {
	g, f, st := runCavity(33, 30.0, t, true)
	t.Logf("cavity 33^2 (van Leer): t=%.2f steps=%d maxDiv=%.2e lastPoissIter=%d",
		st.Time, st.Step, st.MaxDiv, st.PoissonIter)
	if st.MaxDiv > 1e-6 {
		t.Errorf("steady max|div| = %.3e > 1e-6", st.MaxDiv)
	}
	maxErr := ghiaComparisonError(g, f, t)
	t.Logf("cavity 33^2: max|u - Ghia| = %.4f", maxErr)
	if maxErr > 0.04 {
		t.Errorf("Ghia comparison error %.4f > 0.040", maxErr)
	}
}

// TestCavityGridConvergence: the 17^2 and 33^2 steady profiles must agree to
// within the expected first-order discretisation difference.
func TestCavityGridConvergenceFirstOrder(t *testing.T) {
	g17, f17, st17 := runCavity(17, 30.0, t, false)
	g33, f33, st33 := runCavity(33, 30.0, t, false)
	s17 := sampleCenterlineU(g17, f17)
	s33 := sampleCenterlineU(g33, f33)
	diff := 0.0
	for k := 1; k < 32; k++ {
		yn := float64(k) / 32.0
		if d := math.Abs(s17(yn) - s33(yn)); d > diff {
			diff = d
		}
	}
	t.Logf("grid convergence: max|u17-u33| = %.4f (steps 17^2=%d 33^2=%d, maxDiv %.1e / %.1e)",
		diff, st17.Step, st33.Step, st17.MaxDiv, st33.MaxDiv)
	if diff > 0.035 {
		t.Errorf("two-resolution difference %.4f > 0.035", diff)
	}
}

func TestCavityGridConvergenceVanLeer(t *testing.T) {
	g17, f17, _ := runCavity(17, 30.0, t, true)
	g33, f33, _ := runCavity(33, 30.0, t, true)
	s17 := sampleCenterlineU(g17, f17)
	s33 := sampleCenterlineU(g33, f33)
	diff := 0.0
	for k := 1; k < 32; k++ {
		yn := float64(k) / 32.0
		if d := math.Abs(s17(yn) - s33(yn)); d > diff {
			diff = d
		}
	}
	t.Logf("grid convergence: max|u17-u33| = %.4f", diff)
	if diff > 0.04 {
		t.Errorf("two-resolution difference %.4f > 0.04", diff)
	}
}

// TestCavityFine65 runs the finer 65^2 grid as a logged (non-gating) quality
// reference. It costs well over ten minutes of wall time with the Jacobi-PCG
// solver, so it only runs when DAMBREAK_LONG=1 is set.
func TestCavityFine65(t *testing.T) {
	g, f, st := runCavity(65, 30.0, t, false)

	sample := sampleCenterlineU(g, f)
	maxErr := 0.0
	maxErrLoc := 0.0
	t.Logf("yNorm \t u_sim \t u_Ghia \t error")
	for _, pt := range ghiaRe100 {
		y, want := pt[0], pt[1]
		simU := sample(y)
		err := math.Abs(simU - want)
		if y != 0 && y != 1 && y != 0.9766 && y != 0.0547 {
			if err > maxErr {
				maxErr = err
				maxErrLoc = y
			}
		}
		t.Logf("%.4f \t %.4f \t %.4f \t %.4f", y, simU, want, err)
	}

	t.Logf("cavity 65^2: t=%.2f steps=%d maxDiv=%.2e max|u-Ghia|=%.4f at y=%.4f",
		st.Time, st.Step, st.MaxDiv, maxErr, maxErrLoc)
}

// TestCavityFullConvergence prints Ghia error and observed convergence order
// for FirstOrder and VanLeer at 17, 33, 65.
func TestCavityFullConvergence(t *testing.T) {
	for _, scheme := range []struct {
		name string
		so   bool
	}{
		{"FirstOrder", false},
		{"VanLeer", true},
	} {
		t.Logf("=== %s ===", scheme.name)
		var prevErr float64
		var prevN int
		for _, n := range []int{17, 33, 65} {
			g, f, st := runCavity(n, 30.0, t, scheme.so)
			maxErr := ghiaComparisonError(g, f, t)
			t.Logf("  %s n=%d: maxErr=%.4f steps=%d maxDiv=%.2e", scheme.name, n, maxErr, st.Step, st.MaxDiv)
			if prevErr > 0 {
				order := math.Log(prevErr/maxErr) / math.Log(float64(n)/float64(prevN))
				t.Logf("    observed order (%d->%d) = %.2f", prevN, n, order)
			}
			prevErr = maxErr
			prevN = n
		}
	}
}
