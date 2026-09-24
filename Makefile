.PHONY: run build clean test install help

# Default target
all: run

# Run the application
run:
	@echo "🚀 Starting OTC Predictor..."
	go run cmd/main.go

# Build the application
build:
	@echo "🔨 Building OTC Predictor..."
	go build -o bin/otc-predictor cmd/main.go
	@echo "✅ Build complete: bin/otc-predictor"

# Install dependencies
install:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf bin/
	go clean
	@echo "✅ Clean complete"

# Format code
fmt:
	@echo "📝 Formatting code..."
	go fmt ./...
	@echo "✅ Format complete"

# Lint code
lint:
	@echo "🔍 Linting code..."
	golangci-lint run
	@echo "✅ Lint complete"

# Development mode (with auto-reload)
dev:
	@echo "🔧 Starting in development mode..."
	@echo "⚠️  Install 'air' for auto-reload: go install github.com/cosmtrek/air@latest"
	air

# Check configuration
check-config:
	@echo "⚙️  Checking configuration..."
	@test -f config.yaml || (echo "❌ config.yaml not found!" && exit 1)
	@echo "✅ Configuration OK"

# Show API endpoints
endpoints:
	@echo ""
	@echo "📡 API Endpoints:"
	@echo "  Health:      GET  http://localhost:8080/api/health"
	@echo "  Markets:     GET  http://localhost:8080/api/markets"
	@echo "  Predict:     GET  http://localhost:8080/api/predict/:market/:duration"
	@echo "  Stats:       GET  http://localhost:8080/api/stats/:market"
	@echo "  Performance: GET  http://localhost:8080/api/performance"
	@echo "  Dashboard:        http://localhost:8080"
	@echo ""

# Analyze persisted trade results to see which signals actually win
analyze:
	@echo "📊 Analyzing trade results..."
	go run cmd/analyze/main.go

# Show help
help:
	@echo ""
	@echo "🎯 OTC Predictor - Available Commands"
	@echo "======================================"
	@echo ""
	@echo "  make run          - Run the application"
	@echo "  make build        - Build binary to bin/"
	@echo "  make install      - Install dependencies"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Lint code"
	@echo "  make dev          - Development mode with auto-reload"
	@echo "  make check-config - Verify configuration file"
	@echo "  make endpoints    - Show API endpoints"
	@echo "  make help         - Show this help"
	@echo ""
	@echo "📚 Quick Start:"
	@echo "  1. make install"
	@echo "  2. make run"
	@echo "  3. Open http://localhost:8080"
	@echo ""
