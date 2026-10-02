package server

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"dambreak/internal/solver"
)

// Wire protocol
// -------------
// Binary frame (little-endian):
//   [0:4)   magic "DBRK"
//   [4]     version (=1)
//   [5]     flags: bit0 = velocity overlay (u,v float32 planes appended)
//   [6:10)  frame id (uint32)
//   [10:14) Nx (uint32)
//   [14:18) Ny (uint32)
//   [18:26) step (int64)
//   [26:34) t (float64)
//   [34:42) t* (float64)
//   [42:122) 10 diagnostics float64, in order:
//           dt, cfl, volume, refVolume, drift%, maxDiv,
//           frontX, frontX*, poissonIters, poissonResidual
//   [122:)  alpha as uint8 [0..255], Nx*Ny, row 0 = top row
//   [...:)  optional u then v as float32, Nx*Ny each (when flags bit0 set)
//
// Control messages and batched diagnostic time-series are JSON.

var (
	magicDBRK = [4]byte{'D', 'B', 'R', 'K'}

	// ErrShortFrame, ErrBadMagic, ErrBadVersion and ErrBadSize are returned by
	// DecodeFrame for malformed input.
	ErrShortFrame = errors.New("dambreak frame: buffer shorter than header")
	ErrBadMagic   = errors.New("dambreak frame: bad magic")
	ErrBadVersion = errors.New("dambreak frame: unsupported version")
	ErrBadSize    = errors.New("dambreak frame: payload size mismatch")
)

const (
	protocolVersion = 1

	// FrameFlagVelocity marks frames that carry u,v float32 planes.
	FrameFlagVelocity byte = 1

	// DiagFloats is the number of diagnostic float64s in the header.
	DiagFloats = 10

	// FrameHeaderSize is the fixed binary header size in bytes.
	FrameHeaderSize = 4 + 1 + 1 + 4 + 4 + 4 + 8 + 8 + 8 + 8*DiagFloats // 122
)

// Frame is the decoded form of a binary frame.
type Frame struct {
	Flags   byte
	FrameID uint32
	Nx, Ny  int
	Step    int64
	T       float64
	TStar   float64
	Diags   [DiagFloats]float64
	Alpha   []uint8
	U, V    []float32 // nil unless FrameFlagVelocity
}

// EncodeFrame serialises a snapshot into the binary frame format. When
// includeVel is true the cell-centred u,v planes are appended as float32.
func EncodeFrame(frameID uint32, snap *solver.Snapshot, includeVel bool) []byte {
	n := snap.Nx * snap.Ny
	flags := byte(0)
	if includeVel {
		flags |= FrameFlagVelocity
	}
	size := FrameHeaderSize + n
	if includeVel {
		size += 2 * 4 * n
	}
	buf := make([]byte, size)

	copy(buf[0:4], magicDBRK[:])
	buf[4] = protocolVersion
	buf[5] = flags
	binary.LittleEndian.PutUint32(buf[6:10], frameID)
	binary.LittleEndian.PutUint32(buf[10:14], uint32(snap.Nx))
	binary.LittleEndian.PutUint32(buf[14:18], uint32(snap.Ny))
	binary.LittleEndian.PutUint64(buf[18:26], uint64(snap.Step))
	binary.LittleEndian.PutUint64(buf[26:34], math.Float64bits(snap.Time))
	binary.LittleEndian.PutUint64(buf[34:42], math.Float64bits(snap.TStar))

	diags := [DiagFloats]float64{
		snap.DT, snap.CFL,
		snap.Volume, snap.RefVolume, snap.VolumeDriftPct,
		snap.MaxDiv,
		snap.FrontX, snap.FrontXStar,
		float64(snap.PoissonIter), snap.PoissonResidual,
	}
	off := 42
	for _, d := range diags {
		binary.LittleEndian.PutUint64(buf[off:off+8], math.Float64bits(d))
		off += 8
	}

	for k, a := range snap.Alpha {
		v := int32(a*255.0 + 0.5)
		if v < 0 {
			v = 0
		} else if v > 255 {
			v = 255
		}
		buf[off+k] = byte(v)
	}
	off += n

	if includeVel {
		for k := 0; k < n; k++ {
			binary.LittleEndian.PutUint32(buf[off:off+4], math.Float32bits(float32(snap.U[k])))
			off += 4
		}
		for k := 0; k < n; k++ {
			binary.LittleEndian.PutUint32(buf[off:off+4], math.Float32bits(float32(snap.V[k])))
			off += 4
		}
	}
	return buf
}

// DecodeFrame parses a binary frame back into a Frame, validating the magic,
// version and payload size. The returned slices alias buf.
func DecodeFrame(buf []byte) (*Frame, error) {
	if len(buf) < FrameHeaderSize {
		return nil, ErrShortFrame
	}
	if buf[0] != magicDBRK[0] || buf[1] != magicDBRK[1] || buf[2] != magicDBRK[2] || buf[3] != magicDBRK[3] {
		return nil, ErrBadMagic
	}
	if buf[4] != protocolVersion {
		return nil, fmt.Errorf("%w: %d", ErrBadVersion, buf[4])
	}
	f := &Frame{
		Flags:   buf[5],
		FrameID: binary.LittleEndian.Uint32(buf[6:10]),
		Nx:      int(binary.LittleEndian.Uint32(buf[10:14])),
		Ny:      int(binary.LittleEndian.Uint32(buf[14:18])),
		Step:    int64(binary.LittleEndian.Uint64(buf[18:26])),
		T:       math.Float64frombits(binary.LittleEndian.Uint64(buf[26:34])),
		TStar:   math.Float64frombits(binary.LittleEndian.Uint64(buf[34:42])),
	}
	if f.Nx <= 0 || f.Ny <= 0 || f.Nx > 1<<20 || f.Ny > 1<<20 {
		return nil, ErrBadSize
	}
	off := 42
	for k := 0; k < DiagFloats; k++ {
		f.Diags[k] = math.Float64frombits(binary.LittleEndian.Uint64(buf[off : off+8]))
		off += 8
	}
	n := f.Nx * f.Ny
	want := FrameHeaderSize + n
	if f.Flags&FrameFlagVelocity != 0 {
		want += 8 * n
	}
	if len(buf) != want {
		return nil, fmt.Errorf("%w: got %d bytes, want %d", ErrBadSize, len(buf), want)
	}
	f.Alpha = buf[off : off+n]
	off += n
	if f.Flags&FrameFlagVelocity != 0 {
		f.U = make([]float32, n)
		for k := 0; k < n; k++ {
			f.U[k] = math.Float32frombits(binary.LittleEndian.Uint32(buf[off : off+4]))
			off += 4
		}
		f.V = make([]float32, n)
		for k := 0; k < n; k++ {
			f.V[k] = math.Float32frombits(binary.LittleEndian.Uint32(buf[off : off+4]))
			off += 4
		}
	}
	return f, nil
}

// ---- JSON messages ---------------------------------------------------------

// ControlMessage is a JSON message sent by clients.
//
//	{"type":"play"} {"type":"pause"} {"type":"reset"} {"type":"step"}
//	{"type":"setParams","params":{...}}
//	{"type":"setFrontDef","frontDef":"0.5"|"0.99"}
type ControlMessage struct {
	Type     string        `json:"type"`
	Params   *ServerParams `json:"params,omitempty"`
	FrontDef string        `json:"frontDef,omitempty"`
}

// HelloMessage opens a session: current params, validated ranges, enum values.
type HelloMessage struct {
	Type           string                `json:"type"`
	Params         ServerParams          `json:"params"`
	ValidatedRng   map[string][2]float64 `json:"validatedRanges"`
	Schemes        []string              `json:"schemes"`
	TimeScales     []string              `json:"timeScales"`
	FrontDefs      []string              `json:"frontDefs"`
	FrontDef       string                `json:"frontDef"`
	FreeSlipAvail  bool                  `json:"freeSlipAvailable"`
	AutoPauseTStar float64               `json:"autoPauseTStar"`
	Placeholders   bool                  `json:"benchmarkPlaceholder"`
}

// StatusMessage is a low-rate JSON broadcast with measured rates.
type StatusMessage struct {
	Type        string   `json:"type"`
	Running     bool     `json:"running"`
	StepsPerSec float64  `json:"stepsPerSec"`
	SlowFactor  float64  `json:"slowMotionFactor"` // wall seconds per simulated second
	Clients     int      `json:"clients"`
	OutOfRange  []string `json:"outOfRange,omitempty"`
	AutoPaused  bool     `json:"autoPaused,omitempty"`
	LastErr     string   `json:"lastError,omitempty"`
}

// TracePoint is one entry of a run's front-position time series: dimensionless
// time vs front position under both front definitions.
type TracePoint struct {
	T   float64 `json:"t"`   // t*
	X05 float64 `json:"x05"` // X* via alpha=0.5 crossing (solver)
	X99 float64 `json:"x99"` // X* via 99% cumulative alpha
}

// SeriesRun is one completed run kept for comparison.
type SeriesRun struct {
	ID     int          `json:"id"`
	Label  string       `json:"label"`
	Points []TracePoint `json:"points"`
	Params ServerParams `json:"params"`
}

// SeriesMessage carries archived runs and, when Points is set ("trace" type),
// the live in-progress trace as well.
type SeriesMessage struct {
	Type      string       `json:"type"`
	Runs      []SeriesRun  `json:"runs"`
	Points    []TracePoint `json:"points,omitempty"`
	TimeScale string       `json:"timeScale"`
}

// BenchMessage carries the Martin & Moyce comparison data.
type BenchMessage struct {
	Type      string       `json:"type"`
	Verified  bool         `json:"verified"`
	Label     string       `json:"label"`
	TimeScale string       `json:"timeScale"`
	Points    []BenchPoint `json:"points"`
}

// BenchPoint is one experimental marker (t*, X*).
type BenchPoint struct {
	T float64 `json:"t"`
	X float64 `json:"x"`
}

// ErrorMessage is a human-readable failure notice.
type ErrorMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
