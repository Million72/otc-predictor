package strategy

import (
	"otc-predictor/pkg/types"
	"testing"
)

func TestFindExecutionPlanArmsInsteadOfChasing(t *testing.T) {
	candles := make([]types.Candle, 0, 25)
	price := 100.0
	for i := 0; i < 20; i++ {
		candles = append(candles, types.Candle{Open: price, High: price + 0.2, Low: price - 0.2, Close: price, Time: int64(i + 1)})
	}
	// Fresh bullish order block around 100.0-100.4.
	candles[18] = types.Candle{Open: 100.2, High: 100.4, Low: 99.8, Close: 100.0, Time: 19}
	candles[19] = types.Candle{Open: 100.0, High: 100.7, Low: 99.9, Close: 100.5, Time: 20}
	for i := 0; i < 4; i++ {
		candles = append(candles, types.Candle{Open: 100.5, High: 100.6, Low: 100.4, Close: 100.5, Time: int64(21 + i)})
	}
	atr := 1.0
	p := FindExecutionPlan(candles, "bull", &atr, 0.75)
	if p.State != "ARMED" && p.State != "TRIGGERED" {
		t.Fatalf("expected armed/triggered plan, got %#v", p)
	}
	if p.DistanceATR > 0.75 {
		t.Fatalf("plan exceeds chase limit: %#v", p)
	}
}

func TestFindExecutionPlanMissesExtendedMove(t *testing.T) {
	candles := make([]types.Candle, 0, 22)
	for i := 0; i < 22; i++ {
		c := types.Candle{Open: 100, High: 100.2, Low: 99.8, Close: 100, Time: int64(i + 1)}
		candles = append(candles, c)
	}
	candles[18] = types.Candle{Open: 100.2, High: 100.4, Low: 99.8, Close: 100.0, Time: 19}
	candles[19] = types.Candle{Open: 100.0, High: 100.7, Low: 99.9, Close: 100.5, Time: 20}
	candles[20].Close = 104.0
	candles[20].High = 104.2
	candles[21].Close = 104.0
	candles[21].Open = 104.0
	candles[21].High = 104.1
	candles[21].Low = 103.9
	atr := 1.0
	p := FindExecutionPlan(candles, "bull", &atr, 0.75)
	if p.State != "MISSED" {
		t.Fatalf("expected MISSED for extended move, got %#v", p)
	}
}
