# Dam Break 2D VOF Solver

## Reproducibility Map: Tables to Scripts
Every major diagnostic table or result reported can be exactly reproduced by running the corresponding scripts tracked in the tree.

| Report Result / Table | Script / Driver Path |
| :--- | :--- |
| **Grid Convergence Series (N=8, 16, 32)** | `cmd/finalpass/main.go` |
| **Time-Step Convergence (dt, dt/4, substepped)** | `cmd/finalpass/main.go` |
| **Attribution Matrix (Van Leer vs FO, Density)** | `cmd/finalpass/main.go` |
| **Cavity Benchmark Profiles (Ghia Re=100)** | `go test ./internal/solver -run TestCavityFine65 -v` |
| **Poisson Manufactured Solution** | `go test ./internal/solver -run TestPoisson -v` |
| **Loader / Validator Logic** | `go test ./internal/benchmark -v` |
| **Free-Slip vs No-Slip Sanity Checks** | `cmd/fs_check/main.go` |
| **Volume Drift / Conservation Verification** | `cmd/driftdiag/main.go` |

## Unexplained / Outstanding Anomalies
The codebase is numerically frozen. However, the following observations remain open for future mathematical alignment:

1. **Late-Stage Divergence:** Even with aligned time normalization, the $N=16$ simulation outpaces the experimental benchmark after $Z=5.0$, reaching $Z=14.0$ faster than Martin & Moyce's water columns. This implies the numerical front propagates faster / experiences less drag than the physical setup at late times.
2. **Inviscid Theoretical Limit:** The numerical terminal speed roughly steadies at $\sim 1.3 L_0 / t^*$, which is $\approx 65\%$ of the inviscid theoretical bound ($2 L_0 / t^*$). Substantial internal dissipation remains, even in the Free-Slip Van Leer configurations.
3. **Temporal Truncation Drift:** Explicit integration causes the front to advance faster at larger time steps. A fully implicit advection phase or higher-order explicit integration (e.g. RK2) may be needed to achieve absolute temporal convergence at high CFL.
