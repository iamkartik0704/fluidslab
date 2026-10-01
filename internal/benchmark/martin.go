package benchmark

import (
	"math"
)

func InterpTSim(Z_sim, T_sim []float64, Z_target float64) float64 {
	for i := 0; i < len(Z_sim)-1; i++ {
		if Z_target >= Z_sim[i] && Z_target <= Z_sim[i+1] {
			if Z_sim[i+1] == Z_sim[i] {
				return T_sim[i]
			}
			t := (Z_target - Z_sim[i]) / (Z_sim[i+1] - Z_sim[i])
			return T_sim[i] + t*(T_sim[i+1]-T_sim[i])
		}
	}
	return math.NaN()
}

func AlignTime(Z_sim, T_sim []float64, anchorZ, anchorT float64) []float64 {
	tAtAnchor := InterpTSim(Z_sim, T_sim, anchorZ)
	shift := anchorT - tAtAnchor
	alignedT := make([]float64, len(T_sim))
	for i, t := range T_sim {
		alignedT[i] = t + shift
	}
	return alignedT
}

func ValidateMartinData(Z_exp, T_exp []float64) bool {
	if len(Z_exp) != len(T_exp) || len(Z_exp) == 0 {
		return false
	}
	if Z_exp[0] < 1.0 {
		return false
	}
	for i := 1; i < len(Z_exp); i++ {
		if Z_exp[i] <= Z_exp[i-1] || T_exp[i] <= T_exp[i-1] {
			return false
		}
	}
	return true
}
