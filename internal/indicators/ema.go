package indicators

// EMA calculates the exponential moving average
func EMA(prices []float64, period int) *float64 {
	if len(prices) < period {
		return nil
	}
	k := 2.0 / (float64(period) + 1.0)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	v := sum / float64(period)
	for i := period; i < len(prices); i++ {
		v = prices[i]*k + v*(1-k)
	}
	return &v
}
