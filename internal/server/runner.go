package server

import (
	"fmt"
	"sync"

	"dambreak/internal/solver"
)

// Runner owns the Simulation and its goroutine. All solver access happens on
// the runner goroutine; outsiders talk to it exclusively via the channels
// below, so no locking around solver state is needed.
type Runner struct {
	cmdCh    chan runnerCmd
	frameCh  chan *solver.Snapshot // latest-wins frame pipe (1-slot)
	statusCh chan *StatusMessage

	mu         sync.Mutex // guards the fields below
	archived   []SeriesRun
	nextRunID  int
	liveTrace  []TracePoint
	lastErr    string
	autoPaused bool
	running    bool
	timeScale  string
}

type runnerCmd struct {
	kind     string // "play", "pause", "reset", "step", "setParams", "setFrontDef", "quit"
	params   *ServerParams
	frontDef string
}

// FrontDef99 is the selector value for the 99%-cumulative front definition.
const FrontDef99 = "0.99"

// FrontDefHalf is the selector value for the alpha=0.5-crossing definition.
const FrontDefHalf = "0.5"

// AutoPauseTStar stops the run automatically past this t*: the known solver
// issue keeps the simulation unstable beyond the validated window, so the
// server refuses to run past it until that issue is fixed.
const AutoPauseTStar = 4.0

// sampleEvery is the upper bound on solver steps between emitted frames; the
// 30 Hz hub re-clocks whatever lands in the frame pipe. Lowered for smoother updates on slow CPUs.
const sampleEvery = 1

// NewRunner builds a Runner and starts its goroutine with the given params.
func NewRunner(params ServerParams) *Runner {
	r := &Runner{
		cmdCh:     make(chan runnerCmd, 16),
		frameCh:   make(chan *solver.Snapshot, 1),
		statusCh:  make(chan *StatusMessage, 1),
		nextRunID: 1,
	}
	go r.loop(params.Clamp())
	return r
}

// CmdCh exposes the control channel for the hub.
func (r *Runner) CmdCh() chan<- runnerCmd { return r.cmdCh }

// Frames yields the latest snapshot (already drained of stale ones).
func (r *Runner) Frames() <-chan *solver.Snapshot { return r.frameCh }

// PublishStatus hands a status update to the hub (non-blocking).
func (r *Runner) PublishStatus(st *StatusMessage) {
	select {
	case r.statusCh <- st:
	default:
	}
}

// Statuses yields status updates published by the runner.
func (r *Runner) Statuses() <-chan *StatusMessage { return r.statusCh }

// Quit stops the runner goroutine.
func (r *Runner) Quit() {
	select {
	case r.cmdCh <- runnerCmd{kind: "quit"}:
	default:
	}
}

// ArchivedRuns returns a copy of the archived (completed) runs.
func (r *Runner) ArchivedRuns() []SeriesRun {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]SeriesRun, len(r.archived))
	copy(out, r.archived)
	return out
}

// LiveTrace returns the in-progress run's trace (copy).
func (r *Runner) LiveTrace() []TracePoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]TracePoint, len(r.liveTrace))
	copy(out, r.liveTrace)
	return out
}

// LastError returns the most recent solver error string, if any.
func (r *Runner) LastError() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

// AutoPaused reports whether the auto-pause safety engaged.
func (r *Runner) AutoPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.autoPaused
}

// LastRunning reports whether the solver is (or was last) advancing.
func (r *Runner) LastRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// CurrentTimeScale returns the configured time-scaling convention label.
func (r *Runner) CurrentTimeScale() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.timeScale
}

func (r *Runner) loop(params ServerParams) {
	sim := buildSim(params)
	var pending []runnerCmd
	running := false
	simOk := true
	sampling := 0
	setRunning := func(v bool) {
		running = v
		r.mu.Lock()
		r.running = v
		r.mu.Unlock()
	}
	setTimeScale := func(ts string) {
		r.mu.Lock()
		r.timeScale = ts
		r.mu.Unlock()
	}
	setTimeScale(params.TimeScale)

	// nextCmd returns the next command, preferring locally queued ones.
	nextCmd := func() (runnerCmd, bool) {
		if len(pending) > 0 {
			c := pending[0]
			pending = pending[1:]
			return c, true
		}
		c, ok := <-r.cmdCh
		return c, ok
	}
	// stashPending pulls any immediately-available command into the queue and
	// reports whether it found one.
	stashPending := func() bool {
		select {
		case c := <-r.cmdCh:
			pending = append(pending, c)
			return true
		default:
			return false
		}
	}
	setErr := func(err error) {
		r.mu.Lock()
		r.lastErr = err.Error()
		r.mu.Unlock()
		setRunning(false)
		simOk = false
		r.PublishStatus(&StatusMessage{Type: "status", Running: false, LastErr: err.Error()})
	}
	// archiveRun stores the live trace (if any) as a comparison run.
	archiveRun := func(label string) {
		r.mu.Lock()
		if len(r.liveTrace) > 1 {
			r.archived = append(r.archived, SeriesRun{
				ID:     r.nextRunID,
				Label:  label,
				Points: append([]TracePoint(nil), r.liveTrace...),
				Params: params,
			})
			r.nextRunID++
		}
		r.liveTrace = nil
		r.mu.Unlock()
	}
	// clearTransient resets the error/auto-pause state after a rebuild.
	clearTransient := func() {
		r.mu.Lock()
		r.lastErr = ""
		r.autoPaused = false
		r.mu.Unlock()
	}

	publishFrame := func() {
		snap := sim.Snapshot()
		tp := TracePoint{T: snap.TStar, X05: snap.FrontXStar}
		tp.X99 = front99(sim) / sim.Cfg.Domain.L0
		r.mu.Lock()
		if len(r.liveTrace) < 4096 {
			r.liveTrace = append(r.liveTrace, tp)
		}
		r.mu.Unlock()
		// Latest-wins: evict a stale frame if the hub has not caught up.
		select {
		case r.frameCh <- snap:
		default:
			select {
			case <-r.frameCh:
			default:
			}
			select {
			case r.frameCh <- snap:
			default:
			}
		}
	}

	// Publish the initial state so a freshly started server (and any client
	// connecting before the first command) sees the intact column instead of
	// an empty canvas. The 1-slot frame pipe holds it until the hub pumps.
	publishFrame()

	for {
		cmd, ok := nextCmd()
		if !ok {
			return
		}
		switch cmd.kind {
		case "quit":
			return

		case "play":
			if simOk {
				setRunning(true)
				r.mu.Lock()
				r.autoPaused = false
				r.mu.Unlock()
			}

		case "pause":
			setRunning(false)

		case "step":
			if simOk {
				if err := sim.Step(-1); err != nil {
					setErr(err)
				} else {
					publishFrame()
				}
			}

		case "setFrontDef":
			// Display-only choice; the solver front (0.5 crossing) is always
			// reported, the 99% curve is computed alongside it.
			publishFrame() // Push frame so new clients get the initial state on connect

		case "setParams":
			if cmd.params != nil {
				newP := cmd.params.Clamp()
				needReset := reinitNeeded(params, newP)
				params = newP
				if needReset {
					archiveRun(fmt.Sprintf("run %d (restarted)", r.nextRunID))
					sim = buildSim(params)
					simOk = true
					setRunning(false)
					clearTransient()
					publishFrame()
				}
			}

		case "reset":
			archiveRun(fmt.Sprintf("run %d (stopped)", r.nextRunID))
			sim = buildSim(params)
			simOk = true
			setRunning(false)
			clearTransient()
			publishFrame()
		}

		// Advance while playing. Frames are sampled at a fixed sub-rate so the
		// display pipe stays decoupled from the solver step rate; commands are
		// checked between every step for responsiveness.
		for running && simOk {
			r.mu.Lock()
			autoPaused := r.autoPaused
			r.mu.Unlock()
			if autoPaused || sim.State.TStar >= AutoPauseTStar {
				r.mu.Lock()
				r.autoPaused = true
				r.mu.Unlock()
				archiveRun(fmt.Sprintf("run %d", r.nextRunID))
				setRunning(false)
				r.PublishStatus(&StatusMessage{Type: "status", Running: false, AutoPaused: true})
				break
			}
			if err := sim.Step(-1); err != nil {
				setErr(err)
				break
			}
			sampling++
			if sampling >= sampleEvery {
				sampling = 0
				publishFrame()
			}
			if stashPending() {
				setRunning(false) // handle the queued command on the next outer pass
			}
		}
	}
}

// buildSim constructs a fresh Simulation from UI parameters, mapping the UI
// scheme selector onto the solver's knobs.
func buildSim(p ServerParams) *solver.Simulation {
	cfg := solver.DefaultConfig()

	L0 := cfg.Domain.L0
	cfg.Domain.H0 = p.AspectRatio * L0
	cfg.Domain.Width = 8 * L0
	cfg.Domain.Height = 4 * L0

	cells := p.CellsPerL0
	cfg.Domain.Nx = 8 * cells
	cfg.Domain.Ny = 4 * cells

	cfg.Physical.MuW *= p.ViscosityScale
	cfg.Physical.RhoW = 998.0
	cfg.Physical.RhoA = 998.0 / p.DensityRatio

	switch p.Scheme {
	case "first-order":
		cfg.Numerical.AdvectScheme = solver.AdvectUpwind
		cfg.Numerical.SecondOrderAdvect = false
	case "van leer":
		cfg.Numerical.AdvectScheme = solver.AdvectDonorAcceptor
		cfg.Numerical.SecondOrderAdvect = true
	default: // "donor-acceptor"
		cfg.Numerical.AdvectScheme = solver.AdvectDonorAcceptor
		cfg.Numerical.SecondOrderAdvect = false
	}

	switch p.TimeScale {
	case "sqrt(g/L0)":
		cfg.TimeScale = solver.TimeScaleSqrtgOverL0
	default:
		cfg.TimeScale = solver.TimeScaleSqrt2gOverL0
	}

	cfg.Numerical.FreeSlip = p.FreeSlip
	cfg.Numerical.OpenTop = true
	cfg.Numerical.PoissonTol = 1e-6
	cfg.Threads = 1

	width := cfg.Domain.Width
	height := cfg.Domain.Height
	s := solver.NewSimulation(cfg, cfg.Domain.Nx, cfg.Domain.Ny, width, height, false)
	s.InitDamBreak()
	solver.ApplyAlphaBC(s.Grid, s.Fields)
	solver.UpdateProperties(s.Grid, s.Fields, &cfg)
	solver.InterpolateRhoToFaces(s.Grid, s.Fields, &cfg)
	s.UpdateDiagnostics()
	return s
}

// reinitNeeded reports whether a parameter change requires rebuilding the
// simulation (geometry, resolution, physics) rather than nothing at all.
// Every supported parameter needs re-initialisation; a change that does not
// (none currently) would trigger a clean reset anyway per the spec.
func reinitNeeded(a, b ServerParams) bool {
	return a != b
}

// front99 computes the 99%-cumulative-alpha front position: the largest x at
// which the cumulative fluid area to the LEFT reaches 99% of the total.
// Returns metres; 0 when the domain is dry.
func front99(s *solver.Simulation) float64 {
	g, f := s.Grid, s.Fields
	clamp := func(a float64) float64 {
		if a > 1 {
			return 1
		}
		if a < 0 {
			return 0
		}
		return a
	}
	total := 0.0
	colSum := make([]float64, g.Nx+1)
	for i := 1; i <= g.Nx; i++ {
		sum := 0.0
		for j := 1; j <= g.Ny; j++ {
			sum += clamp(f.Alpha[g.IdxCC(i, j)])
		}
		colSum[i] = sum * g.Dy
		total += colSum[i]
	}
	if total <= 0 {
		return 0
	}
	target := 0.99 * total
	cum := 0.0
	for i := 1; i <= g.Nx; i++ {
		if cum+colSum[i] >= target {
			// Crossing column i: walk rows bottom-up accumulating alpha.
			need := (target - cum) / g.Dy // in alpha-units within this column
			acc := 0.0
			for j := 1; j <= g.Ny; j++ {
				a := clamp(f.Alpha[g.IdxCC(i, j)])
				if acc+a >= need {
					frac := 0.0
					if a > 0 {
						frac = (need - acc) / a
					}
					return (float64(i-1) + frac) * g.Dx
				}
				acc += a
			}
			return float64(i) * g.Dx
		}
		cum += colSum[i]
	}
	return float64(g.Nx) * g.Dx
}
