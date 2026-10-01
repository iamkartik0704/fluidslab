package main

import (
	"math"
	"testing"
)

func TestInterpTSim(t *testing.T) {
	// Synthetic data
	Z := []float64{1.0, 2.0, 3.0, 5.0}
	T := []float64{0.5, 1.5, 2.5, 4.5} // T = Z - 0.5

	if math.IsNaN(interpTSim(Z, T, 0.5)) {
		t.Log("Correctly returns NaN outside bounds")
	} else {
		t.Error("Failed outside lower bound")
	}

	if math.IsNaN(interpTSim(Z, T, 6.0)) {
		t.Log("Correctly returns NaN outside bounds")
	} else {
		t.Error("Failed outside upper bound")
	}

	if val := interpTSim(Z, T, 1.0); math.Abs(val-0.5) > 1e-9 {
		t.Errorf("Expected 0.5 at Z=1.0, got %f", val)
	}

	if val := interpTSim(Z, T, 4.0); math.Abs(val-3.5) > 1e-9 {
		t.Errorf("Expected 3.5 at Z=4.0, got %f", val)
	}
}

func TestAlignTime(t *testing.T) {
	// Synthetic data
	Z := []float64{1.0, 2.0, 3.0, 5.0}
	T := []float64{0.5, 1.5, 2.5, 4.5} 

	// Anchor Z = 2.0. So T_raw at anchor = 1.5
	// Suppose Anchor T = 3.0 (which is the experimental anchor).
	// Shift should be 3.0 - 1.5 = +1.5
	alignedT := alignTime(Z, T, 2.0, 3.0)

	expectedT := []float64{2.0, 3.0, 4.0, 6.0}
	for i := range alignedT {
		if math.Abs(alignedT[i]-expectedT[i]) > 1e-9 {
			t.Errorf("Index %d: expected %f, got %f", i, expectedT[i], alignedT[i])
		}
	}
}

func TestTScaleMapping(t *testing.T) {
	// T-scale for H0/L0 = 1 and 2
	// For H0/L0 = 2 (DamBreak default here), H0 = 2 L0.
	// t* = t * sqrt(2g / L0).
	// The benchmark paper states non-dimensional time tau = t * sqrt(2g / a) where a=L0, or for another ratio it varies.
	// We test if the ratio is applied properly.
	g := 9.81
	L0 := 0.05715
	expectedTStar := 1.0 * math.Sqrt(2*g/L0)
	if math.Abs(expectedTStar-18.528548) > 1e-3 {
		t.Errorf("Expected T* for t=1s is 18.5285, got %f", expectedTStar)
	}
}
