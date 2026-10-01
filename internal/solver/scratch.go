package solver

// allocScratch allocates the eight PCG work arrays [z, r, s, Ap, diag, Ax, Ay, invE].
func allocScratch(g *Grid) [][]float64 {
	n := g.TotalCC()
	return [][]float64{
		make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n),
		make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n),
	}
}

// allocScratch8 allocates the nine TotalCC scratch arrays used by the
// projection: PCG arrays + [b]. Simulation allocates once and reuses.
func allocScratch8(g *Grid) [][]float64 {
	n := g.TotalCC()
	s := allocScratch(g)
	s = append(s, make([]float64, n)) // b
	return s
}

// allocMuScratch allocates the predictor's face-viscosity scratch [muU, muV].
func allocMuScratch(g *Grid) [][]float64 {
	return [][]float64{
		make([]float64, g.TotalU()),
		make([]float64, g.TotalV()),
	}
}
