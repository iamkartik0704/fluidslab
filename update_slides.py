import re

# Front-speed data for slide 7 (Lobovsky et al., Table 1). Drawn as plain HTML/CSS
# bars so it needs no extra canvas code in main.js.
FRONT_SPEEDS = [
    ("Ritter", 2.0, True),
    ("Lobovsk&yacute; 300", 1.56, False),
    ("Lobovsk&yacute; 600", 1.34, False),
    ("M&amp;M 57", 1.48, False),
    ("M&amp;M 114", 1.69, False),
    ("Dressler 110", 1.7, False),
    ("Hu", 1.21, False),
    ("Koshizuka", 1.3, False),
]


def build_front_speed_bars():
    rows = []
    for label, value, is_ritter in FRONT_SPEEDS:
        pct = round(value / 2.0 * 100, 1)
        colour = "var(--blue)" if is_ritter else "var(--yellow)"
        rows.append(
            '<div style="display: flex; align-items: center; margin: 4px 0;">'
            f'<div style="width: 140px; font-size: 14px; font-weight: bold;">{label}</div>'
            '<div style="flex: 1; background: white; border: 2px solid var(--ink); height: 20px;">'
            f'<div style="width: {pct}%; height: 100%; background: {colour};"></div>'
            '</div>'
            f'<div style="width: 50px; text-align: right; font-family: var(--font-mono);">{value}</div>'
            '</div>'
        )
    return "\n        ".join(rows)


new_slides = '''const slides = [
  {
    title: "1. THEORETICAL FOUNDATION",
    content: `
      <p><strong>Problem:</strong> dam break is a two-phase (water + air) flow. A water column is released instantly and collapses under gravity, so the region occupied by water is unknown and changes every step.</p>
      <div class="kpi-card kpi-card--blue" style="margin: 15px 0; align-items: center;">
        <div class="kpi-card__title">Navier-Stokes (Momentum, Mixture)</div>
        <div style="font-family: var(--font-mono); font-size: 24px;">&rho;[&part;u/&part;t + (u &middot; &nabla;)u] = &minus;&nabla;p + &nabla; &middot; [&mu;(&nabla;u + &nabla;u<sup>T</sup>)] + &rho;g</div>
      </div>
      <div class="kpi-card kpi-card--yellow" style="margin: 15px 0; align-items: center;">
        <div class="kpi-card__title">Continuity (Mass)</div>
        <div style="font-family: var(--font-mono); font-size: 24px;">&nabla; &middot; u = 0</div>
      </div>
      <p><strong>Mixture Properties (Linear blend of &alpha;):</strong> &rho; = &alpha;&middot;&rho;<sub>water</sub> + (1&minus;&alpha;)&middot;&rho;<sub>air</sub>, &mu; = &alpha;&middot;&mu;<sub>water</sub> + (1&minus;&alpha;)&middot;&mu;<sub>air</sub></p>
      <p><strong>VOF method:</strong> a fixed grid spans both fluids. &alpha; = water volume / cell volume: &alpha; = 1 water, &alpha; = 0 air, 0 &lt; &alpha; &lt; 1 interface cell. &rho; and &mu; are recomputed from &alpha; every step.</p>
    `
  },
  {
    title: "2. ASSUMPTIONS AND WALL CONDITIONS",
    content: `
      <div class="kpi-card kpi-card--yellow" style="margin: 15px 0; align-items: flex-start;">
        <div class="kpi-card__title">What Starts the Motion</div>
        <div>In the dam break, gravity is the only thing that starts the motion. Nothing pushes the water; the column just collapses under its own weight.</div>
      </div>
      <div class="kpi-card kpi-card--blue" style="margin: 15px 0; align-items: flex-start;">
        <div class="kpi-card__title">Continuum Assumption</div>
        <div>Water and air are treated as smooth, continuous fluids, not as individual molecules. Every cell then has a density, velocity and pressure.</div>
      </div>
      <table class="table" style="margin: 20px 0; background: white; border: 2px solid var(--ink);">
        <thead style="background: var(--yellow); color: var(--ink);">
          <tr><th style="color: var(--ink);">Condition</th><th style="color: var(--ink);">Meaning</th></tr>
        </thead>
        <tbody>
          <tr><td style="font-weight: bold;">No-penetration</td><td>Fluid cannot cross the wall. Always true for solid walls.</td></tr>
          <tr><td style="font-weight: bold;">No-slip</td><td>Fluid touching the wall is stuck to it (zero speed there). Realistic for real walls.</td></tr>
          <tr><td style="font-weight: bold;">Free-slip</td><td>Fluid slides along the wall with no friction. An idealisation.</td></tr>
          <tr><td style="font-weight: bold;">Open boundary</td><td>Fluid and air can pass freely, like the open top of a tank.</td></tr>
        </tbody>
      </table>
    `
  },
  {
    title: "3. INTERFACE TRANSPORT",
    content: `
      <p><strong>&alpha; Transport: No diffusion, no source</strong></p>
      <div class="kpi-card kpi-card--blue" style="margin: 15px 0; align-items: center;">
        <div class="kpi-card__title">Volume Fraction Advection</div>
        <div style="font-family: var(--font-mono); font-size: 24px;">&part;&alpha;/&part;t + u &middot; &nabla;&alpha; = 0</div>
      </div>
      <p>Equivalent to &part;&alpha;/&part;t + &nabla; &middot; (&alpha;u) = 0 because &nabla; &middot; u = 0 (conservative form).</p>
      <table class="table" style="margin: 20px 0; background: white; border: 2px solid var(--ink);">
        <thead style="background: var(--yellow); color: var(--ink);">
          <tr><th style="color: var(--ink);">Scheme</th><th style="color: var(--ink);">What it does</th><th style="color: var(--ink);">Trade-off</th></tr>
        </thead>
        <tbody>
          <tr><td>Plain upwind</td><td>&alpha; treated like any other scalar</td><td>Easy, but interface smears</td></tr>
          <tr><td style="font-weight: bold; color: var(--blue);">Donor-acceptor (USED)</td><td style="font-weight: bold; color: var(--blue);">Blends upwind/downwind by interface orientation and donor fullness</td><td style="font-weight: bold; color: var(--blue);">Sharp without geometry; needs CFL &lt; 1</td></tr>
          <tr><td>PLIC</td><td>Straight-line interface per cell, exact geometric fluxes</td><td>Sharpest; many edge cases</td></tr>
          <tr><td>Van Leer</td><td>Flux limiter for momentum, not for &alpha;</td><td>High order when smooth, bounded at jumps</td></tr>
        </tbody>
      </table>
    `
  },
  {
    title: "4. TIME STEP: CHORIN PROJECTION",
    content: `
      <p><strong>1. Predictor (No Pressure):</strong></p>
      <p style="font-family: var(--font-mono); font-size: 18px; margin-left: 20px;">u* = u + &Delta;t &middot; [ &minus;(u &middot; &nabla;)u + (1/&rho;)&nabla; &middot; (&mu;&nabla;u) + g ]</p>
      <div class="kpi-card kpi-card--green" style="margin: 15px 0; align-items: center;">
        <div class="kpi-card__title">2. Pressure Poisson</div>
        <div style="font-family: var(--font-mono); font-size: 24px;">&nabla; &middot; ((1/&rho;)&nabla;p) = (1/&Delta;t) &nabla; &middot; u*</div>
      </div>
      <p><strong>3. Projection:</strong> u(n+1) = u* &minus; (&Delta;t/&rho;)&nabla;p so that &nabla; &middot; u(n+1) = 0</p>
      <p><strong>4.</strong> Advect &alpha; with the corrected velocity (donor-acceptor), then clip &alpha; to [0, 1].</p>
      <p><strong>5.</strong> Recompute &rho; and &mu; from &alpha;; repeat with a CFL-limited &Delta;t.</p>
      <p><em>Why split?</em> u* is where the water wants to go from gravity, momentum and viscosity alone. The Poisson solve finds the pressure that removes the divergence of u*; the projection applies it.</p>
    `
  },
  {
    title: "5. DENSITY RATIO AND PRESSURE SOLVE",
    content: `
      <table class="table" style="margin: 20px 0; background: white; border: 2px solid var(--ink);">
        <thead style="background: var(--yellow); color: var(--ink);">
          <tr><th style="color: var(--ink);">Fluid</th><th style="color: var(--ink);">Density (&rho;)</th><th style="color: var(--ink);">Kinematic Viscosity (&nu;)</th><th style="color: var(--ink);">Ratio</th></tr>
        </thead>
        <tbody>
          <tr><td>Water</td><td>998.0 kg/m&sup3;</td><td>1.00e-6 m&sup2;/s</td><td rowspan="2" style="vertical-align: middle; font-weight: bold; color: var(--blue);">~832&times; (&rho;)</td></tr>
          <tr><td>Air</td><td>1.20 kg/m&sup3;</td><td>1.48e-5 m&sup2;/s</td></tr>
        </tbody>
      </table>
      <p><strong>Why it is ill-conditioned:</strong> The coefficient 1/&rho; jumps ~832&times; across a single interface cell, so the Poisson matrix entries span about three orders of magnitude and plain CG converges slowly.</p>
      <p><strong>FIX: IC(0)-PRECONDITIONED CG</strong></p>
      <p>2450 &rarr; 112 iterations<br>18.5 &rarr; 2.1 ms per step; scaling O(N^1.5) &rarr; O(N^1.2)</p>
      <p><strong>Matrix assembly:</strong> the 5-point Laplacian must treat fluid and empty cells carefully to stay symmetric positive definite.</p>
    `
  },
  {
    title: "6. DAM-BREAK THEORY: RITTER",
    content: `
      <p><strong>Shallow Water (Saint-Venant):</strong> h<sub>t</sub> + (h &middot; u)<sub>x</sub> = 0, u<sub>t</sub> + u &middot; u<sub>x</sub> + g &middot; h<sub>x</sub> = 0</p>
      <p><strong>Water at rest (h = H)</strong></p>
      <div class="kpi-card kpi-card--yellow" style="margin: 15px 0; align-items: center;">
        <div class="kpi-card__title">Ritter Solution, c<sub>0</sub> = &radic;(gH)</div>
        <div style="font-family: var(--font-mono); font-size: 24px;">u = (2/3)(c<sub>0</sub> + x/t)<br>h = (2c<sub>0</sub> &minus; x/t)&sup2; / (9g)</div>
      </div>
      <p><em>for &minus;c<sub>0</sub>t &le; x &le; 2c<sub>0</sub>t</em></p>
      <div style="display: flex; justify-content: space-between; border-top: 4px solid var(--ink); padding-top: 6px; margin: 10px 0 15px 0; font-family: var(--font-mono); font-size: 16px;">
        <span>&minus;c<sub>0</sub>&middot;t</span>
        <span>gate x=0</span>
        <span>2c<sub>0</sub>&middot;t</span>
      </div>
      <p><strong>What it predicts (valid for T &lt; L<sub>0</sub>/c<sub>0</sub>, before reflection off the back wall):</strong><br>
      Front speed 2&radic;(gH) with zero depth. At the gate: h = 4H/9, u = c (Fr = 1), constant discharge q = (8/27)&radic;g &middot; H<sup>1.5</sup>.</p>
    `
  },
  {
    title: "7. REALITY CHECK: FRICTION AND SCALING",
    content: `
      <p><strong>Friction (Dressler, Whitham):</strong> Blunt front, vertical tangent, finite depth. Slower than 2&radic;(gH).</p>
      <p><strong>Average Front Speed v/&radic;(gH), t* &gt; 1</strong></p>
      <div style="margin: 10px 0 15px 0; padding: 10px; background: white; border: 2px solid var(--ink);">
        __FRONT_SPEED_BARS__
      </div>
      <p><strong>Lab Scale, H = 300 mm:</strong> Re = 3.8e6, We = 1.64e5, Fr = 1. Large We: surface tension negligible.</p>
      <p><strong>Early Times, T* &lt; 1:</strong> Not hydrostatic; gate removal (3.5&ndash;4.5 m/s) matters.</p>
      <p style="font-size: 13px;"><em>Source: Lobovsk&yacute; et al., Table 1. M&amp;M = Martin &amp; Moyce; numbers after names are H in mm.</em></p>
    `
  },
  {
    title: "8. VALIDATION",
    content: `
      <table class="table" style="margin: 15px 0; background: white; border: 2px solid var(--ink);">
        <thead style="background: var(--yellow); color: var(--ink);">
          <tr><th style="color: var(--ink);">Source</th><th style="color: var(--ink);">Length</th><th style="color: var(--ink);">Time</th></tr>
        </thead>
        <tbody>
          <tr><td>Martin &amp; Moyce</td><td>initial width a</td><td>T = t&radic;(g/a)</td></tr>
          <tr><td>Lobovsk&yacute;</td><td>depth H</td><td>t* = t&radic;(g/H)</td></tr>
          <tr><td style="font-weight: bold; color: var(--blue);">This solver</td><td style="font-weight: bold; color: var(--blue);">L<sub>0</sub></td><td style="font-weight: bold; color: var(--blue);">T* = t&radic;(g/L<sub>0</sub>)</td></tr>
        </tbody>
      </table>
      <p><strong>Early-Time Check (Ritter):</strong> X* = 1 + 2T*. Front starts at x/L<sub>0</sub> = 1, then moves at 2&radic;(gH); square column.</p>
      <p>Match the axis convention of the data you overlay before comparing curves.</p>
      <div style="border: 2px solid var(--ink); padding: 10px; background: white; margin: 15px 0;">
        <canvas id="deckValChart" width="800" height="250" style="width: 100%; height: 250px;"></canvas>
      </div>
      <p><strong>Acceptance Checks:</strong></p>
      <ul>
        <li>&int;&alpha; dV stays within a stated tolerance (live readout)</li>
        <li>Front X*(T*) vs Martin &amp; Moyce data</li>
        <li>Front stays below Ritter: 1 + 2T*</li>
        <li>Late front speed about 1.1&ndash;1.75 &radic;(gH)</li>
        <li>&nabla; &middot; u &approx; 0 after every projection</li>
        <li>Error shrinks under grid and &Delta;t refinement</li>
      </ul>
    `
  },
  {
    title: "9. IMPACT PRESSURE AT A WALL",
    content: `
      <div class="kpi-card kpi-card--green" style="margin: 15px 0; align-items: center;">
        <div class="kpi-card__title">Median Peak, 3 mm above bed</div>
        <div style="font-family: var(--font-mono); font-size: 24px;">&approx; 3 &times; &rho;gH</div>
      </div>
      <p><strong>97.5th percentile &approx; 4.5&times;.</strong></p>
      <p><strong>P / &rho;V&sup2;, measured:</strong> 1.25 (vs 0.5 for a steady impinging jet).</p>
      <p><strong>Repeated runs per fill height: 100.</strong> Peak pressure is a random variable.</p>
      <p><strong>Time scales:</strong> rise 1.5&ndash;4.5 ms, decay about 10&times; longer. Impulse &int;P dt &approx; &frac12; &times; peak &times; impact time (within about 25%).</p>
      <p><strong>Scaling:</strong> lowest-sensor peak &prop; H (Froude: P ~ &rho;gH); higher sensors are not linear in H.</p>
      <p><em>For your solver:</em> incompressible VOF gives a sharp, grid-dependent spike. Compare arrival time and impulse with the median and 95% band, not one peak.</p>
    `
  },
  {
    title: "10. SCOPE FOR IMPROVEMENT",
    content: `
      <p><strong>Higher-Order Interface Tracking:</strong> Replace donor-acceptor with PLIC for a continuous, sharp geometric interface reconstruction.</p>
      <p><strong>Hardware Acceleration:</strong> Port grid sweeps and the Poisson solver to WebGPU compute shaders; scale to millions of cells and 3D.</p>
      <p><strong>Physics Gaps:</strong> Air compressibility and entrapment at impact, wet-bed jets, and turbulence at Re ~ 1e6 on an under-resolved grid.</p>
      <p><strong>3D Effects:</strong> The lab flow stops being 2D at H = 600 mm, which a 2D solver cannot show.</p>
    `
  }
];'''

new_slides = new_slides.replace("__FRONT_SPEED_BARS__", build_front_speed_bars())

MAIN_JS = r'c:\Users\iamka\Desktop\final fluids\fluidslab\ui\js\main.js'

with open(MAIN_JS, 'r', encoding='utf-8') as f:
    code = f.read()

# Lambda replacement so backslashes in the new text are never treated as regex escapes.
code, n = re.subn(r'const slides = \[.*?\];', lambda m: new_slides, code, count=1, flags=re.DOTALL)
if n != 1:
    raise SystemExit("Could not find the 'const slides = [...]' block in main.js; nothing written.")

with open(MAIN_JS, 'w', encoding='utf-8') as f:
    f.write(code)

print("Updated slides array in main.js (10 slides).")
