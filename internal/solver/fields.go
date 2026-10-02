package solver

import "math"

type Fields struct {
	Alpha      []float64
	Rho        []float64
	Mu         []float64
	P          []float64
	U          []float64
	V          []float64
	UStar      []float64
	VStar      []float64
	RhoU       []float64
	RhoV       []float64
	DivU       []float64
	GradAlphaX []float64
	GradAlphaY []float64
	AlphaPrev  []float64

	FluxX       []float64
	FluxY       []float64
	CFLWarnings int
	TopOutflow  float64
}

func NewFields(g *Grid) *Fields {
	f := &Fields{}
	f.Alpha = make([]float64, g.TotalCC())
	f.Rho = make([]float64, g.TotalCC())
	f.Mu = make([]float64, g.TotalCC())
	f.P = make([]float64, g.TotalCC())
	f.U = make([]float64, g.TotalU())
	f.V = make([]float64, g.TotalV())
	f.UStar = make([]float64, g.TotalU())
	f.VStar = make([]float64, g.TotalV())
	f.RhoU = make([]float64, g.TotalU())
	f.RhoV = make([]float64, g.TotalV())
	f.DivU = make([]float64, g.TotalCC())
	f.GradAlphaX = make([]float64, g.TotalCC())
	f.GradAlphaY = make([]float64, g.TotalCC())
	f.AlphaPrev = make([]float64, g.TotalCC())
	f.FluxX = make([]float64, g.NxG+1)
	f.FluxY = make([]float64, g.NyG+1)
	return f
}

func (f *Fields) ResetVelocity() {
	for i := range f.U {
		f.U[i] = 0
		f.V[i] = 0
		f.UStar[i] = 0
		f.VStar[i] = 0
	}
	for i := range f.P {
		f.P[i] = 0
	}
}

func (f *Fields) CopyAlphaToPrev() {
	copy(f.AlphaPrev, f.Alpha)
}

func (f *Fields) SwapVelocity() {
	f.U, f.UStar = f.UStar, f.U
	f.V, f.VStar = f.VStar, f.V
}

// MaxAbsVel returns the max absolute velocity over INTERIOR u- and v-faces,
// used for CFL control.
func (f *Fields) MaxAbsVel(g *Grid) float64 {
	umax := 0.0
	for j := 1; j <= g.Ny; j++ {
		for i := 1; i < g.Nx; i++ { // wall faces i==0,i==Nx are always 0
			if a := math.Abs(f.U[g.idxU(i, j)]); a > umax {
				umax = a
			}
		}
	}
	vmax := 0.0
	for j := 1; j < g.Ny; j++ {
		for i := 1; i <= g.Nx; i++ { // floor/top faces j==0,j==Ny handled by BCs
			if a := math.Abs(f.V[g.idxV(i, j)]); a > vmax {
				vmax = a
			}
		}
	}
	if vmax > umax {
		return vmax
	}
	return umax
}
