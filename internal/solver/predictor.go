package solver

import "math"

// Predictor: u* = u + dt*( -adv(u) + (1/rho) div(mu grad u) + g )
// (pressure excluded — it is added by the projection).
//
//   - Advection: first-order upwind (donor-cell) by default. When
//     SecondOrderAdvect is enabled, van Leer TVD limited slopes are used.
//   - Viscosity: conservative variable-mu form div(mu grad u) evaluated at
//     faces with face-averaged mu (ComputeFaceMu, props.go), divided by the
//     face density.
//   - Gravity: body force -g on v (y points UP in this grid; the dam sits on
//     the floor at j=0).
func Predict(g *Grid, f *Fields, cfg *Config, dt float64, scratch [][]float64) {
	muU := make([]float64, g.TotalU()) // replaced by caller scratch when provided
	muV := make([]float64, g.TotalV())
	if scratch != nil {
		muU, muV = scratch[0], scratch[1]
	}
	predict(g, f, cfg, dt, muU, muV)
}

// vanLeerLimiter returns ψ(r) = (r+|r|)/(1+|r|) for the van Leer TVD limiter.
// r is the ratio of consecutive gradients: r = (u_C - u_UU) / (u_D - u_C)
// where UU is the upwind-upwind node, D is the downwind node, C is the current node.
func vanLeerLimiter(r float64) float64 {
	if r <= 0 {
		return 0
	}
	return (r + math.Abs(r)) / (1 + math.Abs(r))
}

// upwindGrad computes the upwind derivative with optional van Leer TVD correction.
// uUU is the upwind-upwind value, uU is the upwind value, uC is the cell value,
// uD is the downwind value. h is the grid spacing (dx or dy).
// If secondOrder is false, returns plain first-order upwind gradient.
func tvdGrad(uUU, uU, uC, uD, h float64, secondOrder bool) float64 {
	grad1 := (uC - uU) / h
	if !secondOrder {
		return grad1
	}

	dDown := uD - uC
	dUp := uC - uU
	dUpUp := uU - uUU

	rC := 0.0
	if math.Abs(dDown) > 1e-30 {
		rC = dUp / dDown
	}

	rU := 0.0
	if math.Abs(dUp) > 1e-30 {
		rU = dUpUp / dUp
	}

	psiC := vanLeerLimiter(rC)
	psiU := vanLeerLimiter(rU)

	fluxOut := uC + 0.5*psiC*dDown
	fluxIn := uU + 0.5*psiU*dUp

	return (fluxOut - fluxIn) / h
}

// predict is the shared implementation; muU/muV are caller-owned scratch
// (Simulation preallocates once; tests may allocate per call).
func predict(g *Grid, f *Fields, cfg *Config, dt float64, muU, muV []float64) {
	ComputeFaceMu(g, f, muU, muV)

	invDx := g.InvDx
	invDy := g.InvDy
	so := cfg.Numerical.SecondOrderAdvect

	// ---- u-momentum (u faces) ----
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ { // wall faces stay 0
			idx := g.idxU(i, j)
			rho := f.RhoU[idx]

			// --- advection: u du/dx + v du/dy ---
			uE := f.U[g.idxU(i+1, j)]
			uW := f.U[g.idxU(i-1, j)]
			uC := f.U[idx]
			dudx := 0.0
			if uC >= 0 {
				if so && i >= 2 {
					uWW := f.U[g.idxU(i-2, j)]
					dudx = tvdGrad(uWW, uW, uC, uE, g.Dx, true)
				} else if so {
					dudx = 0.5 * (uE - uW) * invDx
				} else {
					dudx = (uC - uW) * invDx
				}
			} else {
				if so && i+2 <= g.Nx {
					uEE := f.U[g.idxU(i+2, j)]
					dudx = tvdGrad(uEE, uE, uC, uW, g.Dx, true)
					dudx = -dudx // flip sign: downwind is to the left
				} else if so {
					dudx = 0.5 * (uE - uW) * invDx
				} else {
					dudx = (uE - uC) * invDx
				}
			}
			// v on the faces bounding the u-point (averaged from 4 neighbours).
			vSE := f.V[g.idxV(i, j-1)]
			vNE := f.V[g.idxV(i, j)]
			vSW := f.V[g.idxV(i-1, j-1)]
			vNW := f.V[g.idxV(i-1, j)]
			vAtU := 0.25 * (vSE + vNE + vSW + vNW)
			// vAtU >= 0 -> take u from below (j-1); else from above (j+1).
			uS := f.U[g.idxU(i, j-1)]
			uN := f.U[g.idxU(i, j+1)]
			dudy := 0.0
			if vAtU >= 0 {
				if so && j >= 2 {
					uSS := f.U[g.idxU(i, j-2)]
					dudy = tvdGrad(uSS, uS, uC, uN, g.Dy, true)
				} else if so {
					dudy = 0.5 * (uN - uS) * invDy
				} else {
					dudy = (uC - uS) * invDy
				}
			} else {
				if so && j+2 <= g.Ny+1 {
					uNN := f.U[g.idxU(i, j+2)]
					dudy = tvdGrad(uNN, uN, uC, uS, g.Dy, true)
					dudy = -dudy
				} else if so {
					dudy = 0.5 * (uN - uS) * invDy
				} else {
					dudy = (uN - uC) * invDy
				}
			}
			adv := uC*dudx + vAtU*dudy

			// --- viscous: div(mu grad u)/rho, conservative form ---
			muE := muU[g.idxU(i+1, j)]
			muW := muU[g.idxU(i-1, j)]
			muC := muU[idx]
			// mu*ux on the three x-stencils; muC is the face-centre value.
			uxE := (uE - uC) * invDx
			uxW := (uC - uW) * invDx
			lapX := (muE*uxE - muW*uxW) * invDx
			// y-derivative uses u-centred-in-y; ghost rows carry the wall image.
			uyN := (uN - uC) * invDy
			uyS := (uC - uS) * invDy
			muNf := 0.5 * (muC + muV[g.idxV(i, j)])
			muSf := 0.5 * (muC + muV[g.idxV(i, j-1)])
			lapY := (muNf*uyN - muSf*uyS) * invDy

			if cfg.Numerical.SkipAdvection {
				adv = 0
			}
			if cfg.Numerical.SkipViscosity {
				lapX, lapY = 0, 0
			}
			f.UStar[idx] = uC + dt*(-adv+(lapX+lapY)/rho)
		}
	}

	// ---- v-momentum (v faces) ----
	for j := 1; j < g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxV(i, j)
			rho := f.RhoV[idx]

			uSE := f.U[g.idxU(i, j)]     // u right of the v-point, lower row
			uNE := f.U[g.idxU(i, j+1)]   // u right of the v-point, upper row
			uSW := f.U[g.idxU(i-1, j)]   // u left, lower row
			uNW := f.U[g.idxU(i-1, j+1)] // u left, upper row
			uAtV := 0.25 * (uSE + uNE + uSW + uNW)

			vC := f.V[idx]
			vS := f.V[g.idxV(i, j-1)]
			vN := f.V[g.idxV(i, j+1)]
			dvdy := 0.0
			if vC >= 0 {
				if so && j >= 2 {
					vSS := f.V[g.idxV(i, j-2)]
					dvdy = tvdGrad(vSS, vS, vC, vN, g.Dy, true)
				} else if so {
					dvdy = 0.5 * (vN - vS) * invDy
				} else {
					dvdy = (vC - vS) * invDy
				}
			} else {
				if so && j+2 <= g.Ny {
					vNN := f.V[g.idxV(i, j+2)]
					dvdy = tvdGrad(vNN, vN, vC, vS, g.Dy, true)
					dvdy = -dvdy
				} else if so {
					dvdy = 0.5 * (vN - vS) * invDy
				} else {
					dvdy = (vN - vC) * invDy
				}
			}
			dvdx := 0.0
			if uAtV >= 0 {
				if so && i >= 2 {
					vWW := f.V[g.idxV(i-2, j)]
					vW := f.V[g.idxV(i-1, j)]
					vE := f.V[g.idxV(i+1, j)]
					dvdx = tvdGrad(vWW, vW, vC, vE, g.Dx, true)
				} else if so {
					dvdx = 0.5 * (f.V[g.idxV(i+1, j)] - f.V[g.idxV(i-1, j)]) * invDx
				} else {
					dvdx = (vC - f.V[g.idxV(i-1, j)]) * invDx
				}
			} else {
				if so && i+2 <= g.Nx+1 {
					vEE := f.V[g.idxV(i+2, j)]
					vE := f.V[g.idxV(i+1, j)]
					vW := f.V[g.idxV(i-1, j)]
					dvdx = tvdGrad(vEE, vE, vC, vW, g.Dx, true)
					dvdx = -dvdx
				} else if so {
					dvdx = 0.5 * (f.V[g.idxV(i+1, j)] - f.V[g.idxV(i-1, j)]) * invDx
				} else {
					dvdx = (f.V[g.idxV(i+1, j)] - vC) * invDx
				}
			}
			adv := uAtV*dvdx + vC*dvdy

			// viscous: x-direction mirrors the u-case; y-direction is direct.
			muE := muU[g.idxU(i, j)]
			muW := muU[g.idxU(i-1, j)]
			muCv := muV[idx]
			vxE := (f.V[g.idxV(i+1, j)] - vC) * invDx
			vxW := (vC - f.V[g.idxV(i-1, j)]) * invDx
			muEf := 0.5 * (muCv + muE)
			muWf := 0.5 * (muCv + muW)
			lapX := (muEf*vxE - muWf*vxW) * invDx
			vyN := (vN - vC) * invDy
			vyS := (vC - vS) * invDy
			muN := muV[g.idxV(i, j+1)]
			muS := muV[g.idxV(i, j-1)]
			lapY := (muN*vyN - muS*vyS) * invDy

			if cfg.Numerical.SkipAdvection {
				adv = 0
			}
			if cfg.Numerical.SkipViscosity {
				lapX, lapY = 0, 0
			}
			f.VStar[idx] = vC + dt*(-adv+(lapX+lapY)/rho) - dt*cfg.Physical.Gravity
		}
	}
}


func TvdGradExport(uUU, uU, uC, uD, h float64, secondOrder bool) float64 { return tvdGrad(uUU, uU, uC, uD, h, secondOrder) }
