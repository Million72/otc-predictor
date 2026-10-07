package api

import (
	"testing"
	"time"

	"otc-predictor/internal/storage"
	"otc-predictor/internal/streamer"
	"otc-predictor/pkg/types"
)

func TestReversalHysteresisActuallyRequiresTwoScans(t *testing.T) {
	store := storage.NewStore()
	cfg := &Config{}
	s := NewScanner(cfg, nil, store, streamer.NewLiveTickStore())

	store.SetSignal(types.Signal{Symbol: "EURUSD", Timeframe: "1h", Signal: "BUY", CandleTime: 1, Timestamp: time.Now()})

	first := types.Signal{Symbol: "EURUSD", Timeframe: "1h", Signal: "SELL", CandleTime: 2, Timestamp: time.Now()}
	s.checkReversalHysteresis(&first)
	if first.Signal != "WAIT" {
		t.Fatalf("first direct reversal must be held as WAIT")
	}

	second := types.Signal{Symbol: "EURUSD", Timeframe: "1h", Signal: "SELL", CandleTime: 2, Timestamp: time.Now()}
	s.checkReversalHysteresis(&second)
	if second.Signal != "SELL" {
		t.Fatalf("second consecutive reversal confirmation must be allowed")
	}
}
