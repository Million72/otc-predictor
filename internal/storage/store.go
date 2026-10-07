package storage

import (
	"sync"
	"time"

	"otc-predictor/pkg/types"
)

// Store is a thread-safe in-memory store for signals and outcomes.
type Store struct {
	mu         sync.RWMutex
	signals    map[string]types.Signal         // key: symbol+timeframe
	history    []types.Signal                  // append-only log, capped
	outcomes   map[string]*types.SignalOutcome // key: signalID
	maxHistory int
}

func NewStore() *Store {
	return &Store{
		signals:    make(map[string]types.Signal),
		outcomes:   make(map[string]*types.SignalOutcome),
		maxHistory: 2000,
	}
}

func key(symbol, tf string) string { return symbol + "|" + tf }

func (s *Store) SetSignal(sig types.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals[key(sig.Symbol, sig.Timeframe)] = sig

	if sig.Signal == "BUY" || sig.Signal == "SELL" {
		s.history = append(s.history, sig)
		if len(s.history) > s.maxHistory {
			s.history = s.history[len(s.history)-s.maxHistory:]
		}
		// Only create a trackable outcome if real TP1/SL levels exist.
		// Previously missing TP1/SL silently defaulted to 0.0, and since any
		// live price is >= 0, every such BUY got marked a fake instant win
		// the moment the tracker next ran. Signals without real levels can't
		// be meaningfully tracked, so they're deliberately left PENDING-free
		// rather than fabricating a result.
		if sig.TP1 != nil && sig.SL != nil {
			id := sig.Symbol + "-" + sig.Timeframe + "-" + sig.Timestamp.Format(time.RFC3339)
			s.outcomes[id] = &types.SignalOutcome{
				SignalID:   id,
				Symbol:     sig.Symbol,
				Signal:     sig.Signal,
				EntryPrice: sig.Price,
				TP1:        *sig.TP1,
				SL:         *sig.SL,
				Outcome:    "PENDING",
				CreatedAt:  sig.Timestamp,
			}
		}
	}
}

func (s *Store) GetSignal(symbol, tf string) (types.Signal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sig, ok := s.signals[key(symbol, tf)]
	return sig, ok
}

func (s *Store) GetAllSignals(tf string) []types.Signal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []types.Signal{}
	for _, sig := range s.signals {
		if sig.Timeframe == tf {
			result = append(result, sig)
		}
	}
	return result
}

func (s *Store) GetHistory(limit int) []types.Signal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	start := len(s.history) - limit
	out := make([]types.Signal, limit)
	copy(out, s.history[start:])
	return out
}

func (s *Store) GetPendingOutcomes() []*types.SignalOutcome {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pending := []*types.SignalOutcome{}
	for _, o := range s.outcomes {
		if o.Outcome == "PENDING" {
			pending = append(pending, o)
		}
	}
	return pending
}

func (s *Store) UpdateOutcome(id, outcome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.outcomes[id]; ok {
		o.Outcome = outcome
		now := time.Now()
		o.ClosedAt = &now
	}
}

func (s *Store) GetPerformanceStats() []types.PerformanceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agg := map[string]*types.PerformanceStats{}
	for _, o := range s.outcomes {
		st, ok := agg[o.Symbol]
		if !ok {
			st = &types.PerformanceStats{Symbol: o.Symbol}
			agg[o.Symbol] = st
		}
		st.TotalSignals++
		switch o.Outcome {
		case "TP1_HIT":
			st.Wins++
		case "SL_HIT":
			st.Losses++
		case "PENDING":
			st.Pending++
		}
	}
	result := []types.PerformanceStats{}
	for _, st := range agg {
		closed := st.Wins + st.Losses
		if closed > 0 {
			st.WinRate = float64(st.Wins) / float64(closed) * 100
		}
		result = append(result, *st)
	}
	return result
}

func (s *Store) GetAllOutcomes() []*types.SignalOutcome {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*types.SignalOutcome{}
	for _, o := range s.outcomes {
		out = append(out, o)
	}
	return out
}
