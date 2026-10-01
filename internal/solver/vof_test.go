package solver

import (
	"math"
	"testing"
)

func TestVOFRigidBodyRotation(t *testing.T) {
	nx, ny := 100, 100
	L := 1.0

	tests := []struct {
		name   string
		scheme AdvectScheme
	}{
		{"DonorAcceptor", AdvectDonorAcceptor},
		{"Upwind", AdvectUpwind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrid(nx, ny, L, L)
			f := NewFields(g)
			cfg := DefaultConfig()
			cfg.Numerical.ClipAlpha = true
			cfg.Numerical.AdvectScheme = tt.scheme

			cx, cy := 0.5, 0.5
			omega := 2.0 * math.Pi
			for j := 0; j <= g.Ny; j++ {
				for i := 0; i <= g.Nx; i++ {
					if i <= g.Nx {
						y := g.Yu[g.idxU(i, j)]
						f.U[g.idxU(i, j)] = -omega * (y - cy)
					}
					if j <= g.Ny {
						x := g.Xv[g.idxV(i, j)]
						f.V[g.idxV(i, j)] = omega * (x - cx)
					}
				}
			}

			r := 0.15
			bx, by := 0.5, 0.75
			perimeterCells := 2.0 * math.Pi * r / g.Dx
			initialMass := 0.0
			smearedCellsInitial := 0
			
			// Initialize
			for j := 1; j <= g.Ny; j++ {
				for i := 1; i <= g.Nx; i++ {
					x := g.Xc[g.idxCC(i, j)]
					y := g.Yc[g.idxCC(i, j)]
					dist := math.Sqrt((x-bx)*(x-bx) + (y-by)*(y-by))
					if dist <= r {
						f.Alpha[g.idxCC(i, j)] = 1.0
					} else {
						f.Alpha[g.idxCC(i, j)] = 0.0
					}
					alpha := f.Alpha[g.idxCC(i, j)]
					initialMass += alpha
					if alpha > 0.01 && alpha < 0.99 {
						smearedCellsInitial++
					}
				}
			}
			ApplyAlphaBC(g, f)

			// Run 1 rotation
			dt := 0.002
			steps := int(1.0 / dt)
			for s := 0; s < steps; s++ {
				AdvectAlpha(g, f, &cfg, dt, s)
			}

			// Final metrics
			finalMass := 0.0
			smearedCells := 0
			l1Error := 0.0
			
			for j := 1; j <= g.Ny; j++ {
				for i := 1; i <= g.Nx; i++ {
					alpha := f.Alpha[g.idxCC(i, j)]
					finalMass += alpha
					
					// Recompute initial shape for L1 error
					x := g.Xc[g.idxCC(i, j)]
					y := g.Yc[g.idxCC(i, j)]
					dist := math.Sqrt((x-bx)*(x-bx) + (y-by)*(y-by))
					expected := 0.0
					if dist <= r {
						expected = 1.0
					}
					l1Error += math.Abs(alpha - expected) * g.Dx * g.Dy

					// "smeared cells" defined as 0.01 < alpha < 0.99
					if alpha > 0.01 && alpha < 0.99 {
						smearedCells++
					}
				}
			}

			volErrPct := 100.0 * (finalMass - initialMass) / initialMass
			interfaceWidth := float64(smearedCells) / perimeterCells
			
			t.Logf("Scheme: %s", tt.name)
			t.Logf("  Init smeared cells: %d, Perimeter cells: %.1f", smearedCellsInitial, perimeterCells)
			t.Logf("  Vol Err: %.3e %%", volErrPct)
			t.Logf("  L1 Shape Error: %.6f", l1Error)
			t.Logf("  Smeared Cells: %d", smearedCells)
			t.Logf("  Interface Width (cells): %.2f", interfaceWidth)
			
			if tt.scheme == AdvectDonorAcceptor && smearedCells > 350 {
				t.Errorf("Interface smeared too much: %d", smearedCells)
			}
		})
	}
}

func TestVOFTranslation(t *testing.T) {
	nx, ny := 100, 100
	L := 1.0
	g := NewGrid(nx, ny, L, L)
	f := NewFields(g)
	cfg := DefaultConfig()
	cfg.Numerical.ClipAlpha = true
	cfg.Numerical.AdvectScheme = AdvectDonorAcceptor

	// Uniform velocity
	uVel := 1.0
	vVel := 0.5
	for j := 0; j <= g.Ny; j++ {
		for i := 0; i <= g.Nx; i++ {
			if i <= g.Nx {
				f.U[g.idxU(i, j)] = uVel
			}
			if j <= g.Ny {
				f.V[g.idxV(i, j)] = vVel
			}
		}
	}

	// Initialize block
	initialMass := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			x := g.Xc[g.idxCC(i, j)]
			y := g.Yc[g.idxCC(i, j)]
			if x > 0.1 && x < 0.4 && y > 0.1 && y < 0.4 {
				f.Alpha[g.idxCC(i, j)] = 1.0
			}
			initialMass += f.Alpha[g.idxCC(i, j)]
		}
	}
	ApplyAlphaBC(g, f)

	dt := 0.001
	steps := 100 // move by 0.1 in X, 0.05 in Y
	for s := 0; s < steps; s++ {
		AdvectAlpha(g, f, &cfg, dt, s)
	}

	finalMass := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			finalMass += f.Alpha[g.idxCC(i, j)]
		}
	}

	err := math.Abs(finalMass - initialMass)
	t.Logf("Translation Initial Mass: %.15e", initialMass)
	t.Logf("Translation Final Mass:   %.15e", finalMass)
	t.Logf("Translation Abs Error:    %.15e", err)
	
	if err > 1e-10 {
		t.Errorf("Translation mass error too large: %.15e", err)
	}
}


func TestHirtNicholsCF(t *testing.T) {
	cfg := DefaultConfig()
	sim := NewSimulation(cfg, 4, 1, 4.0, 1.0, false)
	g, f := sim.Grid, sim.Fields
	f.Alpha[g.IdxCC(1, 1)] = 1.0
	f.Alpha[g.IdxCC(2, 1)] = 0.8
	f.Alpha[g.IdxCC(3, 1)] = 0.2
	f.Alpha[g.IdxCC(4, 1)] = 0.0
	flux := computeFluxX(g, f, 2, 1, 0.5, 1.0, &cfg)
	if math.Abs(flux-0.3) > 1e-6 {
		t.Errorf("Hirt Nichols CF broken: expected flux 0.3, got %f", flux)
	}
}
