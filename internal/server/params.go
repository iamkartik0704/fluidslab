package server

import "math"

// ServerParams is the parameter set the UI can change. Everything passes
// through Clamp before reaching the solver; clamping is server-side so no
// client can push the solver into an untested regime.
type ServerParams struct {
	AspectRatio    float64 `json:"aspectRatio"`    // H0 / L0
	CellsPerL0     int     `json:"cellsPerL0"`     // 8 / 16 / 24 / 32
	ViscosityScale float64 `json:"viscosityScale"` // multiplier on water viscosity
	DensityRatio   float64 `json:"densityRatio"`   // rho_water / rho_air
	FreeSlip       bool    `json:"freeSlip"`       // rendered but DISABLED in the UI (solver issue under investigation)
	Scheme         string  `json:"scheme"`         // "first-order" | "donor-acceptor" | "van leer"
	TimeScale      string  `json:"timeScale"`      // "sqrt(2g/L0)" | "sqrt(g/L0)"
}

// DefaultParams returns the defaults the prompt mandates: aspect ratio 2,
// 16 cells per L0, no-slip, donor-acceptor (the solver's validated default).
func DefaultParams() ServerParams {
	return ServerParams{
		AspectRatio:    2.0,
		CellsPerL0:     16,
		ViscosityScale: 1.0,
		DensityRatio:   998.0 / 1.2,
		FreeSlip:       false,
		Scheme:         "donor-acceptor",
		TimeScale:      "sqrt(2g/L0)",
	}
}

// ValidatedRanges documents the tested envelope; the UI highlights anything
// outside it with a warning badge.
var ValidatedRanges = map[string][2]float64{
	"aspectRatio":    {1.0, 4.0},
	"cellsPerL0":     {8, 32},
	"viscosityScale": {0.01, 1000},
	"densityRatio":   {1, 2000},
}

// SchemeNames lists the advection schemes offered in the UI, in order.
var SchemeNames = []string{"first-order", "donor-acceptor", "van leer"}

// TimeScaleNames lists the time-scaling conventions offered in the UI.
var TimeScaleNames = []string{"sqrt(2g/L0)", "sqrt(g/L0)"}

// Clamp returns a copy of p forced into the validated envelope:
//   - aspect ratio clamped to [1, 4]
//   - cells-per-L0 snapped to the nearest of 8/16/24/32
//   - viscosity scale clamped to [1e-2, 1e3]
//   - density ratio clamped to [1, 2000]
//   - scheme and time-scale snapped to the known enum values
func (p ServerParams) Clamp() ServerParams {
	c := p
	c.AspectRatio = clampF(p.AspectRatio, 1.0, 4.0)
	c.CellsPerL0 = clampI(p.CellsPerL0, 8, 32)
	c.ViscosityScale = clampF(p.ViscosityScale, 1e-2, 1e3)
	c.DensityRatio = clampF(p.DensityRatio, 1, 2000)

	// Snap cellsPerL0 onto the 8/16/24/32 ladder.
	switch {
	case c.CellsPerL0 < 12:
		c.CellsPerL0 = 8
	case c.CellsPerL0 < 20:
		c.CellsPerL0 = 16
	case c.CellsPerL0 < 28:
		c.CellsPerL0 = 24
	default:
		c.CellsPerL0 = 32
	}

	c.Scheme = matchEnum(p.Scheme, SchemeNames, "donor-acceptor")
	c.TimeScale = matchEnum(p.TimeScale, TimeScaleNames, "sqrt(2g/L0)")
	return c
}

// OutOfRange reports each parameter that sits outside the validated envelope,
// for the UI warning badge. Returns an empty slice when everything is inside.
func (p ServerParams) OutOfRange() []string {
	var out []string
	if p.AspectRatio < 1.0 || p.AspectRatio > 4.0 {
		out = append(out, "aspect ratio")
	}
	if p.CellsPerL0 < 8 || p.CellsPerL0 > 32 || (p.CellsPerL0%8 != 0) {
		out = append(out, "resolution")
	}
	if p.ViscosityScale < 0.01 || p.ViscosityScale > 1000 {
		out = append(out, "viscosity scale")
	}
	if p.DensityRatio < 1 || p.DensityRatio > 2000 {
		out = append(out, "density ratio")
	}
	return out
}

func clampF(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func matchEnum(s string, allowed []string, def string) string {
	for _, a := range allowed {
		if s == a {
			return s
		}
	}
	return def
}
