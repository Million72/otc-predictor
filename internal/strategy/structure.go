package strategy

import "otc-predictor/pkg/types"

type StructureResult struct {
	Bias     string // "BULLISH" | "BEARISH" | "NEUTRAL"
	LastHigh *SwingPoint
	LastLow  *SwingPoint
}

// MarketStructure detects HH/HL vs LH/LL bias from recent swings.
func MarketStructure(candles []types.Candle) StructureResult {
	if len(candles) < 20 {
		return StructureResult{Bias: "NEUTRAL"}
	}
	start := 0
	if len(candles) > 60 {
		start = len(candles) - 60
	}
	slice := candles[start:]
	highs := swingHighs(slice, 3)
	lows := swingLows(slice, 3)

	if len(highs) < 2 || len(lows) < 2 {
		return StructureResult{Bias: "NEUTRAL"}
	}

	h1, h2 := highs[len(highs)-2], highs[len(highs)-1]
	l1, l2 := lows[len(lows)-2], lows[len(lows)-1]

	bias := "NEUTRAL"
	if h2.Price > h1.Price && l2.Price > l1.Price {
		bias = "BULLISH"
	} else if h2.Price < h1.Price && l2.Price < l1.Price {
		bias = "BEARISH"
	}

	lh := highs[len(highs)-1]
	ll := lows[len(lows)-1]

	return StructureResult{Bias: bias, LastHigh: &lh, LastLow: &ll}
}

type SRResult struct {
	Support        *float64
	Resistance     *float64
	NearSupport    bool
	NearResistance bool
}

// SupportResistance finds nearby key levels from swing points.
func SupportResistance(candles []types.Candle) SRResult {
	if len(candles) < 20 {
		return SRResult{}
	}
	start := 0
	if len(candles) > 50 {
		start = len(candles) - 50
	}
	slice := candles[start:]
	highs := swingHighs(slice, 3)
	lows := swingLows(slice, 3)
	price := candles[len(candles)-1].Close

	var resistance, support *float64
	if len(highs) > 0 {
		maxH := highs[0].Price
		for _, h := range highs {
			if h.Price > maxH {
				maxH = h.Price
			}
		}
		resistance = &maxH
	}
	if len(lows) > 0 {
		minL := lows[0].Price
		for _, l := range lows {
			if l.Price < minL {
				minL = l.Price
			}
		}
		support = &minL
	}

	rng := 1.0
	if resistance != nil && support != nil {
		rng = *resistance - *support
		if rng == 0 {
			rng = 1
		}
	}

	res := SRResult{Support: support, Resistance: resistance}
	if resistance != nil {
		res.NearResistance = absF(price-*resistance)/rng < 0.05
	}
	if support != nil {
		res.NearSupport = absF(price-*support)/rng < 0.05
	}
	return res
}

type SweepResult struct {
	Side  string
	Level float64
}

// LiquiditySweep detects a stop-hunt wick beyond recent extremes that closes back inside.
func LiquiditySweep(candles []types.Candle) *SweepResult {
	if len(candles) < 20 {
		return nil
	}
	start := len(candles) - 30
	if start < 0 {
		start = 0
	}
	slice := candles[start:]
	if len(slice) < 2 {
		return nil
	}
	lookback := slice[:len(slice)-1]
	last := candles[len(candles)-1]

	highs := swingHighs(lookback, 3)
	lows := swingLows(lookback, 3)
	if len(highs) == 0 || len(lows) == 0 {
		return nil
	}

	maxH := highs[0].Price
	for _, h := range highs {
		if h.Price > maxH {
			maxH = h.Price
		}
	}
	minL := lows[0].Price
	for _, l := range lows {
		if l.Price < minL {
			minL = l.Price
		}
	}

	if last.Low < minL && last.Close > minL {
		return &SweepResult{Side: "bull", Level: minL}
	}
	if last.High > maxH && last.Close < maxH {
		return &SweepResult{Side: "bear", Level: maxH}
	}
	return nil
}

// BOS detects break of structure against the last swing high/low.
func BOS(candles []types.Candle, structure StructureResult) *SweepResult {
	if structure.LastHigh == nil || structure.LastLow == nil || len(candles) < 2 {
		return nil
	}
	last := candles[len(candles)-1]
	prev := candles[len(candles)-2]

	if prev.Close <= structure.LastHigh.Price && last.Close > structure.LastHigh.Price {
		return &SweepResult{Side: "bull", Level: structure.LastHigh.Price}
	}
	if prev.Close >= structure.LastLow.Price && last.Close < structure.LastLow.Price {
		return &SweepResult{Side: "bear", Level: structure.LastLow.Price}
	}
	return nil
}

// CHoCH detects change of character — first reversal signal against prevailing structure.
func CHoCH(candles []types.Candle, structure StructureResult) *SweepResult {
	if len(candles) < 2 {
		return nil
	}
	last := candles[len(candles)-1]

	if structure.Bias == "BEARISH" && structure.LastHigh != nil {
		if last.Close > structure.LastHigh.Price {
			return &SweepResult{Side: "bull", Level: structure.LastHigh.Price}
		}
	}
	if structure.Bias == "BULLISH" && structure.LastLow != nil {
		if last.Close < structure.LastLow.Price {
			return &SweepResult{Side: "bear", Level: structure.LastLow.Price}
		}
	}
	return nil
}
