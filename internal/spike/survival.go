package spike

import "math"

// SurvivalModel estimates the probability that "time until next spike" follows
// an exponential distribution — the correct model IF spikes occur with roughly
// constant per-tick/per-candle probability, independent of how long it's been
// since the last one (a memoryless process). This matches Deriv's documented
// design for Boom/Crash: a fixed probability per tick of the guaranteed spike.
//
// We are NOT claiming to predict the exact next spike. We ARE honestly
// estimating: "given it's been N candles since the last spike, and spikes
// historically occur roughly every λ candles, what's the probability of a
// spike in the next K candles?" That's real, well-defined survival analysis
// (specifically: an exponential hazard model), not pattern-matching fiction.
type SurvivalModel struct {
	MeanGapCandles float64 // average number of candles between historical spikes
	Lambda         float64 // hazard rate = 1 / MeanGapCandles (spikes per candle)
	SampleCount    int     // how many historical gaps this was estimated from
	Reliable       bool    // true only when we have enough historical spikes to trust the estimate
}

const minGapSamplesForReliability = 8

// FitSurvivalModel computes the mean gap between consecutive spike events
// and derives the exponential hazard rate. More samples = more reliable —
// we refuse to claim reliability below a minimum sample count, since a
// gap estimate from only 2-3 historical spikes is not trustworthy.
func FitSurvivalModel(events []SpikeEvent) SurvivalModel {
	if len(events) < 2 {
		return SurvivalModel{Reliable: false}
	}

	gaps := make([]float64, 0, len(events)-1)
	for i := 1; i < len(events); i++ {
		gap := float64(events[i].Index - events[i-1].Index)
		if gap > 0 {
			gaps = append(gaps, gap)
		}
	}
	if len(gaps) == 0 {
		return SurvivalModel{Reliable: false}
	}

	sum := 0.0
	for _, g := range gaps {
		sum += g
	}
	meanGap := sum / float64(len(gaps))
	if meanGap <= 0 {
		return SurvivalModel{Reliable: false}
	}

	return SurvivalModel{
		MeanGapCandles: meanGap,
		Lambda:         1.0 / meanGap,
		SampleCount:    len(gaps),
		Reliable:       len(gaps) >= minGapSamplesForReliability,
	}
}

// ProbabilityOfSpikeWithin returns P(spike occurs within the next k candles),
// given the exponential hazard model. This is the standard CDF for an
// exponential distribution: P(T <= k) = 1 - e^(-λk).
//
// Important honesty note: because the exponential distribution is
// memoryless, this probability does NOT increase just because it's "been a
// while" since the last spike — that would be the gambler's fallacy, and
// this model deliberately does not fall into it. The probability of a spike
// in the next k candles is the same regardless of how long we've already
// waited, UNLESS the underlying process is proven non-memoryless (see
// OverdueSignal below for a way to honestly check that assumption).
func (m SurvivalModel) ProbabilityOfSpikeWithin(k int) float64 {
	if !m.Reliable || k <= 0 {
		return 0
	}
	return 1 - math.Exp(-m.Lambda*float64(k))
}

// OverdueSignal checks whether the current gap since the last spike is
// unusually long compared to the historical distribution — NOT to predict
// an imminent spike (memorylessness means we can't), but as an honest data
// quality flag: if the current wait is, say, 3x the historical mean gap,
// that's worth surfacing as context, even though it does not mechanically
// raise the true probability of a spike in the next candle.
type OverdueInfo struct {
	CandlesSinceLastSpike int
	RatioToMeanGap        float64 // current gap / historical mean gap
	Unusual               bool    // true if ratio > 2.5 — flagged as context only, not a trading signal by itself
}

func (m SurvivalModel) CheckOverdue(candlesSinceLastSpike int) OverdueInfo {
	if !m.Reliable || m.MeanGapCandles <= 0 {
		return OverdueInfo{CandlesSinceLastSpike: candlesSinceLastSpike}
	}
	ratio := float64(candlesSinceLastSpike) / m.MeanGapCandles
	return OverdueInfo{
		CandlesSinceLastSpike: candlesSinceLastSpike,
		RatioToMeanGap:        ratio,
		Unusual:               ratio > 2.5,
	}
}
