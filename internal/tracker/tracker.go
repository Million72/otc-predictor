package tracker

import (
	"log"
	"time"

	"otc-predictor/internal/collector"
	"otc-predictor/internal/storage"
	"otc-predictor/internal/streamer"
)

// Tracker periodically checks pending signal outcomes against live prices.
// It prefers the continuously-streamed live tick price when fresh, since
// that's cheaper and more current than issuing a new Deriv request per check.
// If the stream has gone stale (or a symbol was never streamed), it falls
// back to a direct candle fetch so tracking never silently stalls.
type Tracker struct {
	store        *storage.Store
	deriv        *collector.DerivClient
	liveStore    *streamer.LiveTickStore
	derivSymbols map[string]string // symbol -> deriv code
}

const liveTickMaxAge = 15 * time.Second

func NewTracker(store *storage.Store, deriv *collector.DerivClient, liveStore *streamer.LiveTickStore, derivSymbols map[string]string) *Tracker {
	return &Tracker{store: store, deriv: deriv, liveStore: liveStore, derivSymbols: derivSymbols}
}

// Run starts the tracking loop — checks pending outcomes every interval.
func (t *Tracker) Run(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		t.checkOutcomes()
	}
}

func (t *Tracker) checkOutcomes() {
	pending := t.store.GetPendingOutcomes()
	if len(pending) == 0 {
		return
	}

	// Group by symbol to avoid redundant fetches within this pass
	checked := map[string]float64{}

	for _, o := range pending {
		// Expire signals older than 24h with no result
		if time.Since(o.CreatedAt) > 24*time.Hour {
			t.store.UpdateOutcome(o.SignalID, "EXPIRED")
			continue
		}

		derivCode, ok := t.derivSymbols[o.Symbol]
		if !ok {
			continue
		}

		price, ok := checked[o.Symbol]
		if !ok {
			// Prefer the live streamed price if it's fresh
			if livePrice, fresh := t.liveStore.Get(o.Symbol, liveTickMaxAge); fresh {
				price = livePrice
			} else {
				candles, err := t.deriv.FetchCandlesWithRetry(derivCode, 60, 2)
				if err != nil || len(candles) == 0 {
					log.Printf("tracker: failed to fetch price for %s: %v", o.Symbol, err)
					continue
				}
				price = candles[len(candles)-1].Close
			}
			checked[o.Symbol] = price
		}

		if o.Signal == "BUY" {
			if price >= o.TP1 {
				t.store.UpdateOutcome(o.SignalID, "TP1_HIT")
			} else if price <= o.SL {
				t.store.UpdateOutcome(o.SignalID, "SL_HIT")
			}
		} else if o.Signal == "SELL" {
			if price <= o.TP1 {
				t.store.UpdateOutcome(o.SignalID, "TP1_HIT")
			} else if price >= o.SL {
				t.store.UpdateOutcome(o.SignalID, "SL_HIT")
			}
		}
	}
}
