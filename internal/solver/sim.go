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
	Cfg   Config
	Grid  *Grid
	Fields *Fields
	State SimState

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

// Step advances one timestep; dt<0 means automatic (ComputeDt).
func (s *Simulation) Step(dtIn float64) error {
	g, f, cfg := s.Grid, s.Fields, &s.Cfg

	dt := dtIn
	if dt <= 0 {
		dt = ComputeDt(g, f, cfg, &s.State)
	}

	Predict(g, f, cfg, dt, s.muScratch)
	ApplyStarBC(f, cfg, g)
	res := Project(g, f, cfg, dt, s.scratch)
	ApplyVelocityBC(f, cfg, g)

	if !res.Converged {
		return fmt.Errorf("step %d: Poisson not converged (iters=%d relRes=%.3e)",
			s.State.Step+1, res.Iterations, res.Residual)
	}

	deltaX, deltaY, deltaClip, outflow, cflWarns := AdvectAlpha(g, f, cfg, dt, s.State.Step)
	s.State.VolSweepX += deltaX
	s.State.VolSweepY += deltaY
	s.State.VolClip += deltaClip
	s.State.ClippedMass += deltaClip // We'll keep ClippedMass tracking deltaClip as well just in case.
	s.State.TopOutflow += outflow
	s.State.CFLWarnings += cflWarns

	UpdateProperties(g, f, cfg)
	InterpolateRhoToFaces(g, f, cfg)

	s.State.Step++
	s.State.Time += dt
	s.State.DT = dt
	s.State.PoissonIter = res.Iterations
	s.State.PoissonResidual = res.Residual
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

	for j := 1; j <= g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ {
			idx := g.idxCC(i, j)
			vol += f.Alpha[idx]
		}
	}
	
	// Front position: sub-cell linear interpolation of the alpha = 0.5 crossing in the lowest row (j=1)
	for i := g.Nx; i >= 1; i-- {
		idx := g.idxCC(i, 1)
		a := f.Alpha[idx]
		if a > 0.5 {
			if i < g.Nx {
				aNext := f.Alpha[g.idxCC(i+1, 1)]
				if aNext <= 0.5 {
					// Interpolate between cell centers Xc[i] and Xc[i+1]
					t := (0.5 - a) / (aNext - a)
					frontX = g.Xc[idx] + t*g.Dx
				} else {
					frontX = g.Xc[idx] + 0.5*g.Dx
				}
			} else {
				frontX = g.Xc[idx] + 0.5*g.Dx
			}
			break
		}
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
