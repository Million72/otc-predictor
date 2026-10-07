package streamer

import (
	"sync"
	"time"
)

// LivePrice holds the freshest known price for a symbol between full candle scans.
type LivePrice struct {
	Price     float64
	UpdatedAt time.Time
}

// LiveTickStore is a thread-safe cache of the latest streamed tick price per symbol.
// This is deliberately separate from the candle-based storage.Store — it exists
// purely to answer "what is the price RIGHT NOW" for the tracker and dashboard,
// without needing to wait for the next scheduled scan.
type LiveTickStore struct {
	mu     sync.RWMutex
	prices map[string]LivePrice
}

func NewLiveTickStore() *LiveTickStore {
	return &LiveTickStore{prices: make(map[string]LivePrice)}
}

func (s *LiveTickStore) Set(symbol string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices[symbol] = LivePrice{Price: price, UpdatedAt: time.Now()}
}

// Get returns the live price and true if it exists and is fresh (updated within maxAge).
// If the stream has gone stale (e.g. connection dropped), callers should fall back
// to the last known candle close instead of trusting an old tick.
func (s *LiveTickStore) Get(symbol string, maxAge time.Duration) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.prices[symbol]
	if !ok {
		return 0, false
	}
	if time.Since(p.UpdatedAt) > maxAge {
		return 0, false
	}
	return p.Price, true
}

func (s *LiveTickStore) All() map[string]LivePrice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]LivePrice, len(s.prices))
	for k, v := range s.prices {
		out[k] = v
	}
	return out
}
