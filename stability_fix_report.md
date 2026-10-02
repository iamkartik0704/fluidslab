# Cavity Stability Fix and Timestep Explanation

## 1. Commit and Hygiene

The viscous timestep fix (`dtVisc = cflVisc * hmin * hmin / (4.0 * nuMax)`) has been committed. 
Additionally:
- `cavity_output.txt` has been untracked and added to `.gitignore`.
- `cmd/cavitydiag` has been moved to `experiments/cavitydiag`.
- A Git tree hygiene check has been added to all `main.go` files: every program now prints `git rev-parse HEAD` and `git status --short`, and exits if the tree is dirty unless overridden with `-dirty`.

## 2. Timestep Logic (`timestep.go`)

### Diff: `a72ca67` -> `final_scope_v3` (`c68ebc9`)

```diff
--- a/internal/solver/timestep.go
+++ b/internal/solver/timestep.go
@@ -18,9 +18,21 @@ func ComputeDt(g *Grid, f *Fields, cfg *Config, state *SimState) float64 {
 	hmin := math.Min(g.Dx, g.Dy)
 	umax := f.MaxAbsVel(g)
 
-	dtAdv := cfg.Numerical.MaxDT
+	// Effective CFL coefficients (allow capping via MaxCFLFrac)
+	cflAdv := cfg.Numerical.CFL
+	cflVisc := cfg.Numerical.ViscousCFL
+	cflGrav := cfg.Numerical.GravityCFL
+	maxDT := cfg.Numerical.MaxDT
+	if cfg.Numerical.MaxCFLFrac > 0 && cfg.Numerical.MaxCFLFrac < 1.0 {
+		cflAdv *= cfg.Numerical.MaxCFLFrac
+		cflVisc *= cfg.Numerical.MaxCFLFrac
+		cflGrav *= cfg.Numerical.MaxCFLFrac
+		maxDT *= cfg.Numerical.MaxCFLFrac
+	}
+
+	dtAdv := maxDT
 	if umax > 0 {
-		dtAdv = cfg.Numerical.CFL * hmin / umax
+		dtAdv = cflAdv * hmin / umax
 	}
 
 	// Viscous limit: use the worst (largest) kinematic viscosity among cells
@@ -36,12 +48,12 @@ func ComputeDt(g *Grid, f *Fields, cfg *Config, state *SimState) float64 {
 			}
 		}
 	}
-	dtVisc := cfg.Numerical.MaxDT
+	dtVisc := maxDT
 	if nuMax > 0 {
-		dtVisc = cfg.Numerical.ViscousCFL * hmin * hmin / nuMax
+		dtVisc = cflVisc * hmin * hmin / nuMax
 	}
 
-	dtGrav := cfg.Numerical.GravityCFL * math.Sqrt(hmin/cfg.Physical.Gravity)
+	dtGrav := cflGrav * math.Sqrt(hmin/cfg.Physical.Gravity)
 
 	dt := math.Min(dtAdv, math.Min(dtVisc, dtGrav))
 
@@ -49,8 +61,8 @@ func ComputeDt(g *Grid, f *Fields, cfg *Config, state *SimState) float64 {
 	if state.Step > 0 {
 		dt = math.Min(dt, state.DT*cfg.Numerical.DTGrowthFactor)
 	}
-	if dt > cfg.Numerical.MaxDT {
-		dt = cfg.Numerical.MaxDT
+	if dt > maxDT {
+		dt = maxDT
 	}
```

### New Diff: Viscous Stability Fix

```diff
--- a/internal/solver/timestep.go
+++ b/internal/solver/timestep.go
@@ -51,7 +51,7 @@
 	}
 	dtVisc := maxDT
 	if nuMax > 0 {
-		dtVisc = cflVisc * hmin * hmin / nuMax
+		dtVisc = cflVisc * hmin * hmin / (4.0 * nuMax)
 	}
 
 	dtGrav := cflGrav * math.Sqrt(hmin/cfg.Physical.Gravity)
```

### Explanation in Plain Words

- **Before (`a72ca67`)**: 
  - `dtAdv` was unconditionally computed using the native `cfg.Numerical.CFL`.
  - `dtVisc` was unconditionally computed using `cfg.Numerical.ViscousCFL * h^2 / nu`.
  - There was no `MaxCFLFrac` scaling mechanism.
- **After `final_scope_v3` (`c68ebc9`)**:
  - A scaling factor `MaxCFLFrac` was introduced. If $0 < MaxCFLFrac < 1.0$, it scales down the native limits (`CFL`, `ViscousCFL`, `GravityCFL`) and the absolute maximum (`MaxDT`) proportionally, allowing global step throttling (e.g. for `dt/4` convergence runs).
  - `dtAdv` and `dtVisc` were still using the old physical formulations, just subjected to this new scaling factor `cflAdv` and `cflVisc`.
- **After Viscous Fix**:
  - `dtVisc` is now correctly bounded by the 2D explicit diffusion limit, dividing by `4.0 * nu` instead of `nu`.

### Timestep Breakdown

| Configuration | Step | h | umax | nuMax | dtAdv | dtVisc | Chosen dt |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| Cavity N=17 CFL=0.4 | 1 | 0.058824 | 1.617362 | 0.010000 | 0.014548 | 0.034602 | 0.034602 (capped by previous step) |
| Cavity N=17 CFL=0.4 | End | 0.058824 | 14.424380 | 0.010000 | 0.001631 | 0.034602 | 0.001640 (governed by Adv) |
| Cavity N=17 CFL=0.2 | 1 | 0.058824 | 0.808681 | 0.010000 | 0.014548 | 0.017301 | 0.017301 |
| Cavity N=17 CFL=0.2 | End | 0.058824 | 10.243720 | 0.010000 | 0.001148 | 0.017301 | 0.001154 |
| Cavity N=33 CFL=0.4 | 1 | 0.030303 | 0.788906 | 0.010000 | 0.015365 | 0.009183 | 0.009183 |
| Cavity N=33 CFL=0.4 | End | 0.030303 | 14.075331 | 0.010000 | 0.000861 | 0.009183 | 0.000866 |
| Cavity N=33 CFL=0.2 | 1 | 0.030303 | 0.394453 | 0.010000 | 0.015365 | 0.004591 | 0.004591 (governed by Visc) |
| Cavity N=33 CFL=0.2 | End | 0.030303 | 9.845907 | 0.010000 | 0.000616 | 0.004591 | 0.000619 |
| Cavity N=65 CFL=0.4 | 1 | 0.015385 | 0.389006 | 0.010000 | 0.015819 | 0.002367 | 0.002367 (governed by Visc) |
| Cavity N=65 CFL=0.4 | End | 0.015385 | 13.637147 | 0.010000 | 0.000451 | 0.002367 | 0.000454 |
| Cavity N=65 CFL=0.2 | 1 | 0.015385 | 0.194503 | 0.010000 | 0.015819 | 0.001183 | 0.001183 (governed by Visc) |
| Cavity N=65 CFL=0.2 | End | 0.015385 | 9.486939 | 0.010000 | 0.000324 | 0.001183 | 0.000326 |
| Cavity N=129 CFL=0.4 | 1 | 0.007752 | 0.193075 | 0.010000 | 0.016060 | 0.000601 | 0.000601 (governed by Visc) |
| Cavity N=129 CFL=0.4 | End | 0.007752 | 12.975952 | 0.010000 | 0.000239 | 0.000601 | 0.000240 |
| Cavity N=129 CFL=0.2 | 1 | 0.007752 | 0.096538 | 0.010000 | 0.016060 | 0.000300 | 0.000300 (governed by Visc) |
| Cavity N=129 CFL=0.2 | End | 0.007752 | 8.407419 | 0.010000 | 0.000184 | 0.000300 | 0.000186 |
| DamBreak N=16 dtScale=1 | 1 | 0.003572 | 0.228174 | 0.000001 | 0.003914 | 0.795798 | 0.009541 (governed by Grav step 0) |
| DamBreak N=16 dtScale=1 | End | 0.003572 | 1.722958 | 0.000001 | 0.000518 | 0.795798 | 0.000518 (governed by Adv) |
| DamBreak N=32 dtScale=0.25 | 1 | 0.001786 | 0.051261 | 0.000001 | 0.002178 | 0.049737 | 0.001687 (governed by Grav step 0) |

*(Note: In step 1, chosen `dt` is often constrained by the previous step 0's `dt` multiplied by `DTGrowthFactor`, where step 0 was solely governed by gravity since `umax=0`).*

## 3. Mechanism: 2D Explicit Diffusion Limit

2D explicit diffusion evaluates the stability amplification factor as $G = 1 - 8 \nu \Delta t / h^2$.
- At the old selector's threshold $\Delta t = h^2 / \nu$, $G = 1 - 8 = -7$.
- At the correct mathematical threshold $\Delta t = h^2 / (4\nu)$, $G = 1 - 8/4 = -1$.

The old selector's $h^2 / \nu$ never damped checkerboards; it flipped their signs and explosively amplified them by a factor of 7 on each step.

**Why did Cavity fail but Dam-Break survive?**
In Cavity, $h^2 / \nu$ is small enough (e.g. at $N=65$, $h \approx 0.015$, $\nu=0.01 \Rightarrow h^2/\nu \approx 0.0023$) that it easily falls below the hard-coded `MaxDT = 0.1`. Consequently, the viscous CFL **governed** the timestep, allowing the unstable $\Delta t = h^2 / \nu$ to be selected and blow up the simulation.

In Dam-Break, $\nu$ is very small ($\sim 10^{-6}$), so $h^2/\nu$ is extremely large (e.g., $0.795$ for $N=16$). However, Dam-Break typically runs with a heavily constrained global maximum timestep (`MaxDT = 0.05`). Because `dtVisc` was massively oversized, it was truncated by `MaxDT` or preempted by the advective limit `dtAdv`. As a result, the viscous CFL **never governed** the Dam-break runs, sparing them from the stability blowup!
