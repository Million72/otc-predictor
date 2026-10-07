package predictor

// TPSLResult holds calculated trade levels
type TPSLResult struct {
	TP1  float64
	TP2  float64
	SL   float64
	Pips float64
}

// CalculateTPSL computes TP1/TP2/SL from ATR, tuned per instrument type.
func CalculateTPSL(side string, price, atr float64, isSynthetic, isJPY, isGold bool) TPSLResult {
	slMult, tp1Mult, tp2Mult := 1.5, 2.0, 3.5
	if isSynthetic {
		slMult, tp1Mult, tp2Mult = 1.2, 1.8, 3.0
	}

	pipMult := 10000.0
	if isJPY || isGold {
		pipMult = 100
	} else if isSynthetic {
		pipMult = 1
	}

	if side == "bull" {
		return TPSLResult{
			SL:   price - atr*slMult,
			TP1:  price + atr*tp1Mult,
			TP2:  price + atr*tp2Mult,
			Pips: atr * pipMult,
		}
	}
	return TPSLResult{
		SL:   price + atr*slMult,
		TP1:  price - atr*tp1Mult,
		TP2:  price - atr*tp2Mult,
		Pips: atr * pipMult,
	}
}

// RiskReward returns the reward:risk ratio for a proposed trade.
func RiskReward(entry, tp1, sl float64) float64 {
	reward := tp1 - entry
	if reward < 0 {
		reward = -reward
	}
	risk := sl - entry
	if risk < 0 {
		risk = -risk
	}
	if risk == 0 {
		return 0
	}
	return reward / risk
}
