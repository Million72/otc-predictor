package api

import "net/http"

func NewRouter(h *Handlers) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/signals", h.GetSignals)
	mux.HandleFunc("/api/signal", h.GetSignal)
	mux.HandleFunc("/api/history", h.GetHistory)
	mux.HandleFunc("/api/performance", h.GetPerformance)
	mux.HandleFunc("/api/outcomes", h.GetOutcomes)
	mux.HandleFunc("/api/reliability", h.GetReliability)
	mux.HandleFunc("/api/live-prices", h.GetLivePrices)
	mux.HandleFunc("/api/markets", h.GetMarkets)
	mux.HandleFunc("/api/debug/symbols", h.DebugSymbols)

	// Serve dashboard
	mux.Handle("/", http.FileServer(http.Dir("./web")))

	return mux
}
