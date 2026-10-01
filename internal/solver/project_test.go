package solver

import (
	"math"
	"math/rand"
	"testing"
)

// Set a still two-layer state: water below hW, air above, u=v=0.
func stillTwoLayer(g *Grid, f *Fields, cfg *Config, hW float64) {
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			if g.Yc[idx] < hW {
				f.Alpha[idx] = 1
			} else {
				f.Alpha[idx] = 0
			}
		}
	}
	ApplyAlphaBC(g, f)
	UpdateProperties(g, f, cfg)
	InterpolateRhoToFaces(g, f, cfg)
	ApplyVelocityBC(f, cfg, g)
}

// --- Step 4a: random divergent field, constant density ------------------------

func TestProjectionDivergenceFreeConstantDensity(t *testing.T) {
	cfg := DefaultConfig()
	g := NewGrid(32, 48, 0.2286, 0.3429)
	f := constantDensityFields(g)
	rng := rand.New(rand.NewSource(7))

	// Random interior face velocities -> wildly divergent u*.
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ {
			f.UStar[g.idxU(i, j)] = rng.NormFloat64()
		}
	}
	for j := 1; j < g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			f.VStar[g.idxV(i, j)] = rng.NormFloat64()
		}
	}
	ApplyVelocityBC(f, &cfg, g)
	ApplyStarBC(f, &cfg, g)

	dt := 1e-3
	// max|div| = dt * |solve residual| ~ dt * relRes * ||b||; hitting 1e-8 at
	// dt=1e-3 needs relRes ~1e-12 (the default 1e-8 is the time-stepping tol).
	cfg.Numerical.PoissonTol = 1e-12
	cfg.Numerical.PoissonMaxIter = 50000
	res := Project(g, f, &cfg, dt, allocScratch8(g))
	if !res.Converged {
		t.Fatalf("projection PCG did not converge: %+v", res)
	}
	maxDiv := MaxAbsDiv(g, f)
	t.Logf("constant density: iters=%d relRes=%.2e  max|div|=%.3e", res.Iterations, res.Residual, maxDiv)
	if maxDiv > 1e-8 {
		t.Errorf("max|div| = %.3e > 1e-8", maxDiv)
	}
}

// --- Step 4b: variable density ------------------------------------------------

func TestProjectionVariableDensity(t *testing.T) {
	cfg := DefaultConfig()
	g := NewGrid(32, 48, 0.2286, 0.3429)
	f := NewFields(g)
	stillTwoLayer(g, f, &cfg, 0.1)
	rng := rand.New(rand.NewSource(11))

	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ {
			f.UStar[g.idxU(i, j)] = rng.NormFloat64()
		}
	}
	for j := 1; j < g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			f.VStar[g.idxV(i, j)] = rng.NormFloat64()
		}
	}
	ApplyVelocityBC(f, &cfg, g)
	ApplyStarBC(f, &cfg, g)

	dt := 1e-3
	// Spec: max|div| below the Poisson tolerance at RHS scale (div(u)/dt).
	// PCG stops on the L2-RELATIVE residual; max|div| is an L∞ quantity, and
	// for this random field the two norms differ by a stable factor ~1.5, so
	// the assertion carries a 5x slack (measured, deterministic seed).
	cfg.Numerical.PoissonTol = 1e-12
	cfg.Numerical.PoissonMaxIter = 50000
	res := Project(g, f, &cfg, dt, allocScratch8(g))
	if !res.Converged {
		t.Fatalf("projection PCG did not converge: %+v", res)
	}
	maxDiv := MaxAbsDiv(g, f)
	t.Logf("variable density: iters=%d relRes=%.2e  max|div|=%.3e (tol %.1e)",
		res.Iterations, res.Residual, maxDiv, cfg.Numerical.PoissonTol)
	if maxDiv > 5*cfg.Numerical.PoissonTol/dt {
		t.Errorf("max|div| = %.3e exceeds 5*tol/dt = %.3e", maxDiv, 5*cfg.Numerical.PoissonTol/dt)
	}
}

// --- Step 4c: HYDROSTATIC GATE ------------------------------------------------
//
// Still water with air above, gravity on, 200 steps. Velocity must stay tiny
// and pressure must approach rho*g*depth. Consistency gate between the density
// interpolation and the Poisson operator.
func TestHydrostaticStillWaterGate(t *testing.T) {
	cfg := DefaultConfig()
	g := NewGrid(48, 72, 0.2286, 0.3429)
	f := NewFields(g)
	hW := 2 * cfg.Domain.H0
	stillTwoLayer(g, f, &cfg, hW)

	scratch := allocScratch8(g)
	muScratch := allocMuScratch(g)
	
	var st SimState
	maxVel := 0.0
	for step := 1; step <= 200; step++ {
		dt := ComputeDt(g, f, &cfg, &st)
		Predict(g, f, &cfg, dt, muScratch)
		ApplyStarBC(f, &cfg, g)
		res := Project(g, f, &cfg, dt, scratch)
		if !res.Converged {
			t.Fatalf("step %d: projection failed: %+v", step, res)
		}
		ApplyVelocityBC(f, &cfg, g)
		st.Step = step
		st.DT = dt
		v := f.MaxAbsVel(g)
		if v > maxVel {
			maxVel = v
		}
	}
	t.Logf("hydrostatic gate: max|vel| over 200 steps = %.3e m/s", maxVel)
	if maxVel > 1e-6 {
		t.Errorf("hydrostatic max velocity %.3e > 1e-6 m/s", maxVel)
	}

	pErr := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			if g.Yc[idx] < 0.9*hW {
				// The total pressure includes the weight of the air column above the water.
				airWeight := cfg.Physical.RhoA * cfg.Physical.Gravity * (cfg.Domain.Height - hW)
				want := cfg.Physical.RhoW * cfg.Physical.Gravity * (hW - g.Yc[idx]) + airWeight
				if d := math.Abs(f.P[idx] - want); d > pErr {
					pErr = d
				}
			}
		}
	}
	lim := 1e-6 * cfg.Physical.RhoW * cfg.Physical.Gravity * hW
	t.Logf("hydrostatic pressure: max|p - p_exact| = %.3e Pa (limit %.3e)", pErr, lim)
	if pErr > lim {
		t.Errorf("hydrostatic pressure error %.3e Pa exceeds 1e-6 of rho*g*H", pErr)
	}
}

func TestHydrostaticThreeLayerGate(t *testing.T) {
	cfg := DefaultConfig()
	g := NewGrid(10, 10, 1.0, 1.0)
	f := NewFields(g)
	
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			if g.Yc[idx] < 0.3 || g.Yc[idx] > 0.7 {
				f.Alpha[idx] = 1.0
			} else {
				f.Alpha[idx] = 0.0
			}
		}
	}
	ApplyAlphaBC(g, f)
	UpdateProperties(g, f, &cfg)
	InterpolateRhoToFaces(g, f, &cfg)
	ApplyVelocityBC(f, &cfg, g)

	scratch := allocScratch8(g)
	muScratch := allocMuScratch(g)
	
	dt := 0.01
	
	Predict(g, f, &cfg, dt, muScratch)
	ApplyStarBC(f, &cfg, g)
	res := Project(g, f, &cfg, dt, scratch)
	if !res.Converged {
		t.Fatalf("projection failed: %+v", res)
	}
	ApplyVelocityBC(f, &cfg, g)

	maxVel := f.MaxAbsVel(g)
	if maxVel > 1e-6 {
		t.Errorf("3-layer hydrostatic max velocity %.3e > 1e-6 m/s", maxVel)
	}
}
