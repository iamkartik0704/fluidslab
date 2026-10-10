package solver

// Snapshot is an immutable, copy-out view of the simulation state at a moment
// in time. The server broadcasts these to websocket clients; it never mutates
// them, so solver internals stay read-only from the server side.
type Snapshot struct {
	Nx, Ny int
	Dx, Dy float64 // cell size in metres

	// Initial column outline (dam-break setup), metres.
	L0, H0 float64
	Width  float64 // fluid domain width (cells 1..Nx)
	Height float64 // fluid domain height (cells 1..Ny)

	// Scalars.
	Time           float64 // simulated time [s]
	TStar          float64 // dimensionless time per the configured convention
	Step           int
	DT             float64
	CFL            float64 // last accepted advective CFL number
	MaxDiv         float64
	Volume         float64 // current fluid volume [m^2 in 2D]
	RefVolume      float64 // initial volume (drift measured against this)
	VolumeDriftPct float64

	FrontX          float64 // front position [m] as reported by the solver
	FrontXStar      float64 // FrontX / L0
	PoissonIter     int
	PoissonResidual float64



	// Alpha holds the INTERIOR cell-centred volume fractions, row-major with
	// row 0 = top of the domain (j = Ny) for direct top-down display.
	// Values are clamped to [0,1].
	Alpha []float64

	// U and V hold interior cell-centred velocity components (averaged from
	// the MAC staggered faces), row 0 = top. Sizes Nx*Ny each. Server may
	// send them optionally (velocity overlay).
	U []float64
	V []float64
}

// Snapshot grabs a consistent, deep copy of the current state. It makes no
// attempt to lock: the solver is single-goroutine, so the server must only
// call it on the solver's own goroutine (it does).
func (s *Simulation) Snapshot() *Snapshot {
	g, f := s.Grid, s.Fields
	sn := &Snapshot{
		Nx: g.Nx, Ny: g.Ny,
		Dx: g.Dx, Dy: g.Dy,
		L0:     s.Cfg.Domain.L0,
		H0:     s.Cfg.Domain.H0,
		Width:  float64(g.Nx) * g.Dx,
		Height: float64(g.Ny) * g.Dy,

		Time:            s.State.Time,
		TStar:           s.State.TStar,
		Step:            s.State.Step,
		DT:              s.State.DT,
		CFL:             s.State.CFL,
		MaxDiv:          s.State.MaxDiv,
		Volume:          s.State.Volume + s.State.VolumeDrift,
		RefVolume:       s.State.Volume,
		PoissonIter:     s.State.PoissonIter,
		PoissonResidual: s.State.PoissonResidual,
		FrontX:          s.State.FrontX,
		FrontXStar:      s.State.FrontXStar,

	}
	if sn.RefVolume > 0 {
		sn.VolumeDriftPct = 100 * (sn.Volume - sn.RefVolume) / sn.RefVolume
	}

	// Deep-copy interior alpha (row 0 = top) clamped to [0,1].
	n := g.Nx * g.Ny
	sn.Alpha = make([]float64, n)
	k := 0
	for j := g.Ny; j >= 1; j-- {
		for i := 1; i <= g.Nx; i++ {
			a := f.Alpha[g.IdxCC(i, j)]
			if a < 0 {
				a = 0
			} else if a > 1 {
				a = 1
			}
			sn.Alpha[k] = a
			k++
		}
	}

	// Cell-centred velocities: average of the four surrounding MAC faces.
	sn.U = make([]float64, n)
	sn.V = make([]float64, n)
	k = 0
	for j := g.Ny; j >= 1; j-- {
		for i := 1; i <= g.Nx; i++ {
			sn.U[k] = 0.5 * (f.U[g.IdxU(i-1, j)] + f.U[g.IdxU(i, j)])
			sn.V[k] = 0.5 * (f.V[g.IdxV(i, j-1)] + f.V[g.IdxV(i, j)])
			k++
		}
	}
	return sn
}

