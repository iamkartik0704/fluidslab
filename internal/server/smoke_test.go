package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestSmokeServer10Frames starts the full HTTP+websocket server, connects,
// orders a play and asserts at least 10 binary frames arrive.
func TestSmokeServer10Frames(t *testing.T) {
	r := NewRunner(DefaultParams())
	defer r.Quit()
	h, err := NewHub(r)
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	stop := make(chan struct{})
	defer close(stop)
	go h.Run(stop) // start the 30 Hz frame/status/trace pump

	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Hello + series + benchmark arrive as text messages first.
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	helloType := readJSONType(t, conn)
	if helloType != "hello" {
		t.Fatalf("first message = %q, want hello", helloType)
	}

	// Ask for a velocity overlay too (exercises the float32 path end-to-end).
	if err := conn.WriteJSON(struct {
		Type    string `json:"type"`
		Enabled bool   `json:"enabled"`
	}{"setOverlay", true}); err != nil {
		t.Fatalf("write setOverlay: %v", err)
	}
	if err := conn.WriteJSON(ControlMessage{Type: "step"}); err != nil {
		t.Fatalf("write step: %v", err)
	}

	frames := 0
	deadline := time.Now().Add(30 * time.Second)
	for frames < 10 && time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second)) // per-read
		kind, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read after %d frames: %v", frames, err)
		}
		if kind != websocket.BinaryMessage {
			continue // text (status/trace/benchmark) while waiting for frames
		}
		f, err := DecodeFrame(data)
		if err != nil {
			t.Fatalf("client decode: %v", err)
		}
		if f.Nx != 128 || f.Ny != 64 {
			t.Fatalf("frame grid %dx%d, want 128x64 (16 cells/L0 in an 8x4 L0 domain)", f.Nx, f.Ny)
		}
		frames++
		if frames == 1 && f.U == nil {
			t.Fatal("overlay requested but first frame has no velocity planes")
		}
		if frames < 3 {
			// Top up with more single steps while the pipeline drains.
			_ = conn.WriteJSON(ControlMessage{Type: "step"})
		}
	}
	if frames < 10 {
		t.Fatalf("only %d frames received", frames)
	}
}

// TestServeStaticAndBenchmark checks the embedded FS routes.
func TestServeStaticAndBenchmark(t *testing.T) {
	r := NewRunner(DefaultParams())
	defer r.Quit()
	h, err := NewHub(r)
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET / status %d", resp.StatusCode)
	}
	buf := make([]byte, 64)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(buf), "<!DOCTYPE html>") && !strings.Contains(strings.ToLower(string(buf)), "<!doctype") {
		t.Fatalf("index.html not served, got %.40q", buf)
	}

	resp2, err := http.Get(srv.URL + "/api/benchmark")
	if err != nil {
		t.Fatalf("GET benchmark: %v", err)
	}
	defer resp2.Body.Close()
	var bm map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&bm); err != nil {
		t.Fatalf("benchmark JSON: %v", err)
	}
	if v, ok := bm["verified"].(bool); !ok || v {
		t.Fatalf("benchmark must be unverified placeholder, got %v", bm["verified"])
	}
}

func readJSONType(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	kind, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("readJSONType: %v", err)
	}
	if kind != websocket.TextMessage {
		t.Fatal("expected text message")
	}
	var m struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json: %v", err)
	}
	return m.Type
}
