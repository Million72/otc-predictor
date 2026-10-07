package strategy

import (
	"otc-predictor/pkg/types"
	"testing"
)

func TestDetectFlagUsesSeparateBreakoutCandle(t *testing.T) {
	candles := make([]types.Candle, 0, 16)
	price := 100.0
	for i := 0; i < 10; i++ {
		open := price
		close := price + 0.8
		candles = append(candles, types.Candle{Open: open, High: close + 0.1, Low: open - 0.1, Close: close, Time: int64(i+1) * 60000})
		price = close
	}
	// Tight downward flag.
	for i := 0; i < 5; i++ {
		open := price
		close := price - 0.1
		candles = append(candles, types.Candle{Open: open, High: open + 0.15, Low: close - 0.15, Close: close, Time: int64(i+11) * 60000})
		price = close
	}
	// Breakout candle: its close is above the flag's high.
	candles = append(candles, types.Candle{Open: price, High: price + 1.5, Low: price - 0.05, Close: price + 1.2, Time: 16 * 60000})

	got := detectFlag(candles)
	if got == nil || got.pattern.Name != "Bull Flag" || got.pattern.Side != "bull" {
		t.Fatalf("expected confirmed bull flag, got %#v", got)
	}
}
