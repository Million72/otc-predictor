package api

import (
	"log"
	"sync"
	"time"

	"otc-predictor/internal/collector"
	"otc-predictor/internal/predictor"
	"otc-predictor/internal/spike"
	"otc-predictor/internal/storage"
	"otc-predictor/internal/strategy"
	"otc-predictor/internal/streamer"
	"otc-predictor/pkg/types"
)

type Scanner struct {
	cfg       *Config
	deriv     *collector.DerivClient
	store     *storage.Store
	cache     *collector.CandleCache
	liveStore *streamer.LiveTickStore
	pending   map[string]string // "symbol|tf" -> direction awaiting a second confirming scan
	pendingMu sync.Mutex
}

func NewScanner(cfg *Config, deriv *collector.DerivClient, store *storage.Store, liveStore *streamer.LiveTickStore) *Scanner {
	return &Scanner{cfg: cfg, deriv: deriv, store: store, cache: collector.NewCandleCache(), liveStore: liveStore, pending: make(map[string]string)}
}

// checkLivePriceDrift compares a just-computed signal (priced off the last
// CLOSED candle) against the actual current live tick price. The candle
// close can be up to one scan interval old, so price may have already moved
// since the numbers on this signal were calculated. Two outcomes:
//   - Price already reached TP1 or SL since the candle closed: the signal is
//     stale and no longer a valid entry — downgrade to WAIT and say why,
//     rather than showing a "BUY" that would already be a loser or whose
//     profit has already been taken.
//   - Price moved, but not past either level: signal stays valid, but a
//     drift note is attached so it's visible instead of hidden.
func (s *Scanner) checkLivePriceDrift(sig *types.Signal) {
	if sig.Signal != "BUY" && sig.Signal != "SELL" {
		return
	}
	if sig.TP1 == nil || sig.SL == nil {
		return
	}
	live, ok := s.liveStore.Get(sig.Symbol, 2*time.Minute)
	if !ok {
		return // no fresh live price to check against — leave the signal as-is
	}

	tp1, sl := *sig.TP1, *sig.SL
	drift := live - sig.Price
	driftPct := 0.0
	if sig.Price != 0 {
		driftPct = (drift / sig.Price) * 100
	}

	if sig.Signal == "BUY" {
		if live >= tp1 {
			sig.Signal = "WAIT"
			sig.BlockReason = "Stale — price already reached TP1 since this candle closed"
			return
		}
		if live <= sl {
			sig.Signal = "WAIT"
			sig.BlockReason = "Stale — price already hit SL since this candle closed"
			return
		}
	} else { // SELL
		if live <= tp1 {
			sig.Signal = "WAIT"
			sig.BlockReason = "Stale — price already reached TP1 since this candle closed"
			return
		}
		if live >= sl {
			sig.Signal = "WAIT"
			sig.BlockReason = "Stale — price already hit SL since this candle closed"
			return
		}
	}

	if driftPct > 0.05 || driftPct < -0.05 {
		sig.PriceDrift = &driftPct
	}
}

// checkReversalHysteresis prevents a signal from flipping straight from BUY to
// SELL (or vice versa) on a single scan. A direct reversal is downgraded to
// WAIT the first time it's seen; only if the same new direction is still
// true on the *next* scan is it allowed through. This stops signals that
// were computed off a now-stale candle close from immediately contradicting
// themselves the moment fresher data comes in.
func (s *Scanner) checkReversalHysteresis(sig *types.Signal) {
	key := sig.Symbol + "|" + sig.Timeframe
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	if sig.Signal != "BUY" && sig.Signal != "SELL" {
		delete(s.pending, key) // not actionable anyway — clear any stale pending flip
		return
	}

	prev, hadPrev := s.store.GetSignal(sig.Symbol, sig.Timeframe)
	isDirectFlip := hadPrev && (prev.Signal == "BUY" || prev.Signal == "SELL") && prev.Signal != sig.Signal
	if !isDirectFlip {
		delete(s.pending, key)
		return
	}

	if s.pending[key] == sig.Signal {
		// Same reversal direction confirmed on a second consecutive scan — allow it.
		delete(s.pending, key)
		return
	}

	// First time seeing this reversal — hold it back and wait for confirmation.
	s.pending[key] = sig.Signal
	original := sig.Signal
	sig.Signal = "WAIT"
	sig.BlockReason = original + " reversal pending confirmation — must hold for 2 consecutive scans before flipping"
}

// Run performs one full scan across all markets and timeframes on a schedule.
func (s *Scanner) Run() {
	s.scanOnce()
	ticker := time.NewTicker(time.Duration(s.cfg.ScanIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.scanOnce()
	}
}

func (s *Scanner) scanOnce() {
	markets := s.cfg.AllMarkets()
	log.Printf("scanner: starting scan of %d markets across %d timeframes", len(markets), len(s.cfg.Timeframes))

	for tfName, tfCfg := range s.cfg.Timeframes {
		s.scanTimeframe(markets, tfName, tfCfg)
	}
	log.Println("scanner: scan complete")
}

func (s *Scanner) scanTimeframe(markets []types.Market, tfName string, tfCfg types.TimeframeConfig) {
	const batchSize = 3
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchSize)

	for _, market := range markets {
		wg.Add(1)
		sem <- struct{}{}
		go func(m types.Market) {
			defer wg.Done()
			defer func() { <-sem }()
			s.scanMarket(m, tfName, tfCfg)
		}(market)
	}
	wg.Wait()
}

// fetchCandles checks the cache first, falls back to a retried live fetch,
// and populates the cache on success. This avoids hammering Deriv when the
// current candle for a granularity hasn't closed yet, while still recovering
// gracefully from transient WebSocket failures.
func (s *Scanner) fetchCandles(symbol string, granularity, count int) ([]types.Candle, error) {
	if cached, ok := s.cache.Get(symbol, granularity); ok && len(cached) >= count {
		return cached, nil
	}
	fresh, err := s.deriv.FetchCandlesWithRetry(symbol, granularity, count)
	if err != nil {
		return nil, err
	}
	s.cache.Set(symbol, granularity, fresh)
	return fresh, nil
}

// isBoomCrash routes Boom/Crash symbols to the dedicated spike engine
// (survival analysis + post-spike reaction) instead of the general
// indicator-scoring engine used for forex and other synthetics — those two
// index types have a fundamentally different underlying process and need
// fundamentally different math, not just different parameters on the same model.
func isBoomCrash(symbol string) bool {
	return spike.ContainsFold(symbol, "boom") || spike.ContainsFold(symbol, "crash")
}

func (s *Scanner) scanMarket(market types.Market, tfName string, tfCfg types.TimeframeConfig) {
	candles, err := s.fetchCandles(market.Deriv, tfCfg.Granularity, s.cfg.CandleCount)
	if err != nil {
		log.Printf("scanner: %s/%s candles error: %v", market.Symbol, tfName, err)
		s.store.SetSignal(types.Signal{
			Symbol: market.Symbol, Type: market.Type, Timeframe: tfName,
			Signal: "WAIT", Error: err.Error(), Timestamp: time.Now(),
		})
		return
	}

	// Boom/Crash indices: route to the dedicated survival-analysis + post-spike
	// reaction engine. No HTF fetch needed here — the spike model works off the
	// raw candle history of the instrument itself, not multi-timeframe trend bias.
	if isBoomCrash(market.Symbol) {
		sig := spike.RunSpikeEngine(market, candles)
		sig.Timeframe = tfName
		s.checkReversalHysteresis(&sig)
		s.checkLivePriceDrift(&sig)
		s.store.SetSignal(sig)
		return
	}

	htf1, err := s.fetchCandles(market.Deriv, tfCfg.HTFGran, s.cfg.HTFCandleCount)
	if err != nil {
		log.Printf("scanner: %s/%s htf1 error: %v", market.Symbol, tfName, err)
		htf1 = nil
	}

	htf2, err := s.fetchCandles(market.Deriv, tfCfg.HTF2Gran, s.cfg.HTFCandleCount)
	if err != nil {
		log.Printf("scanner: %s/%s htf2 error: %v", market.Symbol, tfName, err)
		htf2 = nil
	}

	// SMT Divergence needs a second, correlated instrument's candles. Only
	// EURUSD/GBPUSD currently have a defined partner (see strategy/smt.go) —
	// every other market gets partnerCandles = nil and entry models 3/4
	// simply don't apply to it. Reuses the same cached/retried fetch path.
	var partnerCandles []types.Candle
	partnerSymbol := strategy.GetSMTPartner(market.Symbol)
	if partnerSymbol != "" {
		for _, m := range s.cfg.AllMarkets() {
			if m.Symbol == partnerSymbol {
				pc, perr := s.fetchCandles(m.Deriv, tfCfg.Granularity, s.cfg.CandleCount)
				if perr == nil {
					partnerCandles = pc
				} else {
					log.Printf("scanner: %s/%s SMT partner fetch error: %v", market.Symbol, tfName, perr)
				}
				break
			}
		}
	}

	sig := predictor.BuildSignal(market, tfName, candles, htf1, htf2, partnerCandles)
	s.checkReversalHysteresis(&sig)
	s.checkLivePriceDrift(&sig)
	s.store.SetSignal(sig)
}
