package indicators

import "math"

type BollingerBands struct {
	Upper float64
	Lower float64
	Mid   float64
	Valid bool
}

// Bollinger calculates Bollinger Bands
func Bollinger(closes []float64, period int) BollingerBands {
	if len(closes) < period {
		return BollingerBands{Valid: false}
	}
	slice := closes[len(closes)-period:]
	sum := 0.0
	for _, v := range slice {
		sum += v
	}
	mid := sum / float64(period)
	sqSum := 0.0
	for _, v := range slice {
		sqSum += (v - mid) * (v - mid)
	}
	std := math.Sqrt(sqSum / float64(period))
	return BollingerBands{
		Upper: mid + 2*std,
		Lower: mid - 2*std,
		Mid:   mid,
		Valid: true,
	}
}
