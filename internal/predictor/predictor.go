package predictor

import (
	"time"

	"otc-predictor/internal/strategy"
	"otc-predictor/pkg/types"
)

// BuildSignal runs the engine, validates it, and returns the final Signal for storage/API.
// partnerCandles is only ever non-nil for forex symbols with a defined SMT
// partner (currently EURUSD/GBPUSD) — every other market gets nil.
func BuildSignal(market types.Market, tf string, candles, htf1, htf2, partnerCandles []types.Candle) types.Signal {
	if len(candles) < 20 {
		return types.Signal{
			Symbol: market.Symbol, Type: market.Type, Timeframe: tf,
			Signal: "WAIT", Timestamp: time.Now(), Error: "Not enough candle data",
		}
	}

	result := strategy.RunEngine(market, candles, htf1, htf2, partnerCandles)
	validation := ValidateSignal(result)

	dec := 5
	if market.IsGold {
		dec = 2
	} else if market.IsJPY {
		dec = 3
	} else if market.Type == "synthetic" && result.Price > 999 {
		dec = 2
	} else if market.Type == "synthetic" {
		dec = 3
	}

	price := round(result.Price, dec)

	sig := types.Signal{
		Symbol:    market.Symbol,
		Type:      market.Type,
		Timeframe: tf,
		Price:     price,
		BullScore: result.BullScore,
		BearScore: result.BearScore,
		MaxScore:  result.MaxScore,
		RSI:       round(result.RSI, 1),
		MACDBull:  result.MACDBull,
		Trend:     result.Trend,
		Structure: result.Structure,
		HTF1Bias:  result.HTF1Bias,
		HTF2Bias:  result.HTF2Bias,
		Timestamp: time.Now(),
	}

	// Map factors
	for _, f := range result.Factors {
		sig.Factors = append(sig.Factors, types.Factor{
			Step: f.Step, Label: f.Label, Side: f.Side, Weight: f.Weight,
		})
	}

	// Map entry models — surfaced regardless of validation outcome, so the
	// API/dashboard can show what WAS detected even when a signal is
	// ultimately blocked for some other reason (e.g. confidence floor).
	for _, m := range result.EntryModels {
		sig.EntryModels = append(sig.EntryModels, types.EntryModel{
			Model: m.Model, Side: m.Side, Weight: m.Weight, Label: m.Label,
		})
	}

	bullConf := clampInt(int(result.BullScore / result.MaxScore * 100))
	bearConf := clampInt(int(result.BearScore / result.MaxScore * 100))

	if !validation.Valid {
		sig.Signal = "WAIT"
		sig.BlockReason = validation.Reason
		if validation.Side == "bull" {
			sig.Confidence = bullConf
		} else if validation.Side == "bear" {
			sig.Confidence = bearConf
		} else {
			sig.Confidence = maxInt(bullConf, bearConf)
		}
		return sig
	}

	side := validation.Side

	// Confirmation candle
	last := candles[len(candles)-1]
	confirmed := ConfirmationCandle([]struct{ Open, Close, High, Low float64 }{
		{last.Open, last.Close, last.High, last.Low},
	}, side)

	if !confirmed {
		sig.Signal = "WAIT"
		sig.BlockReason = "No confirmation candle"
		if side == "bull" {
			sig.Confidence = bullConf
		} else {
			sig.Confidence = bearConf
		}
		return sig
	}

	if result.ATR == nil {
		sig.Signal = "WAIT"
		sig.BlockReason = "ATR unavailable"
		sig.Confidence = bullConf
		return sig
	}

	isSynthetic := market.Type == "synthetic"
	levels := CalculateTPSL(side, result.Price, *result.ATR, isSynthetic, market.IsJPY, market.IsGold)
	rr := RiskReward(result.Price, levels.TP1, levels.SL)

	tp1 := round(levels.TP1, dec)
	tp2 := round(levels.TP2, dec)
	sl := round(levels.SL, dec)
	pips := round(levels.Pips, 1)
	atrVal := round(*result.ATR, dec+1)
	rrVal := round(rr, 2)

	sig.TP1 = &tp1
	sig.TP2 = &tp2
	sig.SL = &sl
	sig.Pips = &pips
	sig.ATR = &atrVal
	sig.RR = &rrVal

	if side == "bull" {
		sig.Signal = "BUY"
		sig.Confidence = bullConf
	} else {
		sig.Signal = "SELL"
		sig.Confidence = bearConf
	}

	// Counter-trend flag (forex only, using EMA trend bias)
	if market.Type == "forex" {
		sig.CounterTrend = (result.Trend == "BULLISH" && sig.Signal == "SELL") ||
			(result.Trend == "BEARISH" && sig.Signal == "BUY")
	}

	return sig
}

func round(v float64, dp int) float64 {
	mult := 1.0
	for i := 0; i < dp; i++ {
		mult *= 10
	}
	return float64(int(v*mult+sign(v)*0.5)) / mult
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

func clampInt(v int) int {
	if v > 100 {
		return 100
	}
	if v < 0 {
		return 0
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
