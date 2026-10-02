package main

import (
	"syscall/js"
	"dambreak/internal/solver"
)

var sim *solver.Simulation
var isRunning bool = false

func initSim(this js.Value, args []js.Value) interface{} {
	cfg := solver.DefaultConfig()
	cfg.Numerical.SecondOrderAdvect = true
	cfg.Numerical.FreeSlip = true

	cellsPerL0 := 16
	nx := 15 * cellsPerL0
	ny := 4 * cellsPerL0
	width := 15.0 * cfg.Domain.L0
	height := 4.0 * cfg.Domain.L0 // actually H0=2 L0, so 4 L0 = 2 H0

	sim = solver.NewSimulation(cfg, nx, ny, width, height, false)
	sim.InitDamBreak()
	solver.ApplyAlphaBC(sim.Grid, sim.Fields)
	solver.UpdateProperties(sim.Grid, sim.Fields, &cfg)
	sim.UpdateDiagnostics()
	
	isRunning = true

	updateUI()
	return nil
}

func stepSim(this js.Value, args []js.Value) interface{} {
	if !isRunning || sim == nil {
		return nil
	}
	
	// take a few steps per frame so it doesn't run extremely slow
	for i := 0; i < 5; i++ {
		err := sim.Step(-1)
		if err != nil {
			isRunning = false
			return js.ValueOf("error")
		}
		if sim.State.TStar >= 12.0 {
			isRunning = false
			break
		}
	}

	updateUI()
	return nil
}

func getGridWidth(this js.Value, args []js.Value) interface{} {
	if sim == nil { return 0 }
	return sim.Grid.Nx
}

func getGridHeight(this js.Value, args []js.Value) interface{} {
	if sim == nil { return 0 }
	return sim.Grid.Ny
}

func getAlphaArray(this js.Value, args []js.Value) interface{} {
	if sim == nil { return nil }
	
	g := sim.Grid
	arr := js.Global().Get("Uint8Array").New(g.Nx * g.Ny)
	
	idx := 0
	for j := g.Ny; j >= 1; j-- {
		for i := 1; i <= g.Nx; i++ {
			a := sim.Fields.Alpha[g.IdxCC(i, j)]
			val := uint8(a * 255)
			arr.SetIndex(idx, val)
			idx++
		}
	}
	return arr
}

func getStats(this js.Value, args []js.Value) interface{} {
	if sim == nil { return nil }
	st := &sim.State
	
	obj := js.Global().Get("Object").New()
	obj.Set("time", st.Time)
	obj.Set("tStar", st.TStar)
	obj.Set("zStar", st.FrontXStar)
	obj.Set("cfl", st.CFL)
	obj.Set("dt", st.DT)
	obj.Set("step", st.Step)
	return obj
}

func setRunning(this js.Value, args []js.Value) interface{} {
	if len(args) > 0 {
		isRunning = args[0].Bool()
	}
	return nil
}

func updateUI() {
	js.Global().Call("updateUIFromGo")
}

func main() {
	c := make(chan struct{}, 0)
	js.Global().Set("initSim", js.FuncOf(initSim))
	js.Global().Set("stepSim", js.FuncOf(stepSim))
	js.Global().Set("getGridWidth", js.FuncOf(getGridWidth))
	js.Global().Set("getGridHeight", js.FuncOf(getGridHeight))
	js.Global().Set("getAlphaArray", js.FuncOf(getAlphaArray))
	js.Global().Set("getStats", js.FuncOf(getStats))
	js.Global().Set("setRunning", js.FuncOf(setRunning))
	<-c
}
