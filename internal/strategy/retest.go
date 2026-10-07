package strategy

import "otc-predictor/pkg/types"

type RetestResult struct {
	Side  string
	Level float64
}

// DetectRetest checks whether, after a recent break of structure, price has
// come back to retest the broken level and shown rejection back in the
// breakout direction — the classic price-action confirmation that a level
// has genuinely flipped (old resistance now holding as support, or the
// reverse), rather than just being a one-candle poke through.
//
// This did not exist anywhere in the codebase before — BOS only checked
// whether the break itself just happened on the latest candle, with no
// concept of a pullback-and-confirm afterward.
func DetectRetest(candles []types.Candle, structure StructureResult) *RetestResult {
	const retestWindow = 15          // how many candles back we'll look for the original break
	const touchTolerancePct = 0.0015 // 0.15% — a level is a zone, not a laser line

	if len(candles) < retestWindow+5 || structure.LastHigh == nil || structure.LastLow == nil {
		return nil
	}
	n := len(candles)
	start := n - retestWindow - 2
	if start < 0 {
		start = 0
	}

	// Bullish: find where price first closed above LastHigh, then look for a
	// later candle that pulled back down to that level and closed back above it.
	level := structure.LastHigh.Price
	brokeAt := -1
	for i := start; i < n-1; i++ {
		if candles[i].Close > level && (i == 0 || candles[i-1].Close <= level) {
			brokeAt = i
		}
	}
	if brokeAt != -1 {
		for i := brokeAt + 1; i < n; i++ {
			c := candles[i]
			if c.Low <= level*(1+touchTolerancePct) && c.Close > level {
				return &RetestResult{Side: "bull", Level: level}
			}
		}
	}

	// Bearish: mirror logic against LastLow.
	level = structure.LastLow.Price
	brokeAt = -1
	for i := start; i < n-1; i++ {
		if candles[i].Close < level && (i == 0 || candles[i-1].Close >= level) {
			brokeAt = i
		}
	}
	if brokeAt != -1 {
		for i := brokeAt + 1; i < n; i++ {
			c := candles[i]
			if c.High >= level*(1-touchTolerancePct) && c.Close < level {
				return &RetestResult{Side: "bear", Level: level}
			}
		}
	}

	return nil
}
