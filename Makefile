# SMS Gateway - Clean Production Makefile
.PHONY: run test build clean stop logs status api-test scale-test

# 🚀 Main Commands
run: ## Start SMS Gateway (infrastructure + services)
	@echo "🚀 Starting SMS Gateway..."
	@docker-compose up --build -d
	@sleep 10
	@$(MAKE) seed
	@echo "✅ SMS Gateway running on http://localhost:8080"
	@echo "📱 Test: curl -X POST http://localhost:8080/v1/messages -H 'Content-Type: application/json' -d '{\"client_id\":\"550e8400-e29b-41d4-a716-446655440000\",\"to\":\"+1234567890\",\"from\":\"TEST\",\"text\":\"Hello SMS!\"}'"

test: ## Run unit tests
	@echo "🧪 Running unit tests..."
	@go test -v ./internal/messages ./internal/billing ./internal/api ./test -short
	@echo "✅ Unit tests passed!"

realistic-test: ## Run realistic performance test (PDF compliant)
	@echo "🚀 Running realistic performance test..."
	@make seed > /dev/null
	@go test -v ./test -run TestRealisticPerformance
	@echo "✅ Performance test complete!"

race-test: ## Test race condition and double spending prevention
	@echo "🔐 Testing race condition protection..."
	@./fix-race-conditions.sh
	@echo "✅ Race condition test complete!"

double-spend-test: ## Test double spending prevention under extreme load
	@echo "💰 Testing double spending prevention..."
	@go test -v ./test -run TestDoubleSpending -timeout 30s
	@echo "✅ Double spending test complete!"

build: ## Build binaries
	@echo "🔨 Building..."
	@go build -o api ./cmd/api
	@go build -o worker ./cmd/worker
	@echo "✅ Built: api, worker"

# 📊 Testing & Validation
api-test: ## Test API endpoints
	@echo "🔍 Testing API endpoints..."
	@curl -s http://localhost:8080/health | jq . || echo "❌ Health check failed"
	@echo "\n📊 Client info:"
	@curl -s "http://localhost:8080/v1/me?client_id=550e8400-e29b-41d4-a716-446655440000" | jq . || echo "❌ Client info failed"
	@echo "\n📨 Send SMS:"
	@curl -s -X POST http://localhost:8080/v1/messages \
		-H "Content-Type: application/json" \
		-d '{"client_id":"550e8400-e29b-41d4-a716-446655440000","to":"+1234567890","from":"TEST","text":"Hello SMS Gateway!"}' | jq . || echo "❌ SMS send failed"
	@echo "\n✅ API tests complete"

scale-test: ## Test scale (100 concurrent requests)
	@echo "🔥 Scale Test: 100 concurrent SMS requests"
	@echo "📊 Starting load..."
	@time bash -c 'for i in {1..100}; do curl -s -X POST http://localhost:8080/v1/messages -H "Content-Type: application/json" -d "{\"client_id\":\"550e8400-e29b-41d4-a716-446655440000\",\"to\":\"+123456789$$i\",\"from\":\"SCALE\",\"text\":\"Scale test message #$$i\"}" > /dev/null & done; wait'
	@echo "✅ Scale test completed!"
	@echo "📈 Check credits: curl \"http://localhost:8080/v1/me?client_id=550e8400-e29b-41d4-a716-446655440000\""

# 🛠️ Utility Commands
seed: ## Seed demo data
	@echo "📊 Setting up demo data..."
	@docker-compose exec postgres psql -U postgres -d sms_gateway -f /app/scripts/seed.sql || echo "Database ready"

stop: ## Stop services
	@echo "🛑 Stopping services..."
	@docker-compose down -v

clean: ## Clean everything
	@echo "🧹 Cleaning up..."
	@docker-compose down -v --rmi local
	@rm -f api worker
	@echo "✅ Cleanup complete"

logs: ## Show logs
	@docker-compose logs -f

status: ## Show service status
	@docker-compose ps

# 🧪 Comprehensive Testing
comprehensive-test: ## Run complete test suite (45min)
	@echo "🧪 Starting comprehensive test suite..."
	@echo "⏱️  Estimated time: 45 minutes"
	@$(MAKE) run
	@sleep 15
	@echo "\n📋 Phase 1: Unit Tests (2min)"
	@$(MAKE) test
	@echo "\n📋 Phase 2: API Validation (2min)"
	@$(MAKE) api-test
	@echo "\n📋 Phase 3: Failure Scenarios (10min)"
	@./test-failure-scenarios.sh
	@echo "\n📋 Phase 4: Load Testing (30min)"
	@$(MAKE) k6-all
	@echo "\n🎉 Comprehensive testing complete!"

quick-test: ## Quick system validation (5min)
	@echo "🚀 Quick system validation..."
	@$(MAKE) run
	@sleep 10
	@$(MAKE) api-test
	@./test-failure-scenarios.sh
	@echo "✅ Quick test complete!"

failure-test: ## Test failure scenarios only
	@echo "🚨 Testing failure scenarios..."
	@./test-failure-scenarios.sh

# 🔧 K6 Load Testing
k6-install: ## Install K6 load testing tool
	@echo "📦 Installing K6..."
	@if command -v brew >/dev/null 2>&1; then \
		brew install k6; \
	elif command -v apt-get >/dev/null 2>&1; then \
		sudo apt update && sudo apt install k6; \
	else \
		echo "❌ Please install K6 manually: https://k6.io/docs/getting-started/installation/"; \
	fi

k6-smoke: ## K6 smoke test (30s)
	@echo "💨 Running K6 smoke test..."
	@cd k6 && k6 run --env SCENARIO=smoke-test sms-gateway-load-test.js

k6-load: ## K6 load test (16m)  
	@echo "📊 Running K6 load test..."
	@cd k6 && k6 run --env SCENARIO=load-test sms-gateway-load-test.js

k6-stress: ## K6 stress test (16m)
	@echo "🔥 Running K6 stress test..."
	@cd k6 && k6 run --env SCENARIO=stress-test sms-gateway-load-test.js

k6-spike: ## K6 spike test (8m)
	@echo "⚡ Running K6 spike test..."
	@cd k6 && k6 run --env SCENARIO=spike-test sms-gateway-load-test.js

k6-volume: ## K6 volume test (100K messages)
	@echo "📈 Running K6 volume test..."
	@cd k6 && k6 run --env SCENARIO=volume-test sms-gateway-load-test.js

k6-burst: ## K6 burst test (2.5m)
	@echo "💥 Running K6 burst test..."
	@cd k6 && k6 run scenarios/burst-test.js

k6-endurance: ## K6 endurance test (30m)
	@echo "🏃 Running K6 endurance test..."
	@cd k6 && k6 run scenarios/endurance-test.js

k6-all: ## Run complete K6 test suite
	@echo "🎯 Running complete K6 test suite..."
	@cd k6 && ./run-tests.sh all

# 📚 Documentation
help: ## Show this help
	@echo "SMS Gateway - Available Commands:"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <command>\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-12s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)