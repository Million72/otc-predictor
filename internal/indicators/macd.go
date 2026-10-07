package indicators

// MACD returns macd line, signal line, and histogram
func MACD(closes []float64) (float64, float64, float64) {
	e12 := EMA(closes, 12)
	e26 := EMA(closes, 26)
	if e12 == nil || e26 == nil {
		return 0, 0, 0
	}
	macdLine := *e12 - *e26
	series := []float64{}
	for i := 26; i <= len(closes); i++ {
		a := EMA(closes[:i], 12)
		b := EMA(closes[:i], 26)
		if a != nil && b != nil {
			series = append(series, *a-*b)
		}
	}
	signal := EMA(series, 9)
	sig := macdLine
	if signal != nil {
		sig = *signal
	}
	return macdLine, sig, macdLine - sig
}
