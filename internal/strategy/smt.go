package strategy

import "otc-predictor/pkg/types"

// SMTResult mirrors the JS shape.
type SMTResult struct {
	Side  string
	Label string
}

var smtPairs = map[string]string{
	"EURUSD": "GBPUSD",
	"GBPUSD": "EURUSD",
}

// GetSMTPartner returns the correlated instrument symbol for SMT Divergence,
// or "" if none is defined. Currently only EURUSD/GBPUSD have a standard
// correlated partner among our forex market list.
func GetSMTPartner(symbol string) string {
	return smtPairs[symbol]
}

// DetectSMTDivergence compares this instrument's most recent swing points
// against a correlated partner's — a bullish divergence occurs when this
// instrument makes a LOWER low while the partner makes a HIGHER low
// (partner shows relative strength); bearish is the mirror case at highs.
func DetectSMTDivergence(candles, partnerCandles []types.Candle) *SMTResult {
	if len(candles) < 20 || len(partnerCandles) < 20 {
		return nil
	}

	tail := func(c []types.Candle, n int) []types.Candle {
		if len(c) <= n {
			return c
		}
		return c[len(c)-n:]
	}

	lows := swingLows(tail(candles, 40), 3)
	partnerLows := swingLows(tail(partnerCandles, 40), 3)
	highs := swingHighs(tail(candles, 40), 3)
	partnerHighs := swingHighs(tail(partnerCandles, 40), 3)

	if len(lows) >= 2 && len(partnerLows) >= 2 {
		prevLow, lastLow := lows[len(lows)-2], lows[len(lows)-1]
		partnerPrevLow, partnerLastLow := partnerLows[len(partnerLows)-2], partnerLows[len(partnerLows)-1]
		if lastLow.Price < prevLow.Price && partnerLastLow.Price > partnerPrevLow.Price {
			return &SMTResult{Side: "bull", Label: "SMT Divergence — partner shows relative strength at lows"}
		}
	}

	if len(highs) >= 2 && len(partnerHighs) >= 2 {
		prevHigh, lastHigh := highs[len(highs)-2], highs[len(highs)-1]
		partnerPrevHigh, partnerLastHigh := partnerHighs[len(partnerHighs)-2], partnerHighs[len(partnerHighs)-1]
		if lastHigh.Price > prevHigh.Price && partnerLastHigh.Price < partnerPrevHigh.Price {
			return &SMTResult{Side: "bear", Label: "SMT Divergence — partner shows relative weakness at highs"}
		}
	}

	return nil
}
