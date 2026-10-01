package solver

import (
	"math"
	"math/rand"
	"testing"
)

// --- Step 3a: manufactured solution, constant density ------------------------
//
// p(x,y) = cos(pi x) * cos(pi y / (2H)) on [0,1]x[0,H].
// Compatible with our BCs: dp/dx = 0 at x=0 and x=1 (side walls, Neumann),
// dp/dy = 0 at y=0 (floor, Neumann), p = 0 at y=H (top, Dirichlet).
// A p = -Lap(p) = (pi^2 + pi^2/(4H^2)) p.
func manufacturedRHS(g *Grid, b, exact []float64) {
	pi2 := math.Pi * math.Pi
	H := g.Dy * float64(g.Ny)
	k := math.Pi / (2 * H)
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			x := g.Xc[idx]
			y := g.Yc[idx]
			exact[idx] = math.Cos(math.Pi*x) * math.Cos(k*y)
			b[idx] = (pi2 + pi2*k*k/(math.Pi*math.Pi)) * exact[idx]
		}
	}
}

func TestPoissonManufacturedConstantDensity(t *testing.T) {
	cases := []struct {
		n         int
		wantOrder float64 // expected log2(err ratio) between successive grids
	}{
		{16, 1.6}, // allow a bit below the asymptotic 2.0 at coarse grids
		{32, 1.6},
	}
	grids := []int{8, 16, 32}
	errs := make([]float64, len(grids))
	cfg := DefaultConfig()
	for gi, n := range grids {
		g := NewGrid(n, n, 1.0, 1.0)
		f := constantDensityFields(g)
		b := make([]float64, g.TotalCC())
		exact := make([]float64, g.TotalCC())
		manufacturedRHS(g, b, exact)
		p := make([]float64, g.TotalCC())
		res := SolvePoissonPCG(g, f, b, p, cfg.Numerical.PoissonTol, cfg.Numerical.PoissonMaxIter, 1, allocScratch(g))
		if !res.Converged {
			t.Fatalf("n=%d: PCG did not converge: %+v", n, res)
		}
		emax := 0.0
		for j := 1; j <= g.Ny; j++ {
			for i := 1; i <= g.Nx; i++ {
				idx := g.idxCC(i, j)
				if d := math.Abs(p[idx] - exact[idx]); d > emax {
					emax = d
				}
			}
		}
		errs[gi] = emax
		t.Logf("n=%2d iters=%3d relRes=%.2e  max|p-p_h|=%.3e", n, res.Iterations, res.Residual, emax)
	}
	for k := 0; k < len(cases); k++ {
		order := math.Log2(errs[k] / errs[k+1])
		if order < cases[k].wantOrder {
			t.Errorf("grid %d->%d convergence order = %.2f < %.2f (errs %.3e -> %.3e)",
				grids[k], grids[k+1], order, cases[k].wantOrder, errs[k], errs[k+1])
		}
	}

	// PCG and SOR must agree on the same problem.
	g := NewGrid(16, 16, 1.0, 1.0)
	f := constantDensityFields(g)
	b := make([]float64, g.TotalCC())
	exact := make([]float64, g.TotalCC())
	manufacturedRHS(g, b, exact)
	pPcg := make([]float64, g.TotalCC())
	resPcg := SolvePoissonPCG(g, f, b, pPcg, 1e-10, 20000, 1, allocScratch(g))
	pSor := make([]float64, g.TotalCC())
	// omega=1.9 (the pure-Dirichlet optimum) diverges with Neumann walls;
	// 1.5 is safely convergent for this mixed-BC operator.
	resSor := SolvePoissonSOR(g, f, b, pSor, 1e-10, 200000, 1.5)
	t.Logf("PCG: %+v", resPcg)
	t.Logf("SOR: %+v", resSor)
	if d := maxDiff(pPcg, pSor); d > 1e-6 {
		t.Errorf("PCG vs SOR disagree: max diff %.3e", d)
	}
}

// --- Step 3b: two-layer density, ratio sweep ---------------------------------

func TestPoissonTwoLayerDensityRatios(t *testing.T) {
	for _, ratio := range []float64{1, 10, 100, 1000} {
		g := NewGrid(32, 48, 0.2286, 0.3429)
		rhoTop := 1.0
		rhoBot := rhoTop * ratio
		f := layeredFields(g, rhoBot, rhoTop)

		// RHS from a smooth manufactured-like bump; we only need convergence.
		b := make([]float64, g.TotalCC())
		for j := 1; j <= g.Ny; j++ {
			for i := 1; i <= g.Nx; i++ {
				idx := g.idxCC(i, j)
				x := g.Xc[idx]
				y := g.Yc[idx]
				b[idx] = math.Exp(-40 * ((x-0.11)*(x-0.11) + (y-0.08)*(y-0.08)))
			}
		}
		p := make([]float64, g.TotalCC())
		cfg := DefaultConfig()
		res := SolvePoissonPCG(g, f, b, p, cfg.Numerical.PoissonTol, cfg.Numerical.PoissonMaxIter, 1, allocScratch(g))
		if !res.Converged {
			t.Errorf("ratio %-6g: PCG did not converge: %+v", ratio, res)
		}
		t.Logf("density ratio %-6g iters=%4d relRes=%.2e", ratio, res.Iterations, res.Residual)
	}
}

// --- Step 3c: operator symmetry + positivity ----------------------------------

func TestPoissonOperatorSymmetricPositiveDefinite(t *testing.T) {
	g := NewGrid(12, 16, 0.2286, 0.3429)
	f := layeredFields(g, 1000, 1) // variable coefficients

	rng := rand.New(rand.NewSource(42))
	n := g.TotalCC()
	x := make([]float64, n)
	y := make([]float64, n)
	// Random INTERIOR values; ghosts must be zero because the operator only
	// defines entries for interior rows (top row reads ghost p at j=Ny+1).
	randomizeInterior := func(v []float64) {
		for i := range v {
			v[i] = 0
		}
		for j := 1; j <= g.Ny; j++ {
			for i := 1; i <= g.Nx; i++ {
				v[g.idxCC(i, j)] = rng.NormFloat64()
			}
		}
	}
	randomizeInterior(x)
	randomizeInterior(y)
	Ax := make([]float64, n)
	Ay := make([]float64, n)
	ApplyOperator(g, f, x, Ax)
	ApplyOperator(g, f, y, Ay)

	ytAx := 0.0
	xtAy := 0.0
	for i := range x {
		ytAx += y[i] * Ax[i]
		xtAy += x[i] * Ay[i]
	}
	if math.Abs(ytAx-xtAy) > 1e-9*math.Max(1, math.Abs(ytAx)) {
		t.Errorf("operator not symmetric: yAx=%.12e xAy=%.12e", ytAx, xtAy)
	}

	// Positive on random non-zero vectors: x^T A x > 0.
	for trial := 0; trial < 20; trial++ {
		randomizeInterior(x)
		ApplyOperator(g, f, x, Ax)
		quad := 0.0
		for i := range x {
			quad += x[i] * Ax[i]
		}
		if quad <= 0 {
			t.Errorf("trial %d: x^T A x = %e <= 0", trial, quad)
		}
	}
}
