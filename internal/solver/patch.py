import re

with open('poisson.go', 'r') as f:
    content = f.read()

# Read the helper function
with open('patch_poisson.txt', 'r') as f:
    helper = f.read()

# Create the parallel applyOp
new_pcg = '''func SolvePoissonPCG(g *Grid, f *Fields, b, p []float64, tol float64, maxIter int, threads int,
	scratch [][]float64) PoissonResult {

	z, r, s, Ap := scratch[0], scratch[1], scratch[2], scratch[3]
	
	// Precompute matrix coefficients for matrix-free matvec
	diag := scratch[4]
	Ax := scratch[5] // east coefficient
	Ay := scratch[6] // north coefficient
	
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
	rhoParts := make([]float64, threads)
	bNormParts := make([]float64, threads)
	runParallel(g.Ny, threads, func(jStart, jEnd int) {
		tIdx := (jStart - 1) / ((g.Ny + threads - 1) / threads)
		myRho, myBNorm := 0.0, 0.0
		for j := jStart; j <= jEnd; j++ {
			row := j * g.NxG
			for i := 1; i <= g.Nx; i++ {
				idx := row + i
				if idx == pin {
					r[idx], z[idx] = 0, 0
					continue
				}
				r[idx] = b[idx] - Ap[idx]
				myBNorm += b[idx] * b[idx]
				z[idx] = r[idx] / diag[idx]
				myRho += r[idx] * z[idx]
			}
		}
		rhoParts[tIdx] = myRho
		bNormParts[tIdx] = myBNorm
	})
	for t := 0; t < threads; t++ {
		rho += rhoParts[t]
		bNorm += bNormParts[t]
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

		rhoNew := 0.0
		runParallel(g.Ny, threads, func(jStart, jEnd int) {
			tIdx := (jStart - 1) / ((g.Ny + threads - 1) / threads)
			myRhoNew := 0.0
			for j := jStart; j <= jEnd; j++ {
				row := j * g.NxG
				for i := 1; i <= g.Nx; i++ {
					idx := row + i
					p[idx] += alpha * s[idx]
					r[idx] -= alpha * Ap[idx]
					
					if idx == pin {
						z[idx] = 0
						continue
					}
					z[idx] = r[idx] / diag[idx]
					myRhoNew += r[idx] * z[idx]
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
'''

# Find func SolvePoissonPCG ... return PoissonResult{maxIter, relRes, false}
# and replace it.
pattern = r'func SolvePoissonPCG\(.*?\n\}\n'
# But regex with .*? over newlines is tricky in go.
# Let's split by func SolvePoissonPCG
parts = content.split('func SolvePoissonPCG(g *Grid, f *Fields, b, p []float64, tol float64, maxIter int,')
before = parts[0]
after = parts[1]
# find the end of SolvePoissonPCG which is return PoissonResult{maxIter, relRes, false}\n}
end_idx = after.find('return PoissonResult{maxIter, relRes, false}\n}')
after = after[end_idx + len('return PoissonResult{maxIter, relRes, false}\n}'):]

# Let's completely replace residualNorm too.
end_idx_2 = after.find('return sum\n}')
if end_idx_2 != -1:
	after = after[end_idx_2 + len('return sum\n}'):]

with open('poisson.go', 'w') as f:
    f.write(before + helper + '\n' + new_pcg + '\n' + after)

