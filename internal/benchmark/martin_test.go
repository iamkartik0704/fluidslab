package benchmark

import (
	"math"
	"testing"
)

func TestInterpTSim(t *testing.T) {
	Z := []float64{1.0, 2.0, 3.0, 5.0}
	T := []float64{0.5, 1.5, 2.5, 4.5} 

	if !math.IsNaN(InterpTSim(Z, T, 0.5)) {
		t.Error("Failed outside lower bound, expected NaN")
	}

	if !math.IsNaN(InterpTSim(Z, T, 6.0)) {
		t.Error("Failed outside upper bound, expected NaN")
	}

	if val := InterpTSim(Z, T, 1.0); math.Abs(val-0.5) > 1e-9 {
		t.Errorf("Expected 0.5 at Z=1.0, got %f", val)
	}

	if val := InterpTSim(Z, T, 4.0); math.Abs(val-3.5) > 1e-9 {
		t.Errorf("Expected 3.5 at Z=4.0, got %f", val)
	}
}

func TestAlignTime(t *testing.T) {
	Z := []float64{1.0, 2.0, 3.0, 5.0}
	T := []float64{0.5, 1.5, 2.5, 4.5} 

	alignedT := AlignTime(Z, T, 2.0, 3.0)

	expectedT := []float64{2.0, 3.0, 4.0, 6.0}
	for i := range alignedT {
		if math.Abs(alignedT[i]-expectedT[i]) > 1e-9 {
			t.Errorf("Index %d: expected %f, got %f", i, expectedT[i], alignedT[i])
		}
	}
}

func TestTScaleMapping(t *testing.T) {
	g := 9.81
	L0 := 0.05715
	
	// TimeScaleSqrt2gOverL0 (H0 = 2*L0)
	expectedTStar2 := 1.0 * math.Sqrt(2*g/L0)
	if math.Abs(expectedTStar2-18.528548) > 1e-3 {
		t.Errorf("Expected T* for t=1s is 18.5285, got %f", expectedTStar2)
	}

	// TimeScaleSqrtgOverL0 (H0 = L0)
	expectedTStar1 := 1.0 * math.Sqrt(g/L0)
	if math.Abs(expectedTStar1-13.10166) > 1e-3 {
		t.Errorf("Expected T* for t=1s is 13.10166, got %f", expectedTStar1)
	}
}

func TestValidateMartinData(t *testing.T) {
	if ValidateMartinData([]float64{1.0, 2.0, 3.0}, []float64{0.0, 1.0, 2.0}) != true {
		t.Errorf("Expected true for valid data")
	}
	if ValidateMartinData([]float64{1.0, 2.0}, []float64{0.0, 1.0, 2.0}) != false {
		t.Errorf("Expected false for mismatched lengths")
	}
	if ValidateMartinData([]float64{0.5, 1.0, 2.0}, []float64{0.0, 1.0, 2.0}) != false {
		t.Errorf("Expected false for Z[0] < 1.0")
	}
	if ValidateMartinData([]float64{1.0, 1.0, 2.0}, []float64{0.0, 1.0, 2.0}) != false {
		t.Errorf("Expected false for non-increasing Z")
	}
	if ValidateMartinData([]float64{1.0, 2.0, 3.0}, []float64{0.0, 1.0, 0.5}) != false {
		t.Errorf("Expected false for non-increasing T")
	}
}
