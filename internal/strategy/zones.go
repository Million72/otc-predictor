package strategy

import "otc-predictor/pkg/types"

// Zone represents any smart-money price zone (FVG, IFVG, Order Block,
// Breaker Block, BPR, Mitigation Block) with a consistent shape so entry
// model checking can treat them uniformly.
type Zone struct {
	Type   string // "FVG" | "IFVG" | "OrderBlock" | "BreakerBlock" | "BPR" | "MitigationBlock"
	Side   string // "bull" | "bear" | "neutral" (BPR only)
	Top    float64
	Bottom float64
	Index  int
}

// ── Fair Value Gap (FVG) ────────────────────────────────────────
// 3-candle imbalance: candle1's high/low doesn't overlap candle3's low/high.
func DetectFVGs(candles []types.Candle) []Zone {
	zones := []Zone{}
	for i := 1; i < len(candles)-1; i++ {
		c1 := candles[i-1]
		c3 := candles[i+1]
		if c1.High < c3.Low {
			zones = append(zones, Zone{Type: "FVG", Side: "bull", Top: c3.Low, Bottom: c1.High, Index: i})
		}
		if c1.Low > c3.High {
			zones = append(zones, Zone{Type: "FVG", Side: "bear", Top: c1.Low, Bottom: c3.High, Index: i})
		}
	}
	return zones
}

// ── Inverse Fair Value Gap (IFVG) ────────────────────────────────
// An FVG that price has traded through and closed beyond, flipping its
// expected role.
func DetectIFVGs(candles []types.Candle) []Zone {
	fvgs := DetectFVGs(candles)
	ifvgs := []Zone{}

	for _, fvg := range fvgs {
		for i := fvg.Index + 1; i < len(candles); i++ {
			c := candles[i]
			if fvg.Side == "bull" && c.Close < fvg.Bottom {
				ifvgs = append(ifvgs, Zone{Type: "IFVG", Side: "bear", Top: fvg.Top, Bottom: fvg.Bottom, Index: i})
				break
			}
			if fvg.Side == "bear" && c.Close > fvg.Top {
				ifvgs = append(ifvgs, Zone{Type: "IFVG", Side: "bull", Top: fvg.Top, Bottom: fvg.Bottom, Index: i})
				break
			}
		}
	}
	return ifvgs
}

// ── Order Block ──────────────────────────────────────────────────
// Last opposing-direction candle immediately before a strong impulsive move.
func DetectOrderBlocks(candles []types.Candle) []Zone {
	zones := []Zone{}
	for i := 1; i < len(candles)-1; i++ {
		prev := candles[i-1]
		curr := candles[i]
		if isBear(prev) && isBull(curr) && body(curr) > body(prev)*1.5 {
			zones = append(zones, Zone{Type: "OrderBlock", Side: "bull", Top: prev.High, Bottom: prev.Low, Index: i - 1})
		}
		if isBull(prev) && isBear(curr) && body(curr) > body(prev)*1.5 {
			zones = append(zones, Zone{Type: "OrderBlock", Side: "bear", Top: prev.High, Bottom: prev.Low, Index: i - 1})
		}
	}
	return zones
}

// ── Breaker Block (BB) ───────────────────────────────────────────
// An Order Block that price later breaks through, flipping its expected role.
func DetectBreakerBlocks(candles []types.Candle) []Zone {
	obs := DetectOrderBlocks(candles)
	breakers := []Zone{}

	for _, ob := range obs {
		for i := ob.Index + 1; i < len(candles); i++ {
			c := candles[i]
			if ob.Side == "bull" && c.Close < ob.Bottom {
				breakers = append(breakers, Zone{Type: "BreakerBlock", Side: "bear", Top: ob.Top, Bottom: ob.Bottom, Index: i})
				break
			}
			if ob.Side == "bear" && c.Close > ob.Top {
				breakers = append(breakers, Zone{Type: "BreakerBlock", Side: "bull", Top: ob.Top, Bottom: ob.Bottom, Index: i})
				break
			}
		}
	}
	return breakers
}

// ── Balanced Price Range (BPR) ───────────────────────────────────
// Overlap between a bullish FVG and a bearish FVG formed close together in time.
func DetectBPRs(candles []types.Candle) []Zone {
	fvgs := DetectFVGs(candles)
	bullFvgs := []Zone{}
	bearFvgs := []Zone{}
	for _, f := range fvgs {
		if f.Side == "bull" {
			bullFvgs = append(bullFvgs, f)
		} else {
			bearFvgs = append(bearFvgs, f)
		}
	}

	bprs := []Zone{}
	for _, bull := range bullFvgs {
		for _, bear := range bearFvgs {
			gap := bull.Index - bear.Index
			if gap < 0 {
				gap = -gap
			}
			if gap > 15 {
				continue
			}
			overlapTop := minF(bull.Top, bear.Top)
			overlapBottom := maxF(bull.Bottom, bear.Bottom)
			if overlapTop > overlapBottom {
				idx := bull.Index
				if bear.Index > idx {
					idx = bear.Index
				}
				bprs = append(bprs, Zone{Type: "BPR", Side: "neutral", Top: overlapTop, Bottom: overlapBottom, Index: idx})
			}
		}
	}
	return bprs
}

// ── Mitigation Block ──────────────────────────────────────────────
// The last candle in the SAME direction as an impulsive move, right before
// that move reverses — a position likely entered late and "mitigated" when
// price returns to it. Side is the REVERSAL direction, not the original move.
func DetectMitigationBlocks(candles []types.Candle) []Zone {
	zones := []Zone{}
	for i := 2; i < len(candles)-1; i++ {
		preImpulse := candles[i-1]
		impulse := candles[i]
		next := candles[i+1]

		if isBull(preImpulse) && isBull(impulse) && body(impulse) > body(preImpulse)*1.3 && isBear(next) {
			zones = append(zones, Zone{Type: "MitigationBlock", Side: "bear", Top: preImpulse.High, Bottom: preImpulse.Low, Index: i - 1})
		}
		if isBear(preImpulse) && isBear(impulse) && body(impulse) > body(preImpulse)*1.3 && isBull(next) {
			zones = append(zones, Zone{Type: "MitigationBlock", Side: "bull", Top: preImpulse.High, Bottom: preImpulse.Low, Index: i - 1})
		}
	}
	return zones
}

// IsZoneRelevant checks a zone is both close enough to current price
// (volatility-scaled distance) to matter right now.
func IsZoneRelevant(candles []types.Candle, zone Zone, maxAtrMultiple float64) bool {
	if len(candles) < 15 {
		return false
	}
	atrWindow := candles[len(candles)-15:]
	atrSum := 0.0
	for i := 1; i < len(atrWindow); i++ {
		c, p := atrWindow[i], atrWindow[i-1]
		atrSum += maxF(c.High-c.Low, maxF(absF(c.High-p.Close), absF(c.Low-p.Close)))
	}
	atr := atrSum / float64(len(atrWindow)-1)
	maxDist := atr * maxAtrMultiple

	last := candles[len(candles)-1]
	zoneMid := (zone.Top + zone.Bottom) / 2
	inZone := last.Low <= zone.Top && last.High >= zone.Bottom
	distance := absF(last.Close - zoneMid)

	return inZone && distance <= maxDist
}
