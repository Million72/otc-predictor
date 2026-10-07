package strategy

import "otc-predictor/pkg/types"

// EntryModelMatch mirrors the JS shape.
type EntryModelMatch struct {
	Model  int
	Side   string
	Weight float64
	Label  string
}

const coherenceWindow = 12

// Entry Models — eleven specific ICT/SMC combos. At least ONE must be
// present for a signal to be allowed to fire at all — this is a REQUIRED
// gate (enforced in predictor/validator.go), not an optional bonus.
//
// HTF → MTF → LTF cascade: HTF (highest timeframe) sets BIAS only — entry
// models are only scanned for the side matching this bias (NEUTRAL allows
// both sides). MTF (middle) contributes to scoring elsewhere, not a hard
// gate here. LTF (the timeframe being analyzed, i.e. `candles` passed in)
// is where entry models are actually scanned.
//
// Each matched model requires its component zones to be RECENT (within
// coherenceWindow candles of the most recent candle) — this prevents
// stitching together unrelated components from different moments in
// history into a false "match," and prevents the same model from firing
// bull AND bear simultaneously from two unrelated stale zones.
//
// Models:
//  1. Liquidity Sweep + MSS + FVG
//  2. Liquidity Sweep + BPR
//  3. SMT + MSS + IFVG
//  4. SMT + MSS + BB (Breaker Block)
//  5. Liquidity Sweep + MSS + BB + FVG
//  6. Turtle Soup + MSS + FVG
//  7. Order Block + MSS + FVG
//  8. CHoCH + Order Block
//  9. Mitigation Block + FVG
//  10. CISD + FVG
//  11. Double Liquidity Sweep
//
// NOTE: OTE (Fibonacci-based) is intentionally NOT included — deferred, no
// Fibonacci math in this codebase yet.
func CheckEntryModels(candles []types.Candle, sweep *SweepResult, mss *SweepResult, smt *SMTResult, structure StructureResult, htfBias string) []EntryModelMatch {
	matches := []EntryModelMatch{}
	n := len(candles)

	fvgs := DetectFVGs(candles)
	ifvgs := DetectIFVGs(candles)
	breakers := DetectBreakerBlocks(candles)
	bprs := DetectBPRs(candles)
	obs := DetectOrderBlocks(candles)
	mitigations := DetectMitigationBlocks(candles)
	turtleSoup := DetectTurtleSoup(candles, 20)
	doubleSweep := DetectDoubleSweep(candles, 15)
	cisd := DetectCISD(candles, structure)

	zonesOfSide := func(zones []Zone, side string) []Zone {
		out := []Zone{}
		for _, z := range zones {
			if z.Side == side && IsZoneRelevant(candles, z, 3) {
				out = append(out, z)
			}
		}
		return out
	}

	mostRecentIndex := func(zones []Zone) int {
		if len(zones) == 0 {
			return -1
		}
		idx := zones[0].Index
		for _, z := range zones {
			if z.Index > idx {
				idx = z.Index
			}
		}
		return idx
	}

	isRecent := func(index int) bool {
		if index < 0 {
			return false
		}
		return (n - 1 - index) <= coherenceWindow
	}

	// Determine eligible side(s) per the HTF bias gate.
	sides := []string{}
	switch htfBias {
	case "BULL":
		sides = []string{"bull"}
	case "BEAR":
		sides = []string{"bear"}
	default:
		sides = []string{"bull", "bear"} // NEUTRAL — no bias to filter by
	}

	for _, side := range sides {
		sweepOk := sweep != nil && sweep.Side == side
		mssOk := mss != nil && mss.Side == side
		smtOk := smt != nil && smt.Side == side

		fvgZones := zonesOfSide(fvgs, side)
		ifvgZones := zonesOfSide(ifvgs, side)
		bbZones := zonesOfSide(breakers, side)
		obZones := zonesOfSide(obs, side)
		mitigationZones := zonesOfSide(mitigations, side)

		fvgOk := len(fvgZones) > 0 && isRecent(mostRecentIndex(fvgZones))
		ifvgOk := len(ifvgZones) > 0 && isRecent(mostRecentIndex(ifvgZones))
		bbOk := len(bbZones) > 0 && isRecent(mostRecentIndex(bbZones))
		obOk := len(obZones) > 0 && isRecent(mostRecentIndex(obZones))
		mitigationOk := len(mitigationZones) > 0 && isRecent(mostRecentIndex(mitigationZones))

		turtleOk := turtleSoup != nil && turtleSoup.Side == side
		doubleOk := doubleSweep != nil && doubleSweep.Side == side && isRecent(doubleSweep.Index)
		cisdOk := cisd != nil && cisd.Side == side

		bprOk := false
		for _, z := range bprs {
			if IsZoneRelevant(candles, z, 3) && isRecent(z.Index) {
				bprOk = true
				break
			}
		}

		if sweepOk && mssOk && fvgOk {
			matches = append(matches, EntryModelMatch{Model: 1, Side: side, Weight: 3, Label: "Entry Model 1: Sweep + MSS + FVG (" + side + ")"})
		}
		if sweepOk && bprOk {
			matches = append(matches, EntryModelMatch{Model: 2, Side: side, Weight: 2, Label: "Entry Model 2: Sweep + BPR (" + side + ")"})
		}
		if smtOk && mssOk && ifvgOk {
			matches = append(matches, EntryModelMatch{Model: 3, Side: side, Weight: 3, Label: "Entry Model 3: SMT + MSS + IFVG (" + side + ")"})
		}
		if smtOk && mssOk && bbOk {
			matches = append(matches, EntryModelMatch{Model: 4, Side: side, Weight: 3, Label: "Entry Model 4: SMT + MSS + BB (" + side + ")"})
		}
		if sweepOk && mssOk && bbOk && fvgOk {
			matches = append(matches, EntryModelMatch{Model: 5, Side: side, Weight: 4, Label: "Entry Model 5: Sweep + MSS + BB + FVG (" + side + ")"})
		}
		if turtleOk && mssOk && fvgOk {
			matches = append(matches, EntryModelMatch{Model: 6, Side: side, Weight: 3, Label: "Entry Model 6: Turtle Soup + MSS + FVG (" + side + ")"})
		}
		if obOk && mssOk && fvgOk {
			matches = append(matches, EntryModelMatch{Model: 7, Side: side, Weight: 3, Label: "Entry Model 7: Order Block + MSS + FVG (" + side + ")"})
		}
		if mssOk && obOk {
			matches = append(matches, EntryModelMatch{Model: 8, Side: side, Weight: 2, Label: "Entry Model 8: CHoCH + Order Block (" + side + ")"})
		}
		if mitigationOk && fvgOk {
			matches = append(matches, EntryModelMatch{Model: 9, Side: side, Weight: 2, Label: "Entry Model 9: Mitigation Block + FVG (" + side + ")"})
		}
		if cisdOk && fvgOk {
			matches = append(matches, EntryModelMatch{Model: 10, Side: side, Weight: 2, Label: "Entry Model 10: CISD + FVG (" + side + ")"})
		}
		if doubleOk {
			matches = append(matches, EntryModelMatch{Model: 11, Side: side, Weight: 2, Label: "Entry Model 11: Double Liquidity Sweep (" + side + ")"})
		}
	}

	return matches
}
