package solver

import (
	"math"
	"sync"
)

// Variable-density pressure Poisson solver.
//
// Discrete operator (cell-centred p, matrix-free matvec):
//
//	(A p)_ij = sum over 4 faces of coef_f * (p_ij - p_nb) / d^2
//	coef_f = 1/rho_f   (RhoU/RhoV from props.go)
//
// Boundary treatment (rationale in bc.go):
//   - solid walls (left/right/floor): ZERO face coefficient -> homogeneous
//     Neumann, no flux through the wall;
//   - open top: Dirichlet p=0 AT the ghost cell centre (row j==Ny+1),
//     so the north coefficient is the ordinary 1/rho/dy^2 — the same stencil
//     as the interior, which keeps the projection correction exactly
//     consistent with the operator (and the test at the end of project.go).
// With at least one Dirichlet cell the operator is SPD and no null-space
// pinning is needed. (A fully enclosed case would pin one cell.)
type PoissonResult struct {
	Iterations int
	Residual   float64 // final relative residual ||r||/||b||
	Converged  bool
}

// faceCoef returns the west/east/south/north inverse-density coefficients for
// interior cell (i,j), with solid faces zeroed and the Dirichlet top doubled.
type faceCoefs struct {
	w, e, s, n float64
	nElim      bool // true: north Dirichlet ghost eliminated into the diagonal
}

func poissonCoefs(g *Grid, f *Fields, i, j int) faceCoefs {
	var c faceCoefs
	// west: solid at i==1 (face is the left wall) -> zero coefficient
	if i > 1 {
		c.w = 1.0 / f.RhoU[g.idxU(i-1, j)]
	}
	// east: solid at i==Nx (face is the right wall)
	if i < g.Nx {
		c.e = 1.0 / f.RhoU[g.idxU(i, j)]
	}
	// south: solid at j==1 (floor)
	if j > 1 {
		c.s = 1.0 / f.RhoV[g.idxV(i, j-1)]
	}
	// north face:
	//   open top (no pin): Dirichlet p=0 AT the top face y=H -> half-cell
	//   distance gives the doubled coefficient 2/rho/dy^2. The projection
	//   correction uses the matching half-cell gradient at face Ny, so the
	//   solve and the correction stay exactly consistent (and 2nd-order).
	//   closed top (pin active): Neumann at the lid -> zero coefficient.
	//   The pinned cell itself is skipped entirely (p held at 0).
	if g.hasPin {
		if i == g.PinnedCell[0] && j == g.PinnedCell[1] {
			return faceCoefs{} // A p = 0; enforced exactly, row never enters CG
		}
		// Solid lid: only the TOP-ROW north faces (the lid itself) are zero.
		// Interior north faces keep their ordinary coefficient.
		if j == g.Ny {
			c.n = 0
		} else {
			c.n = 1.0 / f.RhoV[g.idxV(i, j)]
		}
	} else if j == g.Ny {
		// Dirichlet p=0 at the top face: eliminate the ghost row with the
		// anti-mirror p[Ny+1] = -p[Ny] (exact for p_face = 0). The ghost
		// contribution -n0*p[Ny+1] becomes +n0*p[Ny], i.e. the diagonal gains
		// n0 and the ghost COLUMN disappears -> A stays symmetric, and the
		// matching correction gradient is -2*p[Ny]/dy at the top face.
		c.n = 2.0 / f.RhoV[g.idxV(i, j)]
		c.nElim = true
	} else {
		c.n = 1.0 / f.RhoV[g.idxV(i, j)]
	}
	return c
}

// ApplyOperator computes A p -> out using the variable-density operator.
func ApplyOperator(g *Grid, f *Fields, p, out []float64) {
	invDx2 := g.InvDx * g.InvDx
	invDy2 := g.InvDy * g.InvDy
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			c := poissonCoefs(g, f, i, j)
			sum := (c.w+c.e)*invDx2 + (c.s+c.n)*invDy2 // diagonal
			off := 0.0
			if c.w > 0 {
				off += c.w * invDx2 * p[g.idxCC(i-1, j)]
			}
			if c.e > 0 {
				off += c.e * invDx2 * p[g.idxCC(i+1, j)]
			}
			if c.s > 0 {
				off += c.s * invDy2 * p[g.idxCC(i, j-1)]
			}
			if c.n > 0 && !c.nElim {
				off += c.n * invDy2 * p[g.idxCC(i, j+1)]
			}
			out[idx] = sum*p[idx] - off
		}
	}
}

// SolvePoissonPCG solves A p = b with preconditioned conjugate gradients.
// scratch must provide 4 arrays of TotalCC length (z, r, s, Ap)
// plus 3 more for matrix coefficients (diag, Ax, Ay) — we will just allocate them here for now
// ForceSerialPoisson, when true, disables goroutine parallelism in the PCG
// solver. Benchmarks on a Core Ultra 9 288V show serial is 2x faster at 64×64
// (goroutine spawn overhead dominates at small grid sizes).
var ForceSerialPoisson bool

func runParallel(ny int, threads int, worker func(jStart, jEnd int)) {
	if threads <= 1 || ForceSerialPoisson {
		worker(1, ny)

		return
	}
	var wg sync.WaitGroup
	chunk := (ny + threads - 1) / threads
	for t := 0; t < threads; t++ {
		jStart := 1 + t*chunk
		jEnd := jStart + chunk - 1
		if jEnd > ny {
			jEnd = ny
		}
		if jStart <= jEnd {
			wg.Add(1)
			go func(start, end int) {
				worker(start, end)
				wg.Done()
			}(jStart, jEnd)
		}
	}
	wg.Wait()
}

func SolvePoissonPCG(g *Grid, f *Fields, b, p []float64, tol float64, maxIter int, threads int,
	scratch [][]float64) PoissonResult {

	z, r, s, Ap := scratch[0], scratch[1], scratch[2], scratch[3]

	// Precompute matrix coefficients for matrix-free matvec
	diag := scratch[4]
	Ax := scratch[5]   // east coefficient
	Ay := scratch[6]   // north coefficient
	invE := scratch[7] // IC(0) inverse diagonal

	runParallel(g.Ny, threads, func(jStart, jEnd int) {
		for j := jStart; j <= jEnd; j++ {
			row := j * g.NxG
			for i := 1; i <= g.Nx; i++ {
				idx := row + i
				c := poissonCoefs(g, f, i, j)
				diag[idx] = (c.w+c.e)*g.InvDx2 + (c.s+c.n)*g.InvDy2
				Ax[idx] = c.e * g.InvDx2
				if !c.nElim {
					Ay[idx] = c.n * g.InvDy2
				} else {
					Ay[idx] = 0
				}
			}
		}
	})

	// Precompute IC(0) invE sequentially (cannot be parallelized easily)
	for j := 1; j <= g.Ny; j++ {
		row := j * g.NxG
		for i := 1; i <= g.Nx; i++ {
			idx := row + i
			val := diag[idx]
			if i > 1 {
				val -= Ax[idx-1] * Ax[idx-1] * invE[idx-1]
			}
			if j > 1 {
				val -= Ay[idx-g.NxG] * Ay[idx-g.NxG] * invE[idx-g.NxG]
			}
			invE[idx] = 1.0 / val
		}
	}

	applyOp := func(in, out []float64) {
		runParallel(g.Ny, threads, func(jStart, jEnd int) {
			for j := jStart; j <= jEnd; j++ {
				row := j * g.NxG
				for i := 1; i <= g.Nx; i++ {
					idx := row + i
					sum := diag[idx] * in[idx]
					if i > 1 {
						sum -= Ax[idx-1] * in[idx-1]
					}
					if i < g.Nx {
						sum -= Ax[idx] * in[idx+1]
					}
					if j > 1 {
						sum -= Ay[idx-g.NxG] * in[idx-g.NxG]
					}
					if j < g.Ny {
						sum -= Ay[idx] * in[idx+g.NxG]
					}
					out[idx] = sum
				}
			}
		})
	}

	applyOp(p, Ap)

	// r = b - A p ; also Jacobi-diagonal preconditioner M = diag(A).
	var rho, bNorm float64
	pin := g.PinnedIdx()

	// Parallel initial residual
	bNormParts := make([]float64, threads)
	runParallel(g.Ny, threads, func(jStart, jEnd int) {
		tIdx := (jStart - 1) / ((g.Ny + threads - 1) / threads)
		myBNorm := 0.0
		for j := jStart; j <= jEnd; j++ {
			row := j * g.NxG
			for i := 1; i <= g.Nx; i++ {
				idx := row + i
				if idx == pin {
					r[idx] = 0
					continue
				}
				r[idx] = b[idx] - Ap[idx]
				myBNorm += b[idx] * b[idx]
			}
		}
		bNormParts[tIdx] = myBNorm
	})
	for t := 0; t < threads; t++ {
		bNorm += bNormParts[t]
	}

	// Apply IC(0) preconditioner sequentially
	for j := 1; j <= g.Ny; j++ {
		row := j * g.NxG
		for i := 1; i <= g.Nx; i++ {
			idx := row + i
			if idx == pin {
				z[idx] = 0
				continue
			}
			val := r[idx]
			if i > 1 {
				val += Ax[idx-1] * z[idx-1]
			}
			if j > 1 {
				val += Ay[idx-g.NxG] * z[idx-g.NxG]
			}
			z[idx] = val * invE[idx]
		}
	}
	for j := g.Ny; j >= 1; j-- {
		row := j * g.NxG
		for i := g.Nx; i >= 1; i-- {
			idx := row + i
			if idx == pin {
				continue
			}
			val := 0.0
			if i < g.Nx {
				val += Ax[idx] * z[idx+1]
			}
			if j < g.Ny {
				val += Ay[idx] * z[idx+g.NxG]
			}
			z[idx] += val * invE[idx]
		}
	}

	rhoParts := make([]float64, threads)
	runParallel(g.Ny, threads, func(jStart, jEnd int) {
		tIdx := (jStart - 1) / ((g.Ny + threads - 1) / threads)
		myRho := 0.0
		for j := jStart; j <= jEnd; j++ {
			row := j * g.NxG
			for i := 1; i <= g.Nx; i++ {
				idx := row + i
				if idx != pin {
					myRho += r[idx] * z[idx]
				}
			}
		}
		rhoParts[tIdx] = myRho
	})
	for t := 0; t < threads; t++ {
		rho += rhoParts[t]
	}

	if bNorm == 0 {
		return PoissonResult{0, 0, true}
	}
	if pin >= 0 {
		p[pin] = 0 // enclosed mode: reference pressure held at zero
	}
	// Initial search direction s = M^-1 r.
	copy(s, z)

	// We calculate residual norm using parallel bNorm and r.
	relRes := math.Sqrt(residualNormThreads(g, r, threads) / bNorm)
	if relRes <= tol {
		return PoissonResult{0, relRes, true}
	}

	for it := 1; it <= maxIter; it++ {
		applyOp(s, Ap)
		if pin >= 0 {
			Ap[pin] = 0
		}

		alpha := 0.0
		alphaParts := make([]float64, threads)
		runParallel(g.Ny, threads, func(jStart, jEnd int) {
			tIdx := (jStart - 1) / ((g.Ny + threads - 1) / threads)
			myAlpha := 0.0
			for j := jStart; j <= jEnd; j++ {
				row := j * g.NxG
				for i := 1; i <= g.Nx; i++ {
					idx := row + i
					myAlpha += s[idx] * Ap[idx]
				}
			}
			alphaParts[tIdx] = myAlpha
		})
		for t := 0; t < threads; t++ {
			alpha += alphaParts[t]
		}
		alpha = rho / alpha

		runParallel(g.Ny, threads, func(jStart, jEnd int) {
			for j := jStart; j <= jEnd; j++ {
				row := j * g.NxG
				for i := 1; i <= g.Nx; i++ {
					idx := row + i
					p[idx] += alpha * s[idx]
					r[idx] -= alpha * Ap[idx]
				}
			}
		})

		// IC(0) preconditioner sequentially
		for j := 1; j <= g.Ny; j++ {
			row := j * g.NxG
			for i := 1; i <= g.Nx; i++ {
				idx := row + i
				if idx == pin {
					z[idx] = 0
					continue
				}
				val := r[idx]
				if i > 1 {
					val += Ax[idx-1] * z[idx-1]
				}
				if j > 1 {
					val += Ay[idx-g.NxG] * z[idx-g.NxG]
				}
				z[idx] = val * invE[idx]
			}
		}
		for j := g.Ny; j >= 1; j-- {
			row := j * g.NxG
			for i := g.Nx; i >= 1; i-- {
				idx := row + i
				if idx == pin {
					continue
				}
				val := 0.0
				if i < g.Nx {
					val += Ax[idx] * z[idx+1]
				}
				if j < g.Ny {
					val += Ay[idx] * z[idx+g.NxG]
				}
				z[idx] += val * invE[idx]
			}
		}

		rhoNew := 0.0
		runParallel(g.Ny, threads, func(jStart, jEnd int) {
			tIdx := (jStart - 1) / ((g.Ny + threads - 1) / threads)
			myRhoNew := 0.0
			for j := jStart; j <= jEnd; j++ {
				row := j * g.NxG
				for i := 1; i <= g.Nx; i++ {
					idx := row + i
					if idx != pin {
						myRhoNew += r[idx] * z[idx]
					}
				}
			}
			rhoParts[tIdx] = myRhoNew
		})
		for t := 0; t < threads; t++ {
			rhoNew += rhoParts[t]
		}

		relRes = math.Sqrt(residualNormThreads(g, r, threads) / bNorm)
		if relRes <= tol {
			return PoissonResult{it, relRes, true}
		}

		beta := rhoNew / rho
		rho = rhoNew

		runParallel(g.Ny, threads, func(jStart, jEnd int) {
			for j := jStart; j <= jEnd; j++ {
				row := j * g.NxG
				for i := 1; i <= g.Nx; i++ {
					idx := row + i
					s[idx] = z[idx] + beta*s[idx]
				}
			}
		})
	}
	return PoissonResult{maxIter, relRes, false}
}

func residualNormThreads(g *Grid, r []float64, threads int) float64 {
	sum := 0.0
	sumParts := make([]float64, threads)
	runParallel(g.Ny, threads, func(jStart, jEnd int) {
		tIdx := (jStart - 1) / ((g.Ny + threads - 1) / threads)
		mySum := 0.0
		for j := jStart; j <= jEnd; j++ {
			row := j * g.NxG
			for i := 1; i <= g.Nx; i++ {
				v := r[row+i]
				mySum += v * v
			}
		}
		sumParts[tIdx] = mySum
	})
	for t := 0; t < threads; t++ {
		sum += sumParts[t]
	}
	return sum
}

// SolvePoissonSOR solves A p = b by successive over-relaxation. Reference
// solver for the constant-density tests only.
func SolvePoissonSOR(g *Grid, f *Fields, b, p []float64, tol float64, maxIter int, omega float64) PoissonResult {
	bNorm := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			bNorm += b[idx] * b[idx]
		}
	}
	if bNorm == 0 {
		return PoissonResult{0, 0, true}
	}

	var relRes float64
	res := make([]float64, len(b))
	for it := 1; it <= maxIter; it++ {
		// Gauss-Seidel/SOR sweep with the same coefficients as the matvec.
		for j := 1; j <= g.Ny; j++ {
			for i := 1; i <= g.Nx; i++ {
				idx := g.idxCC(i, j)
				c := poissonCoefs(g, f, i, j)
				diag := (c.w+c.e)*g.InvDx2 + (c.s+c.n)*g.InvDy2
				off := 0.0
				if c.w > 0 {
					off += c.w * g.InvDx2 * p[g.idxCC(i-1, j)]
				}
				if c.e > 0 {
					off += c.e * g.InvDx2 * p[g.idxCC(i+1, j)]
				}
				if c.s > 0 {
					off += c.s * g.InvDy2 * p[g.idxCC(i, j-1)]
				}
				if c.n > 0 && !c.nElim {
					off += c.n * g.InvDy2 * p[g.idxCC(i, j+1)]
				}
				// A p = diag*p - off = b  =>  p = (b + off)/diag.
				p[idx] = (1-omega)*p[idx] + (omega/diag)*(b[idx]+off)
			}
		}
		// TRUE residual check (update-based stopping is unreliable).
		ApplyOperator(g, f, p, res)
		for idx := range res {
			res[idx] = b[idx] - res[idx]
		}
		relRes = math.Sqrt(residualNormThreads(g, res, 1) / bNorm)
		if relRes < tol {
			return PoissonResult{it, relRes, true}
		}
	}
	return PoissonResult{maxIter, relRes, false}
}
