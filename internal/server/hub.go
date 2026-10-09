package server

import (
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	dambreak "dambreak"
	"dambreak/internal/solver"
)

// frameInterval is the fixed display period: frames go out at 30 Hz,
// re-clocked by the hub regardless of the solver step rate.
const frameInterval = time.Second / 30

// statusInterval is the low-rate JSON status period.
const statusInterval = time.Second / 5

// traceInterval is the period of batched diagnostic time-series broadcasts.
const traceInterval = time.Second / 2

// Hub owns the HTTP server, the websocket client set and the 30 Hz pump.
type Hub struct {
	runner *Runner
	mux    *http.ServeMux
	wsUp   websocket.Upgrader
	bench  *BenchMessage

	mu      sync.Mutex
	clients map[*wsClient]bool
	status  StatusMessage
}

type wsClient struct {
	conn    *websocket.Conn
	sendMu  sync.Mutex
	closed  bool
	withVel bool
}

// NewHub wires the hub with a runner and the embedded static assets.
func NewHub(r *Runner) (*Hub, error) {
	h := &Hub{
		runner:  r,
		mux:     http.NewServeMux(),
		clients: make(map[*wsClient]bool),
	}
	h.wsUp = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 1 << 20,
		CheckOrigin:     func(r *http.Request) bool { return true }, // local tool
	}

	// ui/ is embedded at the repo root (embed cannot cross directories), so
	// serve the "ui" subtree of the root package FS at "/".
	webSub, err := fs.Sub(dambreak.WebFS, "ui")
	if err != nil {
		return nil, err
	}
	h.mux.Handle("/", http.FileServer(http.FS(webSub)))
	h.mux.HandleFunc("/ws", h.serveWS)
	h.mux.HandleFunc("/api/benchmark", h.serveBenchmark)

	bench, err := loadBenchmark()
	if err != nil {
		return nil, err
	}
	h.bench = bench
	return h, nil
}

// Handler returns the hub's HTTP handler (used by tests too).
func (h *Hub) Handler() http.Handler { return h.mux }

// Serve accepts on ln until the process exits.
func (h *Hub) Serve(ln net.Listener) error {
	srv := &http.Server{Handler: h.mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}

// serveBenchmark sends the benchmark JSON as-is (placeholder marker included).
func (h *Hub) serveBenchmark(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.bench)
}

// serveWS upgrades and registers one client.
func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.wsUp.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &wsClient{conn: conn}
	h.mu.Lock()
	h.clients[c] = true
	h.status.Clients = len(h.clients)
	h.mu.Unlock()

	// Hello with current state.
	hello := HelloMessage{
		Type:           "hello",
		Params:         DefaultParams(),
		ValidatedRng:   ValidatedRanges,
		Schemes:        SchemeNames,
		TimeScales:     TimeScaleNames,
		FrontDefs:      []string{FrontDefHalf, FrontDef99},
		FrontDef:       FrontDefHalf,
		FreeSlipAvail:  false, // toggle rendered but disabled in the UI
		AutoPauseTStar: AutoPauseTStar,
		Placeholders:   true,
	}
	_ = h.sendJSON(c, hello)
	_ = h.sendJSON(c, h.seriesMessage())
	_ = h.sendJSON(c, h.bench)

	go h.readPump(c)
}

// readPump consumes client JSON control messages until close.
func (h *Hub) readPump(c *wsClient) {
	defer h.removeClient(c)
	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg ControlMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			_ = h.sendJSON(c, ErrorMessage{Type: "error", Message: "bad JSON: " + err.Error()})
			continue
		}
		switch msg.Type {
		case "play", "pause", "reset", "step", "setFrontDef":
			select {
			case h.runner.CmdCh() <- runnerCmd{kind: msg.Type, frontDef: msg.FrontDef}:
			default:
				_ = h.sendJSON(c, ErrorMessage{Type: "error", Message: "server busy"})
			}
		case "setParams":
			if msg.Params == nil {
				_ = h.sendJSON(c, ErrorMessage{Type: "error", Message: "setParams missing params"})
				continue
			}
			p := msg.Params.Clamp() // validate+clamp server-side before the runner sees it
			select {
			case h.runner.CmdCh() <- runnerCmd{kind: "setParams", params: &p}:
			default:
				_ = h.sendJSON(c, ErrorMessage{Type: "error", Message: "server busy"})
			}
		case "setOverlay":
			var o struct {
				Enabled bool `json:"enabled"`
			}
			if json.Unmarshal(payload, &o) == nil {
				c.sendMu.Lock()
				c.withVel = o.Enabled
				c.sendMu.Unlock()
			}
		default:
			_ = h.sendJSON(c, ErrorMessage{Type: "error", Message: "unknown message type"})
		}
	}
}

func (h *Hub) removeClient(c *wsClient) {
	h.mu.Lock()
	if !c.closed {
		c.closed = true
		delete(h.clients, c)
		h.status.Clients = len(h.clients)
	}
	h.mu.Unlock()
	_ = c.conn.Close()
}

// sendJSON marshals and sends one JSON message under the client lock.
func (h *Hub) sendJSON(c *wsClient, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
	return c.conn.WriteMessage(websocket.TextMessage, b)
}

// sendBinary sends one binary frame under the client lock; a slow client's
// pending write simply times out (stale frames are never queued up).
func (h *Hub) sendBinary(c *wsClient, b []byte) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
	return c.conn.WriteMessage(websocket.BinaryMessage, b)
}

// Run pumps frames at a fixed 30 Hz and statuses at 5 Hz until stop closes.
// The 30 Hz ticker re-clocks the display: whatever snapshot is newest when the
// tick fires is what goes out; slow clients get a stale-dropping write, never
// an unbounded queue.
func (h *Hub) Run(stop <-chan struct{}) {
	frameTick := time.NewTicker(frameInterval)
	statusTick := time.NewTicker(statusInterval)
	traceTick := time.NewTicker(traceInterval)
	defer frameTick.Stop()
	defer statusTick.Stop()
	defer traceTick.Stop()

	var latest *snapshotBox
	var frameID atomic.Uint32
	var meter rateMeter
	var stepMark int
	var tSimMark float64
	
	for {
		select {
		case <-stop:
			return

		case snap := <-h.runner.Frames():
			latest = &snapshotBox{snap: snap}
			stepMark, tSimMark = snap.Step, snap.Time

		case <-frameTick.C:
			if latest == nil {
				continue
			}
			h.mu.Lock()
			targets := make([]*wsClient, 0, len(h.clients))
			anyVel := false
			for c := range h.clients {
				if c.withVel {
					anyVel = true
				}
				targets = append(targets, c)
			}
			h.mu.Unlock()

			base := EncodeFrame(frameID.Add(1), latest.snap, false)
			var vel []byte
			if anyVel {
				vel = EncodeFrame(frameID.Add(1), latest.snap, true)
			}
			for _, c := range targets {
				payload := base
				if c.withVel && vel != nil {
					payload = vel
				}
				if err := h.sendBinary(c, payload); err != nil {
					h.removeClient(c)
				}
			}
			latest = nil // DO NOT RE-SEND THE SAME FRAME FOREVER

		case <-statusTick.C:
			h.mu.Lock()
			st := h.status
			h.mu.Unlock()
			st.Type = "status"
			st.Running = h.runner.LastRunning()
			st.StepsPerSec, st.SlowFactor = meter.rates(stepMark, tSimMark, time.Now())
			st.Clients = h.clientCount()
			st.LastErr = h.runner.LastError()
			st.AutoPaused = h.runner.AutoPaused()
			h.broadcastJSON(st)

		case st := <-h.runner.Statuses():
			h.broadcastJSON(st)

		case <-traceTick.C:
			msg := SeriesMessage{
				Type:      "trace",
				Runs:      h.runner.ArchivedRuns(),
				Points:    h.runner.LiveTrace(),
				TimeScale: h.runner.CurrentTimeScale(),
			}
			h.broadcastJSON(msg)
		}
	}
}

// snapshotBox is a non-nil holder so the pump can distinguish "nothing yet"
// from a valid snapshot.
type snapshotBox struct {
	snap *solver.Snapshot
}

// rateMeter converts frame-pump marks into steps/s and slow-motion factor.
type rateMeter struct {
	lastStep int
	lastTSim float64
	lastWall time.Time
	inited   bool
}

func (m *rateMeter) rates(step int, tSim float64, now time.Time) (stepsPerSec, slowFactor float64) {
	if !m.inited {
		m.inited = true
		m.lastStep, m.lastTSim, m.lastWall = step, tSim, now
		return 0, 0
	}
	dWall := now.Sub(m.lastWall).Seconds()
	dSim := tSim - m.lastTSim
	dSteps := step - m.lastStep
	m.lastStep, m.lastTSim, m.lastWall = step, tSim, now
	if dWall <= 0 {
		return 0, 0
	}
	sps := float64(dSteps) / dWall
	sf := 0.0
	if dSim > 1e-9 {
		sf = dWall / dSim
	}
	return sps, sf
}

func (h *Hub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// seriesMessage assembles archived runs for a new client.
func (h *Hub) seriesMessage() SeriesMessage {
	return SeriesMessage{
		Type:      "series",
		Runs:      h.runner.ArchivedRuns(),
		TimeScale: h.runner.CurrentTimeScale(),
	}
}

// broadcastJSON sends one JSON message to every client, dropping failures.
func (h *Hub) broadcastJSON(v any) {
	h.mu.Lock()
	targets := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.Unlock()
	for _, c := range targets {
		if err := h.sendJSON(c, v); err != nil {
			h.removeClient(c)
		}
	}
}

// loadBenchmark reads the embedded Martin & Moyce JSON, preserving the
// verified flag (false = PLACEHOLDER).
func loadBenchmark() (*BenchMessage, error) {
	raw, err := dambreak.BenchmarkFS.ReadFile("benchmark/martin_moyce.json")
	if err != nil {
		return nil, err
	}
	var file struct {
		Entries []struct {
			Verified  bool   `json:"verified"`
			TimeScale string `json:"timeScale"`
			Points    []struct {
				T float64 `json:"t"`
				X float64 `json:"x"`
			} `json:"points"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	if len(file.Entries) == 0 {
		return nil, errNoBench
	}
	e := file.Entries[0]
	msg := &BenchMessage{
		Type:      "benchmark",
		Verified:  e.Verified,
		Label:     "Martin & Moyce (1952)",
		TimeScale: e.TimeScale,
	}
	for _, p := range e.Points {
		msg.Points = append(msg.Points, BenchPoint{T: p.T, X: p.X})
	}
	return msg, nil
}

var errNoBench = errString("benchmark JSON has no entries")

type errString string

func (e errString) Error() string { return string(e) }

var _ = log.Print // log is used by OpenBrowser in open.go
