package collector

import (
	"sync"
	"time"

	"otc-predictor/pkg/types"
)

// CandleCache avoids redundant Deriv fetches when the current candle
// for a given granularity hasn't closed yet.
type CandleCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

type cacheEntry struct {
	candles     []types.Candle
	fetchedAt   time.Time
	granularity int
}

func NewCandleCache() *CandleCache {
	return &CandleCache{items: make(map[string]cacheEntry)}
}

func cacheKey(symbol string, granularity int) string {
	return symbol + ":" + itoa(granularity)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Get returns cached candles if still fresh relative to the candle's own granularity.
// A candle is considered "fresh" until roughly one granularity period has passed,
// since the current (still-forming) candle wouldn't have closed yet.
func (c *CandleCache) Get(symbol string, granularity int) ([]types.Candle, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[cacheKey(symbol, granularity)]
	if !ok {
		return nil, false
	}
	maxAge := time.Duration(granularity) * time.Second
	if maxAge > 60*time.Second {
		maxAge = 60 * time.Second // never cache longer than 60s regardless of TF, keeps live price fresh
	}
	if time.Since(entry.fetchedAt) > maxAge {
		return nil, false
	}
	return entry.candles, true
}

func (c *CandleCache) Set(symbol string, granularity int, candles []types.Candle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[cacheKey(symbol, granularity)] = cacheEntry{
		candles: candles, fetchedAt: time.Now(), granularity: granularity,
	}
}
