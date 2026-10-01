# Experiments and Scripts

This directory contains standalone scripts and experiments used to generate the tables and figures in our reports. 

## Benchmark and Final Pass

*   **`finalpass`**: This is the primary driver for all final convergence, matrix, and attribution tables. 
    *   *Usage:* `go run experiments/finalpass/main.go`
    *   *Outputs:* Generates `.csv` trace files in `out/finalpass/` containing `$t^*, X^*, X^*_{0.1}, X^*_{0.01}$`. Prints formatted tables to standard output.
    *   *Tables Generated:* Convergence Series, Time-step Series, Matrix Stats, Full Benchcompare against Martin & Moyce, Revised Attribution Table.

## Historical / Specialized Diagnostics

The following scripts were used for specific diagnostic deep-dives during development:

*   **`advcompare`** / **`advderiv`** / **`advectiontest`**: Analyzed specific VOF advection components (donor-acceptor vs upwind, derivative calculations).
*   **`attribution`**: Early version of the feature-attribution study (now superseded by `finalpass`).
*   **`b_study`** / **`c_study`** / **`c_study2`**: Various parameter sweep scripts for early tuning.
*   **`cavitydiag`** / **`cavityprint`**: Scripts to output and visualize the closed-cavity lid-driven benchmark. (The primary validation is now embedded as unit tests in `internal/solver/cavity_test.go`).
*   **`clipdiag`**: Tracked $\alpha$ bounding/clipping before `ClipRedistribute` was fully implemented.
*   **`converge`** / **`frontconv`**: Previous convergence sweeps for $X^*$.
*   **`driftdiag`** / **`drift_test`**: Isolated tests to output detailed drift decomposition (volume conservation).
*   **`freestall`** / **`fs_check`**: Scripts used to isolate the $t^*=4$ free-slip stall bug (caused by a pinned interior pressure cell in an 8L0 domain).
*   **`hydrotest`**: Standalone hydrostatic gate test before it was integrated into `project_test.go`.
*   **`longrun`**: Script for simulating past $t^*=10$ to assess late-stage volume drift.
*   **`poissonbench`** / **`poisson_diff`**: Specialized benchmarks for profiling the PCG Incomplete Cholesky solver and extracting iteration metrics.
*   **`snapshots`**: Dumps `.csv` state grids for external visualization.
