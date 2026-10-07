package indicators

import (
	"math"
	"otc-predictor/pkg/types"
)

type ADXResult struct {
	ADX     float64
	PlusDI  float64
	MinusDI float64
}

// ADX calculates Average Directional Index
func ADX(candles []types.Candle, period int) ADXResult {
	if len(candles) < period*2 {
		return ADXResult{}
	}
	trList, plusDM, minusDM := []float64{}, []float64{}, []float64{}
	for i := 1; i < len(candles); i++ {
		curr, prev := candles[i], candles[i-1]
		tr := math.Max(curr.High-curr.Low, math.Max(math.Abs(curr.High-prev.Close), math.Abs(curr.Low-prev.Close)))
		pdm := curr.High - prev.High
		mdm := prev.Low - curr.Low
		trList = append(trList, tr)
		if pdm > mdm && pdm > 0 {
			plusDM = append(plusDM, pdm)
		} else {
			plusDM = append(plusDM, 0)
		}
		if mdm > pdm && mdm > 0 {
			minusDM = append(minusDM, mdm)
		} else {
			minusDM = append(minusDM, 0)
		}
	}

	smooth := func(arr []float64) []float64 {
		if len(arr) < period {
			return []float64{}
		}
		s := 0.0
		for i := 0; i < period; i++ {
			s += arr[i]
		}
		result := []float64{s}
		for i := period; i < len(arr); i++ {
			s = s - s/float64(period) + arr[i]
			result = append(result, s)
		}
		return result
	}

	sTR := smooth(trList)
	sPDM := smooth(plusDM)
	sMDM := smooth(minusDM)

	if len(sTR) == 0 {
		return ADXResult{}
	}

	diPlus := make([]float64, len(sTR))
	diMinus := make([]float64, len(sTR))
	for i, t := range sTR {
		if t == 0 {
			diPlus[i], diMinus[i] = 0, 0
		} else {
			diPlus[i] = sPDM[i] / t * 100
			diMinus[i] = sMDM[i] / t * 100
		}
	}

	dx := make([]float64, len(diPlus))
	for i, p := range diPlus {
		sum := p + diMinus[i]
		if sum == 0 {
			dx[i] = 0
		} else {
			dx[i] = math.Abs(p-diMinus[i]) / sum * 100
		}
	}

	adxSmooth := smooth(dx)
	if len(adxSmooth) == 0 {
		return ADXResult{PlusDI: diPlus[len(diPlus)-1], MinusDI: diMinus[len(diMinus)-1]}
	}

	return ADXResult{
		ADX:     adxSmooth[len(adxSmooth)-1],
		PlusDI:  diPlus[len(diPlus)-1],
		MinusDI: diMinus[len(diMinus)-1],
	}
}
