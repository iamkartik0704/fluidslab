# Dam Break 2D VOF Solver

## Reproducibility Map: Tables to Scripts
Every major diagnostic table or result reported can be exactly reproduced by running the corresponding scripts tracked in the tree.

| Report Result / Table | Script / Driver Path |
| :--- | :--- |
| **Dam Break Reference JSON Generator** | `go run cmd/finalbench/main.go` |
| **Grid & Time-Step Convergence Series** | `cmd/finalpass/main.go` |
| **Attribution Matrix (Van Leer vs FO, Density)** | `cmd/finalpass/main.go` |
| **Cavity Benchmark Diagnostics & Profiles** | `go run cmd/cavityprint/main.go` |
| **Poisson Manufactured Solution** | `go test ./internal/solver -run TestPoisson -v` |
| **Loader / Validator Logic** | `go test ./internal/benchmark -v` |
| **Free-Slip vs No-Slip Sanity Checks** | `cmd/fs_check/main.go` |
| **Volume Drift / Conservation Verification** | `cmd/driftdiag/main.go` |

## Unexplained / Outstanding Anomalies
The codebase is numerically frozen. However, the following observations remain open for future mathematical alignment:

1. **Late-Stage Divergence:** Even with aligned time normalization, the $N=32$ simulation lags the experimental benchmark after $Z=5.0$, reaching $Z=14.0$ substantially slower than Martin & Moyce's water columns.
2. **Inviscid Theoretical Limit:** The numerical terminal speed for $N=32 \ dt/4$ between $Z=10$ and $Z=14$ is $1.46 L_0 / t^*$. Using the $t^* = t \sqrt{2g / L_0}$ conversion, the inviscid theoretical limit is $2.0 L_0 / t^*$. The simulation thus travels at $73\%$ of the inviscid bound.
3. **Temporal Truncation Drift:** Explicit integration causes the front to advance faster at larger time steps. This is a dt-dependent splitting/truncation error, mechanism not yet isolated.
