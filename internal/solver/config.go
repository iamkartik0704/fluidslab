package solver

type BCMode int

const (
	BCNoSlip BCMode = iota
	BCFreeSlip
)

type AdvectScheme int

const (
	AdvectUpwind AdvectScheme = iota
	AdvectDonorAcceptor
)

type PoissonSolverType int

const (
	PoissonPCG PoissonSolverType = iota
	PoissonSOR
)

type TimeScaleConvention int

const (
	TimeScaleSqrt2gOverL0 TimeScaleConvention = iota
	TimeScaleSqrtgOverL0
)

type XStarConvention int

const (
	XStarFrontXOverL0 XStarConvention = iota
)

type Config struct {
	Domain struct {
		L0          float64
		H0          float64
		Width       float64
		Height      float64
		Nx          int
		Ny          int
	}
	Physical struct {
		RhoW        float64
		RhoA        float64
		MuW         float64
		MuA         float64
		Gravity     float64
		Sigma       float64 // CSF surface tension coefficient; RESERVED, not yet implemented
		DensityRamp struct {
			Enabled   bool
			StartRatio float64
			TargetRatio float64
			RampSteps  int
		}
	}
	Numerical struct {
		CFL              float64
		ViscousCFL       float64
		GravityCFL       float64
		MaxDT            float64
		DTGrowthFactor   float64
		PoissonTol       float64
		PoissonMaxIter   int
		Preconditioner   string
		PoissonSolver    PoissonSolverType
		AdvectScheme     AdvectScheme
		FaceRhoAvg       string
		FreeSlip         bool
		OpenTop          bool    // true: open boundary (p=0, zero-gradient); false: solid lid for cavity runs
		LidVelocity      float64 // tangential velocity of the closed top wall (cavity benchmark)
		ClipAlpha        bool
		DilatationCorr   bool    // apply VOF dilatation correction
		SplitDivFix      bool    // fix split-sweep non-conservation
		SecondOrderAdvect bool   // use QUICK/van Leer for momentum
		RemoveFlotsam    bool
		FlotsamThreshold float64
		SkipAdvection    bool
		SkipViscosity    bool
		ClipRedistribute bool
	}
	TimeScale TimeScaleConvention
	XStarDef  XStarConvention
	// CSF surface tension: coefficient kept in Physical.Sigma but the model is
	// DISABLED (SigmaActive = 0) until everything else passes validation.
	SigmaActive float64
	Threads   int
	DisplayHz int
}

func DefaultConfig() Config {
	c := Config{}
	c.Domain.L0 = 0.05715
	c.Domain.H0 = 2.0 * c.Domain.L0
	c.Domain.Width = 4.0 * c.Domain.L0
	c.Domain.Height = 3.0 * c.Domain.H0
	c.Domain.Nx = 128
	c.Domain.Ny = 192

	c.Physical.RhoW = 998.0
	c.Physical.RhoA = 1.2
	c.Physical.MuW = 1.0e-3
	c.Physical.MuA = 1.8e-5
	c.Physical.Gravity = 9.81
	c.Physical.		Sigma = 0.072 // kept for later; CSF defaults OFF
	c.Physical.DensityRamp.Enabled = false
	c.Physical.DensityRamp.StartRatio = 100.0
	c.Physical.DensityRamp.TargetRatio = 998.0 / 1.2
	c.Physical.DensityRamp.RampSteps = 500

	c.Numerical.CFL = 0.25
	c.Numerical.ViscousCFL = 0.25
	c.Numerical.GravityCFL = 0.5
	c.Numerical.MaxDT = 1.0e-3
	c.Numerical.DTGrowthFactor = 1.1
	c.Numerical.PoissonTol = 1e-8
	c.Numerical.PoissonMaxIter = 5000
	c.Numerical.Preconditioner = "jacobi"
	c.Numerical.PoissonSolver = PoissonPCG
	c.Numerical.AdvectScheme = AdvectDonorAcceptor
	c.Numerical.FaceRhoAvg = "arithmetic"
	c.Numerical.FreeSlip = false
	c.Numerical.OpenTop = true
	c.Numerical.LidVelocity = 0
	c.Numerical.ClipAlpha = true
	c.Numerical.DilatationCorr = true
	c.Numerical.SplitDivFix = true
	c.Numerical.SecondOrderAdvect = false
	c.Numerical.ClipRedistribute = true
	c.Numerical.RemoveFlotsam = false
	c.Numerical.FlotsamThreshold = 1e-6

	c.TimeScale = TimeScaleSqrt2gOverL0
	c.Threads = 1
	c.DisplayHz = 30

	// CSF surface tension is deliberately OFF: fields are kept for the future
	// milestone but no capillary terms enter the momentum equation yet.
	c.SigmaActive = 0.0

	return c
}

type SimState struct {
	Time       float64
	Step       int
	DT         float64
	CFL        float64
	Volume     float64
	VolumeDrift float64
	FrontX      float64
	FrontXStar  float64
	FrontXStar01 float64
	FrontXStar001 float64
	ResidualHStar float64
	TStar       float64
	PoissonIter int
	PoissonResidual float64
	MaxDiv     float64
	KineticEnergy float64
	ClippedMass  float64
	VolSweepX    float64
	VolSweepY    float64
	VolClip      float64
	TopOutflow   float64
	CFLWarnings  int
	MaxVelInterface float64
	WeberNumber float64
}

type ControlCmd struct {
	Cmd   string
	Params map[string]float64
}

type FrameData struct {
	Nx, Ny       int
	Time         float64
	Step         int
	Alpha        []uint8
	U            []float32
	V            []float32
	Diagnostics  SimState
}