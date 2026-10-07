package strategy

import "otc-predictor/pkg/types"

func isBull(c types.Candle) bool { return c.Close > c.Open }
func isBear(c types.Candle) bool { return c.Close < c.Open }
func body(c types.Candle) float64 {
	v := c.Close - c.Open
	if v < 0 {
		return -v
	}
	return v
}
func candleRange(c types.Candle) float64 {
	r := c.High - c.Low
	if r == 0 {
		return 0.000001
	}
	return r
}
func upWick(c types.Candle) float64 {
	m := c.Open
	if c.Close > m {
		m = c.Close
	}
	return c.High - m
}
func dnWick(c types.Candle) float64 {
	m := c.Open
	if c.Close < m {
		m = c.Close
	}
	return m - c.Low
}

func closesOf(candles []types.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, c := range candles {
		out[i] = c.Close
	}
	return out
}

type SwingPoint struct {
	Index int
	Price float64
}

func swingHighs(candles []types.Candle, window int) []SwingPoint {
	result := []SwingPoint{}
	for i := window; i < len(candles)-window; i++ {
		isHighest := true
		for j := i - window; j <= i+window; j++ {
			if candles[j].High > candles[i].High {
				isHighest = false
				break
			}
		}
		if isHighest {
			result = append(result, SwingPoint{Index: i, Price: candles[i].High})
		}
	}
	return result
}

func swingLows(candles []types.Candle, window int) []SwingPoint {
	result := []SwingPoint{}
	for i := window; i < len(candles)-window; i++ {
		isLowest := true
		for j := i - window; j <= i+window; j++ {
			if candles[j].Low < candles[i].Low {
				isLowest = false
				break
			}
		}
		if isLowest {
			result = append(result, SwingPoint{Index: i, Price: candles[i].Low})
		}
	}
	return result
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func minF(a, b float64) float64 {
	if a < b {
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
