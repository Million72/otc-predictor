package strategy

import "otc-predictor/pkg/types"

// ExecutionPlan separates setup detection from execution. A validated setup
// can be ARMED before price reaches the preferred entry zone, which prevents
// the scanner from chasing an already-extended move.
type ExecutionPlan struct {
	State          string // NONE | ARMED | TRIGGERED | MISSED
	Side           string // bull | bear
	Mode           string // LIMIT_PULLBACK | MARKET
	ZoneType       string
	ZoneLow        float64
	ZoneHigh       float64
	IdealEntry     float64
	DistanceATR    float64
	MaxDistanceATR float64
	Reason         string
}

// FindExecutionPlan selects a fresh FVG/OB zone close enough to price to be
// tradable. The current price is never used as the preferred entry when a
// better structural zone exists. If price has run too far from the zone, the
// setup is explicitly MISSED instead of becoming a chase signal.
func FindExecutionPlan(candles []types.Candle, side string, atr *float64, maxDistanceATR float64) ExecutionPlan {
	if maxDistanceATR <= 0 {
		maxDistanceATR = 0.75
	}
	plan := ExecutionPlan{State: "NONE", Side: side, MaxDistanceATR: maxDistanceATR}
	if len(candles) < 20 || atr == nil || *atr <= 0 {
		plan.Reason = "ATR or candle history unavailable"
		return plan
	}

	current := candles[len(candles)-1].Close
	zones := append(DetectFVGs(candles), DetectOrderBlocks(candles)...)
	bestDistance := 1e100
	var best Zone
	found := false

	for _, z := range zones {
		if z.Side != side || z.Index < len(candles)-15 || z.Index >= len(candles) {
			continue
		}
		if z.Top <= z.Bottom || z.Top-z.Bottom > *atr*1.25 {
			continue
		}
		if !zoneStillValid(candles, z) {
			continue
		}

		mid := (z.Top + z.Bottom) / 2
		var distance float64
		if side == "bull" {
			if current < z.Bottom {
				continue // price already traded through the buy zone
			}
			distance = current - mid
		} else {
			if current > z.Top {
				continue // price already traded through the sell zone
			}
			distance = mid - current
		}
		if distance < 0 {
			distance = 0
		}
		atrDistance := distance / *atr
		if atrDistance < bestDistance {
			bestDistance = atrDistance
			best = z
			found = true
		}
	}

	if !found {
		plan.State = "MISSED"
		plan.Reason = "No fresh actionable FVG/Order Block remains between price and a valid entry"
		return plan
	}

	plan.ZoneType = best.Type
	plan.ZoneLow = best.Bottom
	plan.ZoneHigh = best.Top
	plan.IdealEntry = (best.Top + best.Bottom) / 2
	plan.DistanceATR = bestDistance

	if bestDistance > maxDistanceATR {
		plan.State = "MISSED"
		plan.Reason = "Price is too far from the structural entry zone — do not chase"
		return plan
	}

	inside := current >= best.Bottom && current <= best.Top
	if inside {
		plan.State = "TRIGGERED"
		plan.Mode = "MARKET"
		plan.Reason = "Price is inside the validated structural entry zone"
		return plan
	}

	plan.State = "ARMED"
	plan.Mode = "LIMIT_PULLBACK"
	plan.Reason = "Setup confirmed early; wait for price to return to the entry zone"
	return plan
}

func zoneStillValid(candles []types.Candle, z Zone) bool {
	for i := z.Index + 1; i < len(candles); i++ {
		c := candles[i]
		if z.Side == "bull" && c.Close < z.Bottom {
			return false
		}
		if z.Side == "bear" && c.Close > z.Top {
			return false
		}
	}
	return true
}
