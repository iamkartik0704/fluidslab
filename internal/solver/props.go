// Package solver handles the flow solve.
//
// FACE-INDEX CONVENTION:
// In the staggered grid:
// - Cell centers (CC) are indexed (i, j) for i=0..NxG-1, j=0..NyG-1.
// - Horizontal velocity faces (U faces) are indexed idxU(i, j) = j*(NxG+1) + i.
//   The U face idxU(i, j) lies on the vertical edge BETWEEN cell (i, j) and cell (i+1, j).
// - Vertical velocity faces (V faces) are indexed idxV(i, j) = j*NxG + i.
//   The V face idxV(i, j) lies on the horizontal edge BETWEEN cell (i, j) and cell (i, j+1).
package solver

func UpdateProperties(g *Grid, f *Fields, cfg *Config) {
	for j := 0; j < g.NyG; j++ {
		for i := 0; i < g.NxG; i++ {
			idx := g.idxCC(i, j)
			alpha := f.Alpha[idx]
			if alpha < 0 { alpha = 0 }
			if alpha > 1 { alpha = 1 }
			f.Rho[idx] = alpha*cfg.Physical.RhoW + (1-alpha)*cfg.Physical.RhoA
			f.Mu[idx] = alpha*cfg.Physical.MuW + (1-alpha)*cfg.Physical.MuA
		}
	}
}

func InterpolateRhoToFaces(g *Grid, f *Fields, cfg *Config) {
	useHarmonic := cfg.Numerical.FaceRhoAvg == "harmonic"

	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idxL := g.idxCC(i, j)
			var idxR int
			if cfg.Numerical.DensityBug {
				idxR = g.idxCC(i, j)
			} else {
				idxR = g.idxCC(i+1, j)
			}
			idxU := g.idxU(i, j)

			rhoL := f.Rho[idxL]
			rhoR := f.Rho[idxR]

			if useHarmonic {
				if rhoL > 0 && rhoR > 0 {
					f.RhoU[idxU] = 2.0 / (1.0/rhoL + 1.0/rhoR)
				} else if rhoL > 0 {
					f.RhoU[idxU] = rhoL
				} else {
					f.RhoU[idxU] = rhoR
				}
			} else {
				f.RhoU[idxU] = 0.5 * (rhoL + rhoR)
			}
		}
	}

	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idxB := g.idxCC(i, j)
			var idxT int
			if cfg.Numerical.DensityBug {
				idxT = g.idxCC(i, j)
			} else {
				idxT = g.idxCC(i, j+1)
			}
			idxV := g.idxV(i, j)

			rhoB := f.Rho[idxB]
			rhoT := f.Rho[idxT]

			if useHarmonic {
				if rhoB > 0 && rhoT > 0 {
					f.RhoV[idxV] = 2.0 / (1.0/rhoB + 1.0/rhoT)
				} else if rhoB > 0 {
					f.RhoV[idxV] = rhoB
				} else {
					f.RhoV[idxV] = rhoT
				}
			} else {
				f.RhoV[idxV] = 0.5 * (rhoB + rhoT)
			}
		}
	}
}

func ComputeFaceMu(g *Grid, f *Fields, muU, muV []float64) {
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idxL := g.idxCC(i-1, j)
			idxR := g.idxCC(i, j)
			idxU := g.idxU(i, j)
			muU[idxU] = 0.5 * (f.Mu[idxL] + f.Mu[idxR])
		}
	}

	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idxB := g.idxCC(i, j-1)
			idxT := g.idxCC(i, j)
			idxV := g.idxV(i, j)
			muV[idxV] = 0.5 * (f.Mu[idxB] + f.Mu[idxT])
		}
	}
}

