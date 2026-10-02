package solver

import (
	"fmt"
	"math"
)

// Simulation owns the state and runs the projection-method time loop:
// props -> dt -> BC -> predictor -> star BC -> Poisson -> projection -> BC
// -> diagnostics. Alpha advection is a no-op at this milestone (alpha is
// never advanced: single-fluid runs keep alpha constant).
type Simulation struct {
	Cfg    Config
	Grid   *Grid
	Fields *Fields
	State  SimState

	scratch   [][]float64 // 8 arrays of TotalCC: z, r, s, Ap, diag, Ax, Ay, b
	muScratch [][]float64 // muU, muV
}

// NewSimulation wires up a Simulation; enclosed=true pins a pressure cell.
func NewSimulation(cfg Config, nx, ny int, width, height float64, enclosed bool) *Simulation {
	s := &Simulation{Cfg: cfg}
	s.Grid = NewGrid(nx, ny, width, height)
	s.Fields = NewFields(s.Grid)
	if enclosed {
		// Pin near the centre: a centred reference cell conditions far better
		// than a corner for the all-Neumann operator.
		s.Grid.PinCell(1+nx/2, 1+ny/2)
	}
	s.scratch = allocScratch8(s.Grid)
	s.muScratch = allocMuScratch(s.Grid)

	// Serial Poisson is faster than goroutine-parallel below ~25k cells
	// (measured: 2x faster at 64×64, 1.24x at 128×64).
	if nx*ny <= 25000 {
		ForceSerialPoisson = true
	}

	// Uniform alpha=1 (single-phase) unless the caller sets an initial state.
	for idx := range s.Fields.Alpha {
		s.Fields.Alpha[idx] = 1
	}
	UpdateProperties(s.Grid, s.Fields, &s.Cfg)
	InterpolateRhoToFaces(s.Grid, s.Fields, &s.Cfg)
	ApplyVelocityBC(s.Fields, &s.Cfg, s.Grid)
	return s
}

func (s *Simulation) Step(dtIn float64) error {
	g, f, cfg := s.Grid, s.Fields, &s.Cfg

	dt := dtIn
	if dt <= 0 {
		dt = ComputeDt(g, f, cfg, &s.State)
	}

	hmin := math.Min(g.Dx, g.Dy)
	umax := f.MaxAbsVel(g)
	if umax > 0 {
		advCFL := umax * dt / hmin
		s.State.MaxAdvCFL = math.Max(s.State.MaxAdvCFL, advCFL)
		s.State.SumAdvCFL += advCFL
		s.State.AdvCFLCount++
	}

	nMom := cfg.Numerical.SubstepMom
	if nMom <= 0 {
		nMom = 1
	}
	nVOF := cfg.Numerical.SubstepVOF
	if nVOF <= 0 {
		nVOF = 1
	}

	dtMom := dt / float64(nMom)
	dtVOF := dt / float64(nVOF)

	iterSum := 0
	resLast := 0.0
	for i := 0; i < nMom; i++ {
		Predict(g, f, cfg, dtMom, s.muScratch)
		ApplyStarBC(f, cfg, g)
		res := Project(g, f, cfg, dtMom, s.scratch)
		ApplyVelocityBC(f, cfg, g)
		if !res.Converged {
			return fmt.Errorf("step %d (substep %d): Poisson not converged (iters=%d relRes=%.3e)",
				s.State.Step+1, i+1, res.Iterations, res.Residual)
		}
		iterSum += res.Iterations
		resLast = res.Residual
	}

	cflWarns := 0
	for i := 0; i < nVOF; i++ {
		deltaX, deltaY, deltaClip, outflow, warns := AdvectAlpha(g, f, cfg, dtVOF, s.State.Step)
		s.State.VolSweepX += deltaX
		s.State.VolSweepY += deltaY
		s.State.VolClip += deltaClip
		s.State.ClippedMass += deltaClip
		s.State.TopOutflow += outflow
		cflWarns += warns
	}
	s.State.CFLWarnings += cflWarns

	UpdateProperties(g, f, cfg)
	InterpolateRhoToFaces(g, f, cfg)

	s.State.Step++
	s.State.Time += dt
	s.State.DT = dt
	s.State.PoissonIter = iterSum
	s.State.PoissonResidual = resLast
	s.State.MaxDiv = MaxAbsDiv(g, f)
	s.UpdateDiagnostics()
	return nil
}

// Run advances n steps (dt<0: automatic) and returns the last error, if any.
func (s *Simulation) Run(n int, dtIn float64) error {
	var err error
	for k := 0; k < n; k++ {
		if err = s.Step(dtIn); err != nil {
			return err
		}
	}
	return nil
}

// UpdateDiagnostics computes volume conservation and front position.
func (s *Simulation) UpdateDiagnostics() {
	g := s.Grid
	f := s.Fields
	vol := 0.0
	frontX := 0.0
	frontX01 := 0.0
	frontX001 := 0.0

	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			vol += f.Alpha[idx]
		}
	}

	findCrossing := func(threshold float64) float64 {
		for i := g.Nx; i >= 1; i-- {
			idx := g.idxCC(i, 1)
			a := f.Alpha[idx]
			if a > threshold {
				if i < g.Nx {
					aNext := f.Alpha[g.idxCC(i+1, 1)]
					if aNext <= threshold {
						t := (threshold - a) / (aNext - a)
						return g.Xc[idx] + t*g.Dx
					} else {
						return g.Xc[idx] + 0.5*g.Dx
					}
				} else {
					return g.Xc[idx] + 0.5*g.Dx
				}
			}
		}
		return g.Xc[g.idxCC(1, 1)] - 0.5*g.Dx
	}

	frontX = findCrossing(0.5)
	frontX01 = findCrossing(0.1)
	frontX001 = findCrossing(0.01)

	// Residual-height diagnostic: H = (alpha = 0.5 surface height in the column next to the left wall, sub-cell interpolated) / H0
	// i=1 is the column next to the left wall
	residualH := 0.0
	for j := g.Ny; j >= 1; j-- {
		idx := g.idxCC(1, j)
		a := f.Alpha[idx]
		if a > 0.5 {
			if j < g.Ny {
				aNext := f.Alpha[g.idxCC(1, j+1)]
				if aNext <= 0.5 {
					t := (0.5 - a) / (aNext - a)
					residualH = g.Yc[idx] + t*g.Dy
				} else {
					residualH = g.Yc[idx] + 0.5*g.Dy
				}
			} else {
				residualH = g.Yc[idx] + 0.5*g.Dy
			}
			break
		}
	}
	if residualH == 0.0 {
		residualH = g.Yc[g.idxCC(1, 1)] - 0.5*g.Dy
	}

	vol *= g.Dx * g.Dy
	if s.State.Volume == 0 {
		s.State.Volume = vol // initial volume
	}
	s.State.VolumeDrift = vol - s.State.Volume
	s.State.FrontX = frontX

	// Dimensionless terms
	L0 := s.Cfg.Domain.L0
	gConst := s.Cfg.Physical.Gravity

	s.State.FrontXStar = frontX / L0
	s.State.FrontXStar01 = frontX01 / L0
	s.State.FrontXStar001 = frontX001 / L0
	s.State.ResidualHStar = residualH / s.Cfg.Domain.H0

	// Invariant check: X* should never be less than 1.0 (the column's initial right edge)
	// Allow a tiny tolerance for numerical smearing/rounding at t=0.
	if s.State.FrontXStar < 0.99 && s.State.Step > 0 {
		// disabled for cavity
	}
	// t* = t * sqrt(2g / L0) (from the PDF: t* = t * sqrt(2g/L0) for time scale)
	// Martin & Moyce used sqrt(2g / L0) or sqrt(g / L0). PDF says sqrt(2g / L0).
	if s.Cfg.TimeScale == TimeScaleSqrt2gOverL0 {
		s.State.TStar = s.State.Time * math.Sqrt(2.0*gConst/L0)
	} else {
		s.State.TStar = s.State.Time * math.Sqrt(gConst/L0)
	}
}

// InitDamBreak sets the initial Alpha field for a dam break (water column H0xL0).
func (s *Simulation) InitDamBreak() {
	g := s.Grid
	f := s.Fields
	cfg := s.Cfg
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			x := g.Xc[g.idxCC(i, j)]
			y := g.Yc[g.idxCC(i, j)]
			if x <= cfg.Domain.L0 && y <= cfg.Domain.H0 {
				f.Alpha[g.idxCC(i, j)] = 1.0
			} else {
				f.Alpha[g.idxCC(i, j)] = 0.0
			}
		}
	}
	UpdateProperties(g, f, &s.Cfg)
	InterpolateRhoToFaces(g, f, &s.Cfg)
	s.UpdateDiagnostics()
}
