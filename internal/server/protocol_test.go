package server

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"dambreak/internal/solver"
)

func testSnapshot(nx, ny int) *solver.Snapshot {
	cfg := solver.DefaultConfig()
	cfg.Domain.Nx = nx
	cfg.Domain.Ny = ny
	cfg.Domain.Width = 8 * cfg.Domain.L0
	cfg.Domain.Height = 4 * cfg.Domain.L0
	cfg.Domain.H0 = 2 * cfg.Domain.L0
	s := solver.NewSimulation(cfg, nx, ny, cfg.Domain.Width, cfg.Domain.Height, false)
	s.InitDamBreak()
	return s.Snapshot()
}

func TestFrameRoundTrip(t *testing.T) {
	snap := testSnapshot(32, 16)
	snap.Time = 1.234
	snap.TStar = 2.345
	snap.Step = 77
	snap.DT = 1e-5
	snap.CFL = 0.22
	snap.MaxDiv = 1e-7
	snap.FrontX = 0.123
	snap.FrontXStar = 2.15
	snap.PoissonIter = 42
	snap.PoissonResidual = 8.8e-9
	snap.Volume = 0.01
	snap.RefVolume = 0.01001
	snap.VolumeDriftPct = -0.1
	for k := range snap.Alpha {
		snap.Alpha[k] = float64(k%256) / 255.0
	}

	// Without velocity planes.
	buf := EncodeFrame(9, snap, false)
	f, err := DecodeFrame(buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if f.FrameID != 9 || f.Nx != 32 || f.Ny != 16 || f.Step != 77 {
		t.Fatalf("header mismatch: %+v", f)
	}
	if math.Abs(f.T-snap.Time) > 1e-12 || math.Abs(f.TStar-snap.TStar) > 1e-12 {
		t.Fatalf("t/t* mismatch: %v %v", f.T, f.TStar)
	}
	if f.U != nil || f.V != nil {
		t.Fatal("velocity planes present without flag")
	}
	for k := range snap.Alpha {
		want := uint8(snap.Alpha[k]*255.0 + 0.5)
		if f.Alpha[k] != want {
			t.Fatalf("alpha[%d]: got %d want %d", k, f.Alpha[k], want)
		}
	}
	if f.Diags[7] != snap.FrontXStar || f.Diags[8] != 42 {
		t.Fatalf("diag mismatch: %v", f.Diags)
	}

	// With velocity planes.
	buf2 := EncodeFrame(10, snap, true)
	f2, err := DecodeFrame(buf2)
	if err != nil {
		t.Fatalf("decode vel: %v", err)
	}
	if f2.U == nil || f2.V == nil || len(f2.U) != 32*16 || len(f2.V) != 32*16 {
		t.Fatalf("velocity planes missing or wrong size: %d %d", len(f2.U), len(f2.V))
	}
	for k := 0; k < len(f2.U); k++ {
		if math.Abs(float64(f2.U[k]-float32(snap.U[k]))) > 1e-6 {
			t.Fatalf("u[%d] mismatch: %v vs %v", k, f2.U[k], snap.U[k])
		}
		break
	}
	if !bytes.Contains(buf2, []byte{}) {
		t.Fatal("unreachable") // placate bytes import if trimmed
	}
}

func TestDecodeBadFrames(t *testing.T) {
	snap := testSnapshot(8, 8)
	buf := EncodeFrame(1, snap, false)

	// Short buffer.
	if _, err := DecodeFrame(buf[:10]); !errors.Is(err, ErrShortFrame) {
		t.Fatalf("want ErrShortFrame, got %v", err)
	}
	// Bad magic.
	bad := append([]byte(nil), buf...)
	bad[0] = 'X'
	if _, err := DecodeFrame(bad); !errors.Is(err, ErrBadMagic) {
		t.Fatalf("want ErrBadMagic, got %v", err)
	}
	// Bad version.
	bad = append([]byte(nil), buf...)
	bad[4] = 99
	if _, err := DecodeFrame(bad); !errors.Is(err, ErrBadVersion) {
		t.Fatalf("want ErrBadVersion, got %v", err)
	}
	// Truncated payload.
	if _, err := DecodeFrame(buf[:len(buf)-5]); !errors.Is(err, ErrBadSize) {
		t.Fatalf("want ErrBadSize, got %v", err)
	}
}

func TestFront99InitialColumn(t *testing.T) {
	// Fresh dam break: 99% of the fluid sits inside the initial column, so the
	// 99% front must land at (or just inside) the column edge x = L0.
	cfg := solver.DefaultConfig()
	cfg.Domain.Nx = 64
	cfg.Domain.Ny = 32
	cfg.Domain.Width = 8 * cfg.Domain.L0
	cfg.Domain.Height = 4 * cfg.Domain.L0
	s := solver.NewSimulation(cfg, 64, 32, cfg.Domain.Width, cfg.Domain.Height, false)
	s.InitDamBreak()
	fx := front99(s)
	L0 := cfg.Domain.L0
	if fx < 0.95*L0 || fx > 1.0*L0 {
		t.Fatalf("front99 = %v m, want within [0.95,1.0] L0 (%v)", fx, L0)
	}
}
