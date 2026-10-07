package indicators

import (
	"otc-predictor/pkg/types"
	"testing"
)

func TestADXStaysWithinValidRange(t *testing.T) {
	candles := make([]types.Candle, 80)
	price := 100.0
	for i := range candles {
		open := price
		close := price + 0.5
		candles[i] = types.Candle{Open: open, High: close + 0.2, Low: open - 0.1, Close: close, Time: int64(i) * 60000}
		price = close
	}
	got := ADX(candles, 14)
	if got.ADX < 0 || got.ADX > 100 {
		t.Fatalf("ADX must be in [0,100], got %f", got.ADX)
	}
	if got.PlusDI < 0 || got.PlusDI > 100 || got.MinusDI < 0 || got.MinusDI > 100 {
		t.Fatalf("DI values must be in [0,100], got +DI=%f -DI=%f", got.PlusDI, got.MinusDI)
	}
}
