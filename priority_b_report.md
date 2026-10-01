# Priority B Report: Clean Full Rerun Post-Fix

All results were obtained on a $8 L_0 \times 4 L_0$ physical domain, $H_0 = 2L_0$, density ratio 1000, no-slip boundary conditions (unless noted otherwise).

## 1. Clean Drift Diagnosis (16 cells/L0, t*=4.5)
After fixing the hydrostatic property mapping, the true drift characteristics of the advection scheme were cleanly measured. We accounted for volume loss at each phase.

| Configuration | Tol | Final Drift | Clip | Sweep X | Sweep Y | Outflow |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **SplitDivFix ON** | 1e-6 | -0.12% | -0.12% | 141.44% | -141.44% | ~0 |
| **SplitDivFix ON** | 1e-9 | -0.12% | -0.12% | 141.44% | -141.44% | ~0 |
| **SplitDivFix OFF** | 1e-6 | -4.88% | ~0 | 135.46% | -140.34% | ~0 |
| **SplitDivFix OFF** | 1e-9 | -4.88% | ~0 | 135.35% | -140.23% | ~0 |

**Conclusion:** The dominant cause of volume loss is the **advection splitting error (dilatation)**. With `SplitDivFix` off, the volume drift reaches -4.88%, caused entirely by the non-conservation between the X and Y directional sweeps. When `SplitDivFix` is turned on, the splitting error is completely cancelled out, reducing the net drift to a negligible -0.12% which stems entirely from explicit `ClipAlpha` bounding. The Poisson tolerance (1e-6 vs 1e-9) has virtually zero impact on the drift.

## 2. Front Tracking ($X^*$) and Convergence

We tracked the front $X^*$ using two definitions: 
- $X^*(0.5)$: the lowest row cell where $\alpha = 0.5$
- $X^*(99\%)$: the cumulative x-position where 99% of the water lies.

| Res | t*=1 | t*=2 | t*=3 | t*=4 | t*=5 | t*=6 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **8 cells/L0** | $X^*(0.5)$: 1.764<br>$X^*(99\%)$: 1.688 | $X^*(0.5)$: 3.125<br>$X^*(99\%)$: 3.063 | $X^*(0.5)$: 4.605<br>$X^*(99\%)$: 4.563 | $X^*(0.5)$: 5.961<br>$X^*(99\%)$: 5.938 | $X^*(0.5)$: 7.254<br>$X^*(99\%)$: 7.188 | $X^*(0.5)$: 8.000<br>$X^*(99\%)$: 7.938 |
| **16 cells/L0** | $X^*(0.5)$: 1.778<br>$X^*(99\%)$: 1.656 | $X^*(0.5)$: 3.211<br>$X^*(99\%)$: 3.156 | $X^*(0.5)$: 4.816<br>$X^*(99\%)$: 4.781 | $X^*(0.5)$: 6.418<br>$X^*(99\%)$: 6.344 | $X^*(0.5)$: 7.894<br>$X^*(99\%)$: 7.844 | $X^*(0.5)$: 8.000<br>$X^*(99\%)$: 7.969 |
| **32 cells/L0** | $X^*(0.5)$: 1.825<br>$X^*(99\%)$: 1.703 | $X^*(0.5)$: 3.341<br>$X^*(99\%)$: 3.266 | $X^*(0.5)$: 5.124<br>$X^*(99\%)$: 5.047 | $X^*(0.5)$: 6.873<br>$X^*(99\%)$: 6.797 | $X^*(0.5)$: 8.000<br>$X^*(99\%)$: 7.953 | $X^*(0.5)$: 8.000<br>$X^*(99\%)$: 7.984 |

**Convergence observation:** The difference between 8 and 16 cells is large; the difference between 16 and 32 is roughly similar or slightly larger, suggesting first-order convergence has not fully set in at this coarse scale, or the physical features (thin leading edge) are not yet fully resolved. Both definitions move in tandem.

## 3. Mean / Max Poisson Iterations (with True Properties)
Previously, the bug caused erratic iteration counts. With correct physical scaling:
- **8 cells/L0:** 49 mean / 77 max
- **16 cells/L0:** 98 mean / 151 max
- **32 cells/L0:** 185 mean / 291 max

The true iteration count roughly scales by $\sim 2\times$ per grid refinement, which is perfectly characteristic of an un-preconditioned (or diagonal-preconditioned) Conjugate Gradient solver. 

## 4. Free-Slip vs No-Slip (16 & 32 cells/L0)

| Res | t*=4 (No-Slip) | t*=4 (Free-Slip) |
| :--- | :--- | :--- |
| **16 cells/L0** | 6.418 | 7.186 |
| **32 cells/L0** | 6.873 | 7.423 |

Free-slip substantially accelerates the front propagation since the wall drag is eliminated, speeding up the water surge over the floor.

## 5. Alpha Bounds and Integrity

- **Max volume drift (all runs):** 0.14%
- **Alpha range (after clipping):** $-4.8\times 10^{-196}$ (essentially zero) to $1.00$
- **CFL warnings:** 0 across all runs.

## 6. Van Leer vs First-Order Upwind (16 cells/L0)
- **First-Order (16 cells/L0):** $X^*(0.5) = 6.418$ at $t^*=4$
- **Van Leer (16 cells/L0):** $X^*(0.5) = 6.795$ at $t^*=4$
- **First-Order (32 cells/L0):** $X^*(0.5) = 6.873$ at $t^*=4$

The van Leer scheme at 16 cells/L0 behaves very similarly to First-Order at 32 cells/L0. It preserves the sharper interface momentum significantly better, confirming the prior hypothesis that the upwind artificial viscosity artificially slows down the front.
