package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"otc-predictor/internal/storage"
	"otc-predictor/internal/streamer"
)

type Handlers struct {
	store     *storage.Store
	cfg       *Config
	liveStore *streamer.LiveTickStore
}

func NewHandlers(store *storage.Store, cfg *Config, liveStore *streamer.LiveTickStore) *Handlers {
	return &Handlers{store: store, cfg: cfg, liveStore: liveStore}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(v)
}

// GET /api/signals?tf=1h
func (h *Handlers) GetSignals(w http.ResponseWriter, r *http.Request) {
	tf := r.URL.Query().Get("tf")
	if tf == "" {
		tf = "1h"
	}
	signals := h.store.GetAllSignals(tf)
	writeJSON(w, map[string]interface{}{
		"timeframe": tf,
		"count":     len(signals),
		"signals":   signals,
	})
}

// GET /api/signal?symbol=EURUSD&tf=1h
func (h *Handlers) GetSignal(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	tf := r.URL.Query().Get("tf")
	if tf == "" {
		tf = "1h"
	}
	sig, ok := h.store.GetSignal(symbol, tf)
	if !ok {
		http.Error(w, "signal not found", http.StatusNotFound)
		return
	}
	writeJSON(w, sig)
}

// GET /api/history?limit=50
func (h *Handlers) GetHistory(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}
	history := h.store.GetHistory(limit)
	writeJSON(w, map[string]interface{}{
		"count":   len(history),
		"history": history,
	})
}

// GET /api/performance
func (h *Handlers) GetPerformance(w http.ResponseWriter, r *http.Request) {
	stats := h.store.GetPerformanceStats()
	writeJSON(w, map[string]interface{}{
		"stats": stats,
	})
}

// GET /api/outcomes
func (h *Handlers) GetOutcomes(w http.ResponseWriter, r *http.Request) {
	outcomes := h.store.GetAllOutcomes()
	writeJSON(w, map[string]interface{}{
		"count":    len(outcomes),
		"outcomes": outcomes,
	})
}

// GET /api/markets
func (h *Handlers) GetMarkets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"forex":     h.cfg.Markets.Forex,
		"synthetic": h.cfg.Markets.Synthetic,
	})
}

// GET /api/reliability
func (h *Handlers) GetReliability(w http.ResponseWriter, r *http.Request) {
	flags := h.store.GetReliabilityFlags()
	writeJSON(w, map[string]interface{}{
		"count":       len(flags),
		"reliability": flags,
	})
}

// GET /api/live-prices
func (h *Handlers) GetLivePrices(w http.ResponseWriter, r *http.Request) {
	prices := h.liveStore.All()
	writeJSON(w, map[string]interface{}{
		"count":  len(prices),
		"prices": prices,
	})
}

// GET /health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

// GET /api/debug/symbols?q=volatility
// One-off diagnostic: asks Deriv for its live active_symbols list so real
// symbol codes can be confirmed instead of guessed. Requires ?q= (case
// insensitive substring match against the symbol code or display name) to
// avoid dumping hundreds of entries at once.
func (h *Handlers) DebugSymbols(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "pass ?q=<search text>, e.g. /api/debug/symbols?q=volatility", http.StatusBadRequest)
		return
	}

	url := h.cfg.Deriv.WSURL
	if h.cfg.Deriv.AppID != "" {
		url = fmt.Sprintf("%s?app_id=%s", h.cfg.Deriv.WSURL, h.cfg.Deriv.AppID)
	}
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("dial error: %v", err), http.StatusBadGateway)
		return
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))

	if err := conn.WriteJSON(map[string]interface{}{"active_symbols": "brief"}); err != nil {
		http.Error(w, fmt.Sprintf("write error: %v", err), http.StatusBadGateway)
		return
	}

	// Field names differ between Deriv's legacy and new API, so accept both.
	var resp struct {
		ActiveSymbols []struct {
			Symbol               string `json:"symbol"`
			UnderlyingSymbol     string `json:"underlying_symbol"`
			DisplayName          string `json:"display_name"`
			UnderlyingSymbolName string `json:"underlying_symbol_name"`
			Market               string `json:"market"`
			Submarket            string `json:"submarket"`
		} `json:"active_symbols"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := conn.ReadJSON(&resp); err != nil {
		http.Error(w, fmt.Sprintf("read error: %v", err), http.StatusBadGateway)
		return
	}
	if resp.Error != nil {
		http.Error(w, fmt.Sprintf("deriv error: %s", resp.Error.Message), http.StatusBadGateway)
		return
	}

	type match struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		Market    string `json:"market"`
		Submarket string `json:"submarket"`
	}
	matches := []match{}
	for _, s := range resp.ActiveSymbols {
		code := s.Symbol
		if code == "" {
			code = s.UnderlyingSymbol
		}
		name := s.DisplayName
		if name == "" {
			name = s.UnderlyingSymbolName
		}
		if strings.Contains(strings.ToLower(code), q) || strings.Contains(strings.ToLower(name), q) {
			matches = append(matches, match{Code: code, Name: name, Market: s.Market, Submarket: s.Submarket})
		}
	}

	writeJSON(w, map[string]interface{}{
		"query":   q,
		"count":   len(matches),
		"matches": matches,
	})
}
