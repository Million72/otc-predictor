package spike

import (
	"math"
	"otc-predictor/pkg/types"
)

// SpikeEvent represents one detected historical spike in the tick/candle stream.
type SpikeEvent struct {
	Index int     // position in the candle slice where the spike occurred
	Time  int64   // epoch ms of the spike candle
	Move  float64 // magnitude of the spike move (absolute price change)
}

// SpikeDirection tells us which way this instrument's guaranteed spike moves.
// Boom indices spike UP (price jumps up sharply then the "boom" resets).
// Crash indices spike DOWN (price crashes sharply then resets).
type SpikeDirection string

const (
	SpikeUp   SpikeDirection = "UP"
	SpikeDown SpikeDirection = "DOWN"
)

// DirectionForSymbol returns the expected spike direction based on the instrument name.
// This is structural, not statistical — Boom indices are defined to spike up,
// Crash indices are defined to spike down. Getting this wrong would invalidate
// every downstream calculation, so it's kept explicit and simple.
func DirectionForSymbol(symbol string) SpikeDirection {
	if ContainsFold(symbol, "boom") {
		return SpikeUp
	}
	return SpikeDown
}

// ContainsFold reports whether substr appears in s, case-insensitively.
// Exported so other packages (e.g. the scanner, for routing Boom/Crash
// symbols to this engine) can reuse the exact same matching logic rather
// than re-implementing it and risking the two definitions drifting apart.
func ContainsFold(s, substr string) bool {
	sl := toLower(s)
	subl := toLower(substr)
	return indexOf(sl, subl) >= 0
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// DetectSpikes scans a candle history and flags candles whose single-candle
// move is a statistical outlier relative to the instrument's typical
// candle-to-candle movement. This is how we build the historical spike
// timeline needed for survival analysis — we are NOT trying to predict
// spikes here, only honestly identifying where past spikes occurred so we
// can measure the time gaps between them.
//
// threshold: number of standard deviations above the mean absolute move
// required for a candle to count as a "spike" candle. 4.0 is a reasonably
// conservative default — Boom/Crash spikes are typically extreme relative
// to the constant low-volatility drift between them.
func DetectSpikes(candles []types.Candle, direction SpikeDirection, threshold float64) []SpikeEvent {
	if len(candles) < 30 {
		return nil
	}

	moves := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		move := candles[i].Close - candles[i-1].Close
		if direction == SpikeUp {
			moves = append(moves, move) // signed — we want large positive moves
		} else {
			moves = append(moves, -move) // flip sign so "large positive" = large downward move
		}
	}

	mean, std := meanStd(moves)
	if std == 0 {
		return nil
	}

	events := []SpikeEvent{}
	for i, m := range moves {
		z := (m - mean) / std
		if z >= threshold {
			candleIdx := i + 1
			events = append(events, SpikeEvent{
				Index: candleIdx,
				Time:  candles[candleIdx].Time,
				Move:  math.Abs(candles[candleIdx].Close - candles[candleIdx-1].Close),
			})
		}
	}
	return events
}

func meanStd(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))

	sqSum := 0.0
	for _, v := range vals {
		d := v - mean
		sqSum += d * d
	}
	variance := sqSum / float64(len(vals))
	return mean, math.Sqrt(variance)
}
