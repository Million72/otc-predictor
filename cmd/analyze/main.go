package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"otc-predictor/internal/api"
	"otc-predictor/internal/collector"
	"otc-predictor/internal/config"
	"otc-predictor/internal/predictor"
	"otc-predictor/internal/storage"
	"otc-predictor/internal/tracker"
	"otc-predictor/pkg/types"
)

func main() {
	log.Println("🚀 OTC Predictor Starting...")

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	log.Printf("✅ Configuration loaded: %d markets configured", len(cfg.Markets))

	// Initialize storage
	store := storage.NewMemoryStorage(cfg.Storage.MaxTicksInMemory)
	log.Println("✅ Storage initialized")

	// Initialize result tracker
	resultTracker := tracker.NewResultTracker(store)
	log.Println("✅ Result tracker initialized")

	// Initialize prediction engine
	engine := predictor.NewEngine(store, cfg, resultTracker)
	log.Println("✅ Prediction engine initialized")

	// Initialize OTC collector
	otcCollector := collector.NewOTCCollector(store, cfg.DataSource, cfg.Markets)
	if err := otcCollector.Start(); err != nil {
		log.Fatalf("❌ Failed to start OTC collector: %v", err)
	}

	// Wait for initial data collection (15 seconds) - faster startup
	log.Println("⏳ Collecting initial market data (15 seconds)...")
	time.Sleep(15 * time.Second)

	// Check if we have data
	activeMarkets := store.GetActiveMarkets()
	log.Printf("✅ Data collection started: %d markets active", len(activeMarkets))

	if len(activeMarkets) == 0 {
		log.Println("⚠️  No markets active yet, but continuing...")
	}

	// Start background tasks
	go startBackgroundTasks(engine, store, resultTracker, cfg)

	// Initialize and start API server
	server := api.NewServer(engine, store, resultTracker, cfg.API)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("❌ Failed to start API server: %v", err)
		}
	}()

	// Print usage instructions
	printUsageInstructions(cfg)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("✅ System ready! Press Ctrl+C to stop")
	<-quit

	log.Println("\n🛑 Shutting down gracefully...")

	// Stop collector
	otcCollector.Stop()

	// Shutdown API server
	if err := server.Shutdown(); err != nil {
		log.Printf("⚠️  Error during shutdown: %v", err)
	}

	// Print final performance summary
	log.Println("\n" + resultTracker.GetPerformanceSummary())

	log.Println("👋 Goodbye!")
}

// startBackgroundTasks starts background maintenance tasks
func startBackgroundTasks(engine *predictor.Engine, store *storage.MemoryStorage, tracker *tracker.ResultTracker, cfg types.Config) {
	// Cache cleanup every 30 seconds
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			engine.CleanupCache()
		}
	}()

	// Stats calculation every minute
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Tracking.CalculateStatsInterval) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			tracker.CalculateAllStats()
		}
	}()

	// Storage cleanup every hour
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.Storage.AutoCleanupInterval) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			store.Cleanup(cfg.Storage.KeepPredictionsHours)
			log.Println("🧹 Storage cleanup completed")
		}
	}()

	// Performance summary every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			summary := tracker.GetPerformanceSummary()
			log.Println(summary)
		}
	}()
}

// printUsageInstructions prints API usage instructions
func printUsageInstructions(cfg types.Config) {
	log.Println("\n" + strings.Repeat("=", 70))
	log.Println("📚 API USAGE INSTRUCTIONS")
	log.Println(strings.Repeat("=", 70))
	log.Printf("\n🌐 Dashboard: http://localhost:%d\n", cfg.API.Port)
	log.Printf("\n📡 ENDPOINTS:\n")
	log.Printf("  GET  /api/health                           - Health check\n")
	log.Printf("  GET  /api/markets                          - List active markets\n")
	log.Printf("  GET  /api/predict/:market/:duration        - Get prediction\n")
	log.Printf("  GET  /api/predict/all/:duration            - All market predictions\n")
	log.Printf("  GET  /api/stats                            - All statistics\n")
	log.Printf("  GET  /api/stats/:market                    - Market statistics\n")
	log.Printf("  GET  /api/results/:market                  - Trade results\n")
	log.Printf("  GET  /api/performance                      - Performance summary\n")
	log.Printf("  WS   /api/stream/:market?duration=60       - Real-time predictions\n")
	log.Printf("\n💡 EXAMPLES:\n")
	log.Printf("  curl http://localhost:%d/api/predict/volatility_75_1s/60\n", cfg.API.Port)
	log.Printf("  curl http://localhost:%d/api/stats/volatility_75_1s\n", cfg.API.Port)
	log.Printf("  curl http://localhost:%d/api/markets\n", cfg.API.Port)
	log.Println("\n" + strings.Repeat("=", 70) + "\n")
}

