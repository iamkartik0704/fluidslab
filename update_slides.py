import json
import re

# ---------------------------------------------------------------------------
# Palette taken from the deck XML
# ---------------------------------------------------------------------------
BLUE = "#2E4BFF"
YELLOW = "#FFD600"
GREEN = "#0DA678"
RED = "#FF3366"
INK = "#000000"


# ---------------------------------------------------------------------------
# Small HTML helpers (all styling is inline, on top of your existing
# .kpi-card / .kpi-card--blue|yellow|green / .table classes)
# ---------------------------------------------------------------------------
def card(color, title, body="", align="center", margin="0", extra=""):
    """A neo-brutalist card. color: blue | yellow | green | None (white card)."""
    cls = "kpi-card" + (f" kpi-card--{color}" if color else "")
    white = "" if color else "background: #fff; color: var(--ink);"
    if align == "center":
        al = "align-items: center; text-align: center;"
    else:
        al = "align-items: flex-start; text-align: left;"
    return (
        f'<div class="{cls}" style="margin: {margin}; {al} justify-content: center; {white} {extra}">'
        f'<div class="kpi-card__title">{title}</div>{body}</div>'
    )


def mono(text, size=18, weight="normal", extra=""):
    return (
        f'<div style="font-family: var(--font-mono); font-size: {size}px; '
        f'font-weight: {weight}; line-height: 1.35; {extra}">{text}</div>'
    )


def grid(cols, items, gap=18, margin="15px 0", extra=""):
    return (
        f'<div style="display: grid; grid-template-columns: {cols}; gap: {gap}px; '
        f'margin: {margin}; {extra}">' + "".join(items) + "</div>"
    )


def stack(items, gap=18):
    return f'<div style="display: flex; flex-direction: column; gap: {gap}px;">' + "".join(items) + "</div>"


TH = 'style="color: var(--ink);"'


def table(headers, rows, first_bold=True, highlight_row=None, margin="15px 0"):
    """Yellow-header table. highlight_row: index of the row whose FIRST cell is blue + bold."""
    head = "".join(f"<th {TH}>{h}</th>" for h in headers)
    body = []
    for i, row in enumerate(rows):
        cells = []
        for j, c in enumerate(row):
            style = ""
            if j == 0 and highlight_row == i:
                style = ' style="font-weight: bold; color: var(--blue);"'
            elif j == 0 and first_bold:
                style = ' style="font-weight: bold;"'
            cells.append(f"<td{style}>{c}</td>")
        body.append("<tr>" + "".join(cells) + "</tr>")
    return (
        f'<table class="table" style="margin: {margin}; background: white; border: 2px solid var(--ink);">'
        f'<thead style="background: var(--yellow); color: var(--ink);"><tr>{head}</tr></thead>'
        f'<tbody>{"".join(body)}</tbody></table>'
    )


# ---------------------------------------------------------------------------
# Slide 6 figure: Ritter free-surface profile (SVG, same geometry as the deck)
# The deck shape is flat for the first 25 % of the width, then
#   y = H * (1 - (1 - u)^2),  u = (x - 0.25 W) / (0.75 W)
# ---------------------------------------------------------------------------
def ritter_svg():
    W, H = 484.6, 173.7           # profile size in viewBox units
    ox, oy = 18.3, 41.1           # profile offset inside the box
    flat = 0.25
    pts = [(0, H), (0, 0), (W * flat, 0)]
    n = 24
    for i in range(1, n + 1):
        u = i / n
        pts.append((W * (flat + (1 - flat) * u), H * (1 - (1 - u) ** 2)))
    poly = " ".join(f"{ox + x:.1f},{oy + y:.1f}" for x, y in pts)
    return (
        '<svg viewBox="0 0 521.2 269.7" style="width: 100%; height: auto; display: block;" '
        'xmlns="http://www.w3.org/2000/svg" font-family="Courier New, monospace">'
        '<text x="18.3" y="25" font-size="10.5" font-weight="bold">WATER AT REST (h = H)</text>'
        f'<polygon points="{poly}" fill="{BLUE}" stroke="{INK}" stroke-width="2.9" stroke-linejoin="round"/>'
        f'<line x1="260.6" y1="32" x2="260.6" y2="233" stroke="{INK}" stroke-width="2.2" stroke-dasharray="9 6"/>'
        '<text x="139" y="253" font-size="10.5" font-weight="bold" text-anchor="middle">&minus;c0&middot;t</text>'
        '<text x="260.6" y="253" font-size="10.5" font-weight="bold" text-anchor="middle">gate x=0</text>'
        '<text x="502" y="253" font-size="10.5" font-weight="bold" text-anchor="end">2c0&middot;t</text>'
        "</svg>"
    )


# ---------------------------------------------------------------------------
# Slide 7 figure: front-speed bar chart (SVG, same data/colours as the deck)
# gapWidth 40, y-axis 0..2.4 with 0.5 steps, Ritter bar in red.
# ---------------------------------------------------------------------------
FRONT_SPEEDS = [
    (["Ritter"], 2.00),
    (["Lobovsk&yacute;", "300"], 1.56),
    (["Lobovsk&yacute;", "600"], 1.34),
    (["M&amp;M 57"], 1.48),
    (["M&amp;M 114"], 1.69),
    (["Dressler", "110"], 1.70),
    (["Hu"], 1.21),
    (["Koshizuka"], 1.30),
]


def front_speed_svg():
    left, right, top, bottom = 46, 592, 24, 236     # plot area
    vmax = 2.4
    ys = lambda v: bottom - (bottom - top) * v / vmax
    out = [
        '<svg viewBox="0 0 600 290" style="width: 100%; height: auto; display: block;" '
        'xmlns="http://www.w3.org/2000/svg" font-family="Courier New, monospace" font-size="11">'
    ]
    for tick in (0, 0.5, 1, 1.5, 2):
        y = ys(tick)
        colour = "#888888" if tick == 0 else "#D9D9D9"
        out.append(f'<line x1="{left}" y1="{y:.1f}" x2="{right}" y2="{y:.1f}" stroke="{colour}" stroke-width="1"/>')
        label = f"{tick:g}"
        out.append(f'<text x="{left - 8}" y="{y + 4:.1f}" text-anchor="end">{label}</text>')
    out.append(f'<line x1="{left}" y1="{top}" x2="{left}" y2="{bottom}" stroke="#888888" stroke-width="1"/>')
    cat_w = (right - left) / len(FRONT_SPEEDS)
    bar_w = cat_w / 1.4
    for i, (label, value) in enumerate(FRONT_SPEEDS):
        cx = left + cat_w * (i + 0.5)
        x = cx - bar_w / 2
        y = ys(value)
        colour = RED if i == 0 else BLUE
        out.append(f'<rect x="{x:.1f}" y="{y:.1f}" width="{bar_w:.1f}" height="{bottom - y:.1f}" fill="{colour}"/>')
        out.append(f'<text x="{cx:.1f}" y="{y - 6:.1f}" text-anchor="middle">{value:.2f}</text>')
        for k, line in enumerate(label):
            out.append(f'<text x="{cx:.1f}" y="{bottom + 16 + 13 * k}" text-anchor="middle">{line}</text>')
    out.append("</svg>")
    return "".join(out)


# ---------------------------------------------------------------------------
# Slides
# ---------------------------------------------------------------------------
slides = []


def add(title, content):
    assert "`" not in content and "${" not in content, "content would break the JS template literal"
    slides.append((title, content))


# 1 ---------------------------------------------------------------------
add("1. THEORETICAL FOUNDATION", "".join([
    '<p><strong>Problem:</strong> dam break is a two-phase (water + air) flow. A water column is released '
    'instantly and collapses under gravity, so the region occupied by water is unknown and changes every step.</p>',
    card("blue", "Navier-Stokes (Momentum, Mixture)",
         mono("&rho;[&part;u/&part;t + (u &middot; &nabla;)u] = &minus;&nabla;p + &nabla; &middot; "
              "[&mu;(&nabla;u + &nabla;u<sup>T</sup>)] + &rho;g", 22),
         margin="15px 0"),
    grid("1fr 2fr", [
        card("yellow", "Continuity (Mass)", mono("&nabla; &middot; u = 0", 24)),
        card("green", "Mixture Properties (Linear blend of &alpha;)",
             mono("&rho; = &alpha;&middot;&rho;<sub>water</sub> + (1&minus;&alpha;)&middot;&rho;<sub>air</sub><br>"
                  "&mu; = &alpha;&middot;&mu;<sub>water</sub> + (1&minus;&alpha;)&middot;&mu;<sub>air</sub>", 18)),
    ]),
    '<p><strong>VOF method:</strong> a fixed grid spans both fluids. &alpha; = water volume / cell volume: '
    '&alpha; = 1 water, &alpha; = 0 air, 0 &lt; &alpha; &lt; 1 interface cell. '
    '&rho; and &mu; are recomputed from &alpha; every step.</p>',
]))

# 2 ---------------------------------------------------------------------
add("2. ASSUMPTIONS AND WALL CONDITIONS", "".join([
    grid("1fr 1fr", [
        card("yellow", "What Starts the Motion",
             '<div style="font-size: 17px;">In the dam break, gravity is the only thing that starts the motion. '
             'Nothing pushes the water; the column just collapses under its own weight.</div>', align="left"),
        card("green", "Continuum Assumption",
             '<div style="font-size: 17px;">Water and air are treated as smooth, continuous fluids, not as '
             'individual molecules. Every cell then has a density, velocity and pressure.</div>', align="left"),
    ]),
    table(["Condition", "Meaning"], [
        ["No-penetration", "Fluid cannot cross the wall. Always true for solid walls."],
        ["No-slip", "Fluid touching the wall is stuck to it (zero speed there). Realistic for real walls."],
        ["Free-slip", "Fluid slides along the wall with no friction. An idealisation."],
        ["Open boundary", "Fluid and air can pass freely, like the open top of a tank."],
    ], margin="20px 0"),
]))

# 3 ---------------------------------------------------------------------
add("3. INTERFACE TRANSPORT", "".join([
    card("green", "&alpha; Transport: No diffusion, no source",
         mono("&part;&alpha;/&part;t + u &middot; &nabla;&alpha; = 0", 26) +
         mono("equivalent to &part;&alpha;/&part;t + &nabla; &middot; (&alpha;u) = 0 because "
              "&nabla; &middot; u = 0 (conservative form)", 15),
         margin="15px 0"),
    table(["Scheme", "What it does", "Trade-off"], [
        ["Plain upwind", "&alpha; treated like any other scalar", "Easy, but interface smears"],
        ["Donor-acceptor (USED)", "Blends upwind/downwind by interface orientation and donor fullness",
         "Sharp without geometry; needs CFL &lt; 1"],
        ["PLIC", "Straight-line interface per cell, exact geometric fluxes", "Sharpest; many edge cases"],
        ["Van Leer", "Flux limiter for momentum, not for &alpha;", "High order when smooth, bounded at jumps"],
    ], highlight_row=1, margin="20px 0"),
]))

# 4 ---------------------------------------------------------------------
add("4. TIME STEP: CHORIN PROJECTION", "".join([
    grid("1fr 1fr 1fr", [
        card("blue", "1. Predictor (No Pressure)",
             mono("u* = u + &Delta;t &middot; [ &minus;(u &middot; &nabla;)u<br>"
                  "+ (1/&rho;)&nabla; &middot; (&mu;&nabla;u) + g ]", 17), extra="min-height: 150px;"),
        card("green", "2. Pressure Poisson",
             mono("&nabla; &middot; ((1/&rho;)&nabla;p)<br>= (1/&Delta;t) &nabla; &middot; u*", 20),
             extra="min-height: 150px;"),
        card("yellow", "3. Projection",
             mono("u(n+1) = u* &minus; (&Delta;t/&rho;)&nabla;p<br>so that &nabla; &middot; u(n+1) = 0", 17),
             extra="min-height: 150px;"),
    ]),
    '<p><strong>4.</strong> Advect &alpha; with the corrected velocity (donor-acceptor), then clip &alpha; to [0, 1].</p>',
    '<p><strong>5.</strong> Recompute &rho; and &mu; from &alpha;; repeat with a CFL-limited &Delta;t.</p>',
    '<p><strong>Why split?</strong> u* is where the water wants to go from gravity, momentum and viscosity alone. '
    'The Poisson solve finds the pressure that removes the divergence of u*; the projection applies it.</p>',
]))

# 5 ---------------------------------------------------------------------
add("5. DENSITY RATIO AND PRESSURE SOLVE", "".join([
    '<table class="table" style="margin: 15px 0; background: white; border: 2px solid var(--ink);">'
    f'<thead style="background: var(--yellow); color: var(--ink);"><tr><th {TH}>Fluid</th>'
    f'<th {TH}>Density (&rho;)</th><th {TH}>Kinematic Viscosity (&nu;)</th><th {TH}>Ratio</th></tr></thead>'
    '<tbody>'
    '<tr><td style="font-weight: bold;">Water</td><td>998.0 kg/m&sup3;</td><td>1.00e-6 m&sup2;/s</td>'
    '<td rowspan="2" style="vertical-align: middle; text-align: center; font-weight: bold; color: var(--blue);">~832&times; (&rho;)</td></tr>'
    '<tr><td style="font-weight: bold;">Air</td><td>1.20 kg/m&sup3;</td><td>1.48e-5 m&sup2;/s</td></tr>'
    '</tbody></table>',
    grid("1fr 1fr", [
        card("yellow", "Why it is ill-conditioned",
             '<div style="font-size: 16px;">The coefficient 1/&rho; jumps ~832&times; across a single interface cell, '
             'so the Poisson matrix entries span about three orders of magnitude and plain CG converges slowly.</div>',
             align="left", extra="min-height: 130px;"),
        card("green", "Fix: IC(0)-preconditioned CG",
             mono("2450 &rarr; 112 iterations", 22, "bold") +
             mono("18.5 &rarr; 2.1 ms per step; scaling O(N^1.5) &rarr; O(N^1.2)", 15),
             align="left", extra="min-height: 130px;"),
    ], margin="25px 0 15px 0"),
    '<p><strong>Matrix assembly:</strong> the 5-point Laplacian must treat fluid and empty cells carefully '
    'to stay symmetric positive definite.</p>',
]))

# 6 ---------------------------------------------------------------------
add("6. DAM-BREAK THEORY: RITTER", "".join([
    grid("1fr 1fr", [
        '<div style="background: #fff; border: 3px solid var(--ink); box-shadow: 6px 6px 0 var(--ink); '
        'padding: 4px;">' + ritter_svg() + '</div>',
        stack([
            card("blue", "Shallow Water (Saint-Venant)",
                 mono("h<sub>t</sub> + (h &middot; u)<sub>x</sub> = 0<br>"
                      "u<sub>t</sub> + u &middot; u<sub>x</sub> + g &middot; h<sub>x</sub> = 0", 19)),
            card("yellow", "Ritter Solution, c<sub>0</sub> = &radic;(gH)",
                 mono("u = (2/3)(c<sub>0</sub> + x/t)<br>h = (2&middot;c<sub>0</sub> &minus; x/t)&sup2; / (9g)", 19) +
                 mono("for &minus;c<sub>0</sub>&middot;t &le; x &le; 2&middot;c<sub>0</sub>&middot;t", 14)),
        ], gap=20),
    ], margin="15px 0 20px 0"),
    card("green",
         "What it predicts (valid for T &lt; L<sub>0</sub>/c<sub>0</sub>, before reflection off the back wall)",
         '<div style="font-size: 17px;">Front speed 2&radic;(gH) with zero depth. At the gate: h = 4H/9, u = c (Fr = 1), '
         'constant discharge q = (8/27)&middot;&radic;g&middot;H<sup>1.5</sup>.</div>', align="left"),
]))

# 7 ---------------------------------------------------------------------
add("7. REALITY CHECK: FRICTION AND SCALING", "".join([
    grid("1.6fr 1fr", [
        '<div style="background: #fff; border: 3px solid var(--ink); padding: 10px 14px;">'
        '<div style="font-family: var(--font-mono); font-size: 12px; font-weight: bold; margin-bottom: 6px;">'
        'AVERAGE FRONT SPEED v/&radic;(gH), t* &gt; 1</div>' + front_speed_svg() + '</div>',
        stack([
            card("yellow", "Friction (Dressler, Whitham)",
                 '<div style="font-size: 17px;">Blunt front, vertical tangent, finite depth. '
                 'Slower than 2&radic;(gH).</div>', align="left"),
            card("blue", "Lab Scale, H = 300 mm",
                 '<div style="font-size: 17px;">Re = 3.8e6, We = 1.64e5, Fr = 1<br>'
                 'Large We: surface tension negligible</div>', align="left"),
            card("green", "Early Times, T* &lt; 1",
                 '<div style="font-size: 17px;">Not hydrostatic; gate removal (3.5&ndash;4.5 m/s) matters.</div>',
                 align="left"),
        ], gap=18),
    ], margin="15px 0 10px 0"),
    '<p style="font-size: 13px; margin: 6px 0 0 0;">Source: Lobovsk&yacute; et al., Table 1. '
    'M&amp;M = Martin &amp; Moyce; numbers after names are H in mm.</p>',
]))

# 8 ---------------------------------------------------------------------
checks = [
    "&int;&alpha; dV stays within a stated tolerance (live readout)",
    "Front X*(T*) vs Martin &amp; Moyce data",
    "Front stays below Ritter: 1 + 2T*",
    "Late front speed about 1.1&ndash;1.75 &radic;(gH)",
    "&nabla; &middot; u &approx; 0 after every projection",
    "Error shrinks under grid and &Delta;t refinement",
]
add("8. VALIDATION", "".join([
    grid("1.28fr 1fr", [
        '<div>' +
        table(["Source", "Length", "Time"], [
            ["Martin &amp; Moyce", "initial width a", "T = t&radic;(g/a)"],
            ["Lobovsk&yacute;", "depth H", "t* = t&radic;(g/H)"],
            ["This solver", "L<sub>0</sub>", "T* = t&radic;(g/L<sub>0</sub>)"],
        ], highlight_row=2, margin="0 0 18px 0") +
        card("blue", "Early-Time Check (Ritter)",
             mono("X* = 1 + 2&middot;T*", 26, "bold") +
             mono("front starts at x/L<sub>0</sub> = 1, then moves at 2&radic;(gH); square column", 14),
             extra="min-height: 120px;") +
        '<p style="margin: 14px 0 0 0;">Match the axis convention of the data you overlay before comparing curves.</p>'
        '</div>',
        card("yellow", "",
             '<div style="font-family: var(--font-mono); font-size: 24px; font-weight: bold; '
             'text-transform: uppercase; margin-bottom: 14px;">Acceptance Checks</div>'
             '<ul style="margin: 0; padding-left: 20px; font-size: 16px; line-height: 1.5;">' +
             "".join(f"<li>{c}</li>" for c in checks) + "</ul>",
             align="left", extra="justify-content: flex-start;"),
    ]),
    # Hidden on purpose: the deck has no chart here, but main.js may still draw into this canvas.
    '<canvas id="deckValChart" width="800" height="250" style="display: none;"></canvas>',
]))

# 9 ---------------------------------------------------------------------
add("9. IMPACT PRESSURE AT A WALL", "".join([
    grid("1fr 1fr 1fr", [
        card("blue", "Median Peak, 3 mm above bed",
             mono("&approx; 3 &times; &rho;gH", 30, "bold") +
             mono("97.5th percentile &approx; 4.5&times;", 15), extra="min-height: 150px;"),
        card("yellow", "P / &rho;V&sup2;, measured",
             mono("1.25", 36, "bold") +
             mono("vs 0.5 for a steady impinging jet", 15), extra="min-height: 150px;"),
        card("green", "Repeated runs per fill height",
             mono("100", 36, "bold") +
             mono("peak pressure is a random variable", 15), extra="min-height: 150px;"),
    ]),
    '<p><strong>Time scales:</strong> rise 1.5&ndash;4.5 ms, decay about 10&times; longer. '
    'Impulse &int;P dt &approx; &frac12; &times; peak &times; impact time (within about 25%).</p>',
    '<p><strong>Scaling:</strong> lowest-sensor peak &prop; H (Froude: P ~ &rho;gH); '
    'higher sensors are not linear in H.</p>',
    '<p><strong>For your solver:</strong> incompressible VOF gives a sharp, grid-dependent spike. '
    'Compare arrival time and impulse with the median and 95% band, not one peak.</p>',
]))

# 10 --------------------------------------------------------------------
add("10. SCOPE FOR IMPROVEMENT", grid("1fr 1fr", [
    card("yellow", "Higher-Order Interface Tracking",
         '<div style="font-size: 17px;">Replace donor-acceptor with PLIC for a continuous, sharp geometric '
         'interface reconstruction.</div>', align="left", extra="min-height: 150px;"),
    card("green", "Hardware Acceleration",
         '<div style="font-size: 17px;">Port grid sweeps and the Poisson solver to WebGPU compute shaders; '
         'scale to millions of cells and 3D.</div>', align="left", extra="min-height: 150px;"),
    card("blue", "Physics Gaps",
         '<div style="font-size: 17px;">Air compressibility and entrapment at impact, wet-bed jets, and '
         'turbulence at Re ~ 1e6 on an under-resolved grid.</div>', align="left", extra="min-height: 150px;"),
    card(None, "3D Effects",
         '<div style="font-size: 17px;">The lab flow stops being 2D at H = 600 mm, which a 2D solver '
         'cannot show.</div>', align="left", extra="min-height: 150px;"),
], gap=24))


# ---------------------------------------------------------------------------
# Assemble the JS and patch main.js
# ---------------------------------------------------------------------------
def js_slide(title, content):
    return "  {\n    title: " + json.dumps(title) + ",\n    content: `\n      " + content + "\n    `\n  }"


new_slides = "const slides = [\n" + ",\n".join(js_slide(t, c) for t, c in slides) + "\n];"

MAIN_JS = r'c:\Users\iamka\Desktop\final fluids\fluidslab\ui\js\main.js'

if __name__ == "__main__":
    with open(MAIN_JS, 'r', encoding='utf-8') as f:
        code = f.read()

    # Lambda replacement: backslashes in the new text are never treated as regex escapes.
    code, n = re.subn(r'const slides = \[.*?\];', lambda m: new_slides, code, count=1, flags=re.DOTALL)
    if n != 1:
        raise SystemExit("Could not find the 'const slides = [...]' block in main.js; nothing written.")

    with open(MAIN_JS, 'w', encoding='utf-8') as f:
        f.write(code)

    print(f"Updated slides array in main.js ({len(slides)} slides).")
