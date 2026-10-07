package indicators

import "otc-predictor/pkg/types"

type SupertrendResult struct {
	Direction int // 1 = bull, -1 = bear, 0 = unknown
	Value     float64
}

// Supertrend calculates the SuperTrend indicator
func Supertrend(candles []types.Candle, period int, multiplier float64) SupertrendResult {
	if len(candles) < period+1 {
		return SupertrendResult{}
	}
	atrVals := atrArray(candles, period)
	offset := len(candles) - len(atrVals)
	if offset < 0 || len(atrVals) == 0 {
		return SupertrendResult{}
	}

	var prevUpper, prevLower float64
	trend := 1

	for i, atrV := range atrVals {
		idx := i + offset
		hl2 := (candles[idx].High + candles[idx].Low) / 2
		upperBand := hl2 + multiplier*atrV
		lowerBand := hl2 - multiplier*atrV

		if i > 0 {
			if !(upperBand < prevUpper) && candles[idx-1].Close <= prevUpper {
				upperBand = prevUpper
			}
			if !(lowerBand > prevLower) && candles[idx-1].Close >= prevLower {
				lowerBand = prevLower
			}
		}

		if i == 0 {
			trend = 1
		} else if candles[idx].Close > prevUpper {
			trend = 1
		} else if candles[idx].Close < prevLower {
			trend = -1
		}

		prevUpper = upperBand
		prevLower = lowerBand
	}

	value := prevLower
	if trend == -1 {
		value = prevUpper
	}
	return SupertrendResult{Direction: trend, Value: value}
}

func atrArray(candles []types.Candle, period int) []float64 {
	if len(candles) < period+1 {
		return []float64{}
	}
	trs := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		c, prev := candles[i], candles[i-1]
		tr := maxF(c.High-c.Low, maxF(absF(c.High-prev.Close), absF(c.Low-prev.Close)))
		trs = append(trs, tr)
	}
	if len(trs) < period {
		return []float64{}
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += trs[i]
	}
	result := []float64{sum / float64(period)}
	for i := period; i < len(trs); i++ {
		next := (result[len(result)-1]*float64(period-1) + trs[i]) / float64(period)
		result = append(result, next)
	}
	return result
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func absF(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}
