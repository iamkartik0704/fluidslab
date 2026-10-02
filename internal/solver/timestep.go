package solver

import (
	"fmt"
	"math"
)

// ComputeDt returns the next timestep from explicit stability limits plus the
// configured caps. All limits use the max interior face velocity:
//
//	advective  : dt <= CFL * min(dx,dy) / umax
//	viscous    : dt <= ViscousCFL * min(dx,dy)^2 / max(nu)   (nu = mu/rho, worst cell)
//	gravity    : dt <= GravityCFL * sqrt(min(dx,dy) / g)     (free-surface wave cap)
//
// dt never exceeds MaxDT and grows by at most DTGrowthFactor relative to the
// previous step. umax = 0 (e.g. the initial still field) yields MaxDT.
func ComputeDt(g *Grid, f *Fields, cfg *Config, state *SimState) float64 {
	hmin := math.Min(g.Dx, g.Dy)
	umax := f.MaxAbsVel(g)

	// Effective CFL coefficients (allow capping via MaxCFLFrac)
	cflAdv := cfg.Numerical.CFL
	cflVisc := cfg.Numerical.ViscousCFL
	cflGrav := cfg.Numerical.GravityCFL
	maxDT := cfg.Numerical.MaxDT
	if cfg.Numerical.MaxCFLFrac > 0 && cfg.Numerical.MaxCFLFrac < 1.0 {
		cflAdv *= cfg.Numerical.MaxCFLFrac
		cflVisc *= cfg.Numerical.MaxCFLFrac
		cflGrav *= cfg.Numerical.MaxCFLFrac
		maxDT *= cfg.Numerical.MaxCFLFrac
	}

	dtAdv := maxDT
	if umax > 0 {
		dtAdv = cflAdv * hmin / umax
	}

	// Viscous limit: use the worst (largest) kinematic viscosity among cells
	// containing fluid; for the two-fluid mix nu_air >> nu_water governs.
	nuMax := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			if f.Rho[idx] > 0 {
				if nu := f.Mu[idx] / f.Rho[idx]; nu > nuMax {
					nuMax = nu
				}
			}
		}
	}
	dtVisc := maxDT
	if nuMax > 0 {
		// 2D explicit diffusion stability limit: dt <= h^2 / (4*nu).
		// ViscousCFL is then a safety factor on that limit.
		dtVisc = cflVisc * hmin * hmin / (4.0 * nuMax)
	}

	dtGrav := cflGrav * math.Sqrt(hmin/cfg.Physical.Gravity)

	dt := math.Min(dtAdv, math.Min(dtVisc, dtGrav))

	// Cap growth relative to the previous accepted step.
	if state.Step > 0 {
		dt = math.Min(dt, state.DT*cfg.Numerical.DTGrowthFactor)
	}
	if dt > maxDT {
		dt = maxDT
	}
	if dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		panic(fmt.Sprintf("ComputeDt produced non-finite/non-positive dt: %g", dt))
	}
	return dt
}
