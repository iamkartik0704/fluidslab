# Final Scoped Pass: Convergence, Substepping, and Benchmark Alignment

This report details the final numerical diagnostics, benchcompare data, and structural changes on the repository. All data was generated on the clean tagged commit `814977b` (`final-study`).

## 1. Corrections and Reconciliations

### Temporal Truncation (dt Sensitivity)
The front travels faster at larger $dt$. This is classified as a **dt-dependent splitting/truncation error, mechanism not yet isolated**.

### Mass Clipping and Volume Drift
The clipped mass tracks exactly to the final simulation drift. In the $N=16$ matrix run, the total clipped volume was $-0.0217$ cells. The final drift of the simulation (relative to the initial 512 cell volume) was $-0.0042\%$, which corresponds to *exactly* $-0.0217$ cells. This confirms that mass clipping is the source of the drift, and the `ClipRedistribute` logic is failing to recover it. This reconciles with earlier matrices that showed $\mathcal{O}(10^{-13})$ drift when clipping was near-zero.

### Cavity Errors and Wall Sampling
The centerline sampler has been corrected (`cavity_test.go`), and $u(y=0)$ is now strictly $0.000$ exactly at the wall via hard-coded boundary logic.
The observed spatial convergence errors against Ghia ($Re = 100$, where $\nu = \mu/\rho = 0.01$) are:
* **FirstOrder:** 17 $\to$ 0.0693, 33 $\to$ 0.0501, 65 $\to$ 0.0368. Observed orders: 0.49 (17 $\to$ 33), 0.45 (33 $\to$ 65).
* **VanLeer:** 17 $\to$ 0.0649, 33 $\to$ 0.0389, 65 $\to$ 0.1071. Observed orders: 0.77 (17 $\to$ 33), -1.49 (33 $\to$ 65). The Van Leer scheme regresses at $65^2$.

### Free-Slip History
Regarding the older reported Free-Slip values ($2.6821, 2.3653, 2.4889$), all older, unreproducible Free-Slip values have been retired. The $N=16$ Free-Slip First-Order configuration now in use strictly enforces zero wall-normal velocities and sets tangential velocities equal to the interior cell, and yields $2.604$.

## 2. Matrix Execution: Resolution and Time-Step

Configurations run on a 15$L_0 \times 4L_0$ domain, Free-Slip walls, Van Leer advection, $H_0/L_0=2$, $a=2.25$ inches mapping to $L_0$.

| Config | Wall Time | $T(Z=1.44)$ raw | $T_{align}(Z=3)$ | $Z=5$ | $Z=7$ | $Z=10$ | $Z=14$ |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **$N=8$ Native** | 3.1s | 0.981 | 2.605 | 4.070 | 5.436 | 7.505 | 10.572 |
| **$N=16$ Native** | 34.6s | 0.917 | 2.604 | 4.012 | 5.301 | 7.246 | 10.086 |
| **$N=32$ Native** | 13m 47s | 0.881 | 2.606 | 3.960 | 5.213 | 7.263 | 10.046 |
| **$N=8 \ dt/4$** | 10.8s | 0.990 | 2.618 | 4.117 | 5.541 | 7.695 | 10.853 |
| **$N=12 \ dt/4$** | 32.2s | 0.957 | 2.608 | 4.101 | 5.494 | 7.587 | 10.621 |
| **$N=16 \ dt/4$** | 2m 05s | 0.929 | 2.618 | 4.094 | 5.469 | 7.532 | 10.502 |
| **$N=24 \ dt/4$** | 6m 49s | 0.903 | 2.626 | 4.059 | 5.388 | 7.439 | 10.261 |
| **$N=32 \ dt/4$** | 20m 12s | 0.893 | 2.622 | 4.027 | 5.319 | 7.357 | 10.091 |

**Why $Z=1.44$ Anchor Differs:** The raw time $T_{sim}(Z=1.44)$ differs between grids. The successive differences for the native runs are 0.064 ($N=8 \to 16$) and 0.036 ($N=16 \to 32$). The cause for this difference is untested.

**Space Convergence:** At $dt/4$, the successive differences grow with $N$. For example, at $Z=10$: -0.163 ($N=8 \to 16$), -0.175 ($N=16 \to 32$). At $Z=14$: -0.351 ($N=8 \to 16$), -0.411 ($N=16 \to 32$). The native rows only appeared to agree because the time-step and grid errors cancel out.

## 3. Sub-stepping Isolation (Advection vs Momentum)
The $N=16$ (Van Leer, Free Slip) configuration was executed with the overall $\Delta t$ held at its native limit, but sub-stepped independently:

| Configuration | $T_{align}(Z=7)$ | $T_{align}(Z=10)$ | $\Delta T(Z=10)$ vs Native |
| :--- | :--- | :--- | :--- |
| **Native $dt$** | 5.301 | 7.246 | Reference |
| **Global $dt/4$** | 5.469 | 7.532 | $+0.286$ |
| **VOF Advection Substepped 4x** | 5.364 | 7.409 | $+0.163$ |
| **Momentum/Projection Substepped 4x** | 5.410 | 7.408 | $+0.162$ |

At $Z=7$, the isolated split is $+0.063$ (VOF) and $+0.109$ (momentum). At $Z=10$ it is $+0.163$ (VOF) and $+0.162$ (momentum). The sum of the isolated effects ($0.325$) exceeds the combined global effect ($0.286$). 

## 4. Cost Analysis and CFL Cap
A unified Substepping approach was integrated directly into `Config.Numerical` (via `MaxCFLFrac` to cleanly restrict $dt$, alongside `SubstepVOF`/`SubstepMom`). 

**Wall-Clock Cost Analysis ($N=16$ to $Z=14$):**
- **Native $dt$**: 34.6 seconds
- **Global $dt/4$**: 2 minutes 05.1 seconds 
- **Substepped VOF (4x)**: 29.7 seconds 
- **Substepped Momentum (4x)**: 2 minutes 05.6 seconds (The Poisson solver loop dominates computation time).

## 5. Benchcompare: Simulation vs Martin & Moyce

### Evaluation against $a=2.25$ inches (mean)

| Configuration | $Z \in [1.44, 7]$ Raw | $Z \in [1.44, 7]$ Aligned | $Z \in [1.44, 14]$ Aligned |
| :--- | :--- | :--- | :--- |
| **Van Leer FS $N=32 \ dt/4$** | RMS 11.2%, Max 25.1% | RMS 3.9%, Max 5.3% | RMS 5.1%, Max 8.4% |
| **Van Leer FS $N=16 \ dt/4$** | RMS 9.0%, Max 21.9% | RMS 4.4%, Max 7.2% | RMS 7.0%, Max 12.8% |

**Summary:** The solver lags the experiment at late $Z$ (larger $T$ = slower). The physical water is travelling much faster than the simulation.

## 6. Loader and Validator Logic
The internal testing suite in `internal/benchmark/martin_test.go` (`TestInterpTSim`, `TestAlignTime`, `TestTScaleMapping`, and `TestValidateMartinData`) was run and passed. 

## 7. Unexplained Observations
1. **Late-Stage Divergence:** The simulation lags the experimental benchmark after $Z=5.0$, reaching $Z=14.0$ substantially slower.
2. **Inviscid Theoretical Limit:** The numerical terminal speed for $N=32 \ dt/4$ between $Z=10$ and $Z=14$ is $1.46 L_0 / t^*$. Using the $t^* = t \sqrt{2g / a}$ conversion, the inviscid theoretical limit is $2.0 L_0 / t^*$. The simulation thus travels at $73\%$ of the inviscid bound.
3. **Temporal Truncation Drift:** Explicit integration causes the front to advance faster at larger time steps.
