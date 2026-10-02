package solver

import "math"

func sumAlpha(g *Grid, f *Fields) float64 {
	sum := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			sum += f.Alpha[g.idxCC(i, j)]
		}
	}
	return sum
}

// AdvectAlpha computes the advection of the volume fraction field Alpha
// using the Donor-Acceptor method of Hirt & Nichols (1981).
// It updates f.Alpha in place.
func AdvectAlpha(g *Grid, f *Fields, cfg *Config, dt float64, step int) (float64, float64, float64, float64, int) {
	f.CopyAlphaToPrev()
	startSum := sumAlpha(g, f)

	// Alternate sweep order each step to minimize directional bias
	sweepXFirst := (step%2 == 0)

	var sumAfterX, sumAfterY float64

	if sweepXFirst {
		sweepX(g, f, cfg, dt)
		ApplyAlphaBC(g, f)
		sumAfterX = sumAlpha(g, f)
		sweepY(g, f, cfg, dt)
		ApplyAlphaBC(g, f)
		sumAfterY = sumAlpha(g, f)
	} else {
		sweepY(g, f, cfg, dt)
		ApplyAlphaBC(g, f)
		sumAfterY = sumAlpha(g, f)
		sweepX(g, f, cfg, dt)
		ApplyAlphaBC(g, f)
		sumAfterX = sumAlpha(g, f)
	}

	// Determine the deltas. Note: if sweepXFirst is false, Y happened first,
	// so delta Y = sumAfterY - startSum, delta X = sumAfterX - sumAfterY
	var deltaX, deltaY float64
	if sweepXFirst {
		deltaX = sumAfterX - startSum
		deltaY = sumAfterY - sumAfterX
	} else {
		deltaY = sumAfterY - startSum
		deltaX = sumAfterX - sumAfterY
	}

	// 3. Clip Alpha and record stats
	preClipSum := sumAlpha(g, f)
	outflow := 0.0
	cflWarns := f.CFLWarnings
	f.CFLWarnings = 0

	if cfg.Numerical.ClipAlpha {
		for p := 0; p < 3; p++ {
			changed := false
			for j := 1; j <= g.Ny; j++ {
				for i := 1; i <= g.Nx; i++ {
					idx := g.idxCC(i, j)
					old := f.Alpha[idx]
					if old < 0.0 || old > 1.0 {
						var diff float64
						if old < 0.0 {
							diff = old
							f.Alpha[idx] = 0.0
						} else {
							diff = old - 1.0
							f.Alpha[idx] = 1.0
						}
						changed = true

						if cfg.Numerical.ClipRedistribute {
							var nbrs []int
							for _, n := range []int{g.idxCC(i-1, j), g.idxCC(i+1, j), g.idxCC(i, j-1), g.idxCC(i, j+1)} {
								// don't distribute into ghost cells
								if n >= 0 && n < len(f.Alpha) {
									// only distribute if it can accept
									a := f.Alpha[n]
									if (diff > 0 && a < 1.0) || (diff < 0 && a > 0.0) {
										nbrs = append(nbrs, n)
									}
								}
							}
							if len(nbrs) > 0 {
								share := diff / float64(len(nbrs))
								for _, n := range nbrs {
									f.Alpha[n] += share
								}
							}
						}
					}
				}
			}
			if !changed || !cfg.Numerical.ClipRedistribute {
				break
			}
		}
	}

	postClipSum := sumAlpha(g, f)
	deltaClip := postClipSum - preClipSum

	// outflow is recorded in sweepY
	outflow = f.TopOutflow
	f.TopOutflow = 0.0

	return deltaX, deltaY, deltaClip, outflow, cflWarns
}

func sweepX(g *Grid, f *Fields, cfg *Config, dt float64) {
	for j := 1; j <= g.Ny; j++ {
		// Use preallocated f.FluxX buffer (assumed to exist, will add it to Fields)
		for i := 0; i <= g.Nx; i++ {
			u := f.U[g.idxU(i, j)]
			if i == 0 || i == g.Nx {
				f.FluxX[i] = 0 // wall faces
				continue
			}
			f.FluxX[i] = computeFluxX(g, f, i, j, u, dt, cfg)
		}

		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			f.Alpha[idx] += (f.FluxX[i-1] - f.FluxX[i]) * g.InvDx

			if cfg.Numerical.DilatationCorr {
				// Add dilatation correction: dt * alpha_cell * (u_east - u_west) / dx
				uE := f.U[g.idxU(i, j)]
				uW := f.U[g.idxU(i-1, j)]
				alphaVal := f.Alpha[idx]
				if cfg.Numerical.SplitDivFix {
					alphaVal = f.AlphaPrev[idx]
				}
				divCorr := dt * alphaVal * (uE - uW) * g.InvDx
				f.Alpha[idx] += divCorr
			}
		}
	}
}

func sweepY(g *Grid, f *Fields, cfg *Config, dt float64) {
	for i := 1; i <= g.Nx; i++ {
		for j := 0; j <= g.Ny; j++ {
			v := f.V[g.idxV(i, j)]
			if j == 0 {
				f.FluxY[j] = 0 // wall face (floor)
				continue
			}
			if j == g.Ny {
				// top open boundary: fluid can leave if v > 0, but no air/water enters
				if v > 0 {
					f.FluxY[j] = computeFluxY(g, f, i, j, v, dt, cfg)
					f.TopOutflow += f.FluxY[j] * g.Dx // total volume leaving
				} else {
					f.FluxY[j] = 0
				}
				continue
			}
			f.FluxY[j] = computeFluxY(g, f, i, j, v, dt, cfg)
		}

		for j := 1; j <= g.Ny; j++ {
			idx := g.idxCC(i, j)
			f.Alpha[idx] += (f.FluxY[j-1] - f.FluxY[j]) * g.InvDy

			if cfg.Numerical.DilatationCorr {
				// Add dilatation correction for Y sweep
				vN := f.V[g.idxV(i, j)]
				vS := f.V[g.idxV(i, j-1)]
				alphaVal := f.Alpha[idx]
				if cfg.Numerical.SplitDivFix {
					alphaVal = f.AlphaPrev[idx]
				}
				divCorr := dt * alphaVal * (vN - vS) * g.InvDy
				f.Alpha[idx] += divCorr
			}
		}
	}
}

func computeFluxX(g *Grid, f *Fields, i, j int, u, dt float64, cfg *Config) float64 {
	var D, A int
	if u > 0 {
		D = i
		A = i + 1
	} else {
		D = i + 1
		A = i
	}

	alphaD := f.Alpha[g.idxCC(D, j)]
	alphaA := f.Alpha[g.idxCC(A, j)]

	fraction := math.Abs(u) * dt * g.InvDx
	if fraction > 1.0 {
		f.CFLWarnings++ // silent clamp replaced by counted warning
		fraction = 1.0
	}

	var alphaAD float64
	if cfg.Numerical.AdvectScheme == AdvectDonorAcceptor && isSteepX(g, f, D, j) {
		alphaAD = alphaA
	} else {
		alphaAD = alphaD
	}

	// Hirt & Nichols CF term
	cf := math.Max((1.0-alphaAD)*fraction-(1.0-alphaD), 0.0)
	fluxFrac := math.Min(alphaAD*fraction+cf, alphaD)

	// Keep the swept-volume cap (flux <= fraction). This is needed to ensure
	// we never advect more fluid than the total volume of fluid + air that actually
	// crosses the face in one timestep, preventing unphysical overshoots.
	fluxFrac = math.Min(fraction, fluxFrac)

	flux := fluxFrac * g.Dx
	if u < 0 {
		flux = -flux
	}
	return flux
}

func computeFluxY(g *Grid, f *Fields, i, j int, v, dt float64, cfg *Config) float64 {
	var D, A int
	if v > 0 {
		D = j
		A = j + 1
	} else {
		D = j + 1
		A = j
	}

	alphaD := f.Alpha[g.idxCC(i, D)]
	alphaA := f.Alpha[g.idxCC(i, A)]

	fraction := math.Abs(v) * dt * g.InvDy
	if fraction > 1.0 {
		f.CFLWarnings++
		fraction = 1.0
	}

	var alphaAD float64
	if cfg.Numerical.AdvectScheme == AdvectDonorAcceptor && isSteepY(g, f, i, D) {
		alphaAD = alphaA
	} else {
		alphaAD = alphaD
	}

	cf := math.Max((1.0-alphaAD)*fraction-(1.0-alphaD), 0.0)
	fluxFrac := math.Min(alphaAD*fraction+cf, alphaD)

	// Swept-volume cap: ensures flux doesn't exceed total volume crossing the face
	fluxFrac = math.Min(fraction, fluxFrac)

	flux := fluxFrac * g.Dy
	if v < 0 {
		flux = -flux
	}
	return flux
}

func isSteepX(g *Grid, f *Fields, i, j int) bool {
	dx := f.Alpha[g.idxCC(i+1, j)] - f.Alpha[g.idxCC(i-1, j)]
	dy := f.Alpha[g.idxCC(i, j+1)] - f.Alpha[g.idxCC(i, j-1)]
	return math.Abs(dx) >= math.Abs(dy)
}

func isSteepY(g *Grid, f *Fields, i, j int) bool {
	dx := f.Alpha[g.idxCC(i+1, j)] - f.Alpha[g.idxCC(i-1, j)]
	dy := f.Alpha[g.idxCC(i, j+1)] - f.Alpha[g.idxCC(i, j-1)]
	return math.Abs(dy) >= math.Abs(dx)
}
