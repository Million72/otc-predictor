package spike

import (
	"fmt"
	"time"

	"otc-predictor/pkg/types"
)

// spikeZThreshold controls sensitivity of historical spike detection.
// Higher = only counts more extreme moves as "spikes".
const spikeZThreshold = 4.0

// RunSpikeEngine builds an honest Boom/Crash signal from survival analysis
// and post-spike reaction — NOT from pretending to predict spike timing.
//
// What this legitimately produces:
//  1. A statistically grounded probability that a spike occurs within the
//     next K candles, given the historical gap distribution (exponential
//     hazard model). This is real math, clearly labeled with its own
//     memorylessness caveat.
//  2. A post-spike reaction signal — IF a spike just happened and price is
//     still retracing within the typical reaction window, this surfaces
//     that as a genuinely observable, tradeable condition.
//
// This function deliberately does NOT produce a BUY/SELL "the spike is
// coming" signal, because that claim would misrepresent how these
// instruments are constructed. It only fires a directional signal off the
// POST-SPIKE REACTION, which is the part with real observable structure.
func RunSpikeEngine(market types.Market, candles []types.Candle) types.Signal {
	direction := DirectionForSymbol(market.Symbol)

	events := DetectSpikes(candles, direction, spikeZThreshold)
	model := FitSurvivalModel(events)

	sig := types.Signal{
		Symbol:    market.Symbol,
		Type:      market.Type,
		Timeframe: "spike-analysis",
		Price:     candles[len(candles)-1].Close,
		Timestamp: time.Now(),
		Factors:   []types.Factor{},
	}

	// Always surface the survival model context, even if we don't fire a trade signal.
	if model.Reliable {
		prob10 := model.ProbabilityOfSpikeWithin(10)
		sig.Factors = append(sig.Factors, types.Factor{
			Step:  "Survival Model",
			Label: fmt.Sprintf("Mean gap ~%.0f candles (n=%d) · P(spike within 10 candles) = %.0f%%", model.MeanGapCandles, model.SampleCount, prob10*100),
			Side:  "neutral",
		})
	} else {
		sig.Factors = append(sig.Factors, types.Factor{
			Step:  "Survival Model",
			Label: fmt.Sprintf("Insufficient spike history (%d events) — need at least %d for a reliable estimate", len(events), minGapSamplesForReliability),
			Side:  "neutral",
		})
	}

	if len(events) > 0 {
		lastSpike := events[len(events)-1]
		candlesSince := len(candles) - 1 - lastSpike.Index
		overdue := model.CheckOverdue(candlesSince)
		if overdue.Unusual {
			sig.Factors = append(sig.Factors, types.Factor{
				Step:  "Overdue Context",
				Label: fmt.Sprintf("Current gap (%d candles) is %.1fx the historical mean — context only, does not itself raise next-candle probability", overdue.CandlesSinceLastSpike, overdue.RatioToMeanGap),
				Side:  "neutral",
			})
		}
	}

	// Post-spike reaction — the actually tradeable part.
	reaction := DetectPostSpikeReaction(candles, events, direction)

	if !reaction.SpikeDetected || !reaction.InReactionWindow {
		sig.Signal = "WAIT"
		sig.BlockReason = "No recent spike in reaction window — nothing to trade"
		return sig
	}

	reactionSide := "bull"
	if reaction.RetraceDirection == SpikeDown {
		reactionSide = "bear"
	}
	sig.Factors = append(sig.Factors, types.Factor{
		Step:  "Post-Spike Reaction",
		Label: fmt.Sprintf("Spike %d candles ago, retraced %.0f%% so far", reaction.CandlesSinceSpike, reaction.RetraceSoFar*100),
		Side:  reactionSide,
	})

	// Require a MEANINGFUL retrace already underway before calling it a signal —
	// firing the instant the spike happens (0% retrace) would just be chasing
	// the spike itself, which is exactly the kind of low-conviction, reactive
	// noise we've been removing elsewhere in this system.
	const minRetraceToAct = 0.25
	const maxRetraceToAct = 0.85 // if it's already retraced almost fully, the reaction is over — too late

	if reaction.RetraceSoFar < minRetraceToAct {
		sig.Signal = "WAIT"
		sig.BlockReason = fmt.Sprintf("Retrace only %.0f%% so far — too early, waiting for confirmation", reaction.RetraceSoFar*100)
		return sig
	}
	if reaction.RetraceSoFar > maxRetraceToAct {
		sig.Signal = "WAIT"
		sig.BlockReason = fmt.Sprintf("Retrace already %.0f%% — reaction largely over, too late to act", reaction.RetraceSoFar*100)
		return sig
	}

	// --- Actionable BUY/SELL is INTENTIONALLY DISABLED for Boom/Crash. ---
	//
	// This used to fire BUY/SELL here based on "post-spike reaction" —
	// betting that a retrace after a detected spike would continue. A real
	// trade taken off this logic (Crash 300, see conversation history) lost
	// money: the spike detector operates on candle-close outliers, but a
	// Boom/Crash guaranteed spike is a single-TICK event. On a timeframe
	// like M15, one spike tick gets diluted into a normal-looking candle,
	// while an ordinary multi-candle decline can look identical to "spike
	// then retrace" to this detector — they are not reliably distinguishable
	// at candle resolution. That mismatch isn't a small bug to patch; it's a
	// structural limitation of detecting a tick-level phenomenon from
	// candle-level data, and patching it further here would just be a new
	// unverified guess layered on a symbol that already cost real money.
	//
	// So: no BUY/SELL, no TP/SL, from this engine, until genuine tick-level
	// backtesting exists to validate (or replace) the reaction theory. The
	// survival-model math above (P(spike within 10 candles), overdue
	// context) IS statistically sound and stays visible — it's honest
	// probabilistic context, not a disguised trade call.
	side := "the reaction side"
	if reaction.RetraceDirection == SpikeUp {
		side = "bullish"
	} else {
		side = "bearish"
	}
	sig.Signal = "WAIT"
	sig.BlockReason = fmt.Sprintf(
		"Boom/Crash actionable signals are disabled — post-spike reaction detection isn't validated at candle resolution. (For reference: retrace %.0f%% so far, leaning %s.)",
		reaction.RetraceSoFar*100, side,
	)
	return sig
}
