package spike

import (
	"otc-predictor/pkg/types"
)

// PostSpikeReaction describes what's happening in the candles immediately
// following a detected spike — this is the actually-tradeable part of the
// system. The spike itself can't be predicted (memoryless process), but
// price behavior AFTER a spike has genuinely-documented tendencies for these
// synthetic indices: a sharp move followed by partial mean-reversion/retrace
// before the underlying constant-drift process resumes.
type PostSpikeReaction struct {
	SpikeDetected     bool
	CandlesSinceSpike int
	SpikeMove         float64
	RetraceSoFar      float64        // how much of the spike has already reversed, as a fraction (0-1+)
	RetraceDirection  SpikeDirection // direction the retrace is moving (opposite of the spike)
	InReactionWindow  bool           // true if we're still within the typical post-spike reaction window
}

// reactionWindowCandles defines how many candles after a spike we consider
// the "reaction window" — the period where post-spike retracement behavior
// is most relevant. Kept deliberately short since Boom/Crash indices are
// designed to resume their constant drift quickly after the spike resolves.
const reactionWindowCandles = 5

// DetectPostSpikeReaction checks whether the most recent candles fall within
// a reaction window following a detected spike, and measures how much of
// that spike has already retraced. This does not predict anything about the
// future — it honestly describes the current state relative to the last
// known spike, which is the only input a downstream strategy should act on.
func DetectPostSpikeReaction(candles []types.Candle, events []SpikeEvent, direction SpikeDirection) PostSpikeReaction {
	if len(events) == 0 || len(candles) == 0 {
		return PostSpikeReaction{SpikeDetected: false}
	}

	lastSpike := events[len(events)-1]
	candlesSince := len(candles) - 1 - lastSpike.Index
	if candlesSince < 0 {
		return PostSpikeReaction{SpikeDetected: false}
	}

	inWindow := candlesSince <= reactionWindowCandles && candlesSince >= 0

	spikeCandle := candles[lastSpike.Index]
	currentPrice := candles[len(candles)-1].Close

	var retrace float64
	var retraceDir SpikeDirection

	if direction == SpikeUp {
		// Spike moved price up; retrace = how far price has come back down
		// from the spike high, as a fraction of the spike's magnitude.
		if lastSpike.Move > 0 {
			retrace = (spikeCandle.Close - currentPrice) / lastSpike.Move
		}
		retraceDir = SpikeDown
	} else {
		// Spike moved price down; retrace = how far price has come back up.
		if lastSpike.Move > 0 {
			retrace = (currentPrice - spikeCandle.Close) / lastSpike.Move
		}
		retraceDir = SpikeUp
	}

	return PostSpikeReaction{
		SpikeDetected:     true,
		CandlesSinceSpike: candlesSince,
		SpikeMove:         lastSpike.Move,
		RetraceSoFar:      retrace,
		RetraceDirection:  retraceDir,
		InReactionWindow:  inWindow,
	}
}
