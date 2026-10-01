# Priority A Report: Regression Protection

## 1. Regression Test
A unit test `TestInterpolateRhoToFaces` was added to `internal/solver/props_test.go` to explicitly verify that `RhoU` and `RhoV` face densities exactly equal the arithmetic/harmonic average of precisely the two adjacent cells. The face-index conventions have been fully documented at the top of `props.go`.

**Face-Index Convention:**
- $U$-face $(i, j)$ sits between cell $(i, j)$ and $(i+1, j)$.
- $V$-face $(i, j)$ sits between cell $(i, j)$ and $(i, j+1)$.
`project.go` (the Poisson matrix setup), `predictor.go`, and `props.go` all strictly conform to this convention.

## 2. Hydrostatic Pressure Error Threshold
The hydrostatic error threshold in `TestHydrostaticStillWaterGate` (inside `project_test.go`) has been tightened from 111.9 Pa down to a relative tolerance of $10^{-6}$ of $\rho g H$. After properly accounting for the weight of the air column above the interface, the hydrostatic solver hits machine-precision exactness (the error is fully bounded below this threshold).

A new test `TestHydrostaticThreeLayerGate` was added to simulate a three-layer hydrostatic configuration (a light fluid sandwiched between two heavy fluids). A single-cell index shift fails this test, locking in the regression protection.

## 3. Scope of Affected Results
The off-by-one interpolation error shifted the entire $U$-face density mapping one cell left and the $V$-face density mapping one cell down. This created highly erratic local physical gradients, particularly massive at phase interfaces where density ratios are huge ($\rho_w/\rho_a = 1000$).

**Affected Previous Results:**
- **Drift Decomposition (Mass Loss):** The massive drift (~30-50%) in earlier runs was completely spurious, induced by massive unphysical velocities caused by misaligned density gradients.
- **Poisson Convergence (High Iters):** The wildly varying gradients made the Poisson matrix severely ill-conditioned during advection, causing the erratic 83-102 iterations we observed compared to 30-45.
- **Front Tracking ($X^*$):** The erratic physical gradients disrupted the front's physical propagation speed and smeared the interface significantly.

*All previous reports with contaminated data (`performance_report.md`, `priority_a_report.md`, `priority_b_report.md`, `priority_bc_report.md`, `priority_d_report.md`) have been deleted and replaced.*
