package solver

// Ghost-cell / MAC-layout conventions (authoritative reference for all kernels)
// ----------------------------------------------------------------------------
// Indexing: idx = j*NxG + i, with j=0 the bottom row and i=0 the left column.
//
// Cell-centred fields (Rho, Mu, P, Alpha, Div, ...): stored on the full padded
// array of size NxG*NyG.
//   interior : i in [1..Nx], j in [1..Ny]
//   ghost    : i==0 (left wall), i==Nx+1 (right wall), j==0 (floor), j==Ny+1 (top/open)
//   cell (i,j) centre at ((i-1/2)*dx, (j-1/2)*dy) in physical coords.
//
// U-faces (u stored on VERTICAL faces), array size (NxG+1)*NyG:
//   interior: i in [1..Nx-1]; i==0 and i==Nx are WALL faces (left/right walls).
//   index (i,j): face at x=i*dx, centred in y on cell row j.
//   Walls are REAL faces at the ends of each row: they are SET DIRECTLY by the
//   BCs (never via ghost cells). Their ghost-image partners u[0]/u[Nx] in the
//   padded row exist only so kernels can treat the padded layout uniformly.
//
// V-faces (v stored on HORIZONTAL faces), array size NxG*(NyG+1):
//   interior: j in [1..Ny-1]; j==0 is the FLOOR wall face, j==Ny is the TOP
//   boundary face (open: Dirichlet p=0, zero-gradient velocity).
//   The floor face is SET DIRECTLY (v=0), not through a ghost row.
//
// No-slip: tangential ghost velocity = -tangential interior (u[-1/2]=-u[1/2] etc.).
// Free-slip: tangential ghost = +tangential interior. Normal ghost = -normal
// interior on both conventions (true zero-flux).
// Pressure: Neumann at the three solid walls (zero face coefficient in the
// Poisson operator), Dirichlet p=0 at the open top row (cells j==Ny+1).

import "fmt"

type Grid struct {
	// PinnedCell, when non-nil, holds (i,j) of a cell whose pressure is held
	// at zero. Used by fully enclosed (cavity) runs where all-Neumann BCs
	// would otherwise make the Poisson operator singular.
	PinnedCell     [2]int
	hasPin         bool
	Nx, Ny         int
	NxG, NyG       int
	Dx, Dy         float64
	InvDx, InvDy   float64
	InvDx2, InvDy2 float64
	Xc             []float64
	Yc             []float64
	Xu             []float64
	Yu             []float64
	Xv             []float64
	Yv             []float64
}

func NewGrid(nx, ny int, width, height float64) *Grid {
	g := &Grid{}
	g.Nx = nx
	g.Ny = ny
	g.NxG = nx + 2
	g.NyG = ny + 2
	g.Dx = width / float64(nx)
	g.Dy = height / float64(ny)
	g.InvDx = 1.0 / g.Dx
	g.InvDy = 1.0 / g.Dy
	g.InvDx2 = g.InvDx * g.InvDx
	g.InvDy2 = g.InvDy * g.InvDy

	g.Xc = make([]float64, g.NxG*g.NyG)
	g.Yc = make([]float64, g.NxG*g.NyG)
	g.Xu = make([]float64, (g.NxG+1)*g.NyG)
	g.Yu = make([]float64, (g.NxG+1)*g.NyG)
	g.Xv = make([]float64, g.NxG*(g.NyG+1))
	g.Yv = make([]float64, g.NxG*(g.NyG+1))

	for j := 0; j < g.NyG; j++ {
		yc := (float64(j) - 0.5) * g.Dy
		for i := 0; i < g.NxG; i++ {
			xc := (float64(i) - 0.5) * g.Dx
			idx := g.idxCC(i, j)
			g.Xc[idx] = xc
			g.Yc[idx] = yc
		}
	}

	for j := 0; j < g.NyG; j++ {
		yu := (float64(j) - 0.5) * g.Dy
		for i := 0; i <= g.NxG; i++ {
			xu := float64(i) * g.Dx
			idx := g.idxU(i, j)
			g.Xu[idx] = xu
			g.Yu[idx] = yu
		}
	}

	for j := 0; j <= g.NyG; j++ {
		yv := float64(j) * g.Dy
		for i := 0; i < g.NxG; i++ {
			xv := (float64(i) - 0.5) * g.Dx
			idx := g.idxV(i, j)
			g.Xv[idx] = xv
			g.Yv[idx] = yv
		}
	}

	return g
}

func (g *Grid) idxCC(i, j int) int {
	if i < 0 || i >= g.NxG || j < 0 || j >= g.NyG {
		panic(fmt.Sprintf("CC index out of bounds: (%d,%d) grid=(%d,%d)", i, j, g.NxG, g.NyG))
	}
	return j*g.NxG + i
}

func (g *Grid) idxU(i, j int) int {
	if i < 0 || i > g.NxG || j < 0 || j >= g.NyG {
		panic(fmt.Sprintf("U index out of bounds: (%d,%d) grid=(%d,%d)", i, j, g.NxG+1, g.NyG))
	}
	return j*(g.NxG+1) + i
}

func (g *Grid) idxV(i, j int) int {
	if i < 0 || i >= g.NxG || j < 0 || j > g.NyG {
		panic(fmt.Sprintf("V index out of bounds: (%d,%d) grid=(%d,%d)", i, j, g.NxG, g.NyG+1))
	}
	return j*g.NxG + i
}

func (g *Grid) CellCount() int  { return g.Nx * g.Ny }
func (g *Grid) UFaceCount() int { return (g.Nx + 1) * g.Ny }
func (g *Grid) VFaceCount() int { return g.Nx * (g.Ny + 1) }
func (g *Grid) TotalCC() int    { return g.NxG * g.NyG }
func (g *Grid) TotalU() int     { return (g.NxG + 1) * g.NyG }
func (g *Grid) TotalV() int     { return g.NxG * (g.NyG + 1) }

func (g *Grid) IJCC(idx int) (int, int) {
	return idx % g.NxG, idx / g.NxG
}

func (g *Grid) IJU(idx int) (int, int) {
	return idx % (g.NxG + 1), idx / (g.NxG + 1)
}

func (g *Grid) IJV(idx int) (int, int) {
	return idx % g.NxG, idx / g.NxG
}

func (g *Grid) IsInteriorCC(i, j int) bool {
	return i >= 1 && i <= g.Nx && j >= 1 && j <= g.Ny
}

func (g *Grid) IsInteriorU(i, j int) bool {
	return i >= 1 && i <= g.Nx && j >= 1 && j <= g.Ny
}

func (g *Grid) IsInteriorV(i, j int) bool {
	return i >= 1 && i <= g.Nx && j >= 1 && j <= g.Ny
}

// --- helpers used by the BC, Poisson and projection kernels -----------------

// LeftWallU returns the linear index of the u-face ON the left wall in row j.
func (g *Grid) LeftWallU(j int) int { return g.idxU(0, j) }

// RightWallU returns the u-face ON the right wall in row j.
func (g *Grid) RightWallU(j int) int { return g.idxU(g.Nx, j) }

// FloorV returns the v-face ON the floor in column i.
func (g *Grid) FloorV(i int) int { return g.idxV(i, 0) }

// TopV returns the top-boundary v-face in column i (open boundary).
func (g *Grid) TopV(i int) int { return g.idxV(i, g.Ny) }

// WestVFace returns the vertical (u) face on the WEST side of cell (i,j).
// i runs 1..Nx+1 so that WestVFace(1,j) is the left wall face. Valid 1..Nx+1.
func (g *Grid) WestVFace(i, j int) int { return g.idxU(i-1, j) }

// EastVFace returns the u-face on the EAST side of cell (i,j). Valid 1..Nx+1.
func (g *Grid) EastVFace(i, j int) int { return g.idxU(i, j) }

// SouthHFace returns the horizontal (v) face on the SOUTH side of cell (i,j).
// j runs 1..Ny+1 so that SouthHFace(i,1) is the floor. Valid 1..Ny+1.
func (g *Grid) SouthHFace(i, j int) int { return g.idxV(i, j-1) }

// NorthHFace returns the v-face on the NORTH side of cell (i,j). Valid 1..Ny+1.
func (g *Grid) NorthHFace(i, j int) int { return g.idxV(i, j) }

// PinCell marks cell (i,j) as held at p=0 (enclosed-domain pressure reference).
func (g *Grid) PinCell(i, j int) {
	g.PinnedCell = [2]int{i, j}
	g.hasPin = true
}

// PinnedIdx returns the linear index of the pinned pressure cell, or -1.
func (g *Grid) PinnedIdx() int {
	if !g.hasPin {
		return -1
	}
	return g.idxCC(g.PinnedCell[0], g.PinnedCell[1])
}
func (g *Grid) IdxCC(i, j int) int { return g.idxCC(i, j) }

func (g *Grid) IdxU(i, j int) int { return g.idxU(i, j) }

func (g *Grid) IdxV(i, j int) int { return g.idxV(i, j) }
