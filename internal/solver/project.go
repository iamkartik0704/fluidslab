package solver

// Projection: solve the variable-density Poisson equation for the pressure
// correction p and make the predicted face velocities divergence-free:
//
//	div( (1/rho) grad p ) = div(u*)/dt
//	u = u* - dt * (1/rho_f) * grad p   (at faces)
//
// Face coefficients (1/rho_f from RhoU/RhoV) and BCs (Neumann at walls,
// Dirichlet p=0 at the open top) are identical to poisson.go.

import "math"

// Project solves the projection Poisson problem and corrects the velocities.
// scratch carries [z, r, s, Ap, b] — five preallocated TotalCC arrays
// (Simulation allocates them once).
func Project(g *Grid, f *Fields, cfg *Config, dt float64, scratch [][]float64) PoissonResult {
	// Dirichlet ghost value for the warm-started solve: the operator reads
	// p[Ny+1] on the top row, so it must hold the boundary value 0.
	for i := 0; i < g.NxG; i++ {
		f.P[g.idxCC(i, g.Ny+1)] = 0
	}

	// RHS: div(u*)/dt on interior cells.
	b := scratch[8]
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			div := (f.UStar[g.EastVFace(i, j)]-f.UStar[g.WestVFace(i, j)])*g.InvDx +
				(f.VStar[g.NorthHFace(i, j)]-f.VStar[g.SouthHFace(i, j)])*g.InvDy
			// Sign: our operator is A = -(1/rho) Lap, and the correction gives
			// div(u_corr) = div(u*) + dt * A p  =>  A p = -div(u*)/dt.
			b[idx] = -div / dt
		}
	}

	// Initial guess: previous pressure (warm start).
	p := f.P

	res := SolvePoissonPCG(g, f, b, p, cfg.Numerical.PoissonTol, cfg.Numerical.PoissonMaxIter, cfg.Threads, scratch)

	// Correct face velocities: u = u* - dt/rho_f * dp/dx.
	for j := 1; j <= g.Ny; j++ {
		p[g.idxCC(0, j)] = p[g.idxCC(1, j)]         // left wall (Neumann)
		p[g.idxCC(g.Nx+1, j)] = p[g.idxCC(g.Nx, j)] // right wall (Neumann)
	}
	for i := 0; i < g.NxG; i++ {
		p[g.idxCC(i, 0)] = p[g.idxCC(i, 1)] // floor (Neumann)
		p[g.idxCC(i, g.Ny+1)] = 0           // top ghost: Dirichlet value at face
	}

	// Face idxU(i,j) at x=i*dx lies BETWEEN cell centres (i,j) and (i+1,j)
	// (Xc[i]=(i-1/2)dx, Xu[i]=i*dx), so its gradient is (p[i+1,j]-p[i,j])/dx.
	// Likewise idxV(i,j) at y=j*dy lies between (i,j) and (i,j+1).
	// TOP FACE (open top): the operator imposes Dirichlet p=0 AT the face
	// y=H (doubled coefficient 2/rho/dy^2), so the correction gradient at the
	// top face is the matching one-sided value 2*(0-p[Ny])/dy.
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ {
			idx := g.idxU(i, j)
			f.U[idx] = f.UStar[idx] - dt*(p[g.idxCC(i+1, j)]-p[g.idxCC(i, j)])*g.InvDx/f.RhoU[idx]
		}
	}
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxV(i, j)
			grad := (p[g.idxCC(i, j+1)] - p[g.idxCC(i, j)]) * g.InvDy // interior faces
			if j == g.Ny {
				if g.hasPin {
					grad = 0 // solid lid: Neumann, matching the operator
				} else {
					grad = 2 * (0 - p[g.idxCC(i, g.Ny)]) * g.InvDy // Dirichlet at face
				}
			}
			f.V[idx] = f.VStar[idx] - dt*grad/f.RhoV[idx]
		}
	}

	// Post-projection divergence for diagnostics.
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			f.DivU[idx] = (f.U[g.EastVFace(i, j)]-f.U[g.WestVFace(i, j)])*g.InvDx +
				(f.V[g.NorthHFace(i, j)]-f.V[g.SouthHFace(i, j)])*g.InvDy
		}
	}

	return res
}

// MaxAbsDiv returns max|div u| over interior cells.
func MaxAbsDiv(g *Grid, f *Fields) float64 {
	m := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			if d := math.Abs(f.DivU[g.idxCC(i, j)]); d > m {
				m = d
			}
		}
	}
	return m
}
