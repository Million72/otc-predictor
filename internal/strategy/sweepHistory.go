package strategy

import "otc-predictor/pkg/types"

// SweepEvent represents one historical liquidity sweep found in a window
// of candles (as opposed to LiquiditySweep, which only reports the single
// most recent one).
type SweepEvent struct {
	Index int
	Side  string
	Level float64
}

// DetectAllSweeps scans a window of recent candles and returns every
// instance where a candle wicked beyond a PRIOR swing extreme and closed
// back inside it.
func DetectAllSweeps(candles []types.Candle, lookback int) []SweepEvent {
	if len(candles) < lookback+10 {
		return nil
	}
	start := len(candles) - lookback
	sweeps := []SweepEvent{}

	for i := start; i < len(candles); i++ {
		priorStart := i - 30
		if priorStart < 0 {
			priorStart = 0
		}
		priorSlice := candles[priorStart:i]
		if len(priorSlice) < 10 {
			continue
		}

		highs := swingHighs(priorSlice, 3)
		lows := swingLows(priorSlice, 3)
		if len(highs) == 0 || len(lows) == 0 {
			continue
		}

		recentHigh := highs[0].Price
		for _, h := range highs {
			if h.Price > recentHigh {
				recentHigh = h.Price
			}
		}
		recentLow := lows[0].Price
		for _, l := range lows {
			if l.Price < recentLow {
				recentLow = l.Price
			}
		}

		c := candles[i]
		if c.Low < recentLow && c.Close > recentLow {
			sweeps = append(sweeps, SweepEvent{Index: i, Side: "bull", Level: recentLow})
		} else if c.High > recentHigh && c.Close < recentHigh {
			sweeps = append(sweeps, SweepEvent{Index: i, Side: "bear", Level: recentHigh})
		}
	}
	return sweeps
}

// TurtleSoupResult mirrors SweepResult's shape for consistency.
type TurtleSoupResult struct {
	Side  string
	Level float64
	Label string
}

// DetectTurtleSoup: a stricter sweep variant — price sweeps beyond the
// genuine extreme of a full N-period range (classically 20, the origin of
// the "Turtle Soup" name), then reverses.
func DetectTurtleSoup(candles []types.Candle, periods int) *TurtleSoupResult {
	if len(candles) < periods+5 {
		return nil
	}
	last := candles[len(candles)-1]
	rangeSlice := candles[len(candles)-(periods+1) : len(candles)-1]

	rangeHigh := rangeSlice[0].High
	rangeLow := rangeSlice[0].Low
	for _, c := range rangeSlice {
		if c.High > rangeHigh {
			rangeHigh = c.High
		}
		if c.Low < rangeLow {
			rangeLow = c.Low
		}
	}

	if last.Low < rangeLow && last.Close > rangeLow {
		return &TurtleSoupResult{Side: "bull", Level: rangeLow, Label: "Turtle Soup — swept range low, reversed"}
	}
	if last.High > rangeHigh && last.Close < rangeHigh {
		return &TurtleSoupResult{Side: "bear", Level: rangeHigh, Label: "Turtle Soup — swept range high, reversed"}
	}
	return nil
}

// DoubleSweepResult mirrors the JS shape.
type DoubleSweepResult struct {
	Side  string
	Label string
	Index int // index of the LATER of the two sweeps — lets callers check recency
}

// DetectDoubleSweep: two sweeps on the SAME side occurring close together
// in recent history.
func DetectDoubleSweep(candles []types.Candle, maxGap int) *DoubleSweepResult {
	sweeps := DetectAllSweeps(candles, 50)
	if len(sweeps) < 2 {
		return nil
	}
	last := sweeps[len(sweeps)-1]
	prev := sweeps[len(sweeps)-2]

	gap := last.Index - prev.Index
	if last.Side == prev.Side && gap <= maxGap && gap >= 2 {
		return &DoubleSweepResult{Side: last.Side, Label: "Double Liquidity Sweep — two sweeps close together", Index: last.Index}
	}
	return nil
}
