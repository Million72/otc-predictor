package strategy

import "otc-predictor/pkg/types"

// CISDResult mirrors SweepResult's shape.
type CISDResult struct {
	Side  string
	Level float64
	Label string
}

// DetectCISD (Change in State of Delivery) is a STRICTER variant of
// MSS/CHoCH, not an alias. Where MSS/CHoCH only requires the candle's
// CLOSE to break a prior swing level, CISD requires:
//  1. A full-body close beyond the level (the candle's OPEN must already
//     be on the correct side too)
//  2. Confirmation from the very next candle continuing in the same direction
//
// This deliberately fires less often than MSS/CHoCH alone — the point of a
// separate name and detector, rather than silently relabeling the same
// signal a third time.
func DetectCISD(candles []types.Candle, structure StructureResult) *CISDResult {
	if structure.LastHigh == nil || structure.LastLow == nil || len(candles) < 4 {
		return nil
	}

	candidate := candles[len(candles)-2]
	confirm := candles[len(candles)-1]

	if structure.Bias == "BEARISH" {
		recentHigh := structure.LastHigh.Price
		fullBodyBreak := candidate.Open > recentHigh && candidate.Close > recentHigh
		confirmed := confirm.Close > candidate.Close
		if fullBodyBreak && confirmed {
			return &CISDResult{Side: "bull", Level: recentHigh, Label: "CISD — Bullish delivery shift confirmed"}
		}
	}
	if structure.Bias == "BULLISH" {
		recentLow := structure.LastLow.Price
		fullBodyBreak := candidate.Open < recentLow && candidate.Close < recentLow
		confirmed := confirm.Close < candidate.Close
		if fullBodyBreak && confirmed {
			return &CISDResult{Side: "bear", Level: recentLow, Label: "CISD — Bearish delivery shift confirmed"}
		}
	}
	return nil
}
