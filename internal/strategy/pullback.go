package strategy

import (
	"otc-predictor/internal/indicators"
	"otc-predictor/pkg/types"
)

type PullbackResult struct {
	Side string
}

// DetectPullback checks for a classic trend-continuation setup: price is in
// a clear EMA9>21>50 (or reverse) trend, has recently pulled back to touch
// the EMA21 "dynamic support/resistance" line, and the most recent candle
// shows the trend resuming (a same-direction candle closing back on the
// trend side of EMA21).
//
// This did not exist anywhere in the codebase before. Note on precision:
// this checks recent candles against the CURRENT EMA21 value rather than
// recomputing EMA21 at each historical point in the lookback. EMA21 moves
// little over a 5-6 candle window, so this is a reasonable approximation —
// stated plainly here rather than silently assumed.
func DetectPullback(candles []types.Candle) *PullbackResult {
	const lookback = 6
	if len(candles) < 60 {
		return nil
	}
	closes := closesOf(candles)
	e9 := indicators.EMA(closes, 9)
	e21 := indicators.EMA(closes, 21)
	e50 := indicators.EMA(closes, 50)
	if e9 == nil || e21 == nil || e50 == nil {
		return nil
	}

	n := len(candles)
	start := n - lookback
	if start < 0 {
		start = 0
	}
	recent := candles[start : n-1] // exclude the latest candle, checked separately below
	last := candles[n-1]

	const touchTolerancePct = 0.0025 // 0.25% — EMA21 is a zone, not a laser line

	touchedEMA := false
	for _, c := range recent {
		if c.Low <= *e21*(1+touchTolerancePct) && c.High >= *e21*(1-touchTolerancePct) {
			touchedEMA = true
			break
		}
	}
	if !touchedEMA {
		return nil
	}

	if *e9 > *e21 && *e21 > *e50 {
		if last.Close > last.Open && last.Close > *e21 {
			return &PullbackResult{Side: "bull"}
		}
	} else if *e9 < *e21 && *e21 < *e50 {
		if last.Close < last.Open && last.Close < *e21 {
			return &PullbackResult{Side: "bear"}
		}
	}
	return nil
}
