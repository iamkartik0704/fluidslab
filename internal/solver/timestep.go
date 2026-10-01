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

	dtAdv := cfg.Numerical.MaxDT
	if umax > 0 {
		dtAdv = cfg.Numerical.CFL * hmin / umax
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
	dtVisc := cfg.Numerical.MaxDT
	if nuMax > 0 {
		dtVisc = cfg.Numerical.ViscousCFL * hmin * hmin / nuMax
	}

	dtGrav := cfg.Numerical.GravityCFL * math.Sqrt(hmin/cfg.Physical.Gravity)

	dt := math.Min(dtAdv, math.Min(dtVisc, dtGrav))

	// Cap growth relative to the previous accepted step.
	if state.Step > 0 {
		dt = math.Min(dt, state.DT*cfg.Numerical.DTGrowthFactor)
	}
	if dt > cfg.Numerical.MaxDT {
		dt = cfg.Numerical.MaxDT
	}
	if dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		panic(fmt.Sprintf("ComputeDt produced non-finite/non-positive dt: %g", dt))
	}
	return dt
}
