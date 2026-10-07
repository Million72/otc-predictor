package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"otc-predictor/internal/api"
	"otc-predictor/internal/collector"
	"otc-predictor/internal/storage"
	"otc-predictor/internal/streamer"
	"otc-predictor/internal/tracker"
)

func main() {
	cfg, err := api.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	deriv := collector.NewDerivClient(cfg.Deriv.AppID, cfg.Deriv.WSURL)
	store := storage.NewStore()

	// Build symbol -> deriv code map for tracker
	derivSymbols := map[string]string{}
	for _, m := range cfg.AllMarkets() {
		derivSymbols[m.Symbol] = m.Deriv
	}

	// Start persistent live tick streaming — keeps a continuously-updated
	// current price per symbol between full candle scans, independent of
	// the on-demand candle fetches the scanner uses.
	liveStore := streamer.NewLiveTickStore()
	tickStreamer := streamer.NewDerivStreamer(cfg.Deriv.WSURL, cfg.Deriv.AppID, cfg.AllMarkets(), liveStore)
	tickStreamer.Start()

	// Start background scanner (full candle-based analysis)
	scanner := api.NewScanner(cfg, deriv, store, liveStore)
	go scanner.Run()

	// Start outcome tracker (checks every 2 minutes) — uses live stream price when fresh
	trk := tracker.NewTracker(store, deriv, liveStore, derivSymbols)
	go trk.Run(2 * time.Minute)

	// Start HTTP API
	handlers := api.NewHandlers(store, cfg, liveStore)
	router := api.NewRouter(handlers)

	port := cfg.Server.Port
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	addr := ":" + strconv.Itoa(port)
	log.Printf("otc-predictor listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
