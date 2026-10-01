package solver

// Boundary conditions on the staggered grid.
//
// Pressure BC (documented here, ENFORCED INSIDE the Poisson operator in
// poisson.go — there is deliberately no ghost-cell pressure hack):
//   - Solid walls (left, right, floor): homogeneous Neumann dP/dn = 0. This is
//     realised in the matrix by a ZERO coefficient on the face between the
//     interior cell and the wall ghost cell (no flux through a solid face).
//   - Open top: Dirichlet p = 0 AT the ghost cell centre (row j == Ny+1).
//     The operator keeps the ordinary full-cell north coefficient 1/rho/dy^2,
//     so the solve and the face-correction gradient share one stencil. The
//     Dirichlet row makes the operator non-singular, so no null-space
//     (mean-pressure) pinning is required.
//     A fully enclosed case would need one pinned cell instead.

// ApplyVelocityBC enforces wall velocities.
//   - Normal components: u on the left/right wall faces and v on the floor face
//     are set to exactly zero (they are real MAC faces, set directly).
//   - Top is open: zero-gradient, v[Ny] := v[Ny-1].
//   - Tangential components (no-slip default): the tangential ghost value is
//     set to minus the interior value so a central-difference wall shear
//     evaluates to 2*u_interior/dn.
//   - Free-slip (cfg.Numerical.FreeSlip): tangential ghost = +interior.
// ApplyStarBC imposes the same constraints on the predictor output UStar/VStar
// so the Poisson RHS is consistent at the open boundary: wall faces zero,
// zero-gradient at the top.
func ApplyStarBC(f *Fields, cfg *Config, g *Grid) {
	for j := 0; j < g.NyG; j++ {
		f.UStar[g.LeftWallU(j)] = 0
		f.UStar[g.RightWallU(j)] = 0
	}
	for i := 0; i < g.NxG; i++ {
		f.VStar[g.FloorV(i)] = 0
		if cfg.Numerical.OpenTop {
			f.VStar[g.TopV(i)] = f.VStar[g.idxV(i, g.Ny-1)] // zero-gradient outflow
		} else {
			f.VStar[g.TopV(i)] = 0 // solid lid: zero flux
		}
	}
}

func ApplyVelocityBC(f *Fields, cfg *Config, g *Grid) {
	freeSlip := cfg.Numerical.FreeSlip
	tangSign := -1.0 // no-slip
	if freeSlip {
		tangSign = +1.0
	}

	// Left/right walls: u faces ON the wall (i==0 and i==Nx) are zero.
	for j := 0; j < g.NyG; j++ {
		f.U[g.LeftWallU(j)] = 0
		f.U[g.RightWallU(j)] = 0
		// Tangential (v) ghost columns just inside the wall image arrays.
		vL := tangSign * f.V[g.idxV(1, j)]
		vR := tangSign * f.V[g.idxV(g.Nx, j)]
		f.V[g.idxV(0, j)] = vL
		f.V[g.idxV(g.Nx+1, j)] = vR
	}

	// Floor: v faces ON the floor (j==0) are zero.
	for i := 0; i < g.NxG; i++ {
		f.V[g.FloorV(i)] = 0
		// Tangential (u) ghost row below the floor.
		f.U[g.idxU(i, 0)] = tangSign * f.U[g.idxU(i, 1)]
	}

	// Top boundary.
	if cfg.Numerical.OpenTop {
		// Open: zero-gradient for the boundary v-face, tangential ghost zero-
		// gradient too (outflow carries no imposed shear).
		for i := 0; i < g.NxG; i++ {
			f.V[g.TopV(i)] = f.V[g.idxV(i, g.Ny-1)]
			f.U[g.idxU(i, g.Ny+1)] = f.U[g.idxU(i, g.Ny)]
		}
	} else {
		// Closed lid (cavity benchmark): normal velocity zero on the top face,
		// tangential u set to the lid velocity on INTERIOR faces only — the
		// corner u faces i==0/i==Nx are wall-normal and must stay zero.
		for i := 0; i < g.NxG; i++ {
			f.V[g.TopV(i)] = 0
		}
		for i := 1; i < g.Nx; i++ {
			f.U[g.idxU(i, g.Ny+1)] = 2.0*cfg.Numerical.LidVelocity + tangSign*f.U[g.idxU(i, g.Ny)]
		}
	}
}

// ApplyAlphaBC applies zero-gradient (no-flux) conditions for the volume
// fraction at all walls and at the open top, by filling the ghost cells.
func ApplyAlphaBC(g *Grid, f *Fields) {
	for j := 0; j < g.NyG; j++ {
		f.Alpha[g.idxCC(0, j)] = f.Alpha[g.idxCC(1, j)]
		f.Alpha[g.idxCC(g.Nx+1, j)] = f.Alpha[g.idxCC(g.Nx, j)]
	}
	for i := 0; i < g.NxG; i++ {
		f.Alpha[g.idxCC(i, 0)] = f.Alpha[g.idxCC(i, 1)]
		f.Alpha[g.idxCC(i, g.Ny+1)] = f.Alpha[g.idxCC(i, g.Ny)]
	}
}

// ApplyPressureBC exists for diagnostics only: the pressure BC lives in the
// Poisson operator assembly. This writes p=0 into the top ghost row so that
// exported/visualised fields are consistent.
func ApplyPressureBC(g *Grid, f *Fields) {
	for i := 0; i < g.NxG; i++ {
		f.P[g.idxCC(i, g.Ny+1)] = 0
	}
}
