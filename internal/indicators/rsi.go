package indicators

// RSI calculates the Relative Strength Index
func RSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 50
	}
	deltas := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		deltas[i-1] = closes[i] - closes[i-1]
	}
	ag, al := 0.0, 0.0
	for i := 0; i < period; i++ {
		if deltas[i] > 0 {
			ag += deltas[i]
		} else {
			al -= deltas[i]
		}
	}
	ag /= float64(period)
	al /= float64(period)
	for i := period; i < len(deltas); i++ {
		gain := 0.0
		if deltas[i] > 0 {
			gain = deltas[i]
		}
		loss := 0.0
		if deltas[i] < 0 {
			loss = -deltas[i]
		}
		ag = (ag*float64(period-1) + gain) / float64(period)
		al = (al*float64(period-1) + loss) / float64(period)
	}
	if al == 0 {
		return 100
	}
	return 100 - 100/(1+ag/al)
}
