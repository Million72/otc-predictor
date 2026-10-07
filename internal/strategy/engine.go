package strategy

import (
	"otc-predictor/internal/indicators"
	"otc-predictor/pkg/types"
)

type Factor struct {
	Step   string
	Label  string
	Side   string
	Weight float64
}

type EngineResult struct {
	BullScore    float64
	BearScore    float64
	MaxScore     float64
	Factors      []Factor
	RSI          float64
	MACDBull     bool
	Trend        string // BULLISH | BEARISH | NEUTRAL
	Structure    string
	HTF1Bias     string
	HTF2Bias     string
	ATR          *float64
	Price        float64
	ADXValue     float64
	VolatilityOK bool // true when ATR is healthy relative to recent range — false means dead/illiquid conditions
	EntryModels  []EntryModelMatch
}

// MaxScore: 34 base (after removing the entry-model double-count — see git
// history) + 4 for Retest (2) and Pullback (2) + 1 more because Chart
// Pattern can now score up to 4 (Triple Top/Bottom, Head & Shoulders) where
// it previously maxed at 3 (Double Top/Bottom only).
const MaxScore = 39.0

func biasFromCandles(candles []types.Candle) string {
	if len(candles) < 50 {
		return "NEUTRAL"
	}
	closes := closesOf(candles)
	e9 := indicators.EMA(closes, 9)
	e21 := indicators.EMA(closes, 21)
	e50 := indicators.EMA(closes, 50)
	if e9 == nil || e21 == nil || e50 == nil {
		return "NEUTRAL"
	}
	if *e9 > *e21 && *e21 > *e50 {
		return "BULL"
	}
	if *e9 < *e21 && *e21 < *e50 {
		return "BEAR"
	}
	return "NEUTRAL"
}

// isVolatilityHealthy compares the current ATR against the recent 20-candle
// average true range. If ATR is well below that average, the market is
// effectively dead/illiquid right now and any signal is less trustworthy.
func isVolatilityHealthy(candles []types.Candle, atr *float64) bool {
	if atr == nil || len(candles) < 20 {
		return true // not enough data to judge — don't penalize unfairly
	}
	recent := candles[len(candles)-20:]
	sum := 0.0
	for _, c := range recent {
		sum += c.High - c.Low
	}
	avgRange := sum / float64(len(recent))
	if avgRange == 0 {
		return true
	}
	return *atr >= avgRange*0.3
}

// RunEngine executes the full decision flow for a market and returns the scored result.
// partnerCandles is only ever non-nil for forex symbols with a defined SMT
// partner (currently EURUSD/GBPUSD) — every other market gets nil and SMT
// (and entry models 3/4, which require SMT) simply don't apply to it.
func RunEngine(market types.Market, candles, htf1, htf2, partnerCandles []types.Candle) EngineResult {
	closes := closesOf(candles)
	price := closes[len(closes)-1]

	dec := 5
	if market.IsGold {
		dec = 2
	} else if market.IsJPY {
		dec = 3
	} else if market.Type == "synthetic" && price > 999 {
		dec = 2
	} else if market.Type == "synthetic" {
		dec = 3
	}

	e9 := indicators.EMA(closes, 9)
	e21 := indicators.EMA(closes, 21)
	e50 := indicators.EMA(closes, 50)
	e200 := indicators.EMA(closes, 200)
	rsi := indicators.RSI(closes, 14)
	_, _, macdHist := indicators.MACD(closes)
	bb := indicators.Bollinger(closes, 20)
	atr := indicators.ATR(candles, 14)
	structure := MarketStructure(candles)
	sr := SupportResistance(candles)
	sweep := LiquiditySweep(candles)
	bos := BOS(candles, structure)
	choch := CHoCH(candles, structure)
	pa := CandlestickPatterns(candles)
	cp := ChartPatterns(candles, dec)
	adx := indicators.ADX(candles, 14)
	st := indicators.Supertrend(candles, 10, 3)
	volOK := isVolatilityHealthy(candles, atr)

	// htf2Bias (the true HIGHEST timeframe) is computed here, ahead of entry
	// model scanning, since entry models are gated by it (HTF sets bias
	// only — entry models are only scanned for the matching side; see
	// entryModels.go's cascade documentation).
	htf2Bias := biasFromCandles(htf2)

	// MSS — same underlying detector as CHoCH, aliased per ICT terminology.
	mss := DetectMSS(candles, structure)

	// SMT Divergence — only meaningful when a correlated partner's candles
	// were actually supplied.
	var smt *SMTResult
	if partnerCandles != nil {
		smt = DetectSMTDivergence(candles, partnerCandles)
	}

	// Entry Models — scanned on THIS (LTF/selected) timeframe's candles
	// only, gated by htf2Bias. See entryModels.go for full cascade docs.
	entryModelMatches := CheckEntryModels(candles, sweep, mss, smt, structure, htf2Bias)

	factors := []Factor{}
	bull, bear := 0.0, 0.0

	add := func(step, label, side string, weight float64) {
		if side == "bull" {
			bull += weight
		} else if side == "bear" {
			bear += weight
		}
		factors = append(factors, Factor{Step: step, Label: label, Side: side, Weight: weight})
	}

	// Trend bias from EMA stack
	trendBias := "NEUTRAL"
	if e9 != nil && e21 != nil && e50 != nil {
		if *e9 > *e21 && *e21 > *e50 {
			trendBias = "BULLISH"
			add("EMA Stack", "EMA Stack Bullish 9>21>50", "bull", 3)
		} else if *e9 < *e21 && *e21 < *e50 {
			trendBias = "BEARISH"
			add("EMA Stack", "EMA Stack Bearish 9<21<50", "bear", 3)
		} else {
			add("EMA Stack", "EMA Stack Mixed", "neutral", 0)
		}
	}

	if e21 != nil {
		if price > *e21 {
			add("Price vs EMA21", "Price above EMA21", "bull", 1)
		} else {
			add("Price vs EMA21", "Price below EMA21", "bear", 1)
		}
	}
	if e200 != nil {
		if price > *e200 {
			add("EMA200", "Above EMA200 — long-term bullish", "bull", 1)
		} else {
			add("EMA200", "Below EMA200 — long-term bearish", "bear", 1)
		}
	}

	// RSI
	switch {
	case rsi > 55 && rsi < 70:
		add("RSI", "RSI bullish momentum", "bull", 2)
	case rsi < 45 && rsi > 30:
		add("RSI", "RSI bearish momentum", "bear", 2)
	case rsi >= 70:
		add("RSI", "RSI overbought", "bear", 1)
	case rsi <= 30:
		add("RSI", "RSI oversold", "bull", 1)
	default:
		add("RSI", "RSI neutral", "neutral", 0)
	}

	// MACD
	macdBull := macdHist > 0
	if macdBull {
		add("MACD", "MACD histogram positive", "bull", 2)
	} else {
		add("MACD", "MACD histogram negative", "bear", 2)
	}

	// Bollinger Bands
	if bb.Valid {
		if price < bb.Lower {
			add("Bollinger", "Below Lower BB — oversold", "bull", 2)
		} else if price > bb.Upper {
			add("Bollinger", "Above Upper BB — overbought", "bear", 2)
		} else if price > bb.Mid {
			add("Bollinger", "Above BB midline", "bull", 1)
		} else {
			add("Bollinger", "Below BB midline", "bear", 1)
		}
	}

	// Support/Resistance
	if sr.NearSupport {
		add("Support", "Near support", "bull", 2)
	}
	if sr.NearResistance {
		add("Resistance", "Near resistance", "bear", 2)
	}

	// Market Structure
	add("Market Structure", "Structure: "+structure.Bias, mapBias(structure.Bias), 2)

	// Liquidity Sweep
	if sweep != nil {
		add("Liquidity Sweep", "Liquidity sweep detected", sweep.Side, 3)
	}
	// BOS
	if bos != nil {
		add("BOS", "Break of structure", bos.Side, 3)
	}
	// CHoCH
	if choch != nil {
		add("CHoCH", "Change of character", choch.Side, 3)
	}

	// Sweep + BOS confluence bonus — a liquidity grab immediately followed by
	// a structural break in the SAME direction is a materially stronger signal
	// than either alone, so it earns an extra point on top of their individual scores.
	if sweep != nil && bos != nil && sweep.Side == bos.Side {
		add("Sweep+BOS Confluence", "Liquidity sweep confirmed by BOS in same direction", sweep.Side, 2)
	}

	// Retest — did price come back to confirm a broken level held?
	if retest := DetectRetest(candles, structure); retest != nil {
		add("Retest", "Retest of broken level held", retest.Side, 2)
	}

	// Pullback — classic trend-continuation: pulled back to EMA21, resuming.
	if pullback := DetectPullback(candles); pullback != nil {
		add("Pullback", "Pullback to EMA21, trend resuming", pullback.Side, 2)
	}

	// Entry Models are NOT scored here — they are a mandatory pass/fail gate
	// (validator.go requires at least one match on the winning side, full
	// stop). Scoring them on top of that gate double-counted the same fact.
	// The matches themselves are still returned below (EntryModels field) so
	// the gate and the dashboard's "entry model" display both still work.

	// Candlestick patterns
	for _, p := range pa {
		add("Candlestick", p.Name, p.Side, p.Strength)
	}
	// Chart patterns
	for _, p := range cp {
		add("Chart Pattern", p.Name, p.Side, p.Strength)
	}

	// SuperTrend
	if st.Direction == 1 {
		add("SuperTrend", "SuperTrend bullish", "bull", 2)
	} else if st.Direction == -1 {
		add("SuperTrend", "SuperTrend bearish", "bear", 2)
	}

	// ADX — now actually gates the score instead of just logging.
	// Strong trend (ADX > 25) earns a bonus in whichever direction DI confirms.
	// Weak trend (ADX < 20) applies a penalty to BOTH sides, since a ranging
	// market makes any directional call less trustworthy regardless of what
	// the other indicators say.
	switch {
	case adx.ADX >= 25 && adx.PlusDI > adx.MinusDI:
		add("ADX", "ADX strong trend confirmed bullish", "bull", 2)
	case adx.ADX >= 25 && adx.MinusDI > adx.PlusDI:
		add("ADX", "ADX strong trend confirmed bearish", "bear", 2)
	case adx.ADX < 20:
		// Applied as a symmetric penalty so it reduces whichever side is
		// currently leading, without artificially inflating the other.
		penalty := 3.0
		if bull > bear {
			bull -= penalty
			if bull < 0 {
				bull = 0
			}
		} else if bear > bull {
			bear -= penalty
			if bear < 0 {
				bear = 0
			}
		}
		factors = append(factors, Factor{Step: "ADX", Label: "ADX weak trend — ranging market, confidence reduced", Side: "neutral", Weight: -penalty})
	default:
		add("ADX", "ADX moderate — no strong trend confirmation", "neutral", 0)
	}

	// Volatility health — dead/illiquid conditions reduce confidence in
	// whichever side is currently leading, mirroring the ADX penalty logic.
	if !volOK {
		penalty := 2.0
		if bull > bear {
			bull -= penalty
			if bull < 0 {
				bull = 0
			}
		} else if bear > bull {
			bear -= penalty
			if bear < 0 {
				bear = 0
			}
		}
		factors = append(factors, Factor{Step: "Volatility", Label: "ATR below healthy range — low liquidity, confidence reduced", Side: "neutral", Weight: -penalty})
	}

	htf1Bias := biasFromCandles(htf1)
	if htf1Bias == "BULL" {
		add("HTF1", "HTF1 bullish", "bull", 2)
	} else if htf1Bias == "BEAR" {
		add("HTF1", "HTF1 bearish", "bear", 2)
	}

	return EngineResult{
		BullScore:    bull,
		BearScore:    bear,
		MaxScore:     MaxScore,
		Factors:      factors,
		RSI:          rsi,
		MACDBull:     macdBull,
		Trend:        trendBias,
		Structure:    structure.Bias,
		HTF1Bias:     htf1Bias,
		HTF2Bias:     htf2Bias,
		ATR:          atr,
		Price:        price,
		ADXValue:     adx.ADX,
		VolatilityOK: volOK,
		EntryModels:  entryModelMatches,
	}
}

func mapBias(bias string) string {
	if bias == "BULLISH" {
		return "bull"
	}
	if bias == "BEARISH" {
		return "bear"
	}
	return "neutral"
}
