package indicators

import (
	"math"
	"otc-predictor/pkg/types"
)

// ATR calculates the Average True Range
func ATR(candles []types.Candle, period int) *float64 {
	if len(candles) < period+1 {
		return nil
	}
	trs := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		c, prev := candles[i], candles[i-1]
		tr := math.Max(c.High-c.Low, math.Max(math.Abs(c.High-prev.Close), math.Abs(c.Low-prev.Close)))
		trs = append(trs, tr)
	}
	start := len(trs) - period
	if start < 0 {
		start = 0
	}
	sum := 0.0
	for _, v := range trs[start:] {
		sum += v
	}
	result := sum / float64(period)
	return &result
}
