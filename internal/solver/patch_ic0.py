import re

with open('poisson.go', 'r') as f:
    content = f.read()

# We find the string:
# 	// Precompute matrix coefficients for matrix-free matvec
# 	diag := scratch[4]
# 	Ax := scratch[5] // east coefficient
# 	Ay := scratch[6] // north coefficient
target1 = '''	// Precompute matrix coefficients for matrix-free matvec
	diag := scratch[4]
	Ax := scratch[5] // east coefficient
	Ay := scratch[6] // north coefficient'''

replacement1 = '''	// Precompute matrix coefficients for matrix-free matvec
	diag := scratch[4]
	Ax := scratch[5] // east coefficient
	Ay := scratch[6] // north coefficient
	invE := scratch[7] // IC(0) inverse diagonal'''

content = content.replace(target1, replacement1)

# We find the string:
# 				} else {
# 				    Ay[idx] = 0
# 				}
# 			}
# 		}
# 	})
# And add the IC(0) computation immediately after.
target2 = '''				} else {
				    Ay[idx] = 0
				}
			}
		}
	})'''

replacement2 = target2 + '''
	
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
	}'''
content = content.replace(target2, replacement2)

# Replace the sequential and parallel preconditioner application:
# Initially it's in two places. 
# 1. First time:
target3 = '''	// Parallel initial residual
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
	}'''

replacement3 = '''	// Parallel initial residual
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
	}'''

content = content.replace(target3, replacement3)

# Now replace the loop inside the iterations
target4 = '''		rhoNew := 0.0
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
		}'''

replacement4 = '''		runParallel(g.Ny, threads, func(jStart, jEnd int) {
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
		}'''

content = content.replace(target4, replacement4)

with open('poisson.go', 'w') as f:
    f.write(content)

