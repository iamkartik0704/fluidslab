# Priority D Report: Performance Pass + First Dam-Break Look

## 1. Poisson Solver Configuration
- **Algorithm:** Preconditioned Conjugate Gradient (PCG).
- **Preconditioner:** Incomplete Cholesky (IC(0)).
- **Convergence Tolerance:** 1e-6 (Relative residual $||r||/||b||$).
- **Initial Guess:** Warm start. The `f.P` array is preserved between time steps and passed directly into `SolvePoissonPCG` without being zeroed, meaning the previous step's pressure correction serves as the initial guess.

## 2. Profiling (64x96 Dam-Break, 200 Steps)

The dam-break ran at 64x96 for 200 steps with CPU profiling enabled. Out of ~3.13s of cumulative time spent in `Simulation.Step`, the time breakdown is extremely lopsided:

**Split between phases:**
- **Projection / Poisson:** 87.5% (2.74s cum)
- **Predictor:** 4.8% (0.15s cum)
- **VOF (AdvectAlpha):** 3.2% (0.10s cum)
- **Diagnostics:** 2.5% (0.08s cum)

**Top 10 Functions by Cumulative Time:**
1. `Simulation.Step`: 3.13s
2. `Project`: 2.74s
3. `SolvePoissonPCG`: 2.69s (plus parallel worker goroutines)
4. `SolvePoissonPCG.func2.func10` (parallel worker): 0.62s
5. `runtime.systemstack`: 0.44s
6. `SolvePoissonPCG.func6` (parallel worker): 0.37s
7. `runtime.goexit0`: 0.37s
8. `runtime.findRunnable`: 0.34s
9. `runParallel`: 0.24s
10. `runtime.newproc.func1`: 0.21s

**Observation:** The Poisson solver (and the goroutine scheduling overhead for its parallel loops) utterly dominates the execution time. The VOF algorithm is surprisingly fast (only ~3% of runtime). All performance optimizations must target `SolvePoissonPCG`, specifically reducing the threading overhead or accelerating the IC(0) sequence.
