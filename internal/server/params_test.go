package server

import "testing"

func TestClampParams(t *testing.T) {
	in := ServerParams{
		AspectRatio:    9.0,  // > 4 -> 4
		CellsPerL0:     13,   // -> 16
		ViscosityScale: 0.0,  // -> 0.01
		DensityRatio:   -5,   // -> 1
		FreeSlip:       true, // stays true in params; UI/server keeps it off by policy
		Scheme:         "bogus",

	}
	got := in.Clamp()
	if got.AspectRatio != 4.0 {
		t.Errorf("aspect: got %v", got.AspectRatio)
	}
	if got.CellsPerL0 != 16 {
		t.Errorf("cells: got %v", got.CellsPerL0)
	}
	if got.ViscosityScale != 0.01 {
		t.Errorf("visc: got %v", got.ViscosityScale)
	}
	if got.DensityRatio != 1 {
		t.Errorf("density: got %v", got.DensityRatio)
	}
	if got.Scheme != "donor-acceptor" {
		t.Errorf("scheme: got %q", got.Scheme)
	}


	// Cells ladder snap: <12->8, <20->16, <28->24, else 32.
	for _, tc := range []struct{ in, want int }{{7, 8}, {8, 8}, {12, 16}, {19, 16}, {20, 24}, {27, 24}, {28, 32}, {100, 32}} {
		got := ServerParams{CellsPerL0: tc.in}.Clamp()
		if got.CellsPerL0 != tc.want {
			t.Errorf("cells %d -> %d, want %d", tc.in, got.CellsPerL0, tc.want)
		}
	}
}

func TestOutOfRange(t *testing.T) {
	ok := DefaultParams()
	if bad := ok.OutOfRange(); len(bad) != 0 {
		t.Errorf("defaults flagged: %v", bad)
	}
	bad := ServerParams{AspectRatio: 0.5, CellsPerL0: 3, ViscosityScale: 1e9, DensityRatio: 0}
	got := bad.OutOfRange()
	if len(got) != 4 {
		t.Errorf("want 4 flags, got %v", got)
	}
}

func TestReinitNeeded(t *testing.T) {
	a := DefaultParams()
	b := DefaultParams()
	b.ViscosityScale = 2.0
	if !reinitNeeded(a, b) {
		t.Error("viscosity change must trigger re-init")
	}
	if reinitNeeded(a, a) {
		t.Error("identical params must not trigger re-init")
	}
}
