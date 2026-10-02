package solver

import (
	"testing"
)

func TestInterpolateRhoToFaces(t *testing.T) {
	nx, ny := 10, 10
	cfg := DefaultConfig()
	g := NewGrid(nx, ny, 1.0, 1.0)
	f := NewFields(g)

	// Set unique density in every cell
	for j := 0; j < g.NyG; j++ {
		for i := 0; i < g.NxG; i++ {
			f.Rho[g.idxCC(i, j)] = float64(1000*i + j)
		}
	}

	testCases := []struct {
		name string
		face string // "arithmetic" or "harmonic"
	}{
		{"Arithmetic", "arithmetic"},
		{"Harmonic", "harmonic"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg.Numerical.FaceRhoAvg = tc.face
			InterpolateRhoToFaces(g, f, &cfg)

			// Check U faces (vertical faces between cells i and i+1)
			for j := 1; j <= g.Ny; j++ {
				for i := 1; i < g.Nx; i++ { // For interior faces
					idxU := g.idxU(i, j)
					rhoL := f.Rho[g.idxCC(i, j)]
					rhoR := f.Rho[g.idxCC(i+1, j)]

					var expected float64
					if tc.face == "harmonic" {
						expected = 2.0 / (1.0/rhoL + 1.0/rhoR)
					} else {
						expected = 0.5 * (rhoL + rhoR)
					}

					if f.RhoU[idxU] != expected {
						t.Errorf("U face (i=%d, j=%d) expected %v, got %v", i, j, expected, f.RhoU[idxU])
					}
				}
			}

			// Check V faces (horizontal faces between cells j and j+1)
			for j := 1; j < g.Ny; j++ { // For interior faces
				for i := 1; i <= g.Nx; i++ {
					idxV := g.idxV(i, j)
					rhoB := f.Rho[g.idxCC(i, j)]
					rhoT := f.Rho[g.idxCC(i, j+1)]

					var expected float64
					if tc.face == "harmonic" {
						expected = 2.0 / (1.0/rhoB + 1.0/rhoT)
					} else {
						expected = 0.5 * (rhoB + rhoT)
					}

					if f.RhoV[idxV] != expected {
						t.Errorf("V face (i=%d, j=%d) expected %v, got %v", i, j, expected, f.RhoV[idxV])
					}
				}
			}
		})
	}
}
