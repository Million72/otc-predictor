package collector

import (
	"otc-predictor/pkg/types"
	"testing"
	"time"
)

func TestClosedCandlesExcludesFormingCandle(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	candles := []types.Candle{
		{Time: now.Add(-2 * time.Minute).UnixMilli()},
		{Time: now.Add(-1 * time.Minute).UnixMilli()},
		{Time: now.UnixMilli()},
	}
	closed := ClosedCandles(candles, 60, now)
	if len(closed) != 2 {
		t.Fatalf("expected 2 closed candles, got %d", len(closed))
	}
	if closed[len(closed)-1].Time != now.Add(-time.Minute).UnixMilli() {
		t.Fatalf("latest returned candle should be the last fully closed candle")
	}
}
