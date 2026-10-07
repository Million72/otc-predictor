package storage

import (
	"otc-predictor/pkg/types"
	"testing"
	"time"
)

func TestStoreDeduplicatesSameSetup(t *testing.T) {
	tp, sl := 110.0, 95.0
	created := time.Unix(1_700_000_000, 0)
	s := NewStore()
	sig := types.Signal{Symbol: "EURUSD", Timeframe: "1h", Price: 100, Signal: "BUY", TP1: &tp, SL: &sl, CandleTime: created.UnixMilli(), Timestamp: created}
	s.SetSignal(sig)
	s.SetSignal(sig)
	if got := len(s.GetHistory(0)); got != 1 {
		t.Fatalf("expected one history entry, got %d", got)
	}
	if got := len(s.GetPendingOutcomes()); got != 1 {
		t.Fatalf("expected one pending outcome, got %d", got)
	}
}

func TestGetPendingOutcomesReturnsCopies(t *testing.T) {
	tp, sl := 110.0, 95.0
	s := NewStore()
	sig := types.Signal{Symbol: "EURUSD", Timeframe: "1h", Price: 100, Signal: "BUY", TP1: &tp, SL: &sl, CandleTime: 1, Timestamp: time.Unix(1, 0)}
	s.SetSignal(sig)
	pending := s.GetPendingOutcomes()
	pending[0].Outcome = "SL_HIT"
	again := s.GetPendingOutcomes()
	if again[0].Outcome != "PENDING" {
		t.Fatalf("mutating returned outcome must not mutate store")
	}
}
