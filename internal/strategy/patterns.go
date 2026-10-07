package strategy

import "otc-predictor/pkg/types"

type Pattern struct {
	Name     string
	Side     string // "bull" | "bear" | "neutral"
	Strength float64
}

type candidate struct {
	pattern    Pattern
	confirmIdx int
}

// CandlestickPatterns detects single/multi-candle formations on the last 3 candles.
func CandlestickPatterns(candles []types.Candle) []Pattern {
	if len(candles) < 3 {
		return nil
	}
	pats := []Pattern{}
	n := len(candles)
	c0, c1, c2 := candles[n-1], candles[n-2], candles[n-3]

	if isBear(c1) && isBull(c0) && c0.Open <= c1.Close && c0.Close >= c1.Open {
		pats = append(pats, Pattern{"Bullish Engulfing", "bull", 3})
	}
	if isBull(c1) && isBear(c0) && c0.Open >= c1.Close && c0.Close <= c1.Open {
		pats = append(pats, Pattern{"Bearish Engulfing", "bear", 3})
	}
	if dnWick(c0) > body(c0)*2 && upWick(c0) < body(c0)*0.5 && body(c0) < candleRange(c0)*0.4 {
		pats = append(pats, Pattern{"Hammer", "bull", 2})
	}
	if upWick(c0) > body(c0)*2 && dnWick(c0) < body(c0)*0.5 && body(c0) < candleRange(c0)*0.4 {
		pats = append(pats, Pattern{"Shooting Star", "bear", 2})
	}
	if isBull(c2) && isBull(c1) && isBull(c0) && c1.Close > c2.Close && c0.Close > c1.Close {
		pats = append(pats, Pattern{"Three White Soldiers", "bull", 3})
	}
	if isBear(c2) && isBear(c1) && isBear(c0) && c1.Close < c2.Close && c0.Close < c1.Close {
		pats = append(pats, Pattern{"Three Black Crows", "bear", 3})
	}
	if isBear(c2) && body(c1) < candleRange(c1)*0.3 && isBull(c0) && c0.Close > (c2.Open+c2.Close)/2 {
		pats = append(pats, Pattern{"Morning Star", "bull", 3})
	}
	if isBull(c2) && body(c1) < candleRange(c1)*0.3 && isBear(c0) && c0.Close < (c2.Open+c2.Close)/2 {
		pats = append(pats, Pattern{"Evening Star", "bear", 3})
	}
	return pats
}

// ChartPatterns detects multi-swing formations: Double Top/Bottom, Triple
// Top/Bottom, Head & Shoulders/Inverse H&S, Rising/Falling Wedge, and
// Bull/Bear Flag. At most ONE fires per call — see the mutual-exclusivity
// resolution near the end of this function.
//
// NOTE: this used to also report a crude "BOS Bullish/Bearish" here, but
// structure.go's BOS() already does this properly (checks the close
// actually crossed the level on THIS candle, vs. the prior candle still
// being inside it) and is already scored separately in RunEngine. Keeping
// both double-counted the same break-of-structure fact under two different
// factor names, so it's removed here — BOS now lives only in structure.go.
func ChartPatterns(candles []types.Candle, dec int) []Pattern {
	if len(candles) < 30 {
		return nil
	}
	pats := []Pattern{}
	start := 0
	if len(candles) > 80 {
		start = len(candles) - 80
	}
	slice := candles[start:]
	tol := 0.012

	sH := swingHighs(slice, 3)
	sL := swingLows(slice, 3)
	n := len(slice)
	curr := slice[n-1].Close

	var bearCandidates, bullCandidates []candidate

	// Double Bottom — search from the MOST RECENT swings backward (not oldest
	// forward), so if multiple qualifying pairs exist in the window, the one
	// found is the most recent, not whichever happens to appear first
	// chronologically.
	for a := len(sL) - 2; a >= 0; a-- {
		found := false
		for b := a + 1; b < len(sL); b++ {
			if sL[b].Index-sL[a].Index < 5 {
				continue
			}
			if absF(sL[a].Price-sL[b].Price)/sL[a].Price < tol {
				peakBetween := false
				for _, h := range sH {
					if h.Index > sL[a].Index && h.Index < sL[b].Index {
						peakBetween = true
						break
					}
				}
				if peakBetween {
					bullCandidates = append(bullCandidates, candidate{Pattern{"Double Bottom", "bull", 3}, sL[b].Index})
					found = true
					break
				}
			}
		}
		if found {
			break
		}
	}

	// Double Top — same most-recent-first search.
	for a := len(sH) - 2; a >= 0; a-- {
		found := false
		for b := a + 1; b < len(sH); b++ {
			if sH[b].Index-sH[a].Index < 5 {
				continue
			}
			if absF(sH[a].Price-sH[b].Price)/sH[a].Price < tol {
				troughBetween := false
				for _, l := range sL {
					if l.Index > sH[a].Index && l.Index < sH[b].Index {
						troughBetween = true
						break
					}
				}
				if troughBetween {
					bearCandidates = append(bearCandidates, candidate{Pattern{"Double Top", "bear", 3}, sH[b].Index})
					found = true
					break
				}
			}
		}
		if found {
			break
		}
	}

	// Triple Top / Triple Bottom — deliberately only checks the 3 MOST
	// RECENT swing points (not any historical triple like Double Top/Bottom
	// does), so this can't match some old, structurally dead formation.
	// Weighted higher (4) than Double Top/Bottom (3): three confirmations at
	// a similar level is objectively stronger evidence than two.
	const tripleTol = 0.015
	if len(sH) >= 3 {
		a, b, c := sH[len(sH)-3], sH[len(sH)-2], sH[len(sH)-1]
		if absF(a.Price-b.Price)/a.Price < tripleTol && absF(b.Price-c.Price)/b.Price < tripleTol {
			if lowestInRange(sL, a.Index, b.Index) != nil && lowestInRange(sL, b.Index, c.Index) != nil {
				bearCandidates = append(bearCandidates, candidate{Pattern{"Triple Top", "bear", 4}, c.Index})
			}
		}
	}
	if len(sL) >= 3 {
		a, b, c := sL[len(sL)-3], sL[len(sL)-2], sL[len(sL)-1]
		if absF(a.Price-b.Price)/a.Price < tripleTol && absF(b.Price-c.Price)/b.Price < tripleTol {
			if highestInRange(sH, a.Index, b.Index) != nil && highestInRange(sH, b.Index, c.Index) != nil {
				bullCandidates = append(bullCandidates, candidate{Pattern{"Triple Bottom", "bull", 4}, c.Index})
			}
		}
	}

	// Head & Shoulders / Inverse H&S — also only the 3 most recent swings.
	// Only counted once price has actually broken the neckline on the
	// CURRENT candle (classic TA convention — an unconfirmed H&S is just a
	// shape, not a signal), which conveniently makes recency automatic: the
	// confirming index is always "now" by construction.
	const shoulderTol = 0.025
	if len(sH) >= 3 {
		ls, head, rs := sH[len(sH)-3], sH[len(sH)-2], sH[len(sH)-1]
		if head.Price > ls.Price && head.Price > rs.Price && absF(ls.Price-rs.Price)/ls.Price < shoulderTol {
			troughL := lowestInRange(sL, ls.Index, head.Index)
			troughR := lowestInRange(sL, head.Index, rs.Index)
			if troughL != nil && troughR != nil {
				neckline := (troughL.Price + troughR.Price) / 2
				if curr < neckline {
					bearCandidates = append(bearCandidates, candidate{Pattern{"Head & Shoulders", "bear", 4}, n - 1})
				}
			}
		}
	}
	if len(sL) >= 3 {
		ls, head, rs := sL[len(sL)-3], sL[len(sL)-2], sL[len(sL)-1]
		if head.Price < ls.Price && head.Price < rs.Price && absF(ls.Price-rs.Price)/ls.Price < shoulderTol {
			peakL := highestInRange(sH, ls.Index, head.Index)
			peakR := highestInRange(sH, head.Index, rs.Index)
			if peakL != nil && peakR != nil {
				neckline := (peakL.Price + peakR.Price) / 2
				if curr > neckline {
					bullCandidates = append(bullCandidates, candidate{Pattern{"Inverse Head & Shoulders", "bull", 4}, n - 1})
				}
			}
		}
	}

	// Rising Wedge / Falling Wedge — trendline-based, unlike everything
	// above. Weighted 3: a real pattern, but trendline fits are inherently
	// less precise than exact swing-point comparisons.
	if wedge := detectWedge(slice, sH, sL); wedge != nil {
		if wedge.pattern.Side == "bear" {
			bearCandidates = append(bearCandidates, *wedge)
		} else {
			bullCandidates = append(bullCandidates, *wedge)
		}
	}

	// Bull Flag / Bear Flag — a strong directional move (the pole) followed
	// by tight counter-drift consolidation, confirmed by a breakout
	// continuing the ORIGINAL direction.
	if flag := detectFlag(slice); flag != nil {
		if flag.pattern.Side == "bear" {
			bearCandidates = append(bearCandidates, *flag)
		} else {
			bullCandidates = append(bullCandidates, *flag)
		}
	}

	// Pick the single strongest bear candidate (highest weight, tie-broken
	// by most recent confirmation), and the same for bull.
	var bestBear, bestBull *candidate
	for i := range bearCandidates {
		c := bearCandidates[i]
		if bestBear == nil || c.pattern.Strength > bestBear.pattern.Strength ||
			(c.pattern.Strength == bestBear.pattern.Strength && c.confirmIdx > bestBear.confirmIdx) {
			cc := c
			bestBear = &cc
		}
	}
	for i := range bullCandidates {
		c := bullCandidates[i]
		if bestBull == nil || c.pattern.Strength > bestBull.pattern.Strength ||
			(c.pattern.Strength == bestBull.pattern.Strength && c.confirmIdx > bestBull.confirmIdx) {
			cc := c
			bestBull = &cc
		}
	}

	// Mutually exclusive across the WHOLE category: a window can't be live
	// evidence of both a bullish AND a bearish reversal shape at once —
	// that's a contradiction, not two confirmations. Keep only whichever
	// single best candidate (either side) confirmed more recently.
	switch {
	case bestBear != nil && bestBull != nil:
		if bestBull.confirmIdx >= bestBear.confirmIdx {
			pats = append(pats, bestBull.pattern)
		} else {
			pats = append(pats, bestBear.pattern)
		}
	case bestBear != nil:
		pats = append(pats, bestBear.pattern)
	case bestBull != nil:
		pats = append(pats, bestBull.pattern)
	}

	return pats
}

// lowestInRange returns the lowest-priced swing point whose index is
// strictly between from and to, or nil if there isn't one.
func lowestInRange(points []SwingPoint, from, to int) *SwingPoint {
	var best *SwingPoint
	for _, p := range points {
		if p.Index > from && p.Index < to {
			if best == nil || p.Price < best.Price {
				pp := p
				best = &pp
			}
		}
	}
	return best
}

// highestInRange mirrors lowestInRange for swing highs.
func highestInRange(points []SwingPoint, from, to int) *SwingPoint {
	var best *SwingPoint
	for _, p := range points {
		if p.Index > from && p.Index < to {
			if best == nil || p.Price > best.Price {
				pp := p
				best = &pp
			}
		}
	}
	return best
}

// linregSlope fits a simple least-squares line through the given swing
// points (x = candle index, y = price) and returns its slope and intercept.
// Needs at least 2 points to fit a line.
func linregSlope(points []SwingPoint) (slope, intercept float64, ok bool) {
	n := float64(len(points))
	if n < 2 {
		return 0, 0, false
	}
	var sumX, sumY, sumXY, sumXX float64
	for _, p := range points {
		x := float64(p.Index)
		y := p.Price
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return 0, 0, false
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return slope, intercept, true
}

// detectWedge fits a trendline through the last few swing highs and another
// through the last few swing lows. A genuine wedge requires BOTH lines
// sloping the same direction while CONVERGING (the range narrows over
// time) — that's what separates it from an ordinary trend channel (parallel
// lines, no convergence). Confirmed only once price actually breaks out the
// opposite way from the wedge's own direction — a rising wedge is a bearish
// reversal pattern, a falling wedge bullish, which is often counterintuitive
// to people reading the slope direction alone.
func detectWedge(slice []types.Candle, sH, sL []SwingPoint) *candidate {
	const wedgeLookback = 4
	if len(sH) < wedgeLookback || len(sL) < wedgeLookback {
		return nil
	}
	highs := sH[len(sH)-wedgeLookback:]
	lows := sL[len(sL)-wedgeLookback:]

	hSlope, hIntercept, ok1 := linregSlope(highs)
	lSlope, lIntercept, ok2 := linregSlope(lows)
	if !ok1 || !ok2 {
		return nil
	}

	n := len(slice)
	curr := slice[n-1].Close
	nowX := float64(n - 1)
	upperNow := hSlope*nowX + hIntercept
	lowerNow := lSlope*nowX + lIntercept
	if upperNow <= lowerNow {
		return nil // lines have already crossed — not a coherent wedge shape
	}

	startX := float64(highs[0].Index)
	widthStart := (hSlope*startX + hIntercept) - (lSlope*startX + lIntercept)
	widthNow := upperNow - lowerNow
	if widthStart <= 0 || widthNow >= widthStart*0.75 {
		return nil // not meaningfully converging — just two roughly parallel lines
	}

	if hSlope > 0 && lSlope > 0 && curr < lowerNow {
		return &candidate{Pattern{"Rising Wedge", "bear", 3}, n - 1}
	}
	if hSlope < 0 && lSlope < 0 && curr > upperNow {
		return &candidate{Pattern{"Falling Wedge", "bull", 3}, n - 1}
	}
	return nil
}

// detectFlag looks for a strong directional "flagpole" move, followed by a
// tight consolidation that drifts counter to the pole (the "flag"), and
// confirms only on a breakout that continues the ORIGINAL pole direction —
// a continuation pattern, not a reversal one.
func detectFlag(candles []types.Candle) *candidate {
	const poleLookback = 10    // candles searched for the pole
	const consolidationLen = 5 // most recent candles treated as the flag
	const poleThresholdPct = 0.004

	n := len(candles)
	if n < poleLookback+consolidationLen+1 {
		return nil
	}

	consolidation := candles[n-consolidationLen:]
	poleWindow := candles[n-poleLookback-consolidationLen : n-consolidationLen]

	poleStart := poleWindow[0].Close
	poleEnd := poleWindow[len(poleWindow)-1].Close
	poleMove := poleEnd - poleStart
	if poleMove == 0 || poleStart == 0 {
		return nil
	}
	poleMovePct := poleMove / poleStart

	conHigh, conLow := consolidation[0].High, consolidation[0].Low
	for _, c := range consolidation {
		if c.High > conHigh {
			conHigh = c.High
		}
		if c.Low < conLow {
			conLow = c.Low
		}
	}
	conRange := conHigh - conLow
	if conRange > absF(poleMove)*0.5 {
		return nil // consolidation too wide relative to the pole to call it a flag
	}

	curr := candles[n-1].Close
	if poleMovePct > poleThresholdPct && curr > conHigh {
		return &candidate{Pattern{"Bull Flag", "bull", 3}, n - 1}
	}
	if poleMovePct < -poleThresholdPct && curr < conLow {
		return &candidate{Pattern{"Bear Flag", "bear", 3}, n - 1}
	}
	return nil
}
