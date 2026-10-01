# Front Position Regression Hunt

## 1. Invariant Check and X* < 1 Explanation
An invariant check `if s.State.FrontXStar < 0.99` has been added to `sim.go` inside `UpdateDiagnostics`, along with a unit test `TestFrontInvariant` in `front_test.go` that runs the solver and strictly verifies this condition. 

**Why was X* < 1 reported?**
The script `cmd/frontconv/main.go` defined the front crossing as:
`xStarCross = (xFront - L0) / L0`
This offset subtracted `L0` from the absolute front position, shifting the coordinate system such that the column's initial face (`x = L0`) was mapped to `X* = 0` instead of `X* = 1`.

## 2. Regression Experiments

We reproduced the 8L0x4L0 domain at 16 cells/L0, no-slip, first-order, SplitDivFix on, tol 1e-6, parallel Poisson, ClipRedistribute off. The baseline and toggled factors yield the following `X*` (using `frontX / L0`):

| Configuration | t*=1 | t*=2 | t*=3 | t*=4 |
| :--- | :--- | :--- | :--- | :--- |
| **Baseline** | 1.4458 | 2.3225 | 3.3918 | 4.5111 |
| (a) ClipRedistribute ON | 1.4458 | 2.3227 | 3.3920 | 4.5138 |
| (b) ForceSerialPoisson ON | 1.4458 | 2.3225 | 3.3918 | 4.5111 |
| (c) Domain 10L0 | 1.4458 | 2.3225 | 3.3920 | 4.5133 |
| (d) Closed Top | 1.4458 | 2.3207 | 3.3889 | 4.5089 |
| FreeSlip walls | 1.4889 | 2.6821 | 3.9341 | (stall) |
| Old Hirt-Nichols CF bug | 1.4574 | 2.3541 | 3.4240 | 4.5552 |
| Density Interpolation Bug | 1.4295 | 2.3016 | 3.3678 | 4.4846 |

**None of these factors moved the front to 1.778!** 
The physical configuration is fundamentally stable and consistent.

## 3. The True Cause: Time Scale and Coordinate Definitions

The apparent ~2x change in the front position (`1.778, 3.211, 4.816, 6.418` vs `0.446, 1.323, 2.392, 3.515`) is a purely diagnostic reporting artifact caused by two overlapping definition changes:

1. **Time Scale Definition:** The older report used `t* = t * sqrt(g / L0)` (which corresponds to `TimeScaleSqrtGOverL0` in config), whereas the newer report used `t* = t * sqrt(2g / L0)` (`TimeScaleSqrt2gOverL0`). At `t* = 1` in the old scale, the physical time `t` is $\sqrt{2} \approx 1.414$ times larger. Interpolating our current baseline data to `t* = 1.414` gives `X* ≈ 1.80`, perfectly aligning with the old report's `1.778`. Likewise, `t* = 2` under the old scale corresponds to `t* = 2.828` under the new scale, which yields `X* ≈ 3.208` (vs old `3.211`).
2. **Coordinate Offset:** As identified above, the newer report used `X* = (frontX - L0) / L0` while the older report used `X* = frontX / L0`. Adding `1.0` to the new values (`0.446 -> 1.446`) perfectly aligns with our baseline above.

## 4. ApplyVelocityBC Lid Fix Diff

The lid fix only modified the top row of the domain. It has been isolated with the `OpenTop` flag so that it does not affect the dam break open top boundary:
```diff
-		for i := 0; i < g.NxG; i++ {
-			f.V[g.TopV(i)] = 0
-		}
-		for i := 1; i < g.Nx; i++ {
-			f.U[g.idxU(i, g.Ny+1)] = 2.0*cfg.Numerical.LidVelocity + tangSign*f.U[g.idxU(i, g.Ny)]
-		}
+		if !cfg.Numerical.OpenTop {
+			// Apply lid logic
+			for i := 0; i < g.NxG; i++ {
+				f.V[g.TopV(i)] = 0
+			}
+			for i := 1; i < g.Nx; i++ {
+				f.U[g.idxU(i, g.Ny+1)] = 2.0*cfg.Numerical.LidVelocity + tangSign*f.U[g.idxU(i, g.Ny)]
+			}
+		}
```
Restoring the previous unconditionally closed top (`Closed Top` configuration in the table above) has less than a 0.2% effect on the front position, confirming the lid fix did not cause the regression.
